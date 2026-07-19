package agent

// Build metadata, injected via -ldflags at release time (Part 08). Defaults are
// used for local `go run`/`go build`.
var (
	Version   = "dev"
	Commit    = ""
	BuildDate = ""
)
