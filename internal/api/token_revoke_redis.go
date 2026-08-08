package api

import (
	"context"
	"log/slog"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"

	"github.com/trqsh-uz/trqsh/internal/api/auth"
)

// redisRevokePrefix namespaces revoked refresh-token jtis in the Redis instance
// shared with the route registry, cert storage, rate limiter, OAuth state, and
// device store.
const redisRevokePrefix = "trqsh:revoked:"

// redisRevoker is the shared, cross-replica auth.Revoker: a logout on any API
// replica records the refresh token's jti here, so the replica a later refresh
// lands on rejects it too. Mirrors the OAuth-state / device-store seam
// (oauth_state.go, device_redis.go) — an in-process default plus a Redis-backed
// variant chosen by whether TRQSH_REDIS_URL is set.
type redisRevoker struct {
	rdb     *redis.Client
	prefix  string
	timeout time.Duration
	log     *slog.Logger

	warnMu   sync.Mutex
	lastWarn time.Time
}

var _ auth.Revoker = (*redisRevoker)(nil)

func newRedisRevoker(rdb *redis.Client, log *slog.Logger) *redisRevoker {
	if log == nil {
		log = slog.Default()
	}
	return &redisRevoker{
		rdb:     rdb,
		prefix:  redisRevokePrefix,
		timeout: redisAllowTimeout, // reuse the package's short Redis round-trip budget
		log:     log,
	}
}

func (r *redisRevoker) Revoke(jti string, ttl time.Duration) {
	if jti == "" || ttl <= 0 {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), r.timeout)
	defer cancel()
	// SETEX: the key auto-expires with the token, so revocations never accumulate.
	// A write failure is logged, not fatal — worst case the logout doesn't stick
	// on this replica and the (short-lived) access token still lapses on its own.
	if err := r.rdb.Set(ctx, r.prefix+jti, "1", ttl).Err(); err != nil {
		r.warn(err)
	}
}

func (r *redisRevoker) IsRevoked(jti string) bool {
	if jti == "" {
		return false
	}
	ctx, cancel := context.WithTimeout(context.Background(), r.timeout)
	defer cancel()
	n, err := r.rdb.Exists(ctx, r.prefix+jti).Result()
	if err != nil {
		// Fail OPEN: a Redis blip must not reject every refresh (which would sign
		// users out on an infra hiccup). Revocation is a best-effort kill switch;
		// the 1h access-token TTL still bounds a stolen refresh token's window until
		// Redis recovers. (Unlike the OAuth-state nonce, which fails closed because
		// admitting an unverifiable nonce would be a CSRF bypass.)
		r.warn(err)
		return false
	}
	return n > 0
}

// warn logs a Redis-unavailability warning at most once per 30s so an outage
// doesn't produce a warning per refresh.
func (r *redisRevoker) warn(err error) {
	r.warnMu.Lock()
	defer r.warnMu.Unlock()
	if time.Since(r.lastWarn) < 30*time.Second {
		return
	}
	r.lastWarn = time.Now()
	r.log.Warn("token revoker: redis unavailable, revocation best-effort", "err", err)
}
