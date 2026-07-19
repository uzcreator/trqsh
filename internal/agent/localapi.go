package agent

import (
	"context"
	"encoding/json"
	"net/http"
	"sync"
	"time"
)

// LocalAPI serves the Core over loopback HTTP (JSON + SSE) so the GUI (Part 04)
// can drive the same agent core out-of-process. It re-broadcasts the core's
// single Events() stream to any number of connected clients.
type LocalAPI struct {
	core *Agent

	mu   sync.Mutex
	subs map[int]chan Event
	next int
}

// NewLocalAPI builds a control API over the agent core.
func NewLocalAPI(core *Agent) *LocalAPI {
	return &LocalAPI{core: core, subs: make(map[int]chan Event)}
}

// Serve runs the control API until ctx is canceled.
func (l *LocalAPI) Serve(ctx context.Context, addr string) error {
	go l.pump(ctx)
	srv := &http.Server{
		Addr:              addr,
		Handler:           l.Handler(),
		ReadHeaderTimeout: 5 * time.Second, // slowloris guard (gosec G112)
		IdleTimeout:       120 * time.Second,
		// No WriteTimeout: /api/stream serves long-lived SSE to the GUI.
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

// Handler returns the control API routes.
func (l *LocalAPI) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("ok"))
	})
	mux.HandleFunc("GET /status", func(w http.ResponseWriter, _ *http.Request) {
		apiWriteJSON(w, http.StatusOK, l.core.Status())
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
	mux.HandleFunc("GET /events", l.events)
	return mux
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

	id, ch := l.subscribe()
	defer l.unsubscribe(id)
	enc := json.NewEncoder(w)
	for {
		select {
		case <-r.Context().Done():
			return
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
