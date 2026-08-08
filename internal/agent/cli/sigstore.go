package cli

// Verifies the cosign/Sigstore bundle that CI publishes alongside
// checksums.txt (see .goreleaser.yaml's signs: block), so self-update no
// longer trusts checksums.txt on HTTPS-delivery alone — the same guarantee
// the packaging READMEs already document as a manual `cosign verify-blob`
// command for anyone who wants to check by hand.

import (
	"bytes"
	"fmt"

	"github.com/sigstore/sigstore-go/pkg/bundle"
	"github.com/sigstore/sigstore-go/pkg/root"
	"github.com/sigstore/sigstore-go/pkg/verify"
)

// releaseCertIdentityRegexp and releaseCertOIDCIssuer must match the values
// documented in packaging/npm/README.md and packaging/pypi/README.md's
// manual `cosign verify-blob` instructions: the signature was minted by the
// uzcreator/trqsh repo's own GitHub Actions OIDC token (release.yml), not by
// uzcreator/trqshcli, which only hosts the published artifacts.
//
// releaseCertIdentityRegexp is a var (not const) so a test can swap in a
// mismatched identity to prove verifySigstoreBundle actually enforces it.
var releaseCertIdentityRegexp = "https://github.com/uzcreator/trqsh/.*"

const releaseCertOIDCIssuer = "https://token.actions.githubusercontent.com"

// verifySigstoreBundleFunc is called by installUpdate; tests that don't want
// to hit real Sigstore infra (or fake one — a valid bundle needs a real
// Fulcio-issued cert) swap this for a stub, the same indirection
// updateReleasesAPIURL/updateDownloadBase use for the release API/download.
var verifySigstoreBundleFunc = verifySigstoreBundle

// verifySigstoreBundle checks that sigstoreJSON is a valid Sigstore bundle
// over checksumsData, signed by the expected release workflow identity. Any
// error means the bundle didn't verify — installUpdate treats that as fatal,
// the same fail-closed posture it already applies to a missing checksums.txt.
func verifySigstoreBundle(checksumsData, sigstoreJSON []byte) error {
	trustedRoot, err := root.FetchTrustedRoot()
	if err != nil {
		return fmt.Errorf("fetch sigstore trusted root: %w", err)
	}

	verifier, err := verify.NewVerifier(trustedRoot,
		verify.WithSignedCertificateTimestamps(1),
		verify.WithTransparencyLog(1),
		verify.WithObserverTimestamps(1),
	)
	if err != nil {
		return fmt.Errorf("build sigstore verifier: %w", err)
	}

	var b bundle.Bundle
	if err := b.UnmarshalJSON(sigstoreJSON); err != nil {
		return fmt.Errorf("parse checksums.txt.sigstore.json: %w", err)
	}

	certID, err := verify.NewShortCertificateIdentity(
		releaseCertOIDCIssuer, "", "", releaseCertIdentityRegexp)
	if err != nil {
		return fmt.Errorf("build certificate identity: %w", err)
	}

	policy := verify.NewPolicy(
		verify.WithArtifact(bytes.NewReader(checksumsData)),
		verify.WithCertificateIdentity(certID),
	)
	if _, err := verifier.Verify(&b, policy); err != nil {
		return fmt.Errorf("sigstore verification failed: %w", err)
	}
	return nil
}
