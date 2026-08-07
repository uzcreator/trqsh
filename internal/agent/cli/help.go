package cli

// This file replaces cobra's stock help/usage rendering with trqsh's own: a
// branded header, commands grouped by purpose (not one flat alphabetical dump),
// copy-pasteable examples, and consistent color via the ui package. It's the
// single biggest lever on how "finished" the CLI feels, so it lives on its own.

import (
	"fmt"
	"io"
	"strings"

	"github.com/spf13/cobra"
	"github.com/trqsh-uz/trqsh/internal/agent/cli/ui"
)

// helpGroups defines the sections of the root help and the order of commands in
// each — deliberately by task flow (expose → manage → tear down), not
// alphabetically, so related actions read together. Any available command not
// listed here still shows up under "OTHER", so a newly-added command can never
// silently vanish from the help.
var helpGroups = []struct {
	title string
	names []string
}{
	{"CONSOLE", []string{"ui"}},
	{"TUNNELS", []string{"http", "tcp", "udp", "start", "ls", "open", "stop", "down"}},
	{"ACCOUNT", []string{"login", "logout", "whoami", "subdomains", "domains"}},
	{"SYSTEM", []string{"status", "config", "update", "version", "uninstall"}},
}

// rootExamples are the handful of invocations that cover most real usage. Each
// opens the console; the ones with arguments run that as the first command.
var rootExamples = []string{
	"trqsh                        open the console",
	"trqsh http 3000              expose port 3000, then keep working",
	"trqsh http 8080 --subdomain myapp",
	"trqsh login                  sign in",
}

// nameColWidth is the padded width of the command-name column in help listings.
// "subdomains" (10) is the longest command shown, so 11 leaves a clean gutter.
const nameColWidth = 11

// applyBranding installs trqsh's custom help and usage rendering across the
// whole command tree (children inherit the root's funcs).
func applyBranding(root *cobra.Command) {
	root.SetHelpFunc(brandedHelp)
	root.SetUsageFunc(brandedUsage)
}

// brandedHelp routes to the root overview or a single command's help.
func brandedHelp(cmd *cobra.Command, _ []string) {
	if cmd.HasParent() {
		printCommandHelp(cmd)
		return
	}
	printRootHelp(cmd)
}

// printRootHelp renders the top-level `trqsh` / `trqsh --help` screen.
func printRootHelp(root *cobra.Command) {
	out := root.OutOrStdout()
	byName := commandIndex(root)

	_, _ = fmt.Fprintf(out, "\n  %s %s\n", ui.AccentBold("trqsh"), ui.Gray(versionString()))
	_, _ = fmt.Fprintf(out, "  %s\n", root.Short)
	_, _ = fmt.Fprintf(out, "  %s\n\n", ui.Gray("An interactive console — run trqsh to open it; everything happens inside."))

	_, _ = fmt.Fprintf(out, "  %s\n    %s %s\n\n", ui.Title("USAGE"), ui.Bold("trqsh"), ui.Gray("[command …]   (opens the console, running any command you pass)"))

	shown := map[string]bool{}
	for _, g := range helpGroups {
		printGroup(out, g.title, g.names, byName, shown)
	}
	// Surface any available command not assigned to a group above, so the help
	// stays complete without hand-editing helpGroups for every new command.
	var extra []string
	for _, c := range root.Commands() {
		n := c.Name()
		if shown[n] || c.Hidden || !c.IsAvailableCommand() || n == "help" || n == "completion" {
			continue
		}
		extra = append(extra, n)
	}
	printGroup(out, "OTHER", extra, byName, shown)

	if fu := strings.TrimRight(root.PersistentFlags().FlagUsages(), "\n"); fu != "" {
		_, _ = fmt.Fprintf(out, "  %s\n%s\n\n", ui.Title("GLOBAL FLAGS"), indentLines(fu, "  "))
	}

	_, _ = fmt.Fprintf(out, "  %s\n", ui.Title("EXAMPLES"))
	for _, ex := range rootExamples {
		_, _ = fmt.Fprintf(out, "    %s%s\n", ui.Gray("$ "), ex)
	}
	_, _ = fmt.Fprintln(out)

	_, _ = fmt.Fprintf(out, "  Inside the console, type %s for all commands (%s to autocomplete).\n", ui.Accent("/help"), ui.Accent("/"))
	_, _ = fmt.Fprintf(out, "  Docs: %s\n\n", ui.Link("https://trqsh.uz/docs"))
}

// printGroup prints one titled section listing the named commands (in the given
// order), marking each as shown so the OTHER fallback doesn't repeat them.
func printGroup(out io.Writer, title string, names []string, byName map[string]*cobra.Command, shown map[string]bool) {
	var cmds []*cobra.Command
	for _, n := range names {
		c, ok := byName[n]
		if !ok || shown[n] || c.Hidden || !c.IsAvailableCommand() {
			continue
		}
		cmds = append(cmds, c)
		shown[n] = true
	}
	if len(cmds) == 0 {
		return
	}
	_, _ = fmt.Fprintf(out, "  %s\n", ui.Title(title))
	for _, c := range cmds {
		_, _ = fmt.Fprintf(out, "    %s  %s\n", ui.Bold(ui.Pad(c.Name(), nameColWidth)), c.Short)
	}
	_, _ = fmt.Fprintln(out)
}

// printCommandHelp renders the help for a single (sub)command: description,
// usage line, any subcommands, examples, and flags.
func printCommandHelp(cmd *cobra.Command) {
	out := cmd.OutOrStdout()
	_, _ = fmt.Fprintln(out)

	desc := cmd.Long
	if desc == "" {
		desc = cmd.Short
	}
	_, _ = fmt.Fprintf(out, "%s\n\n", indentLines(desc, "  "))

	_, _ = fmt.Fprintf(out, "  %s\n    %s\n\n", ui.Title("USAGE"), cmd.UseLine())

	if cmd.HasAvailableSubCommands() {
		_, _ = fmt.Fprintf(out, "  %s\n", ui.Title("COMMANDS"))
		for _, c := range cmd.Commands() {
			if !c.IsAvailableCommand() || c.Name() == "help" {
				continue
			}
			_, _ = fmt.Fprintf(out, "    %s  %s\n", ui.Bold(ui.Pad(c.Name(), nameColWidth)), c.Short)
		}
		_, _ = fmt.Fprintln(out)
	}

	if cmd.Example != "" {
		_, _ = fmt.Fprintf(out, "  %s\n%s\n\n", ui.Title("EXAMPLES"), indentLines(strings.TrimRight(cmd.Example, "\n"), "    "))
	}

	if fu := strings.TrimRight(cmd.LocalFlags().FlagUsages(), "\n"); fu != "" {
		_, _ = fmt.Fprintf(out, "  %s\n%s\n", ui.Title("FLAGS"), indentLines(fu, "  "))
	}
	if fu := strings.TrimRight(cmd.InheritedFlags().FlagUsages(), "\n"); fu != "" {
		_, _ = fmt.Fprintf(out, "\n  %s\n%s\n", ui.Title("GLOBAL FLAGS"), indentLines(fu, "  "))
	}
}

// brandedUsage is what cobra prints when it wants a usage hint (e.g. on a flag
// parse error). It's intentionally terse — the full help is a keystroke away.
func brandedUsage(cmd *cobra.Command) error {
	out := cmd.OutOrStderr()
	_, _ = fmt.Fprintf(out, "\n  %s %s\n", ui.Gray("Usage:"), cmd.UseLine())
	_, _ = fmt.Fprintf(out, "  %s %s\n", ui.Gray("Help: "), ui.Accent(cmd.CommandPath()+" --help"))
	return nil
}

// commandIndex maps a root's direct subcommands by name for quick lookup.
func commandIndex(root *cobra.Command) map[string]*cobra.Command {
	m := make(map[string]*cobra.Command, len(root.Commands()))
	for _, c := range root.Commands() {
		m[c.Name()] = c
	}
	return m
}

// indentLines prefixes every non-blank line of s with prefix, preserving the
// internal alignment cobra already applied to flag listings.
func indentLines(s, prefix string) string {
	lines := strings.Split(s, "\n")
	for i, l := range lines {
		if strings.TrimSpace(l) == "" {
			lines[i] = ""
			continue
		}
		lines[i] = prefix + l
	}
	return strings.Join(lines, "\n")
}
