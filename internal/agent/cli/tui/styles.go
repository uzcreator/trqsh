package tui

import "github.com/charmbracelet/lipgloss"

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
