# Desktop app

trqsh ships a polished native app for **macOS, Windows, and Linux**, built on the
same open-source agent core as the CLI. If you'd rather click than type, start
here.

## Install

Download the signed app for your OS from the [download page](/download). macOS
builds are notarized; Windows builds are Authenticode-signed.

## What's inside

- **One-click tunnels** — pick a port and protocol, hit start, copy the URL.
- **Live tunnel list** — every active tunnel with its public URL, a copy button,
  live traffic sparkline, uptime, and request rate. Copy any tunnel as a ready
  `curl` command.
- **Built-in inspector** — the same request inspector and replay as
  `localhost:4040`, embedded in the app, with search, method/status filters,
  request/response tabs, JSON pretty-printing, and copy-as-cURL.
- **Command palette** — press `⌘K` / `Ctrl-K` to jump anywhere, start a tunnel,
  or run any action. Shortcuts: `⌘N` new tunnel, `⌘1`–`⌘4` switch screens.
- **Account & billing** — see your plan, session usage meters, and an upgrade shortcut.
- **System tray** — keep tunnels running quietly in the background; the tray
  reflects your connection state and lists active tunnels.
- **Auto-update** — the app checks for and applies signed updates automatically.
- **Native window** — real minimize / maximize / close, a collapsible sidebar,
  light & dark themes, and a layout that adapts as you resize.

## Security

The desktop app is built to be safe by default: the embedded UI runs under a
strict Content-Security-Policy with no remote code, external links are restricted
to `http`/`https`, and your API key is masked by default and never displayed
unless you reveal it.

## Sign in

On first launch, sign in with the same account you use on the web. Your API key is
stored securely in the OS keychain.

## GUI and CLI together

The desktop app and the CLI share your account and config. A reserved subdomain or
custom domain set up in one is available in the other. Use whichever fits the
moment — the app for everyday work, the [CLI](/docs/quickstart) for scripts and CI.
