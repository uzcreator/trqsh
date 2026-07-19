// Package inspect implements the agent's local request inspector: an in-memory
// ring buffer of captured HTTP request/response pairs, plus a small web UI and
// replay served at 127.0.0.1:4040.
package inspect

import (
	"fmt"
	"sync"
	"time"
)

// CapturedRequest is one inspected HTTP exchange.
type CapturedRequest struct {
	ID          string            `json:"id"`
	TunnelID    string            `json:"tunnel_id"`
	Proto       string            `json:"proto"`
	Method      string            `json:"method"`
	Host        string            `json:"host"`
	Path        string            `json:"path"`
	Status      int               `json:"status"`
	StartedAt   time.Time         `json:"started_at"`
	DurationMs  int64             `json:"duration_ms"`
	ReqHeaders  map[string]string `json:"req_headers,omitempty"`
	RespHeaders map[string]string `json:"resp_headers,omitempty"`
	ReqBody     []byte            `json:"req_body,omitempty"`
	RespBody    []byte            `json:"resp_body,omitempty"`
	BytesIn     int64             `json:"bytes_in"`
	BytesOut    int64             `json:"bytes_out"`
	LocalAddr   string            `json:"local_addr"`
}

// MaxBodyCapture bounds how many body bytes are retained per direction.
const MaxBodyCapture = 64 * 1024

// Recorder is a concurrency-safe ring buffer of captured requests with live
// subscribers for streaming to the UI/GUI.
type Recorder struct {
	mu   sync.Mutex
	capN int
	buf  []CapturedRequest
	seq  uint64

	subMu   sync.Mutex
	subs    map[int]chan CapturedRequest
	nextSub int
}

// NewRecorder creates a recorder holding up to capacity captures.
func NewRecorder(capacity int) *Recorder {
	if capacity <= 0 {
		capacity = 128
	}
	return &Recorder{
		capN: capacity,
		buf:  make([]CapturedRequest, 0, capacity),
		subs: make(map[int]chan CapturedRequest),
	}
}

// Add stores a capture (assigning an ID if unset) and notifies subscribers.
func (r *Recorder) Add(c CapturedRequest) CapturedRequest {
	r.mu.Lock()
	r.seq++
	if c.ID == "" {
		c.ID = fmt.Sprintf("r%d", r.seq)
	}
	if len(r.buf) >= r.capN {
		copy(r.buf, r.buf[1:])
		r.buf = r.buf[:len(r.buf)-1]
	}
	r.buf = append(r.buf, c)
	r.mu.Unlock()

	r.subMu.Lock()
	for _, ch := range r.subs {
		select {
		case ch <- c:
		default:
		}
	}
	r.subMu.Unlock()
	return c
}

// List returns captures, newest first.
func (r *Recorder) List() []CapturedRequest {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make([]CapturedRequest, len(r.buf))
	for i, c := range r.buf {
		out[len(r.buf)-1-i] = c
	}
	return out
}

// Get returns a capture by ID.
func (r *Recorder) Get(id string) (CapturedRequest, bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, c := range r.buf {
		if c.ID == id {
			return c, true
		}
	}
	return CapturedRequest{}, false
}

// Subscribe registers a live channel; call Unsubscribe with the returned id.
func (r *Recorder) Subscribe() (int, <-chan CapturedRequest) {
	r.subMu.Lock()
	defer r.subMu.Unlock()
	id := r.nextSub
	r.nextSub++
	ch := make(chan CapturedRequest, 32)
	r.subs[id] = ch
	return id, ch
}

// Unsubscribe removes a live channel.
func (r *Recorder) Unsubscribe(id int) {
	r.subMu.Lock()
	defer r.subMu.Unlock()
	if ch, ok := r.subs[id]; ok {
		close(ch)
		delete(r.subs, id)
	}
}

func capBody(b []byte) []byte {
	if len(b) > MaxBodyCapture {
		return b[:MaxBodyCapture]
	}
	return b
}
