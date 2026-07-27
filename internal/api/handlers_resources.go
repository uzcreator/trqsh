package api

import (
	"net"
	"net/http"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/trqsh-uz/trqsh/internal/api/auth"
	"github.com/trqsh-uz/trqsh/internal/api/store"
)

func orgOf(r *http.Request) string {
	p, _ := auth.PrincipalFrom(r.Context())
	return p.OrgID
}

// --- API keys ---

func (s *Server) handleCreateAPIKey(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Name string `json:"name"`
	}
	if !decode(w, r, &req) {
		return
	}
	if req.Name == "" {
		req.Name = "default"
	}
	gen, err := auth.GenerateAPIKey()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	k, err := s.store.CreateAPIKey(r.Context(), store.APIKey{
		OrgID: orgOf(r), Name: req.Name, Prefix: gen.Prefix, Hash: gen.Hash,
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	// The plaintext key is returned exactly once.
	writeJSON(w, http.StatusCreated, map[string]any{
		"id": k.ID, "name": k.Name, "prefix": k.Prefix, "api_key": gen.Full, "created_at": k.CreatedAt,
	})
}

func (s *Server) handleListAPIKeys(w http.ResponseWriter, r *http.Request) {
	keys, err := s.store.ListAPIKeys(r.Context(), orgOf(r))
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	out := make([]map[string]any, 0, len(keys))
	for _, k := range keys {
		out = append(out, map[string]any{
			"id": k.ID, "name": k.Name, "prefix": k.Prefix,
			"last_used_at": k.LastUsedAt, "revoked_at": k.RevokedAt, "created_at": k.CreatedAt,
		})
	}
	writeJSON(w, http.StatusOK, out)
}

func (s *Server) handleRevokeAPIKey(w http.ResponseWriter, r *http.Request) {
	if err := s.store.RevokeAPIKey(r.Context(), chi.URLParam(r, "id"), orgOf(r), time.Now()); err != nil {
		writeError(w, http.StatusNotFound, "no such key")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// --- Subdomains ---

var subdomainRe = regexp.MustCompile(`^[a-z0-9]([a-z0-9-]{0,61}[a-z0-9])?$`)

func (s *Server) handleReserveSubdomain(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Subdomain string `json:"subdomain"`
	}
	if !decode(w, r, &req) {
		return
	}
	sub := strings.ToLower(strings.TrimSpace(req.Subdomain))
	if !subdomainRe.MatchString(sub) {
		writeError(w, http.StatusBadRequest, "invalid subdomain")
		return
	}
	org, err := s.store.GetOrg(r.Context(), orgOf(r))
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	plan := PlanFor(effectivePlan(org))
	count, _ := s.store.CountSubdomains(r.Context(), org.ID)
	if count >= plan.MaxReservedSubdomains {
		writeError(w, http.StatusForbidden, "reserved subdomain limit reached for your plan")
		return
	}
	res, err := s.store.ReserveSubdomain(r.Context(), store.ReservedSubdomain{OrgID: org.ID, Subdomain: sub})
	if err == store.ErrConflict {
		writeError(w, http.StatusConflict, "subdomain already taken")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, res)
}

func (s *Server) handleListSubdomains(w http.ResponseWriter, r *http.Request) {
	subs, err := s.store.ListSubdomains(r.Context(), orgOf(r))
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, subs)
}

func (s *Server) handleReleaseSubdomain(w http.ResponseWriter, r *http.Request) {
	if err := s.store.ReleaseSubdomain(r.Context(), chi.URLParam(r, "id"), orgOf(r)); err != nil {
		writeError(w, http.StatusNotFound, "no such subdomain")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// --- Custom domains ---

func (s *Server) handleAddDomain(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Domain string `json:"domain"`
	}
	if !decode(w, r, &req) {
		return
	}
	domain := strings.ToLower(strings.TrimSpace(req.Domain))
	if domain == "" || !strings.Contains(domain, ".") {
		writeError(w, http.StatusBadRequest, "invalid domain")
		return
	}
	org, err := s.store.GetOrg(r.Context(), orgOf(r))
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	plan := PlanFor(effectivePlan(org))
	if !plan.AllowCustomDomains {
		writeError(w, http.StatusForbidden, "custom domains require a paid plan")
		return
	}
	count, _ := s.store.CountCustomDomains(r.Context(), org.ID)
	if count >= plan.MaxCustomDomains {
		writeError(w, http.StatusForbidden, "custom domain limit reached for your plan")
		return
	}
	token := "trqsh-verify=" + store.NewID("v")[2:]
	d, err := s.store.AddCustomDomain(r.Context(), store.CustomDomain{OrgID: org.ID, Domain: domain, VerifyToken: token})
	if err == store.ErrConflict {
		writeError(w, http.StatusConflict, "domain already added")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{
		"domain": d,
		"dns_instructions": map[string]string{
			"txt_name":    "_trqsh-challenge." + domain,
			"txt_value":   token,
			"cname_name":  domain,
			"cname_value": s.cfg.BaseDomain,
		},
	})
}

func (s *Server) handleListDomains(w http.ResponseWriter, r *http.Request) {
	domains, err := s.store.ListCustomDomains(r.Context(), orgOf(r))
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, domains)
}

func (s *Server) handleVerifyDomain(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	d, err := s.store.GetCustomDomain(r.Context(), id, orgOf(r))
	if err != nil {
		writeError(w, http.StatusNotFound, "no such domain")
		return
	}
	// Dev shortcut: skip DNS when explicitly forced (local/testing only).
	forced := s.cfg.DevAuth && r.URL.Query().Get("force") == "true"
	if !forced && !dnsHasTXT("_trqsh-challenge."+d.Domain, d.VerifyToken) {
		writeError(w, http.StatusBadRequest, "DNS TXT record not found yet; add it and retry")
		return
	}
	if err := s.store.VerifyCustomDomain(r.Context(), id, orgOf(r), time.Now()); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	verified, _ := s.store.GetCustomDomain(r.Context(), id, orgOf(r))
	writeJSON(w, http.StatusOK, verified)
}

func dnsHasTXT(name, want string) bool {
	records, err := net.LookupTXT(name)
	if err != nil {
		return false
	}
	for _, rec := range records {
		if strings.TrimSpace(rec) == want {
			return true
		}
	}
	return false
}

// --- Usage / orgs / tunnels / plans ---

func (s *Server) handleUsage(w http.ResponseWriter, r *http.Request) {
	org, err := s.store.GetOrg(r.Context(), orgOf(r))
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	since := time.Now().AddDate(0, 0, -30)
	usage, _ := s.store.UsageForOrg(r.Context(), org.ID, since)
	planCode := effectivePlan(org)
	plan := PlanFor(planCode)
	writeJSON(w, http.StatusOK, map[string]any{
		"usage": usage,
		"limits": map[string]any{
			"bandwidth_bytes_mo": plan.MaxBandwidthBytesMo,
			"requests_mo":        plan.MaxRequestsMo,
		},
		"plan":            planCode,
		"plan_expires_at": org.PlanExpiresAt,
	})
}

func (s *Server) handleListOrgs(w http.ResponseWriter, r *http.Request) {
	p, _ := auth.PrincipalFrom(r.Context())
	if p.UserID == "" {
		// API-key principal: return just its org.
		org, err := s.store.GetOrg(r.Context(), p.OrgID)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, []store.Org{org})
		return
	}
	orgs, err := s.store.OrgsForUser(r.Context(), p.UserID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, orgs)
}

func (s *Server) handleListTunnels(w http.ResponseWriter, r *http.Request) {
	// TODO(part-06): the edge should publish an org-scoped active-tunnel index in
	// Redis; until then, live tunnels are read directly by the dashboard/edge.
	writeJSON(w, http.StatusOK, []any{})
}

// handleMe returns the caller's profile, effective plan (+ expiry), and current
// usage in one call. Unlike /account it also works for an API-key principal — the
// desktop app authenticates every cloud call with the stored agent key — in which
// case the profile shown is the org's owner. This is the single endpoint the
// desktop reads to render its Account panel (plan, expiry, usage).
func (s *Server) handleMe(w http.ResponseWriter, r *http.Request) {
	p, _ := auth.PrincipalFrom(r.Context())
	org, err := s.store.GetOrg(r.Context(), p.OrgID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	// Resolve the profile user: the JWT subject, or (API key) the org owner.
	userID := p.UserID
	if userID == "" {
		if members, merr := s.store.ListOrgMembers(r.Context(), org.ID); merr == nil {
			userID = ownerUserID(members)
		}
	}
	var u store.User
	if userID != "" {
		u, _ = s.store.GetUser(r.Context(), userID)
	}

	planCode := effectivePlan(org)
	plan := PlanFor(planCode)
	since := time.Now().AddDate(0, 0, -30)
	usage, _ := s.store.UsageForOrg(r.Context(), org.ID, since)
	// Reflect the effective plan on the org we return, so a client never sees a
	// still-"pro" org whose grant has lapsed.
	org.Plan = planCode

	writeJSON(w, http.StatusOK, map[string]any{
		"user":            u,
		"org":             org,
		"plan":            planCode,
		"plan_expires_at": org.PlanExpiresAt,
		"via_api_key":     p.ViaAPIKey,
		"usage":           usage,
		"limits": map[string]any{
			"bandwidth_bytes_mo":      plan.MaxBandwidthBytesMo,
			"requests_mo":             plan.MaxRequestsMo,
			"max_concurrent_tunnels":  plan.MaxConcurrentTunnels,
			"max_reserved_subdomains": plan.MaxReservedSubdomains,
			"max_custom_domains":      plan.MaxCustomDomains,
		},
	})
}

// ownerUserID picks the org's owner (falling back to the first member) so an
// API-key principal can still be shown a human profile.
func ownerUserID(members []store.OrgMember) string {
	for _, m := range members {
		if m.Role == "owner" {
			return m.UserID
		}
	}
	if len(members) > 0 {
		return members[0].UserID
	}
	return ""
}

func (s *Server) handleListPlans(w http.ResponseWriter, r *http.Request) {
	plans := make([]Plan, 0, len(Catalog))
	for _, p := range Catalog {
		plans = append(plans, p)
	}
	sort.Slice(plans, func(i, j int) bool { return plans[i].MaxConcurrentTunnels < plans[j].MaxConcurrentTunnels })
	writeJSON(w, http.StatusOK, plans)
}
