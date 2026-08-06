//go:build !windows

package ui

import "os"

// isTerminal reports whether f looks like a terminal. Unix exposes this as the
// character-device bit on the file's mode, which is cleared when stdout is a
// pipe or a regular file — enough to gate colorization without pulling in an
// ioctl/termios dependency.
func isTerminal(f *os.File) bool {
	fi, err := f.Stat()
	if err != nil {
		return false
	}
	return fi.Mode()&os.ModeCharDevice != 0
}

// enableVT is a no-op off Windows: every mainstream Unix terminal interprets
// ANSI sequences without any per-process opt-in.
func enableVT(*os.File) {}
