package api_test

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/trqsh-uz/trqsh/internal/api"
	"github.com/trqsh-uz/trqsh/internal/entitlerpc"
	"github.com/trqsh-uz/trqsh/pkg/authz"
	"github.com/trqsh-uz/trqsh/pkg/proto"
)

func testServer(t *testing.T) *httptest.Server {
	t.Helper()
	cfg := api.DefaultConfig()
	cfg.DevAuth = true
	srv, err := api.New(cfg, slog.New(slog.NewTextHandler(io.Discard, nil)))
	if err != nil {
		t.Fatalf("api.New: %v", err)
	}
	ts := httptest.NewServer(srv.Router())
	t.Cleanup(ts.Close)
	return ts
}

func postJSON(t *testing.T, url, bearer string, body any, out any) *http.Response {
	t.Helper()
	buf, _ := json.Marshal(body)
	req, _ := http.NewRequest("POST", url, bytes.NewReader(buf))
	req.Header.Set("Content-Type", "application/json")
	if bearer != "" {
		req.Header.Set("Authorization", "Bearer "+bearer)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("POST %s: %v", url, err)
	}
	if out != nil {
		b, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		if err := json.Unmarshal(b, out); err != nil {
			t.Fatalf("decode %s: %v (body: %s)", url, err, b)
		}
	}
	return resp
}

// signupAndKey creates an account and returns (accessToken, apiKey).
func signupAndKey(t *testing.T, ts *httptest.Server, email string) (string, string) {
	t.Helper()
	var signup struct {
		Tokens struct {
			Access string `json:"access_token"`
		} `json:"tokens"`
	}
	postJSON(t, ts.URL+"/v1/auth/signup", "", map[string]string{"email": email, "name": "Test"}, &signup)
	if signup.Tokens.Access == "" {
		t.Fatal("no access token from signup")
	}
	var key struct {
		APIKey string `json:"api_key"`
	}
	resp := postJSON(t, ts.URL+"/v1/api-keys", signup.Tokens.Access, map[string]string{"name": "cli"}, &key)
	if resp.StatusCode != http.StatusCreated || !strings.HasPrefix(key.APIKey, "tq_live_") {
		t.Fatalf("create api-key failed: status %d key %q", resp.StatusCode, key.APIKey)
	}
	return signup.Tokens.Access, key.APIKey
}

func TestSignupAndAPIKeyLifecycle(t *testing.T) {
	ts := testServer(t)
	access, apiKey := signupAndKey(t, ts, "a@example.com")

	// List keys shows the created key (without plaintext).
	req, _ := http.NewRequest("GET", ts.URL+"/v1/api-keys", nil)
	req.Header.Set("Authorization", "Bearer "+access)
	resp, _ := http.DefaultClient.Do(req)
	body, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	if !strings.Contains(string(body), "tq_live_") || strings.Contains(string(body), apiKey) {
		t.Fatalf("list should show prefix but not the full key: %s", body)
	}

	// Unauthenticated request is rejected.
	r2, _ := http.Get(ts.URL + "/v1/api-keys")
	if r2.StatusCode != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", r2.StatusCode)
	}
	r2.Body.Close()
}

func TestEntitlementsOverInternalRPC(t *testing.T) {
	ts := testServer(t)
	_, apiKey := signupAndKey(t, ts, "b@example.com")

	// The edge's entitlements client talks to the API's internal RPC.
	client := entitlerpc.NewClient(ts.URL, api.DefaultConfig().InternalToken)
	ctx := context.Background()

	acct, plan, err := client.Authenticate(ctx, apiKey)
	if err != nil || plan != "free" || acct == "" {
		t.Fatalf("Authenticate over RPC: acct=%q plan=%q err=%v", acct, plan, err)
	}

	// Free plan denies UDP.
	dec, err := client.CheckBind(ctx, authz.BindRequest{APIKey: apiKey, Type: "udp"})
	if err != nil {
		t.Fatalf("CheckBind udp: %v", err)
	}
	if dec.Allow || dec.ErrorCode != proto.CodePlanForbids {
		t.Fatalf("free UDP should be denied with PLAN_FORBIDS, got allow=%v code=%s", dec.Allow, dec.ErrorCode)
	}

	// Free plan allows HTTP with an assigned subdomain.
	dec2, err := client.CheckBind(ctx, authz.BindRequest{APIKey: apiKey, Type: "http"})
	if err != nil {
		t.Fatalf("CheckBind http: %v", err)
	}
	if !dec2.Allow || dec2.AssignedSubdomain == "" {
		t.Fatalf("free HTTP should be allowed with a subdomain, got %+v", dec2)
	}

	// A bad internal token is rejected.
	bad := entitlerpc.NewClient(ts.URL, "wrong-token")
	if _, _, err := bad.Authenticate(ctx, apiKey); err == nil {
		t.Fatal("expected internal-token rejection")
	}
}

func TestReservedSubdomainLimit(t *testing.T) {
	ts := testServer(t)
	access, _ := signupAndKey(t, ts, "c@example.com")

	// Free plan allows exactly 1 reserved subdomain.
	r1 := postJSON(t, ts.URL+"/v1/subdomains", access, map[string]string{"subdomain": "first"}, nil)
	if r1.StatusCode != http.StatusCreated {
		t.Fatalf("first reserve: status %d", r1.StatusCode)
	}
	r2 := postJSON(t, ts.URL+"/v1/subdomains", access, map[string]string{"subdomain": "second"}, nil)
	if r2.StatusCode != http.StatusForbidden {
		t.Fatalf("second reserve should hit plan limit (403), got %d", r2.StatusCode)
	}
}

func TestMeWithAPIKey(t *testing.T) {
	ts := testServer(t)
	_, apiKey := signupAndKey(t, ts, "me@example.com")

	req, _ := http.NewRequest("GET", ts.URL+"/v1/me", nil)
	req.Header.Set("Authorization", "Bearer "+apiKey)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("GET /v1/me: %v", err)
	}
	b, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("/v1/me status %d: %s", resp.StatusCode, b)
	}
	var me struct {
		User struct {
			Email string `json:"email"`
		} `json:"user"`
		Plan string `json:"plan"`
		Org  struct {
			ID string `json:"id"`
		} `json:"org"`
	}
	_ = json.Unmarshal(b, &me)
	// The profile for an API-key principal resolves to the org owner.
	if me.User.Email != "me@example.com" || me.Plan != "free" || me.Org.ID == "" {
		t.Fatalf("unexpected /v1/me for api key: %+v (%s)", me, b)
	}
}

// TestAdminGrantAndRevoke proves the approve.<base> flow: an admin logs in, grants
// a plan by email (with an expiry), the customer's key then authenticates as that
// plan, and revoke returns them to Free. Also checks the endpoints require auth.
func TestAdminGrantAndRevoke(t *testing.T) {
	cfg := api.DefaultConfig()
	cfg.DevAuth = true
	cfg.AdminUser = "root"
	cfg.AdminPassword = "s3cret-pass"
	srv, err := api.New(cfg, slog.New(slog.NewTextHandler(io.Discard, nil)))
	if err != nil {
		t.Fatalf("api.New: %v", err)
	}
	ts := httptest.NewServer(srv.Router())
	t.Cleanup(ts.Close)

	_, apiKey := signupAndKey(t, ts, "cust@example.com")

	jar, _ := cookiejar.New(nil)
	client := &http.Client{Jar: jar}
	req := func(method, url string, body, out any) *http.Response {
		var r io.Reader
		if body != nil {
			buf, _ := json.Marshal(body)
			r = bytes.NewReader(buf)
		}
		rq, _ := http.NewRequest(method, url, r)
		if body != nil {
			rq.Header.Set("Content-Type", "application/json")
		}
		resp, e := client.Do(rq)
		if e != nil {
			t.Fatalf("%s %s: %v", method, url, e)
		}
		if out != nil {
			b, _ := io.ReadAll(resp.Body)
			resp.Body.Close()
			_ = json.Unmarshal(b, out)
		}
		return resp
	}

	// Grant without a session is rejected.
	if r := req("POST", ts.URL+"/v1/admin/grant", map[string]any{"email": "cust@example.com", "plan": "pro", "months": 1}, nil); r.StatusCode != http.StatusUnauthorized {
		t.Fatalf("grant without admin session should be 401, got %d", r.StatusCode)
	}

	// Admin login sets the session cookie in the jar.
	if r := req("POST", ts.URL+"/admin/login", map[string]string{"username": "root", "password": "s3cret-pass"}, nil); r.StatusCode != http.StatusOK {
		t.Fatalf("admin login: %d", r.StatusCode)
	}

	// Grant Pro for a month.
	var granted struct {
		Plan string  `json:"plan"`
		Exp  *string `json:"plan_expires_at"`
	}
	if r := req("POST", ts.URL+"/v1/admin/grant", map[string]any{"email": "cust@example.com", "plan": "pro", "months": 1}, &granted); r.StatusCode != http.StatusOK {
		t.Fatalf("grant: %d", r.StatusCode)
	}
	if granted.Plan != "pro" || granted.Exp == nil {
		t.Fatalf("grant did not apply: plan=%s exp=%v", granted.Plan, granted.Exp)
	}

	// The customer's key now authenticates as Pro (UDP allowed) over the edge RPC.
	rpc := entitlerpc.NewClient(ts.URL, api.DefaultConfig().InternalToken)
	if dec, _ := rpc.CheckBind(context.Background(), authz.BindRequest{APIKey: apiKey, Type: "udp"}); !dec.Allow {
		t.Fatalf("granted pro should allow UDP: %s", dec.ErrorCode)
	}

	// Revoke → back to Free (UDP denied).
	req("POST", ts.URL+"/v1/admin/revoke", map[string]string{"email": "cust@example.com"}, nil).Body.Close()
	if dec, _ := rpc.CheckBind(context.Background(), authz.BindRequest{APIKey: apiKey, Type: "udp"}); dec.Allow {
		t.Fatal("revoked plan should deny UDP")
	}
}

func TestDeviceFlow(t *testing.T) {
	ts := testServer(t)
	access, _ := signupAndKey(t, ts, "d@example.com")

	// 1) CLI requests a device code.
	var code struct {
		DeviceCode string `json:"device_code"`
		UserCode   string `json:"user_code"`
	}
	postJSON(t, ts.URL+"/v1/auth/device/code", "", nil, &code)
	if code.DeviceCode == "" || code.UserCode == "" {
		t.Fatal("device code missing")
	}

	// 2) Browser (authed) approves the user code.
	rApprove := postJSON(t, ts.URL+"/v1/auth/device/approve", access, map[string]string{"user_code": code.UserCode}, nil)
	if rApprove.StatusCode != http.StatusOK {
		t.Fatalf("approve: status %d", rApprove.StatusCode)
	}

	// 3) CLI polls and receives an API key.
	var tok struct {
		APIKey string `json:"api_key"`
	}
	postJSON(t, ts.URL+"/v1/auth/device/token", "", map[string]string{"device_code": code.DeviceCode}, &tok)
	if !strings.HasPrefix(tok.APIKey, "tq_live_") {
		t.Fatalf("device token did not return an api key: %q", tok.APIKey)
	}
}
