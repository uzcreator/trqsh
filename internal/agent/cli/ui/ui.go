// Package ui centralizes trqsh's terminal presentation — color, icons, spinners,
// and the semantic status printers (✓ / ✗ / ⚠) — so every command renders with
// one consistent, professional look instead of ad-hoc fmt.Printf calls.
//
// Color is enabled only when stdout is a real terminal and the user hasn't
// opted out (NO_COLOR / TRQSH_NO_COLOR), so piping to a file or a CI log yields
// clean, unescaped text. On Windows it additionally flips on virtual-terminal
// processing (see vt_windows.go) so ANSI sequences render in the classic
// conhost console, not just in Windows Terminal.
package ui

import (
	"fmt"
	"io"
	"os"
	"strings"
)

// Stdout and Stderr are the sinks the package prints to. They're variables so
// tests (and, later, a global --quiet) can redirect output.
var (
	Stdout io.Writer = os.Stdout
	Stderr io.Writer = os.Stderr
)

// enabled reports whether ANSI styling is active. It's decided once from the
// environment and the stdout terminal; SetEnabled overrides it (tests, flags).
var enabled = decideColor(os.Stdout)

// SetEnabled forces color on or off, e.g. for tests or a --no-color flag.
func SetEnabled(v bool) { enabled = v }

// Enabled reports whether styling is currently active. Callers that need to
// know whether they're driving a real terminal (spinners, progress) can ask.
func Enabled() bool { return enabled }

// decideColor resolves the color decision from the standard signals used by
// well-behaved CLIs: NO_COLOR (any value disables), TERM=dumb, and an explicit
// FORCE_COLOR / CLICOLOR_FORCE override, otherwise on iff f is a terminal.
func decideColor(f *os.File) bool {
	if os.Getenv("NO_COLOR") != "" || os.Getenv("TRQSH_NO_COLOR") != "" {
		return false
	}
	if strings.EqualFold(os.Getenv("TERM"), "dumb") {
		return false
	}
	if os.Getenv("FORCE_COLOR") != "" || os.Getenv("CLICOLOR_FORCE") == "1" {
		enableVT(os.Stdout)
		enableVT(os.Stderr)
		return true
	}
	if !isTerminal(f) {
		return false
	}
	enableVT(os.Stdout)
	enableVT(os.Stderr)
	return true
}

const reset = "\x1b[0m"

// style wraps s in the given SGR code(s) when color is on; otherwise it returns
// s untouched so plain-text output stays byte-for-byte clean.
func style(code, s string) string {
	if !enabled || s == "" {
		return s
	}
	return "\x1b[" + code + "m" + s + reset
}

// --- weights ---

func Bold(s string) string      { return style("1", s) }
func Faint(s string) string     { return style("2", s) }
func Underline(s string) string { return style("4", s) }

// --- palette (standard SGR, universally supported once VT is enabled) ---

func Red(s string) string    { return style("31", s) }
func Green(s string) string  { return style("32", s) }
func Yellow(s string) string { return style("33", s) }
func Blue(s string) string   { return style("34", s) }
func Cyan(s string) string   { return style("36", s) }
func Gray(s string) string   { return style("90", s) } // bright black; a reliable "muted"

// --- brand accent: trqsh teal ---
//
// Truecolor so it matches the site's palette on modern terminals; on the rare
// terminal without 24-bit color the sequence is ignored and the text simply
// renders in the default color, which is harmless.

func Accent(s string) string     { return style("38;2;45;212;191", s) }
func AccentBold(s string) string { return style("1;38;2;45;212;191", s) }

// Link renders a URL so it reads as clickable (cyan + underline).
func Link(s string) string { return Underline(Cyan(s)) }

// Title renders a section heading for help/version blocks.
func Title(s string) string { return AccentBold(s) }

// Icons. These match the glyphs the CLI already used, so nothing regresses on a
// legacy raster-font console while looking crisp in a modern terminal.
const (
	IconOK    = "✓"
	IconErr   = "✗"
	IconWarn  = "⚠"
	IconInfo  = "•"
	IconArrow = "→"
)

// --- semantic line printers ---
//
// All use the same two-space gutter the CLI already established, so mixing
// these with existing fmt.Printf lines stays visually aligned.

// Success prints a green ✓ line to stdout.
func Success(format string, a ...any) {
	fmt.Fprintf(Stdout, "  %s %s\n", Green(IconOK), fmt.Sprintf(format, a...))
}

// Fail prints a red ✗ line to stderr.
func Fail(format string, a ...any) {
	fmt.Fprintf(Stderr, "  %s %s\n", Red(IconErr), fmt.Sprintf(format, a...))
}

// Warn prints a yellow ⚠ line to stderr.
func Warn(format string, a ...any) {
	fmt.Fprintf(Stderr, "  %s %s\n", Yellow(IconWarn), fmt.Sprintf(format, a...))
}

// Info prints a muted • line to stdout — a neutral status, not an error.
func Info(format string, a ...any) {
	fmt.Fprintf(Stdout, "  %s %s\n", Gray(IconInfo), fmt.Sprintf(format, a...))
}

// Step prints an accented → line to stdout — an action taken or a next step.
func Step(format string, a ...any) {
	fmt.Fprintf(Stdout, "  %s %s\n", Accent(IconArrow), fmt.Sprintf(format, a...))
}

// Printf writes an indented, styled line verbatim (no icon), for output that
// doesn't fit the ✓/✗/⚠ vocabulary.
func Printf(format string, a ...any) {
	fmt.Fprintf(Stdout, format, a...)
}

// Pad right-pads s with spaces to width n (measured in bytes, which equals
// display width for the ASCII command names and keys it's used on). Padding is
// applied to the raw string before styling so columns line up regardless of the
// invisible escape sequences a caller may wrap around the result.
func Pad(s string, n int) string {
	if len(s) >= n {
		return s
	}
	return s + strings.Repeat(" ", n-len(s))
}
