package ui

import (
	"fmt"
	"sync"
	"time"
)

// Spinner is a lightweight progress indicator for network operations (checking
// for updates, downloading, waiting on browser sign-in). When color is off or
// output isn't a terminal it degrades to a single printed line, so scripts and
// CI logs never accumulate carriage-return animation frames.
type Spinner struct {
	mu     sync.Mutex
	msg    string
	active bool
	stop   chan struct{}
	done   chan struct{}
}

// Braille frames give a smooth, compact spin that renders in any UTF-8 terminal.
var spinFrames = []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}

// NewSpinner returns a spinner labeled msg. It does not start until Start.
func NewSpinner(msg string) *Spinner { return &Spinner{msg: msg} }

// Start begins animating on stderr. With color disabled it prints the label
// once and animates nothing, keeping non-interactive output tidy.
func (s *Spinner) Start() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.active {
		return
	}
	if !enabled {
		_, _ = fmt.Fprintf(Stderr, "  %s…\n", s.msg)
		return
	}
	s.active = true
	s.stop = make(chan struct{})
	s.done = make(chan struct{})
	go s.run()
}

func (s *Spinner) run() {
	defer close(s.done)
	t := time.NewTicker(90 * time.Millisecond)
	defer t.Stop()
	i := 0
	for {
		select {
		case <-s.stop:
			return
		case <-t.C:
			s.mu.Lock()
			msg := s.msg
			s.mu.Unlock()
			// \r returns to column 0; \x1b[2K clears the whole line so a shorter
			// message never leaves stale characters from a longer previous one.
			_, _ = fmt.Fprintf(Stderr, "\r\x1b[2K  %s %s", Accent(spinFrames[i%len(spinFrames)]), msg)
			i++
		}
	}
}

// Update changes the label shown next to the spinner while it runs.
func (s *Spinner) Update(msg string) {
	s.mu.Lock()
	s.msg = msg
	s.mu.Unlock()
	if !s.isActive() {
		// Not animating (color off): surface the new phase as its own line.
		_, _ = fmt.Fprintf(Stderr, "  %s…\n", msg)
	}
}

func (s *Spinner) isActive() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.active
}

// halt stops the animation goroutine (if any) and clears the spinner line so a
// final status can be printed cleanly in its place.
func (s *Spinner) halt() {
	s.mu.Lock()
	active := s.active
	s.active = false
	s.mu.Unlock()
	if !active {
		return
	}
	close(s.stop)
	<-s.done
	_, _ = fmt.Fprint(Stderr, "\r\x1b[2K")
}

// Success stops the spinner and prints a green ✓ line in its place.
func (s *Spinner) Success(format string, a ...any) {
	s.halt()
	Success(format, a...)
}

// Fail stops the spinner and prints a red ✗ line in its place.
func (s *Spinner) Fail(format string, a ...any) {
	s.halt()
	Fail(format, a...)
}

// Stop stops the spinner and clears its line without printing anything.
func (s *Spinner) Stop() { s.halt() }
