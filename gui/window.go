package main

import (
	"sync"

	"github.com/wailsapp/wails/v3/pkg/application"
)

// WindowService exposes native window controls to the frontend. The app runs a
// frameless window (main.go) so the React titlebar draws its own min/max/close
// buttons on Windows/Linux; those buttons call these methods so the app behaves
// like a first-class native window instead of a fixed panel.
//
// It is bound as "WindowService.<Method>" (see frontend/src/lib/agent.ts).
type WindowService struct {
	mu  sync.Mutex
	app *application.App
	win *application.WebviewWindow
}

// set wires the running app + window. Called once from main after both exist;
// the frontend only invokes these methods well after startup, so the late set
// is safe (and still mutex-guarded).
func (w *WindowService) set(app *application.App, win *application.WebviewWindow) {
	w.mu.Lock()
	w.app, w.win = app, win
	w.mu.Unlock()
}

func (w *WindowService) window() *application.WebviewWindow {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.win
}

// Minimize sends the window to the taskbar/dock.
func (w *WindowService) Minimize() {
	if win := w.window(); win != nil {
		win.Minimise()
	}
}

// ToggleMaximize maximizes the window, or restores it if already maximized.
func (w *WindowService) ToggleMaximize() {
	win := w.window()
	if win == nil {
		return
	}
	if win.IsMaximised() {
		win.UnMaximise()
		return
	}
	win.Maximise()
}

// IsMaximized reports whether the window currently fills the work area, so the
// titlebar can render the correct restore/maximize glyph.
func (w *WindowService) IsMaximized() bool {
	if win := w.window(); win != nil {
		return win.IsMaximised()
	}
	return false
}

// Hide removes the window (minimize-to-tray). The tray keeps tunnels alive and
// can reopen it.
func (w *WindowService) Hide() {
	if win := w.window(); win != nil {
		win.Hide()
	}
}

// Show brings the window back and focuses it.
func (w *WindowService) Show() {
	if win := w.window(); win != nil {
		win.Show()
		win.Focus()
	}
}

// Quit terminates the whole application (used by the close button when
// minimize-to-tray is off, and by the command palette / tray).
func (w *WindowService) Quit() {
	w.mu.Lock()
	app := w.app
	w.mu.Unlock()
	if app != nil {
		app.Quit()
	}
}
