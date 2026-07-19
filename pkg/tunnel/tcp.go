package tunnel

import (
	"context"
	"io"
	"net"
	"time"

	"github.com/hashicorp/yamux"
)

// yamuxConfig builds a yamux config with keepalive tuned to the given interval
// and logging silenced (the transport layer surfaces errors via returned errs).
func yamuxConfig(keepAlive time.Duration) *yamux.Config {
	c := yamux.DefaultConfig()
	c.LogOutput = io.Discard
	if keepAlive > 0 {
		c.EnableKeepAlive = true
		c.KeepAliveInterval = keepAlive
	}
	return c
}

// yamuxSession adapts a *yamux.Session (over a TLS-TCP conn) to the Session
// interface. yamux has no application error codes, so CloseWithError degrades
// to a plain close; the reason is best-effort only.
type yamuxSession struct {
	sess   *yamux.Session
	ctx    context.Context
	cancel context.CancelFunc
}

func newYamuxSession(sess *yamux.Session) *yamuxSession {
	ctx, cancel := context.WithCancel(context.Background())
	s := &yamuxSession{sess: sess, ctx: ctx, cancel: cancel}
	// Cancel the session context once the underlying mux closes.
	go func() {
		<-sess.CloseChan()
		cancel()
	}()
	return s
}

func (s *yamuxSession) OpenStream(ctx context.Context) (Stream, error) {
	st, err := s.sess.OpenStream()
	if err != nil {
		return nil, err
	}
	return &yamuxStream{Stream: st}, nil
}

func (s *yamuxSession) AcceptStream(ctx context.Context) (Stream, error) {
	st, err := s.sess.AcceptStreamWithContext(ctx)
	if err != nil {
		return nil, err
	}
	return &yamuxStream{Stream: st}, nil
}

func (s *yamuxSession) Kind() Kind               { return KindTCP }
func (s *yamuxSession) RemoteAddr() net.Addr     { return s.sess.RemoteAddr() }
func (s *yamuxSession) Context() context.Context { return s.ctx }

func (s *yamuxSession) CloseWithError(code uint32, msg string) error {
	s.cancel()
	return s.sess.Close()
}

// yamuxStream adapts a *yamux.Stream to the Stream interface. yamux.Stream
// already satisfies net.Conn; we only add ID().
type yamuxStream struct {
	*yamux.Stream
}

func (s *yamuxStream) ID() uint64 { return uint64(s.StreamID()) }
