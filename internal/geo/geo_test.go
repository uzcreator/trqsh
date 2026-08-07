package geo

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestPrivateAndUnknown(t *testing.T) {
	s := New(Config{}) // no provider: only private IPs resolve
	if loc := s.FromIP(context.Background(), "127.0.0.1"); loc.Source != "private" || loc.Known() {
		t.Fatalf("loopback: got %+v", loc)
	}
	if loc := s.FromIP(context.Background(), "10.0.0.5"); loc.Source != "private" {
		t.Fatalf("private: got %+v", loc)
	}
	// A public IP with no provider configured is simply unknown (fails open).
	if loc := s.FromIP(context.Background(), "8.8.8.8"); loc.Known() {
		t.Fatalf("public without provider should be unknown, got %+v", loc)
	}
}

func TestHTTPResolverAndBackfill(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		// ip-api.com-style success payload with only a country code + city.
		_, _ = w.Write([]byte(`{"status":"success","countryCode":"de","city":"Berlin","lat":52.5,"lon":13.4}`))
	}))
	defer srv.Close()

	s := New(Config{APIURL: srv.URL + "/{ip}"})
	loc := s.FromIP(context.Background(), "203.0.113.7")
	if loc.Country != "DE" {
		t.Fatalf("country: got %q", loc.Country)
	}
	// Name + continent are backfilled from the static table even though the
	// provider only returned a code.
	if loc.CountryName != "Germany" || loc.Continent != "EU" {
		t.Fatalf("backfill failed: %+v", loc)
	}
	if loc.City != "Berlin" || loc.Source != "api" {
		t.Fatalf("fields: %+v", loc)
	}
	// Second lookup should be served from cache (same result).
	if again := s.FromIP(context.Background(), "203.0.113.7"); again.Country != "DE" {
		t.Fatalf("cache miss: %+v", again)
	}
}

func TestHeaderPath(t *testing.T) {
	s := New(Config{Header: "CF-IPCountry"})
	r := httptest.NewRequest(http.MethodGet, "/v1/geo", nil)
	r.Header.Set("CF-IPCountry", "JP")
	loc := s.FromRequest(r, "203.0.113.9")
	if loc.Country != "JP" || loc.CountryName != "Japan" || loc.Source != "header" {
		t.Fatalf("header path: %+v", loc)
	}
}

func TestNearestRegion(t *testing.T) {
	s := New(Config{})
	cases := map[string]string{
		"US": "us", // North America -> US PoP
		"DE": "eu", // Europe -> EU PoP
		"UZ": "ap", // Uzbekistan (Asia) -> AP PoP
		"SG": "ap",
		"BR": "us", // South America has no PoP -> NA fallback
		"NG": "eu", // Africa -> Europe fallback
		"AU": "ap", // Oceania -> Asia fallback
	}
	for country, want := range cases {
		loc := Location{Country: country, Continent: Continent(country)}
		if got := s.Nearest(loc).Code; got != want {
			t.Errorf("Nearest(%s)=%s, want %s", country, got, want)
		}
	}
	// Unknown location falls back to the default region.
	if got := s.Nearest(Location{}).Code; got != "eu" {
		t.Errorf("Nearest(unknown)=%s, want eu (default)", got)
	}
}

func TestParseRegions(t *testing.T) {
	regions, err := ParseRegions(`[{"code":"US","name":"US","continent":"na","endpoint":"us.example:443"}]`)
	if err != nil {
		t.Fatal(err)
	}
	if len(regions) != 1 || regions[0].Code != "us" || regions[0].Continent != "NA" {
		t.Fatalf("normalize failed: %+v", regions)
	}
	if empty, err := ParseRegions("  "); err != nil || empty != nil {
		t.Fatalf("empty should be (nil,nil): %v %v", empty, err)
	}
}
