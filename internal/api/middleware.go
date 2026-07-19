package api

import (
	"math"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"
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

// rateLimiter is a per-key token-bucket limiter. Buckets are created lazily and
// reaped when idle, bounding memory under a flood of distinct source IPs.
type rateLimiter struct {
	mu       sync.Mutex
	buckets  map[string]*tokenBucket
	rps      float64
	burst    float64
	trustXFF bool
}

type tokenBucket struct {
	tokens float64
	last   time.Time
}

// newRateLimiter builds a limiter allowing `rps` requests/second per key with a
// `burst` allowance. trustXFF honors X-Forwarded-For (only enable behind a
// trusted proxy). It starts a background reaper for the process lifetime.
func newRateLimiter(rps, burst float64, trustXFF bool) *rateLimiter {
	rl := &rateLimiter{
		buckets:  make(map[string]*tokenBucket),
		rps:      rps,
		burst:    burst,
		trustXFF: trustXFF,
	}
	go rl.reap()
	return rl
}

func (rl *rateLimiter) allow(key string) bool {
	rl.mu.Lock()
	defer rl.mu.Unlock()
	now := time.Now()
	b, ok := rl.buckets[key]
	if !ok {
		rl.buckets[key] = &tokenBucket{tokens: rl.burst - 1, last: now}
		return true
	}
	b.tokens = math.Min(rl.burst, b.tokens+now.Sub(b.last).Seconds()*rl.rps)
	b.last = now
	if b.tokens < 1 {
		return false
	}
	b.tokens--
	return true
}

func (rl *rateLimiter) reap() {
	t := time.NewTicker(10 * time.Minute)
	defer t.Stop()
	for range t.C {
		cutoff := time.Now().Add(-10 * time.Minute)
		rl.mu.Lock()
		for k, b := range rl.buckets {
			if b.last.Before(cutoff) {
				delete(rl.buckets, k)
			}
		}
		rl.mu.Unlock()
	}
}

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
// when trustXFF is set (RIFT_TRUST_PROXY), since otherwise a client can spoof
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
