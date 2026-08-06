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

	// autocomplete menu (command names, or argument options for /pin, /unpin)
	menuItems []menuEntry
	menuIdx   int

	// command history: up/down recalls previously entered lines to edit.
	history    []string
	historyIdx int // cursor into history; == len(history) means "not browsing"

	welcomeShown bool // the welcome banner has been placed in the transcript
	welcomeLen   int  // how many leading transcript lines the banner occupies
	loginActive  bool // a device-flow sign-in is polling
	initialRan   bool // the InitialCommand (if any) has been dispatched
	quitting     bool
}

// menuEntry is one row of the autocomplete popup. insert is what replaces the
// input when it's accepted; run means Enter should execute it immediately
// (no-argument commands and fully-chosen argument values) rather than just
// completing the text so the user can keep typing.
type menuEntry struct {
	insert string
	label  string
	desc   string
	run    bool
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
	// WithMouseCellMotion makes the wheel arrive as mouse events the viewport
	// scrolls with, instead of the terminal translating it to ↑/↓ key presses
	// (which would drive command history). Hold Shift to select/copy text.
	p := tea.NewProgram(newModel(opts, cl), tea.WithAltScreen(), tea.WithMouseCellMotion())

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

	vp := viewport.New(0, 0)
	vp.MouseWheelEnabled = true

	// The welcome banner is placed on the first WindowSizeMsg, once the width is
	// known to size its box to.
	return model{opts: opts, cl: cl, input: ti, vp: vp}
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
		if !m.welcomeShown {
			m.welcomeShown = true
			w := m.renderWelcome()
			m.welcomeLen = len(w)
			m.transcript = append(w, m.transcript...)
			m.relayout()
			m.vp.GotoTop() // show the welcome from its top border, not the tail
		} else {
			m.relayout()
		}
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
		m.refreshWelcome()
		m.relayout()

	case accountMsg:
		m.acct, m.authed = msg.acct, msg.authed
		m.refreshWelcome()
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
	case tea.KeyEnd: // jump to the latest output (the "scroll to bottom" affordance)
		m.vp.GotoBottom()
		return m, nil
	}

	switch msg.String() {
	case "up", "ctrl+p":
		if menuOpen {
			if m.menuIdx > 0 {
				m.menuIdx--
			}
			return m, nil
		}
		m.historyPrev() // recall an earlier command to edit
		m.relayout()
		return m, nil
	case "down", "ctrl+n":
		if menuOpen {
			if m.menuIdx < len(m.menuItems)-1 {
				m.menuIdx++
			}
			return m, nil
		}
		m.historyNext()
		m.relayout()
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

	// Ordinary editing: let the text input handle the key, reset the history
	// browse cursor to the end, recompute the menu, and re-flow the layout.
	var c tea.Cmd
	m.input, c = m.input.Update(msg)
	m.historyIdx = len(m.history)
	m.refreshMenu()
	m.relayout()
	return m, c
}

// historyPrev recalls the previous command into the input (older).
func (m *model) historyPrev() {
	if len(m.history) == 0 {
		return
	}
	if m.historyIdx > 0 {
		m.historyIdx--
	}
	m.input.SetValue(m.history[m.historyIdx])
	m.input.CursorEnd()
	m.refreshMenu()
}

// historyNext moves toward newer commands, clearing the input past the newest.
func (m *model) historyNext() {
	if len(m.history) == 0 {
		return
	}
	if m.historyIdx < len(m.history)-1 {
		m.historyIdx++
		m.input.SetValue(m.history[m.historyIdx])
	} else {
		m.historyIdx = len(m.history)
		m.input.SetValue("")
	}
	m.input.CursorEnd()
	m.refreshMenu()
}

// submit runs the current input line, or — when the command menu is open —
// accepts the highlighted command (running it if it needs no arguments,
// otherwise completing it so the user can add them).
func (m model) submit() (tea.Model, tea.Cmd) {
	if m.menuOpen() {
		sel := m.menuItems[m.menuIdx]
		m.input.SetValue(sel.insert)
		m.input.CursorEnd()
		if !sel.run { // a command that still needs arguments: complete, keep editing
			m.refreshMenu()
			m.relayout()
			return m, nil
		}
	}
	line := strings.TrimSpace(m.input.Value())
	m.input.SetValue("")
	m.refreshMenu()
	if line == "" {
		m.relayout()
		return m, nil
	}
	m.pushHistory(line)
	return m.run(line)
}

// pushHistory records a submitted line for up/down recall, skipping consecutive
// duplicates, and resets the browse cursor to the end.
func (m *model) pushHistory(line string) {
	if n := len(m.history); n == 0 || m.history[n-1] != line {
		m.history = append(m.history, line)
	}
	m.historyIdx = len(m.history)
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
	if !strings.HasPrefix(v, "/") {
		return
	}
	fields := strings.Fields(v[1:])
	trailingSpace := strings.HasSuffix(v, " ")

	// Still choosing the command name (one partial token, no space yet).
	if len(fields) <= 1 && !trailingSpace {
		prefix := ""
		if len(fields) == 1 {
			prefix = strings.ToLower(fields[0])
		}
		for _, c := range slashCommands {
			if strings.HasPrefix(c.name, prefix) {
				insert, run := "/"+c.name, c.args == ""
				label := "/" + c.name
				if c.args != "" {
					insert += " "
					label += " " + c.args
				}
				m.menuItems = append(m.menuItems, menuEntry{insert: insert, label: label, desc: c.desc, run: run})
			}
		}
		return
	}

	// Choosing an argument value for a command that offers fixed choices, so the
	// user can arrow-select "traffic" instead of typing it.
	name := strings.ToLower(fields[0])
	argPrefix := ""
	if !trailingSpace && len(fields) >= 2 {
		argPrefix = strings.ToLower(fields[len(fields)-1])
	}
	for _, opt := range m.argOptions(name) {
		if strings.HasPrefix(opt, argPrefix) {
			m.menuItems = append(m.menuItems, menuEntry{insert: "/" + name + " " + opt, label: "/" + name + " " + opt, run: true})
		}
	}
}

// argOptions returns the selectable argument values for a command (empty for
// commands whose arguments are free-form, like a port number).
func (m *model) argOptions(name string) []string {
	switch name {
	case "pin":
		return []string{"traffic", "tunnels", "status"}
	case "unpin":
		return append(append([]string{}, m.pins...), "all")
	}
	return nil
}

func (m *model) acceptMenu() {
	if !m.menuOpen() {
		return
	}
	m.input.SetValue(m.menuItems[m.menuIdx].insert)
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
	// Fixed chrome around the scrolling transcript: header(1)+rule(1), the pinned
	// panels, a scroll-hint line, the autocomplete menu, the 3-line input box, a
	// hint line, and a bottom gap so the box isn't glued to the terminal edge.
	pinnedH := len(m.renderPinnedBlock())
	menuH := len(m.renderMenu())
	chrome := 2 + pinnedH + 1 + menuH + 3 + 1 + 1
	vpH := max(m.height-chrome, 1)
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
	lines = append(lines, strings.Split(m.vp.View(), "\n")...) // transcript (welcome banner first)
	lines = append(lines, m.renderScrollHint())                // empty when at the bottom
	lines = append(lines, m.renderPinnedBlock()...)            // pinned panels sit just above the prompt
	lines = append(lines, m.renderMenu()...)
	lines = append(lines, strings.Split(m.renderInputBox(), "\n")...)
	lines = append(lines, m.renderHint())
	lines = append(lines, "") // bottom gap so the input box floats off the edge

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
	return who + " · " + plural(len(m.tunnels), "tunnel")
}

// plural formats a count with its noun, adding an "s" for anything but 1.
func plural(n int, noun string) string {
	if n == 1 {
		return "1 " + noun
	}
	return fmt.Sprintf("%d %ss", n, noun)
}

// pinnedRows is how many body lines each pinned panel shows — shared so panels
// in a row share a height and their bottom edges line up.
const pinnedRows = 4

// renderPinnedBlock lays the pinned panels out as boxes from left to right,
// wrapping to a new row when they'd overflow the width.
func (m model) renderPinnedBlock() []string {
	if len(m.pins) == 0 {
		return nil
	}
	boxes := make([]string, 0, len(m.pins))
	for _, p := range m.pins {
		boxes = append(boxes, m.pinBox(p))
	}
	return packBoxes(boxes, m.width)
}

func (m model) pinBox(name string) string {
	switch name {
	case "tunnels":
		w := 40
		return panelBox(fmt.Sprintf("TUNNELS (%d)", len(m.tunnels)),
			strings.Join(fitLines(m.tunnelsBody(w-4), pinnedRows, w-4), "\n"), w)
	case "traffic":
		w := 54
		return panelBox("TRAFFIC",
			strings.Join(fitLines(m.trafficBody(w-4), pinnedRows, w-4), "\n"), w)
	case "status":
		w := 40
		return panelBox("STATUS",
			strings.Join(fitLines(m.statusBody(), pinnedRows, w-4), "\n"), w)
	}
	return ""
}

func (m model) tunnelsBody(w int) []string {
	if len(m.tunnels) == 0 {
		return []string{stDim.Render("none — /http <port>")}
	}
	var out []string
	for _, t := range m.tunnels {
		out = append(out, stURL.Render(trunc(t.PublicURL, w-6))+stDim.Render(fmt.Sprintf("  %dr", t.Metrics.Requests)))
	}
	return out
}

func (m model) trafficBody(w int) []string {
	if len(m.requests) == 0 {
		return []string{stDim.Render("waiting for requests…")}
	}
	start := max(0, len(m.requests)-pinnedRows)
	var out []string
	for _, r := range m.requests[start:] {
		method := methodStyle(r.method).Render(pad(r.method, 6))
		status := statusStyle(r.status).Render(fmt.Sprintf("%3d", r.status))
		out = append(out, fmt.Sprintf("%s %s %s", method, status, trunc(r.path, max(4, w-11))))
	}
	return out
}

func (m model) statusBody() []string {
	glyph, word, col := m.connState()
	return []string{
		lipglossFg(col, glyph+" "+word),
		stDim.Render("edge " + firstNonEmpty(m.status.Edge, "—")),
		stDim.Render(firstNonEmpty(m.status.Kind, "—") + " · " + plural(len(m.tunnels), "tunnel")),
	}
}

// packBoxes joins rendered boxes left-to-right with a one-column gap, wrapping
// to a new row when the next box wouldn't fit in width.
func packBoxes(boxes []string, width int) []string {
	if len(boxes) == 0 {
		return nil
	}
	gap := strings.TrimRight(strings.Repeat(" \n", lipgloss.Height(boxes[0])), "\n") // a full-height 1-col spacer
	var out, row []string
	rowW := 0
	flush := func() {
		if len(row) > 0 {
			out = append(out, strings.Split(lipgloss.JoinHorizontal(lipgloss.Top, row...), "\n")...)
			row, rowW = nil, 0
		}
	}
	for _, b := range boxes {
		bw := lipgloss.Width(b)
		if rowW > 0 && rowW+1+bw > width {
			flush()
		}
		if rowW > 0 {
			row = append(row, gap)
			rowW++
		}
		row = append(row, b)
		rowW += bw
	}
	flush()
	return out
}

// --- welcome banner, input box, hints ---

// trqshMascot is the console's little critter, shown in the welcome banner
// (like Claude Code's robot). ASCII-only so it renders in any terminal font.
var trqshMascot = []string{
	" /\\_/\\ ",
	"( o.o )",
	" > ~ < ",
}

// renderWelcome builds the startup banner: a rounded box (title in its border)
// with three columns — the mascot, the account/status, and getting-started tips.
func (m model) renderWelcome() []string {
	mascot := colorLines(trqshMascot, colBrand)
	acct := m.welcomeLeft()
	tips := m.welcomeRight()
	rows := max(max(len(mascot), len(acct)), len(tips))
	const mascotW, acctW = 9, 24
	sep := stDim.Render("│ ")
	body := make([]string, 0, rows)
	for i := range rows {
		body = append(body, padCell(at(mascot, i), mascotW)+" "+padCell(at(acct, i), acctW)+sep+at(tips, i))
	}
	box := roundedBox("trqsh "+m.opts.Version, strings.Join(body, "\n"), m.width-2, colBrand, "")
	return append(strings.Split(box, "\n"), "")
}

// colorLines applies a foreground color to each line (for the mascot art).
func colorLines(lines []string, c lipgloss.Color) []string {
	out := make([]string, len(lines))
	for i, l := range lines {
		out[i] = lipglossFg(c, l)
	}
	return out
}

// at returns sl[i], or "" when out of range — for zipping ragged columns.
func at(sl []string, i int) string {
	if i >= 0 && i < len(sl) {
		return sl[i]
	}
	return ""
}

func (m model) welcomeLeft() []string {
	glyph, word, col := m.connState()
	name := "guest"
	email := stDim.Render("not signed in — /login")
	plan := ""
	if m.acct != nil {
		name = firstNonEmpty(m.acct.User.Name, m.acct.User.Email, "you")
		email = stDim.Render(m.acct.User.Email)
		plan = firstNonEmpty(m.acct.Plan, m.acct.Org.Plan, "free") + " plan · "
	}
	return []string{
		stBrand.Render("Welcome, " + name),
		email,
		stDim.Render(plan) + lipglossFg(col, glyph+" "+word),
		"",
	}
}

// refreshWelcome re-renders the banner in place so it reflects the account and
// connection once they load — as long as it's still at the top of the transcript
// (it won't be after /clear, where welcomeLen is reset to 0).
func (m *model) refreshWelcome() {
	if !m.welcomeShown || m.welcomeLen == 0 || len(m.transcript) < m.welcomeLen {
		return
	}
	fresh := m.renderWelcome()
	m.transcript = append(fresh, m.transcript[m.welcomeLen:]...)
	m.welcomeLen = len(fresh)
}

func (m model) welcomeRight() []string {
	return []string{
		stTitle.Render("Getting started"),
		stKey.Render("/") + stDim.Render("            browse all commands"),
		stKey.Render("/http 3000") + stDim.Render("   expose a local port"),
		stKey.Render("/qr") + stDim.Render("          QR to open on your phone"),
	}
}

// padCell truncates/pads a (possibly styled) string to exactly w display cells.
func padCell(s string, w int) string {
	if lipgloss.Width(s) > w {
		return ansi.Truncate(s, w, "…")
	}
	return s + strings.Repeat(" ", w-lipgloss.Width(s))
}

func (m model) renderInputBox() string {
	return inputBox(m.input.View(), m.width)
}

func (m model) renderScrollHint() string {
	if m.vp.AtBottom() {
		return ""
	}
	return "  " + stWarn.Render("↓ more below") + stDim.Render("  ·  End to jump to latest, PgDn to scroll")
}

func (m model) renderHint() string {
	if m.menuOpen() {
		return "  " + stDim.Render("↑↓ select · tab complete · enter run · esc close")
	}
	return "  " + stDim.Render("↑↓ history · / commands · pgup/pgdn scroll · ctrl+c quit")
}

func (m model) renderMenu() []string {
	if !m.menuOpen() {
		return nil
	}
	const window = 7
	start := 0
	if m.menuIdx >= window {
		start = m.menuIdx - window + 1
	}
	end := min(start+window, len(m.menuItems))
	var out []string
	for i := start; i < end; i++ {
		e := m.menuItems[i]
		label := pad(e.label, 26)
		if i == m.menuIdx {
			row := stMenuSel.Render(" " + label + " ")
			if e.desc != "" {
				row += "  " + stDim.Render(e.desc)
			}
			out = append(out, "  "+row)
		} else {
			row := "  " + stKey.Render(label)
			if e.desc != "" {
				row += "  " + stDim.Render(e.desc)
			}
			out = append(out, "  "+row)
		}
	}
	return out
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
