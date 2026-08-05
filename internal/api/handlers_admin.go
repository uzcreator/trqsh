package api

import (
	"crypto/hmac"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/trqsh-uz/trqsh/internal/api/store"
)

// This file powers approve.<base> — a tiny, single-admin console for granting
// subscriptions manually (there is no Stripe). The edge routes approve.<base> to
// the API, which serves the page and the /v1/admin/* endpoints on the same origin
// so the admin session cookie flows without CORS. Access is gated by a
// username/password (TRQSH_ADMIN_USER / TRQSH_ADMIN_PASSWORD); empty creds disable
// the whole surface.

const adminCookie = "trqsh_admin"
const adminTTL = 12 * time.Hour

// adminEnabled reports whether admin credentials are configured.
func (s *Server) adminEnabled() bool {
	return s.cfg.AdminUser != "" && s.cfg.AdminPassword != ""
}

// isAdminHost reports whether the request arrived on approve.<base> (the edge
// preserves the original Host and sets X-Forwarded-Host when proxying). The admin
// console answers only there, keeping it off the bare api.<base>.
func (s *Server) isAdminHost(r *http.Request) bool {
	host := r.Host
	if xf := r.Header.Get("X-Forwarded-Host"); xf != "" {
		host = xf
	}
	return strings.HasPrefix(strings.ToLower(host), "approve.")
}

// handleRoot serves the admin console at the root of approve.<base> so the bare
// hostname works; every other host gets a plain 404 (the API is not browsable).
func (s *Server) handleRoot(w http.ResponseWriter, r *http.Request) {
	if s.adminEnabled() && s.isAdminHost(r) {
		s.handleAdminPage(w, r)
		return
	}
	http.NotFound(w, r)
}

// signAdmin issues an opaque, HMAC-signed session value "<exp>.<mac>" bound to the
// JWT secret. It carries no identity beyond "is the admin", which is all we need.
func (s *Server) signAdmin(exp time.Time) string {
	payload := strconv.FormatInt(exp.Unix(), 10)
	mac := hmac.New(sha256.New, []byte(s.cfg.JWTSecret))
	mac.Write([]byte("admin|" + payload))
	return payload + "." + hex.EncodeToString(mac.Sum(nil))
}

func (s *Server) verifyAdmin(value string) bool {
	i := strings.LastIndexByte(value, '.')
	if i <= 0 {
		return false
	}
	payload, sig := value[:i], value[i+1:]
	mac := hmac.New(sha256.New, []byte(s.cfg.JWTSecret))
	mac.Write([]byte("admin|" + payload))
	want := hex.EncodeToString(mac.Sum(nil))
	if subtle.ConstantTimeCompare([]byte(sig), []byte(want)) != 1 {
		return false
	}
	exp, err := strconv.ParseInt(payload, 10, 64)
	return err == nil && time.Now().Before(time.Unix(exp, 0))
}

// requireAdmin gates the /v1/admin/* mutations on a valid admin session.
func (s *Server) requireAdmin(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !s.adminEnabled() {
			writeError(w, http.StatusNotFound, "admin console is disabled")
			return
		}
		c, err := r.Cookie(adminCookie)
		if err != nil || !s.verifyAdmin(c.Value) {
			writeError(w, http.StatusUnauthorized, "not signed in")
			return
		}
		next.ServeHTTP(w, r)
	})
}

func (s *Server) handleAdminLogin(w http.ResponseWriter, r *http.Request) {
	if !s.adminEnabled() {
		writeError(w, http.StatusNotFound, "admin console is disabled")
		return
	}
	var req struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if !decode(w, r, &req) {
		return
	}
	userOK := subtle.ConstantTimeCompare([]byte(req.Username), []byte(s.cfg.AdminUser)) == 1
	passOK := subtle.ConstantTimeCompare([]byte(req.Password), []byte(s.cfg.AdminPassword)) == 1
	if !userOK || !passOK {
		writeError(w, http.StatusUnauthorized, "invalid credentials")
		return
	}
	// #nosec G124 -- Secure is set below (s.cfg.IsProduction()); gosec's cookie check only recognizes a literal `true`, not a conditional
	http.SetCookie(w, &http.Cookie{
		Name: adminCookie, Value: s.signAdmin(time.Now().Add(adminTTL)), Path: "/",
		HttpOnly: true, Secure: s.cfg.IsProduction(), SameSite: http.SameSiteLaxMode,
		MaxAge: int(adminTTL.Seconds()),
	})
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (s *Server) handleAdminLogout(w http.ResponseWriter, r *http.Request) {
	// #nosec G124 -- Secure is set below (s.cfg.IsProduction()); gosec's cookie check only recognizes a literal `true`, not a conditional
	http.SetCookie(w, &http.Cookie{
		Name: adminCookie, Value: "", Path: "/",
		HttpOnly: true, Secure: s.cfg.IsProduction(), SameSite: http.SameSiteLaxMode, MaxAge: -1,
	})
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// adminOrgFor resolves the (first) org owned by the account with this email.
func (s *Server) adminOrgFor(r *http.Request, email string) (store.User, store.Org, error) {
	u, err := s.store.GetUserByEmail(r.Context(), strings.TrimSpace(email))
	if err != nil {
		return store.User{}, store.Org{}, err
	}
	orgs, err := s.store.OrgsForUser(r.Context(), u.ID)
	if err != nil || len(orgs) == 0 {
		return u, store.Org{}, errors.New("account has no org")
	}
	return u, orgs[0], nil
}

func adminOrgView(u store.User, o store.Org) map[string]any {
	return map[string]any{
		"email":           u.Email,
		"name":            u.Name,
		"org_id":          o.ID,
		"plan":            effectivePlan(o),
		"stored_plan":     o.Plan,
		"plan_expires_at": o.PlanExpiresAt,
	}
}

// handleAdminSession is a cheap "am I signed in?" probe for the page (gated by
// requireAdmin, so reaching it at all means yes).
func (s *Server) handleAdminSession(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func (s *Server) handleAdminLookup(w http.ResponseWriter, r *http.Request) {
	u, o, err := s.adminOrgFor(r, r.URL.Query().Get("email"))
	if err != nil {
		writeError(w, http.StatusNotFound, "no account with that email")
		return
	}
	writeJSON(w, http.StatusOK, adminOrgView(u, o))
}

func (s *Server) handleAdminGrant(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Email  string `json:"email"`
		Plan   string `json:"plan"`
		Months int    `json:"months"`
	}
	if !decode(w, r, &req) {
		return
	}
	plan := strings.ToLower(strings.TrimSpace(req.Plan))
	if plan != PlanPro && plan != PlanTeam && plan != PlanFree {
		writeError(w, http.StatusBadRequest, "plan must be pro, team, or free")
		return
	}
	if req.Months < 1 || req.Months > 60 {
		writeError(w, http.StatusBadRequest, "months must be between 1 and 60")
		return
	}
	u, o, err := s.adminOrgFor(r, req.Email)
	if err != nil {
		writeError(w, http.StatusNotFound, "no account with that email")
		return
	}
	expires := time.Now().AddDate(0, req.Months, 0)
	if err := s.store.SetOrgPlan(r.Context(), o.ID, plan, &expires); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	s.log.Info("admin granted plan", "email", sanitizeLog(u.Email), "org", o.ID, "plan", plan, "months", req.Months)
	o.Plan, o.PlanExpiresAt = plan, &expires
	writeJSON(w, http.StatusOK, adminOrgView(u, o))
}

func (s *Server) handleAdminRevoke(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Email string `json:"email"`
	}
	if !decode(w, r, &req) {
		return
	}
	u, o, err := s.adminOrgFor(r, req.Email)
	if err != nil {
		writeError(w, http.StatusNotFound, "no account with that email")
		return
	}
	if err := s.store.SetOrgPlan(r.Context(), o.ID, PlanFree, nil); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	s.log.Info("admin revoked plan", "email", u.Email, "org", o.ID)
	o.Plan, o.PlanExpiresAt = PlanFree, nil
	writeJSON(w, http.StatusOK, adminOrgView(u, o))
}

func (s *Server) handleAdminPage(w http.ResponseWriter, r *http.Request) {
	// Hidden when disabled, and (in production) only served on approve.<base> so
	// the console never appears on the bare api.<base>.
	if !s.adminEnabled() || (s.cfg.IsProduction() && !s.isAdminHost(r)) {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("X-Frame-Options", "DENY")
	_, _ = w.Write([]byte(adminPageHTML))
}
