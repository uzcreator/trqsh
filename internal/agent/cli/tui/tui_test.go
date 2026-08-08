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

// TestRemoteCommandBlocklist proves a paired-phone command runs through the
// exact same run() as locally typed input for an allowed command, but a
// remoteBlocked one (e.g. logout) is refused outright — never even reaching
// lookupCommand's execution, just a warning line.
func TestRemoteCommandBlocklist(t *testing.T) {
	m := &model{}
	cmd := m.handleRemoteEvent(&agent.RemoteEvent{Kind: "command", Command: "/logout"})
	if cmd != nil {
		t.Fatal("a blocked remote command should not return a tea.Cmd")
	}
	joined := strings.Join(m.transcript, "\n")
	if !strings.Contains(joined, "blocked") || !strings.Contains(joined, "/logout") {
		t.Fatalf("expected a blocked-command warning naming /logout, got transcript: %q", joined)
	}
	if strings.Contains(joined, "📱") {
		t.Fatalf("a blocked command should not echo the remote-run prefix, got: %q", joined)
	}

	m2 := &model{}
	m2.handleRemoteEvent(&agent.RemoteEvent{Kind: "command", Command: "/status"})
	joined2 := strings.Join(m2.transcript, "\n")
	if !strings.Contains(joined2, "📱") || !strings.Contains(joined2, "/status") {
		t.Fatalf("an allowed remote command should echo with the phone prefix, got: %q", joined2)
	}

	// An alias resolving to a blocked command (q -> quit) must also be caught,
	// not just the canonical name.
	m3 := &model{}
	m3.handleRemoteEvent(&agent.RemoteEvent{Kind: "command", Command: "/q"})
	joined3 := strings.Join(m3.transcript, "\n")
	if !strings.Contains(joined3, "blocked") {
		t.Fatalf("expected the quit alias /q to be blocked too, got: %q", joined3)
	}
}

// TestRemoteEventPresenceAndEnded covers the two non-command notifications a
// pairing relay can send: presence (the phone connecting/disconnecting) and
// ended (the session is over) — both update model state the header/welcome
// banner read, without running anything.
func TestRemoteEventPresenceAndEnded(t *testing.T) {
	m := &model{remoteCode: "ABCD-1234", remoteURL: "https://qr.example/ABCD-1234"}
	m.handleRemoteEvent(&agent.RemoteEvent{Kind: "presence", Connected: true})
	if !m.remoteConnected {
		t.Fatal("presence(connected=true) should set remoteConnected")
	}
	m.handleRemoteEvent(&agent.RemoteEvent{Kind: "presence", Connected: false})
	if m.remoteConnected {
		t.Fatal("presence(connected=false) should clear remoteConnected")
	}
	m.handleRemoteEvent(&agent.RemoteEvent{Kind: "ended"})
	if m.remoteCode != "" || m.remoteURL != "" || m.remoteConnected {
		t.Fatalf("ended should clear all pairing state, got code=%q url=%q connected=%v", m.remoteCode, m.remoteURL, m.remoteConnected)
	}
}

// TestCmdQRDispatch covers /qr's three-way branch: bare (pairing), "stop",
// and an argument (the original tunnel-URL QR), including the two guard
// cases that need no network access to observe (not signed in; nothing to
// stop).
func TestCmdQRDispatch(t *testing.T) {
	// Bare /qr with no account requires signing in first, and does not
	// attempt to reach the daemon.
	m := &model{}
	if cmd := cmdQR(m, nil); cmd != nil {
		t.Fatal("cmdQR with no signed-in account should not return a tea.Cmd")
	}
	if !strings.Contains(strings.Join(m.transcript, "\n"), "/login") {
		t.Fatalf("expected a sign-in prompt, got: %q", m.transcript)
	}

	// /qr stop with nothing active is a friendly no-op, not an error.
	m2 := &model{}
	if cmd := cmdQR(m2, []string{"stop"}); cmd != nil {
		t.Fatal("stopping with no active pairing should not return a tea.Cmd")
	}
	if !strings.Contains(strings.Join(m2.transcript, "\n"), "no active pairing") {
		t.Fatalf("expected a no-active-pairing message, got: %q", m2.transcript)
	}

	// /qr <id> with no matching tunnel keeps the original tunnel-QR error.
	m3 := &model{}
	if cmd := cmdQR(m3, []string{"bogus-id"}); cmd != nil {
		t.Fatal("cmdQR for an unknown tunnel id should not return a tea.Cmd")
	}
	if !strings.Contains(strings.Join(m3.transcript, "\n"), "no tunnel to show") {
		t.Fatalf("expected the tunnel-QR error, got: %q", m3.transcript)
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

	// /clear resets the transcript back to just the welcome banner, not a
	// blank screen.
	m = typeStr(m, "/clear")
	m = feed(m, tea.KeyMsg{Type: tea.KeyEnter})
	if m.welcomeLen == 0 || len(m.transcript) != m.welcomeLen {
		t.Fatalf("after /clear, transcript has %d lines (welcomeLen=%d), want just the welcome banner", len(m.transcript), m.welcomeLen)
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

// TestInputGrowsAndShrinks proves a long line grows the input box downward
// (wrapping) rather than scrolling it sideways, that the growth is capped at
// maxInputRows, and that it collapses back once the text is cleared — all
// while the overall view keeps filling the terminal exactly.
func TestInputGrowsAndShrinks(t *testing.T) {
	m := newModel(Options{Version: "test"}, newClient("127.0.0.1:0", ""))
	m = feed(m, tea.WindowSizeMsg{Width: 60, Height: 24})
	if h := m.input.Height(); h != 1 {
		t.Fatalf("empty input height = %d, want 1", h)
	}

	m = typeStr(m, strings.Repeat("wide ", 60)) // far more than one row at width 60
	if h := m.input.Height(); h <= 1 {
		t.Fatalf("input height after a long line = %d, want > 1", h)
	}
	if h := m.input.Height(); h > maxInputRows {
		t.Fatalf("input height = %d, exceeds maxInputRows %d", h, maxInputRows)
	}
	if lines := strings.Split(m.View(), "\n"); len(lines) != m.height {
		t.Fatalf("view has %d lines while grown, want %d", len(lines), m.height)
	}

	m.input.SetValue("")
	m.relayout()
	if h := m.input.Height(); h != 1 {
		t.Fatalf("input height after clearing = %d, want 1", h)
	}
	if lines := strings.Split(m.View(), "\n"); len(lines) != m.height {
		t.Fatalf("view has %d lines after clearing, want %d", len(lines), m.height)
	}
}

// TestInputGrowsForMultiLinePaste covers a clipboard paste that contains real
// newlines (textarea, unlike the old textinput, preserves them instead of
// collapsing them to spaces): LineInfo alone only wrap-counts the cursor's
// own logical line, so relayout must fall back to LineCount to size the box
// tall enough to show every pasted line.
func TestInputGrowsForMultiLinePaste(t *testing.T) {
	m := newModel(Options{Version: "test"}, newClient("127.0.0.1:0", ""))
	m = feed(m, tea.WindowSizeMsg{Width: 60, Height: 24})

	m.input.SetValue("first\nsecond\nthird")
	m.relayout()
	if h := m.input.Height(); h != 3 {
		t.Fatalf("input height after a 3-line paste = %d, want 3", h)
	}
	if lines := strings.Split(m.View(), "\n"); len(lines) != m.height {
		t.Fatalf("view has %d lines with a multi-line paste, want %d", len(lines), m.height)
	}

	// A paste taller than the cap still renders a valid, fully-filled frame.
	m.input.SetValue(strings.Repeat("line\n", maxInputRows+5))
	m.relayout()
	if h := m.input.Height(); h != maxInputRows {
		t.Fatalf("input height with an oversized paste = %d, want capped at %d", h, maxInputRows)
	}
	if lines := strings.Split(m.View(), "\n"); len(lines) != m.height {
		t.Fatalf("view has %d lines with an oversized paste, want %d", len(lines), m.height)
	}
}
