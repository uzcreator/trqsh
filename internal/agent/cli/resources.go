package cli

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"
	"github.com/trqsh-uz/trqsh/internal/agent"
)

// newSubdomainsCmd lists reserved subdomains, with reserve/release subcommands.
func newSubdomainsCmd(g *globalFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "subdomains",
		Aliases: []string{"subs", "subdomain"},
		Short:   "Manage reserved subdomains",
		Example: "trqsh subdomains\n" +
			"trqsh subdomains reserve myapp\n" +
			"trqsh subdomains release myapp",
		RunE: func(cmd *cobra.Command, _ []string) error {
			cfg, err := requireAuth(g, cmd)
			if err != nil {
				return err
			}
			cc := newCloudClient(cfg)
			subs, err := cc.listSubdomains(cmd.Context())
			if err != nil {
				return err
			}
			if len(subs) == 0 {
				fmt.Println("no reserved subdomains — reserve one with `trqsh subdomains reserve <name>`")
				return nil
			}
			base := baseDomainOf(cfg)
			tw := newTab()
			_, _ = fmt.Fprintln(tw, "SUBDOMAIN\tHOST\tRESERVED")
			for _, s := range subs {
				_, _ = fmt.Fprintf(tw, "%s\t%s.%s\t%s\n", s.Subdomain, s.Subdomain, base, s.CreatedAt.Format("2006-01-02"))
			}
			return tw.Flush()
		},
	}
	cmd.AddCommand(
		&cobra.Command{
			Use:   "reserve <name>",
			Short: "Reserve a subdomain (needs a paid plan)",
			Args:  cobra.ExactArgs(1),
			RunE: func(cmd *cobra.Command, args []string) error {
				cfg, err := requireAuth(g, cmd)
				if err != nil {
					return err
				}
				cc := newCloudClient(cfg)
				s, err := cc.reserveSubdomain(cmd.Context(), strings.ToLower(strings.TrimSpace(args[0])))
				if err != nil {
					return err
				}
				fmt.Printf("reserved %s.%s\n", s.Subdomain, baseDomainOf(cfg))
				return nil
			},
		},
		&cobra.Command{
			Use:   "release <name|id>",
			Short: "Release a reserved subdomain",
			Args:  cobra.ExactArgs(1),
			RunE: func(cmd *cobra.Command, args []string) error {
				cfg, err := requireAuth(g, cmd)
				if err != nil {
					return err
				}
				cc := newCloudClient(cfg)
				subs, err := cc.listSubdomains(cmd.Context())
				if err != nil {
					return err
				}
				id := resolveSubdomainID(subs, args[0])
				if id == "" {
					return fmt.Errorf("no reserved subdomain matched %q (see `trqsh subdomains`)", args[0])
				}
				if err := cc.releaseSubdomain(cmd.Context(), id); err != nil {
					return err
				}
				fmt.Printf("released %s\n", args[0])
				return nil
			},
		},
	)
	return cmd
}

// newDomainsCmd lists custom domains, with add/verify subcommands.
func newDomainsCmd(g *globalFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "domains",
		Aliases: []string{"domain"},
		Short:   "Manage custom domains",
		Example: "trqsh domains\n" +
			"trqsh domains add example.com\n" +
			"trqsh domains verify example.com",
		RunE: func(cmd *cobra.Command, _ []string) error {
			cfg, err := requireAuth(g, cmd)
			if err != nil {
				return err
			}
			cc := newCloudClient(cfg)
			domains, err := cc.listDomains(cmd.Context())
			if err != nil {
				return err
			}
			if len(domains) == 0 {
				fmt.Println("no custom domains — add one with `trqsh domains add <domain>`")
				return nil
			}
			tw := newTab()
			_, _ = fmt.Fprintln(tw, "DOMAIN\tSTATUS\tCERT\tADDED")
			for _, d := range domains {
				status := "pending"
				if d.VerifiedAt != nil {
					status = "verified"
				}
				_, _ = fmt.Fprintf(tw, "%s\t%s\t%s\t%s\n", d.Domain, status, firstNonEmpty(d.CertStatus, "—"), d.CreatedAt.Format("2006-01-02"))
			}
			return tw.Flush()
		},
	}
	cmd.AddCommand(
		&cobra.Command{
			Use:   "add <domain>",
			Short: "Add a custom domain and print the DNS records to set",
			Args:  cobra.ExactArgs(1),
			RunE: func(cmd *cobra.Command, args []string) error {
				cfg, err := requireAuth(g, cmd)
				if err != nil {
					return err
				}
				cc := newCloudClient(cfg)
				res, err := cc.addDomain(cmd.Context(), strings.ToLower(strings.TrimSpace(args[0])))
				if err != nil {
					return err
				}
				fmt.Printf("added %s\n\n  Add these DNS records, then run `trqsh domains verify %s`:\n\n", res.Domain.Domain, res.Domain.Domain)
				tw := newTab()
				_, _ = fmt.Fprintln(tw, "  TYPE\tNAME\tVALUE")
				if res.DNSInstructions["txt_name"] != "" {
					_, _ = fmt.Fprintf(tw, "  TXT\t%s\t%s\n", res.DNSInstructions["txt_name"], res.DNSInstructions["txt_value"])
				}
				if res.DNSInstructions["cname_name"] != "" {
					_, _ = fmt.Fprintf(tw, "  CNAME\t%s\t%s\n", res.DNSInstructions["cname_name"], res.DNSInstructions["cname_value"])
				}
				_ = tw.Flush()
				fmt.Println()
				return nil
			},
		},
		&cobra.Command{
			Use:   "verify <domain|id>",
			Short: "Check DNS and verify a custom domain",
			Args:  cobra.ExactArgs(1),
			RunE: func(cmd *cobra.Command, args []string) error {
				cfg, err := requireAuth(g, cmd)
				if err != nil {
					return err
				}
				cc := newCloudClient(cfg)
				domains, err := cc.listDomains(cmd.Context())
				if err != nil {
					return err
				}
				id := resolveDomainID(domains, args[0])
				if id == "" {
					return fmt.Errorf("no domain matched %q (see `trqsh domains`)", args[0])
				}
				d, err := cc.verifyDomain(cmd.Context(), id)
				if err != nil {
					return err
				}
				if d.VerifiedAt != nil {
					fmt.Printf("verified %s ✓\n", d.Domain)
				} else {
					fmt.Printf("%s is not verified yet\n", d.Domain)
				}
				return nil
			},
		},
	)
	return cmd
}

// requireAuth loads config and fails early if no API key is stored.
func requireAuth(g *globalFlags, cmd *cobra.Command) (agent.Config, error) {
	cfg, err := g.loadConfig(cmd)
	if err != nil {
		return cfg, err
	}
	if strings.TrimSpace(cfg.APIKey) == "" {
		return cfg, errNotSignedIn
	}
	return cfg, nil
}

func resolveSubdomainID(subs []subdomain, q string) string {
	q = strings.TrimSpace(q)
	for _, s := range subs {
		if s.ID == q || strings.EqualFold(s.Subdomain, q) {
			return s.ID
		}
	}
	return ""
}

func resolveDomainID(domains []domain, q string) string {
	q = strings.TrimSpace(q)
	for _, d := range domains {
		if d.ID == q || strings.EqualFold(d.Domain, q) {
			return d.ID
		}
	}
	return ""
}
