package api

import (
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// TestMetricsMiddlewareRecordsAndExposes drives a request through the main router
// (so the metrics middleware records it) and then scrapes the ops /metrics
// handler, confirming the trqsh_api_* series are present with a bounded route
// label. This exercises item-2 end to end at the Go level.
func TestMetricsMiddlewareRecordsAndExposes(t *testing.T) {
	cfg := DefaultConfig() // in-memory store, dev
	s, err := New(cfg, slog.New(slog.NewTextHandler(io.Discard, nil)))
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	rr := httptest.NewRecorder()
	s.Router().ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/healthz", nil))
	if rr.Code != http.StatusOK {
		t.Fatalf("/healthz status %d, want 200", rr.Code)
	}

	mr := httptest.NewRecorder()
	s.opsHandler().ServeHTTP(mr, httptest.NewRequest(http.MethodGet, "/metrics", nil))
	if mr.Code != http.StatusOK {
		t.Fatalf("/metrics status %d, want 200", mr.Code)
	}
	body := mr.Body.String()
	for _, want := range []string{
		"trqsh_api_http_requests_total",
		"trqsh_api_http_requests_in_flight",
		"trqsh_api_http_request_duration_seconds",
		`route="/healthz"`,
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("/metrics output missing %q", want)
		}
	}
}
