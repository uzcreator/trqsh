package api_test

import (
	"encoding/json"
	"io"
	"net/http"
	"testing"
)

// TestPlansPublicUnauthenticatedAndOrdered proves the endpoint the marketing
// site's build-time genplans.mjs depends on: no auth required, and plans come
// back in display order (free, pro, team, payg) rather than Go's randomized
// map order, since the site renders them in that sequence without re-sorting.
func TestPlansPublicUnauthenticatedAndOrdered(t *testing.T) {
	ts := testServer(t)

	resp, err := http.Get(ts.URL + "/v1/plans/public")
	if err != nil {
		t.Fatalf("GET /v1/plans/public: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200 (no auth header sent)", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read body: %v", err)
	}
	var plans []map[string]any
	if err := json.Unmarshal(body, &plans); err != nil {
		t.Fatalf("decode: %v (body: %s)", err, body)
	}
	if len(plans) != 4 {
		t.Fatalf("got %d plans, want 4", len(plans))
	}

	wantOrder := []string{"free", "pro", "team", "payg"}
	for i, code := range wantOrder {
		if got := plans[i]["code"]; got != code {
			t.Errorf("plans[%d].code = %v, want %q", i, got, code)
		}
	}

	for _, p := range plans {
		if _, present := p["stripe_prices"]; present {
			t.Errorf("plan %v: stripe_prices must not appear in the public catalog", p["code"])
		}
	}
}
