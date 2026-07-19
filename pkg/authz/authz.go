package authz

import (
	"context"
	"time"
)

// Limits describes what a plan permits. A zero value means "no allowance";
// unlimited numeric fields are represented by 0 only where noted.
type Limits struct {
	MaxConcurrentTunnels   int
	MaxBandwidthBytesMo    int64 // 0 = unlimited
	MaxRequestsMo          int64 // 0 = unlimited
	AllowCustomDomains     bool
	AllowReservedSubdomain bool
	AllowTCP               bool
	AllowUDP               bool
	RateLimitRPS           int // 0 = unlimited
}

// BindRequest is what the edge asks the control plane to authorize when an
// agent binds a tunnel.
type BindRequest struct {
	APIKey     string
	Type       string // "http|https|tls|tcp|udp"
	Subdomain  string
	CustomHost string
	RemotePort int
	Region     string
}

// Decision is the control plane's answer to a BindRequest.
type Decision struct {
	Allow             bool
	AccountID         string
	Plan              string
	Limits            Limits
	AssignedSubdomain string // server may assign a random one
	ErrorCode         string // frozen code (proto.Code*) when Allow=false
	ErrorMessage      string
}

// Usage is a metered slice of traffic reported by the edge for billing.
type Usage struct {
	AccountID   string
	TunnelID    string
	BytesIn     int64
	BytesOut    int64
	Requests    int64
	WindowStart time.Time
	WindowEnd   time.Time
}

// Entitlements is the seam between the edge (Part 02) and the control/billing
// plane (Parts 05/07). The edge compiles against this interface and swaps
// StubEntitlements for the real implementation without any protocol change.
type Entitlements interface {
	Authenticate(ctx context.Context, apiKey string) (accountID, plan string, err error)
	CheckBind(ctx context.Context, req BindRequest) (Decision, error)
	ReportUsage(ctx context.Context, u Usage) error
}
