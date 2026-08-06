// Package tui implements trqsh's interactive terminal UI: a slash-command REPL
// (type `/` to browse commands), a scrolling transcript of what you've run, and
// pinnable live panels (`/pin traffic`) that stay on screen while you keep
// working — the same shape as Claude Code's console, driven entirely from the
// keyboard. It's a client of the local daemon's control API (see client.go), so
// tunnels it starts persist in the background daemon exactly like `trqsh http`.
package tui

import (
	"context"
	"fmt"
	"runtime"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
	"github.com/trqsh-uz/trqsh/internal/agent"
)

// Options configures a TUI session. cli.runTUI fills these from the loaded
// config after ensuring a daemon is up.
type Options struct {
	Addr       string // control-API address (127.0.0.1:4041)
	Token      string // loopback control-API token
	APIKey     string // stored API key, pushed into the daemon so /cloud/me works
	BaseDomain string // e.g. trqsh.uz (for display)
	Version    string // agent.Version, shown in the header

	// InitialCommand is the slash command to run on startup — the args the user
	// passed to `trqsh` (e.g. `trqsh http 3000` opens the console and runs it).
	InitialCommand string
	// StartSpecs are the tunnels defined in the config file, used by /start.
	StartSpecs []agent.TunnelSpec
	// ConfigRows are pre-rendered key/value pairs shown by /config, built by cli
	// so the tui package needn't know the config's internals.
	ConfigRows [][2]string

	// OnUpdate and OnUninstall bridge to package cli for the two operations that
	// touch the binary/filesystem (self-replace, local-data removal). They return
	// a summary line to print in the transcript. Nil disables the command.
	OnUpdate    func(context.Context) (string, error)
	OnUninstall func(context.Context) (string, error)
}

// reqRow is one line in the live traffic feed.
type reqRow struct {
	method string
	path   string
	status int
	ms     int64
	at     time.Time
}

type model struct {
	opts Options
	cl   *client

	width, height int
	ready         bool

	input textinput.Model
	vp    viewport.Model

	transcript []string // rendered lines (may contain ANSI), shown in the viewport

	// live data (kept fresh by polling + the SSE feed)
	status   agent.Status
	authed   bool
	acct     *account
	tunnels  []agent.Tunnel
	requests []reqRow

	pins []string // ordered, unique: "traffic" | "tunnels" | "status"

	// slash-command autocomplete menu
	menuItems []slashCmd
	menuIdx   int

	loginActive bool // a device-flow sign-in is polling
	initialRan  bool // the InitialCommand (if any) has been dispatched
	quitting    bool
}

// --- messages ---

type tickMsg time.Time
type dataMsg struct {
	tunnels []agent.Tunnel
	status  agent.Status
	ok      bool
}
type accountMsg struct {
	acct   *account
	authed bool
}
type eventMsg agent.Event
type printMsg struct{ lines []string } // append lines to the transcript
type loginStartedMsg struct{ d *deviceStart }
type loginTickMsg struct {
	deviceCode string
	interval   time.Duration
}
type loginDoneMsg struct {
	ok  bool
	msg string
}

// Run starts the TUI and blocks until the user quits.
func Run(opts Options) error {
	cl := newClient(opts.Addr, opts.Token)
	p := tea.NewProgram(newModel(opts, cl), tea.WithAltScreen())

	// Fan the daemon's SSE feed into the Bubble Tea message loop. Canceling ctx
	// on return unblocks the reader so the goroutine doesn't leak past quit.
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go cl.streamEvents(ctx, func(ev agent.Event) { p.Send(eventMsg(ev)) })

	_, err := p.Run()
	return err
}

func newModel(opts Options, cl *client) model {
	ti := textinput.New()
	ti.Prompt = stBrand.Render(" › ")
	ti.Placeholder = "type / for commands"
	ti.CharLimit = 240
	ti.Focus()

	m := model{opts: opts, cl: cl, input: ti, vp: viewport.New(0, 0)}
	m.transcript = []string{
		"  " + stBrand.Render("trqsh") + " " + stDim.Render(opts.Version) + stDim.Render("  — interactive console"),
		"  " + stDim.Render("Type ") + stKey.Render("/help") + stDim.Render(" for commands, or ") + stKey.Render("/") + stDim.Render(" to browse. ") + stKey.Render("/pin traffic") + stDim.Render(" keeps requests on screen."),
		"",
	}
	return m
}

func (m model) Init() tea.Cmd {
	return tea.Batch(textinput.Blink, m.poll(), m.fetchAccount(), tickCmd())
}

// --- commands (async work) ---

func tickCmd() tea.Cmd {
	return tea.Tick(1500*time.Millisecond, func(t time.Time) tea.Msg { return tickMsg(t) })
}

func (m model) poll() tea.Cmd {
	cl := m.cl
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		ts, err := cl.tunnels(ctx)
		st, _ := cl.status(ctx)
		return dataMsg{tunnels: ts, status: st, ok: err == nil}
	}
}

func (m model) fetchAccount() tea.Cmd {
	cl, key := m.cl, m.opts.APIKey
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
		defer cancel()
		if key != "" {
			_ = cl.login(ctx, key) // make sure the daemon can reach the cloud as us
		}
		a, err := cl.me(ctx)
		return accountMsg{acct: a, authed: err == nil}
	}
}

func (m model) pollLogin(deviceCode string, interval time.Duration) tea.Cmd {
	cl := m.cl
	return func() tea.Msg {
		time.Sleep(interval)
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		authed, pending, err := cl.oauthPoll(ctx, deviceCode)
		switch {
		case err != nil:
			return loginDoneMsg{ok: false, msg: err.Error()}
		case authed:
			return loginDoneMsg{ok: true}
		default:
			_ = pending
			return loginTickMsg{deviceCode: deviceCode, interval: interval}
		}
	}
}

// --- update loop ---

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		m.ready = true
		m.relayout()
		// Run the command the user typed after `trqsh` (e.g. `trqsh http 3000`),
		// now that the viewport is sized so its output lands in the transcript.
		if !m.initialRan && strings.TrimSpace(m.opts.InitialCommand) != "" {
			m.initialRan = true
			return m.run(m.opts.InitialCommand)
		}
		return m, nil

	case tea.KeyMsg:
		return m.handleKey(msg)

	case tickMsg:
		cmds = append(cmds, m.poll(), tickCmd())

	case dataMsg:
		if msg.ok {
			m.tunnels = msg.tunnels
		}
		m.status = msg.status
		m.relayout()

	case accountMsg:
		m.acct, m.authed = msg.acct, msg.authed
		m.relayout()

	case eventMsg:
		m.handleEvent(agent.Event(msg))
		m.relayout()

	case printMsg:
		m.appendLines(msg.lines...)

	case loginStartedMsg:
		m.loginActive = true
		url := firstNonEmpty(msg.d.VerificationURIComplete, msg.d.VerificationURI)
		m.appendLines(
			"  "+stTitle.Render("Sign in"),
			"  "+stDim.Render("open ")+stURL.Render(url),
			"  "+stDim.Render("code ")+stKey.Render(msg.d.UserCode),
			stDim.Render("  waiting for approval… (keep using other commands)"),
		)
		_ = openBrowser(url)
		interval := time.Duration(max(msg.d.Interval, 2)) * time.Second
		cmds = append(cmds, m.pollLogin(msg.d.DeviceCode, interval))

	case loginTickMsg:
		cmds = append(cmds, m.pollLogin(msg.deviceCode, msg.interval))

	case loginDoneMsg:
		m.loginActive = false
		if msg.ok {
			m.appendLines(stOK.Render("  ✓ signed in"))
			cmds = append(cmds, m.fetchAccount())
		} else {
			m.appendLines(stErr.Render("  ✗ sign-in failed: " + msg.msg))
		}
	}

	// Keep the input caret blinking and the viewport responsive to mouse wheel.
	var c tea.Cmd
	m.input, c = m.input.Update(msg)
	cmds = append(cmds, c)
	m.vp, c = m.vp.Update(msg)
	cmds = append(cmds, c)
	return m, tea.Batch(cmds...)
}

func (m model) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	menuOpen := m.menuOpen()

	switch msg.Type {
	case tea.KeyCtrlC:
		m.quitting = true
		return m, tea.Quit
	case tea.KeyPgUp:
		m.vp.HalfPageUp()
		return m, nil
	case tea.KeyPgDown:
		m.vp.HalfPageDown()
		return m, nil
	}

	switch msg.String() {
	case "up", "ctrl+p":
		if menuOpen {
			if m.menuIdx > 0 {
				m.menuIdx--
			}
		} else {
			m.vp.ScrollUp(1)
		}
		return m, nil
	case "down", "ctrl+n":
		if menuOpen {
			if m.menuIdx < len(m.menuItems)-1 {
				m.menuIdx++
			}
		} else {
			m.vp.ScrollDown(1)
		}
		return m, nil
	case "tab":
		if menuOpen {
			m.acceptMenu()
			m.relayout()
			return m, nil
		}
	case "esc":
		if menuOpen {
			m.input.SetValue("")
			m.refreshMenu()
			m.relayout()
			return m, nil
		}
	case "enter":
		return m.submit()
	}

	// Ordinary editing: let the text input handle the key, then recompute the
	// autocomplete menu from the new value and re-flow the layout.
	var c tea.Cmd
	m.input, c = m.input.Update(msg)
	m.refreshMenu()
	m.relayout()
	return m, c
}

// submit runs the current input line, or — when the command menu is open —
// accepts the highlighted command (running it if it needs no arguments,
// otherwise completing it so the user can add them).
func (m model) submit() (tea.Model, tea.Cmd) {
	if m.menuOpen() {
		sel := m.menuItems[m.menuIdx]
		if sel.args != "" {
			m.input.SetValue("/" + sel.name + " ")
			m.input.CursorEnd()
			m.refreshMenu()
			m.relayout()
			return m, nil
		}
		m.input.SetValue("/" + sel.name)
	}
	line := strings.TrimSpace(m.input.Value())
	m.input.SetValue("")
	m.refreshMenu()
	if line == "" {
		m.relayout()
		return m, nil
	}
	return m.run(line)
}

func (m model) run(line string) (tea.Model, tea.Cmd) {
	m.appendLines(stDim.Render("  › ") + line)
	name, args := parseLine(line)
	cmd, ok := lookupCommand(name)
	if !ok {
		m.appendLines(stErr.Render("  ✗ unknown command: "+name) + stDim.Render("   (type /help)"))
		m.relayout()
		return m, nil
	}
	c := cmd.run(&m, args)
	m.relayout()
	return m, c
}

// --- autocomplete menu ---

func (m model) menuOpen() bool { return len(m.menuItems) > 0 }

func (m *model) refreshMenu() {
	m.menuItems = nil
	m.menuIdx = 0
	v := m.input.Value()
	if !strings.HasPrefix(v, "/") || strings.Contains(v, " ") {
		return
	}
	prefix := strings.ToLower(strings.TrimPrefix(v, "/"))
	for _, c := range slashCommands {
		if strings.HasPrefix(c.name, prefix) {
			m.menuItems = append(m.menuItems, c)
		}
	}
}

func (m *model) acceptMenu() {
	if !m.menuOpen() {
		return
	}
	m.input.SetValue("/" + m.menuItems[m.menuIdx].name + " ")
	m.input.CursorEnd()
	m.refreshMenu()
}

// --- live event handling ---

func (m *model) handleEvent(ev agent.Event) {
	switch ev.Type {
	case "request":
		if ev.Request != nil {
			m.requests = append(m.requests, reqRow{
				method: ev.Request.Method, path: ev.Request.Path,
				status: ev.Request.Status, ms: ev.Request.DurationMs, at: time.Now(),
			})
			if len(m.requests) > 500 {
				m.requests = m.requests[len(m.requests)-500:]
			}
		}
	case "status":
		if ev.Status != nil {
			m.status = *ev.Status
		}
	case "error":
		if ev.Err != "" {
			m.appendLines(stWarn.Render("  ⚠ " + ev.Err))
		}
	}
}

// appendLines adds rendered lines to the transcript and follows to the bottom.
func (m *model) appendLines(lines ...string) {
	m.transcript = append(m.transcript, lines...)
	if m.width > 0 {
		m.vp.SetContent(strings.Join(m.transcript, "\n"))
		m.vp.GotoBottom()
	}
}

// relayout re-sizes the viewport to whatever space the header, pinned panels,
// autocomplete menu, and input line leave, preserving the reader's scroll
// position unless they were already following the tail.
func (m *model) relayout() {
	if m.width == 0 {
		return
	}
	pinnedH := 0
	if ph := len(m.renderPinned()); ph > 0 {
		pinnedH = ph + 1 // panels + a divider under them
	}
	menuH := len(m.renderMenu())
	const headerH, inputH = 2, 2
	vpH := max(m.height-headerH-inputH-pinnedH-menuH, 1)
	atBottom := m.vp.AtBottom()
	m.vp.Width = m.width
	m.vp.Height = vpH
	m.vp.SetContent(strings.Join(m.transcript, "\n"))
	if atBottom {
		m.vp.GotoBottom()
	}
	m.input.Width = max(10, m.width-6)
}

// --- rendering ---

func (m model) View() string {
	if !m.ready {
		return "  starting trqsh…"
	}
	var lines []string
	lines = append(lines, m.renderHeader())
	lines = append(lines, stDim.Render(m.rule()))
	if pinned := m.renderPinned(); len(pinned) > 0 {
		lines = append(lines, pinned...)
		lines = append(lines, stDim.Render(m.rule()))
	}
	lines = append(lines, strings.Split(m.vp.View(), "\n")...)
	if menu := m.renderMenu(); len(menu) > 0 {
		lines = append(lines, menu...)
	}
	lines = append(lines, m.renderInputLines()...)

	// Guarantee exactly m.height lines and never overflow the width (which would
	// wrap and throw the whole layout off).
	for i := range lines {
		lines[i] = m.clamp(lines[i])
	}
	if len(lines) > m.height {
		lines = lines[len(lines)-m.height:]
	}
	for len(lines) < m.height {
		lines = append(lines, "")
	}
	return strings.Join(lines, "\n")
}

func (m model) rule() string { return strings.Repeat("─", max(0, m.width)) }

func (m model) clamp(s string) string {
	if lipgloss.Width(s) <= m.width {
		return s
	}
	return ansi.Truncate(s, m.width, "…")
}

func (m model) spread(left, right string) string {
	gap := max(m.width-lipgloss.Width(left)-lipgloss.Width(right)-1, 1)
	return left + strings.Repeat(" ", gap) + right + " "
}

func (m model) renderHeader() string {
	left := stBrand.Render(" trqsh") + " " + stDim.Render(m.opts.Version)
	glyph, word, col := m.connState()
	right := lipgloss.NewStyle().Foreground(col).Render(glyph + " " + word)
	if meta := m.headerMeta(); meta != "" {
		right += stDim.Render("  " + meta)
	}
	return m.spread(left, right)
}

func (m model) connState() (glyph, word string, col lipgloss.Color) {
	switch {
	case len(m.tunnels) > 0 && m.status.Connected:
		return "●", "online", colGreen
	case len(m.tunnels) > 0:
		return "●", "connecting", colYellow
	default:
		return "○", "idle", colDim
	}
}

func (m model) headerMeta() string {
	who := "guest"
	switch {
	case m.acct != nil:
		who = firstNonEmpty(m.acct.Plan, m.acct.Org.Plan, "free") + " plan"
	case m.authed:
		who = "signed in"
	}
	return fmt.Sprintf("%s · %d tunnels", who, len(m.tunnels))
}

func (m model) renderPinned() []string {
	var out []string
	for _, p := range m.pins {
		if len(out) > 0 {
			out = append(out, "") // a blank line between stacked panels
		}
		switch p {
		case "tunnels":
			out = append(out, m.pinTunnels()...)
		case "traffic":
			out = append(out, m.pinTraffic()...)
		case "status":
			out = append(out, m.pinStatus()...)
		}
	}
	return out
}

func (m model) pinTunnels() []string {
	lines := []string{"  " + stTitle.Render("TUNNELS") + stDim.Render(fmt.Sprintf("  (%d)", len(m.tunnels)))}
	if len(m.tunnels) == 0 {
		return append(lines, "  "+stDim.Render("none — /http <port> to start one"))
	}
	const cap = 6
	for i, t := range m.tunnels {
		if i >= cap {
			lines = append(lines, "  "+stDim.Render(fmt.Sprintf("…and %d more", len(m.tunnels)-cap)))
			break
		}
		left := "  " + stURL.Render(t.PublicURL) + stDim.Render(" → "+t.LocalAddr)
		right := stDim.Render(fmt.Sprintf("%s · %s · %d req", t.Status, uptime(t.CreatedAt), t.Metrics.Requests))
		lines = append(lines, m.spread(left, right))
	}
	return lines
}

func (m model) pinTraffic() []string {
	lines := []string{"  " + stTitle.Render("TRAFFIC") + stDim.Render("  (live)")}
	if len(m.requests) == 0 {
		return append(lines, "  "+stDim.Render("waiting for requests…"))
	}
	const cap = 8
	start := max(0, len(m.requests)-cap)
	for _, r := range m.requests[start:] {
		lines = append(lines, "  "+m.renderReq(r))
	}
	return lines
}

func (m model) pinStatus() []string {
	glyph, word, col := m.connState()
	line := "  " + stTitle.Render("STATUS") + "   " +
		lipgloss.NewStyle().Foreground(col).Render(glyph+" "+word) +
		stDim.Render("   edge "+firstNonEmpty(m.status.Edge, "—")+" · "+firstNonEmpty(m.status.Kind, "—"))
	return []string{line}
}

func (m model) renderReq(r reqRow) string {
	method := methodStyle(r.method).Render(pad(r.method, 6)) // widest common verb is DELETE
	status := statusStyle(r.status).Render(fmt.Sprintf("%3d", r.status))
	dur := stDim.Render(fmt.Sprintf("%5dms", r.ms))
	pathW := max(10, m.width-24)
	path := pad(trunc(r.path, pathW), pathW)
	return fmt.Sprintf("%s %s  %s  %s", method, status, path, dur)
}

func (m model) renderMenu() []string {
	if !m.menuOpen() {
		return nil
	}
	const window = 8
	start := 0
	if m.menuIdx >= window {
		start = m.menuIdx - window + 1
	}
	end := min(start+window, len(m.menuItems))
	var out []string
	for i := start; i < end; i++ {
		c := m.menuItems[i]
		label := pad("/"+c.name+" "+c.args, 26)
		if i == m.menuIdx {
			out = append(out, stMenuSel.Render(" "+label+" ")+"  "+stDim.Render(c.desc))
		} else {
			out = append(out, " "+stKey.Render("/"+c.name)+stDim.Render(pad(" "+c.args, 26-len("/"+c.name)))+"  "+stDim.Render(c.desc))
		}
	}
	return out
}

func (m model) renderInputLines() []string {
	var hint string
	if m.menuOpen() {
		hint = "  " + stDim.Render("↑↓ select · tab complete · enter run · esc clear")
	} else {
		hint = "  " + stDim.Render("/ commands · enter run · pgup/pgdn scroll · ctrl+c quit")
	}
	return []string{m.input.View(), hint}
}

// firstNonEmpty returns the first non-blank value, or "".
func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if strings.TrimSpace(v) != "" {
			return v
		}
	}
	return ""
}

// runtimePlatform is a tiny indirection so /version stays testable/readable.
func runtimePlatform() string { return runtime.GOOS + "/" + runtime.GOARCH }
