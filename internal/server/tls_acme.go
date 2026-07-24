package server

import (
	"context"
	"crypto/tls"

	"github.com/caddyserver/certmagic"
	"github.com/libdns/cloudflare"
)

// prodCertManager is the production CertManager: real ACME certificates from
// Let's Encrypt, backed by CertMagic. Two issuance paths:
//
//   - Wildcard for the base zone (`*.<base>` + `<base>`) via DNS-01, so every
//     random/reserved subdomain is served by one cert with no per-name ACME
//     order (needs a DNS provider token, e.g. Cloudflare).
//   - On-demand for custom domains (bring-your-own), gated by a DecisionFunc so
//     the edge only asks Let's Encrypt for hostnames an agent has actually bound
//     — protecting against abuse and LE rate limits.
//
// Renewal, OCSP stapling, storage and locking are handled by CertMagic. Certs
// persist to disk (TRQSH_TLS_STORAGE_DIR) so restarts don't re-issue.
type prodCertManager struct {
	magic          *certmagic.Config
	base           string
	manageWildcard bool
}

// certWarmer is implemented by cert managers that can pre-provision certs at
// startup (the ACME manager obtains the wildcard). Detected by Server.Run.
type certWarmer interface {
	Warm(ctx context.Context) error
}

// The ACME TLS-ALPN-01 protocol id (RFC 8737). Served on :443 alongside HTTP so
// on-demand certs for custom domains can be validated without extra ports.
const acmeTLSALPNProto = "acme-tls/1"

func newACMECertManager(cfg Config, decide func(ctx context.Context, name string) error) (*prodCertManager, error) {
	// CertMagic needs the cache to hand back the Config for each cert; the config
	// is created just after, so the callback closes over the pointer.
	var magic *certmagic.Config
	cache := certmagic.NewCache(certmagic.CacheOptions{
		GetConfigForCert: func(certmagic.Certificate) (*certmagic.Config, error) {
			return magic, nil
		},
	})

	mcfg := certmagic.Config{
		DefaultServerName: cfg.BaseDomain,
		OnDemand:          &certmagic.OnDemandConfig{DecisionFunc: decide},
	}
	if cfg.TLSStorageDir != "" {
		mcfg.Storage = &certmagic.FileStorage{Path: cfg.TLSStorageDir}
	}
	magic = certmagic.New(cache, mcfg)

	issuer := certmagic.ACMEIssuer{
		CA:     certmagic.LetsEncryptProductionCA,
		Email:  cfg.ACMEEmail,
		Agreed: true,
	}
	if cfg.ACMEStaging {
		issuer.CA = certmagic.LetsEncryptStagingCA
	}

	manageWildcard := false
	if cfg.CloudflareToken != "" {
		issuer.DNS01Solver = &certmagic.DNS01Solver{
			DNSManager: certmagic.DNSManager{
				DNSProvider: &cloudflare.Provider{APIToken: cfg.CloudflareToken},
			},
		}
		manageWildcard = true
	}
	magic.Issuers = []certmagic.Issuer{certmagic.NewACMEIssuer(magic, issuer)}

	return &prodCertManager{magic: magic, base: cfg.BaseDomain, manageWildcard: manageWildcard}, nil
}

func (p *prodCertManager) GetCertificate(hello *tls.ClientHelloInfo) (*tls.Certificate, error) {
	return p.magic.GetCertificate(hello)
}

// TLSConfig is the public HTTPS ingress config: CertMagic supplies certs per SNI
// and answers TLS-ALPN-01 challenges. The ingress speaks HTTP/1.1 only (the proxy
// is a manual HTTP/1.x reader/writer), so we must NOT advertise "h2" — otherwise
// browsers negotiate HTTP/2 over ALPN and the connection breaks.
func (p *prodCertManager) TLSConfig() *tls.Config {
	return &tls.Config{
		GetCertificate: p.magic.GetCertificate,
		NextProtos:     []string{"http/1.1", acmeTLSALPNProto},
		MinVersion:     tls.VersionTLS12,
	}
}

// Warm proactively obtains (and then keeps renewing) the wildcard cert when a
// DNS provider is configured. Without one, subdomains fall back to on-demand
// TLS-ALPN issuance, so this is best-effort and never fatal.
func (p *prodCertManager) Warm(ctx context.Context) error {
	if !p.manageWildcard {
		return nil
	}
	return p.magic.ManageAsync(ctx, []string{p.base, "*." + p.base})
}
