# Observability

Metrics, dashboards, alerts, and tracing for Rift.

```
observability/
├── otel-collector.yaml              # OTLP receiver → Prometheus/exporters
├── prometheus/
│   ├── prometheus.yml               # scrape config (edge /metrics)
│   └── alerts.yml                   # alert rules over real edge metrics
└── grafana/
    ├── provisioning/                # auto-load datasource + dashboards
    └── dashboards/rift-overview.json
```

## Local

```bash
make observability      # docker compose --profile observability up -d
```

- Grafana → http://localhost:3001 (admin / admin), "Rift — Edge Overview" pre-loaded.
- Prometheus → http://localhost:9091
- The edge is scraped at `edge:9090/metrics`.

## Metrics (from `internal/server/metrics.go`)

| Metric | Type | Labels |
|---|---|---|
| `rift_sessions_active` | gauge | — |
| `rift_tunnels_active` | gauge | — |
| `rift_streams_opened_total` | counter | `proto` |
| `rift_bytes_total` | counter | `dir` (in/out) |
| `rift_agent_handshakes_total` | counter | `kind` (quic/tcp) |
| `rift_http_requests_total` | counter | `scheme` |
| `rift_errors_total` | counter | `kind` |

## Kubernetes

The Helm chart ships a `ServiceMonitor` (enable with `serviceMonitor.enabled=true`)
for the Prometheus Operator to scrape the edge. Import `rift-overview.json` into a
Grafana instance or via the Grafana Operator.

## Known gaps

- **Request latency histograms (p50/p95)** are not yet emitted by the edge; the
  dashboard covers throughput, sessions, handshakes, requests, and errors. Add a
  `prometheus.Histogram` for weld/proxy latency in `internal/server` to light up a
  latency panel.
- **Control-API metrics**: `riftapi` exposes `/healthz` but not `/metrics` yet;
  webhook/quota alerts will attach once it registers a Prometheus handler.
