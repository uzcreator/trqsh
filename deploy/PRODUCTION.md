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
docker compose -f docker-compose.prod.yml pull && \
  docker compose -f docker-compose.prod.yml up -d --build     # update after git pull
docker compose -f docker-compose.prod.yml down                # stop (keeps volumes)
```
