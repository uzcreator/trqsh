package server

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"fmt"
	"math/big"
	"net"
	"sync"
	"time"
)

// CertManager supplies TLS certificates for public HTTPS ingress. The dev
// implementation mints self-signed certs on demand; production swaps in a
// CertMagic-backed manager doing DNS-01 wildcard + on-demand custom domains.
type CertManager interface {
	GetCertificate(hello *tls.ClientHelloInfo) (*tls.Certificate, error)
	// TLSConfig returns a *tls.Config wired to GetCertificate with public ALPN.
	TLSConfig() *tls.Config
}

// devCertManager issues self-signed certificates, one per requested SNI, signed
// by a stable in-memory key. Suitable for local `curl -k` testing only.
//
// TODO(prod): replace with a CertMagic manager:
//   - DNS-01 wildcard for *.<base_domain> (lego provider from RIFT_DNS_* env),
//   - on-demand TLS for custom domains gated by a control-plane/Redis allowlist,
//   - shared cert cache (Redis) so all edges reuse issued certs,
//   - Let's Encrypt staging when RIFT_ACME_STAGING=1.
type devCertManager struct {
	baseDomain string
	key        *ecdsa.PrivateKey
	mu         sync.Mutex
	cache      map[string]*tls.Certificate
}

// NewDevCertManager builds a self-signed cert manager for the base domain.
func NewDevCertManager(baseDomain string) (CertManager, error) {
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return nil, fmt.Errorf("tls: gen key: %w", err)
	}
	return &devCertManager{
		baseDomain: baseDomain,
		key:        key,
		cache:      make(map[string]*tls.Certificate),
	}, nil
}

func (d *devCertManager) GetCertificate(hello *tls.ClientHelloInfo) (*tls.Certificate, error) {
	name := hello.ServerName
	if name == "" {
		name = "*." + d.baseDomain
	}
	return d.certFor(name)
}

func (d *devCertManager) certFor(name string) (*tls.Certificate, error) {
	d.mu.Lock()
	defer d.mu.Unlock()
	if c, ok := d.cache[name]; ok {
		return c, nil
	}
	tmpl := x509.Certificate{
		SerialNumber: big.NewInt(time.Now().UnixNano()),
		Subject:      pkix.Name{CommonName: name},
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     time.Now().Add(90 * 24 * time.Hour),
		KeyUsage:     x509.KeyUsageDigitalSignature,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
	}
	if ip := net.ParseIP(name); ip != nil {
		tmpl.IPAddresses = []net.IP{ip}
	} else {
		tmpl.DNSNames = []string{name}
	}
	der, err := x509.CreateCertificate(rand.Reader, &tmpl, &tmpl, &d.key.PublicKey, d.key)
	if err != nil {
		return nil, fmt.Errorf("tls: create cert for %q: %w", name, err)
	}
	cert := &tls.Certificate{Certificate: [][]byte{der}, PrivateKey: d.key}
	d.cache[name] = cert
	return cert, nil
}

func (d *devCertManager) TLSConfig() *tls.Config {
	return &tls.Config{
		GetCertificate: d.GetCertificate,
		NextProtos:     []string{"h2", "http/1.1"},
		MinVersion:     tls.VersionTLS12,
	}
}

// tlsListener wraps a raw listener with the cert manager's TLS config.
func tlsListener(inner net.Listener, cm CertManager) net.Listener {
	return tls.NewListener(inner, cm.TLSConfig())
}
