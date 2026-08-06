package cli

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/trqsh-uz/trqsh/internal/agent"
	"github.com/trqsh-uz/trqsh/internal/agent/cli/ui"
)

// newLoginCmd signs the CLI in. By default it runs the browser device-flow: it
// asks the control plane for a code pair, opens the verification URL (where the
// user signs in with GitHub/Google and approves), then polls until an API key is
// minted and stored. `--token` keeps the old paste-a-key path for CI/headless.
func newLoginCmd(g *globalFlags) *cobra.Command {
	var token string
	var noBrowser bool
	cmd := &cobra.Command{
		Use:   "login",
		Short: "Sign in through your browser",
		Long: "Sign in to trqsh.\n\n" +
			"With no flags this opens your browser: sign in with GitHub or Google and\n" +
			"approve the request, and the CLI stores the API key automatically.\n" +
			"For CI or headless machines, pass an existing key with --token.",
		Example: "trqsh login\n" +
			"trqsh login --token tq_xxx\n" +
			"trqsh login --no-browser",
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := g.loadConfig(cmd)
			if err != nil {
				return err
			}
			if token == "" && len(args) > 0 {
				token = args[0]
			}
			if token != "" {
				return saveAndConfirm(cmd.Context(), cfg, strings.TrimSpace(token))
			}
			return browserLogin(cfg, noBrowser)
		},
	}
	cmd.Flags().StringVar(&token, "token", "", "sign in with an existing API key instead of the browser")
	cmd.Flags().BoolVar(&noBrowser, "no-browser", false, "print the sign-in URL instead of opening a browser")
	return cmd
}

// browserLogin runs the device-authorization flow end to end.
func browserLogin(cfg agent.Config, noBrowser bool) error {
	cc := newCloudClient(cfg)
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	dev, err := cc.startDevice(ctx)
	if err != nil {
		return fmt.Errorf("could not start sign-in: %w", err)
	}
	url := firstNonEmpty(dev.VerificationURIComplete, dev.VerificationURI)

	fmt.Printf("\n  To sign in, open this URL in your browser:\n\n    %s\n\n  Verification code: %s\n", url, bold(dev.UserCode))
	if noBrowser {
		fmt.Printf("\n  Waiting for approval… (Ctrl+C to cancel)\n")
	} else if err := openBrowser(url); err == nil {
		fmt.Printf("\n  Opened your browser. Waiting for approval… (Ctrl+C to cancel)\n")
	} else {
		fmt.Printf("\n  Couldn't open a browser automatically — open the URL above.\n  Waiting for approval… (Ctrl+C to cancel)\n")
	}

	interval := time.Duration(maxInt(dev.Interval, 2)) * time.Second
	expiry := time.Duration(orDefault(dev.ExpiresIn, 600)) * time.Second
	deadline := time.After(expiry)
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return errors.New("sign-in canceled")
		case <-deadline:
			return errors.New("sign-in timed out — run `trqsh login` again")
		case <-ticker.C:
			key, pending, err := cc.pollDevice(ctx, dev.DeviceCode)
			if err != nil {
				return err
			}
			if pending {
				continue
			}
			return saveAndConfirm(ctx, cfg, key)
		}
	}
}

// saveAndConfirm persists the API key and prints the resulting account so the
// user immediately sees who they signed in as (and that the key works).
func saveAndConfirm(ctx context.Context, cfg agent.Config, key string) error {
	if key == "" {
		return errors.New("no API key received")
	}
	cfg.APIKey = key
	if err := cfg.Save(agent.DefaultConfigPath()); err != nil {
		return err
	}
	ui.Success("Signed in — key saved to %s", ui.Gray(agent.DefaultConfigPath()))
	cc := newCloudClient(cfg)
	if me, err := cc.me(ctx); err == nil {
		printAccount(me)
	}
	return nil
}

// newLogoutCmd removes the saved API key and stops any background tunnels.
func newLogoutCmd(g *globalFlags) *cobra.Command {
	return &cobra.Command{
		Use:   "logout",
		Short: "Sign out",
		RunE: func(cmd *cobra.Command, _ []string) error {
			cfg, err := g.loadConfig(cmd)
			if err != nil {
				return err
			}
			if strings.TrimSpace(cfg.APIKey) == "" {
				ui.Info("already signed out")
				return nil
			}
			// Best-effort: tear down background tunnels so nothing keeps serving
			// under the old identity.
			addr := controlAddr(cfg)
			if daemonAlive(addr) {
				var tunnels []agent.Tunnel
				if controlGET(addr, "/tunnels", &tunnels) == nil {
					for _, t := range tunnels {
						_ = controlDelete(addr, "/tunnels/"+t.ID)
					}
				}
			}
			cfg.APIKey = ""
			if err := cfg.Save(agent.DefaultConfigPath()); err != nil {
				return err
			}
			ui.Success("signed out")
			return nil
		},
	}
}

// newWhoamiCmd shows the signed-in account, plan and usage.
func newWhoamiCmd(g *globalFlags) *cobra.Command {
	return &cobra.Command{
		Use:     "whoami",
		Aliases: []string{"account", "me"},
		Short:   "Show account, plan and usage",
		RunE: func(cmd *cobra.Command, _ []string) error {
			cfg, err := g.loadConfig(cmd)
			if err != nil {
				return err
			}
			if strings.TrimSpace(cfg.APIKey) == "" {
				return errNotSignedIn
			}
			cc := newCloudClient(cfg)
			me, err := cc.me(cmd.Context())
			if err != nil {
				return err
			}
			printAccount(me)
			return nil
		},
	}
}

// printAccount renders the account summary shown after login and by `whoami`.
func printAccount(me *meResp) {
	name := firstNonEmpty(me.User.Name, me.User.Email)
	plan := firstNonEmpty(me.Plan, me.Org.Plan)
	fmt.Printf("\n  %s\n", bold(name))
	if me.User.Email != "" && me.User.Email != name {
		fmt.Printf("  %s\n", me.User.Email)
	}
	fmt.Printf("\n  org    %s\n  plan   %s", firstNonEmpty(me.Org.Name, "—"), plan)
	if me.PlanExpiresAt != nil {
		fmt.Printf("  (expires %s)", me.PlanExpiresAt.Format("2006-01-02"))
	}
	fmt.Printf("\n  usage  %s in · %s out · %s req  (last 30d)\n\n",
		humanBytes(me.Usage.BytesIn), humanBytes(me.Usage.BytesOut), humanInt(me.Usage.Requests))
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func orDefault(v, def int) int {
	if v <= 0 {
		return def
	}
	return v
}
