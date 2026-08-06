package cli

import (
	"strings"

	"github.com/trqsh-uz/trqsh/internal/agent"
)

// versionString formats the build metadata shown by the root's --version flag
// and the branded help header, so the two never drift.
func versionString() string {
	s := agent.Version
	if agent.Commit != "" {
		s += " (" + agent.Commit + ")"
	}
	if agent.BuildDate != "" {
		s += " " + agent.BuildDate
	}
	return s
}

// firstNonEmpty returns a if it's non-empty, else b.
func firstNonEmpty(a, b string) string {
	if a != "" {
		return a
	}
	return b
}

// baseDomainOf derives the public base domain (e.g. "trqsh.uz") from the edge
// server address, for showing reserved subdomains as full hosts in the console.
func baseDomainOf(cfg agent.Config) string {
	h := strings.TrimSpace(cfg.Server)
	if i := strings.LastIndex(h, ":"); i > 0 {
		h = h[:i]
	}
	if h == "" {
		return "trqsh.uz"
	}
	return h
}
