package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestReadyzDBUp(t *testing.T) {
	router := NewRouter(Deps{PingDB: func(context.Context) error { return nil }})

	req := httptest.NewRequest(http.MethodGet, "/readyz", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body = %s", rec.Code, rec.Body.String())
	}
	var body readyResponse
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if body.Status != "ok" || body.DB != "up" {
		t.Fatalf("body = %+v, want status=ok db=up", body)
	}
}

func TestReadyzDBDown(t *testing.T) {
	router := NewRouter(Deps{PingDB: func(context.Context) error {
		return errors.New("connection refused")
	}})

	req := httptest.NewRequest(http.MethodGet, "/readyz", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want 503", rec.Code)
	}
	var body errorResponse
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if body.Code != "DB_UNAVAILABLE" {
		t.Fatalf("code = %q, want DB_UNAVAILABLE", body.Code)
	}
}

func TestReadyzRespectsContext(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping ~2s test in -short mode")
	}

	// A PingDB that blocks until the context fires. This simulates a hung
	// database and is the only way to prove the 2-second deadline is real.
	blockingPing := func(ctx context.Context) error {
		<-ctx.Done()
		return ctx.Err()
	}

	router := NewRouter(Deps{PingDB: blockingPing})

	req := httptest.NewRequest(http.MethodGet, "/readyz", nil)
	rec := httptest.NewRecorder()

	start := time.Now()
	router.ServeHTTP(rec, req)
	elapsed := time.Since(start)

	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want 503; body = %s", rec.Code, rec.Body.String())
	}
	if elapsed < 1900*time.Millisecond {
		t.Fatalf("elapsed = %v, want >= 1.9s (timeout too short or missing)", elapsed)
	}
	if elapsed > 2500*time.Millisecond {
		t.Fatalf("elapsed = %v, want <= 2.5s (timeout too long)", elapsed)
	}

	var body errorResponse
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if body.Code != "DB_UNAVAILABLE" {
		t.Fatalf("code = %q, want DB_UNAVAILABLE", body.Code)
	}
}
