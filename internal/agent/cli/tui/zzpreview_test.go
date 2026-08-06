package tui

import (
	"fmt"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/trqsh-uz/trqsh/internal/agent"
)

func renderAt(w, h int, frame int) string {
	m := newModel(Options{Version: "0.1.7"}, newClient("127.0.0.1:0", ""))
	m = feed(m, tea.WindowSizeMsg{Width: w, Height: h})
	m.acct = &account{Plan: "pro"}
	m.acct.User.Name = "Otash"
	m.acct.User.Email = "otashdev1@gmail.com"
	m.status = agent.Status{Connected: true, Edge: "eu", Kind: "quic"}
	m.mascotFrame = frame
	m.refreshWelcome()
	m.relayout()
	return m.View()
}

func TestPreviewRender(t *testing.T) {
	fmt.Println("\n----WIDE (100) eyes-open----")
	fmt.Println(renderAt(100, 16, 0))
	fmt.Println("----WIDE blink----")
	for _, l := range splitN(renderAt(100, 16, 4), 5) {
		fmt.Println(l)
	}
	fmt.Println("----NARROW (54) no tips----")
	fmt.Println(renderAt(54, 16, 0))
}

func splitN(s string, n int) []string {
	var out []string
	cur := ""
	c := 0
	for _, r := range s {
		cur += string(r)
		if r == '\n' {
			out = append(out, cur[:len(cur)-1])
			cur = ""
			c++
			if c >= n {
				break
			}
		}
	}
	return out
}
