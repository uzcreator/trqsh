package agent

// Build metadata, injected via -ldflags at release time (Part 08). Defaults are
// used for local `go run`/`go build`.
var (
	Version   = "dev"
	Commit    = ""
	BuildDate = ""
)

// DefaultServer is the edge the agent connects to when the user has not set one
// (via --server, TRQSH_SERVER, or the config file). It points at the hosted
// trqsh edge so a freshly downloaded CLI works out of the box; override it at
// build time with -ldflags "-X .../internal/agent.DefaultServer=host:port" for
// self-hosted or local-dev builds.
var DefaultServer = "trqsh.uz:4443"
