# Webhooks & CI

Two of the most common reasons to reach for a tunnel: receiving webhooks during
local development, and sharing an ephemeral URL from a CI job.

## Receiving webhooks locally

Point a provider (Stripe, GitHub, Slack, …) at your machine while you develop:

```sh
trqsh http 3000 --subdomain myapp
```

Give the provider `https://myapp.trqsh.uz/webhooks`. When events arrive, open the
[inspector](/docs/inspector) at `localhost:4040` to see the exact payload — and
**replay** it as you iterate on your handler, without asking the provider to send
again.

A [reserved subdomain](/docs/reserved-subdomains) keeps the webhook URL stable so
you don't have to reconfigure the provider on every restart.

## Using trqsh in CI

Expose a service from a CI job — for example to run end-to-end tests against a
preview, or to share a build with a reviewer.

1. Store a dedicated API key as a CI secret (see [Authentication](/docs/authentication)).
2. Provide it via the environment and start a tunnel in the background:

```sh
export TRQSH_API_KEY=tq_live_ci_key
trqsh http 8080 --subdomain pr-${PR_NUMBER} &
```

3. Use the resulting URL in your test job, then let the job end to tear the tunnel
   down.

### Tips

- Use a **separate key per pipeline** so you can revoke it independently.
- Pick subdomains from CI variables (like a PR number) for predictable URLs.
- Watch for [`ERR_QUOTA_TUNNELS`](/docs/errors#err_quota_tunnels) if many jobs run
  at once — concurrency is bounded by your plan.
