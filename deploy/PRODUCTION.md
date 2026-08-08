# Production deploy — single host

Bring the whole trqsh stack up on one Ubuntu server with Docker Compose.

## 0. Prerequisites (once)

- Docker + compose plugin (`curl -fsSL https://get.docker.com | sh`)
- Firewall open: `22, 80, 443` tcp and `4443` tcp+udp
- Repo cloned to `/opt/trqsh`

## 1. DNS — point the domain at this server

The edge serves every tunnel as `<name>.trqsh.uz`, so a **wildcard** record is
required. At your DNS provider create A records to the server's public IP:

| Type | Name | Value |
|------|------|-------|
| A | `@`  | `<SERVER_IP>` |
| A | `*`  | `<SERVER_IP>` |
| A | `api`| `<SERVER_IP>` |
| A | `app`| `<SERVER_IP>` |

Verify (from anywhere): `dig +short test123.trqsh.uz` → the server IP.

**TLS choice:**
- **Wildcard (recommended):** host DNS at Cloudflare and put a `Zone.DNS:Edit`
  API token in `TRQSH_CLOUDFLARE_API_TOKEN` → one `*.trqsh.uz` cert via DNS-01.
- **On-demand (simplest):** leave the token blank → each subdomain gets its own
  cert on first hit via TLS-ALPN. Works with any DNS provider.

## 2. Configure secrets

```bash
cd /opt/trqsh/deploy
cp .env.prod.example .env
# generate strong secrets:
printf 'POSTGRES_PASSWORD=%s\nTRQSH_JWT_SECRET=%s\nTRQSH_INTERNAL_TOKEN=%s\n' \
  "$(openssl rand -hex 16)" "$(openssl rand -hex 32)" "$(openssl rand -hex 32)"
# paste those into .env, then edit the rest (email, domain, TLS mode)
nano .env
```

`deploy/.env` is the **single source of configuration** — the compose file loads
the whole file into the control API (`env_file`), so everything below is a `.env`
edit + `up -d`, never a compose or code change. The example is grouped and
self-documenting; notable optional groups:

- **Admin dashboard** — set `TRQSH_ADMIN_USER` + `TRQSH_ADMIN_PASSWORD` to enable
  the fleet console (stats, users, orgs, tunnels, subscription grants) at
  `https://approve.<domain>`. Empty = disabled.
- **Location / geo** — set `TRQSH_GEOIP_API` (or `TRQSH_GEOIP_HEADER` behind a
  CDN) to enable country detection + nearest-region routing (`/v1/geo`) and the
  geo data in tunnel history. Blank = detection off, regions still served.
- **Billing, OAuth** — fill the Stripe / OAuth groups to turn those on.

## 3. Bring it up (staging first)

Keep `TRQSH_ACME_STAGING=1` for the first run — proves cert issuance without
hitting Let's Encrypt rate limits (certs will be untrusted; that's expected).

```bash
docker compose -f docker-compose.prod.yml up -d --build
docker compose -f docker-compose.prod.yml ps
docker compose -f docker-compose.prod.yml logs -f edge      # watch cert issuance
```

## 4. Go live (trusted certs)

Once staging looks healthy, switch to real certificates:

```bash
sed -i 's/^TRQSH_ACME_STAGING=1/TRQSH_ACME_STAGING=0/' .env
docker compose -f docker-compose.prod.yml up -d --force-recreate edge
```

## 5. Verify

```bash
curl -I https://trqsh.uz            # edge answers over real TLS
```

Then from a laptop: `trqsh http 3000` → open the printed `https://<name>.trqsh.uz`.

## Operate

```bash
docker compose -f docker-compose.prod.yml logs -f api edge   # tail logs
docker compose -f docker-compose.prod.yml ps                 # status
docker compose -f docker-compose.prod.yml down               # stop (keeps volumes)
```

## Updating — scoped, one service at a time

`api`, `edge`, `dashboard`, `site` and `migrate` run **prebuilt images** from
GHCR (built by `.github/workflows/images.yml` on every push to `main` and every
`v*` tag). `TRQSH_IMAGE_TAG` in `.env` pins which tag runs — default `latest`,
which is the last **tagged** release (main-branch builds are tagged `main` and
`sha-xxxxxxx`, not `latest`). Updating is a **pull + recreate of just the changed
service**, not a full-stack rebuild:

```bash
# Deploy a new build of ONE service (e.g. the marketing site):
export TRQSH_IMAGE_TAG=v0.2.4        # or a specific sha-xxxxxxx, or `latest`
docker compose -f docker-compose.prod.yml pull site
docker compose -f docker-compose.prod.yml up -d site
```

Do **not** run an unscoped `up -d` (nor the old `up -d --build`) for a routine
change. `edge` `depends_on` `api`/`site`/`dashboard`, so recreating everything
bounces `edge` — which **drops every active tunnel and agent session**, possibly
for an unrelated site/dashboard change. Scoping `pull`/`up -d` to the one changed
service leaves `edge` (and every live tunnel) untouched.

When the change *does* touch the edge, or ships a DB migration, order it so the
edge bounce is deliberate:

```bash
docker compose -f docker-compose.prod.yml pull migrate api edge
docker compose -f docker-compose.prod.yml up -d migrate    # runs goose, exits 0
docker compose -f docker-compose.prod.yml up -d api edge    # edge bounce expected here
```

## Backups

The `backup` service `pg_dump`s the `trqsh` database once every 24h and writes a
gzipped, timestamped dump — `trqsh-YYYYMMDD-HHMMSS.sql.gz` — to the
**`trqsh_pg_backups`** named volume (`/backups` inside the container). That is a
**separate** volume from the live `trqsh_pg` data volume on purpose: operator
error (`docker volume rm trqsh_pg`) or corruption of one must not take the other.

- **Retention:** dumps older than **14 days** are pruned automatically.
- **Offsite:** nothing leaves the box yet. Pick a provider, then uncomment the
  `rclone copy /backups remote:trqsh-backups` line in the `backup` service's
  command in `docker-compose.prod.yml` (and mount an rclone config). That single
  line is the extension point for S3 / Spaces / Backblaze.

Inspect what's there:

```bash
docker compose -f docker-compose.prod.yml exec backup ls -lh /backups
docker compose -f docker-compose.prod.yml logs backup | tail
```

### Restore drill (into a THROWAWAY container — never the live DB)

> ⚠️ **Never** restore a dump into the live `trqsh_pg` volume to "test" it — a bad
> restore can clobber production data. Always restore into a fresh throwaway
> Postgres container as below, spot-check, then remove it.

```bash
# 1. Spin up a throwaway postgres (its own ephemeral volume, no host port). Its
#    first-boot init creates the trqsh role + empty trqsh DB for us.
docker run -d --name trqsh_restore_test \
  -e POSTGRES_USER=trqsh -e POSTGRES_DB=trqsh -e POSTGRES_PASSWORD=test \
  postgres:16-alpine
sleep 5

# 2. Stream the newest dump from the backup volume into it (plain-SQL dump → psql).
docker compose -f docker-compose.prod.yml exec -T backup \
  sh -c 'gunzip -c "$(ls -1 /backups/trqsh-*.sql.gz | tail -1)"' \
  | docker exec -i trqsh_restore_test psql -U trqsh -d trqsh

# 3. Spot-check a table.
docker exec -i trqsh_restore_test psql -U trqsh -d trqsh -c 'select count(*) from users;'

# 4. Tear it down (removes its data too).
docker rm -f trqsh_restore_test
```

## Kernel & connection limits

The `edge` and `api` services set `ulimits.nofile` to 65535 (soft + hard) in the
compose file — they each hold many concurrent connections and the container
default of 1024 open files is far too low.

One limit lives **outside** Docker Compose: the host kernel's listen-backlog cap,
`net.core.somaxconn`. Raise it on the host and persist it:

```bash
sudo sysctl -w net.core.somaxconn=65535
echo 'net.core.somaxconn = 65535' | sudo tee /etc/sysctl.d/99-trqsh.conf
sudo sysctl --system     # reload persisted sysctls
```

## Scaling the control API

The `api` service keeps its durable state in Postgres + Redis, so you can run
several replicas on one host:

```bash
docker compose -f docker-compose.prod.yml up -d --scale api=3
```

`--scale` works because the `api` service declares no `container_name` and no host
port (recent Compose also honours a `deploy.replicas: N` key on `up`). Read the two
caveats below **before** scaling past 1.

**1. Load distribution is uneven without a balancer.** The edge reaches the API by
the `api` service name, which Compose's embedded DNS round-robins — but the edge
*pools* upstream connections (the entitlement RPC client and the `api.<base>`
reverse proxy both keep keep-alive connections warm). Pooled connections stick to
whichever replicas they first resolved, so steady traffic piles onto a subset of
replicas. The fix is the opt-in **`apilb`** Caddy sidecar, which re-resolves `api`
to *all* replica IPs and least-conn balances each request:

```bash
# in deploy/.env:  TRQSH_API_TARGET=apilb
docker compose -f docker-compose.prod.yml --profile apilb up -d --scale api=3
```

**2. Two auth flows still hold per-replica in-memory state.** Until these move to
Redis, running >1 replica can make them fail intermittently:

- **OAuth web login** keeps the CSRF `state` in memory on the replica that started
  it; if the provider callback lands on a *different* replica, validation fails.
  `apilb` largely hides this because start + callback share the browser's IP and
  Caddy keeps a client on one replica — but it is not guaranteed.
- **Device-code login** (`trqsh login` from the CLI) keeps the pending code in
  memory. The CLI polls on one connection while the **browser** approves on
  another — two different clients — so **no** load-balancer setting can keep them on
  the same replica. This one genuinely breaks under N replicas.

Everything else is replica-safe today: **password login, API-key auth, and all
JWT-authenticated traffic** are stateless/Postgres-backed, and **rate limiting is
already shared via Redis** (so N replicas don't multiply the limits). The edge's own
high-volume entitlement RPC is API-key/token based and scales cleanly.

**Practical guidance:** the edge↔API entitlement traffic (the hot path) scales fine
with `--scale api=N` + `apilb`. If you also serve **public OAuth / device login**,
either keep `api` at **1 replica**, or first move the OAuth `states` map and the
device-code registry to Redis (the API already holds a Redis client — a small,
well-scoped follow-up). This is the top thing to fix before relying on API
horizontal scale-out.

## Beyond one host — multi-region edges

This file covers the single-host stack. To run **multiple edges** (per-region
droplets with cross-edge forwarding + health-checked DNS steering) provision them
with `deploy/terraform/` — see [terraform/README.md](terraform/README.md). The
compose edge already ships the forwarding code (Stage D); it simply runs as a
single edge here because `TRQSH_FORWARD_ADDR` is unset. Cross-edge forwarding turns
on automatically once more than one edge shares the same Redis and each sets
`TRQSH_FORWARD_ADDR` / `TRQSH_FORWARD_ADVERTISE_ADDR`, which the Terraform cloud-init
does for you.

## Staging environment

Before pointing production at the riskier Stage C/D changes (shared Redis cert
storage, cross-edge forwarding), rehearse them on a **separate staging host** built
from the same compose file with [`.env.staging.example`](.env.staging.example)
(copy it to `deploy/.env` there). It keeps `TRQSH_ACME_STAGING=1` (untrusted certs,
no Let's Encrypt rate limits) and uses a separate domain + secrets, so you can prove
issuance and forwarding end-to-end without risking the prod cert quota or live
tunnels. Load-test it with [`loadtest/`](loadtest/README.md).

## Verify the wildcard cert is actually being used

With `TRQSH_CLOUDFLARE_API_TOKEN` set, the edge issues **one** `*.trqsh.uz`
wildcard cert via DNS-01 and serves it for every subdomain. To confirm the
wildcard (and not per-subdomain on-demand certs) is in play, check the SAN on two
*different* random subdomains — a wildcard shows the same `DNS:*.trqsh.uz` on both:

```bash
# Two different random subdomains, e.g. a1b2c3 and z9y8x7:
openssl s_client -connect trqsh.uz:443 -servername a1b2c3.trqsh.uz </dev/null 2>/dev/null \
  | openssl x509 -noout -text | grep -A1 "Subject Alternative Name"
openssl s_client -connect trqsh.uz:443 -servername z9y8x7.trqsh.uz </dev/null 2>/dev/null \
  | openssl x509 -noout -text | grep -A1 "Subject Alternative Name"
```

- **Both show `DNS:*.trqsh.uz`** → the wildcard is working.
- **Each shows a distinct single-name SAN** (`DNS:<sub>.trqsh.uz`) → the edge fell
  back to per-subdomain on-demand issuance. Either `TRQSH_CLOUDFLARE_API_TOKEN`
  was never set (check `deploy/.env` on the server) or DNS-01 issuance is failing
  silently. Check the edge logs for ACME / DNS-01 errors (issuance runs through
  `newACMECertManager` / `Warm` in `internal/server/tls_acme.go`):

```bash
docker compose -f docker-compose.prod.yml logs edge | grep -iE 'acme|dns-01|cloudflare|cert'
```

## Dashboard build args

The dashboard is a Next.js app that **inlines `TRQSH_API_URL` and
`NEXT_PUBLIC_TRQSH_BASE_DOMAIN` at build time** (see `deploy/docker/Dockerfile.dashboard`).
`images.yml` now passes the production values (`TRQSH_API_URL=https://api.trqsh.uz`,
`NEXT_PUBLIC_TRQSH_BASE_DOMAIN=trqsh.uz`) as per-service build-args, so the GHCR
`dashboard` image is **prod-correct out of the box** — no action needed for a
`trqsh.uz` deployment.

If you run a **different** base domain, the baked-in values won't match. Build the
dashboard on the box via the commented `build:` fallback in
`docker-compose.prod.yml` (it passes the args from your `.env`), or edit the
`dashboard` entry's `extra_build_args` in `images.yml`.

`site`, `api`, `edge` and `migrate` have no such build-time coupling.

## Observability (optional)

Prometheus + Grafana ship as an **opt-in** compose profile — the base stack runs
without them. Neither is exposed on a public port: both bind to `127.0.0.1` only,
so reach them over an SSH tunnel.

```bash
cd /opt/trqsh/deploy
# (optional) set a Grafana admin password in .env: GRAFANA_ADMIN_PASSWORD=...
docker compose -f docker-compose.prod.yml --profile observability up -d
```

Prometheus scrapes the edge (`edge:9090`) and control API (`api:9090`) on their
internal metrics ports. From your laptop, tunnel and browse:

```bash
ssh -L 3001:127.0.0.1:3001 -L 9091:127.0.0.1:9091 <SERVER>
# then: Grafana http://localhost:3001 (admin / GRAFANA_ADMIN_PASSWORD)
#       Prometheus http://localhost:9091
```

Stop just the observability services without touching the base stack:

```bash
docker compose -f docker-compose.prod.yml --profile observability stop grafana prometheus
```
