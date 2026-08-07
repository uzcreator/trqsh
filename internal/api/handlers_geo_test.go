package api_test

import (
	"encoding/json"
	"io"
	"net/http"
	"testing"
)

func getJSON(t *testing.T, url string, out any) *http.Response {
	t.Helper()
	resp, err := http.Get(url) //nolint:noctx // test
	if err != nil {
		t.Fatalf("GET %s: %v", url, err)
	}
	if out != nil {
		b, _ := io.ReadAll(resp.Body)
		_ = resp.Body.Close()
		if err := json.Unmarshal(b, out); err != nil {
			t.Fatalf("decode %s: %v (body: %s)", url, err, b)
		}
	}
	return resp
}

func TestRegionsEndpoint(t *testing.T) {
	ts := testServer(t)
	var regions []struct {
		Code     string `json:"code"`
		Endpoint string `json:"endpoint"`
	}
	resp := getJSON(t, ts.URL+"/v1/regions", &regions)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status %d", resp.StatusCode)
	}
	if len(regions) != 3 {
		t.Fatalf("expected 3 built-in regions, got %d", len(regions))
	}
	seen := map[string]bool{}
	for _, r := range regions {
		if r.Endpoint == "" {
			t.Errorf("region %s has no endpoint", r.Code)
		}
		seen[r.Code] = true
	}
	for _, want := range []string{"eu", "us", "ap"} {
		if !seen[want] {
			t.Errorf("missing region %q", want)
		}
	}
}

func TestGeoEndpoint(t *testing.T) {
	ts := testServer(t) // DevAuth on => ?ip override honored

	// A private/loopback IP is not "detected" but still gets a recommended region
	// (the catalog default) and the full catalog.
	var geo struct {
		Detected bool `json:"detected"`
		Region   struct {
			Code string `json:"code"`
		} `json:"region"`
		Regions []json.RawMessage `json:"regions"`
	}
	resp := getJSON(t, ts.URL+"/v1/geo?ip=127.0.0.1", &geo)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status %d", resp.StatusCode)
	}
	if geo.Detected {
		t.Errorf("loopback should not be detected")
	}
	if geo.Region.Code != "eu" {
		t.Errorf("default region = %q, want eu", geo.Region.Code)
	}
	if len(geo.Regions) != 3 {
		t.Errorf("catalog size = %d, want 3", len(geo.Regions))
	}
}
