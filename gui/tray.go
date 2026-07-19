package main

import (
	"github.com/wailsapp/wails/v3/pkg/application"
)

// setupTray installs a system-tray icon so the app keeps tunnels alive in the
// background (MinimizeToTray) and can be reopened quickly. The tray icon asset
// is provided per-platform by the Part 08 packaging step; Wails falls back to a
// default when none is set.
func setupTray(app *application.App, win *application.WebviewWindow) {
	tray := app.NewSystemTray()
	tray.SetLabel("Rift")

	menu := app.NewMenu()
	menu.Add("Open Rift").OnClick(func(_ *application.Context) {
		win.Show()
		win.Focus()
	})
	menu.AddSeparator()
	menu.Add("Quit Rift").OnClick(func(_ *application.Context) {
		app.Quit()
	})
	tray.SetMenu(menu)

	// Left-clicking the tray icon toggles the window.
	tray.OnClick(func() {
		if win.IsVisible() {
			win.Hide()
			return
		}
		win.Show()
		win.Focus()
	})
}
