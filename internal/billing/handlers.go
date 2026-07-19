package billing

import (
	"encoding/json"
	"net/http"

	"github.com/rift/rift/internal/api/auth"
)

// orgOf extracts the authenticated caller's org from the request context.
func orgOf(r *http.Request) string {
	p, _ := auth.PrincipalFrom(r.Context())
	return p.OrgID
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}

func decode(w http.ResponseWriter, r *http.Request, v any) bool {
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20)).Decode(v); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return false
	}
	return true
}
