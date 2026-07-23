package tunnel

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"net"
	"time"

	"github.com/hashicorp/yamux"
	quic "github.com/quic-go/quic-go"
)

// Dialer establishes a multiplexed Session to an edge. By default it tries QUIC
// first and transparently falls back to TCP+yamux when QUIC/UDP is blocked.
type Dialer struct {
	// TLSConfig is used for both QUIC and TLS-over-TCP. ALPN is forced to the
	// trqsh protocol token if the caller left NextProtos empty.
	TLSConfig *tls.Config
	// Prefer selects the primary transport. Zero value means KindQUIC.
	Prefer Kind
	// ForceKind pins a single transport and disables fallback when set.
	ForceKind Kind
	// DialTimeout bounds each individual transport attempt. Zero uses a default.
	DialTimeout time.Duration
	// KeepAlive sets the QUIC keepalive period and yamux keepalive interval.
	KeepAlive time.Duration
}

// Dial connects to addr, returning a live Session. Unless ForceKind pins a
// transport, a failed QUIC attempt falls back to TCP+yamux (and vice-versa).
func (d *Dialer) Dial(ctx context.Context, addr string) (Session, error) {
	order := d.attemptOrder()
	var errs []error
	for _, k := range order {
		var (
			s   Session
			err error
		)
		switch k {
		case KindQUIC:
			s, err = d.dialQUIC(ctx, addr)
		case KindTCP:
			s, err = d.dialTCP(ctx, addr)
		}
		if err == nil {
			return s, nil
		}
		errs = append(errs, fmt.Errorf("%s: %w", k, err))
		// Respect caller cancellation instead of grinding through fallbacks.
		if ctx.Err() != nil {
			break
		}
	}
	return nil, fmt.Errorf("tunnel: dial %s failed: %w", addr, errors.Join(errs...))
}

// attemptOrder returns the transports to try, in order. ForceKind yields a
// single entry; otherwise the preferred transport is tried first and the other
// second as fallback.
func (d *Dialer) attemptOrder() []Kind {
	if d.ForceKind == KindQUIC || d.ForceKind == KindTCP {
		return []Kind{d.ForceKind}
	}
	primary := d.Prefer
	if primary == "" {
		primary = KindQUIC
	}
	if primary == KindTCP {
		return []Kind{KindTCP, KindQUIC}
	}
	return []Kind{KindQUIC, KindTCP}
}

func (d *Dialer) dialTimeout() time.Duration {
	if d.DialTimeout > 0 {
		return d.DialTimeout
	}
	return defaultDialTimeout
}

func (d *Dialer) keepAlive() time.Duration {
	if d.KeepAlive > 0 {
		return d.KeepAlive
	}
	return defaultKeepAlive
}

// tlsClone returns a copy of TLSConfig with the trqsh ALPN token ensured.
func (d *Dialer) tlsClone() *tls.Config {
	var c *tls.Config
	if d.TLSConfig != nil {
		c = d.TLSConfig.Clone()
	} else {
		c = &tls.Config{}
	}
	if len(c.NextProtos) == 0 {
		c.NextProtos = []string{ALPNProto}
	}
	return c
}

func (d *Dialer) dialQUIC(ctx context.Context, addr string) (Session, error) {
	dctx, cancel := context.WithTimeout(ctx, d.dialTimeout())
	defer cancel()
	conf := &quic.Config{
		KeepAlivePeriod: d.keepAlive(),
		MaxIdleTimeout:  defaultMaxIdle,
		EnableDatagrams: true,
	}
	conn, err := quic.DialAddr(dctx, addr, d.tlsClone(), conf)
	if err != nil {
		return nil, err
	}
	return newQUICSession(conn), nil
}

func (d *Dialer) dialTCP(ctx context.Context, addr string) (Session, error) {
	dctx, cancel := context.WithTimeout(ctx, d.dialTimeout())
	defer cancel()
	nd := &net.Dialer{}
	raw, err := nd.DialContext(dctx, "tcp", addr)
	if err != nil {
		return nil, err
	}
	tlsConn := tls.Client(raw, d.tlsClone())
	if err := tlsConn.HandshakeContext(dctx); err != nil {
		raw.Close()
		return nil, err
	}
	sess, err := yamux.Client(tlsConn, yamuxConfig(d.keepAlive()))
	if err != nil {
		tlsConn.Close()
		return nil, err
	}
	return newYamuxSession(sess), nil
}
