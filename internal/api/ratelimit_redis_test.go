package api

import (
	"context"
	"io"
	"log/slog"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
)

// nopRedisLogger silences go-redis's internal connection-pool logger (which prints
// dial-failure noise to stderr during the fail-open test). It structurally
// satisfies the unexported internal.Logging interface accepted by redis.SetLogger.
type nopRedisLogger struct{}

func (nopRedisLogger) Printf(context.Context, string, ...interface{}) {}

func init() { redis.SetLogger(nopRedisLogger{}) }

func discardLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

func newTestRedis(t *testing.T) (*miniredis.Miniredis, *redis.Client) {
	t.Helper()
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatalf("miniredis: %v", err)
	}
	t.Cleanup(mr.Close)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = rdb.Close() })
	return mr, rdb
}

// TestRedisBucketConcurrentBurst is the atomicity check. If the refill-decide-write
// cycle were NOT atomic (a get-then-set from Go), concurrent requests would lose
// decrements and admit more than the burst. With the Lua script, firing many
// goroutines at one key admits exactly `burst`. A frozen clock guarantees zero
// refill during the burst, so any admission beyond `burst` can only be a race.
func TestRedisBucketConcurrentBurst(t *testing.T) {
	_, rdb := newTestRedis(t)
	const burst = 100
	store := newRedisBucketStore(rdb, "auth", 5, burst, discardLogger())
	frozen := time.Now()
	store.now = func() time.Time { return frozen }

	const workers = 600
	var admitted int64
	var wg sync.WaitGroup
	start := make(chan struct{})
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			if store.allow("1.2.3.4") {
				atomic.AddInt64(&admitted, 1)
			}
		}()
	}
	close(start)
	wg.Wait()

	if admitted != burst {
		t.Fatalf("admitted=%d, want exactly %d (burst); >burst means the read-refill-write cycle raced (lost updates)", admitted, burst)
	}
}

// TestRedisBucketSeparateKeys confirms distinct client IPs get independent buckets.
func TestRedisBucketSeparateKeys(t *testing.T) {
	_, rdb := newTestRedis(t)
	store := newRedisBucketStore(rdb, "auth", 5, 3, discardLogger())
	frozen := time.Now()
	store.now = func() time.Time { return frozen }

	for i := 0; i < 3; i++ {
		if !store.allow("a") {
			t.Fatalf("key a request %d denied within burst", i)
		}
	}
	if store.allow("a") {
		t.Fatal("key a admitted beyond burst")
	}
	// A different key must still have its full burst.
	if !store.allow("b") {
		t.Fatal("key b denied despite its own fresh bucket")
	}
}

// TestRedisBucketNamespaceIsolation confirms the auth (5/10) and api (50/100)
// limiters keep independent buckets for the same client IP — the per-limiter name
// must be part of the key.
func TestRedisBucketNamespaceIsolation(t *testing.T) {
	_, rdb := newTestRedis(t)
	frozen := time.Now()
	auth := newRedisBucketStore(rdb, "auth", 5, 2, discardLogger())
	api := newRedisBucketStore(rdb, "api", 50, 2, discardLogger())
	auth.now = func() time.Time { return frozen }
	api.now = func() time.Time { return frozen }
	ip := "9.9.9.9"
	// Exhaust the auth bucket.
	if !auth.allow(ip) || !auth.allow(ip) {
		t.Fatal("auth burst should admit 2")
	}
	if auth.allow(ip) {
		t.Fatal("auth admitted beyond burst")
	}
	// The api bucket for the same IP must be untouched.
	if !api.allow(ip) || !api.allow(ip) {
		t.Fatal("api bucket wrongly shares state with auth for the same IP")
	}
}

// TestRedisBucketRefill exercises the time-based refill via an injected clock: after
// the burst is spent, a token returns once >= 1/rps seconds pass.
func TestRedisBucketRefill(t *testing.T) {
	_, rdb := newTestRedis(t)
	const rps, burst = 5.0, 3.0
	store := newRedisBucketStore(rdb, "auth", rps, burst, discardLogger())
	base := time.Now()
	cur := base
	store.now = func() time.Time { return cur }

	for i := 0; i < int(burst); i++ {
		if !store.allow("c") {
			t.Fatalf("request %d denied within burst", i)
		}
	}
	if store.allow("c") {
		t.Fatal("admitted immediately after burst exhausted")
	}
	// Advance just past one refill interval (1/rps = 200ms) => ~1 token returns.
	cur = base.Add(time.Duration(float64(time.Second)/rps) + 5*time.Millisecond)
	if !store.allow("c") {
		t.Fatal("token did not refill after advancing the clock past 1/rps")
	}
	// ...but only one token: a second immediate request is denied again.
	if store.allow("c") {
		t.Fatal("refilled more than one token for a single interval")
	}
}

// TestRedisBucketFailOpen documents the Redis-unavailable decision: requests fall
// back to a bounded LOCAL bucket (graceful degradation), never 0 (which would be a
// self-inflicted outage) and never unlimited.
func TestRedisBucketFailOpen(t *testing.T) {
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatalf("miniredis: %v", err)
	}
	// Fast-failing client so each call gives up quickly once Redis is gone.
	rdb := redis.NewClient(&redis.Options{
		Addr:        mr.Addr(),
		DialTimeout: 30 * time.Millisecond,
		MaxRetries:  -1,
		PoolSize:    1,
	})
	t.Cleanup(func() { _ = rdb.Close() })

	const burst = 5
	store := newRedisBucketStore(rdb, "auth", 0.01, burst, discardLogger())
	store.timeout = 40 * time.Millisecond
	mr.Close() // Redis now unreachable.

	admitted := 0
	for i := 0; i < 30; i++ {
		if store.allow("8.8.8.8") {
			admitted++
		}
	}
	if admitted == 0 {
		t.Fatal("fail-open expected: fallback must admit within the local burst, not deny everything")
	}
	if admitted > burst+1 {
		t.Fatalf("fallback must bound admissions to ~burst(%d), got %d", burst, admitted)
	}
}
