package api

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/rift/rift/internal/api/auth"
	"github.com/rift/rift/internal/api/store"
)

type signupRequest struct {
	Email string `json:"email"`
	Name  string `json:"name"`
}

type authResponse struct {
	User   store.User  `json:"user"`
	Org    store.Org   `json:"org"`
	Tokens auth.Tokens `json:"tokens"`
}

func (s *Server) handleSignup(w http.ResponseWriter, r *http.Request) {
	if !s.cfg.DevAuth {
		writeError(w, http.StatusForbidden, "password-less signup is disabled; use OAuth")
		return
	}
	var req signupRequest
	if !decode(w, r, &req) {
		return
	}
	if !strings.Contains(req.Email, "@") {
		writeError(w, http.StatusBadRequest, "valid email required")
		return
	}
	if _, err := s.store.GetUserByEmail(r.Context(), req.Email); err == nil {
		writeError(w, http.StatusConflict, "user already exists")
		return
	}
	u, org, err := s.provisionUser(r.Context(), req.Email, req.Name, "email")
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	tokens, err := s.auth.IssueTokens(u.ID, org.ID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, authResponse{User: u, Org: org, Tokens: tokens})
}

func (s *Server) handleLogin(w http.ResponseWriter, r *http.Request) {
	if !s.cfg.DevAuth {
		writeError(w, http.StatusForbidden, "password-less login is disabled; use OAuth")
		return
	}
	var req signupRequest
	if !decode(w, r, &req) {
		return
	}
	u, err := s.store.GetUserByEmail(r.Context(), req.Email)
	if err != nil {
		writeError(w, http.StatusUnauthorized, "no such user")
		return
	}
	org, err := s.firstOrg(r.Context(), u.ID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	tokens, err := s.auth.IssueTokens(u.ID, org.ID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, authResponse{User: u, Org: org, Tokens: tokens})
}

func (s *Server) handleRefresh(w http.ResponseWriter, r *http.Request) {
	var req struct {
		RefreshToken string `json:"refresh_token"`
	}
	if !decode(w, r, &req) {
		return
	}
	tokens, err := s.auth.Refresh(req.RefreshToken)
	if err != nil {
		writeError(w, http.StatusUnauthorized, "invalid refresh token")
		return
	}
	writeJSON(w, http.StatusOK, tokens)
}

func (s *Server) handleAccount(w http.ResponseWriter, r *http.Request) {
	p, _ := auth.PrincipalFrom(r.Context())
	u, err := s.store.GetUser(r.Context(), p.UserID)
	if err != nil {
		writeError(w, http.StatusNotFound, "user not found")
		return
	}
	orgs, _ := s.store.OrgsForUser(r.Context(), p.UserID)
	writeJSON(w, http.StatusOK, map[string]any{"user": u, "orgs": orgs, "active_org": p.OrgID})
}

// --- OAuth ---

func (s *Server) handleOAuthStart(w http.ResponseWriter, r *http.Request) {
	name := chi.URLParam(r, "provider")
	p, ok := s.providers[name]
	if !ok || !p.Configured() {
		writeError(w, http.StatusNotFound, "provider not configured")
		return
	}
	state := s.newState()
	http.Redirect(w, r, p.AuthCodeURL(state), http.StatusFound)
}

func (s *Server) handleOAuthCallback(w http.ResponseWriter, r *http.Request) {
	name := chi.URLParam(r, "provider")
	p, ok := s.providers[name]
	if !ok {
		writeError(w, http.StatusNotFound, "provider not configured")
		return
	}
	if !s.checkState(r.URL.Query().Get("state")) {
		writeError(w, http.StatusBadRequest, "invalid state")
		return
	}
	prof, err := p.Exchange(r.Context(), r.URL.Query().Get("code"))
	if err != nil {
		writeError(w, http.StatusBadGateway, err.Error())
		return
	}
	u, org, err := s.provisionUser(r.Context(), prof.Email, prof.Name, prof.Provider)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	tokens, err := s.auth.IssueTokens(u.ID, org.ID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, authResponse{User: u, Org: org, Tokens: tokens})
}

// --- Device flow (CLI/GUI login) ---

func (s *Server) handleDeviceCode(w http.ResponseWriter, r *http.Request) {
	req := s.auth.Devices().Create()
	writeJSON(w, http.StatusOK, map[string]any{
		"device_code":               req.DeviceCode,
		"user_code":                 req.UserCode,
		"verification_uri":          s.cfg.PublicURL + "/device",
		"verification_uri_complete": s.cfg.PublicURL + "/device?code=" + req.UserCode,
		"interval":                  2,
		"expires_in":                600,
	})
}

func (s *Server) handleDeviceApprove(w http.ResponseWriter, r *http.Request) {
	p, _ := auth.PrincipalFrom(r.Context())
	var req struct {
		UserCode string `json:"user_code"`
	}
	if !decode(w, r, &req) {
		return
	}
	// Mint an API key for the org so the CLI can authenticate as an agent.
	gen, err := auth.GenerateAPIKey()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if _, err := s.store.CreateAPIKey(r.Context(), store.APIKey{
		OrgID: p.OrgID, Name: "cli-device", Prefix: gen.Prefix, Hash: gen.Hash,
	}); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if err := s.auth.Devices().Approve(req.UserCode, p.OrgID, gen.Full); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "approved"})
}

func (s *Server) handleDeviceToken(w http.ResponseWriter, r *http.Request) {
	var req struct {
		DeviceCode string `json:"device_code"`
	}
	if !decode(w, r, &req) {
		return
	}
	key, err := s.auth.Devices().Poll(req.DeviceCode)
	if err != nil {
		// RFC 8628-style pending/expired errors use 400 with an error code.
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"api_key": key})
}

// --- helpers ---

func (s *Server) provisionUser(ctx context.Context, email, name, provider string) (store.User, store.Org, error) {
	u, err := s.store.GetUserByEmail(ctx, email)
	if errors.Is(err, store.ErrNotFound) {
		if name == "" {
			name = strings.Split(email, "@")[0]
		}
		u, err = s.store.CreateUser(ctx, store.User{Email: email, Name: name, OAuthProvider: provider})
		if err != nil {
			return store.User{}, store.Org{}, err
		}
		org, err := s.store.CreateOrg(ctx, store.Org{Name: name + "'s org", Plan: PlanFree})
		if err != nil {
			return store.User{}, store.Org{}, err
		}
		if err := s.store.AddOrgMember(ctx, store.OrgMember{OrgID: org.ID, UserID: u.ID, Role: "owner"}); err != nil {
			return store.User{}, store.Org{}, err
		}
		return u, org, nil
	}
	if err != nil {
		return store.User{}, store.Org{}, err
	}
	org, err := s.firstOrg(ctx, u.ID)
	return u, org, err
}

func (s *Server) firstOrg(ctx context.Context, userID string) (store.Org, error) {
	orgs, err := s.store.OrgsForUser(ctx, userID)
	if err != nil {
		return store.Org{}, err
	}
	if len(orgs) == 0 {
		return store.Org{}, errors.New("user has no org")
	}
	return orgs[0], nil
}

func (s *Server) newState() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	state := hex.EncodeToString(b)
	s.stateMu.Lock()
	s.states[state] = time.Now().Add(10 * time.Minute)
	s.stateMu.Unlock()
	return state
}

func (s *Server) checkState(state string) bool {
	if state == "" {
		return false
	}
	s.stateMu.Lock()
	defer s.stateMu.Unlock()
	exp, ok := s.states[state]
	delete(s.states, state)
	return ok && time.Now().Before(exp)
}
