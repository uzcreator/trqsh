# Quickstart

Go from nothing to a public HTTPS URL for your local server in under a minute.

## 1. Install the CLI

On macOS or Linux:

```sh
curl -fsSL https://rift.dev/install.sh | sh
```

On Windows (PowerShell):

```sh
scoop install rift
```

Prefer a GUI, Homebrew, or a raw binary? See [all install options](/download).

## 2. Log in

```sh
rift login
```

This opens your browser and links the CLI to your account using a device code. No
account yet? Signing in creates one on the free plan — no credit card. You can
also skip the browser and paste an API key; see [Authentication](/docs/authentication).

## 3. Start a tunnel

Make sure something is listening locally — for example a dev server on port 3000 —
then run:

```sh
rift http 3000
```

Rift prints your live URL and a local inspector address:

```
● session online   transport quic   region us-east
Forwarding  https://tidy-otter-4f2a.rift.sh  →  http://localhost:3000
Inspect     http://localhost:4040
```

Open the `https://…rift.sh` URL in any browser, on any device — your local app is
now on the internet, with TLS, over QUIC.

## 4. Watch the traffic

Open <http://localhost:4040> to see every request in real time, inspect headers and
bodies, and **replay** any request while you debug. More in [Request inspector](/docs/inspector).

## Where to go next

- Keep the same URL every time with a [reserved subdomain](/docs/reserved-subdomains).
- Use [your own domain](/docs/custom-domains) with guided DNS.
- Tunnel SSH, databases, or game servers with [TCP & UDP tunnels](/docs/tcp-udp-tunnels).
- Put it all in a [config file](/docs/configuration) and run `rift start`.

Hitting an error code? Every one is explained in the [error reference](/docs/errors).
