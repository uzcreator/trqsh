# deploy/loadtest — capacity & load harness

Turns "we can handle ~15,000 concurrent tunnels" from a **guess** into a **measured
number** (the overall plan's Phase 11 intent). Nothing here runs against anything
live automatically — you point it at a real **staging** (preferred) or **prod**
server yourself, once one exists.

There are two halves, because trqsh has two very different load surfaces:

| Component | What it loads | Tool | Why |
|---|---|---|---|
| `tunnelload/` | data plane: N concurrent agent **sessions + tunnels** | custom **Go** harness | an agent session speaks the trqsh protocol (QUIC/TCP + `pkg/tunnel` framing), *not* HTTP, so no generic tool can open one — it reuses the real `internal/agent` Core |
| `httpload/` | control plane + web + traffic **through** tunnels (plain HTTP) | **vegeta** | standard HTTP RPS fits a standard tool |

### Why vegeta (not k6)

A single static Go binary with no runtime — it matches this Go-heavy repo, installs
in one line in CI or on a throwaway load box (no Node), and its `attack | report`
output gives exactly the latency percentiles + histograms you want for a capacity
number. k6 (JS) would also work and suits the repo's Node frontends, but pulls in a
runtime the load box otherwise doesn't need. Either is defensible; we picked the
lighter, more consistent one.

---

## 1. Data-plane load — `tunnelload` (Go)

Opens `-n` real synthetic agents against a target edge, each binding a tunnel to a
small HTTP server built into the harness, and reports **connect success rate** and
**connect-latency percentiles**. Then it holds the tunnels open (`-duration`) so the
HTTP step can drive traffic through them.

```bash
# From the repo root. Build-verified with `go build ./deploy/loadtest/...`.
go run ./deploy/loadtest/tunnelload \
  -server staging.trqsh.uz:4443 \
  -apikey tq_xxxxxxxx \
  -n 500 -concurrency 50 -rampup 20s \
  -duration 5m \
  -insecure \
  -urls-out /tmp/trqsh-urls.txt
```

Key flags: `-n` (tunnels), `-concurrency` (in-flight connects), `-rampup` (spread
starts to avoid a thundering herd), `-transport auto|quic|tcp`, `-insecure` (for
Let's Encrypt **staging** certs), `-urls-out` (feed the HTTP step). `-h` for all.

**What it can and cannot tell you**

- ✅ How many concurrent agent sessions + tunnels an edge accepts, and the connect
  latency distribution as you scale `-n`. This is the real "N concurrent" number.
- ✅ With `-urls-out`, a set of live public URLs to push request load through.
- ⚠️ It is a **connect/hold** model, not a realistic per-tunnel traffic mix. Combine
  it with `httpload/tunnels.sh` for request throughput.
- ⚠️ Ceilings you'll hit first are usually **not** the edge: the **load box** file
  descriptors / ephemeral ports (raise `ulimit -n`), and your **account's
  concurrent-tunnel limit** (`-n` above the plan cap shows up as `plan_forbids`
  failures in the summary — use a high-limit test account).

## 2. Control-plane / web + through-tunnel load — `httpload` (vegeta)

```bash
cd deploy/loadtest/httpload

# Control plane + marketing/dashboard (read-only, safe defaults):
cp controlplane.targets.example controlplane.targets   # edit hostnames
./run.sh controlplane.targets 200 30s

# Request traffic THROUGH the tunnels tunnelload is holding open:
./tunnels.sh /tmp/trqsh-urls.txt 500 60s
```

`run.sh <targets> [rate] [duration]` prints a vegeta report + latency histogram.
`tunnels.sh <urls-file> [rate] [duration]` turns the `-urls-out` list into targets.

**Caveat — the rate limiter caps single-source auth load.** `/v1/auth/*` is per-IP
limited (5 sustained / 10 burst) and the rest of `/v1` at 50/100, so hammering auth
from one IP throttles to `429`s fast. That *proves the limiter works*; to measure raw
auth capacity, raise the limits in staging or attack from many IPs.

## Interpreting results

- **Connect success rate** should stay ~100% as you raise `-n`; the `-n` where it
  drops (or p95 connect latency spikes) is your practical per-edge session ceiling.
- **vegeta**: watch `Success` ratio, `Latencies [p50/p95/p99]`, and the histogram.
  Non-200s in the through-tunnel run point at the edge→agent path; non-200s on the
  control plane that are `429` are the rate limiter, not saturation.
- Scale horizontally by adding edges (Terraform `edge_nodes_per_region`) and API
  replicas (`--scale api=N` + the `apilb` profile — see `deploy/PRODUCTION.md`), then
  re-measure.

## Deferred on purpose

Running these for real needs a server to run them **against**, so it is left until a
staging/prod environment exists (spin one up with `deploy/.env.staging.example`).
CI does not run load tests — only `go build`/`vet` of the harness and `sh -n` of the
scripts.
