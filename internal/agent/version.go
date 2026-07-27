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

// DefaultAPIBase is the control-plane API the desktop app reaches (through the
// agent) for account/plan/usage/keys/domains and OAuth device login. Overridable
// via TRQSH_API_URL or -ldflags "-X .../internal/agent.DefaultAPIBase=https://…".
var DefaultAPIBase = "https://api.trqsh.uz"
