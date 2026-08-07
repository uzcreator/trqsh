package api_test

import (
	"bytes"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/trqsh-uz/trqsh/internal/api"
	"github.com/trqsh-uz/trqsh/internal/entitlerpc"
)

func getJSONAuth(t *testing.T, url, bearer string, out any) *http.Response {
	t.Helper()
	req, _ := http.NewRequest("GET", url, nil)
	if bearer != "" {
		req.Header.Set("Authorization", "Bearer "+bearer)
	}
	resp, err := http.DefaultClient.Do(req)
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

// reportTunnel posts a tunnel event to the internal RPC using the dev internal token.
func reportTunnel(t *testing.T, tsURL string, rep entitlerpc.TunnelReport) {
	t.Helper()
	buf, _ := json.Marshal(rep)
	req, _ := http.NewRequest("POST", tsURL+entitlerpc.PathReportTunnel, bytes.NewReader(buf))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set(entitlerpc.HeaderToken, api.DefaultConfig().InternalToken)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("report tunnel: %v", err)
	}
	_ = resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("report tunnel status %d", resp.StatusCode)
	}
}

func TestTunnelHistoryFlow(t *testing.T) {
	ts := testServer(t)
	access, _ := signupAndKey(t, ts, "hist@example.com")

	var me struct {
		Org struct {
			ID string `json:"id"`
		} `json:"org"`
	}
	getJSONAuth(t, ts.URL+"/v1/me", access, &me)
	if me.Org.ID == "" {
		t.Fatal("no org id from /me")
	}

	// Edge reports a tunnel open.
	rep := entitlerpc.TunnelReport{
		Action: "open", EdgeID: "e1", SessionID: "s1", TunnelID: "t1", AccountID: me.Org.ID,
		Type: "http", PublicURL: "https://demo.trqsh.uz", Region: "eu", ClientIP: "8.8.8.8", At: time.Now(),
	}
	reportTunnel(t, ts.URL, rep)

	// It appears in the live list.
	var live []map[string]any
	getJSONAuth(t, ts.URL+"/v1/tunnels", access, &live)
	if len(live) != 1 || live[0]["public_url"] != "https://demo.trqsh.uz" {
		t.Fatalf("live tunnels = %+v", live)
	}

	// Close it with final traffic.
	rep.Action, rep.BytesIn, rep.BytesOut, rep.Requests, rep.At = "close", 10, 20, 2, time.Now()
	reportTunnel(t, ts.URL, rep)

	// Live list is now empty; history has the closed session.
	getJSONAuth(t, ts.URL+"/v1/tunnels", access, &live)
	if len(live) != 0 {
		t.Fatalf("expected no live tunnels, got %d", len(live))
	}
	var hist struct {
		Sessions []map[string]any `json:"sessions"`
		Total    int              `json:"total"`
	}
	getJSONAuth(t, ts.URL+"/v1/tunnels/history", access, &hist)
	if hist.Total != 1 || len(hist.Sessions) != 1 {
		t.Fatalf("history = %+v", hist)
	}
	if hist.Sessions[0]["status"] != "closed" {
		t.Fatalf("session status = %v, want closed", hist.Sessions[0]["status"])
	}

	// Usage history endpoint responds with a series shape.
	var usage struct {
		Bucket string `json:"bucket"`
		Series []any  `json:"series"`
	}
	resp := getJSONAuth(t, ts.URL+"/v1/usage/history?bucket=day&days=7", access, &usage)
	if resp.StatusCode != http.StatusOK || usage.Bucket != "day" {
		t.Fatalf("usage history: status %d bucket %q", resp.StatusCode, usage.Bucket)
	}
}

func adminServer(t *testing.T) *httptest.Server {
	t.Helper()
	cfg := api.DefaultConfig()
	cfg.DevAuth = true
	cfg.AdminUser = "admin"
	cfg.AdminPassword = "secret"
	srv, err := api.New(cfg, slog.New(slog.NewTextHandler(io.Discard, nil)))
	if err != nil {
		t.Fatalf("api.New: %v", err)
	}
	ts := httptest.NewServer(srv.Router())
	t.Cleanup(ts.Close)
	return ts
}

func TestAdminDashboardEndpoints(t *testing.T) {
	ts := adminServer(t)
	// Populate a little data.
	signupAndKey(t, ts, "a@example.com")
	signupAndKey(t, ts, "b@example.com")

	// Unauthenticated admin access is rejected.
	if resp := getJSONAuth(t, ts.URL+"/v1/admin/stats", "", nil); resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("admin stats without login = %d, want 401", resp.StatusCode)
	}

	// Admin login (sets the session cookie in a jar).
	jar, _ := cookiejar.New(nil)
	client := &http.Client{Jar: jar}
	body, _ := json.Marshal(map[string]string{"username": "admin", "password": "secret"})
	resp, err := client.Post(ts.URL+"/admin/login", "application/json", bytes.NewReader(body))
	if err != nil || resp.StatusCode != http.StatusOK {
		t.Fatalf("admin login: err=%v status=%v", err, resp.StatusCode)
	}
	_ = resp.Body.Close()

	get := func(path string, out any) {
		r, err := client.Get(ts.URL + path)
		if err != nil {
			t.Fatalf("GET %s: %v", path, err)
		}
		defer func() { _ = r.Body.Close() }()
		if r.StatusCode != http.StatusOK {
			t.Fatalf("GET %s status %d", path, r.StatusCode)
		}
		if out != nil {
			_ = json.NewDecoder(r.Body).Decode(out)
		}
	}

	var stats struct {
		Stats struct {
			Users int `json:"users"`
			Orgs  int `json:"orgs"`
		} `json:"stats"`
	}
	get("/v1/admin/stats", &stats)
	if stats.Stats.Users < 2 || stats.Stats.Orgs < 2 {
		t.Fatalf("admin stats undercount: %+v", stats.Stats)
	}

	var users struct {
		Users []map[string]any `json:"users"`
	}
	get("/v1/admin/users", &users)
	if len(users.Users) < 2 {
		t.Fatalf("admin users = %d, want >=2", len(users.Users))
	}

	var orgs struct {
		Orgs []map[string]any `json:"orgs"`
	}
	get("/v1/admin/orgs", &orgs)
	if len(orgs.Orgs) < 2 {
		t.Fatalf("admin orgs = %d, want >=2", len(orgs.Orgs))
	}

	// Tunnels view responds (may be empty).
	get("/v1/admin/tunnels", nil)
}
