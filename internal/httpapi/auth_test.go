package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"

	"financialtracker/internal/sessions"
	"financialtracker/internal/users"
)

// fakeStoredUser is the fixed user returned by the fake findUser in tests.
// The hash is a real bcrypt hash of "correct-password-123".
func fakeStoredUser(t *testing.T) *users.User {
	t.Helper()
	hash, err := bcrypt.GenerateFromPassword([]byte("correct-password-123"), bcrypt.DefaultCost)
	if err != nil {
		t.Fatalf("bcrypt: %v", err)
	}
	return &users.User{
		ID:           uuid.MustParse("aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa"),
		Email:        "test@example.com",
		PasswordHash: string(hash),
		CreatedAt:    time.Now(),
	}
}

func postLogin(t *testing.T, findUser func(context.Context, string) (*users.User, error), createSession func(context.Context, uuid.UUID) (string, error), body string) *httptest.ResponseRecorder {
	t.Helper()
	router := NewRouter(Deps{
		LoginFindUser:    findUser,
		LoginCreateToken: createSession,
	})
	req := httptest.NewRequest(http.MethodPost, "/v1/login", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	return rec
}

func TestLoginSuccess(t *testing.T) {
	u := fakeStoredUser(t)
	find := func(ctx context.Context, email string) (*users.User, error) {
		if email != "test@example.com" {
			t.Fatalf("find received %q, want test@example.com", email)
		}
		return u, nil
	}
	create := func(ctx context.Context, uid uuid.UUID) (string, error) {
		if uid != u.ID {
			t.Fatalf("createSession received user %v, want %v", uid, u.ID)
		}
		return "fake-token-abc123", nil
	}

	rec := postLogin(t, find, create, `{"email":"  Test@Example.COM  ","password":"correct-password-123"}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body = %s", rec.Code, rec.Body.String())
	}

	var resp loginResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp.Token != "fake-token-abc123" {
		t.Fatalf("token = %q, want fake-token-abc123", resp.Token)
	}
	if resp.ExpiresAt.Before(time.Now()) {
		t.Fatal("ExpiresAt is in the past")
	}
}

func TestLoginWrongPassword(t *testing.T) {
	u := fakeStoredUser(t)
	find := func(context.Context, string) (*users.User, error) { return u, nil }
	create := func(context.Context, uuid.UUID) (string, error) {
		t.Fatal("createSession should not be called when password is wrong")
		return "", nil
	}

	rec := postLogin(t, find, create, `{"email":"test@example.com","password":"wrong"}`)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", rec.Code)
	}
	var resp errorResponse
	_ = json.NewDecoder(rec.Body).Decode(&resp)
	if resp.Code != "INVALID_CREDENTIALS" {
		t.Fatalf("code = %q, want INVALID_CREDENTIALS", resp.Code)
	}
}

func TestLoginUnknownEmailSameResponse(t *testing.T) {
	// Important: unknown email must NOT be distinguishable from wrong
	// password. Both must return the same status and the same code.
	find := func(context.Context, string) (*users.User, error) {
		return nil, errors.New("not found")
	}
	create := func(context.Context, uuid.UUID) (string, error) {
		t.Fatal("createSession should not be called for unknown email")
		return "", nil
	}

	rec := postLogin(t, find, create, `{"email":"nobody@example.com","password":"whatever"}`)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", rec.Code)
	}
	var resp errorResponse
	_ = json.NewDecoder(rec.Body).Decode(&resp)
	if resp.Code != "INVALID_CREDENTIALS" {
		t.Fatalf("code = %q, want INVALID_CREDENTIALS", resp.Code)
	}
}

func TestRequireAuthMissingHeader(t *testing.T) {
	called := false
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
	})
	handler := requireAuth(func(context.Context, string) (*sessions.Session, error) {
		t.Fatal("findSession should not be called without a token")
		return nil, nil
	})(next)

	req := httptest.NewRequest(http.MethodGet, "/protected", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", rec.Code)
	}
	if called {
		t.Fatal("next handler was called without a token — middleware is broken")
	}
}

func TestRequireAuthValidToken(t *testing.T) {
	expectedUser := uuid.MustParse("bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb")
	called := false
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		got, ok := UserIDFromContext(r.Context())
		if !ok {
			t.Fatal("UserIDFromContext returned false")
		}
		if got != expectedUser {
			t.Fatalf("UserID = %v, want %v", got, expectedUser)
		}
	})
	handler := requireAuth(func(ctx context.Context, tok string) (*sessions.Session, error) {
		if tok != "good-token" {
			t.Fatalf("findSession received %q, want good-token", tok)
		}
		return &sessions.Session{UserID: expectedUser}, nil
	})(next)

	req := httptest.NewRequest(http.MethodGet, "/protected", nil)
	req.Header.Set("Authorization", "Bearer good-token")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if !called {
		t.Fatal("next handler was not called despite valid token")
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
}

func TestRequireAuthInvalidToken(t *testing.T) {
	called := false
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { called = true })
	handler := requireAuth(func(context.Context, string) (*sessions.Session, error) {
		return nil, sessions.ErrNotFound
	})(next)

	req := httptest.NewRequest(http.MethodGet, "/protected", nil)
	req.Header.Set("Authorization", "Bearer wrong-token")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", rec.Code)
	}
	if called {
		t.Fatal("next handler was called with an invalid token")
	}
}
