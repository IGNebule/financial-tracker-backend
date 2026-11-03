package httpapi

import (
	"encoding/json"
	"net/http"

	"financialtracker/internal/money"
)

const maxEchoBodyBytes = 1 << 20

type echoRequest struct {
	AmountMinor int64  `json:"amount_minor"`
	Currency    string `json:"currency"`
}

type echoResponse struct {
	Formatted string `json:"formatted"`
}

func handleEcho(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, maxEchoBodyBytes)

	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()

	var req echoRequest
	if err := dec.Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "INVALID_JSON", "request body must be valid JSON with fields amount_minor and currency")
		return
	}

	formatted, err := money.Format(req.AmountMinor, req.Currency)
	if err != nil {
		writeError(w, http.StatusBadRequest, "UNSUPPORTED_CURRENCY", err.Error())
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(echoResponse{Formatted: formatted})
}
