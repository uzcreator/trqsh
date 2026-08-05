package tunnel

import (
	"context"
	"net"
	"time"

	quic "github.com/quic-go/quic-go"
)

// quicConfig builds the QUIC configuration shared by both ends of a session.
// Flow-control windows and the concurrent-stream limit are raised well above
// quic-go's conservative defaults so one agent session can carry a busy site's
// worth of simultaneous request streams at full link speed. keepAlive<=0 uses
// the package default.
func quicConfig(keepAlive time.Duration) *quic.Config {
	if keepAlive <= 0 {
		keepAlive = defaultKeepAlive
	}
	return &quic.Config{
		KeepAlivePeriod:                keepAlive,
		MaxIdleTimeout:                 defaultMaxIdle,
		EnableDatagrams:                true,
		InitialStreamReceiveWindow:     quicInitialStreamWindow,
		MaxStreamReceiveWindow:         quicMaxStreamWindow,
		InitialConnectionReceiveWindow: quicInitialConnWindow,
		MaxConnectionReceiveWindow:     quicMaxConnWindow,
		MaxIncomingStreams:             quicMaxIncomingStreams,
	}
}

// quicSession adapts a *quic.Conn to the Session interface.
type quicSession struct {
	conn *quic.Conn
}

func newQUICSession(conn *quic.Conn) *quicSession { return &quicSession{conn: conn} }

func (s *quicSession) OpenStream(ctx context.Context) (Stream, error) {
	st, err := s.conn.OpenStreamSync(ctx)
	if err != nil {
		return nil, err
	}
	return &quicStream{Stream: st, conn: s.conn}, nil
}

func (s *quicSession) AcceptStream(ctx context.Context) (Stream, error) {
	st, err := s.conn.AcceptStream(ctx)
	if err != nil {
		return nil, err
	}
	return &quicStream{Stream: st, conn: s.conn}, nil
}

func (s *quicSession) Kind() Kind               { return KindQUIC }
func (s *quicSession) RemoteAddr() net.Addr     { return s.conn.RemoteAddr() }
func (s *quicSession) Context() context.Context { return s.conn.Context() }

func (s *quicSession) CloseWithError(code uint32, msg string) error {
	return s.conn.CloseWithError(quic.ApplicationErrorCode(code), msg)
}

// quicStream adapts a *quic.Stream to the Stream interface. quic streams do not
// expose LocalAddr/RemoteAddr, so those are delegated to the owning connection.
type quicStream struct {
	*quic.Stream
	conn *quic.Conn
}

func (s *quicStream) ID() uint64           { return uint64(s.StreamID()) } // #nosec G115 -- QUIC stream IDs are protocol-guaranteed non-negative (RFC 9000)
func (s *quicStream) LocalAddr() net.Addr  { return s.conn.LocalAddr() }
func (s *quicStream) RemoteAddr() net.Addr { return s.conn.RemoteAddr() }
