# Observability

Metrics, dashboards, alerts, and tracing for trqsh.

```
observability/
├── otel-collector.yaml              # OTLP receiver → Prometheus/exporters
├── prometheus/
│   ├── prometheus.yml               # scrape config (edge /metrics)
│   └── alerts.yml                   # alert rules over real edge metrics
└── grafana/
    ├── provisioning/                # auto-load datasource + dashboards
    └── dashboards/trqsh-overview.json
```

## Local

```bash
make observability      # docker compose --profile observability up -d
```

- Grafana → http://localhost:3001 (admin / admin), "trqsh — Edge Overview" pre-loaded.
- Prometheus → http://localhost:9091
- The edge is scraped at `edge:9090/metrics`.

## Edge metrics (from `internal/server/metrics.go`)

Scraped at `edge:9090/metrics`.

| Metric | Type | Labels |
|---|---|---|
| `trqsh_sessions_active` | gauge | — |
| `trqsh_tunnels_active` | gauge | — |
| `trqsh_streams_opened_total` | counter | `proto` |
| `trqsh_bytes_total` | counter | `dir` (in/out) |
| `trqsh_agent_handshakes_total` | counter | `kind` (quic/tcp) |
| `trqsh_http_requests_total` | counter | `scheme` |
| `trqsh_forwards_total` | counter | `dir` (out=handed to a peer edge, in=received from a peer edge) |
| `trqsh_errors_total` | counter | `kind` |

Cross-edge forwarding (multi-edge only, see `internal/server/forward.go` and
`forward_listener.go`) adds these `trqsh_errors_total{kind=...}` values on top of the
pre-existing ones: `forward_dial` (couldn't reach the owning edge — down, draining,
or a stale registry entry), `forward_write` (dial succeeded but replaying the
request failed), `forward_auth` (a hop arrived with a missing/wrong
`TRQSH_INTERNAL_TOKEN`), `forward_proto` (a hop announced something other than
http/https — not yet supported). A healthy single-edge deployment
(`TRQSH_FORWARD_ADDR` unset) never emits `trqsh_forwards_total` or these `kind`s.

## Control-API metrics (from `internal/api/metrics.go`)

Scraped at `api:9090/metrics` — a **separate internal ops port** from the public
`:8080` the edge reverse-proxies (so `/metrics` is never publicly reachable).
Recorded automatically for every route by router middleware.

| Metric | Type | Labels |
|---|---|---|
| `trqsh_api_http_requests_total` | counter | `route`, `method`, `code` |
| `trqsh_api_http_requests_in_flight` | gauge | — |
| `trqsh_api_http_request_duration_seconds` | histogram | `route`, `method` |

## Kubernetes

The Helm chart ships a `ServiceMonitor` (enable with `serviceMonitor.enabled=true`)
for the Prometheus Operator to scrape the edge. Import `trqsh-overview.json` into a
Grafana instance or via the Grafana Operator.

## Known gaps

- **Request latency histograms (p50/p95)** are not yet emitted by the edge; the
  dashboard covers throughput, sessions, handshakes, requests, and errors. Add a
  `prometheus.Histogram` for weld/proxy latency in `internal/server` to light up a
  latency panel. (The control API already emits
  `trqsh_api_http_request_duration_seconds`.)
- **Grafana dashboard for the control API**: the pre-loaded dashboard covers the
  edge only; the API metrics above are scraped but not yet paneled.
