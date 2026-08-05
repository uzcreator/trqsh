package inspect

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
	"time"
)

// Server serves the inspector web UI and JSON API.
type Server struct {
	rec    *Recorder
	client *http.Client
}

// NewServer builds an inspector server over the recorder.
func NewServer(rec *Recorder) *Server {
	return &Server{rec: rec, client: &http.Client{Timeout: 30 * time.Second}}
}

// Handler returns the HTTP handler for the inspector.
func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = io.WriteString(w, indexHTML)
	})
	mux.HandleFunc("GET /api/requests", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, http.StatusOK, s.rec.List())
	})
	mux.HandleFunc("GET /api/requests/{id}", func(w http.ResponseWriter, r *http.Request) {
		c, ok := s.rec.Get(r.PathValue("id"))
		if !ok {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		writeJSON(w, http.StatusOK, c)
	})
	mux.HandleFunc("POST /api/requests/{id}/replay", func(w http.ResponseWriter, r *http.Request) {
		c, ok := s.rec.Get(r.PathValue("id"))
		if !ok {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		replayed, err := s.replay(r.Context(), c)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadGateway)
			return
		}
		writeJSON(w, http.StatusOK, replayed)
	})
	mux.HandleFunc("GET /api/stream", s.stream)
	return mux
}

// Serve runs the inspector until ctx is canceled.
func (s *Server) Serve(ctx context.Context, addr string) error {
	srv := &http.Server{
		Addr:              addr,
		Handler:           s.Handler(),
		ReadHeaderTimeout: 5 * time.Second, // slowloris guard (gosec G112)
		IdleTimeout:       120 * time.Second,
		// No WriteTimeout: /api/stream serves long-lived SSE.
	}
	// #nosec G118 -- ctx is already Done() by the time this fires; a fresh context.Background() timeout is required for Shutdown's own grace period (standard graceful-shutdown pattern)
	go func() {
		<-ctx.Done()
		sc, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		_ = srv.Shutdown(sc)
	}()
	if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		return err
	}
	return nil
}

// replay re-issues a captured request against its local address.
func (s *Server) replay(ctx context.Context, c CapturedRequest) (CapturedRequest, error) {
	if c.LocalAddr == "" {
		return CapturedRequest{}, fmt.Errorf("no local address recorded")
	}
	host := c.LocalAddr
	if _, _, err := net.SplitHostPort(host); err != nil {
		host = host + ":80"
	}
	url := "http://" + host + c.Path
	req, err := http.NewRequestWithContext(ctx, c.Method, url, bytes.NewReader(c.ReqBody))
	if err != nil {
		return CapturedRequest{}, err
	}
	for k, v := range c.ReqHeaders {
		if isHopByHop(k) {
			continue
		}
		req.Header.Set(k, v)
	}
	start := time.Now()
	resp, err := s.client.Do(req)
	if err != nil {
		return CapturedRequest{}, err
	}
	defer func() { _ = resp.Body.Close() }()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, MaxBodyCapture))

	out := CapturedRequest{
		TunnelID:    c.TunnelID,
		Proto:       c.Proto,
		Method:      c.Method,
		Host:        c.Host,
		Path:        c.Path + " (replay)",
		Status:      resp.StatusCode,
		StartedAt:   start,
		DurationMs:  time.Since(start).Milliseconds(),
		ReqHeaders:  c.ReqHeaders,
		RespHeaders: headerMap(resp.Header),
		ReqBody:     c.ReqBody,
		RespBody:    capBody(body),
		LocalAddr:   c.LocalAddr,
	}
	return s.rec.Add(out), nil
}

func (s *Server) stream(w http.ResponseWriter, r *http.Request) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming unsupported", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	id, ch := s.rec.Subscribe()
	defer s.rec.Unsubscribe(id)
	for {
		select {
		case <-r.Context().Done():
			return
		case c := <-ch:
			b, _ := json.Marshal(c)
			_, _ = fmt.Fprintf(w, "data: %s\n\n", b)
			flusher.Flush()
		}
	}
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func headerMap(h http.Header) map[string]string {
	m := make(map[string]string, len(h))
	for k, v := range h {
		m[k] = strings.Join(v, ", ")
	}
	return m
}

func isHopByHop(k string) bool {
	switch http.CanonicalHeaderKey(k) {
	case "Connection", "Keep-Alive", "Transfer-Encoding", "Upgrade", "Proxy-Connection", "Te", "Trailer":
		return true
	}
	return false
}

const indexHTML = `<!doctype html>
<html><head><meta charset="utf-8"><title>trqsh Inspector</title>
<style>
:root{color-scheme:dark}
body{font-family:ui-sans-serif,system-ui,sans-serif;margin:0;background:#0b0d12;color:#e6e8ee;display:flex;height:100vh}
header{padding:.6rem 1rem;font-weight:600;border-bottom:1px solid #1c2130;background:#0e1119}
#list{width:38%;overflow:auto;border-right:1px solid #1c2130}
#detail{flex:1;overflow:auto;padding:1rem}
.row{padding:.5rem .8rem;border-bottom:1px solid #151a26;cursor:pointer;font-size:.86rem;display:flex;gap:.5rem;align-items:center}
.row:hover{background:#141926}
.m{font-weight:700;color:#7aa2ff;min-width:3.2rem}
.s{margin-left:auto;font-variant-numeric:tabular-nums}
.ok{color:#57d18a}.err{color:#ff6b6b}
pre{background:#0e1119;border:1px solid #1c2130;border-radius:.4rem;padding:.6rem;overflow:auto;white-space:pre-wrap;word-break:break-word}
button{background:#7aa2ff;color:#04101f;border:0;border-radius:.35rem;padding:.4rem .8rem;font-weight:600;cursor:pointer}
h2{margin:.2rem 0 .6rem;font-size:1rem}small{color:#9aa3b2}
.col{display:flex;flex-direction:column;height:100vh}
</style></head>
<body>
<div class="col" id="list">
  <header>trqsh Inspector <small id="cnt"></small></header>
  <div id="rows"></div>
</div>
<div id="detail"><p><small>Select a request.</small></p></div>
<script>
let sel=null;
async function load(){
  const rs=await (await fetch('/api/requests')).json();
  document.getElementById('cnt').textContent=rs.length?('· '+rs.length):'';
  document.getElementById('rows').innerHTML=rs.map(r=>
    '<div class="row" onclick="show(\''+r.id+'\')"><span class="m">'+r.method+'</span><span>'+
    (r.path||'')+'</span><span class="s '+(r.status>=400?'err':'ok')+'">'+(r.status||'—')+'</span></div>').join('');
}
function h(o){return o?Object.entries(o).map(([k,v])=>k+': '+v).join('\n'):''}
function b64(s){try{return atob(s)}catch(e){return ''}}
async function show(id){
  sel=id;
  const r=await (await fetch('/api/requests/'+id)).json();
  document.getElementById('detail').innerHTML=
    '<h2>'+r.method+' '+r.path+' <small>'+(r.status||'')+' · '+r.duration_ms+'ms</small></h2>'+
    '<button onclick="replay(\''+id+'\')">Replay</button>'+
    '<h2>Request headers</h2><pre>'+h(r.req_headers)+'</pre>'+
    (r.req_body?'<h2>Request body</h2><pre>'+b64(r.req_body)+'</pre>':'')+
    '<h2>Response headers</h2><pre>'+h(r.resp_headers)+'</pre>'+
    (r.resp_body?'<h2>Response body</h2><pre>'+b64(r.resp_body)+'</pre>':'');
}
async function replay(id){await fetch('/api/requests/'+id+'/replay',{method:'POST'});load();}
load();setInterval(load,1500);
</script>
</body></html>`
