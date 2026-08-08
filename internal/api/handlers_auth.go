package api

import (
	"context"
	"crypto/subtle"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/trqsh-uz/trqsh/internal/api/auth"
	"github.com/trqsh-uz/trqsh/internal/api/store"
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
	u, org, err := s.provisionUser(r.Context(), req.Email, req.Name, "", "email")
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
	// Preserve a safe post-login destination (e.g. /device?code=… for desktop
	// sign-in) so the callback can return the user where they started.
	if next := safeNextPath(r.URL.Query().Get("next")); next != "" {
		s.setNextCookie(w, next)
	}
	state := s.newState()
	s.setStateCookie(w, state)
	http.Redirect(w, r, p.AuthCodeURL(state), http.StatusFound)
}

// handleLogout clears the shared session cookies so a different account can sign
// in, and revokes the refresh token server-side so it can't mint new access
// tokens even if it was captured before logout. The access token is short-lived
// (1h) and expires on its own.
func (s *Server) handleLogout(w http.ResponseWriter, r *http.Request) {
	if rt := refreshTokenFromRequest(r); rt != "" {
		s.auth.RevokeRefresh(rt)
	}
	s.clearSessionCookies(w)
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// refreshTokenFromRequest pulls the refresh token from the shared cookie
// (browser) or, failing that, a JSON {refresh_token} body (programmatic clients).
func refreshTokenFromRequest(r *http.Request) string {
	if c, err := r.Cookie("trqsh_refresh"); err == nil && c.Value != "" {
		return c.Value
	}
	if r.Body == nil {
		return ""
	}
	var body struct {
		RefreshToken string `json:"refresh_token"`
	}
	_ = json.NewDecoder(io.LimitReader(r.Body, 1<<16)).Decode(&body)
	return body.RefreshToken
}

func (s *Server) handleOAuthCallback(w http.ResponseWriter, r *http.Request) {
	name := chi.URLParam(r, "provider")
	p, ok := s.providers[name]
	if !ok {
		writeError(w, http.StatusNotFound, "provider not configured")
		return
	}
	// CSRF: the returned state must match BOTH the server-issued single-use state
	// and the state cookie set on this browser when the flow started (double-submit).
	qstate := r.URL.Query().Get("state")
	sc, _ := r.Cookie("trqsh_oauth_state")
	s.clearStateCookie(w)
	if sc == nil || subtle.ConstantTimeCompare([]byte(sc.Value), []byte(qstate)) != 1 || !s.checkState(qstate) {
		writeError(w, http.StatusBadRequest, "invalid state")
		return
	}
	prof, err := p.Exchange(r.Context(), r.URL.Query().Get("code"))
	if err != nil {
		writeError(w, http.StatusBadGateway, err.Error())
		return
	}
	u, org, err := s.provisionUser(r.Context(), prof.Email, prof.Name, prof.AvatarURL, prof.Provider)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	tokens, err := s.auth.IssueTokens(u.ID, org.ID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	// Browser flow: set the session as cookies shared across *.<base> and send the
	// user to the dashboard — or back to a preserved destination such as the
	// device-approval page. (Programmatic clients use the JSON login endpoints.)
	dest := s.appBase() + "/"
	if nc, _ := r.Cookie("trqsh_oauth_next"); nc != nil {
		if n := safeNextPath(nc.Value); n != "" {
			dest = s.appBase() + n
		}
	}
	s.clearNextCookie(w)
	s.setSessionCookies(w, tokens)
	http.Redirect(w, r, dest, http.StatusFound) // #nosec G710 -- dest is either appBase()+"/" or appBase()+safeNextPath(...), which rejects anything not a same-site absolute path
}

// appBase returns the public dashboard origin (no trailing slash).
func (s *Server) appBase() string {
	if s.cfg.AppURL != "" {
		return strings.TrimRight(s.cfg.AppURL, "/")
	}
	return "https://app." + s.cfg.BaseDomain
}

// safeNextPath accepts only same-site absolute paths (e.g. "/device?code=…"),
// rejecting anything that could redirect off-site.
func safeNextPath(p string) string {
	if p == "" || !strings.HasPrefix(p, "/") || strings.HasPrefix(p, "//") || strings.HasPrefix(p, "/\\") {
		return ""
	}
	return p
}

func (s *Server) setNextCookie(w http.ResponseWriter, next string) {
	// #nosec G124 -- Secure is set below (s.cfg.IsProduction()); gosec's cookie check only recognizes a literal `true`, not a conditional
	http.SetCookie(w, &http.Cookie{
		Name: "trqsh_oauth_next", Value: next, Path: "/v1/auth/oauth",
		HttpOnly: true, Secure: s.cfg.IsProduction(), SameSite: http.SameSiteLaxMode, MaxAge: 600,
	})
}

func (s *Server) clearNextCookie(w http.ResponseWriter) {
	// #nosec G124 -- Secure is set below (s.cfg.IsProduction()); gosec's cookie check only recognizes a literal `true`, not a conditional
	http.SetCookie(w, &http.Cookie{
		Name: "trqsh_oauth_next", Value: "", Path: "/v1/auth/oauth",
		HttpOnly: true, Secure: s.cfg.IsProduction(), SameSite: http.SameSiteLaxMode, MaxAge: -1,
	})
}

// setSessionCookies writes the access + refresh JWTs as cookies scoped to the
// whole base domain, so the dashboard (app.<base>) shares the API's session.
func (s *Server) setSessionCookies(w http.ResponseWriter, t auth.Tokens) {
	domain := "." + s.cfg.BaseDomain
	secure := s.cfg.IsProduction()
	// #nosec G124 -- Secure: secure is set from s.cfg.IsProduction() just above; gosec's cookie check only recognizes a literal `true`, not a variable
	http.SetCookie(w, &http.Cookie{
		Name: "trqsh_access", Value: t.Access, Path: "/", Domain: domain,
		HttpOnly: true, Secure: secure, SameSite: http.SameSiteLaxMode, MaxAge: t.ExpiresIn,
	})
	// #nosec G124 -- Secure: secure is set from s.cfg.IsProduction() above; gosec's cookie check only recognizes a literal `true`, not a variable
	http.SetCookie(w, &http.Cookie{
		Name: "trqsh_refresh", Value: t.Refresh, Path: "/", Domain: domain,
		HttpOnly: true, Secure: secure, SameSite: http.SameSiteLaxMode, MaxAge: 30 * 24 * 60 * 60,
	})
}

// clearSessionCookies expires the shared session cookies (logout).
func (s *Server) clearSessionCookies(w http.ResponseWriter) {
	domain := "." + s.cfg.BaseDomain
	secure := s.cfg.IsProduction()
	// #nosec G124 -- Secure: secure is set from s.cfg.IsProduction() just above; gosec's cookie check only recognizes a literal `true`, not a variable
	for _, name := range []string{"trqsh_access", "trqsh_refresh"} {
		http.SetCookie(w, &http.Cookie{
			Name: name, Value: "", Path: "/", Domain: domain,
			HttpOnly: true, Secure: secure, SameSite: http.SameSiteLaxMode, MaxAge: -1,
		})
	}
}

// setStateCookie binds the OAuth state to this browser (host-only on the API),
// so a stolen or guessed state alone can't complete the flow.
func (s *Server) setStateCookie(w http.ResponseWriter, state string) {
	// #nosec G124 -- Secure is set below (s.cfg.IsProduction()); gosec's cookie check only recognizes a literal `true`, not a conditional
	http.SetCookie(w, &http.Cookie{
		Name: "trqsh_oauth_state", Value: state, Path: "/v1/auth/oauth",
		HttpOnly: true, Secure: s.cfg.IsProduction(), SameSite: http.SameSiteLaxMode, MaxAge: 600,
	})
}

func (s *Server) clearStateCookie(w http.ResponseWriter) {
	// #nosec G124 -- Secure is set below (s.cfg.IsProduction()); gosec's cookie check only recognizes a literal `true`, not a conditional
	http.SetCookie(w, &http.Cookie{
		Name: "trqsh_oauth_state", Value: "", Path: "/v1/auth/oauth",
		HttpOnly: true, Secure: s.cfg.IsProduction(), SameSite: http.SameSiteLaxMode, MaxAge: -1,
	})
}

// --- Device flow (CLI/GUI login) ---

func (s *Server) handleDeviceCode(w http.ResponseWriter, r *http.Request) {
	req := s.auth.Devices().Create()
	// The approval page lives on the dashboard (app.<base>/device) where the user
	// already has (or can start) an OAuth session. Fall back to the API's own
	// PublicURL in dev when no dashboard origin is configured.
	verifyBase := s.cfg.AppURL
	if verifyBase == "" {
		verifyBase = s.cfg.PublicURL
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"device_code":               req.DeviceCode,
		"user_code":                 req.UserCode,
		"verification_uri":          verifyBase + "/device",
		"verification_uri_complete": verifyBase + "/device?code=" + req.UserCode,
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

func (s *Server) provisionUser(ctx context.Context, email, name, avatar, provider string) (store.User, store.Org, error) {
	u, err := s.store.GetUserByEmail(ctx, email)
	if errors.Is(err, store.ErrNotFound) {
		if name == "" {
			name = strings.Split(email, "@")[0]
		}
		u, err = s.store.CreateUser(ctx, store.User{Email: email, Name: name, AvatarURL: avatar, OAuthProvider: provider})
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
	// Refresh the profile (name/avatar) on each sign-in — keeps the picture current
	// and back-fills accounts created before avatars were stored.
	if (avatar != "" && avatar != u.AvatarURL) || (name != "" && name != u.Name) {
		if uerr := s.store.UpdateUserProfile(ctx, u.ID, name, avatar); uerr == nil {
			if name != "" {
				u.Name = name
			}
			if avatar != "" {
				u.AvatarURL = avatar
			}
		}
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
	return s.oauthState.New()
}

func (s *Server) checkState(state string) bool {
	return s.oauthState.Check(state)
}
