package cli

import (
	"os/exec"
	"runtime"
)

// openBrowser opens url in the user's default browser. It returns the error from
// launching the opener; callers that can proceed without a browser (device-flow
// sign-in prints the URL as a fallback) may ignore it.
// #nosec G204 -- url is our own device-flow verification URL (or user-supplied
// via the CLI, same trust level as any argument they pass themselves); it's
// passed as a single argv element to a fixed program name, never through a
// shell, so it cannot inject additional commands.
func openBrowser(url string) error {
	switch runtime.GOOS {
	case "windows":
		// rundll32 avoids the quoting pitfalls of `cmd /c start` with URLs that
		// contain & or ? (our device URLs carry a ?code= query).
		return exec.Command("rundll32", "url.dll,FileProtocolHandler", url).Start()
	case "darwin":
		return exec.Command("open", url).Start()
	default:
		return exec.Command("xdg-open", url).Start()
	}
}
