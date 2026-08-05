package cli

import (
	"fmt"
	"os/exec"
	"runtime"
	"strings"
)

// openBrowser opens url in the user's default browser. It returns the error from
// launching the opener; callers that can proceed without a browser (device-flow
// sign-in prints the URL as a fallback) may ignore it.
//
// url always comes from a server response we already trust for this session
// (the control plane's device-flow verification URL, or our own edge's tunnel
// public URL) — but as defense in depth against a compromised control plane or
// a MITM'd --insecure session, only http(s) is accepted. The OS openers here
// (rundll32 url.dll,FileProtocolHandler in particular) will happily hand off
// file:// paths or other registered protocol handlers to whatever's on the
// other end, which is not something a tunnel URL should ever need to do.
func openBrowser(url string) error {
	if !isHTTPURL(url) {
		return fmt.Errorf("refusing to open non-http(s) URL: %s", url)
	}
	switch runtime.GOOS {
	case "windows":
		// rundll32 avoids the quoting pitfalls of `cmd /c start` with URLs that
		// contain & or ? (our device URLs carry a ?code= query).
		// #nosec G204 -- url is scheme-checked above; argv element to a fixed program name, never through a shell
		return exec.Command("rundll32", "url.dll,FileProtocolHandler", url).Start()
	case "darwin":
		// #nosec G204 -- url is scheme-checked above; argv element to a fixed program name, never through a shell
		return exec.Command("open", url).Start()
	default:
		// #nosec G204 -- url is scheme-checked above; argv element to a fixed program name, never through a shell
		return exec.Command("xdg-open", url).Start()
	}
}

// isHTTPURL reports whether url starts with an http(s) scheme, case-insensitively.
func isHTTPURL(url string) bool {
	u := strings.ToLower(url)
	return strings.HasPrefix(u, "http://") || strings.HasPrefix(u, "https://")
}
