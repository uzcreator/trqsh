package api

import (
	"net/http"
	"time"

	"github.com/trqsh-uz/trqsh/internal/api/store"
)

// This file adds the read-only, whole-deployment views behind the approve.<base>
// admin console: a fleet overview, account directories, and the live/historical
// tunnel list with geo. All handlers are mounted under requireAdmin (admin session
// cookie), never exposed to normal user JWTs.

// handleAdminStats returns the fleet overview: headline counts plus the geo
// breakdown of tunnel sessions, for the dashboard's top cards + map.
func (s *Server) handleAdminStats(w http.ResponseWriter, r *http.Request) {
	stats, err := s.store.AdminOverview(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	geoAll, _ := s.store.TunnelCountryBreakdown(r.Context(), "", false)
	geoActive, _ := s.store.TunnelCountryBreakdown(r.Context(), "", true)
	writeJSON(w, http.StatusOK, map[string]any{
		"stats":        stats,
		"geo":          geoAll,
		"geo_active":   geoActive,
		"regions":      s.geo.Regions(),
		"generated_at": time.Now().UTC(),
	})
}

// handleAdminUsers lists accounts (newest first) with their primary org + plan,
// so an admin can browse/search the user base. ?q= filters by email/name.
func (s *Server) handleAdminUsers(w http.ResponseWriter, r *http.Request) {
	limit, offset := parsePage(r, 50, 200)
	users, err := s.store.ListUsers(r.Context(), limit, offset, r.URL.Query().Get("q"))
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	out := make([]map[string]any, 0, len(users))
	for _, u := range users {
		row := map[string]any{
			"id": u.ID, "email": u.Email, "name": u.Name,
			"oauth_provider": u.OAuthProvider, "created_at": u.CreatedAt,
		}
		// Best-effort enrichment with the user's primary org + effective plan.
		if orgs, e := s.store.OrgsForUser(r.Context(), u.ID); e == nil && len(orgs) > 0 {
			row["org_id"] = orgs[0].ID
			row["plan"] = effectivePlan(orgs[0])
			row["plan_expires_at"] = orgs[0].PlanExpiresAt
		}
		out = append(out, row)
	}
	writeJSON(w, http.StatusOK, map[string]any{"users": out, "limit": limit, "offset": offset})
}

// handleAdminOrgs lists orgs (newest first) with member count + effective plan.
// ?plan= filters by stored plan code.
func (s *Server) handleAdminOrgs(w http.ResponseWriter, r *http.Request) {
	limit, offset := parsePage(r, 50, 200)
	orgs, err := s.store.ListOrgs(r.Context(), limit, offset, r.URL.Query().Get("plan"))
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	out := make([]map[string]any, 0, len(orgs))
	for _, o := range orgs {
		members, _ := s.store.ListOrgMembers(r.Context(), o.ID)
		out = append(out, map[string]any{
			"id": o.ID, "name": o.Name,
			"plan":            effectivePlan(o),
			"stored_plan":     o.Plan,
			"plan_expires_at": o.PlanExpiresAt,
			"members":         len(members),
			"created_at":      o.CreatedAt,
		})
	}
	writeJSON(w, http.StatusOK, map[string]any{"orgs": out, "limit": limit, "offset": offset})
}

// handleAdminTunnels lists tunnel sessions across ALL orgs, newest first, for the
// fleet view. ?active=true limits to live tunnels; ?limit/?offset paginate. The
// total count backs pagination in the UI.
func (s *Server) handleAdminTunnels(w http.ResponseWriter, r *http.Request) {
	limit, offset := parsePage(r, 50, 200)
	activeOnly := r.URL.Query().Get("active") == "true"
	sessions, err := s.store.ListTunnelSessions(r.Context(), store.TunnelSessionFilter{
		ActiveOnly: activeOnly, Limit: limit, Offset: offset,
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	total, _ := s.store.CountTunnelSessions(r.Context(), "", activeOnly)
	writeJSON(w, http.StatusOK, map[string]any{
		"sessions": sessions, "total": total, "limit": limit, "offset": offset,
	})
}
