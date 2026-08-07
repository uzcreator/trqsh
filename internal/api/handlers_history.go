package api

import (
	"net/http"
	"strconv"
	"time"

	"github.com/trqsh-uz/trqsh/internal/api/store"
)

// parsePage reads limit/offset query params with sane defaults + caps, so history
// listings can't be asked to return unbounded result sets.
func parsePage(r *http.Request, defLimit, maxLimit int) (limit, offset int) {
	limit, offset = defLimit, 0
	if v, err := strconv.Atoi(r.URL.Query().Get("limit")); err == nil && v > 0 {
		limit = v
	}
	if limit > maxLimit {
		limit = maxLimit
	}
	if v, err := strconv.Atoi(r.URL.Query().Get("offset")); err == nil && v > 0 {
		offset = v
	}
	return limit, offset
}

// handleTunnelHistory returns the caller org's tunnel sessions, newest first, with
// pagination + total count so the dashboard can render "full history". Includes
// closed and active sessions; pass ?active=true to filter to live ones.
func (s *Server) handleTunnelHistory(w http.ResponseWriter, r *http.Request) {
	org := orgOf(r)
	limit, offset := parsePage(r, 50, 200)
	activeOnly := r.URL.Query().Get("active") == "true"

	sessions, err := s.store.ListTunnelSessions(r.Context(), store.TunnelSessionFilter{
		OrgID: org, ActiveOnly: activeOnly, Limit: limit, Offset: offset,
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	total, _ := s.store.CountTunnelSessions(r.Context(), org, activeOnly)
	writeJSON(w, http.StatusOK, map[string]any{
		"sessions": sessions,
		"total":    total,
		"limit":    limit,
		"offset":   offset,
	})
}

// handleUsageHistory returns the caller org's traffic as a time series for
// dashboard graphs. ?bucket=hour|day (default day), ?days=N window (default 30).
func (s *Server) handleUsageHistory(w http.ResponseWriter, r *http.Request) {
	org := orgOf(r)
	bucket := r.URL.Query().Get("bucket")
	if bucket != "hour" {
		bucket = "day"
	}
	days := 30
	if v, err := strconv.Atoi(r.URL.Query().Get("days")); err == nil && v > 0 {
		days = v
	}
	if days > 365 {
		days = 365
	}
	since := time.Now().AddDate(0, 0, -days)

	series, err := s.store.UsageSeriesForOrg(r.Context(), org, since, bucket)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if series == nil {
		series = []store.UsageBucket{}
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"bucket": bucket,
		"since":  since.UTC(),
		"series": series,
	})
}
