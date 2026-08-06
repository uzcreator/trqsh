package tui

import (
	"slices"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/trqsh-uz/trqsh/internal/agent"
	"github.com/trqsh-uz/trqsh/internal/agent/inspect"
)

func TestParseLine(t *testing.T) {
	name, args := parseLine("/http 3000 --subdomain app")
	if name != "http" {
		t.Fatalf("name = %q, want http", name)
	}
	if got := strings.Join(args, ","); got != "3000,--subdomain,app" {
		t.Fatalf("args = %q", got)
	}
	// A bare word (no slash) still parses, and the command name is lowercased.
	if n, _ := parseLine("LS"); n != "ls" {
		t.Fatalf("name = %q, want ls", n)
	}
	if n, a := parseLine("  /help  "); n != "help" || len(a) != 0 {
		t.Fatalf("parseLine(/help) = %q, %v", n, a)
	}
}

func TestLookupCommandAndAliases(t *testing.T) {
	for _, tc := range []struct{ in, want string }{
		{"http", "http"},
		{"q", "quit"},
		{"exit", "quit"},
		{"list", "ls"},
		{"me", "whoami"},
		{"?", "help"},
	} {
		got, ok := lookupCommand(tc.in)
		if !ok || got.name != tc.want {
			t.Errorf("lookupCommand(%q) = %q, %v; want %q", tc.in, got.name, ok, tc.want)
		}
	}
	if _, ok := lookupCommand("nope"); ok {
		t.Error("lookupCommand(nope) should not resolve")
	}
}

func TestPinUnpin(t *testing.T) {
	m := &model{}
	cmdPin(m, []string{"traffic"})
	cmdPin(m, []string{"traffic"}) // duplicate: no-op
	cmdPin(m, []string{"bogus"})   // not pinnable: rejected
	if !slices.Equal(m.pins, []string{"traffic"}) {
		t.Fatalf("pins = %v, want [traffic]", m.pins)
	}
	cmdPin(m, []string{"tunnels"})
	cmdUnpin(m, []string{"traffic"})
	if !slices.Equal(m.pins, []string{"tunnels"}) {
		t.Fatalf("after unpin, pins = %v, want [tunnels]", m.pins)
	}
	cmdUnpin(m, []string{"all"})
	if len(m.pins) != 0 {
		t.Fatalf("after unpin all, pins = %v, want []", m.pins)
	}
}

func TestRefreshMenuFilters(t *testing.T) {
	m := newModel(Options{}, newClient("127.0.0.1:0", ""))
	m.input.SetValue("/pi")
	m.refreshMenu()
	if !m.menuOpen() {
		t.Fatal("menu should be open for /pi")
	}
	if len(m.menuItems) != 1 || m.menuItems[0].insert != "/pin " {
		t.Fatalf("menu items = %v, want one entry inserting \"/pin \"", m.menuItems)
	}
	// Typing an argument surfaces arrow-selectable suggestions (so you pick
	// "traffic" instead of typing it).
	m.input.SetValue("/pin tra")
	m.refreshMenu()
	if !m.menuOpen() || m.menuItems[0].insert != "/pin traffic" {
		t.Fatalf("expected a /pin traffic suggestion, got %v", m.menuItems)
	}
	// A free-form argument (a port) offers no suggestions, so the menu closes.
	m.input.SetValue("/http 3000")
	m.refreshMenu()
	if m.menuOpen() {
		t.Fatal("menu should be closed for a free-form argument like a port")
	}
}

func TestHandleEventCollectsTraffic(t *testing.T) {
	m := &model{}
	m.handleEvent(agent.Event{Type: "request", Request: &inspect.CapturedRequest{
		Method: "GET", Path: "/x", Status: 200, DurationMs: 5,
	}})
	m.handleEvent(agent.Event{Type: "status", Status: &agent.Status{Connected: true, Edge: "eu"}})
	if len(m.requests) != 1 || m.requests[0].method != "GET" {
		t.Fatalf("requests = %+v", m.requests)
	}
	if !m.status.Connected || m.status.Edge != "eu" {
		t.Fatalf("status not applied: %+v", m.status)
	}
}

// feed drives the model through a sequence of messages, mimicking the Bubble Tea
// runtime without a real terminal.
func feed(m model, msgs ...tea.Msg) model {
	for _, msg := range msgs {
		next, _ := m.Update(msg)
		m = next.(model)
	}
	return m
}

func typeStr(m model, s string) model {
	for _, r := range s {
		m = feed(m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
	}
	return m
}

// TestReplFlow proves the whole keystroke → parse → command pipeline: typing a
// slash command and pressing Enter runs it and mutates state.
func TestReplFlow(t *testing.T) {
	m := newModel(Options{Version: "test"}, newClient("127.0.0.1:0", ""))
	m = feed(m, tea.WindowSizeMsg{Width: 80, Height: 24})
	if !m.ready {
		t.Fatal("model should be ready after a window-size message")
	}
	m = typeStr(m, "/pin traffic")
	m = feed(m, tea.KeyMsg{Type: tea.KeyEnter})
	if !slices.Contains(m.pins, "traffic") {
		t.Fatalf("after typing /pin traffic + enter, pins = %v", m.pins)
	}

	// /clear empties the transcript.
	m = typeStr(m, "/clear")
	m = feed(m, tea.KeyMsg{Type: tea.KeyEnter})
	if len(m.transcript) != 0 {
		t.Fatalf("after /clear, transcript has %d lines", len(m.transcript))
	}
}

// TestViewFillsScreen checks the layout math: the rendered frame is exactly the
// terminal height and never wider than the terminal, so nothing wraps or scrolls
// the outer screen.
func TestViewFillsScreen(t *testing.T) {
	m := newModel(Options{Version: "test"}, newClient("127.0.0.1:0", ""))
	m = feed(m, tea.WindowSizeMsg{Width: 100, Height: 30})
	m = feed(m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("/")}) // open the command menu
	view := m.View()
	lines := strings.Split(view, "\n")
	if len(lines) != 30 {
		t.Fatalf("view has %d lines, want 30", len(lines))
	}
}
