package tui

import (
	"fmt"
	"os/exec"
	"runtime"
	"strings"
)

// openBrowser opens url in the user's default browser. It mirrors the CLI's own
// opener (internal/agent/cli/browser.go) — kept local so the tui package stays
// decoupled from cli — including the http(s)-only guard: the OS handlers will
// happily launch file:// or other protocol handlers, which a tunnel URL should
// never need, so anything non-http is refused as defense in depth.
func openBrowser(url string) error {
	u := strings.ToLower(url)
	if !strings.HasPrefix(u, "http://") && !strings.HasPrefix(u, "https://") {
		return fmt.Errorf("refusing to open non-http(s) URL: %s", url)
	}
	switch runtime.GOOS {
	case "windows":
		// #nosec G204 -- url is scheme-checked above; passed as an argv element to a fixed program, never via a shell
		return exec.Command("rundll32", "url.dll,FileProtocolHandler", url).Start()
	case "darwin":
		// #nosec G204 -- url is scheme-checked above; passed as an argv element to a fixed program, never via a shell
		return exec.Command("open", url).Start()
	default:
		// #nosec G204 -- url is scheme-checked above; passed as an argv element to a fixed program, never via a shell
		return exec.Command("xdg-open", url).Start()
	}
}
