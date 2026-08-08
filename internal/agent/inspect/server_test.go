package inspect

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// The inspector serves captured request/response bodies (cookies, tokens,
// secrets in the developer's own traffic) with no auth, relying on the 127.0.0.1
// bind. A Host guard is what actually stops a malicious web page from reaching it
// via DNS rebinding, so it gets a regression test.
func TestInspectorRejectsNonLoopbackHost(t *testing.T) {
	h := NewServer(NewRecorder(8)).Handler()

	// DNS-rebinding: the attacker's domain (rebased to 127.0.0.1) rides in Host.
	req := httptest.NewRequest(http.MethodGet, "http://attacker.example/api/requests", nil)
	req.Host = "attacker.example"
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("non-loopback Host: got %d, want 403", rec.Code)
	}

	// A genuine loopback request is still served.
	req2 := httptest.NewRequest(http.MethodGet, "http://127.0.0.1:4040/api/requests", nil)
	req2.Host = "127.0.0.1:4040"
	rec2 := httptest.NewRecorder()
	h.ServeHTTP(rec2, req2)
	if rec2.Code != http.StatusOK {
		t.Fatalf("loopback Host: got %d, want 200", rec2.Code)
	}
}

func TestIsLoopbackHost(t *testing.T) {
	cases := map[string]bool{
		"127.0.0.1:4040":   true,
		"localhost:4040":   true,
		"[::1]:4040":       true,
		"127.0.0.1":        true,
		"attacker.example": false,
		"10.0.0.5:4040":    false,
		"0.0.0.0:4040":     false,
		"":                 false,
	}
	for host, want := range cases {
		if got := isLoopbackHost(host); got != want {
			t.Errorf("isLoopbackHost(%q) = %v, want %v", host, got, want)
		}
	}
}

// The captured method/path/headers/bodies are attacker-controlled (anyone hitting
// the public tunnel picks them), so the UI must HTML-escape them before writing
// innerHTML — otherwise it's a stored-XSS sink firing in the developer's browser.
func TestInspectorUIEscapesCapturedData(t *testing.T) {
	if !strings.Contains(indexHTML, "function esc(") {
		t.Fatal("inspector UI is missing the esc() HTML-escaper")
	}
	// The raw, unescaped interpolations that were the original XSS sinks must be
	// gone: every dynamic value now flows through esc(...).
	for _, bad := range []string{"'+r.method+'", "'+r.path+'", "'+b64(r.req_body)+'", "'+b64(r.resp_body)+'"} {
		if strings.Contains(indexHTML, bad) {
			t.Errorf("inspector UI interpolates %q without esc()", bad)
		}
	}
}
