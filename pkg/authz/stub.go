package authz

import (
	"context"
	"errors"
)

// StubEntitlements is an allow-all implementation for local development and
// tests. The edge (Part 02) uses it until the real control plane (Part 05)
// is wired in.
type StubEntitlements struct{}

// Authenticate accepts any non-empty API key and returns a dev account on the
// pro plan.
func (StubEntitlements) Authenticate(_ context.Context, apiKey string) (string, string, error) {
	if apiKey == "" {
		return "", "", errors.New("authz: empty api key")
	}
	return "dev", "pro", nil
}

// CheckBind allows every bind with generous limits.
func (StubEntitlements) CheckBind(_ context.Context, _ BindRequest) (Decision, error) {
	return Decision{
		Allow:     true,
		AccountID: "dev",
		Plan:      "pro",
		Limits: Limits{
			MaxConcurrentTunnels:   100,
			AllowCustomDomains:     true,
			AllowReservedSubdomain: true,
			AllowTCP:               true,
			AllowUDP:               true,
		},
	}, nil
}

// ReportUsage discards usage.
func (StubEntitlements) ReportUsage(_ context.Context, _ Usage) error { return nil }

// StubEntitlements must satisfy the Entitlements interface.
var _ Entitlements = StubEntitlements{}
