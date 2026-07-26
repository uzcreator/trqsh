package agent

import (
	"context"
	"crypto/rand"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// LocalAPI serves the Core over loopback HTTP (JSON + SSE) so the desktop app
// (and `trqsh status`/`stop`) can drive the same agent core out-of-process. It
// re-broadcasts the core's single Events() stream to any number of connected
// clients.
//
// Security: the API binds to loopback only. When a token is set (SetToken) every
// request except /healthz must present it as `Authorization: Bearer <token>` or
// `?token=<token>`. A Host-header loopback guard blocks DNS-rebinding, and CORS
// is reflected so the desktop app's WebView origin can call it.
type LocalAPI struct {
	core  *Agent
	token string

	mu   sync.Mutex
	subs map[int]chan Event
	next int
}

// NewLocalAPI builds a control API over the agent core.
func NewLocalAPI(core *Agent) *LocalAPI {
	return &LocalAPI{core: core, subs: make(map[int]chan Event)}
}

// SetToken enables bearer-token auth on the control API. Pass "" to disable it
// (dev only). Returns the receiver for chaining.
func (l *LocalAPI) SetToken(tok string) *LocalAPI {
	l.token = tok
	return l
}

// Serve runs the control API until ctx is canceled.
func (l *LocalAPI) Serve(ctx context.Context, addr string) error {
	go l.pump(ctx)
	srv := &http.Server{
		Addr:              addr,
		Handler:           l.secure(l.Handler()),
		ReadHeaderTimeout: 5 * time.Second, // slowloris guard (gosec G112)
		IdleTimeout:       120 * time.Second,
		// No WriteTimeout: /events serves long-lived SSE to the desktop app.
	}
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

// sessionInfo tells the desktop app whether an API key is stored (signed in) and
// the live transport status, without polluting the frozen Status wire type.
type sessionInfo struct {
	Authed bool   `json:"authed"`
	Status Status `json:"status"`
}

// Handler returns the control API routes.
func (l *LocalAPI) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("ok"))
	})
	mux.HandleFunc("GET /status", func(w http.ResponseWriter, _ *http.Request) {
		apiWriteJSON(w, http.StatusOK, l.core.Status())
	})
	mux.HandleFunc("GET /session", func(w http.ResponseWriter, _ *http.Request) {
		apiWriteJSON(w, http.StatusOK, sessionInfo{Authed: storedAPIKey() != "", Status: l.core.Status()})
	})
	mux.HandleFunc("POST /login", func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			Token string `json:"token"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		body.Token = strings.TrimSpace(body.Token)
		if body.Token == "" {
			http.Error(w, "missing token", http.StatusBadRequest)
			return
		}
		if err := l.core.Login(r.Context(), body.Token); err != nil {
			apiWriteJSON(w, http.StatusBadGateway, map[string]string{"error": err.Error()})
			return
		}
		apiWriteJSON(w, http.StatusOK, sessionInfo{Authed: true, Status: l.core.Status()})
	})
	mux.HandleFunc("POST /logout", func(w http.ResponseWriter, r *http.Request) {
		// Stop every tunnel but keep the daemon alive so the user can re-login.
		for _, t := range l.core.List() {
			_ = l.core.StopTunnel(r.Context(), t.ID)
		}
		_ = clearStoredAPIKey()
		w.WriteHeader(http.StatusNoContent)
	})
	mux.HandleFunc("GET /tunnels", func(w http.ResponseWriter, _ *http.Request) {
		apiWriteJSON(w, http.StatusOK, l.core.List())
	})
	mux.HandleFunc("POST /tunnels", func(w http.ResponseWriter, r *http.Request) {
		var spec TunnelSpec
		if err := json.NewDecoder(r.Body).Decode(&spec); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		t, err := l.core.StartTunnel(r.Context(), spec)
		if err != nil {
			apiWriteJSON(w, http.StatusBadGateway, map[string]string{"error": err.Error()})
			return
		}
		apiWriteJSON(w, http.StatusOK, t)
	})
	mux.HandleFunc("DELETE /tunnels/{id}", func(w http.ResponseWriter, r *http.Request) {
		if err := l.core.StopTunnel(r.Context(), r.PathValue("id")); err != nil {
			http.Error(w, err.Error(), http.StatusNotFound)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	})
	mux.HandleFunc("GET /requests", func(w http.ResponseWriter, _ *http.Request) {
		apiWriteJSON(w, http.StatusOK, l.core.Inspector().List())
	})
	mux.HandleFunc("DELETE /requests", func(w http.ResponseWriter, _ *http.Request) {
		l.core.Inspector().Clear()
		w.WriteHeader(http.StatusNoContent)
	})
	mux.HandleFunc("GET /events", l.events)
	return mux
}

// secure wraps the mux with CORS, a loopback Host guard (anti DNS-rebinding), and
// optional bearer-token auth. /healthz is exempt from the token so a supervisor
// can cheaply check liveness.
func (l *LocalAPI) secure(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if origin := r.Header.Get("Origin"); origin != "" {
			w.Header().Set("Access-Control-Allow-Origin", origin)
			w.Header().Add("Vary", "Origin")
			w.Header().Set("Access-Control-Allow-Methods", "GET, POST, DELETE, OPTIONS")
			w.Header().Set("Access-Control-Allow-Headers", "Authorization, Content-Type")
			w.Header().Set("Access-Control-Max-Age", "600")
		}
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		if !isLoopbackHost(r.Host) {
			http.Error(w, "forbidden: non-loopback host", http.StatusForbidden)
			return
		}
		if l.token != "" && r.URL.Path != "/healthz" {
			if !tokenMatches(r, l.token) {
				http.Error(w, "unauthorized", http.StatusUnauthorized)
				return
			}
		}
		next.ServeHTTP(w, r)
	})
}

func tokenMatches(r *http.Request, want string) bool {
	got := ""
	if h := r.Header.Get("Authorization"); strings.HasPrefix(h, "Bearer ") {
		got = strings.TrimSpace(strings.TrimPrefix(h, "Bearer "))
	}
	if got == "" {
		got = r.URL.Query().Get("token")
	}
	return subtle.ConstantTimeCompare([]byte(got), []byte(want)) == 1
}

func isLoopbackHost(host string) bool {
	if host == "" {
		return false
	}
	h := host
	if hh, _, err := net.SplitHostPort(host); err == nil {
		h = hh
	}
	if strings.EqualFold(h, "localhost") {
		return true
	}
	ip := net.ParseIP(h)
	return ip != nil && ip.IsLoopback()
}

func (l *LocalAPI) pump(ctx context.Context) {
	ev := l.core.Events()
	for {
		select {
		case <-ctx.Done():
			return
		case e := <-ev:
			l.broadcast(e)
		}
	}
}

func (l *LocalAPI) broadcast(e Event) {
	l.mu.Lock()
	defer l.mu.Unlock()
	for _, ch := range l.subs {
		select {
		case ch <- e:
		default:
		}
	}
}

func (l *LocalAPI) subscribe() (int, chan Event) {
	l.mu.Lock()
	defer l.mu.Unlock()
	id := l.next
	l.next++
	ch := make(chan Event, 64)
	l.subs[id] = ch
	return id, ch
}

func (l *LocalAPI) unsubscribe(id int) {
	l.mu.Lock()
	defer l.mu.Unlock()
	if ch, ok := l.subs[id]; ok {
		close(ch)
		delete(l.subs, id)
	}
}

func (l *LocalAPI) events(w http.ResponseWriter, r *http.Request) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming unsupported", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.WriteHeader(http.StatusOK)
	// Open the stream immediately (before any event) so the desktop app's
	// EventSource fires `onopen` right away and a silent connection isn't left
	// looking stalled while there's no traffic.
	_, _ = io.WriteString(w, ": connected\n\n")
	flusher.Flush()

	id, ch := l.subscribe()
	defer l.unsubscribe(id)

	// Comment-only keepalives keep idle SSE connections from being closed by the
	// client or any intermediary during quiet periods.
	keepalive := time.NewTicker(25 * time.Second)
	defer keepalive.Stop()

	enc := json.NewEncoder(w)
	for {
		select {
		case <-r.Context().Done():
			return
		case <-keepalive.C:
			_, _ = io.WriteString(w, ": keepalive\n\n")
			flusher.Flush()
		case e := <-ch:
			_, _ = w.Write([]byte("data: "))
			_ = enc.Encode(e) // writes the JSON + newline
			_, _ = w.Write([]byte("\n"))
			flusher.Flush()
		}
	}
}

func apiWriteJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

// --- control-API token (loopback shared secret with the desktop app) ---

// ControlTokenPath is the file holding the loopback control-API token
// (~/.trqsh/control.token). The desktop app reads it to authenticate.
func ControlTokenPath() string {
	return filepath.Join(filepath.Dir(DefaultConfigPath()), "control.token")
}

// LoadControlToken returns the stored control token, or "" if none exists.
func LoadControlToken() string {
	data, err := os.ReadFile(ControlTokenPath())
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(data))
}

// LoadOrCreateControlToken returns the stored control token, generating and
// persisting a new random one (0600) if absent.
func LoadOrCreateControlToken() (string, error) {
	if tok := LoadControlToken(); tok != "" {
		return tok, nil
	}
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	tok := hex.EncodeToString(buf)
	path := ControlTokenPath()
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return "", err
	}
	if err := os.WriteFile(path, []byte(tok), 0o600); err != nil {
		return "", err
	}
	return tok, nil
}

func storedAPIKey() string {
	cfg, err := Load(DefaultConfigPath())
	if err != nil {
		return ""
	}
	return cfg.APIKey
}

func clearStoredAPIKey() error {
	cfg, err := Load(DefaultConfigPath())
	if err != nil {
		return err
	}
	cfg.APIKey = ""
	return cfg.Save(DefaultConfigPath())
}
