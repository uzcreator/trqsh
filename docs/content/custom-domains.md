# Custom domains

Serve your tunnel from a domain you own — `tunnel.yourcompany.com` — with an
automatically issued TLS certificate.

## 1. Add the domain

In the dashboard, open **Domains** and add your hostname. Rift returns two DNS
records to create:

- A **TXT** record proving you control the domain.
- A **CNAME** record pointing the hostname at the Rift edge.

## 2. Create the DNS records

At your DNS provider, add exactly what the dashboard shows. For example:

```
TXT    _rift-challenge.tunnel.yourcompany.com    rift-verify=abc123…
CNAME  tunnel.yourcompany.com                    edge.rift.sh
```

DNS can take a few minutes (occasionally longer) to propagate.

## 3. Verify

Click **Verify** in the dashboard. Once the TXT record is visible, the domain is
marked verified and Rift begins issuing a certificate for it.

## 4. Use it

```sh
rift http 3000 --domain tunnel.yourcompany.com
```

Or in a config file:

```yaml
tunnels:
  web:
    proto: http
    addr: "localhost:3000"
    custom_domain: "tunnel.yourcompany.com"
```

## Plan limits

Custom domains are available on Pro (up to 5) and Team (up to 50). On a plan
without them you'll get [`ERR_PLAN_FORBIDS`](/docs/errors#err_plan_forbids).

## Errors

- [`ERR_DOMAIN_UNVERIFIED`](/docs/errors#err_domain_unverified) — the domain hasn't
  passed DNS verification yet. Re-check the TXT record and try again.

## Certificates

Rift provisions and renews certificates for verified domains automatically via
Let's Encrypt. You don't manage keys or renewals.
