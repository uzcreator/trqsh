package auth

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
)

// Principal is the authenticated caller attached to a request context.
type Principal struct {
	UserID    string
	OrgID     string
	ViaAPIKey bool
	APIKeyID  string
}

type ctxKey int

const principalKey ctxKey = iota

// WithPrincipal stores a principal in the context.
func WithPrincipal(ctx context.Context, p Principal) context.Context {
	return context.WithValue(ctx, principalKey, p)
}

// PrincipalFrom retrieves the principal from the context.
func PrincipalFrom(ctx context.Context) (Principal, bool) {
	p, ok := ctx.Value(principalKey).(Principal)
	return p, ok
}

// Middleware authenticates a request via a Bearer JWT (dashboard) or an API key
// (programmatic), rejecting anything else with 401.
func (a *Auth) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		tok := bearerToken(r)
		if tok == "" {
			writeUnauthorized(w)
			return
		}
		if strings.HasPrefix(tok, "rk_") {
			k, err := a.AuthenticateAPIKey(r.Context(), tok)
			if err != nil {
				writeUnauthorized(w)
				return
			}
			ctx := WithPrincipal(r.Context(), Principal{OrgID: k.OrgID, ViaAPIKey: true, APIKeyID: k.ID})
			next.ServeHTTP(w, r.WithContext(ctx))
			return
		}
		claims, err := a.ParseToken(tok)
		if err != nil || claims.Kind != "access" {
			writeUnauthorized(w)
			return
		}
		ctx := WithPrincipal(r.Context(), Principal{UserID: claims.Subject, OrgID: claims.OrgID})
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func bearerToken(r *http.Request) string {
	h := r.Header.Get("Authorization")
	if h == "" {
		return ""
	}
	if strings.HasPrefix(strings.ToLower(h), "bearer ") {
		return strings.TrimSpace(h[len("bearer "):])
	}
	return strings.TrimSpace(h)
}

func writeUnauthorized(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusUnauthorized)
	_ = json.NewEncoder(w).Encode(map[string]string{"error": "unauthorized"})
}
