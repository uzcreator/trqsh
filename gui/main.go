// Command rift-gui is the Rift desktop application: a Wails v3 shell (native
// window + WebView) around the same agent core the CLI uses (internal/agent).
// The React/TS frontend in frontend/ drives it through the AgentService binding.
package main

import (
	"embed"
	"log/slog"
	"os"

	"github.com/wailsapp/wails/v3/pkg/application"

	"github.com/rift/rift/internal/agent"
)

// The built frontend (frontend/dist) is embedded into the binary and served by
// Wails inside the WebView. Run `pnpm build` in frontend/ before `wails3 build`.
//
//go:embed all:frontend/dist
var assets embed.FS

// Populated at build time via -ldflags "-X main.version=...".
var version = "0.1.0"

func main() {
	log := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelInfo}))

	// One agent core, loaded from ~/.rift/rift.yml, shared with the CLI.
	cfg, err := agent.Load(agent.DefaultConfigPath())
	if err != nil {
		log.Warn("load config; using defaults", "err", err)
		cfg = agent.DefaultConfig()
	}
	svc := NewAgentService(agent.New(cfg, log), log)

	app := application.New(application.Options{
		Name:        "Rift",
		Description: "Expose localhost to the internet — fast.",
		Services: []application.Service{
			application.NewService(svc),
		},
		Assets: application.AssetOptions{
			Handler: application.AssetFileServerFS(assets),
		},
		Mac: application.MacOptions{
			// Keep running in the tray after the last window closes.
			ApplicationShouldTerminateAfterLastWindowClosed: false,
		},
	})

	win := app.NewWebviewWindowWithOptions(application.WebviewWindowOptions{
		Title:     "Rift",
		Width:     980,
		Height:    680,
		MinWidth:  760,
		MinHeight: 520,
		// The frontend renders its own titlebar (components/titlebar.tsx), so we
		// hide the native chrome and let that bar own the drag region.
		Frameless:        true,
		BackgroundColour: application.NewRGB(13, 13, 13),
		Mac: application.MacWindow{
			InvisibleTitleBarHeight: 44,
			Backdrop:                application.MacBackdropTranslucent,
			TitleBar:                application.MacTitleBarHiddenInset,
		},
		Windows: application.WindowsWindow{
			HiddenOnTaskbar: false,
		},
		URL: "/",
	})

	// Start the event pump (core.Events → "agent:event") and wire the tray.
	svc.Attach(app)
	setupTray(app, win)

	if err := app.Run(); err != nil {
		log.Error("application exited with error", "err", err)
		os.Exit(1)
	}
}
