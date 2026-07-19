package proto

// Frozen error taxonomy carried in ErrorMsg.code and surfaced by the CLI, GUI,
// and API. See plan/00-ARCHITECTURE.md section 8.
const (
	CodeAuthRequired        = "ERR_AUTH_REQUIRED"
	CodeAuthInvalid         = "ERR_AUTH_INVALID"
	CodeVersionUnsupported  = "ERR_VERSION_UNSUPPORTED"
	CodeQuotaTunnels        = "ERR_QUOTA_TUNNELS"
	CodeQuotaBandwidth      = "ERR_QUOTA_BANDWIDTH"
	CodeQuotaRequests       = "ERR_QUOTA_REQUESTS"
	CodeSubdomainTaken      = "ERR_SUBDOMAIN_TAKEN"
	CodeSubdomainForbidden  = "ERR_SUBDOMAIN_FORBIDDEN"
	CodeDomainUnverified    = "ERR_DOMAIN_UNVERIFIED"
	CodePlanForbids         = "ERR_PLAN_FORBIDS"
	CodeProtoUnsupported    = "ERR_PROTO_UNSUPPORTED"
	CodePortUnavailable     = "ERR_PORT_UNAVAILABLE"
	CodeRateLimited         = "ERR_RATE_LIMITED"
	CodeInternal            = "ERR_INTERNAL"
	CodeUpstreamUnreachable = "ERR_UPSTREAM_UNREACHABLE"
)

// NewError builds an ErrorMsg with the given frozen code and a human message.
func NewError(code, msg string) *ErrorMsg {
	return &ErrorMsg{Code: code, Message: msg}
}
