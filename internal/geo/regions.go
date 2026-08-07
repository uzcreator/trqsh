package geo

import (
	"encoding/json"
	"sort"
	"strings"
)

// Region describes one trqsh edge PoP: where it physically sits and the endpoint
// agents dial to reach it. The catalog is advertised at GET /v1/regions and drives
// nearest-PoP selection so a developer connects to a geographically close edge
// (lower latency) instead of a fixed one. Codes mirror the deploy topology
// (deploy/terraform: fra1/nyc3/sgp1).
type Region struct {
	Code      string  `json:"code"`               // stable id, e.g. "eu"
	Name      string  `json:"name"`               // human label, e.g. "Europe (Frankfurt)"
	City      string  `json:"city,omitempty"`
	Country   string  `json:"country,omitempty"`  // ISO alpha-2 the PoP sits in
	Continent string  `json:"continent"`          // AF|AN|AS|EU|NA|OC|SA — the match key
	Endpoint  string  `json:"endpoint"`           // host:port agents dial
	Lat       float64 `json:"lat,omitempty"`
	Lon       float64 `json:"lon,omitempty"`
	Default   bool    `json:"default,omitempty"`  // chosen when nothing else matches
}

// DefaultRegions is the built-in PoP catalog used when TRQSH_REGIONS is unset. It
// intentionally matches the reference Terraform edge_regions so a fresh deployment
// advertises sane endpoints out of the box; override via config in production.
func DefaultRegions() []Region {
	return []Region{
		{Code: "eu", Name: "Europe (Frankfurt)", City: "Frankfurt", Country: "DE", Continent: "EU", Endpoint: "eu.trqsh.uz:443", Lat: 50.11, Lon: 8.68, Default: true},
		{Code: "us", Name: "US East (New York)", City: "New York", Country: "US", Continent: "NA", Endpoint: "us.trqsh.uz:443", Lat: 40.71, Lon: -74.01},
		{Code: "ap", Name: "Asia Pacific (Singapore)", City: "Singapore", Country: "SG", Continent: "AS", Endpoint: "ap.trqsh.uz:443", Lat: 1.35, Lon: 103.82},
	}
}

// ParseRegions decodes a TRQSH_REGIONS JSON array into a catalog. An empty string
// or any parse error returns (nil, err|nil) so the caller can fall back to
// DefaultRegions. Codes and continents are normalized.
func ParseRegions(jsonArray string) ([]Region, error) {
	jsonArray = strings.TrimSpace(jsonArray)
	if jsonArray == "" {
		return nil, nil
	}
	var regions []Region
	if err := json.Unmarshal([]byte(jsonArray), &regions); err != nil {
		return nil, err
	}
	for i := range regions {
		regions[i].Code = strings.ToLower(strings.TrimSpace(regions[i].Code))
		regions[i].Continent = strings.ToUpper(strings.TrimSpace(regions[i].Continent))
	}
	return regions, nil
}

// nearestRegion picks the best PoP for a location. Selection is coarse but robust
// and dependency-free: exact continent match wins; otherwise the catalog default
// (or first entry) is used. This mirrors how GeoDNS would steer the shared
// edge.<base> hostname, but is also exposed directly so clients can display and
// override the choice.
func nearestRegion(catalog []Region, loc Location) Region {
	if len(catalog) == 0 {
		return Region{}
	}
	continent := loc.Continent
	if continent == "" && loc.Country != "" {
		if info, ok := countries[loc.Country]; ok {
			continent = info.continent
		}
	}
	if continent != "" {
		for _, r := range catalog {
			if r.Continent == continent {
				return r
			}
		}
		// Nearby-continent fallbacks so a location without an exact PoP still lands
		// somewhere sensible (e.g. South America -> North America, Africa -> Europe).
		for _, r := range catalog {
			if r.Continent == continentFallback(continent) {
				return r
			}
		}
	}
	return defaultRegion(catalog)
}

// continentFallback maps a continent with (often) no dedicated PoP to the closest
// one that usually does.
func continentFallback(continent string) string {
	switch continent {
	case "SA": // South America -> North America
		return "NA"
	case "AF": // Africa -> Europe
		return "EU"
	case "OC": // Oceania -> Asia
		return "AS"
	case "AN": // Antarctica -> Oceania (then Asia via default)
		return "OC"
	default:
		return ""
	}
}

func defaultRegion(catalog []Region) Region {
	for _, r := range catalog {
		if r.Default {
			return r
		}
	}
	return catalog[0]
}

// SortRegions orders a catalog by code for stable API output.
func SortRegions(regions []Region) {
	sort.Slice(regions, func(i, j int) bool { return regions[i].Code < regions[j].Code })
}
