# Configuration

Rift reads a YAML config from `~/.rift/rift.yml`. Define your tunnels once, then
start them all with `rift start`.

## Full schema

```yaml
version: 1
api_key: "rk_live_xxx"          # or env RIFT_API_KEY, or `rift login`
server: "edge.rift.dev:443"     # default endpoint (nearest region is resolved)
region: "auto"                  # auto | us | eu | ap ...
transport: "auto"               # auto | quic | tcp
tunnels:
  web:
    proto: http                 # http | https | tls | tcp | udp
    addr: "localhost:3000"
    subdomain: "myapp"          # optional; reserved subdomains need Pro
    basic_auth: "user:pass"     # optional
  ssh:
    proto: tcp
    addr: "localhost:22"
    remote_port: 2222           # optional reserved port
inspector:
  enabled: true
  addr: "127.0.0.1:4040"
log_level: "info"               # debug | info | warn | error
```

## Start everything

```sh
rift start
```

This brings up every tunnel defined under `tunnels:`. To run a single one ad hoc,
use the direct commands like `rift http 3000` instead.

## Precedence

When the same setting is specified in more than one place, Rift uses this order
(highest wins):

1. **CLI flags** — e.g. `--subdomain`, `--region`
2. **Environment variables** — `RIFT_API_KEY`, `RIFT_*`
3. **Config file** — `~/.rift/rift.yml`
4. **Defaults**

## Transport

`transport: auto` (the default) dials QUIC first and falls back to TCP + yamux if
UDP/QUIC is blocked. Pin it with `quic` or `tcp` if you need deterministic
behavior on a locked-down network.

## Editing

```sh
rift config
```

opens the config for editing and validates it. A malformed file is reported with
the offending field.
