package api

import (
	"net/http"

	"github.com/trqsh-uz/trqsh/internal/geo"
)

// handleGeo detects the caller's country and recommends the nearest edge region.
// It powers location-aware connection: the CLI/desktop/agent call it (before or
// after auth) to learn which PoP to dial and to show the user where they'll
// connect. Public and side-effect free.
//
// The source IP is the caller's, extracted with the same trusted-proxy rules as
// rate limiting. In development an explicit ?ip= override is honored for testing;
// in production it is ignored so the endpoint can't be used as an open GeoIP proxy
// for arbitrary addresses.
func (s *Server) handleGeo(w http.ResponseWriter, r *http.Request) {
	ip := clientIP(r, s.cfg.TrustProxy)
	if s.cfg.DevAuth {
		if override := r.URL.Query().Get("ip"); override != "" {
			ip = override
		}
	}

	loc := s.geo.FromRequest(r, ip)
	region := s.geo.Nearest(loc)
	regions := s.geo.Regions()
	geo.SortRegions(regions)

	writeJSON(w, http.StatusOK, map[string]any{
		"ip":       loc.IP,
		"location": loc,
		"region":   region,  // recommended nearest PoP
		"regions":  regions, // full catalog so the client can offer a choice
		"detected": loc.Known(),
	})
}

// handleListRegions returns the edge PoP catalog (code, city, endpoint, …) so a
// client can render a region picker or resolve a region code to an endpoint.
func (s *Server) handleListRegions(w http.ResponseWriter, _ *http.Request) {
	regions := s.geo.Regions()
	geo.SortRegions(regions)
	writeJSON(w, http.StatusOK, regions)
}
