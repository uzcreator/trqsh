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
	Close() error
}

// --- in-memory ---

type memEntry struct {
	b      Binding
	expiry time.Time
}

// InMemoryRegistry is a process-local Registry used when TRQSH_REDIS_URL is
// unset and in tests. TTLs are honored lazily on Lookup.
type InMemoryRegistry struct {
	mu sync.RWMutex
	m  map[string]memEntry
}

// NewInMemoryRegistry returns an empty in-memory registry.
func NewInMemoryRegistry() *InMemoryRegistry {
	return &InMemoryRegistry{m: make(map[string]memEntry)}
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

func (r *InMemoryRegistry) Close() error { return nil }

// --- redis ---

const (
	redisRoutePrefix = "trqsh:route:"
	redisRouteEvents = "trqsh:routes"
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

func (r *RedisRegistry) Close() error { return r.rdb.Close() }

func expiryFrom(ttl time.Duration) time.Time {
	if ttl <= 0 {
		return time.Time{}
	}
	return time.Now().Add(ttl)
}
