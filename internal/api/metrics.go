package api

import (
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/prometheus/client_golang/prometheus"
)

// metrics holds the control-plane API's Prometheus collectors on a private
// registry (mirrors internal/server's Metrics). The edge names metrics trqsh_*;
// the API namespaces its own under trqsh_api_* to keep the two apart in one
// Prometheus. It stays deliberately small — recorded automatically for every
// route by middleware rather than hand-instrumented per handler.
type metrics struct {
	reg      *prometheus.Registry
	requests *prometheus.CounterVec // by route, method, code
	inFlight prometheus.Gauge
	duration *prometheus.HistogramVec // by route, method
}

func newMetrics() *metrics {
	reg := prometheus.NewRegistry()
	m := &metrics{
		reg: reg,
		requests: prometheus.NewCounterVec(prometheus.CounterOpts{
			Namespace: "trqsh", Subsystem: "api",
			Name: "http_requests_total", Help: "Control-API HTTP requests, by route, method and status.",
		}, []string{"route", "method", "code"}),
		inFlight: prometheus.NewGauge(prometheus.GaugeOpts{
			Namespace: "trqsh", Subsystem: "api",
			Name: "http_requests_in_flight", Help: "In-flight control-API HTTP requests.",
		}),
		duration: prometheus.NewHistogramVec(prometheus.HistogramOpts{
			Namespace: "trqsh", Subsystem: "api",
			Name: "http_request_duration_seconds", Help: "Control-API HTTP request latency, by route and method.",
			Buckets: prometheus.DefBuckets,
		}, []string{"route", "method"}),
	}
	reg.MustRegister(m.requests, m.inFlight, m.duration)
	return m
}

// Registry exposes the Prometheus registry for the /metrics handler.
func (m *metrics) Registry() *prometheus.Registry { return m.reg }

// middleware records per-request metrics for every route on the router. The
// label is the chi route pattern (e.g. /v1/api-keys/{id}), not the raw path, so
// cardinality stays bounded regardless of how many distinct IDs are requested.
func (m *metrics) middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		m.inFlight.Inc()
		defer m.inFlight.Dec()
		ww := middleware.NewWrapResponseWriter(w, r.ProtoMajor)
		start := time.Now()
		next.ServeHTTP(ww, r)
		route := routePattern(r)
		m.duration.WithLabelValues(route, r.Method).Observe(time.Since(start).Seconds())
		m.requests.WithLabelValues(route, r.Method, strconv.Itoa(ww.Status())).Inc()
	})
}

// routePattern returns the matched chi pattern after routing, or "unmatched" for
// requests that hit no route (bounds label cardinality against arbitrary 404s).
func routePattern(r *http.Request) string {
	if rctx := chi.RouteContext(r.Context()); rctx != nil {
		if p := rctx.RoutePattern(); p != "" {
			return p
		}
	}
	return "unmatched"
}
