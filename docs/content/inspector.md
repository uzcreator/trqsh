# Request inspector

Every `rift` agent runs a local inspector — a web UI at
<http://localhost:4040> that shows traffic flowing through your HTTP tunnels in
real time.

## Open it

Start any HTTP tunnel and visit the address Rift prints:

```
Inspect     http://localhost:4040
```

## What you can do

- **See every request** as it happens: method, path, status, and timing.
- **Inspect** full request and response headers and bodies.
- **Replay** any captured request with one click — invaluable for debugging
  webhooks without asking the sender to fire again.

## Replay

Open a request in the inspector and choose **Replay**. Rift re-sends the exact same
request to your local server, so you can iterate on your handler and re-run it
instantly.

## History retention

How long captures are kept depends on your plan:

- Free: 1 hour
- Pro / Team: 30 days (also available in the cloud dashboard inspector)

The local inspector keeps captures **on your machine**; nothing about request
bodies leaves your computer beyond what your plan's cloud inspector retains.

## Change the address

The inspector listens on `127.0.0.1:4040` by default. Change or disable it in your
[config file](/docs/configuration):

```yaml
inspector:
  enabled: true
  addr: "127.0.0.1:4040"
```

## In the desktop app

The [desktop app](/docs/desktop-gui) has the same inspector built in, with a live
traffic view and replay — no separate browser tab needed.
