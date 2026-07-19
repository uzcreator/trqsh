# Reserved subdomains

By default each tunnel gets a random subdomain like `tidy-otter-4f2a.rift.sh`.
Reserve one to keep the **same URL every time** — handy for webhooks, OAuth
callbacks, and sharing a stable link.

## Reserve one

In the dashboard, open **Subdomains** and reserve a name, e.g. `myapp`. It's then
yours across restarts. Your plan sets how many you can hold:

- Free: 1
- Pro: 10
- Team: 50

The exact numbers are on the [pricing page](/pricing) (and enforced by the edge).

## Use it

```sh
rift http 3000 --subdomain myapp
```

You'll consistently get `https://myapp.rift.sh`.

## In a config file

```yaml
tunnels:
  web:
    proto: http
    addr: "localhost:3000"
    subdomain: "myapp"
```

Then just run `rift start`. See [Configuration](/docs/configuration).

## Errors

- [`ERR_SUBDOMAIN_TAKEN`](/docs/errors#err_subdomain_taken) — someone else holds it;
  pick another.
- [`ERR_SUBDOMAIN_FORBIDDEN`](/docs/errors#err_subdomain_forbidden) — you haven't
  reserved it, or you're out of slots. Reserve it first, or upgrade for more.

## Custom domains

Want `tunnel.yourcompany.com` instead of a `rift.sh` subdomain? Use a
[custom domain](/docs/custom-domains).
