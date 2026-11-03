package httpapi

import (
	"context"
	"encoding/json"
	"net/http"
	"time"
)

type readyResponse struct {
	Status string `json:"status"`
	DB     string `json:"db"`
}

// handleReady returns a handler that reports service readiness. It pings the
// database; a failure returns 503 with a generic error body. The underlying
// error is deliberately not echoed to the client — a readiness endpoint is
// often reachable without authentication, and internal details (hostnames,
// credentials in error text) must not leak.
func handleReady(pingDB func(context.Context) error) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
		defer cancel()

		if err := pingDB(ctx); err != nil {
			writeError(w, http.StatusServiceUnavailable, "DB_UNAVAILABLE", "database not reachable")
			return
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(readyResponse{Status: "ok", DB: "up"})
	}
}
