package tunnel

import (
	"context"
	"net"

	quic "github.com/quic-go/quic-go"
)

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

func (s *quicStream) ID() uint64           { return uint64(s.StreamID()) }
func (s *quicStream) LocalAddr() net.Addr  { return s.conn.LocalAddr() }
func (s *quicStream) RemoteAddr() net.Addr { return s.conn.RemoteAddr() }
