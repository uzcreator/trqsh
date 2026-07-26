package cli

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/trqsh-uz/trqsh/internal/agent"
)

func newHTTPCmd(g *globalFlags) *cobra.Command {
	var subdomain, basicAuth, hostHeader string
	cmd := &cobra.Command{
		Use:   "http <port|addr>",
		Short: "Expose a local HTTP service",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runTunnels(cmd, g, []agent.TunnelSpec{{
				Proto:      "http",
				Addr:       normalizeAddr(args[0]),
				Subdomain:  subdomain,
				BasicAuth:  basicAuth,
				HostHeader: hostHeader,
			}})
		},
	}
	cmd.Flags().StringVar(&subdomain, "subdomain", "", "requested subdomain (reserved subdomains need Pro)")
	cmd.Flags().StringVar(&basicAuth, "basic-auth", "", "protect with basic auth (user:pass)")
	cmd.Flags().StringVar(&hostHeader, "host-header", "", "rewrite the Host header sent to the local service")
	return cmd
}

func newTCPCmd(g *globalFlags) *cobra.Command {
	var remotePort int
	cmd := &cobra.Command{
		Use:   "tcp <port|addr>",
		Short: "Expose a local TCP service",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runTunnels(cmd, g, []agent.TunnelSpec{{
				Proto:      "tcp",
				Addr:       normalizeAddr(args[0]),
				RemotePort: remotePort,
			}})
		},
	}
	cmd.Flags().IntVar(&remotePort, "remote-port", 0, "requested remote port (0 = ephemeral)")
	return cmd
}

func newUDPCmd(g *globalFlags) *cobra.Command {
	var remotePort int
	cmd := &cobra.Command{
		Use:   "udp <port|addr>",
		Short: "Expose a local UDP service",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runTunnels(cmd, g, []agent.TunnelSpec{{
				Proto:      "udp",
				Addr:       normalizeAddr(args[0]),
				RemotePort: remotePort,
			}})
		},
	}
	cmd.Flags().IntVar(&remotePort, "remote-port", 0, "requested remote port (0 = ephemeral)")
	return cmd
}

func newStartCmd(g *globalFlags) *cobra.Command {
	return &cobra.Command{
		Use:   "start",
		Short: "Start all tunnels defined in the config file",
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
			return runTunnels(cmd, g, specs)
		},
	}
}

func newStatusCmd(g *globalFlags) *cobra.Command {
	return &cobra.Command{
		Use:   "status",
		Short: "Show the status of a running agent (via the local control API)",
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

func newStopCmd(g *globalFlags) *cobra.Command {
	return &cobra.Command{
		Use:   "stop",
		Short: "Stop all tunnels on a running agent",
		RunE: func(cmd *cobra.Command, _ []string) error {
			cfg, err := g.loadConfig(cmd)
			if err != nil {
				return err
			}
			var tunnels []agent.Tunnel
			if err := controlGET(cfg.ControlAddr, "/tunnels", &tunnels); err != nil {
				return fmt.Errorf("no running agent at %s", cfg.ControlAddr)
			}
			for _, t := range tunnels {
				_ = controlDelete(cfg.ControlAddr, "/tunnels/"+t.ID)
				fmt.Printf("stopped %s\n", t.Name)
			}
			return nil
		},
	}
}

func newLoginCmd(g *globalFlags) *cobra.Command {
	var token string
	cmd := &cobra.Command{
		Use:   "login",
		Short: "Save an API key",
		RunE: func(cmd *cobra.Command, args []string) error {
			if token == "" && len(args) > 0 {
				token = args[0]
			}
			if token == "" {
				fmt.Print("API key: ")
				sc := bufio.NewScanner(os.Stdin)
				if sc.Scan() {
					token = strings.TrimSpace(sc.Text())
				}
			}
			if token == "" {
				return fmt.Errorf("no token provided")
			}
			cfg, err := g.loadConfig(cmd)
			if err != nil {
				return err
			}
			cfg.APIKey = token
			if err := cfg.Save(agent.DefaultConfigPath()); err != nil {
				return err
			}
			fmt.Printf("saved API key to %s\n", agent.DefaultConfigPath())
			return nil
		},
	}
	cmd.Flags().StringVar(&token, "token", "", "API key to save")
	return cmd
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

func newVersionCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Print version information",
		Run: func(_ *cobra.Command, _ []string) {
			fmt.Printf("trqsh %s", agent.Version)
			if agent.Commit != "" {
				fmt.Printf(" (%s)", agent.Commit)
			}
			if agent.BuildDate != "" {
				fmt.Printf(" %s", agent.BuildDate)
			}
			fmt.Println()
		},
	}
}

func newUpdateCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "update",
		Short: "Check for and install updates",
		RunE: func(_ *cobra.Command, _ []string) error {
			// TODO(part-08): query the release feed, compare versions, download +
			// verify a signed binary, and swap in place.
			fmt.Println("self-update is not available yet — see https://trqsh.uz/download")
			return nil
		},
	}
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
	defer resp.Body.Close()
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
