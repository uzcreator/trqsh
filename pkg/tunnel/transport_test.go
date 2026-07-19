package tunnel

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"fmt"
	"io"
	"math/big"
	"net"
	"sync"
	"testing"
	"time"

	"github.com/rift/rift/pkg/proto"
)

// testTLS returns a server config with a fresh self-signed cert and a matching
// insecure client config, both advertising the rift ALPN token.
func testTLS(t *testing.T) (server, client *tls.Config) {
	t.Helper()
	priv, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("genkey: %v", err)
	}
	tmpl := x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject:      pkix.Name{CommonName: "rift-test"},
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     time.Now().Add(time.Hour),
		KeyUsage:     x509.KeyUsageDigitalSignature | x509.KeyUsageCertSign,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		DNSNames:     []string{"localhost"},
		IPAddresses:  []net.IP{net.ParseIP("127.0.0.1"), net.ParseIP("::1")},
	}
	der, err := x509.CreateCertificate(rand.Reader, &tmpl, &tmpl, &priv.PublicKey, priv)
	if err != nil {
		t.Fatalf("create cert: %v", err)
	}
	cert := tls.Certificate{Certificate: [][]byte{der}, PrivateKey: priv}
	server = &tls.Config{Certificates: []tls.Certificate{cert}, NextProtos: []string{ALPNProto}}
	client = &tls.Config{InsecureSkipVerify: true, NextProtos: []string{ALPNProto}}
	return server, client
}

// echoServer accepts sessions on l and echoes every stream back to the sender.
func echoServer(t *testing.T, l Listener) {
	t.Helper()
	go func() {
		for {
			sess, err := l.Accept(context.Background())
			if err != nil {
				return
			}
			go func(s Session) {
				for {
					st, err := s.AcceptStream(context.Background())
					if err != nil {
						return
					}
					go func(st Stream) {
						_, _ = io.Copy(st, st)
						st.Close()
					}(st)
				}
			}(sess)
		}
	}()
}

// exerciseStreams opens n concurrent streams, sends a unique payload on each,
// and asserts the echo matches.
func exerciseStreams(t *testing.T, sess Session, n int) {
	t.Helper()
	var wg sync.WaitGroup
	errCh := make(chan error, n)
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			st, err := sess.OpenStream(ctx)
			if err != nil {
				errCh <- fmt.Errorf("open %d: %w", i, err)
				return
			}
			defer st.Close()
			msg := []byte(fmt.Sprintf("stream-%d-payload-%d", i, i*7+1))
			if _, err := st.Write(msg); err != nil {
				errCh <- fmt.Errorf("write %d: %w", i, err)
				return
			}
			buf := make([]byte, len(msg))
			if _, err := io.ReadFull(st, buf); err != nil {
				errCh <- fmt.Errorf("read %d: %w", i, err)
				return
			}
			if string(buf) != string(msg) {
				errCh <- fmt.Errorf("stream %d echo mismatch: got %q want %q", i, buf, msg)
			}
		}(i)
	}
	wg.Wait()
	close(errCh)
	for err := range errCh {
		t.Error(err)
	}
}

func TestQUICEchoConcurrentStreams(t *testing.T) {
	srvTLS, cliTLS := testTLS(t)
	l, err := Listen("127.0.0.1:0", "", srvTLS)
	if err != nil {
		t.Fatalf("Listen: %v", err)
	}
	defer l.Close()
	echoServer(t, l)

	addr := l.(*multiListener).quicLn.Addr().String()
	d := &Dialer{TLSConfig: cliTLS, ForceKind: KindQUIC, DialTimeout: 5 * time.Second}
	sess, err := d.Dial(context.Background(), addr)
	if err != nil {
		t.Fatalf("Dial: %v", err)
	}
	defer sess.CloseWithError(0, "done")

	if sess.Kind() != KindQUIC {
		t.Fatalf("expected KindQUIC, got %s", sess.Kind())
	}
	exerciseStreams(t, sess, 20)
}

func TestTCPEchoConcurrentStreams(t *testing.T) {
	srvTLS, cliTLS := testTLS(t)
	l, err := Listen("", "127.0.0.1:0", srvTLS)
	if err != nil {
		t.Fatalf("Listen: %v", err)
	}
	defer l.Close()
	echoServer(t, l)

	addr := l.(*multiListener).tcpLn.Addr().String()
	d := &Dialer{TLSConfig: cliTLS, ForceKind: KindTCP, DialTimeout: 5 * time.Second}
	sess, err := d.Dial(context.Background(), addr)
	if err != nil {
		t.Fatalf("Dial: %v", err)
	}
	defer sess.CloseWithError(0, "done")

	if sess.Kind() != KindTCP {
		t.Fatalf("expected KindTCP, got %s", sess.Kind())
	}
	exerciseStreams(t, sess, 20)
}

// TestQUICToTCPFallback dials an edge that only speaks TCP with a QUIC-first
// dialer; the QUIC attempt must time out and fall back to TCP+yamux on the same
// address, still yielding a working session.
func TestQUICToTCPFallback(t *testing.T) {
	srvTLS, cliTLS := testTLS(t)

	// Reserve a port, then listen for TCP only on it (no QUIC/UDP listener).
	port := freePort(t)
	addr := fmt.Sprintf("127.0.0.1:%d", port)
	l, err := Listen("", addr, srvTLS)
	if err != nil {
		t.Fatalf("Listen: %v", err)
	}
	defer l.Close()
	echoServer(t, l)

	d := &Dialer{TLSConfig: cliTLS, DialTimeout: 800 * time.Millisecond} // QUIC-first, fallback allowed
	sess, err := d.Dial(context.Background(), addr)
	if err != nil {
		t.Fatalf("Dial (expected fallback): %v", err)
	}
	defer sess.CloseWithError(0, "done")

	if sess.Kind() != KindTCP {
		t.Fatalf("expected fallback to KindTCP, got %s", sess.Kind())
	}
	exerciseStreams(t, sess, 5)
}

// TestPingPongOverControlStream exercises the proto codec over a real stream:
// the client sends a Ping envelope and the server replies with a Pong.
func TestPingPongOverControlStream(t *testing.T) {
	srvTLS, cliTLS := testTLS(t)
	l, err := Listen("127.0.0.1:0", "", srvTLS)
	if err != nil {
		t.Fatalf("Listen: %v", err)
	}
	defer l.Close()

	// Server: read a Ping, echo a Pong with the same timestamp.
	go func() {
		sess, err := l.Accept(context.Background())
		if err != nil {
			return
		}
		st, err := sess.AcceptStream(context.Background())
		if err != nil {
			return
		}
		env, err := proto.ReadMsg(st)
		if err != nil {
			return
		}
		ping := env.GetPing()
		if ping == nil {
			return
		}
		_ = proto.WriteMsg(st, &proto.Envelope{Msg: &proto.Envelope_Pong{Pong: &proto.Pong{TsUnixMs: ping.TsUnixMs}}})
	}()

	addr := l.(*multiListener).quicLn.Addr().String()
	d := &Dialer{TLSConfig: cliTLS, ForceKind: KindQUIC, DialTimeout: 5 * time.Second}
	sess, err := d.Dial(context.Background(), addr)
	if err != nil {
		t.Fatalf("Dial: %v", err)
	}
	defer sess.CloseWithError(0, "done")

	st, err := sess.OpenStream(context.Background())
	if err != nil {
		t.Fatalf("OpenStream: %v", err)
	}
	ts := time.Now().UnixMilli()
	if err := proto.WriteMsg(st, &proto.Envelope{Msg: &proto.Envelope_Ping{Ping: &proto.Ping{TsUnixMs: ts}}}); err != nil {
		t.Fatalf("WriteMsg ping: %v", err)
	}
	reply, err := proto.ReadMsg(st)
	if err != nil {
		t.Fatalf("ReadMsg pong: %v", err)
	}
	pong := reply.GetPong()
	if pong == nil {
		t.Fatalf("expected Pong, got %v", reply)
	}
	if pong.TsUnixMs != ts {
		t.Fatalf("pong ts mismatch: got %d want %d", pong.TsUnixMs, ts)
	}
}

// freePort returns a TCP port that was free at call time.
func freePort(t *testing.T) int {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("reserve port: %v", err)
	}
	defer ln.Close()
	return ln.Addr().(*net.TCPAddr).Port
}
