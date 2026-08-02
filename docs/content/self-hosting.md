# Self-hosting

trqsh is **fully open source** (Apache-2.0) — not just the agent, but the entire stack: the
edge, the control plane, and billing. You can run the whole thing yourself, or use the
hosted service at [trqsh.uz](https://trqsh.uz) and skip the operations.

## Just the agent

If you only want to point the CLI at the hosted service (or someone else's edge), build the
agent from source:

```sh
git clone https://github.com/uzcreator/trqsh
cd trqsh
go build ./cmd/trqsh
```

You now have a `trqsh` binary identical to the released one (releases are just this,
cross-compiled and signed). Point it at an edge with your API key and it behaves exactly
like the packaged CLI.

- **Trust** — see precisely what runs on your machine and what it sends.
- **Scriptability** — embed the agent core in your own tools; the Go API is stable.
- **Portability** — build for any platform Go targets.

## The whole platform

Everything needed to run your own trqsh — edge, control API, dashboard, and site — is in
the repository:

```sh
git clone https://github.com/uzcreator/trqsh
cd trqsh
make dev          # full local stack: postgres, redis, migrate, api, edge, mailhog
```

For a real deployment, [`deploy/`](https://github.com/uzcreator/trqsh/tree/main/deploy) has
everything: multi-stage Dockerfiles, `docker-compose` for a single box, a Helm chart (edge
DaemonSet, API autoscaling, ingress, migrations, network policies), and Terraform for a
multi-region setup with managed Postgres/Redis and wildcard DNS. Start with
[`deploy/PRODUCTION.md`](https://github.com/uzcreator/trqsh/blob/main/deploy/PRODUCTION.md).

You'll need a domain with a wildcard DNS record pointed at your edge, and — for public
HTTPS — a wildcard TLS certificate, which the edge can obtain automatically via Let's
Encrypt DNS-01.

## Hosted vs self-hosted

| | Hosted (trqsh.uz) | Self-hosted |
|---|---|---|
| Setup | none — just `trqsh login` | run the edge + control plane |
| Domain & TLS | managed | your domain + wildcard cert |
| Multi-region | included | your infrastructure |
| Cost | subscription | your servers |
| Best for | most people | air-gapped, on-prem, compliance, tinkering |

## Contributing

Issues and pull requests are welcome on [GitHub](https://github.com/uzcreator/trqsh). See the
repository's [contributing guide](https://github.com/uzcreator/trqsh/blob/main/CONTRIBUTING.md)
for the development setup and coding standards.
