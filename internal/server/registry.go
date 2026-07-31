package server

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"
)

// Route identifies a public entry point: a hostname (HTTP/HTTPS/TLS) or a
// proto+port (TCP/UDP).
type Route struct {
	Host  string
	Proto string // "tcp" | "udp" when Host == ""
	Port  int
}

// Key is the stable registry key for the route.
func (r Route) Key() string {
	if r.Host != "" {
		return "host:" + strings.ToLower(r.Host)
	}
	return fmt.Sprintf("port:%s:%d", r.Proto, r.Port)
}

// Binding is where a route is homed.
type Binding struct {
	EdgeID    string
	SessionID string
}

// Registry is the shared, cross-edge route store. The in-memory implementation
// serves a single edge; the Redis implementation lets many edges share routes
// (and, later, forward traffic between them — see forward.go).
type Registry interface {
	Bind(ctx context.Context, r Route, b Binding, ttl time.Duration) error
	Refresh(ctx context.Context, r Route, ttl time.Duration) error
	Lookup(ctx context.Context, r Route) (Binding, bool, error)
	Unbind(ctx context.Context, r Route) error

	// Edge presence: each edge advertises its own internal forwarding address under
	// its EdgeID so a peer can turn a route's Binding.EdgeID into a dialable address
	// (see forward.go). This mirrors the route Bind/Refresh/Unbind TTL pattern,
	// keyed by EdgeID instead of Route: RegisterEdge sets (and, re-called on the
	// heartbeat cadence, refreshes) the address with a TTL; UnregisterEdge removes
	// it at drain so peers stop forwarding to a shutting-down edge.
	RegisterEdge(ctx context.Context, edgeID, addr string, ttl time.Duration) error
	LookupEdge(ctx context.Context, edgeID string) (addr string, ok bool, err error)
	UnregisterEdge(ctx context.Context, edgeID string) error

	Close() error
}

// --- in-memory ---

type memEntry struct {
	b      Binding
	expiry time.Time
}

type edgeEntry struct {
	addr   string
	expiry time.Time
}

// InMemoryRegistry is a process-local Registry used when TRQSH_REDIS_URL is
// unset and in tests. TTLs are honored lazily on Lookup. A single-edge process
// never forwards (no peer bindings can exist), so the edge map is present only to
// satisfy the interface.
type InMemoryRegistry struct {
	mu    sync.RWMutex
	m     map[string]memEntry
	edges map[string]edgeEntry
}

// NewInMemoryRegistry returns an empty in-memory registry.
func NewInMemoryRegistry() *InMemoryRegistry {
	return &InMemoryRegistry{m: make(map[string]memEntry), edges: make(map[string]edgeEntry)}
}

func (r *InMemoryRegistry) Bind(_ context.Context, route Route, b Binding, ttl time.Duration) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.m[route.Key()] = memEntry{b: b, expiry: expiryFrom(ttl)}
	return nil
}

func (r *InMemoryRegistry) Refresh(_ context.Context, route Route, ttl time.Duration) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if e, ok := r.m[route.Key()]; ok {
		e.expiry = expiryFrom(ttl)
		r.m[route.Key()] = e
	}
	return nil
}

func (r *InMemoryRegistry) Lookup(_ context.Context, route Route) (Binding, bool, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	e, ok := r.m[route.Key()]
	if !ok {
		return Binding{}, false, nil
	}
	if !e.expiry.IsZero() && time.Now().After(e.expiry) {
		return Binding{}, false, nil
	}
	return e.b, true, nil
}

func (r *InMemoryRegistry) Unbind(_ context.Context, route Route) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.m, route.Key())
	return nil
}

func (r *InMemoryRegistry) RegisterEdge(_ context.Context, edgeID, addr string, ttl time.Duration) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.edges[edgeID] = edgeEntry{addr: addr, expiry: expiryFrom(ttl)}
	return nil
}

func (r *InMemoryRegistry) LookupEdge(_ context.Context, edgeID string) (string, bool, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	e, ok := r.edges[edgeID]
	if !ok {
		return "", false, nil
	}
	if !e.expiry.IsZero() && time.Now().After(e.expiry) {
		return "", false, nil
	}
	return e.addr, true, nil
}

func (r *InMemoryRegistry) UnregisterEdge(_ context.Context, edgeID string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.edges, edgeID)
	return nil
}

func (r *InMemoryRegistry) Close() error { return nil }

// --- redis ---

const (
	redisRoutePrefix = "trqsh:route:"
	redisRouteEvents = "trqsh:routes"
	// Edge forwarding addresses live under a distinct namespace in the same Redis
	// (TRQSH_REDIS_URL) the route registry and cert storage share.
	redisEdgePrefix = "trqsh:edge:"
)

// RedisRegistry stores routes in Redis with a TTL and announces bind/unbind on
// a pub/sub channel so peer edges can maintain a routing view.
type RedisRegistry struct {
	rdb *redis.Client
}

// NewRedisRegistry connects to Redis using a redis:// URL.
func NewRedisRegistry(url string) (*RedisRegistry, error) {
	opt, err := redis.ParseURL(url)
	if err != nil {
		return nil, fmt.Errorf("registry: parse redis url: %w", err)
	}
	return &RedisRegistry{rdb: redis.NewClient(opt)}, nil
}

func (r *RedisRegistry) key(route Route) string { return redisRoutePrefix + route.Key() }

func (r *RedisRegistry) Bind(ctx context.Context, route Route, b Binding, ttl time.Duration) error {
	if err := r.rdb.Set(ctx, r.key(route), b.EdgeID+"|"+b.SessionID, ttl).Err(); err != nil {
		return err
	}
	r.rdb.Publish(ctx, redisRouteEvents, "bind "+route.Key())
	return nil
}

func (r *RedisRegistry) Refresh(ctx context.Context, route Route, ttl time.Duration) error {
	return r.rdb.Expire(ctx, r.key(route), ttl).Err()
}

func (r *RedisRegistry) Lookup(ctx context.Context, route Route) (Binding, bool, error) {
	v, err := r.rdb.Get(ctx, r.key(route)).Result()
	if err == redis.Nil {
		return Binding{}, false, nil
	}
	if err != nil {
		return Binding{}, false, err
	}
	edge, sess, _ := strings.Cut(v, "|")
	return Binding{EdgeID: edge, SessionID: sess}, true, nil
}

func (r *RedisRegistry) Unbind(ctx context.Context, route Route) error {
	if err := r.rdb.Del(ctx, r.key(route)).Err(); err != nil {
		return err
	}
	r.rdb.Publish(ctx, redisRouteEvents, "unbind "+route.Key())
	return nil
}

// RegisterEdge SETs the edge's forwarding address with a TTL. Re-calling it (each
// heartbeat) refreshes value + TTL atomically — more robust than an EXPIRE-only
// refresh, which would silently no-op if the key had already lapsed.
func (r *RedisRegistry) RegisterEdge(ctx context.Context, edgeID, addr string, ttl time.Duration) error {
	return r.rdb.Set(ctx, redisEdgePrefix+edgeID, addr, ttl).Err()
}

func (r *RedisRegistry) LookupEdge(ctx context.Context, edgeID string) (string, bool, error) {
	addr, err := r.rdb.Get(ctx, redisEdgePrefix+edgeID).Result()
	if err == redis.Nil {
		return "", false, nil
	}
	if err != nil {
		return "", false, err
	}
	return addr, true, nil
}

func (r *RedisRegistry) UnregisterEdge(ctx context.Context, edgeID string) error {
	return r.rdb.Del(ctx, redisEdgePrefix+edgeID).Err()
}

func (r *RedisRegistry) Close() error { return r.rdb.Close() }

func expiryFrom(ttl time.Duration) time.Time {
	if ttl <= 0 {
		return time.Time{}
	}
	return time.Now().Add(ttl)
}
