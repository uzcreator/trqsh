package auth

import (
	"testing"

	"github.com/trqsh-uz/trqsh/internal/api/store"
)

const testSecret = "test-secret-at-least-32-chars-long!!"

// A logged-out (revoked) refresh token must stop minting new access tokens,
// even though its signature is still valid — the whole point of server-side
// revocation.
func TestRefreshRevoked(t *testing.T) {
	a := New(store.NewMemStore(), testSecret)
	toks, err := a.IssueTokens("user_1", "org_1")
	if err != nil {
		t.Fatalf("issue: %v", err)
	}
	if _, err := a.Refresh(toks.Refresh); err != nil {
		t.Fatalf("refresh before revoke should work: %v", err)
	}
	a.RevokeRefresh(toks.Refresh)
	if _, err := a.Refresh(toks.Refresh); err == nil {
		t.Fatal("refresh after revoke: want error, got nil")
	}
}

// Revoking is scoped to the exact token: killing one session must not kill
// another, and handing RevokeRefresh an access token is a harmless no-op.
func TestRevokeIsScopedToTheToken(t *testing.T) {
	a := New(store.NewMemStore(), testSecret)
	s1, _ := a.IssueTokens("user_1", "org_1")
	s2, _ := a.IssueTokens("user_1", "org_1")

	c1, err := a.ParseToken(s1.Refresh)
	if err != nil {
		t.Fatalf("parse s1: %v", err)
	}
	c2, _ := a.ParseToken(s2.Refresh)
	if c1.ID == "" || c2.ID == "" {
		t.Fatal("refresh tokens must carry a jti")
	}
	if c1.ID == c2.ID {
		t.Fatal("distinct tokens must have distinct jti")
	}

	// Revoking with an access token does nothing (wrong kind).
	a.RevokeRefresh(s1.Access)
	if _, err := a.Refresh(s1.Refresh); err != nil {
		t.Fatalf("access-token revoke must be a no-op: %v", err)
	}

	// Revoking session 1's refresh token must not affect session 2.
	a.RevokeRefresh(s1.Refresh)
	if _, err := a.Refresh(s1.Refresh); err == nil {
		t.Fatal("s1 refresh should be revoked")
	}
	if _, err := a.Refresh(s2.Refresh); err != nil {
		t.Fatalf("s2 refresh must still work: %v", err)
	}
}

// The fresh tokens minted by a refresh carry their own new jti, so the new
// session is independently revocable (and the old jti's revocation doesn't
// bleed onto it).
func TestRefreshRotatesJTI(t *testing.T) {
	a := New(store.NewMemStore(), testSecret)
	s1, _ := a.IssueTokens("user_1", "org_1")
	s2, err := a.Refresh(s1.Refresh)
	if err != nil {
		t.Fatalf("refresh: %v", err)
	}
	c1, _ := a.ParseToken(s1.Refresh)
	c2, _ := a.ParseToken(s2.Refresh)
	if c1.ID == c2.ID {
		t.Fatal("a refreshed token must get a new jti")
	}
}
