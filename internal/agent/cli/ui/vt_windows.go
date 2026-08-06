//go:build windows

package ui

import (
	"os"

	"golang.org/x/sys/windows"
)

// isTerminal reports whether f is attached to a console. GetConsoleMode
// succeeds only for a real console handle, so it doubles as the "is this a TTY"
// check and returns false when output is redirected to a file or pipe.
func isTerminal(f *os.File) bool {
	var mode uint32
	return windows.GetConsoleMode(windows.Handle(f.Fd()), &mode) == nil
}

// enableVT turns on ENABLE_VIRTUAL_TERMINAL_PROCESSING for f's console so ANSI
// color and cursor sequences are interpreted rather than printed literally.
// Windows Terminal enables this by default, but the classic conhost.exe (still
// the default in many setups) does not — without this, styled output shows up
// as raw "\x1b[..m" garbage. Best-effort: if f isn't a console, we've already
// decided not to colorize, so a failure here is a no-op.
func enableVT(f *os.File) {
	h := windows.Handle(f.Fd())
	var mode uint32
	if err := windows.GetConsoleMode(h, &mode); err != nil {
		return
	}
	_ = windows.SetConsoleMode(h, mode|windows.ENABLE_VIRTUAL_TERMINAL_PROCESSING)
}
