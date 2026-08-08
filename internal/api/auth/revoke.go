package auth

import (
	"sync"
	"time"
)

// Revoker records refresh-token IDs (jti) that must no longer be honored, so a
// logout (or an admin session kill) actually invalidates a long-lived refresh
// token server-side instead of trusting the client to drop it.
//
// It is consulted only on the refresh path — access tokens stay short-lived
// (1h) and are NOT looked up per request, keeping the authentication hot path
// free of a store round-trip. Revoking a session therefore stops it from
// minting new access tokens; any already-issued access token still expires on
// its own within the hour.
//
// Implementations must be safe for concurrent use. The default is process-local
// (memRevoker); a Redis-backed one (internal/api) is injected via SetRevoker so
// a logout on one API replica is seen by the replica a later refresh lands on —
// the same seam DeviceStore and the OAuth state store already use.
type Revoker interface {
	// Revoke marks jti revoked for ttl (its remaining lifetime); the entry may be
	// dropped after ttl since the token itself has expired by then anyway.
	Revoke(jti string, ttl time.Duration)
	// IsRevoked reports whether jti is currently revoked.
	IsRevoked(jti string) bool
}

var _ Revoker = (*memRevoker)(nil)

// memRevoker is the process-local Revoker: a map of jti -> expiry guarded by one
// mutex, with a background reaper dropping lapsed entries so a stream of logouts
// can't grow it without bound. Zero-config default; replaced by a Redis-backed
// variant in multi-replica deployments (Auth.SetRevoker).
type memRevoker struct {
	mu      sync.Mutex
	revoked map[string]time.Time
}

func newMemRevoker() *memRevoker {
	m := &memRevoker{revoked: make(map[string]time.Time)}
	go m.reap()
	return m
}

func (m *memRevoker) Revoke(jti string, ttl time.Duration) {
	if jti == "" || ttl <= 0 {
		return
	}
	m.mu.Lock()
	m.revoked[jti] = time.Now().Add(ttl)
	m.mu.Unlock()
}

func (m *memRevoker) IsRevoked(jti string) bool {
	if jti == "" {
		return false
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	exp, ok := m.revoked[jti]
	if !ok {
		return false
	}
	if time.Now().After(exp) {
		delete(m.revoked, jti)
		return false
	}
	return true
}

func (m *memRevoker) reap() {
	t := time.NewTicker(10 * time.Minute)
	defer t.Stop()
	for range t.C {
		now := time.Now()
		m.mu.Lock()
		for jti, exp := range m.revoked {
			if now.After(exp) {
				delete(m.revoked, jti)
			}
		}
		m.mu.Unlock()
	}
}
