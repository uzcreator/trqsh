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
	"time"

	"github.com/spf13/cobra"
	"github.com/trqsh-uz/trqsh/internal/agent"
)

// This file is the CLI's plumbing for the background daemon: spawning it and
// talking to its loopback control API. It's all the console needs from package
// cli — the interactive UI (package tui) has its own richer client, but the
// launcher still spawns the daemon and, for uninstall, tells it to shut down.

// controlAddr returns the local control-API address the daemon listens on.
func controlAddr(cfg agent.Config) string {
	if cfg.ControlAddr != "" {
		return cfg.ControlAddr
	}
	return "127.0.0.1:4041"
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
	defer func() { _ = resp.Body.Close() }()
	return resp.StatusCode == http.StatusOK
}

// waitDaemonExit polls addr until the daemon stops answering /healthz (or
// timeout elapses), so a caller that just told it to shut down doesn't race
// its own cleanup against a daemon that's still exiting. The control API's
// /shutdown handler acks immediately and only then triggers the actual
// shutdown, so without this a caller like removeLocalData could try to
// delete daemon.log while the daemon process still has it open — harmless on
// Unix (open files can be unlinked while held), but Windows refuses to
// delete a file another process still holds, so uninstall would report it
// couldn't fully remove ~/.trqsh.
func waitDaemonExit(addr string, timeout time.Duration) {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if !daemonAlive(addr) {
			return
		}
		time.Sleep(100 * time.Millisecond)
	}
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
	defer func() { _ = resp.Body.Close() }()
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
