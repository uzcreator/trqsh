# Authentication

The agent authenticates to the edge with an **API key**. The dashboard uses JWT
sessions; you never need those for the CLI.

## Interactive login

```sh
rift login
```

This starts a device-authorization flow: the CLI shows a short code, opens your
browser, and once you approve, it stores an API key in `~/.rift/rift.yml`. This is
the easiest path and works on any machine with a browser.

## API keys

API keys look like `rk_live_…`. Create and manage them in the dashboard under
**API keys**. A key is shown **once** at creation — copy it then. Keys are hashed
at rest and can be revoked at any time.

Provide a key without `rift login` in any of these ways (highest priority first):

1. A flag: `rift http 3000 --api-key rk_live_…`
2. An environment variable: `export RIFT_API_KEY=rk_live_…`
3. The config file: set `api_key` in `~/.rift/rift.yml` (see [Configuration](/docs/configuration)).

Environment variables are ideal for CI, where you'd store the key as a secret.

## Which key for which machine

Use a **separate key per machine or CI pipeline** so you can revoke one without
disrupting the others, and so usage is attributable. Revoking a key immediately
stops any agent using it — the next bind fails with
[`ERR_AUTH_INVALID`](/docs/errors#err_auth_invalid).

## Logging out

```sh
rift logout
```

This removes the stored key from your config. Revoke the key in the dashboard too
if the machine is shared or compromised.

## Common errors

- [`ERR_AUTH_REQUIRED`](/docs/errors#err_auth_required) — no key was provided.
- [`ERR_AUTH_INVALID`](/docs/errors#err_auth_invalid) — the key was rejected or revoked.
