// The desktop GUI is a SEPARATE Go module, intentionally excluded from the root
// go.work (which uses only "."). This keeps `go build ./...` / `go test ./...`
// at the repo root green in CGO-less CI, because the Wails v3 shell needs CGO +
// a platform WebView (WebView2 / WebKit) that isn't available there. Build this
// module with the Wails toolchain instead (see gui/README.md).
//
// The trqsh module is consumed via a relative replace so the GUI embeds the very
// same agent core (internal/agent) the CLI uses — no drift.
module github.com/trqsh-uz/trqsh/gui

go 1.25.0

require (
	github.com/trqsh-uz/trqsh v0.0.0
	github.com/wailsapp/wails/v3 v3.0.0-alpha.11
)

replace github.com/trqsh-uz/trqsh => ../

// NOTE: the exact wails/v3 version must match the installed @wailsio/runtime
// (frontend/package.json). `wails3 update` / `go mod tidy` under the Wails
// toolchain will resolve the full dependency set; it is not vendored here.
