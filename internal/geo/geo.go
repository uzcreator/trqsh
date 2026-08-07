// Package geo resolves a client IP to an approximate location (country/city) and
// maps that location to the nearest trqsh edge region. It is the "location" seam
// used by the control API to (a) tell an agent/desktop which edge PoP to connect
// to, and (b) enrich tunnel history + the admin dashboard with where sessions and
// visitors come from.
//
// Resolution is deliberately pluggable and dependency-free: it tries, in order, a
// private-IP short-circuit, a trusted proxy/CDN country header, and an optional
// external HTTP GeoIP provider (opt-in via config). A MaxMind mmdb resolver can be
// slotted into the same Resolver chain later without touching callers. Every path
// fails open to "unknown" — geo is best-effort metadata, never a hard dependency.
package geo

import (
	"context"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"
)

// Location is a best-effort geolocation for an IP. Zero value = unknown.
type Location struct {
	IP          string  `json:"ip,omitempty"`
	Country     string  `json:"country,omitempty"`      // ISO 3166-1 alpha-2, uppercase (e.g. "US")
	CountryName string  `json:"country_name,omitempty"` // English name (e.g. "United States")
	Continent   string  `json:"continent,omitempty"`    // AF|AN|AS|EU|NA|OC|SA
	Region      string  `json:"region,omitempty"`       // subdivision/state, best-effort
	City        string  `json:"city,omitempty"`
	Lat         float64 `json:"lat,omitempty"`
	Lon         float64 `json:"lon,omitempty"`
	Source      string  `json:"source,omitempty"` // which resolver answered
}

// Known reports whether a country was determined.
func (l Location) Known() bool { return l.Country != "" }

// Resolver maps an IP string to a Location. ok=false means "unknown" — callers
// treat that as no-geo, not an error. Implementations must be concurrency-safe.
type Resolver interface {
	Lookup(ctx context.Context, ip string) (Location, bool)
}

// Config configures the geo Service (env-sourced by the caller).
type Config struct {
	// Header, when set, is a request header carrying an ISO country code injected
	// by a trusted front proxy/CDN (e.g. "CF-IPCountry" behind Cloudflare, or a
	// custom "X-Geo-Country"). Only trust it when the API is actually behind that
	// proxy. Empty disables the header path.
	Header string
	// APIURL, when set, enables the external HTTP GeoIP provider. It is a URL
	// template with a "{ip}" placeholder returning ip-api.com-style JSON, e.g.
	// "http://ip-api.com/json/{ip}?fields=status,countryCode,country,regionName,city,lat,lon".
	// Empty disables the network path (header-only / unknown).
	APIURL string
	// HTTPTimeout bounds a single provider lookup. Defaults to 2s.
	HTTPTimeout time.Duration
	// CacheTTL is how long a resolved (or negatively-resolved) IP is cached.
	// Defaults to 1h. Set <0 to disable caching.
	CacheTTL time.Duration
	// Regions is the edge PoP catalog advertised to clients and used for nearest-
	// region selection. Empty falls back to DefaultRegions().
	Regions []Region
}

// Service resolves locations and selects regions. Safe for concurrent use.
type Service struct {
	header    string
	resolvers []Resolver
	catalog   []Region
	cache     *ttlCache
}

// New builds a geo Service from cfg. It never fails: an empty/zero cfg yields a
// service that only knows private IPs and the region catalog (all public IPs
// resolve to "unknown"), which is the safe default when no GeoIP source is wired.
func New(cfg Config) *Service {
	if cfg.HTTPTimeout <= 0 {
		cfg.HTTPTimeout = 2 * time.Second
	}
	if cfg.CacheTTL == 0 {
		cfg.CacheTTL = time.Hour
	}
	regions := cfg.Regions
	if len(regions) == 0 {
		regions = DefaultRegions()
	}

	s := &Service{
		header:  http.CanonicalHeaderKey(strings.TrimSpace(cfg.Header)),
		catalog: regions,
	}
	if cfg.CacheTTL > 0 {
		s.cache = newTTLCache(cfg.CacheTTL)
	}
	// Order matters: cheapest/most-authoritative first. The private-IP resolver
	// short-circuits LANs so we never ship a dev/internal IP to a provider.
	s.resolvers = append(s.resolvers, privateResolver{})
	if cfg.APIURL != "" {
		s.resolvers = append(s.resolvers, newHTTPResolver(cfg.APIURL, cfg.HTTPTimeout))
	}
	return s
}

// FromRequest locates the client of an HTTP request. It prefers the trusted
// country header (when configured and present), then falls back to resolving ip.
// ip should already be the real client IP (e.g. from the trusted-proxy-aware
// extractor); this method does not itself parse X-Forwarded-For.
func (s *Service) FromRequest(r *http.Request, ip string) Location {
	if s.header != "" && r != nil {
		if cc := normalizeCountry(r.Header.Get(s.header)); cc != "" {
			loc := Location{IP: ip, Source: "header"}
			applyCountry(&loc, cc)
			return loc
		}
	}
	return s.FromIP(r.Context(), ip)
}

// FromIP resolves an IP to a Location via the resolver chain (with caching).
func (s *Service) FromIP(ctx context.Context, ip string) Location {
	ip = strings.TrimSpace(ip)
	if ip == "" {
		return Location{}
	}
	if s.cache != nil {
		if loc, ok := s.cache.get(ip); ok {
			return loc
		}
	}
	var out Location
	for _, r := range s.resolvers {
		if loc, ok := r.Lookup(ctx, ip); ok {
			out = loc
			break
		}
	}
	out.IP = ip
	// Backfill country name/continent from the static table when a resolver gave
	// only a code (or gave a name but no continent), so every consumer sees a
	// consistent, fully-populated Location.
	if out.Country != "" {
		applyCountry(&out, out.Country)
	}
	if s.cache != nil {
		s.cache.set(ip, out)
	}
	return out
}

// Regions returns the advertised edge PoP catalog (defensive copy).
func (s *Service) Regions() []Region {
	out := make([]Region, len(s.catalog))
	copy(out, s.catalog)
	return out
}

// Nearest returns the recommended edge region for a location, falling back to the
// catalog's default when the location is unknown or its continent has no PoP.
func (s *Service) Nearest(loc Location) Region {
	return nearestRegion(s.catalog, loc)
}

// applyCountry fills Country/CountryName/Continent from the static country table
// for a (possibly lowercase) ISO code, preserving any richer data already set.
func applyCountry(loc *Location, code string) {
	cc := normalizeCountry(code)
	if cc == "" {
		return
	}
	loc.Country = cc
	if info, ok := countries[cc]; ok {
		if loc.CountryName == "" {
			loc.CountryName = info.name
		}
		if loc.Continent == "" {
			loc.Continent = info.continent
		}
	}
}

// normalizeCountry upper-cases and validates a 2-letter ISO country code.
func normalizeCountry(s string) string {
	s = strings.ToUpper(strings.TrimSpace(s))
	if len(s) != 2 || s[0] < 'A' || s[0] > 'Z' || s[1] < 'A' || s[1] > 'Z' {
		return ""
	}
	return s
}

// --- private-IP resolver ---

// privateResolver classifies loopback/private/link-local/unspecified addresses so
// dev and internal traffic never hits an external provider and is clearly marked.
type privateResolver struct{}

func (privateResolver) Lookup(_ context.Context, ip string) (Location, bool) {
	parsed := net.ParseIP(ip)
	if parsed == nil {
		return Location{}, false
	}
	if parsed.IsLoopback() || parsed.IsPrivate() || parsed.IsLinkLocalUnicast() || parsed.IsUnspecified() {
		return Location{IP: ip, Source: "private"}, true
	}
	return Location{}, false
}

// --- TTL cache ---

type cacheEntry struct {
	loc    Location
	expiry time.Time
}

// ttlCache is a tiny mutex-guarded IP->Location cache with a size cap. On
// overflow it drops the whole map (simplest bounded strategy; entries are cheap
// to re-resolve). Adequate for the request-rate a control API sees.
type ttlCache struct {
	ttl time.Duration
	mu  sync.Mutex
	m   map[string]cacheEntry
	max int
}

func newTTLCache(ttl time.Duration) *ttlCache {
	return &ttlCache{ttl: ttl, m: make(map[string]cacheEntry), max: 50000}
}

func (c *ttlCache) get(ip string) (Location, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	e, ok := c.m[ip]
	if !ok || time.Now().After(e.expiry) {
		return Location{}, false
	}
	return e.loc, true
}

func (c *ttlCache) set(ip string, loc Location) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if len(c.m) >= c.max {
		c.m = make(map[string]cacheEntry, c.max/2)
	}
	c.m[ip] = cacheEntry{loc: loc, expiry: time.Now().Add(c.ttl)}
}
