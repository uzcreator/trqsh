package server

import (
	"github.com/prometheus/client_golang/prometheus"
)

// Metrics holds the edge's Prometheus collectors, registered on a private
// registry so tests can construct independent Server instances.
type Metrics struct {
	reg *prometheus.Registry

	SessionsActive prometheus.Gauge
	TunnelsActive  prometheus.Gauge
	StreamsOpened  *prometheus.CounterVec // by proto
	BytesTotal     *prometheus.CounterVec // by dir (in/out)
	Handshakes     *prometheus.CounterVec // by kind (quic/tcp)
	Requests       *prometheus.CounterVec // by scheme
	Errors         *prometheus.CounterVec // by kind
}

// NewMetrics builds and registers the collectors.
func NewMetrics() *Metrics {
	reg := prometheus.NewRegistry()
	m := &Metrics{
		reg: reg,
		SessionsActive: prometheus.NewGauge(prometheus.GaugeOpts{
			Name: "rift_sessions_active", Help: "Active agent sessions.",
		}),
		TunnelsActive: prometheus.NewGauge(prometheus.GaugeOpts{
			Name: "rift_tunnels_active", Help: "Active bound tunnels.",
		}),
		StreamsOpened: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "rift_streams_opened_total", Help: "Data streams opened, by proto.",
		}, []string{"proto"}),
		BytesTotal: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "rift_bytes_total", Help: "Bytes welded, by direction.",
		}, []string{"dir"}),
		Handshakes: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "rift_agent_handshakes_total", Help: "Agent sessions accepted, by transport.",
		}, []string{"kind"}),
		Requests: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "rift_http_requests_total", Help: "HTTP requests proxied, by scheme.",
		}, []string{"scheme"}),
		Errors: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "rift_errors_total", Help: "Edge errors, by kind.",
		}, []string{"kind"}),
	}
	reg.MustRegister(
		m.SessionsActive, m.TunnelsActive, m.StreamsOpened,
		m.BytesTotal, m.Handshakes, m.Requests, m.Errors,
	)
	return m
}

// Registry exposes the Prometheus registry for the /metrics handler.
func (m *Metrics) Registry() *prometheus.Registry { return m.reg }

func (m *Metrics) countBytes(up, down int64) {
	if up > 0 {
		m.BytesTotal.WithLabelValues("in").Add(float64(up))
	}
	if down > 0 {
		m.BytesTotal.WithLabelValues("out").Add(float64(down))
	}
}
