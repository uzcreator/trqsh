package api

import (
	"net"
	"net/http"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/rift/rift/internal/api/auth"
	"github.com/rift/rift/internal/api/store"
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
	plan := PlanFor(org.Plan)
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
	plan := PlanFor(org.Plan)
	if !plan.AllowCustomDomains {
		writeError(w, http.StatusForbidden, "custom domains require a paid plan")
		return
	}
	count, _ := s.store.CountCustomDomains(r.Context(), org.ID)
	if count >= plan.MaxCustomDomains {
		writeError(w, http.StatusForbidden, "custom domain limit reached for your plan")
		return
	}
	token := "rift-verify=" + store.NewID("v")[2:]
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
			"txt_name":    "_rift-challenge." + domain,
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
	if !forced && !dnsHasTXT("_rift-challenge."+d.Domain, d.VerifyToken) {
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
	plan := PlanFor(org.Plan)
	writeJSON(w, http.StatusOK, map[string]any{
		"usage": usage,
		"limits": map[string]any{
			"bandwidth_bytes_mo": plan.MaxBandwidthBytesMo,
			"requests_mo":        plan.MaxRequestsMo,
		},
		"plan": org.Plan,
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

func (s *Server) handleListPlans(w http.ResponseWriter, r *http.Request) {
	plans := make([]Plan, 0, len(Catalog))
	for _, p := range Catalog {
		plans = append(plans, p)
	}
	sort.Slice(plans, func(i, j int) bool { return plans[i].MaxConcurrentTunnels < plans[j].MaxConcurrentTunnels })
	writeJSON(w, http.StatusOK, plans)
}
