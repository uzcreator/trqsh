package tui

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
)

// The TUI's palette. Lipgloss downsamples these to the terminal's real color
// profile (truecolor → 256 → 16 → none) and honors NO_COLOR automatically, so
// the same code looks right everywhere from Windows Terminal to a bare TTY.
var (
	colBrand  = lipgloss.Color("#2dd4bf") // trqsh teal
	colDim    = lipgloss.Color("#6b7280") // muted gray
	colGreen  = lipgloss.Color("#22c55e")
	colYellow = lipgloss.Color("#eab308")
	colRed    = lipgloss.Color("#ef4444")
	colCyan   = lipgloss.Color("#38bdf8")
	colViolet = lipgloss.Color("#a78bfa")

	// Surfaces: subtle backgrounds so the pinned panels and the input box read as
	// distinct raised surfaces rather than blending into the terminal.
	colBorder  = lipgloss.Color("#33524d") // muted teal frame
	colPanelBg = lipgloss.Color("#0e1b19") // pinned-panel background
	colInputBg = lipgloss.Color("#13211f") // input-box background
)

var (
	stBrand = lipgloss.NewStyle().Foreground(colBrand).Bold(true)
	stTitle = lipgloss.NewStyle().Foreground(colBrand).Bold(true)
	stDim   = lipgloss.NewStyle().Foreground(colDim)
	stKey   = lipgloss.NewStyle().Foreground(colBrand).Bold(true)
	stURL   = lipgloss.NewStyle().Foreground(colCyan)
	stOK    = lipgloss.NewStyle().Foreground(colGreen)
	stErr   = lipgloss.NewStyle().Foreground(colRed)
	stWarn  = lipgloss.NewStyle().Foreground(colYellow)

	// Command-menu selection highlight (a subtle inverse bar).
	stMenuSel = lipgloss.NewStyle().Foreground(lipgloss.Color("#0b0f10")).Background(colBrand).Bold(true)
)

// lipglossFg renders s in foreground color c — a shorthand for the one-off
// dynamically-colored spans (like the connection dot) that don't warrant a
// named style.
func lipglossFg(c lipgloss.Color, s string) string {
	return lipgloss.NewStyle().Foreground(c).Render(s)
}

// statusStyle colors an HTTP status code by class: 2xx green, 3xx cyan, 4xx
// yellow, 5xx red.
func statusStyle(code int) lipgloss.Style {
	switch {
	case code >= 500:
		return lipgloss.NewStyle().Foreground(colRed)
	case code >= 400:
		return lipgloss.NewStyle().Foreground(colYellow)
	case code >= 300:
		return lipgloss.NewStyle().Foreground(colCyan)
	case code >= 200:
		return lipgloss.NewStyle().Foreground(colGreen)
	default:
		return stDim
	}
}

// roundedBox renders body inside a rounded box of the given OUTER width, with an
// optional title spliced into the top border and an optional background so the
// box reads as a raised surface. body may contain ANSI (its own colors survive).
func roundedBox(title, body string, outerW int, border, bg lipgloss.Color) string {
	st := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(border).
		Width(outerW-2). // Width is the content+padding box; the border adds 2
		Padding(0, 1)
	if bg != "" {
		st = st.Background(bg).BorderBackground(bg)
		body = fillBg(body, bg) // keep the background continuous across colored spans
	}
	return spliceTitle(st.Render(body), title, border, bg)
}

// bgSeq returns the raw SGR sequence that switches the background to bg for the
// active color profile (empty when color is disabled), by rendering a sentinel
// byte and slicing off everything the style emits before it.
func bgSeq(bg lipgloss.Color) string {
	s := lipgloss.NewStyle().Background(bg).Render("\x00")
	if i := strings.IndexByte(s, 0); i > 0 {
		return s[:i]
	}
	return ""
}

// fillBg re-asserts bg after every SGR reset in content, so a panel's background
// stays unbroken across inner foreground-colored spans. lipgloss ends each styled
// run with a full reset (\x1b[0m), which clears the background too; without this,
// colored text inside a filled box shows up on the terminal's default background,
// punching holes in the panel.
func fillBg(content string, bg lipgloss.Color) string {
	seq := bgSeq(bg)
	if seq == "" {
		return content
	}
	return seq + strings.ReplaceAll(content, "\x1b[0m", "\x1b[0m"+seq)
}

// panelBox is a pinned-panel surface (teal frame, dark background, titled).
func panelBox(title, body string, outerW int) string {
	return roundedBox(title, body, outerW, colBrand, colPanelBg)
}

// inputBox is the box the prompt sits in, clearly separated from the transcript.
func inputBox(body string, outerW int) string {
	return roundedBox("", body, outerW, colBorder, colInputBg)
}

// spliceTitle overwrites the top border of a rendered box with "╭─ title ─…─╮",
// keeping the frame color and background. It rebuilds the line from scratch (the
// original top edge is a single uniform run, so nothing else is lost).
func spliceTitle(box, title string, fg, bg lipgloss.Color) string {
	if title == "" {
		return box
	}
	lines := strings.Split(box, "\n")
	if len(lines) == 0 {
		return box
	}
	w := lipgloss.Width(lines[0])
	frame := lipgloss.NewStyle().Foreground(fg).Background(bg)
	titleStyle := lipgloss.NewStyle().Foreground(colBrand).Background(bg).Bold(true)
	head := "╭─ "
	tail := " "
	dashCount := w - lipgloss.Width(head) - lipgloss.Width(title) - lipgloss.Width(tail) - 1
	if dashCount < 0 {
		return box // too narrow for a title; leave the plain border
	}
	lines[0] = frame.Render(head) + titleStyle.Render(title) + frame.Render(tail+strings.Repeat("─", dashCount)+"╮")
	return strings.Join(lines, "\n")
}

// fitLines pads (or truncates) body to exactly n lines of width w, so panels in
// a horizontal row share a height and edge cleanly.
func fitLines(body []string, n, w int) []string {
	out := make([]string, 0, n)
	for i := range n {
		if i < len(body) {
			line := body[i]
			if lipgloss.Width(line) > w {
				line = ansi.Truncate(line, w, "…")
			}
			out = append(out, line)
		} else {
			out = append(out, "")
		}
	}
	return out
}

// methodStyle gives each HTTP method a stable accent so the eye can scan the
// live feed by verb.
func methodStyle(method string) lipgloss.Style {
	switch method {
	case "GET":
		return lipgloss.NewStyle().Foreground(colGreen)
	case "POST":
		return lipgloss.NewStyle().Foreground(colCyan)
	case "PUT", "PATCH":
		return lipgloss.NewStyle().Foreground(colYellow)
	case "DELETE":
		return lipgloss.NewStyle().Foreground(colRed)
	default:
		return lipgloss.NewStyle().Foreground(colViolet)
	}
}
