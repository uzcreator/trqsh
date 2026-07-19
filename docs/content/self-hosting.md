# Self-hosting

The Rift **agent is open source**. You can audit it, build it yourself, and run it
however you like. The hosted edge, control plane, and billing are operated by Rift
as a service.

## Build the agent from source

```sh
git clone https://github.com/rift/rift
cd rift
go build ./cmd/rift
```

You now have a `rift` binary identical to the released one (releases are just this,
cross-compiled and signed). Point it at the hosted edge with your API key and it
behaves exactly like the packaged CLI.

## Why the agent is open

- **Trust** — you can see precisely what runs on your machine and what it sends.
- **Scriptability** — embed the agent core in your own tools; the Go API is stable.
- **Portability** — build for any platform Go targets.

## What isn't open (yet)

The edge (`riftd`), control API, and billing are proprietary and run as the hosted
service — that's what your subscription pays for, and it's what keeps the network,
certificates, and multi-region routing maintained.

## Running your own edge

A fully self-hosted deployment (your own edge + control plane) isn't part of the
free open-source distribution today. If you have a strong need — air-gapped
networks, compliance, on-prem — [get in touch](mailto:hello@rift.dev); we're
interested in an open-core path.

## Contributing

Issues and pull requests for the agent are welcome on
[GitHub](https://github.com/rift/rift). See the repository's contributing guide for
the development setup and coding standards.
