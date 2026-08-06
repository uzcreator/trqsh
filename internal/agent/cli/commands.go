package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"runtime"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/trqsh-uz/trqsh/internal/agent"
	"github.com/trqsh-uz/trqsh/internal/agent/cli/ui"
)

func newHTTPCmd(g *globalFlags) *cobra.Command {
	var subdomain, basicAuth, hostHeader string
	var detach bool
	cmd := &cobra.Command{
		Use:   "http <port|addr>",
		Short: "Expose a local HTTP service",
		Example: "trqsh http 3000\n" +
			"trqsh http 8080 --subdomain myapp\n" +
			"trqsh http 3000 --basic-auth user:pass\n" +
			"trqsh http 3000 --detach",
		Args: portArg(),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runOrDetach(cmd, g, []agent.TunnelSpec{{
				Proto:      "http",
				Addr:       normalizeAddr(args[0]),
				Subdomain:  subdomain,
				BasicAuth:  basicAuth,
				HostHeader: hostHeader,
			}}, detach)
		},
	}
	cmd.Flags().StringVar(&subdomain, "subdomain", "", "requested subdomain (reserved subdomains need Pro)")
	cmd.Flags().StringVar(&basicAuth, "basic-auth", "", "protect with basic auth (user:pass)")
	cmd.Flags().StringVar(&hostHeader, "host-header", "", "rewrite the Host header sent to the local service")
	cmd.Flags().BoolVarP(&detach, "detach", "d", false, "run in the background and return immediately")
	return cmd
}

func newTCPCmd(g *globalFlags) *cobra.Command {
	var remotePort int
	var detach bool
	cmd := &cobra.Command{
		Use:   "tcp <port|addr>",
		Short: "Expose a local TCP port",
		Example: "trqsh tcp 5432\n" +
			"trqsh tcp 22 --remote-port 2222",
		Args: portArg(),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runOrDetach(cmd, g, []agent.TunnelSpec{{
				Proto:      "tcp",
				Addr:       normalizeAddr(args[0]),
				RemotePort: remotePort,
			}}, detach)
		},
	}
	cmd.Flags().IntVar(&remotePort, "remote-port", 0, "requested remote port (0 = ephemeral)")
	cmd.Flags().BoolVarP(&detach, "detach", "d", false, "run in the background and return immediately")
	return cmd
}

func newUDPCmd(g *globalFlags) *cobra.Command {
	var remotePort int
	var detach bool
	cmd := &cobra.Command{
		Use:     "udp <port|addr>",
		Short:   "Expose a local UDP port",
		Example: "trqsh udp 5353",
		Args:    portArg(),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runOrDetach(cmd, g, []agent.TunnelSpec{{
				Proto:      "udp",
				Addr:       normalizeAddr(args[0]),
				RemotePort: remotePort,
			}}, detach)
		},
	}
	cmd.Flags().IntVar(&remotePort, "remote-port", 0, "requested remote port (0 = ephemeral)")
	cmd.Flags().BoolVarP(&detach, "detach", "d", false, "run in the background and return immediately")
	return cmd
}

func newStartCmd(g *globalFlags) *cobra.Command {
	var detach bool
	cmd := &cobra.Command{
		Use:     "start",
		Short:   "Start all tunnels from your config",
		Example: "trqsh start\ntrqsh start --detach",
		RunE: func(cmd *cobra.Command, _ []string) error {
			cfg, err := g.loadConfig(cmd)
			if err != nil {
				return err
			}
			if len(cfg.Tunnels) == 0 {
				return fmt.Errorf("no tunnels in config (%s); add some or use `trqsh http <port>`", agent.DefaultConfigPath())
			}
			specs := make([]agent.TunnelSpec, 0, len(cfg.Tunnels))
			for name, t := range cfg.Tunnels {
				specs = append(specs, agent.TunnelSpec{
					Name: name, Proto: t.Proto, Addr: t.Addr, Subdomain: t.Subdomain,
					CustomDomain: t.CustomDomain, BasicAuth: t.BasicAuth, HostHeader: t.HostHeader,
					RemotePort: t.RemotePort,
				})
			}
			return runOrDetach(cmd, g, specs, detach)
		},
	}
	cmd.Flags().BoolVarP(&detach, "detach", "d", false, "run in the background and return immediately")
	return cmd
}

func newStatusCmd(g *globalFlags) *cobra.Command {
	return &cobra.Command{
		Use:   "status",
		Short: "Show a running agent's status",
		RunE: func(cmd *cobra.Command, _ []string) error {
			cfg, err := g.loadConfig(cmd)
			if err != nil {
				return err
			}
			var st agent.Status
			if err := controlGET(cfg.ControlAddr, "/status", &st); err != nil {
				return fmt.Errorf("no running agent at %s (%w)", cfg.ControlAddr, err)
			}
			fmt.Printf("connected: %v  plan: %s  edge: %s  transport: %s\n", st.Connected, st.Plan, st.Edge, st.Kind)
			var tunnels []agent.Tunnel
			if err := controlGET(cfg.ControlAddr, "/tunnels", &tunnels); err == nil {
				for _, t := range tunnels {
					fmt.Printf("  %-16s %-24s → %s  (req=%d)\n", t.Name, t.PublicURL, t.LocalAddr, t.Metrics.Requests)
				}
			}
			return nil
		},
	}
}

func newConfigCmd(g *globalFlags) *cobra.Command {
	return &cobra.Command{
		Use:   "config",
		Short: "Show config path and current values",
		RunE: func(cmd *cobra.Command, _ []string) error {
			cfg, err := g.loadConfig(cmd)
			if err != nil {
				return err
			}
			fmt.Printf("path: %s\n", firstNonEmpty(g.configPath, agent.DefaultConfigPath()))
			fmt.Printf("server: %s\nregion: %s\ntransport: %s\ninspector: %s (enabled=%v)\ncontrol: %s\ntunnels: %d\n",
				cfg.Server, cfg.Region, cfg.Transport, cfg.Inspector.Addr, cfg.Inspector.Enabled, cfg.ControlAddr, len(cfg.Tunnels))
			return nil
		},
	}
}

// versionString formats the build metadata that newVersionCmd and the root
// command's --version flag (wired up in cli.go) both print, so the two never
// drift apart.
func versionString() string {
	s := agent.Version
	if agent.Commit != "" {
		s += " (" + agent.Commit + ")"
	}
	if agent.BuildDate != "" {
		s += " " + agent.BuildDate
	}
	return s
}

func newVersionCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Show version information",
		Run: func(cmd *cobra.Command, _ []string) {
			printVersion(cmd.OutOrStdout())
		},
	}
}

// printVersion renders the branded version block: the release on top, then the
// build and runtime facts worth having when someone files a bug report.
func printVersion(w io.Writer) {
	fmt.Fprintf(w, "\n  %s %s\n", ui.AccentBold("trqsh"), ui.Bold(agent.Version))
	for _, r := range [][2]string{
		{"commit", firstNonEmpty(agent.Commit, "—")},
		{"built", firstNonEmpty(agent.BuildDate, "—")},
		{"platform", runtime.GOOS + "/" + runtime.GOARCH},
		{"go", runtime.Version()},
	} {
		fmt.Fprintf(w, "  %s  %s\n", ui.Gray(ui.Pad(r[0], 8)), r[1])
	}
	fmt.Fprintln(w)
}

// --- local control API client helpers ---

func controlGET(addr, path string, out any) error {
	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, "http://"+addr+path, nil)
	if err != nil {
		return err
	}
	if tok := agent.LoadControlToken(); tok != "" {
		req.Header.Set("Authorization", "Bearer "+tok)
	}
	resp, err := (&http.Client{Timeout: 3 * time.Second}).Do(req)
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("status %d", resp.StatusCode)
	}
	return json.NewDecoder(resp.Body).Decode(out)
}

func controlDelete(addr, path string) error {
	req, err := http.NewRequestWithContext(context.Background(), http.MethodDelete, "http://"+addr+path, nil)
	if err != nil {
		return err
	}
	if tok := agent.LoadControlToken(); tok != "" {
		req.Header.Set("Authorization", "Bearer "+tok)
	}
	resp, err := (&http.Client{Timeout: 3 * time.Second}).Do(req)
	if err != nil {
		return err
	}
	return resp.Body.Close()
}

// portArg validates that exactly one port/address was supplied, replacing
// cobra's terse "accepts 1 arg(s), received 0" with a message that shows the
// fix — the kind of small touch that makes the CLI feel finished.
func portArg() cobra.PositionalArgs {
	return func(cmd *cobra.Command, args []string) error {
		switch len(args) {
		case 1:
			return nil
		case 0:
			return fmt.Errorf("give a local port or address to expose, e.g. `%s 3000`", cmd.CommandPath())
		default:
			return fmt.Errorf("expose one address at a time — got %d (e.g. `%s 3000`)", len(args), cmd.CommandPath())
		}
	}
}

// normalizeAddr turns "3000" into "localhost:3000", leaving host:port as-is.
func normalizeAddr(a string) string {
	a = strings.TrimSpace(a)
	if i := strings.Index(a, "://"); i >= 0 {
		a = a[i+3:]
	}
	if _, _, err := net.SplitHostPort(a); err == nil {
		return a
	}
	return "localhost:" + a
}

func firstNonEmpty(a, b string) string {
	if a != "" {
		return a
	}
	return b
}
