package agent

import "github.com/rift/rift/pkg/proto"

// Error wraps a frozen protocol error code (§8) with a human message and an
// optional actionable hint (e.g. an upgrade prompt for plan limits).
type Error struct {
	Code    string
	Message string
}

func (e *Error) Error() string {
	if e.Message != "" {
		return e.Code + ": " + e.Message
	}
	return e.Code
}

// Hint returns a short, user-facing suggestion for a given error code.
func (e *Error) Hint() string {
	switch e.Code {
	case proto.CodeAuthRequired, proto.CodeAuthInvalid:
		return "Run `rift login --token <key>` with a valid API key."
	case proto.CodePlanForbids, proto.CodeQuotaTunnels, proto.CodeQuotaBandwidth, proto.CodeQuotaRequests:
		return "Your plan does not allow this. Upgrade at https://rift.sh/pricing."
	case proto.CodeSubdomainTaken:
		return "That subdomain is in use — try another or omit --subdomain for a random one."
	case proto.CodeSubdomainForbidden:
		return "Reserved subdomains require a Pro plan."
	case proto.CodeDomainUnverified:
		return "Verify your custom domain in the dashboard first."
	case proto.CodePortUnavailable:
		return "No remote port available — omit --remote-port to get an ephemeral one."
	case proto.CodeUpstreamUnreachable:
		return "The local address is not reachable — is your service running?"
	default:
		return ""
	}
}

// errorFromResp converts a protocol ErrorMsg into an *Error.
func errorFromResp(m *proto.ErrorMsg) *Error {
	if m == nil {
		return &Error{Code: proto.CodeInternal, Message: "unknown error"}
	}
	return &Error{Code: m.Code, Message: m.Message}
}
