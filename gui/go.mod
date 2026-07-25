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
	github.com/wailsapp/wails/v3 v3.0.0-alpha2.117
)

require (
	github.com/adrg/xdg v0.5.3 // indirect
	github.com/coder/websocket v1.8.14 // indirect
	github.com/go-ole/go-ole v1.3.0 // indirect
	github.com/godbus/dbus/v5 v5.2.2 // indirect
	github.com/hashicorp/yamux v0.1.2 // indirect
	github.com/jchv/go-winloader v0.0.0-20250406163304-c1995be93bd1 // indirect
	github.com/kr/text v0.2.0 // indirect
	github.com/mattn/go-colorable v0.1.14 // indirect
	github.com/mattn/go-isatty v0.0.20 // indirect
	github.com/quic-go/quic-go v0.60.0 // indirect
	github.com/rogpeppe/go-internal v1.14.1 // indirect
	golang.org/x/crypto v0.51.0 // indirect
	golang.org/x/net v0.55.0 // indirect
	golang.org/x/sys v0.45.0 // indirect
	google.golang.org/protobuf v1.36.11 // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
)

replace github.com/trqsh-uz/trqsh => ../

// NOTE: the exact wails/v3 version must match the installed @wailsio/runtime
// (frontend/package.json). `wails3 update` / `go mod tidy` under the Wails
// toolchain will resolve the full dependency set; it is not vendored here.
