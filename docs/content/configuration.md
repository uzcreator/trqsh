# Configuration

trqsh reads a YAML config from `~/.trqsh-uz/trqsh.yml`. Define your tunnels once, then
start them all with `trqsh start`.

## Full schema

```yaml
version: 1
api_key: "tq_live_xxx"          # or env TRQSH_API_KEY, or `trqsh login`
server: "edge.trqsh.uz:443"     # default endpoint (nearest region is resolved)
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
trqsh start
```

This brings up every tunnel defined under `tunnels:`. To run a single one ad hoc,
use the direct commands like `trqsh http 3000` instead.

## Precedence

When the same setting is specified in more than one place, trqsh uses this order
(highest wins):

1. **CLI flags** — e.g. `--subdomain`, `--region`
2. **Environment variables** — `TRQSH_API_KEY`, `TRQSH_*`
3. **Config file** — `~/.trqsh-uz/trqsh.yml`
4. **Defaults**

## Transport

`transport: auto` (the default) dials QUIC first and falls back to TCP + yamux if
UDP/QUIC is blocked. Pin it with `quic` or `tcp` if you need deterministic
behavior on a locked-down network.

## Editing

```sh
trqsh config
```

opens the config for editing and validates it. A malformed file is reported with
the offending field.
