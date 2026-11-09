package httpapi

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"financialtracker/internal/users"
)

var fakeUser = &users.User{
	ID:           uuid.MustParse("11111111-1111-1111-1111-111111111111"),
	Email:        "test@example.com",
	PasswordHash: "$2a$10$fake-hash-for-testing-only",
	CreatedAt:    time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
}

func postUser(t *testing.T, createUser func(context.Context, string, string) (*users.User, error), body string) *httptest.ResponseRecorder {
	t.Helper()
	router := NewRouter(Deps{CreateUser: createUser})
	req := httptest.NewRequest(http.MethodPost, "/v1/users", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	return rec
}

func TestCreateUserSuccess(t *testing.T) {
	var gotEmail, gotHash string
	fakeCreate := func(ctx context.Context, email, hash string) (*users.User, error) {
		gotEmail = email
		gotHash = hash
		return &users.User{
			ID:           fakeUser.ID,
			Email:        email,
			PasswordHash: hash,
			CreatedAt:    fakeUser.CreatedAt,
		}, nil
	}

	rec := postUser(t, fakeCreate, `{"email":"  Test@Example.COM  ","password":"correct horse battery staple"}`)
	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, want 201; body = %s", rec.Code, rec.Body.String())
	}
	if gotEmail != "test@example.com" {
		t.Fatalf("createUser received email %q, want %q", gotEmail, "test@example.com")
	}
	if gotHash == "correct horse battery staple" {
		t.Fatal("createUser received the plaintext password; hashing is missing")
	}
	if !strings.HasPrefix(gotHash, "$2") {
		t.Fatalf("createUser received %q, which is not a bcrypt hash", gotHash)
	}

	var resp map[string]any
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if resp["id"] != fakeUser.ID.String() {
		t.Fatalf("response id = %v, want %v", resp["id"], fakeUser.ID.String())
	}
	if resp["email"] != "test@example.com" {
		t.Fatalf("response email = %v, want test@example.com", resp["email"])
	}
	if _, present := resp["password_hash"]; present {
		t.Fatal("response contains password_hash; it must never be returned")
	}
	if _, present := resp["passowrd"]; present {
		t.Fatal("response contains password; it must never be returned")
	}
}

func TestCreateUserDuplicateEmail(t *testing.T) {
	fakeCreate := func(ctx context.Context, email, hash string) (*users.User, error) {
		return nil, users.ErrDuplicateEmail
	}
	rec := postUser(t, fakeCreate, `{"email":"test@example.com","password":"password123"}`)

	if rec.Code != http.StatusConflict {
		t.Fatalf("status = %d, want 409", rec.Code)
	}

	var body errorResponse
	_ = json.NewDecoder(rec.Body).Decode(&body)
	if body.Code != "EMAIL_EXISTS" {
		t.Fatalf("code = %q, want EMAIL_EXISTS", body.Code)
	}
}

func TestCreateUserInvalidEmail(t *testing.T) {
	called := false
	fakeCreate := func(ctx context.Context, email, hash string) (*users.User, error) {
		called = true
		return fakeUser, nil
	}

	rec := postUser(t, fakeCreate, `{"email":"not-an-email","password":"password123"}`)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", rec.Code)
	}
	if called {
		t.Fatal("createUser was called despite invalid email; validation is bypassed")
	}

	var body errorResponse
	_ = json.NewDecoder(rec.Body).Decode(&body)
	if body.Code != "INVALID_EMAIL" {
		t.Fatalf("code = %q, want INVALID_EMAIL", body.Code)
	}
}

func TestCreateUserShortPassword(t *testing.T) {
	called := false
	fakeCreate := func(ctx context.Context, email, hash string) (*users.User, error) {
		called = true
		return fakeUser, nil
	}
	rec := postUser(t, fakeCreate, `{"email":"test@example.com","password":"short"}`)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", rec.Code)
	}
	if called {
		t.Fatal("createUser was called despite short password")
	}
	var body errorResponse
	_ = json.NewDecoder(rec.Body).Decode(&body)
	if body.Code != "INVALID_PASSWORD" {
		t.Fatalf("code = %q, want INVALID_PASSWORD", body.Code)
	}
}

func TestCreateUserTooLongPassword(t *testing.T) {
	// Case A: 72 bytes - Exactly at the bcrypt limit, should succeed (201 Created)
	t.Run("72_bytes_valid", func(t *testing.T) {
		called := false
		fakeCreate := func(ctx context.Context, email, hash string) (*users.User, error) {
			called = true
			return fakeUser, nil
		}

		password72 := strings.Repeat("a", 72)
		rec := postUser(t, fakeCreate, `{"email":"test@example.com","password":"`+password72+`"}`)

		if rec.Code != http.StatusCreated {
			t.Fatalf("status = %d, want 201 for 72-byte password; body = %s", rec.Code, rec.Body.String())
		}
		if !called {
			t.Fatal("createUser was not called for valid 72-byte password")
		}
	})

	// Case B: 73 bytes - Over the bcrypt limit, should be rejected by handler (400 Bad Request)
	t.Run("73_bytes_invalid", func(t *testing.T) {
		called := false
		fakeCreate := func(ctx context.Context, email, hash string) (*users.User, error) {
			called = true
			return fakeUser, nil
		}

		password73 := strings.Repeat("a", 73)
		rec := postUser(t, fakeCreate, `{"email":"test@example.com","password":"`+password73+`"}`)

		if rec.Code != http.StatusBadRequest {
			t.Fatalf("status = %d, want 400 for 73-byte password; body = %s", rec.Code, rec.Body.String())
		}
		if called {
			t.Fatal("createUser was called despite 73-byte password exceeding limit")
		}

		var body errorResponse
		if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
			t.Fatalf("decode response: %v", err)
		}
		if body.Code != "INVALID_PASSWORD" {
			t.Fatalf("code = %q, want INVALID_PASSWORD", body.Code)
		}
	})
}

func TestCreateUserMalformedJSON(t *testing.T) {
	fakeCreate := func(ctx context.Context, email, hash string) (*users.User, error) {
		return fakeUser, nil
	}
	rec := postUser(t, fakeCreate, `not json`)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", rec.Code)
	}
}

func TestIsValidEmail(t *testing.T) {
	cases := []struct {
		name  string
		email string
		want  bool
	}{
		{"simple", "test@example.com", true},
		{"uppercase", "TEST@EXAMPLE.COM", true},
		{"subdomain", "test@mail.example.com", true},
		{"no_at", "test.example.com", false},
		{"empty_local", "@example.com", false},
		{"empty_domain", "test@", false},
		{"empty", "", false},
		{"double_at", "test@@example.com", false},
		{"space_in_local", "test @example.com", false},
		{"too_long", strings.Repeat("a", 250) + "@example.com", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := isValidEmail(tc.email); got != tc.want {
				t.Fatalf("isValidEmail(%q) = %v, want %v", tc.email, got, tc.want)
			}
		})
	}
}
