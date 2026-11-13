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

	"financialtracker/internal/ledger"
	"financialtracker/internal/sessions"
)

// fixedUser is the authenticated user for protected-route tests. The auth
// middleware accepts any token and puts this ID in context.
var (
	fixedUser = uuid.MustParse("cccccccc-cccc-cccc-cccc-cccccccccccc")
	fixedTok  = "good-token"
)

// protectedRouter wires fake deps for a protected-route test. The fake
// FindSession always accepts fixedTok and returns fixedUser.
func protectedRouter(t *testing.T, createAcct func(context.Context, uuid.UUID, string, ledger.AccountType, string) (*ledger.Account, error), listAcct func(context.Context, uuid.UUID) ([]*ledger.Account, error)) http.Handler {
	t.Helper()
	return NewRouter(Deps{
		FindSession: func(ctx context.Context, tok string) (*sessions.Session, error) {
			if tok != fixedTok {
				return nil, sessions.ErrNotFound
			}
			return &sessions.Session{UserID: fixedUser}, nil
		},
		CreateAccount: createAcct,
		ListAccounts:  listAcct,
	})
}

func doProtected(t *testing.T, router http.Handler, method, path, body string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+fixedTok)
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	return rec
}

func TestCreateAccountSuccess(t *testing.T) {
	var gotUser uuid.UUID
	var gotName string
	var gotType ledger.AccountType
	var gotCur string

	create := func(ctx context.Context, uid uuid.UUID, name string, typ ledger.AccountType, cur string) (*ledger.Account, error) {
		gotUser, gotName, gotType, gotCur = uid, name, typ, cur
		return &ledger.Account{
			ID:        uuid.MustParse("dddddddd-dddd-dddd-dddd-dddddddddddd"),
			UserID:    uid,
			Name:      name,
			Type:      typ,
			Currency:  cur,
			CreatedAt: time.Now(),
		}, nil
	}
	router := protectedRouter(t, create, nil)

	rec := doProtected(t, router, http.MethodPost, "/v1/accounts",
		`{"name":"  Checking  ","type":"Asset","currency":"eur"}`)

	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, want 201; body = %s", rec.Code, rec.Body.String())
	}
	if gotUser != fixedUser {
		t.Fatalf("create received user %v, want %v", gotUser, fixedUser)
	}
	if gotName != "Checking" {
		t.Fatalf("name = %q, want Checking (trimmed)", gotName)
	}
	if gotType != ledger.AccountTypeAsset {
		t.Fatalf("type = %q, want asset (lowercased)", gotType)
	}
	if gotCur != "EUR" {
		t.Fatalf("currency = %q, want EUR (uppercased)", gotCur)
	}
}

func TestCreateAccountUnauthenticated(t *testing.T) {
	create := func(context.Context, uuid.UUID, string, ledger.AccountType, string) (*ledger.Account, error) {
		t.Fatal("createAccount should not be called without auth")
		return nil, nil
	}
	router := protectedRouter(t, create, nil)

	req := httptest.NewRequest(http.MethodPost, "/v1/accounts", strings.NewReader(`{"name":"x","type":"asset","currency":"EUR"}`))
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", rec.Code)
	}
}

func TestCreateAccountDuplicate(t *testing.T) {
	create := func(context.Context, uuid.UUID, string, ledger.AccountType, string) (*ledger.Account, error) {
		return nil, ledger.ErrDuplicateAccountName
	}
	router := protectedRouter(t, create, nil)

	rec := doProtected(t, router, http.MethodPost, "/v1/accounts",
		`{"name":"Checking","type":"asset","currency":"EUR"}`)

	if rec.Code != http.StatusConflict {
		t.Fatalf("status = %d, want 409", rec.Code)
	}
	var body errorResponse
	_ = json.NewDecoder(rec.Body).Decode(&body)
	if body.Code != "ACCOUNT_NAME_EXISTS" {
		t.Fatalf("code = %q, want ACCOUNT_NAME_EXISTS", body.Code)
	}
}

func TestCreateAccountInvalidType(t *testing.T) {
	create := func(context.Context, uuid.UUID, string, ledger.AccountType, string) (*ledger.Account, error) {
		t.Fatal("createAccount should not be called with invalid type")
		return nil, nil
	}
	router := protectedRouter(t, create, nil)

	rec := doProtected(t, router, http.MethodPost, "/v1/accounts",
		`{"name":"X","type":"bogus","currency":"EUR"}`)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", rec.Code)
	}
}

func TestCreateAccountUnsupportedCurrency(t *testing.T) {
	create := func(context.Context, uuid.UUID, string, ledger.AccountType, string) (*ledger.Account, error) {
		t.Fatal("createAccount should not be called with unsupported currency")
		return nil, nil
	}
	router := protectedRouter(t, create, nil)

	rec := doProtected(t, router, http.MethodPost, "/v1/accounts",
		`{"name":"X","type":"asset","currency":"XYZ"}`)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", rec.Code)
	}
}

func TestListAccountsEmpty(t *testing.T) {
	list := func(context.Context, uuid.UUID) ([]*ledger.Account, error) {
		return []*ledger.Account{}, nil
	}
	router := protectedRouter(t, nil, list)

	rec := doProtected(t, router, http.MethodGet, "/v1/accounts", "")

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	body := strings.TrimSpace(rec.Body.String())
	if body != "[]" {
		t.Fatalf("body = %q, want []", body)
	}
}

func TestListAccountsNonEmpty(t *testing.T) {
	list := func(_ context.Context, uid uuid.UUID) ([]*ledger.Account, error) {
		return []*ledger.Account{
			{ID: uuid.MustParse("eeeeeeee-eeee-eeee-eeee-eeeeeeeeeeee"), UserID: uid, Name: "A", Type: ledger.AccountTypeAsset, Currency: "EUR", CreatedAt: time.Now()},
			{ID: uuid.MustParse("ffffffff-ffff-ffff-ffff-ffffffffffff"), UserID: uid, Name: "B", Type: ledger.AccountTypeExpense, Currency: "EUR", CreatedAt: time.Now()},
		}, nil
	}
	router := protectedRouter(t, nil, list)

	rec := doProtected(t, router, http.MethodGet, "/v1/accounts", "")

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	var resp []accountResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(resp) != 2 || resp[0].Name != "A" || resp[1].Name != "B" {
		t.Fatalf("resp = %+v", resp)
	}
}

func TestListAccountsPassesAuthUser(t *testing.T) {
	var got uuid.UUID
	list := func(_ context.Context, uid uuid.UUID) ([]*ledger.Account, error) {
		got = uid
		return nil, nil
	}
	router := protectedRouter(t, nil, list)
	_ = doProtected(t, router, http.MethodGet, "/v1/accounts", "")

	if got != fixedUser {
		t.Fatalf("listAccounts received %v, want %v", got, fixedUser)
	}
}

// ensure imported errors is used by at least one test — kept for future
// extensions that assert against a specific error type.
var _ = errors.Is
