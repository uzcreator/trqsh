package api

import (
	_ "embed"
	"net/http"
	"runtime"
	"sort"
	"time"
)

// Version is the API build version, overridable at link time with
//
//	-X github.com/rift/rift/internal/api.Version=v1.2.3
var Version = "0.1.0-dev"

// processStart marks server boot, for uptime on /status.
var processStart = time.Now()

// openAPISpec is the canonical Part 05 contract, embedded so the binary serves
// its own docs. Keep it in sync with docs/openapi.yaml via `make openapi-sync`
// (CI fails on drift).
//
//go:embed openapi.yaml
var openAPISpec []byte

// handleOpenAPISpec serves the raw OpenAPI document that powers /docs.
func (s *Server) handleOpenAPISpec(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/yaml; charset=utf-8")
	w.Header().Set("Cache-Control", "no-cache")
	_, _ = w.Write(openAPISpec)
}

// handleStatus reports live backend health + configuration. Public and
// side-effect free; drives the status header on /docs and any uptime probe.
func (s *Server) handleStatus(w http.ResponseWriter, _ *http.Request) {
	providers := make([]string, 0, len(s.providers))
	for k := range s.providers {
		providers = append(providers, k)
	}
	sort.Strings(providers)

	writeJSON(w, http.StatusOK, map[string]any{
		"service":         "rift-control-api",
		"status":          "ok",
		"version":         Version,
		"store":           storeKind(s.cfg),
		"base_domain":     s.cfg.BaseDomain,
		"billing_enabled": s.billing.Config().Enabled(),
		"oauth_providers": providers,
		"started_at":      processStart.UTC().Format(time.RFC3339),
		"uptime_seconds":  int(time.Since(processStart).Seconds()),
		"go_version":      runtime.Version(),
		"goroutines":      runtime.NumGoroutine(),
		"now":             time.Now().UTC().Format(time.RFC3339),
	})
}

// handleDocsUI serves a Swagger-UI page (with a live status header) for the API.
func (s *Server) handleDocsUI(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = w.Write([]byte(docsHTML))
}

// docsHTML is a self-hosted Swagger-UI shell (assets from a pinned CDN) with a
// brand header that live-polls /status. No backticks inside — it is itself a raw
// string literal.
const docsHTML = `<!doctype html>
<html lang="en">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<title>Rift Control API — Docs</title>
<link rel="icon" href="data:,">
<link rel="stylesheet" href="https://unpkg.com/swagger-ui-dist@5.17.14/swagger-ui.css">
<style>
  :root { --brand:#2A78D6; }
  body { margin:0; background:#f7f7f5; font-family: system-ui,-apple-system,"Segoe UI",Roboto,sans-serif; }
  .rift-bar { display:flex; align-items:center; gap:14px; padding:12px 20px; background:#0d1526; color:#fff; position:sticky; top:0; z-index:10; }
  .rift-logo { width:30px; height:30px; border-radius:8px; background:var(--brand); display:flex; align-items:center; justify-content:center; font-weight:700; }
  .rift-title { font-weight:600; font-size:15px; line-height:1.1; }
  .rift-sub { color:#9aa4b2; font-size:12px; margin-top:2px; }
  .rift-pills { margin-left:auto; display:flex; gap:8px; flex-wrap:wrap; justify-content:flex-end; }
  .pill { display:inline-flex; align-items:center; gap:6px; background:#182338; color:#cbd5e1; border:1px solid #24314c; border-radius:999px; padding:4px 10px; font-size:12px; white-space:nowrap; }
  .dot { width:8px; height:8px; border-radius:50%; background:#8a8f98; }
  .dot.ok { background:#16c60c; box-shadow:0 0 0 3px rgba(22,198,12,.18); }
  .swagger-ui .topbar { display:none; }
  #swagger-ui { max-width:1080px; margin:0 auto; }
</style>
</head>
<body>
<div class="rift-bar">
  <div class="rift-logo">R</div>
  <div>
    <div class="rift-title">Rift &middot; Control API</div>
    <div class="rift-sub">Interactive reference &amp; live backend status</div>
  </div>
  <div class="rift-pills" id="pills">
    <span class="pill"><span class="dot"></span><span>connecting&hellip;</span></span>
  </div>
</div>
<div id="swagger-ui"></div>
<script src="https://unpkg.com/swagger-ui-dist@5.17.14/swagger-ui-bundle.js" crossorigin></script>
<script>
window.ui = SwaggerUIBundle({
  url: '/openapi.yaml',
  dom_id: '#swagger-ui',
  deepLinking: true,
  tryItOutEnabled: true,
  docExpansion: 'list',
  defaultModelsExpandDepth: 0
});
function fmtUptime(s){ if(s<60) return s+'s'; if(s<3600) return Math.floor(s/60)+'m'; return Math.floor(s/3600)+'h '+Math.floor((s%3600)/60)+'m'; }
function esc(x){ var d=document.createElement('div'); d.textContent=String(x); return d.innerHTML; }
function loadStatus(){
  fetch('/status').then(function(r){ return r.json(); }).then(function(d){
    var p = '';
    p += '<span class="pill"><span class="dot ok"></span>online</span>';
    p += '<span class="pill">v' + esc(d.version||'?') + '</span>';
    p += '<span class="pill">store: ' + esc(d.store||'?') + '</span>';
    p += '<span class="pill">domain: ' + esc(d.base_domain||'-') + '</span>';
    p += '<span class="pill">billing: ' + (d.billing_enabled ? 'on' : 'off') + '</span>';
    p += '<span class="pill">up ' + esc(fmtUptime(d.uptime_seconds||0)) + '</span>';
    document.getElementById('pills').innerHTML = p;
  }).catch(function(){
    document.getElementById('pills').innerHTML = '<span class="pill"><span class="dot"></span>status unavailable</span>';
  });
}
loadStatus();
setInterval(loadStatus, 10000);
</script>
</body>
</html>`
