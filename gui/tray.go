package main

import (
	"fmt"

	"github.com/wailsapp/wails/v3/pkg/application"
)

// Tray is the system-tray controller. It keeps the app resident so tunnels stay
// alive when the window is hidden (minimize-to-tray), reflects live connection
// state in its label, and offers quick actions that read the current tunnel set
// at click time. refresh() is wired to AgentService.OnState so it re-renders
// whenever the connection or tunnel set changes.
type Tray struct {
	app  *application.App
	win  *application.WebviewWindow
	svc  *AgentService
	tray *application.SystemTray
}

// setupTray installs the tray and returns the controller. The tray icon asset is
// supplied per-platform by the Part 08 packaging step; Wails falls back to a
// default when none is set.
func setupTray(app *application.App, win *application.WebviewWindow, svc *AgentService) *Tray {
	t := &Tray{app: app, win: win, svc: svc, tray: app.NewSystemTray()}

	// Left-clicking the tray icon toggles window visibility.
	t.tray.OnClick(func() {
		if t.win.IsVisible() {
			t.win.Hide()
			return
		}
		t.win.Show()
		t.win.Focus()
	})

	t.refresh()
	return t
}

// refresh rebuilds the tray menu and label from current agent state.
func (t *Tray) refresh() {
	st := t.svc.Status()
	web := t.svc.webTunnels()

	label := "trqsh"
	switch {
	case st.Connected && len(web) > 0:
		label = fmt.Sprintf("trqsh — %d active", len(web))
	case st.Connected:
		label = "trqsh — connected"
	}
	t.tray.SetLabel(label)

	menu := t.app.NewMenu()
	menu.Add(label)
	menu.AddSeparator()

	menu.Add("Open trqsh").OnClick(func(_ *application.Context) {
		t.win.Show()
		t.win.Focus()
	})
	menu.Add("New tunnel…").OnClick(func(_ *application.Context) {
		t.win.Show()
		t.win.Focus()
		t.app.EmitEvent("ui:new-tunnel")
	})

	if len(web) > 0 {
		menu.AddSeparator()
		for _, tn := range web {
			url := tn.PublicURL
			menu.Add("↗  " + url).OnClick(func(_ *application.Context) {
				_ = openBrowser(url)
			})
		}
	}

	menu.AddSeparator()
	menu.Add("Quit trqsh").OnClick(func(_ *application.Context) {
		t.app.Quit()
	})

	t.tray.SetMenu(menu)
}
