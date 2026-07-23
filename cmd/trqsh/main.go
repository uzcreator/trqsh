// Command trqsh is the trqsh agent and CLI — the open-source client that exposes
// a local service through the edge. See plan/03-agent-cli.md.
package main

import "github.com/trqsh-uz/trqsh/internal/agent/cli"

func main() {
	cli.Execute()
}
