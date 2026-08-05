package server

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"io/fs"
	"log/slog"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/caddyserver/certmagic"
	"github.com/redis/go-redis/v9"
)

// Redis key layout for cert storage. Namespaced distinctly from the route registry
// (`trqsh:route:`) since both subsystems typically share one Redis (TRQSH_REDIS_URL):
//
//	trqsh:certmagic:f:<key>   HASH  one stored "file": value + size + modified
//	trqsh:certmagic:index     SET   every stored file key (for directory ops)
//	trqsh:certmagic:lock:<n>  STR   distributed issuance lock (SET NX PX)
const (
	certmagicKeyPrefix  = "trqsh:certmagic:"
	certmagicFilePrefix = certmagicKeyPrefix + "f:"
	certmagicIndexKey   = certmagicKeyPrefix + "index"
	certmagicLockPrefix = certmagicKeyPrefix + "lock:"
	certmagicFieldValue = "v"
	certmagicFieldSize  = "s"
	certmagicFieldModNs = "m"
)

// Lock lease timings. The lease auto-expires so a crashed edge can't deadlock
// issuance for the others; a background refresher extends it while the lock is
// held so a slow ACME order (DNS-01 propagation can take minutes) doesn't lose it.
const (
	certmagicLockTTL      = 60 * time.Second
	certmagicLockPollWait = 1 * time.Second
)

// releaseScript deletes the lock key only if we still own it (value == our token),
// so a lease that expired and was re-acquired by another edge is never clobbered.
var releaseScript = redis.NewScript(`
if redis.call("GET", KEYS[1]) == ARGV[1] then
  return redis.call("DEL", KEYS[1])
end
return 0
`)

// refreshScript extends the lease only while we still own the lock.
var refreshScript = redis.NewScript(`
if redis.call("GET", KEYS[1]) == ARGV[1] then
  return redis.call("PEXPIRE", KEYS[1], ARGV[2])
end
return 0
`)

// RedisStorage is a certmagic.Storage backed by Redis so multiple edges share one
// certificate cache AND coordinate issuance through a distributed lock. Without
// shared storage each edge would independently order the same names from Let's
// Encrypt, risking a per-domain rate-limit lockout that breaks renewals for every
// edge. CertMagic treats instances sharing a Storage as one cluster.
//
// This is a small first-party client over github.com/redis/go-redis/v9 (already a
// dependency), mirroring the repo's preference for hand-rolled clients over heavy
// third-party plugins (cf. internal/billing/stripe).
type RedisStorage struct {
	rdb      *redis.Client
	lockTTL  time.Duration
	pollWait time.Duration
	log      *slog.Logger

	mu    sync.Mutex
	locks map[string]*heldLock
}

// heldLock tracks a lock this instance currently holds so Unlock can safely release
// it (token match) and stop its background lease refresher.
type heldLock struct {
	token string
	stop  chan struct{}
	done  chan struct{}
}

// NewRedisStorage connects to Redis using a redis:// URL (the same TRQSH_REDIS_URL
// the route registry uses — one Redis, distinct key namespaces).
func NewRedisStorage(url string, log *slog.Logger) (*RedisStorage, error) {
	opt, err := redis.ParseURL(url)
	if err != nil {
		return nil, fmt.Errorf("server: certmagic redis storage: parse url: %w", err)
	}
	if log == nil {
		log = slog.Default()
	}
	return &RedisStorage{
		rdb:      redis.NewClient(opt),
		lockTTL:  certmagicLockTTL,
		pollWait: certmagicLockPollWait,
		log:      log,
		locks:    make(map[string]*heldLock),
	}, nil
}

// Interface guard: catch any signature drift against the pinned certmagic version.
var _ certmagic.Storage = (*RedisStorage)(nil)

func (s *RedisStorage) fileKey(key string) string { return certmagicFilePrefix + key }

// Store puts value at key, creating or overwriting it. The value + metadata hash
// and the directory index are updated atomically (MULTI/EXEC) so a listing never
// races ahead of the data.
func (s *RedisStorage) Store(ctx context.Context, key string, value []byte) error {
	pipe := s.rdb.TxPipeline()
	pipe.HSet(ctx, s.fileKey(key),
		certmagicFieldValue, value,
		certmagicFieldSize, len(value),
		certmagicFieldModNs, time.Now().UnixNano(),
	)
	pipe.SAdd(ctx, certmagicIndexKey, key)
	if _, err := pipe.Exec(ctx); err != nil {
		return fmt.Errorf("server: certmagic redis store %q: %w", key, err)
	}
	return nil
}

// Load retrieves the value at key, returning fs.ErrNotExist when absent (certmagic
// checks errors.Is(err, fs.ErrNotExist)).
func (s *RedisStorage) Load(ctx context.Context, key string) ([]byte, error) {
	v, err := s.rdb.HGet(ctx, s.fileKey(key), certmagicFieldValue).Bytes()
	if err == redis.Nil {
		return nil, fs.ErrNotExist
	}
	if err != nil {
		return nil, fmt.Errorf("server: certmagic redis load %q: %w", key, err)
	}
	return v, nil
}

// Delete removes key and, when key is a directory (a prefix of other keys), every
// key under it — matching FileStorage's os.RemoveAll semantics, including returning
// nil (not an error) when the key does not exist.
func (s *RedisStorage) Delete(ctx context.Context, key string) error {
	members, err := s.members(ctx)
	if err != nil {
		return fmt.Errorf("server: certmagic redis delete %q: %w", key, err)
	}
	prefix := key + "/"
	var victims []string
	for _, m := range members {
		if m == key || strings.HasPrefix(m, prefix) {
			victims = append(victims, m)
		}
	}
	if len(victims) == 0 {
		return nil // RemoveAll of a missing path is not an error.
	}
	pipe := s.rdb.TxPipeline()
	for _, v := range victims {
		pipe.Del(ctx, s.fileKey(v))
		pipe.SRem(ctx, certmagicIndexKey, v)
	}
	if _, err := pipe.Exec(ctx); err != nil {
		return fmt.Errorf("server: certmagic redis delete %q: %w", key, err)
	}
	return nil
}

// Exists reports whether key exists as a file or as a directory (a prefix of other
// keys). It returns false on any error, matching FileStorage.
func (s *RedisStorage) Exists(ctx context.Context, key string) bool {
	n, err := s.rdb.Exists(ctx, s.fileKey(key)).Result()
	if err != nil {
		return false
	}
	if n > 0 {
		return true
	}
	ok, err := s.hasChildren(ctx, key)
	return err == nil && ok
}

// List returns the keys under prefix. Non-recursive (the only mode certmagic uses)
// returns the immediate children — files and subdirectories one level down — as
// full keys; recursive returns every descendant file key. A missing directory
// returns fs.ErrNotExist, matching FileStorage walking a non-existent path.
func (s *RedisStorage) List(ctx context.Context, prefix string, recursive bool) ([]string, error) {
	members, err := s.members(ctx)
	if err != nil {
		return nil, fmt.Errorf("server: certmagic redis list %q: %w", prefix, err)
	}
	prefix = strings.TrimSuffix(prefix, "/")
	var childPrefix string
	if prefix != "" {
		childPrefix = prefix + "/"
	}

	seen := make(map[string]struct{})
	var out []string
	prefixIsFile := false
	for _, m := range members {
		if m == prefix {
			prefixIsFile = true
			continue
		}
		if childPrefix != "" && !strings.HasPrefix(m, childPrefix) {
			continue
		}
		rest := m[len(childPrefix):]
		if rest == "" {
			continue
		}
		entry := m // recursive: the full descendant key
		if !recursive {
			// immediate child = first path component after the prefix
			if i := strings.IndexByte(rest, '/'); i >= 0 {
				entry = childPrefix + rest[:i]
			}
		}
		if _, ok := seen[entry]; !ok {
			seen[entry] = struct{}{}
			out = append(out, entry)
		}
	}
	if len(out) == 0 {
		if prefixIsFile {
			return []string{}, nil // listing a file yields no children, no error
		}
		return nil, fs.ErrNotExist
	}
	sort.Strings(out)
	return out, nil
}

// Stat returns metadata about key. Files report Size/Modified and IsTerminal=true;
// directories report IsTerminal=false; a missing key returns fs.ErrNotExist.
func (s *RedisStorage) Stat(ctx context.Context, key string) (certmagic.KeyInfo, error) {
	res, err := s.rdb.HMGet(ctx, s.fileKey(key), certmagicFieldSize, certmagicFieldModNs).Result()
	if err != nil {
		return certmagic.KeyInfo{}, fmt.Errorf("server: certmagic redis stat %q: %w", key, err)
	}
	if res[0] != nil {
		size, _ := toInt64(res[0])
		modNs, _ := toInt64(res[1])
		return certmagic.KeyInfo{
			Key:        key,
			Modified:   time.Unix(0, modNs),
			Size:       size,
			IsTerminal: true,
		}, nil
	}
	ok, err := s.hasChildren(ctx, key)
	if err != nil {
		return certmagic.KeyInfo{}, fmt.Errorf("server: certmagic redis stat %q: %w", key, err)
	}
	if ok {
		return certmagic.KeyInfo{Key: key, IsTerminal: false}, nil
	}
	return certmagic.KeyInfo{}, fs.ErrNotExist
}

// Lock acquires a cluster-wide named lock, blocking until it is obtained or ctx is
// canceled. It is the mechanism that stops two edges from issuing the same
// certificate at once: acquisition is an atomic SET NX PX, and the lease is
// auto-expiring (so a dead holder can't deadlock the cluster) yet refreshed while
// held (so a slow issuance doesn't lose it).
func (s *RedisStorage) Lock(ctx context.Context, name string) error {
	key := certmagicLockPrefix + name
	token, err := randomToken()
	if err != nil {
		return fmt.Errorf("server: certmagic redis lock %q: %w", name, err)
	}
	for {
		ok, err := s.rdb.SetNX(ctx, key, token, s.lockTTL).Result()
		if err != nil {
			return fmt.Errorf("server: certmagic redis lock %q: %w", name, err)
		}
		if ok {
			lk := &heldLock{token: token, stop: make(chan struct{}), done: make(chan struct{})}
			s.mu.Lock()
			s.locks[name] = lk
			s.mu.Unlock()
			go s.refreshLock(key, lk) // #nosec G118 -- deliberately decoupled from ctx: the lock must outlive this Lock() call and stay held until Unlock(); its lifecycle is lk.stop/lk.done, not the acquiring request's context
			return nil
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(s.pollWait):
		}
	}
}

// Unlock releases a lock previously obtained by Lock. It stops the refresher and
// deletes the key only if we still own it.
func (s *RedisStorage) Unlock(ctx context.Context, name string) error {
	s.mu.Lock()
	lk := s.locks[name]
	delete(s.locks, name)
	s.mu.Unlock()
	if lk == nil {
		return fmt.Errorf("server: certmagic redis unlock %q: not held by this instance", name)
	}
	close(lk.stop)
	<-lk.done // ensure the refresher can't extend the lease after we release it
	if err := releaseScript.Run(ctx, s.rdb, []string{certmagicLockPrefix + name}, lk.token).Err(); err != nil {
		return fmt.Errorf("server: certmagic redis unlock %q: %w", name, err)
	}
	return nil
}

// Close stops all lock refreshers and closes the Redis client. Wired into the
// edge's drain via io.Closer.
func (s *RedisStorage) Close() error {
	s.mu.Lock()
	held := make([]*heldLock, 0, len(s.locks))
	for name, lk := range s.locks {
		held = append(held, lk)
		delete(s.locks, name)
	}
	s.mu.Unlock()
	for _, lk := range held {
		close(lk.stop)
		<-lk.done
	}
	return s.rdb.Close()
}

// refreshLock extends the lease periodically while the lock is held, so a slow
// certificate order doesn't lose the lock to expiry mid-issuance.
func (s *RedisStorage) refreshLock(key string, lk *heldLock) {
	defer close(lk.done)
	t := time.NewTicker(s.lockTTL / 3)
	defer t.Stop()
	for {
		select {
		case <-lk.stop:
			return
		case <-t.C:
			rctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			err := refreshScript.Run(rctx, s.rdb, []string{key}, lk.token, s.lockTTL.Milliseconds()).Err()
			cancel()
			if err != nil {
				s.log.Warn("certmagic redis: lock lease refresh failed", "key", key, "err", err)
			}
		}
	}
}

// members returns every stored file key. The cert set is small (the wildcard plus a
// handful of custom domains), so SMEMBERS is cheap and avoids scanning a keyspace
// shared with the registry/rate limiter.
func (s *RedisStorage) members(ctx context.Context) ([]string, error) {
	return s.rdb.SMembers(ctx, certmagicIndexKey).Result()
}

// hasChildren reports whether any stored key sits under key as a directory.
func (s *RedisStorage) hasChildren(ctx context.Context, key string) (bool, error) {
	members, err := s.members(ctx)
	if err != nil {
		return false, err
	}
	prefix := key + "/"
	for _, m := range members {
		if strings.HasPrefix(m, prefix) {
			return true, nil
		}
	}
	return false, nil
}

func randomToken() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

func toInt64(v any) (int64, error) {
	s, ok := v.(string)
	if !ok {
		return 0, fmt.Errorf("expected string, got %T", v)
	}
	return strconv.ParseInt(s, 10, 64)
}
