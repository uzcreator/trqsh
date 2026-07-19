# Security & abuse

How Rift protects your traffic and your account — and how to report a problem.

## Transport security

- **TLS everywhere.** Agent ↔ edge, public ↔ edge, and browser ↔ dashboard are all
  encrypted. There is no plaintext path in production.
- **QUIC/HTTP-3** by default, with an authenticated TLS-over-TCP fallback. Either
  way the session is mutually established before any tunnel binds.

## Account & keys

- API keys are **high-entropy**, **hashed at rest**, shown once, and revocable.
- Use a **distinct key per machine or pipeline** so revocation is surgical.
- The dashboard uses short-lived JWT sessions with refresh; OAuth via GitHub/Google
  is supported.

## Tenant isolation

A session can only bind subdomains, custom domains, and ports your account is
entitled to. Quotas and protocol access are enforced at the edge on every bind —
not just in the UI — so limits can't be bypassed by scripting the agent.

## Abuse controls

Public hostnames are subject to **phishing and malware screening**. Tunnels that
host illegal content, distribute malware, or attack others may be suspended. Per-
account rate limits and network-level DDoS protections run at the edge.

## Reporting abuse

Seeing a Rift URL used for phishing or malware? Email **abuse@rift.dev** with the
hostname and details. We act on verified reports quickly.

## Reporting a vulnerability

Found a security issue in Rift or the open-source agent? Please disclose it
responsibly to **security@rift.dev** rather than filing a public issue. We'll
acknowledge, investigate, and keep you updated. We don't take legal action against
good-faith research that respects our users' privacy and data.

## Your responsibilities

- Keep your API keys secret; rotate them if exposed.
- Don't tunnel services you aren't authorized to expose.
- Follow the [Terms of Service](/terms) and applicable law.
