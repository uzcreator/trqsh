// Package auth implements the control-plane authentication: JWT sessions for
// the dashboard, API keys (argon2id-hashed) for agents, an OAuth provider
// abstraction, and a device flow for the CLI/GUI.
package auth

import (
	"context"
	"errors"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/rift/rift/internal/api/store"
)

// Errors surfaced by the auth layer.
var (
	ErrUnauthorized = errors.New("auth: unauthorized")
	ErrKeyRevoked   = errors.New("auth: api key revoked")
)

// Auth bundles the JWT signer and the store used to resolve identities.
type Auth struct {
	store      store.Store
	secret     []byte
	accessTTL  time.Duration
	refreshTTL time.Duration
	devices    *deviceStore
}

// New builds an Auth manager. secret signs JWTs (HS256).
func New(st store.Store, secret string) *Auth {
	return &Auth{
		store:      st,
		secret:     []byte(secret),
		accessTTL:  1 * time.Hour,
		refreshTTL: 30 * 24 * time.Hour,
		devices:    newDeviceStore(),
	}
}

// Claims are the JWT claims carried in a session token.
type Claims struct {
	OrgID string `json:"org"`
	Kind  string `json:"knd"` // "access" | "refresh"
	jwt.RegisteredClaims
}

// Tokens is an access/refresh pair.
type Tokens struct {
	Access    string `json:"access_token"`
	Refresh   string `json:"refresh_token"`
	ExpiresIn int    `json:"expires_in"`
	TokenType string `json:"token_type"`
}

// IssueTokens mints an access+refresh pair for a user's active org.
func (a *Auth) IssueTokens(userID, orgID string) (Tokens, error) {
	access, err := a.sign(userID, orgID, "access", a.accessTTL)
	if err != nil {
		return Tokens{}, err
	}
	refresh, err := a.sign(userID, orgID, "refresh", a.refreshTTL)
	if err != nil {
		return Tokens{}, err
	}
	return Tokens{
		Access:    access,
		Refresh:   refresh,
		ExpiresIn: int(a.accessTTL.Seconds()),
		TokenType: "Bearer",
	}, nil
}

func (a *Auth) sign(userID, orgID, kind string, ttl time.Duration) (string, error) {
	now := time.Now()
	claims := Claims{
		OrgID: orgID,
		Kind:  kind,
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   userID,
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(ttl)),
			Issuer:    "rift-api",
		},
	}
	return jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString(a.secret)
}

// ParseToken verifies a JWT and returns its claims.
func (a *Auth) ParseToken(token string) (*Claims, error) {
	claims := &Claims{}
	_, err := jwt.ParseWithClaims(token, claims, func(t *jwt.Token) (any, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, ErrUnauthorized
		}
		return a.secret, nil
	})
	if err != nil {
		return nil, ErrUnauthorized
	}
	return claims, nil
}

// Refresh exchanges a valid refresh token for a fresh token pair.
func (a *Auth) Refresh(refreshToken string) (Tokens, error) {
	claims, err := a.ParseToken(refreshToken)
	if err != nil {
		return Tokens{}, err
	}
	if claims.Kind != "refresh" {
		return Tokens{}, ErrUnauthorized
	}
	return a.IssueTokens(claims.Subject, claims.OrgID)
}

// Store exposes the backing store for handlers that need it.
func (a *Auth) Store() store.Store { return a.store }

// Devices exposes the device-flow store.
func (a *Auth) Devices() *deviceStore { return a.devices }

// AuthenticateAPIKey validates a raw API key and returns the owning org and key.
func (a *Auth) AuthenticateAPIKey(ctx context.Context, raw string) (store.APIKey, error) {
	prefix, ok := KeyPrefix(raw)
	if !ok {
		return store.APIKey{}, ErrUnauthorized
	}
	k, err := a.store.GetAPIKeyByPrefix(ctx, prefix)
	if err != nil {
		return store.APIKey{}, ErrUnauthorized
	}
	if k.RevokedAt != nil {
		return store.APIKey{}, ErrKeyRevoked
	}
	if !VerifyKey(raw, k.Hash) {
		return store.APIKey{}, ErrUnauthorized
	}
	_ = a.store.TouchAPIKey(ctx, k.ID, time.Now())
	return k, nil
}
