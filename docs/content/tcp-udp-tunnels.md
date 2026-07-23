# TCP & UDP tunnels

trqsh isn't just for web apps. Tunnel any TCP service — and, unlike ngrok, **UDP**
too.

## TCP tunnels

Expose a raw TCP port, such as SSH:

```sh
trqsh tcp 22
```

trqsh assigns a public host and port:

```
Forwarding  tcp://tcp.trqsh.uz:24187  →  localhost:22
```

Connect through it like any other host:

```sh
ssh -p 24187 user@tcp.trqsh.uz
```

TCP tunnels are great for SSH, Postgres/MySQL, Redis, SMTP, and any custom
protocol.

### Reserve a port

Ask for a specific remote port (subject to availability and your plan):

```sh
trqsh tcp 22 --remote-port 2222
```

A taken port returns [`ERR_PORT_UNAVAILABLE`](/docs/errors#err_port_unavailable);
omit `--remote-port` to get an ephemeral one.

## UDP tunnels

UDP is a first-class citizen — QUIC makes it natural. Expose a UDP service such as
a DNS resolver, game server, or WebRTC endpoint:

```sh
trqsh udp 51820
```

```
Forwarding  udp://udp.trqsh.uz:39912  →  localhost:51820
```

UDP tunnels require a plan that includes UDP. On the free plan you'll see
[`ERR_PLAN_FORBIDS`](/docs/errors#err_plan_forbids) with the plan needed — compare
options on the [pricing page](/pricing).

## TLS tunnels

To terminate TLS yourself (passthrough), use a `tls` tunnel so the edge forwards
the encrypted stream untouched:

```sh
trqsh tls 8443
```

## Protocol support by plan

HTTP, HTTPS, and TCP are available on every plan. UDP and TLS are included on Pro
and above. See the full matrix on the [pricing page](/pricing).
