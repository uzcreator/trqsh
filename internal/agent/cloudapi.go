package agent

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"time"
)

// This file gives the desktop app its "cloud" surface: the control-plane account
// (profile/plan/expiry/usage), API-key + domain management, and browser (device-
// flow) sign-in. Every call is made server-side by the agent using the stored API
// key, so the WebView never talks to api.trqsh.uz directly (no CORS) and never
// holds the credential. See localapi.go for the route registrations.

var cloudHTTP = &http.Client{Timeout: 20 * time.Second}

// cloudBase returns the configured control-plane API base URL (no trailing slash).
func cloudBase() string {
	base := DefaultAPIBase
	if cfg, err := Load(DefaultConfigPath()); err == nil && strings.TrimSpace(cfg.APIBase) != "" {
		base = cfg.APIBase
	}
	return strings.TrimRight(base, "/")
}

// cloudGET is a convenience for the many read-only proxy routes.
func (l *LocalAPI) cloudGET(upstreamPath string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) { l.cloudProxy(w, r, http.MethodGet, upstreamPath) }
}

// cloudProxy forwards a desktop request to the control-plane API, injecting the
// stored API key as the bearer credential and streaming back the upstream status
// and JSON body.
func (l *LocalAPI) cloudProxy(w http.ResponseWriter, r *http.Request, method, upstreamPath string) {
	key := storedAPIKey()
	if key == "" {
		apiWriteJSON(w, http.StatusUnauthorized, map[string]string{"error": "not signed in"})
		return
	}

	var body io.Reader
	if r.Body != nil {
		b, _ := io.ReadAll(io.LimitReader(r.Body, 1<<20))
		if len(b) > 0 {
			body = bytes.NewReader(b)
		}
	}

	req, err := http.NewRequestWithContext(r.Context(), method, cloudBase()+upstreamPath, body)
	if err != nil {
		apiWriteJSON(w, http.StatusBadGateway, map[string]string{"error": err.Error()})
		return
	}
	req.Header.Set("Authorization", "Bearer "+key)
	req.Header.Set("Accept", "application/json")
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := cloudHTTP.Do(req)
	if err != nil {
		apiWriteJSON(w, http.StatusBadGateway, map[string]string{"error": "cloud unreachable: " + err.Error()})
		return
	}
	defer resp.Body.Close()

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(resp.StatusCode)
	_, _ = io.Copy(w, io.LimitReader(resp.Body, 4<<20))
}

// oauthStart begins a device-authorization flow: it asks the control plane for a
// code pair and returns them to the desktop, which opens verification_uri_complete
// in the browser and then polls.
func (l *LocalAPI) oauthStart(w http.ResponseWriter, r *http.Request) {
	resp, err := cloudHTTP.Post(cloudBase()+"/v1/auth/device/code", "application/json", nil)
	if err != nil {
		apiWriteJSON(w, http.StatusBadGateway, map[string]string{"error": "cloud unreachable: " + err.Error()})
		return
	}
	defer resp.Body.Close()
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(resp.StatusCode)
	_, _ = io.Copy(w, io.LimitReader(resp.Body, 1<<16))
}

// oauthPoll checks whether the browser approval has completed. On success it
// stores the minted API key (same as a manual sign-in) and reports authed; while
// waiting it reports pending; a terminal error (expired/invalid) is surfaced so
// the desktop stops polling.
func (l *LocalAPI) oauthPoll(w http.ResponseWriter, r *http.Request) {
	var in struct {
		DeviceCode string `json:"device_code"`
	}
	_ = json.NewDecoder(io.LimitReader(r.Body, 1<<16)).Decode(&in)
	if strings.TrimSpace(in.DeviceCode) == "" {
		http.Error(w, "missing device_code", http.StatusBadRequest)
		return
	}

	payload, _ := json.Marshal(map[string]string{"device_code": in.DeviceCode})
	resp, err := cloudHTTP.Post(cloudBase()+"/v1/auth/device/token", "application/json", bytes.NewReader(payload))
	if err != nil {
		apiWriteJSON(w, http.StatusBadGateway, map[string]string{"error": "cloud unreachable: " + err.Error()})
		return
	}
	defer resp.Body.Close()

	var tok struct {
		APIKey string `json:"api_key"`
		Error  string `json:"error"`
	}
	raw, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<16))
	_ = json.Unmarshal(raw, &tok)

	switch {
	case tok.APIKey != "":
		if err := l.core.Login(r.Context(), tok.APIKey); err != nil {
			apiWriteJSON(w, http.StatusBadGateway, map[string]string{"error": err.Error()})
			return
		}
		apiWriteJSON(w, http.StatusOK, map[string]any{"authed": true})
	case tok.Error == "authorization_pending" || tok.Error == "slow_down":
		apiWriteJSON(w, http.StatusOK, map[string]any{"pending": true})
	default:
		msg := tok.Error
		if msg == "" {
			msg = "authorization failed"
		}
		apiWriteJSON(w, http.StatusOK, map[string]any{"error": msg})
	}
}
