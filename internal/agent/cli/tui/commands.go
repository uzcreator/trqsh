package tui

// This file is the slash-command registry — the "strong logic" behind the
// console. Each command has a name, an argument hint (shown in the autocomplete
// menu and used to decide whether Enter runs it or completes it), a one-line
// description, and a handler. Handlers get *model so they can print to the
// transcript synchronously and/or return a tea.Cmd for async work (API calls).

import (
	"bytes"
	"context"
	"fmt"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/atotto/clipboard"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/mdp/qrterminal/v3"
	"github.com/trqsh-uz/trqsh/internal/agent"
)

type slashCmd struct {
	name string
	args string // usage hint; non-empty means "takes arguments"
	desc string
	run  func(m *model, args []string) tea.Cmd
}

// slashCommands is the ordered command list (also the order shown in /help and
// the autocomplete menu). It's populated in init rather than as a static
// initializer because cmdHelp reads it back, which the compiler would otherwise
// flag as an initialization cycle.
var slashCommands []slashCmd

func init() {
	slashCommands = []slashCmd{
		{"http", "<port>", "Expose a local HTTP port", cmdHTTP},
		{"tcp", "<port>", "Expose a local TCP port", cmdTCP},
		{"udp", "<port>", "Expose a local UDP port", cmdUDP},
		{"start", "", "Start tunnels from your config", cmdStart},
		{"ls", "", "List running tunnels", cmdLs},
		{"open", "<id>", "Open a tunnel URL in your browser", cmdOpen},
		{"qr", "[id]", "Show a QR code for a tunnel URL", cmdQR},
		{"copy", "[id]", "Copy a tunnel URL to the clipboard", cmdCopy},
		{"stop", "<id|all>", "Stop a tunnel (or all)", cmdStop},
		{"down", "", "Stop the background daemon and quit", cmdDown},
		{"requests", "", "Show recently captured requests", cmdRequests},
		{"pin", "<traffic|tunnels|status>", "Keep a live panel on screen", cmdPin},
		{"unpin", "<name|all>", "Remove a pinned panel", cmdUnpin},
		{"status", "", "Show connection status", cmdStatus},
		{"whoami", "", "Show account, plan and usage", cmdWhoami},
		{"login", "[key]", "Sign in via browser (or paste a key)", cmdLogin},
		{"logout", "", "Sign out", cmdLogout},
		{"subdomains", "[reserve|release <name>]", "Manage reserved subdomains", cmdSubdomains},
		{"domains", "[add|verify <domain>]", "Manage custom domains", cmdDomains},
		{"config", "", "Show configuration", cmdConfig},
		{"update", "[check]", "Check for / install a newer trqsh", cmdUpdate},
		{"uninstall", "", "Remove trqsh's local data", cmdUninstall},
		{"version", "", "Show version", cmdVersion},
		{"clear", "", "Clear the transcript", cmdClear},
		{"help", "", "Show all commands", cmdHelp},
		{"quit", "", "Exit the console", cmdQuit},
	}

}

// lookupCommand resolves a name (or a short alias) to a command.
func lookupCommand(name string) (slashCmd, bool) {
	for _, c := range slashCommands {
		if c.name == name {
			return c, true
		}
	}
	switch name {
	case "q", "exit":
		return lookupCommand("quit")
	case "list", "ps":
		return lookupCommand("ls")
	case "account", "me":
		return lookupCommand("whoami")
	case "subs", "subdomain":
		return lookupCommand("subdomains")
	case "domain":
		return lookupCommand("domains")
	case "?":
		return lookupCommand("help")
	}
	return slashCmd{}, false
}

// parseLine splits "/http 3000 --subdomain x" into ("http", ["3000","--subdomain","x"]).
func parseLine(line string) (string, []string) {
	fields := strings.Fields(strings.TrimPrefix(strings.TrimSpace(line), "/"))
	if len(fields) == 0 {
		return "", nil
	}
	return strings.ToLower(fields[0]), fields[1:]
}

// --- tunnel commands ---

func cmdHTTP(m *model, a []string) tea.Cmd { return startTunnelCmd(m, "http", a) }
func cmdTCP(m *model, a []string) tea.Cmd  { return startTunnelCmd(m, "tcp", a) }
func cmdUDP(m *model, a []string) tea.Cmd  { return startTunnelCmd(m, "udp", a) }

func startTunnelCmd(m *model, proto string, args []string) tea.Cmd {
	if len(args) == 0 {
		m.appendLines(stErr.Render("  ✗ usage: ") + stKey.Render("/"+proto+" <port>"))
		return nil
	}
	spec := agent.TunnelSpec{Proto: proto, Addr: normalizeAddr(args[0])}
	for i := 1; i < len(args); i++ { // a couple of familiar flags
		switch args[i] {
		case "--subdomain", "-s":
			if i+1 < len(args) {
				spec.Subdomain = args[i+1]
				i++
			}
		case "--remote-port":
			if i+1 < len(args) {
				spec.RemotePort, _ = strconv.Atoi(args[i+1])
				i++
			}
		}
	}
	m.appendLines(stDim.Render("  starting " + proto + " tunnel → " + spec.Addr + " …"))
	cl := m.cl
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		t, err := cl.startTunnel(ctx, spec)
		if err != nil {
			return printMsg{[]string{stErr.Render("  ✗ " + err.Error())}}
		}
		return printMsg{[]string{stOK.Render("  ✓ ") + stURL.Render(t.PublicURL) + stDim.Render("  → "+t.LocalAddr)}}
	}
}

func cmdLs(m *model, _ []string) tea.Cmd {
	if len(m.tunnels) == 0 {
		m.appendLines(stDim.Render("  no tunnels — ") + stKey.Render("/http <port>") + stDim.Render(" to start one"))
		return nil
	}
	for _, t := range m.tunnels {
		m.appendLines("  " + stKey.Render(shortID(t.ID)) + "  " + stURL.Render(t.PublicURL) +
			stDim.Render(fmt.Sprintf("  → %s · %s · %d req", t.LocalAddr, t.Status, t.Metrics.Requests)))
	}
	return nil
}

func cmdOpen(m *model, args []string) tea.Cmd {
	t := m.pickTunnel(args)
	if t == nil {
		m.appendLines(stErr.Render("  ✗ no matching tunnel"))
		return nil
	}
	if err := openBrowser(t.PublicURL); err != nil {
		m.appendLines(stErr.Render("  ✗ couldn't open browser: " + err.Error()))
		return nil
	}
	m.appendLines(stOK.Render("  ✓ opened ") + stURL.Render(t.PublicURL))
	return nil
}

func cmdQR(m *model, args []string) tea.Cmd {
	t := m.pickTunnel(args)
	if t == nil {
		m.appendLines(stErr.Render("  ✗ no tunnel to show — ") + stKey.Render("/http <port>") + stDim.Render(" first"))
		return nil
	}
	var buf bytes.Buffer
	qrterminal.GenerateHalfBlock(t.PublicURL, qrterminal.L, &buf)
	m.appendLines(stDim.Render("  scan to open ") + stURL.Render(t.PublicURL) + stDim.Render(" on your phone:"))
	for line := range strings.SplitSeq(strings.TrimRight(buf.String(), "\n"), "\n") {
		m.appendLines("  " + line)
	}
	return nil
}

func cmdCopy(m *model, args []string) tea.Cmd {
	t := m.pickTunnel(args)
	if t == nil {
		m.appendLines(stErr.Render("  ✗ no tunnel to copy"))
		return nil
	}
	if err := clipboard.WriteAll(t.PublicURL); err != nil {
		m.appendLines(stErr.Render("  ✗ couldn't copy: " + err.Error()))
		return nil
	}
	m.appendLines(stOK.Render("  ✓ copied ") + stURL.Render(t.PublicURL) + stDim.Render(" to the clipboard"))
	return nil
}

func cmdRequests(m *model, _ []string) tea.Cmd {
	cl := m.cl
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
		defer cancel()
		reqs, err := cl.requests(ctx)
		if err != nil {
			return printMsg{[]string{stErr.Render("  ✗ " + err.Error())}}
		}
		if len(reqs) == 0 {
			return printMsg{[]string{stDim.Render("  no requests captured yet — expose an HTTP service with /http")}}
		}
		out := []string{"  " + stTitle.Render("RECENT REQUESTS")}
		for _, r := range reqs[max(0, len(reqs)-20):] {
			out = append(out, "  "+methodStyle(r.Method).Render(pad(r.Method, 6))+" "+
				statusStyle(r.Status).Render(fmt.Sprintf("%3d", r.Status))+" "+
				stDim.Render(r.Host)+r.Path+stDim.Render(fmt.Sprintf("  %dms", r.DurationMs)))
		}
		return printMsg{out}
	}
}

func cmdStop(m *model, args []string) tea.Cmd {
	if len(args) == 0 {
		m.appendLines(stErr.Render("  ✗ usage: ") + stKey.Render("/stop <id|all>"))
		return nil
	}
	target := args[0]
	tunnels := append([]agent.Tunnel(nil), m.tunnels...) // snapshot for the goroutine
	cl := m.cl
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
		defer cancel()
		var victims []agent.Tunnel
		if strings.EqualFold(target, "all") {
			victims = tunnels
		} else if t := matchTunnel(tunnels, target); t != nil {
			victims = []agent.Tunnel{*t}
		}
		if len(victims) == 0 {
			return printMsg{[]string{stErr.Render("  ✗ no tunnel matched " + target)}}
		}
		var out []string
		for _, t := range victims {
			if err := cl.stopTunnel(ctx, t.ID); err != nil {
				out = append(out, stErr.Render("  ✗ "+err.Error()))
			} else {
				out = append(out, stOK.Render("  ✓ stopped ")+stDim.Render(t.PublicURL))
			}
		}
		return printMsg{out}
	}
}

// --- pins ---

var pinnable = map[string]bool{"traffic": true, "tunnels": true, "status": true}

func cmdPin(m *model, args []string) tea.Cmd {
	if len(args) == 0 {
		m.appendLines(stErr.Render("  ✗ usage: ") + stKey.Render("/pin <traffic|tunnels|status>"))
		return nil
	}
	name := strings.ToLower(args[0])
	if !pinnable[name] {
		m.appendLines(stErr.Render("  ✗ can't pin ") + name + stDim.Render(" — try traffic, tunnels or status"))
		return nil
	}
	if slices.Contains(m.pins, name) {
		m.appendLines(stDim.Render("  " + name + " is already pinned"))
		return nil
	}
	m.pins = append(m.pins, name)
	m.appendLines(stOK.Render("  ✓ pinned ") + stKey.Render(name) + stDim.Render(" — it stays on screen"))
	return nil
}

func cmdUnpin(m *model, args []string) tea.Cmd {
	if len(args) == 0 {
		m.appendLines(stErr.Render("  ✗ usage: ") + stKey.Render("/unpin <name|all>"))
		return nil
	}
	if strings.EqualFold(args[0], "all") {
		m.pins = nil
		m.appendLines(stOK.Render("  ✓ unpinned all"))
		return nil
	}
	name := strings.ToLower(args[0])
	kept := m.pins[:0]
	found := false
	for _, p := range m.pins {
		if p == name {
			found = true
			continue
		}
		kept = append(kept, p)
	}
	m.pins = kept
	if found {
		m.appendLines(stOK.Render("  ✓ unpinned " + name))
	} else {
		m.appendLines(stErr.Render("  ✗ " + name + " isn't pinned"))
	}
	return nil
}

// --- account / status / info ---

func cmdStatus(m *model, _ []string) tea.Cmd {
	glyph, word, col := m.connState()
	dot := lipglossFg(col, glyph+" "+word)
	m.appendLines("  " + dot + stDim.Render(fmt.Sprintf("   edge %s · %s · %d tunnels",
		firstNonEmpty(m.status.Edge, "—"), firstNonEmpty(m.status.Kind, "—"), len(m.tunnels))))
	return nil
}

func cmdWhoami(m *model, _ []string) tea.Cmd {
	if m.acct == nil {
		m.appendLines(stDim.Render("  not signed in — ") + stKey.Render("/login"))
		return m.fetchAccount()
	}
	a := m.acct
	m.appendLines(
		"  "+stBrand.Render(firstNonEmpty(a.User.Name, a.User.Email, "—"))+stDim.Render("   "+firstNonEmpty(a.Plan, a.Org.Plan, "free")+" plan"),
		stDim.Render(fmt.Sprintf("  usage  %s in · %s out · %d req (30d)",
			humanBytes(a.Usage.BytesIn), humanBytes(a.Usage.BytesOut), a.Usage.Requests)),
	)
	return nil
}

func cmdLogin(m *model, args []string) tea.Cmd {
	if len(args) > 0 { // paste an API key directly
		key := args[0]
		cl := m.cl
		m.appendLines(stDim.Render("  signing in with key…"))
		return func() tea.Msg {
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			if err := cl.login(ctx, key); err != nil {
				return printMsg{[]string{stErr.Render("  ✗ sign-in failed: " + err.Error())}}
			}
			return loginDoneMsg{ok: true}
		}
	}
	if m.loginActive {
		m.appendLines(stDim.Render("  sign-in already in progress…"))
		return nil
	}
	cl := m.cl
	m.appendLines(stDim.Render("  starting browser sign-in…"))
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		d, err := cl.oauthStart(ctx)
		if err != nil {
			return printMsg{[]string{stErr.Render("  ✗ couldn't start sign-in: " + err.Error())}}
		}
		return loginStartedMsg{d: d}
	}
}

func cmdUpdate(m *model, args []string) tea.Cmd {
	checkOnly := len(args) > 0 && strings.EqualFold(args[0], "check")
	cur, cl, onUpdate := m.opts.Version, m.cl, m.opts.OnUpdate
	m.appendLines(stDim.Render("  checking for updates…"))
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
		defer cancel()
		latest, _, err := cl.latestVersion(ctx)
		switch {
		case err != nil:
			return printMsg{[]string{stErr.Render("  ✗ couldn't check: " + err.Error())}}
		case latest == "":
			return printMsg{[]string{stDim.Render("  no release info available")}}
		case cur == "dev" || cur == "":
			return printMsg{[]string{stDim.Render("  dev build; latest release is " + latest)}}
		case latest == cur:
			return printMsg{[]string{stOK.Render("  ✓ trqsh " + cur + " is up to date")}}
		}
		if checkOnly || onUpdate == nil {
			return printMsg{[]string{
				stWarn.Render("  ▲ update available: ") + stKey.Render(cur+" → "+latest),
				stDim.Render("  run ") + stKey.Render("/update") + stDim.Render(" to install it"),
			}}
		}
		summary, err := onUpdate(ctx)
		if err != nil {
			return printMsg{[]string{stErr.Render("  ✗ update failed: " + err.Error())}}
		}
		return printMsg{[]string{stOK.Render("  ✓ " + summary), stDim.Render("  restart the console to run the new version")}}
	}
}

func cmdStart(m *model, _ []string) tea.Cmd {
	specs := m.opts.StartSpecs
	if len(specs) == 0 {
		m.appendLines(stDim.Render("  no tunnels in your config — add some, or use ") + stKey.Render("/http <port>"))
		return nil
	}
	cl := m.cl
	m.appendLines(stDim.Render(fmt.Sprintf("  starting %d tunnel(s) from config…", len(specs))))
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 40*time.Second)
		defer cancel()
		var out []string
		for _, spec := range specs {
			t, err := cl.startTunnel(ctx, spec)
			if err != nil {
				out = append(out, stErr.Render("  ✗ "+firstNonEmpty(spec.Name, spec.Addr)+": "+err.Error()))
			} else {
				out = append(out, stOK.Render("  ✓ ")+stURL.Render(t.PublicURL)+stDim.Render("  → "+t.LocalAddr))
			}
		}
		return printMsg{out}
	}
}

func cmdDown(m *model, _ []string) tea.Cmd {
	cl := m.cl
	m.appendLines(stDim.Render("  stopping the background daemon…"))
	// Stop the daemon, then quit — there's nothing left to show once it's gone.
	return tea.Sequence(
		func() tea.Msg {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			_ = cl.shutdown(ctx)
			return nil
		},
		tea.Quit,
	)
}

func cmdLogout(m *model, _ []string) tea.Cmd {
	cl := m.cl
	m.acct, m.authed = nil, false
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
		defer cancel()
		if err := cl.logout(ctx); err != nil {
			return printMsg{[]string{stErr.Render("  ✗ " + err.Error())}}
		}
		return printMsg{[]string{stOK.Render("  ✓ signed out")}}
	}
}

func cmdConfig(m *model, _ []string) tea.Cmd {
	if len(m.opts.ConfigRows) == 0 {
		m.appendLines(stDim.Render("  (no configuration to show)"))
		return nil
	}
	m.appendLines("  " + stTitle.Render("CONFIG"))
	for _, r := range m.opts.ConfigRows {
		m.appendLines("  " + stDim.Render(pad(r[0], 12)) + r[1])
	}
	return nil
}

func cmdSubdomains(m *model, args []string) tea.Cmd {
	cl, base := m.cl, firstNonEmpty(m.opts.BaseDomain, "trqsh.uz")
	if len(args) >= 2 {
		switch strings.ToLower(args[0]) {
		case "reserve", "add":
			name := strings.ToLower(args[1])
			return func() tea.Msg {
				ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
				defer cancel()
				s, err := cl.reserveSubdomain(ctx, name)
				if err != nil {
					return printMsg{[]string{stErr.Render("  ✗ " + err.Error())}}
				}
				return printMsg{[]string{stOK.Render("  ✓ reserved ") + stURL.Render(s.Subdomain+"."+base)}}
			}
		case "release", "remove", "rm":
			name := args[1]
			return func() tea.Msg {
				ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
				defer cancel()
				subs, err := cl.listSubdomains(ctx)
				if err != nil {
					return printMsg{[]string{stErr.Render("  ✗ " + err.Error())}}
				}
				id := ""
				for _, s := range subs {
					if s.ID == name || strings.EqualFold(s.Subdomain, name) {
						id = s.ID
						break
					}
				}
				if id == "" {
					return printMsg{[]string{stErr.Render("  ✗ no reserved subdomain matched " + name)}}
				}
				if err := cl.releaseSubdomain(ctx, id); err != nil {
					return printMsg{[]string{stErr.Render("  ✗ " + err.Error())}}
				}
				return printMsg{[]string{stOK.Render("  ✓ released " + name)}}
			}
		}
	}
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		subs, err := cl.listSubdomains(ctx)
		if err != nil {
			return printMsg{[]string{stErr.Render("  ✗ " + err.Error())}}
		}
		if len(subs) == 0 {
			return printMsg{[]string{stDim.Render("  no reserved subdomains — ") + stKey.Render("/subdomains reserve <name>")}}
		}
		out := []string{"  " + stTitle.Render("SUBDOMAINS")}
		for _, s := range subs {
			out = append(out, "  "+stURL.Render(s.Subdomain+"."+base))
		}
		return printMsg{out}
	}
}

func cmdDomains(m *model, args []string) tea.Cmd {
	cl := m.cl
	if len(args) >= 2 {
		switch strings.ToLower(args[0]) {
		case "add":
			name := strings.ToLower(args[1])
			return func() tea.Msg {
				ctx, cancel := context.WithTimeout(context.Background(), 12*time.Second)
				defer cancel()
				res, err := cl.addDomain(ctx, name)
				if err != nil {
					return printMsg{[]string{stErr.Render("  ✗ " + err.Error())}}
				}
				out := []string{
					stOK.Render("  ✓ added ") + stURL.Render(res.Domain.Domain),
					stDim.Render("  set these DNS records, then ") + stKey.Render("/domains verify "+res.Domain.Domain) + stDim.Render(":"),
				}
				if res.DNSInstructions["txt_name"] != "" {
					out = append(out, stDim.Render("    TXT   "+res.DNSInstructions["txt_name"]+"  "+res.DNSInstructions["txt_value"]))
				}
				if res.DNSInstructions["cname_name"] != "" {
					out = append(out, stDim.Render("    CNAME "+res.DNSInstructions["cname_name"]+"  "+res.DNSInstructions["cname_value"]))
				}
				return printMsg{out}
			}
		case "verify":
			name := args[1]
			return func() tea.Msg {
				ctx, cancel := context.WithTimeout(context.Background(), 12*time.Second)
				defer cancel()
				doms, err := cl.listDomains(ctx)
				if err != nil {
					return printMsg{[]string{stErr.Render("  ✗ " + err.Error())}}
				}
				id := ""
				for _, d := range doms {
					if d.ID == name || strings.EqualFold(d.Domain, name) {
						id = d.ID
						break
					}
				}
				if id == "" {
					return printMsg{[]string{stErr.Render("  ✗ no domain matched " + name)}}
				}
				d, err := cl.verifyDomain(ctx, id)
				if err != nil {
					return printMsg{[]string{stErr.Render("  ✗ " + err.Error())}}
				}
				if d.VerifiedAt != nil {
					return printMsg{[]string{stOK.Render("  ✓ verified " + d.Domain)}}
				}
				return printMsg{[]string{stWarn.Render("  " + d.Domain + " is not verified yet")}}
			}
		}
	}
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		doms, err := cl.listDomains(ctx)
		if err != nil {
			return printMsg{[]string{stErr.Render("  ✗ " + err.Error())}}
		}
		if len(doms) == 0 {
			return printMsg{[]string{stDim.Render("  no custom domains — ") + stKey.Render("/domains add <domain>")}}
		}
		out := []string{"  " + stTitle.Render("DOMAINS")}
		for _, d := range doms {
			status := "pending"
			if d.VerifiedAt != nil {
				status = "verified"
			}
			out = append(out, "  "+stURL.Render(d.Domain)+stDim.Render("  "+status+" · "+firstNonEmpty(d.CertStatus, "—")))
		}
		return printMsg{out}
	}
}

func cmdUninstall(m *model, args []string) tea.Cmd {
	onUninstall := m.opts.OnUninstall
	if onUninstall == nil {
		m.appendLines(stDim.Render("  uninstall isn't available in this build"))
		return nil
	}
	if len(args) == 0 || !strings.EqualFold(args[0], "yes") {
		m.appendLines(
			stWarn.Render("  ⚠ this removes trqsh's local data (config, credentials, cached binaries) and stops tunnels"),
			stDim.Render("  type ")+stKey.Render("/uninstall yes")+stDim.Render(" to confirm"),
		)
		return nil
	}
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		msg, err := onUninstall(ctx)
		if err != nil {
			return printMsg{[]string{stErr.Render("  ✗ " + err.Error())}}
		}
		return printMsg{[]string{stOK.Render("  ✓ " + msg)}}
	}
}

func cmdVersion(m *model, _ []string) tea.Cmd {
	m.appendLines("  " + stBrand.Render("trqsh") + " " + m.opts.Version + stDim.Render("  "+runtimePlatform()))
	return nil
}

func cmdClear(m *model, _ []string) tea.Cmd {
	m.transcript = nil
	m.welcomeLen = 0 // the banner is gone; don't try to refresh it in place
	m.vp.SetContent("")
	return nil
}

func cmdHelp(m *model, _ []string) tea.Cmd {
	m.appendLines("  " + stTitle.Render("COMMANDS"))
	for _, c := range slashCommands {
		m.appendLines("  " + stKey.Render(pad("/"+c.name, 9)) + " " + stDim.Render(pad(c.args, 18)) + c.desc)
	}
	m.appendLines(stDim.Render("  tip: type ") + stKey.Render("/") + stDim.Render(" to autocomplete; ") + stKey.Render("/pin traffic") + stDim.Render(" to watch requests live"))
	return nil
}

func cmdQuit(_ *model, _ []string) tea.Cmd { return tea.Quit }

// --- helpers ---

// pickTunnel resolves the tunnel an /open call refers to: the arg (id prefix,
// name, or URL substring), or the first tunnel when no arg is given.
func (m model) pickTunnel(args []string) *agent.Tunnel {
	if len(args) == 0 {
		if len(m.tunnels) == 0 {
			return nil
		}
		return &m.tunnels[0]
	}
	return matchTunnel(m.tunnels, args[0])
}

func matchTunnel(tunnels []agent.Tunnel, q string) *agent.Tunnel {
	ql := strings.ToLower(strings.TrimSpace(q))
	if ql == "" {
		return nil
	}
	for i := range tunnels {
		t := &tunnels[i]
		if t.ID == q || strings.EqualFold(t.Name, q) || strings.HasPrefix(t.ID, q) ||
			strings.Contains(strings.ToLower(t.PublicURL), ql) {
			return t
		}
	}
	return nil
}
