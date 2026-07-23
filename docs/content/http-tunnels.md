# HTTP tunnels

The most common tunnel: expose a local web app or API over a public HTTPS URL.

## Basics

```sh
trqsh http 3000
```

`3000` is your local port. trqsh assigns a random subdomain and serves it over
HTTPS with a valid certificate — no TLS setup on your side. You can also point at a
full address:

```sh
trqsh http localhost:3000
trqsh http 127.0.0.1:8080
```

## Pick a subdomain

```sh
trqsh http 3000 --subdomain myapp
```

You'll get `https://myapp.trqsh.uz` if it's available. To keep a subdomain reserved
for your account across restarts, see [Reserved subdomains](/docs/reserved-subdomains).
Requesting one you don't own returns
[`ERR_SUBDOMAIN_FORBIDDEN`](/docs/errors#err_subdomain_forbidden); one already taken
returns [`ERR_SUBDOMAIN_TAKEN`](/docs/errors#err_subdomain_taken).

## Protect it with basic auth

```sh
trqsh http 3000 --basic-auth user:secret
```

Anyone visiting the URL is prompted for those credentials before the request
reaches your machine.

## Rewrite the Host header

Some frameworks are picky about the `Host` header. Send them the host they expect:

```sh
trqsh http 3000 --host-header localhost:3000
```

## HTTPS upstreams

If your local server already speaks TLS, tunnel it as `https`:

```sh
trqsh https 8443
```

## What you get

- A valid public certificate, issued and renewed automatically.
- QUIC/HTTP-3 transport with automatic TCP fallback — see
  [why that's faster](/#speed).
- Every request visible in the [inspector](/docs/inspector) at `localhost:4040`.

## Troubleshooting

If the public URL loads but returns errors, check that your local server is
actually running on the port you tunneled — the edge reports
[`ERR_UPSTREAM_UNREACHABLE`](/docs/errors#err_upstream_unreachable) when it can't
reach it.
