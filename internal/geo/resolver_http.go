package geo

import (
	"context"
	"encoding/json"
	"io"
	"net"
	"net/http"
	"strings"
	"time"
)

// httpResolver resolves an IP by calling an external GeoIP JSON provider. It is
// opt-in (Config.APIURL) because it sends the client IP to a third party and adds
// a network hop; operators who front trqsh with a CDN should prefer the country
// header instead. The default template targets ip-api.com's free JSON endpoint,
// but any provider returning a superset of those fields works.
//
// It fails open: any transport, status, or decode error yields ok=false so the
// Service falls through to "unknown" rather than erroring the request.
type httpResolver struct {
	urlTemplate string
	client      *http.Client
}

func newHTTPResolver(urlTemplate string, timeout time.Duration) *httpResolver {
	return &httpResolver{
		urlTemplate: urlTemplate,
		// Proxy disabled: this is a direct provider call, not proxied traffic.
		client: &http.Client{Timeout: timeout, Transport: &http.Transport{Proxy: nil}},
	}
}

// providerResponse is the union of fields we read across common providers
// (ip-api.com naming). Absent fields simply stay zero.
type providerResponse struct {
	Status      string  `json:"status"`  // ip-api: "success" | "fail"
	Message     string  `json:"message"` // failure reason
	CountryCode string  `json:"countryCode"`
	Country     string  `json:"country"`
	RegionName  string  `json:"regionName"`
	City        string  `json:"city"`
	Lat         float64 `json:"lat"`
	Lon         float64 `json:"lon"`
}

func (h *httpResolver) Lookup(ctx context.Context, ip string) (Location, bool) {
	// Only ever interpolate a validated IP literal into the provider URL. ip can
	// originate from a client-supplied header (X-Forwarded-For behind a trusted
	// proxy), so an unvalidated value could inject a different host or path into
	// our outbound request (SSRF — CWE-918). net.ParseIP rejects anything that is
	// not a bare IPv4/IPv6 address, so no URL metacharacters (/, @, ?, #, …) can
	// survive into the request target.
	parsedIP := net.ParseIP(ip)
	if parsedIP == nil {
		return Location{}, false
	}
	// Interpolate the canonical form of the validated IP, never the raw input, so
	// the outbound request target provably cannot carry user-controlled host/path
	// characters (breaks the taint flow at the value, not just behind a guard).
	url := strings.ReplaceAll(h.urlTemplate, "{ip}", parsedIP.String())
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil) // #nosec G704 -- url derives only from a net.ParseIP-validated IP (checked above) and the operator-configured template; there is no user-controlled host or path
	if err != nil {
		return Location{}, false
	}
	req.Header.Set("Accept", "application/json")
	resp, err := h.client.Do(req) // #nosec G704 -- see above: the request target is a validated IP literal interpolated into the operator's template, not attacker-controlled
	if err != nil {
		return Location{}, false
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return Location{}, false
	}
	var pr providerResponse
	if err := json.NewDecoder(io.LimitReader(resp.Body, 1<<16)).Decode(&pr); err != nil {
		return Location{}, false
	}
	// ip-api signals per-IP failures (reserved ranges, rate limit) in the body.
	if pr.Status != "" && !strings.EqualFold(pr.Status, "success") {
		return Location{}, false
	}
	cc := normalizeCountry(pr.CountryCode)
	if cc == "" {
		return Location{}, false
	}
	return Location{
		IP:          ip,
		Country:     cc,
		CountryName: strings.TrimSpace(pr.Country),
		Region:      strings.TrimSpace(pr.RegionName),
		City:        strings.TrimSpace(pr.City),
		Lat:         pr.Lat,
		Lon:         pr.Lon,
		Source:      "api",
	}, true
}
