package geo

import (
	"context"
	"encoding/json"
	"io"
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
	url := strings.ReplaceAll(h.urlTemplate, "{ip}", ip)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return Location{}, false
	}
	req.Header.Set("Accept", "application/json")
	resp, err := h.client.Do(req)
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
