package api

import (
	"log/slog"
	"math"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"
)

// securityHeaders sets conservative security response headers on every response.
// It intentionally avoids a strict CSP so the self-hosted Swagger UI (/docs,
// which loads a pinned CDN bundle) keeps working; the API otherwise returns JSON.
func (s *Server) securityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h := w.Header()
		h.Set("X-Content-Type-Options", "nosniff")
		h.Set("X-Frame-Options", "DENY")
		h.Set("Referrer-Policy", "no-referrer")
		h.Set("Cross-Origin-Opener-Policy", "same-origin")
		if s.cfg.IsProduction() {
			h.Set("Strict-Transport-Security", "max-age=63072000; includeSubDomains")
		}
		next.ServeHTTP(w, r)
	})
}

// rateLimiter enforces a per-client token-bucket limit. The bucket state lives
// behind a bucketStore so the same middleware can be backed either process-locally
// (single API instance) or by Redis (shared across API replicas). Multiple
// replicas each keeping their own in-process buckets would silently grant N× the
// intended per-client rate — a real abuse/security regression — so a shared Redis
// store is used whenever TRQSH_REDIS_URL is set. Mirrors the Registry interface
// split in internal/server/registry.go.
type rateLimiter struct {
	store    bucketStore
	trustXFF bool
}

// bucketStore decides whether a request keyed by client IP may proceed, consuming
// one token on success. Implementations MUST be safe for concurrent use.
type bucketStore interface {
	allow(key string) bool
}

// newRateLimiter builds a limiter allowing `rps` requests/second per key with a
// `burst` allowance. When rdb is non-nil the buckets live in Redis under a
// per-limiter namespace (`name`), shared across API replicas; otherwise they are
// process-local. trustXFF honors X-Forwarded-For (only enable behind a trusted
// proxy). log receives throttled warnings if the Redis store must degrade.
func newRateLimiter(rdb *redis.Client, name string, rps, burst float64, trustXFF bool, log *slog.Logger) *rateLimiter {
	var store bucketStore
	if rdb != nil {
		store = newRedisBucketStore(rdb, name, rps, burst, log)
	} else {
		store = newLocalBucketStore(rps, burst)
	}
	return &rateLimiter{store: store, trustXFF: trustXFF}
}

func (rl *rateLimiter) allow(key string) bool { return rl.store.allow(key) }

// middleware enforces the limit per client IP, returning 429 when exceeded.
func (rl *rateLimiter) middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !rl.allow(clientIP(r, rl.trustXFF)) {
			w.Header().Set("Retry-After", "1")
			writeError(w, http.StatusTooManyRequests, "rate limit exceeded")
			return
		}
		next.ServeHTTP(w, r)
	})
}

// clientIP returns the source IP. X-Forwarded-For / X-Real-IP are honored ONLY
// when trustXFF is set (TRQSH_TRUST_PROXY), since otherwise a client can spoof
// them to evade per-IP limits.
func clientIP(r *http.Request, trustXFF bool) string {
	if trustXFF {
		if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
			if ip := strings.TrimSpace(strings.Split(xff, ",")[0]); ip != "" {
				return ip
			}
		}
		if xr := strings.TrimSpace(r.Header.Get("X-Real-IP")); xr != "" {
			return xr
		}
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}

// --- process-local bucket store ---

// localBucketStore is the single-instance bucketStore: a per-key token bucket
// guarded by one mutex. Buckets are created lazily and reaped when idle, bounding
// memory under a flood of distinct source IPs. This is the zero-config default and
// the graceful-degradation fallback when Redis is unreachable.
type localBucketStore struct {
	mu      sync.Mutex
	buckets map[string]*tokenBucket
	rps     float64
	burst   float64
}

type tokenBucket struct {
	tokens float64
	last   time.Time
}

func newLocalBucketStore(rps, burst float64) *localBucketStore {
	s := &localBucketStore{
		buckets: make(map[string]*tokenBucket),
		rps:     rps,
		burst:   burst,
	}
	go s.reap()
	return s
}

func (s *localBucketStore) allow(key string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	now := time.Now()
	b, ok := s.buckets[key]
	if !ok {
		s.buckets[key] = &tokenBucket{tokens: s.burst - 1, last: now}
		return true
	}
	b.tokens = math.Min(s.burst, b.tokens+now.Sub(b.last).Seconds()*s.rps)
	b.last = now
	if b.tokens < 1 {
		return false
	}
	b.tokens--
	return true
}

func (s *localBucketStore) reap() {
	t := time.NewTicker(10 * time.Minute)
	defer t.Stop()
	for range t.C {
		cutoff := time.Now().Add(-10 * time.Minute)
		s.mu.Lock()
		for k, b := range s.buckets {
			if b.last.Before(cutoff) {
				delete(s.buckets, k)
			}
		}
		s.mu.Unlock()
	}
}
