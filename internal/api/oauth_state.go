package api

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"log/slog"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"
)

// oauthStateTTL bounds the lifetime of an OAuth CSRF state nonce: long enough for a
// user to complete the provider redirect, short enough to limit replay exposure.
const oauthStateTTL = 10 * time.Minute

// redisOAuthStatePrefix namespaces OAuth state nonces in a Redis instance shared
// with the route registry (trqsh:route:), cert storage (trqsh:certmagic:) and rate
// limiter (trqsh:rl:).
const redisOAuthStatePrefix = "trqsh:oauthstate:"

// oauthStateStore issues single-use OAuth CSRF state nonces and consumes them on
// callback. The in-process implementation serves a single API replica; the Redis
// implementation lets every replica share one set of nonces, so an OAuth callback a
// load balancer routes to a different replica than the one that started the flow
// still validates (the bug this fixes). Mirrors the Registry / bucketStore split
// (internal/server/registry.go, middleware.go): an in-memory default plus a
// Redis-backed variant chosen by whether TRQSH_REDIS_URL is set.
type oauthStateStore interface {
	// New issues, records, and returns a fresh single-use state nonce.
	New() string
	// Check reports whether state is a currently-valid nonce, consuming it so a
	// second Check of the same nonce returns false (single-use). Empty state is never
	// valid.
	Check(state string) bool
}

var (
	_ oauthStateStore = (*memStateStore)(nil)
	_ oauthStateStore = (*redisStateStore)(nil)
)

// newOAuthStateNonce returns a 128-bit random hex nonce (an unguessable CSRF token).
func newOAuthStateNonce() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

// --- in-process ---

// memStateStore is the process-local oauthStateStore: a map of nonce -> expiry
// guarded by one mutex. It is the zero-config default (single API replica) and
// preserves the original Server.states behavior exactly (nonces consumed on Check).
type memStateStore struct {
	mu     sync.Mutex
	states map[string]time.Time
}

func newMemStateStore() *memStateStore {
	return &memStateStore{states: make(map[string]time.Time)}
}

func (m *memStateStore) New() string {
	state := newOAuthStateNonce()
	m.mu.Lock()
	m.states[state] = time.Now().Add(oauthStateTTL)
	m.mu.Unlock()
	return state
}

func (m *memStateStore) Check(state string) bool {
	if state == "" {
		return false
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	exp, ok := m.states[state]
	delete(m.states, state)
	return ok && time.Now().Before(exp)
}

// --- redis ---

// redisStateStore is the shared, cross-replica oauthStateStore. All API replicas
// pointed at the same Redis validate against ONE set of nonces, so a flow started on
// one replica completes on any other.
type redisStateStore struct {
	rdb     *redis.Client
	prefix  string
	ttl     time.Duration
	timeout time.Duration
	log     *slog.Logger

	// warn logging is throttled so a Redis outage can't flood the logs.
	warnMu   sync.Mutex
	lastWarn time.Time
}

func newRedisStateStore(rdb *redis.Client, log *slog.Logger) *redisStateStore {
	if log == nil {
		log = slog.Default()
	}
	return &redisStateStore{
		rdb:     rdb,
		prefix:  redisOAuthStatePrefix,
		ttl:     oauthStateTTL,
		timeout: redisAllowTimeout, // reuse the package's 200ms Redis round-trip budget
		log:     log,
	}
}

func (s *redisStateStore) New() string {
	state := newOAuthStateNonce()
	ctx, cancel := context.WithTimeout(context.Background(), s.timeout)
	defer cancel()
	// SETEX: the nonce auto-expires after ttl, so abandoned flows never accumulate
	// (unlike the in-process map). A write failure is logged, not fatal: the returned
	// nonce simply won't validate at callback, so the login fails closed and the user
	// retries — never a CSRF bypass.
	if err := s.rdb.Set(ctx, s.prefix+state, "1", s.ttl).Err(); err != nil {
		s.warn(err)
	}
	return state
}

func (s *redisStateStore) Check(state string) bool {
	if state == "" {
		return false
	}
	ctx, cancel := context.WithTimeout(context.Background(), s.timeout)
	defer cancel()
	// GETDEL is an atomic check-and-consume: the nonce is deleted as it is read, so
	// even two callbacks racing on the same nonce (a double-submit-cookie replay) can
	// succeed at most once. A GET-then-DEL pair would let both reads observe the value
	// before either delete lands — a genuine single-use race. GETDEL closes it without
	// the token-bucket Lua machinery, which is needed only for read-modify-write
	// counters, not a single check-and-delete.
	err := s.rdb.GetDel(ctx, s.prefix+state).Err()
	if err == redis.Nil {
		return false // unknown, already-consumed, or expired
	}
	if err != nil {
		// Redis unreachable: fail closed (reject) rather than admit an unverifiable
		// nonce. Unlike the rate limiter there is no safe local fallback — a nonce
		// lives on exactly one shared store, so a per-replica map cannot answer.
		s.warn(err)
		return false
	}
	return true
}

// warn logs a Redis-unavailability warning at most once per 30s so a Redis outage
// during the OAuth flow does not produce a warning per request.
func (s *redisStateStore) warn(err error) {
	s.warnMu.Lock()
	defer s.warnMu.Unlock()
	if time.Since(s.lastWarn) < 30*time.Second {
		return
	}
	s.lastWarn = time.Now()
	s.log.Warn("oauth state store: redis unavailable, state rejected (login fails closed)", "err", err)
}
