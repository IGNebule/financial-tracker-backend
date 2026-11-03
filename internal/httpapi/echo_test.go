package httpapi

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func postEcho(t *testing.T, body string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, "/v1/echo", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	NewRouter(Deps{}).ServeHTTP(rec, req)
	return rec
}

func TestEchoSuccess(t *testing.T) {
	rec := postEcho(t, `{"amount_minor":1234,"currency":"EUR"}`)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body = %s", rec.Code, rec.Body.String())
	}

	var resp echoResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if want := "12.34 EUR"; resp.Formatted != want {
		t.Fatalf("formatted = %q, want %q", resp.Formatted, want)
	}
}

func TestEchoUnsupportedCurrency(t *testing.T) {
	rec := postEcho(t, `{"amount_minor":1234,"currency":"XYZ"}`)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", rec.Code)
	}

	var resp errorResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp.Code != "UNSUPPORTED_CURRENCY" {
		t.Fatalf("code = %q, want UNSUPPORTED_CURRENCY", resp.Code)
	}
	if !strings.Contains(resp.Message, "XYZ") {
		t.Fatalf("message = %q, want it to mention XYZ", resp.Message)
	}
}

func TestEchoMalformedJSON(t *testing.T) {
	rec := postEcho(t, `not json at all`)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", rec.Code)
	}
	var resp errorResponse
	_ = json.NewDecoder(rec.Body).Decode(&resp)
	if resp.Code != "INVALID_JSON" {
		t.Fatalf("code = %q, want INVALID_JSON", resp.Code)
	}
}

func TestEchoRejectsUnknownField(t *testing.T) {
	// amount_minr is a typo for amount_minor. Without DisallowUnknownFields
	// this would silently decode as amount_minor = 0 and succeed.
	rec := postEcho(t, `{"amount_minr":1234,"currency":"EUR"}`)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", rec.Code)
	}
	var resp errorResponse
	_ = json.NewDecoder(rec.Body).Decode(&resp)
	if resp.Code != "INVALID_JSON" {
		t.Fatalf("code = %q, want INVALID_JSON", resp.Code)
	}
}

func TestEchoRejectsEmptyBody(t *testing.T) {
	rec := postEcho(t, ``)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400; body = %s", rec.Code, rec.Body.String())
	}

	var resp errorResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode error response: %v", err)
	}

	if resp.Code != "INVALID_JSON" {
		t.Fatalf("code = %q, want %q", resp.Code, "INVALID_JSON")
	}
}
