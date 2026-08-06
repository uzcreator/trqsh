package tui

import (
	"fmt"
	"net"
	"strings"
	"time"
)

// normalizeAddr turns "3000" into "localhost:3000", leaving host:port as-is —
// the same rule the `trqsh http` CLI applies, so tunnels started in the TUI
// behave identically to ones started from the shell.
func normalizeAddr(a string) string {
	a = strings.TrimSpace(a)
	if i := strings.Index(a, "://"); i >= 0 {
		a = a[i+3:]
	}
	if _, _, err := net.SplitHostPort(a); err == nil {
		return a
	}
	return "localhost:" + a
}

// trunc shortens s to at most n display cells, appending an ellipsis when cut.
// It counts runes (good enough for the ASCII-heavy URLs and paths shown here).
func trunc(s string, n int) string {
	if n <= 0 {
		return ""
	}
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	if n == 1 {
		return "…"
	}
	return string(r[:n-1]) + "…"
}

// pad right-pads s with spaces to width n (rune count).
func pad(s string, n int) string {
	r := []rune(s)
	if len(r) >= n {
		return s
	}
	return s + strings.Repeat(" ", n-len(r))
}

// shortID trims a tunnel id to its first 8 characters for compact display.
func shortID(id string) string {
	if len(id) > 8 {
		return id[:8]
	}
	return id
}

// uptime renders how long ago t was, at a coarseness that suits a live view.
func uptime(t time.Time) string {
	if t.IsZero() {
		return "-"
	}
	d := time.Since(t)
	switch {
	case d < time.Minute:
		return d.Round(time.Second).String()
	case d < time.Hour:
		return d.Round(time.Minute).String()
	default:
		return d.Round(time.Hour).String()
	}
}

// humanBytes renders a byte count compactly (used by /whoami usage).
func humanBytes(n int64) string {
	const unit = 1024
	if n < unit {
		return fmt.Sprintf("%d B", n)
	}
	div, exp := int64(unit), 0
	for x := n / unit; x >= unit; x /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(n)/float64(div), "KMGTPE"[exp])
}
