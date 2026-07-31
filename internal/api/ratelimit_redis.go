package api

import (
	"context"
	"log/slog"
	"math"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"
)

// redisRateLimitPrefix namespaces rate-limit buckets in Redis, distinct from the
// route registry (`trqsh:route:`) and cert storage (`trqsh:certmagic:`) that may
// share the same instance. The per-limiter `name` is appended so the auth and API
// limiters keep independent buckets for the same client IP.
const redisRateLimitPrefix = "trqsh:rl:"

// redisAllowTimeout bounds each Redis round-trip so a slow/unreachable Redis
// degrades gracefully (see allow) instead of stalling request handling.
const redisAllowTimeout = 200 * time.Millisecond

// allowScript is an atomic token-bucket refill-decide-write. Doing the whole cycle
// in one Lua script is what makes the limiter correct under concurrency: a
// get-then-set from Go would race (lost decrements) across replicas and admit more
// than the burst. Semantics match localBucketStore.allow exactly: a first-seen key
// starts full and consumes one token; thereafter tokens refill at rps up to burst
// and each admitted request costs one.
//
//	KEYS[1] = bucket key
//	ARGV[1] = rps          (tokens per second)
//	ARGV[2] = burst        (bucket capacity)
//	ARGV[3] = now          (unix milliseconds)
//	ARGV[4] = ttl          (key TTL in milliseconds)
var allowScript = redis.NewScript(`
local rps   = tonumber(ARGV[1])
local burst = tonumber(ARGV[2])
local now   = tonumber(ARGV[3])
local ttl   = tonumber(ARGV[4])

local data   = redis.call('HMGET', KEYS[1], 'tokens', 'ts')
local tokens = tonumber(data[1])
local last   = tonumber(data[2])

if tokens == nil or last == nil then
  redis.call('HSET', KEYS[1], 'tokens', burst - 1, 'ts', now)
  redis.call('PEXPIRE', KEYS[1], ttl)
  return 1
end

local elapsed = (now - last) / 1000.0
if elapsed < 0 then elapsed = 0 end
tokens = math.min(burst, tokens + elapsed * rps)

local allowed = 0
if tokens >= 1 then
  tokens  = tokens - 1
  allowed = 1
end

redis.call('HSET', KEYS[1], 'tokens', tokens, 'ts', now)
redis.call('PEXPIRE', KEYS[1], ttl)
return allowed
`)

// redisBucketStore is the shared, cross-replica bucketStore. All API replicas
// pointed at the same Redis enforce ONE bucket per client, so the intended per-IP
// rate holds no matter how many replicas run.
type redisBucketStore struct {
	rdb     *redis.Client
	prefix  string
	rps     float64
	burst   float64
	ttlMs   int64
	timeout time.Duration    // per-request Redis round-trip budget
	now     func() time.Time // injectable clock (tests)

	// fallback is used only when Redis errors: graceful degradation to a bounded
	// local allowance (see allow).
	fallback *localBucketStore
	log      *slog.Logger

	// warn logging is throttled so a Redis outage can't flood the logs.
	warnMu   sync.Mutex
	lastWarn time.Time
}

func newRedisBucketStore(rdb *redis.Client, name string, rps, burst float64, log *slog.Logger) *redisBucketStore {
	if log == nil {
		log = slog.Default()
	}
	// TTL = time to refill an empty bucket to full (after which the stored state is
	// equivalent to a fresh key) plus a 1s margin. Idle buckets thus expire on their
	// own — Redis's PEXPIRE replaces the local reaper — while an actively-limited key
	// is rewritten (and its TTL refreshed) on every request.
	ttlMs := int64(60_000)
	if rps > 0 {
		ttlMs = int64(math.Ceil(burst/rps*1000)) + 1000
	}
	return &redisBucketStore{
		rdb:      rdb,
		prefix:   redisRateLimitPrefix + name + ":",
		rps:      rps,
		burst:    burst,
		ttlMs:    ttlMs,
		timeout:  redisAllowTimeout,
		now:      time.Now,
		fallback: newLocalBucketStore(rps, burst),
		log:      log,
	}
}

func (s *redisBucketStore) allow(key string) bool {
	ctx, cancel := context.WithTimeout(context.Background(), s.timeout)
	defer cancel()

	res, err := allowScript.Run(ctx, s.rdb, []string{s.prefix + key},
		s.rps, s.burst, s.now().UnixMilli(), s.ttlMs).Int()
	if err != nil {
		// Redis is briefly unreachable. Fail OPEN onto a bounded local bucket rather
		// than fail closed: taking down auth/login for every client during a
		// transient Redis blip would be a self-inflicted outage. Degrading to a
		// per-replica local limit still bounds abuse to (roughly) the pre-Redis
		// behavior — never unlimited — so it is strictly safer than a blanket allow.
		s.warn(err)
		return s.fallback.allow(key)
	}
	return res == 1
}

// warn logs a Redis-unavailability warning at most once per 30s so an outage does
// not produce a warning per request.
func (s *redisBucketStore) warn(err error) {
	s.warnMu.Lock()
	defer s.warnMu.Unlock()
	if time.Since(s.lastWarn) < 30*time.Second {
		return
	}
	s.lastWarn = time.Now()
	s.log.Warn("rate limiter: redis unavailable, falling back to local per-IP limit", "err", err)
}
