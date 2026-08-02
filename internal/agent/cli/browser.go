package cli

import (
	"os/exec"
	"runtime"
)

// openBrowser opens url in the user's default browser. It returns the error from
// launching the opener; callers that can proceed without a browser (device-flow
// sign-in prints the URL as a fallback) may ignore it.
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
