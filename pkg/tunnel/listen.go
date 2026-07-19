package tunnel

import (
	"context"
	"crypto/tls"
	"errors"
	"net"
	"sync"

	"github.com/hashicorp/yamux"
	quic "github.com/quic-go/quic-go"
)

// Listen starts an edge-side Listener that accepts agent sessions over both
// QUIC (on quicAddr, UDP) and TLS-over-TCP+yamux (on tcpAddr). Either address
// may be empty to disable that transport, but at least one is required. The
// same tlsConf backs both; its ALPN is forced to the rift protocol token when
// the caller leaves NextProtos empty.
func Listen(quicAddr, tcpAddr string, tlsConf *tls.Config) (Listener, error) {
	if quicAddr == "" && tcpAddr == "" {
		return nil, errors.New("tunnel: Listen requires at least one of quicAddr/tcpAddr")
	}
	conf := tlsConf.Clone()
	if len(conf.NextProtos) == 0 {
		conf.NextProtos = []string{ALPNProto}
	}

	ctx, cancel := context.WithCancel(context.Background())
	m := &multiListener{
		conf:    conf,
		accepts: make(chan acceptResult),
		ctx:     ctx,
		cancel:  cancel,
	}

	if quicAddr != "" {
		ql, err := quic.ListenAddr(quicAddr, conf, &quic.Config{
			KeepAlivePeriod: defaultKeepAlive,
			MaxIdleTimeout:  defaultMaxIdle,
			EnableDatagrams: true,
		})
		if err != nil {
			cancel()
			return nil, err
		}
		m.quicLn = ql
		go m.acceptQUIC(ql)
	}

	if tcpAddr != "" {
		tl, err := net.Listen("tcp", tcpAddr)
		if err != nil {
			cancel()
			if m.quicLn != nil {
				m.quicLn.Close()
			}
			return nil, err
		}
		m.tcpLn = tl
		go m.acceptTCP(tl)
	}

	return m, nil
}

type acceptResult struct {
	sess Session
	err  error
}

type multiListener struct {
	conf   *tls.Config
	quicLn *quic.Listener
	tcpLn  net.Listener

	accepts chan acceptResult
	ctx     context.Context
	cancel  context.CancelFunc
	once    sync.Once
}

func (m *multiListener) acceptQUIC(ln *quic.Listener) {
	for {
		conn, err := ln.Accept(m.ctx)
		if err != nil {
			if m.ctx.Err() != nil {
				return
			}
			m.push(acceptResult{err: err})
			continue
		}
		m.push(acceptResult{sess: newQUICSession(conn)})
	}
}

func (m *multiListener) acceptTCP(ln net.Listener) {
	for {
		raw, err := ln.Accept()
		if err != nil {
			if m.ctx.Err() != nil {
				return
			}
			m.push(acceptResult{err: err})
			continue
		}
		// Handshake + mux setup off the accept loop so a slow client cannot
		// stall other incoming connections.
		go m.serveTCP(raw)
	}
}

func (m *multiListener) serveTCP(raw net.Conn) {
	tlsConn := tls.Server(raw, m.conf)
	if err := tlsConn.HandshakeContext(m.ctx); err != nil {
		tlsConn.Close()
		return
	}
	sess, err := yamux.Server(tlsConn, yamuxConfig(defaultKeepAlive))
	if err != nil {
		tlsConn.Close()
		return
	}
	m.push(acceptResult{sess: newYamuxSession(sess)})
}

// push delivers a result to Accept, or drops it if the listener is closing.
func (m *multiListener) push(r acceptResult) {
	select {
	case m.accepts <- r:
	case <-m.ctx.Done():
		if r.sess != nil {
			r.sess.CloseWithError(0, "listener closed")
		}
	}
}

func (m *multiListener) Accept(ctx context.Context) (Session, error) {
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-m.ctx.Done():
		return nil, net.ErrClosed
	case r := <-m.accepts:
		return r.sess, r.err
	}
}

func (m *multiListener) Addr() net.Addr {
	if m.quicLn != nil {
		return m.quicLn.Addr()
	}
	return m.tcpLn.Addr()
}

func (m *multiListener) Close() error {
	var err error
	m.once.Do(func() {
		m.cancel()
		if m.quicLn != nil {
			if e := m.quicLn.Close(); e != nil {
				err = e
			}
		}
		if m.tcpLn != nil {
			if e := m.tcpLn.Close(); e != nil {
				err = e
			}
		}
	})
	return err
}
