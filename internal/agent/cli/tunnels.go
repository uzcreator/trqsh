package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/trqsh-uz/trqsh/internal/agent"
)

// This file implements the background-tunnel surface: `-d/--detach` spawns (or
// reuses) a headless `trqsh daemon` that owns the tunnels and keeps them up after
// the CLI returns, and `ls`/`open`/`stop`/`down` manage that daemon over its
// loopback control API. See daemon.go for the daemon itself and localapi.go for
// the control API it serves.

// controlAddr returns the local control-API address the daemon listens on.
func controlAddr(cfg agent.Config) string {
	if cfg.ControlAddr != "" {
		return cfg.ControlAddr
	}
	return "127.0.0.1:4041"
}

// runOrDetach dispatches a tunnel command to the foreground runner or, with -d,
// to the background daemon.
func runOrDetach(cmd *cobra.Command, g *globalFlags, specs []agent.TunnelSpec, detach bool) error {
	if detach {
		return runDetached(cmd, g, specs)
	}
	return runTunnels(cmd, g, specs)
}

// runDetached ensures a background daemon is running, hands it the tunnel specs,
// prints the public URLs, and returns — leaving the tunnels serving in the daemon.
func runDetached(cmd *cobra.Command, g *globalFlags, specs []agent.TunnelSpec) error {
	cfg, err := g.loadConfig(cmd)
	if err != nil {
		return err
	}
	addr := controlAddr(cfg)
	if err := ensureDaemon(cmd, g, addr); err != nil {
		return err
	}
	// If we're signed in, make sure the (possibly pre-existing) daemon uses the
	// current key, so a daemon started before login can still authenticate.
	if strings.TrimSpace(cfg.APIKey) != "" {
		_ = controlPOST(addr, "/login", map[string]string{"token": cfg.APIKey}, nil)
	}
	for _, spec := range specs {
		var t agent.Tunnel
		if err := controlPOST(addr, "/tunnels", spec, &t); err != nil {
			return fmt.Errorf("start tunnel: %w", err)
		}
		fmt.Printf("\n  ✓ %s  →  %s   (background)\n", bold(t.PublicURL), t.LocalAddr)
	}
	fmt.Printf("\n  Running in the background. Manage with:\n" +
		"    trqsh ls          list tunnels\n" +
		"    trqsh open <id>   open in browser\n" +
		"    trqsh stop <id>   stop one  (or: trqsh stop all)\n" +
		"    trqsh down        stop the daemon\n\n")
	return nil
}

// ensureDaemon returns once a daemon is reachable at addr, spawning a detached
// `trqsh daemon` if none is running yet.
func ensureDaemon(cmd *cobra.Command, g *globalFlags, addr string) error {
	if daemonAlive(addr) {
		return nil
	}
	// Create the shared loopback token up front so the CLI and the daemon it
	// spawns agree on it without a startup race (unless auth is disabled for dev).
	if os.Getenv("TRQSH_CONTROL_NO_AUTH") != "1" {
		if _, err := agent.LoadOrCreateControlToken(); err != nil {
			return fmt.Errorf("control token: %w", err)
		}
	}
	exe, err := os.Executable()
	if err != nil {
		return err
	}
	args := append([]string{"daemon"}, daemonPassthroughFlags(g, cmd)...)
	c := exec.Command(exe, args...) // #nosec G204 -- exe is our own os.Executable() path, re-exec'ing ourselves as a detached daemon; args are our own passthrough flags
	c.SysProcAttr = detachSysProcAttr()
	c.Stdin = nil
	// Send the daemon's logs to a file so a background failure is diagnosable.
	logPath := filepath.Join(filepath.Dir(agent.DefaultConfigPath()), "daemon.log")
	if f, ferr := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600); ferr == nil { // #nosec G304 -- logPath is derived from our own DefaultConfigPath(), not attacker input
		c.Stdout = f
		c.Stderr = f
		defer func() { _ = f.Close() }()
	}
	if err := c.Start(); err != nil {
		return fmt.Errorf("start daemon: %w", err)
	}
	// Detach: we don't wait on the child, and releasing frees our handle to it.
	_ = c.Process.Release()

	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		if daemonAlive(addr) {
			return nil
		}
		time.Sleep(200 * time.Millisecond)
	}
	return fmt.Errorf("daemon did not become ready at %s (see %s)", addr, logPath)
}

// daemonPassthroughFlags forwards the global flags the user set on the CLI to the
// spawned daemon, so a `--server`/`--insecure`/`--config` etc. carries over.
func daemonPassthroughFlags(g *globalFlags, cmd *cobra.Command) []string {
	var a []string
	fl := cmd.Flags()
	if fl.Changed("config") && g.configPath != "" {
		a = append(a, "--config", g.configPath)
	}
	if fl.Changed("server") {
		a = append(a, "--server", g.server)
	}
	if fl.Changed("region") {
		a = append(a, "--region", g.region)
	}
	if fl.Changed("transport") {
		a = append(a, "--transport", g.transport)
	}
	if fl.Changed("insecure") && g.insecure {
		a = append(a, "--insecure")
	}
	if fl.Changed("control-addr") {
		a = append(a, "--control-addr", g.controlA)
	}
	if fl.Changed("log") {
		a = append(a, "--log", g.logFormat)
	}
	return a
}

// daemonAlive reports whether a control API answers /healthz at addr.
func daemonAlive(addr string) bool {
	req, err := http.NewRequest(http.MethodGet, "http://"+addr+"/healthz", nil)
	if err != nil {
		return false
	}
	resp, err := (&http.Client{Timeout: time.Second}).Do(req)
	if err != nil {
		return false
	}
	defer resp.Body.Close()
	return resp.StatusCode == http.StatusOK
}

// controlPOST posts JSON to the daemon's control API, attaching the loopback
// token, and decodes a 2xx body into out (when non-nil).
func controlPOST(addr, path string, in, out any) error {
	var body io.Reader
	if in != nil {
		b, err := json.Marshal(in)
		if err != nil {
			return err
		}
		body = bytes.NewReader(b)
	}
	req, err := http.NewRequestWithContext(context.Background(), http.MethodPost, "http://"+addr+path, body)
	if err != nil {
		return err
	}
	if in != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if tok := agent.LoadControlToken(); tok != "" {
		req.Header.Set("Authorization", "Bearer "+tok)
	}
	resp, err := (&http.Client{Timeout: 30 * time.Second}).Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode/100 != 2 {
		var e struct {
			Error string `json:"error"`
		}
		_ = json.NewDecoder(io.LimitReader(resp.Body, 1<<16)).Decode(&e)
		if e.Error != "" {
			return errors.New(e.Error)
		}
		return fmt.Errorf("status %d", resp.StatusCode)
	}
	if out != nil {
		return json.NewDecoder(io.LimitReader(resp.Body, 1<<20)).Decode(out)
	}
	return nil
}

// newLsCmd lists the tunnels running in the background daemon.
func newLsCmd(g *globalFlags) *cobra.Command {
	return &cobra.Command{
		Use:     "ls",
		Aliases: []string{"list", "ps"},
		Short:   "List running (background) tunnels",
		RunE: func(cmd *cobra.Command, _ []string) error {
			cfg, err := g.loadConfig(cmd)
			if err != nil {
				return err
			}
			addr := controlAddr(cfg)
			var tunnels []agent.Tunnel
			if err := controlGET(addr, "/tunnels", &tunnels); err != nil {
				fmt.Println("no background daemon running — start one with `trqsh http <port> -d`")
				return nil
			}
			if len(tunnels) == 0 {
				fmt.Println("no running tunnels")
				return nil
			}
			tw := newTab()
			fmt.Fprintln(tw, "ID\tURL\tLOCAL\tSTATUS\tUPTIME\tREQ")
			for _, t := range tunnels {
				fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%s\t%d\n",
					shortID(t.ID), t.PublicURL, t.LocalAddr, t.Status, uptime(t.CreatedAt), t.Metrics.Requests)
			}
			return tw.Flush()
		},
	}
}

// newOpenCmd opens a running tunnel's public URL in the browser.
func newOpenCmd(g *globalFlags) *cobra.Command {
	return &cobra.Command{
		Use:   "open <id|name|subdomain>",
		Short: "Open a running tunnel's public URL in your browser",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := g.loadConfig(cmd)
			if err != nil {
				return err
			}
			addr := controlAddr(cfg)
			var tunnels []agent.Tunnel
			if err := controlGET(addr, "/tunnels", &tunnels); err != nil {
				return fmt.Errorf("no background daemon running at %s", addr)
			}
			t := resolveTunnel(tunnels, args[0])
			if t == nil {
				return fmt.Errorf("no tunnel matched %q (see `trqsh ls`)", args[0])
			}
			fmt.Printf("opening %s\n", t.PublicURL)
			return openBrowser(t.PublicURL)
		},
	}
}

// newStopCmd stops one background tunnel (by id/name), or all of them.
func newStopCmd(g *globalFlags) *cobra.Command {
	return &cobra.Command{
		Use:     "stop [id|name|all]",
		Aliases: []string{"kill", "rm"},
		Short:   "Stop a background tunnel (or all with `stop all`)",
		Args:    cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := g.loadConfig(cmd)
			if err != nil {
				return err
			}
			addr := controlAddr(cfg)
			var tunnels []agent.Tunnel
			if err := controlGET(addr, "/tunnels", &tunnels); err != nil {
				return fmt.Errorf("no background daemon running at %s", addr)
			}
			if len(tunnels) == 0 {
				fmt.Println("no running tunnels")
				return nil
			}
			target := "all"
			if len(args) == 1 {
				target = args[0]
			}
			stopped := 0
			for i := range tunnels {
				t := &tunnels[i]
				if target == "all" || matchesTunnel(t, target) {
					if err := controlDelete(addr, "/tunnels/"+t.ID); err == nil {
						fmt.Printf("stopped %s (%s)\n", firstNonEmpty(t.Name, shortID(t.ID)), t.PublicURL)
						stopped++
					}
				}
			}
			if stopped == 0 {
				return fmt.Errorf("no tunnel matched %q (see `trqsh ls`)", target)
			}
			return nil
		},
	}
}

// newDownCmd stops the whole background daemon (and every tunnel it holds).
func newDownCmd(g *globalFlags) *cobra.Command {
	return &cobra.Command{
		Use:     "down",
		Aliases: []string{"quit", "shutdown"},
		Short:   "Stop the background daemon and all its tunnels",
		RunE: func(cmd *cobra.Command, _ []string) error {
			cfg, err := g.loadConfig(cmd)
			if err != nil {
				return err
			}
			addr := controlAddr(cfg)
			if !daemonAlive(addr) {
				fmt.Println("no background daemon running")
				return nil
			}
			// The daemon replies 204 then exits shortly after; a dropped connection
			// here just means it already went down, which is the goal.
			_ = controlPOST(addr, "/shutdown", nil, nil)
			fmt.Println("background daemon stopped")
			return nil
		},
	}
}

// resolveTunnel finds the tunnel a user reference points at: exact id/name first,
// then an id prefix or a substring of the name or public URL.
func resolveTunnel(tunnels []agent.Tunnel, q string) *agent.Tunnel {
	q = strings.TrimSpace(q)
	for i := range tunnels {
		if tunnels[i].ID == q || tunnels[i].Name == q {
			return &tunnels[i]
		}
	}
	for i := range tunnels {
		if matchesTunnel(&tunnels[i], q) {
			return &tunnels[i]
		}
	}
	return nil
}

func matchesTunnel(t *agent.Tunnel, q string) bool {
	ql := strings.ToLower(strings.TrimSpace(q))
	if ql == "" {
		return false
	}
	return strings.HasPrefix(t.ID, q) ||
		strings.Contains(strings.ToLower(t.Name), ql) ||
		strings.Contains(strings.ToLower(t.PublicURL), ql)
}

func shortID(id string) string {
	if len(id) > 8 {
		return id[:8]
	}
	return id
}

func uptime(t time.Time) string {
	if t.IsZero() {
		return "-"
	}
	d := time.Since(t)
	if d < time.Minute {
		return d.Round(time.Second).String()
	}
	if d < time.Hour {
		return d.Round(time.Minute).String()
	}
	return d.Round(time.Hour).String()
}
