package server

import (
	"context"
	"crypto/tls"
	"testing"
)

// The on-demand DecisionFunc is a security boundary: it must let the edge issue
// certs for its own zone but refuse arbitrary hostnames, or anyone could make us
// hammer Let's Encrypt (rate-limit exhaustion / abuse).
func TestTLSDecision(t *testing.T) {
	s := &Server{cfg: Config{BaseDomain: "trqsh.uz"}, hub: NewHub(), reg: NewInMemoryRegistry()}
	ctx := context.Background()

	allowed := []string{"trqsh.uz", "demo.trqsh.uz", "Abc.TRQSH.uz", "reserved.trqsh.uz."}
	for _, n := range allowed {
		if err := s.tlsDecision(ctx, n); err != nil {
			t.Errorf("tlsDecision(%q) = %v, want allow", n, err)
		}
	}

	denied := []string{"evil.example.com", "trqsh.uz.attacker.net", "random.io"}
	for _, n := range denied {
		if err := s.tlsDecision(ctx, n); err == nil {
			t.Errorf("tlsDecision(%q) = allow, want deny (unbound external domain)", n)
		}
	}
}

// The ACME manager must build offline and expose an ingress TLS config that can
// actually serve browsers (HTTP/2 + HTTP/1.1) and answer TLS-ALPN challenges.
func TestACMECertManagerBuilds(t *testing.T) {
	cfg := Config{BaseDomain: "trqsh.uz", ACMEEmail: "ops@trqsh.uz", ACMEStaging: true}
	cm, err := newACMECertManager(cfg, nil, func(context.Context, string) error { return nil })
	if err != nil {
		t.Fatalf("newACMECertManager: %v", err)
	}
	tc := cm.TLSConfig()
	if tc.GetCertificate == nil {
		t.Fatal("ingress TLSConfig must set GetCertificate")
	}
	if tc.MinVersion < tls.VersionTLS12 {
		t.Errorf("MinVersion = %x, want >= TLS 1.2", tc.MinVersion)
	}
	want := map[string]bool{"http/1.1": false, "acme-tls/1": false}
	for _, p := range tc.NextProtos {
		if p == "h2" {
			t.Errorf("ingress must not advertise h2 — the edge speaks HTTP/1.1 only")
		}
		if _, ok := want[p]; ok {
			want[p] = true
		}
	}
	for proto, seen := range want {
		if !seen {
			t.Errorf("ingress ALPN missing %q (got %v)", proto, tc.NextProtos)
		}
	}
}
