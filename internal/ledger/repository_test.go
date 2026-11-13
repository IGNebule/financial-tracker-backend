package ledger

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"

	"financialtracker/internal/config"
	"financialtracker/internal/db"
)

func testPool(t *testing.T) *Repository {
	t.Helper()
	if testing.Short() {
		t.Skip("integration test skipped in -short mode")
	}
	cfg, err := config.Load()
	if err != nil {
		t.Skipf("config unavailable: %v", err)
	}
	database, err := db.Open(context.Background(), cfg.DB)
	if err != nil {
		t.Skipf("db unavailable: %v", err)
	}
	t.Cleanup(database.Close)
	return NewRepository(database.Pool())
}

func testUser(t *testing.T, repo *Repository) uuid.UUID {
	t.Helper()
	var id uuid.UUID
	err := repo.pool.QueryRow(context.Background(),
		`INSERT INTO users (email, password_hash) VALUES ($1, $2) RETURNING id`,
		"ledger-account-test-"+uuid.NewString()+"@example.invalid", "not-real-hash",
	).Scan(&id)
	if err != nil {
		t.Fatalf("insert user: %v", err)
	}
	return id
}

func TestCreateAccount(t *testing.T) {
	repo := testPool(t)
	userID := testUser(t, repo)
	ctx := context.Background()

	a, err := repo.CreateAccount(ctx, userID, "Checking", AccountTypeAsset, "EUR")
	if err != nil {
		t.Fatalf("CreateAccount: %v", err)
	}
	if a.ID == uuid.Nil {
		t.Fatal("ID is nil")
	}
	if a.UserID != userID {
		t.Fatalf("UserID = %v, want %v", a.UserID, userID)
	}
	if a.Name != "Checking" || a.Type != AccountTypeAsset || a.Currency != "EUR" {
		t.Fatalf("account = %+v", a)
	}
}

func TestCreateAccountDuplicateName(t *testing.T) {
	repo := testPool(t)
	userID := testUser(t, repo)
	ctx := context.Background()

	if _, err := repo.CreateAccount(ctx, userID, "Checking", AccountTypeAsset, "EUR"); err != nil {
		t.Fatalf("first CreateAccount: %v", err)
	}
	_, err := repo.CreateAccount(ctx, userID, "Checking", AccountTypeAsset, "EUR")
	if !errors.Is(err, ErrDuplicateAccountName) {
		t.Fatalf("second CreateAccount = %v, want ErrDuplicateAccountName", err)
	}
}

func TestCreateAccountSameNameDifferentUsers(t *testing.T) {
	repo := testPool(t)
	u1 := testUser(t, repo)
	u2 := testUser(t, repo)
	ctx := context.Background()

	if _, err := repo.CreateAccount(ctx, u1, "Checking", AccountTypeAsset, "EUR"); err != nil {
		t.Fatalf("u1 CreateAccount: %v", err)
	}
	if _, err := repo.CreateAccount(ctx, u2, "Checking", AccountTypeAsset, "EUR"); err != nil {
		t.Fatalf("u2 CreateAccount same name: %v (UNIQUE should be per-user)", err)
	}
}

func TestListAccountsByUser(t *testing.T) {
	repo := testPool(t)
	userID := testUser(t, repo)
	ctx := context.Background()

	if _, err := repo.CreateAccount(ctx, userID, "B", AccountTypeAsset, "EUR"); err != nil {
		t.Fatalf("create B: %v", err)
	}
	if _, err := repo.CreateAccount(ctx, userID, "A", AccountTypeAsset, "EUR"); err != nil {
		t.Fatalf("create A: %v", err)
	}

	list, err := repo.ListAccountsByUser(ctx, userID)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(list) != 2 {
		t.Fatalf("len = %d, want 2", len(list))
	}
	// ORDER BY name: "A" before "B"
	if list[0].Name != "A" || list[1].Name != "B" {
		t.Fatalf("order = %q, %q; want A, B", list[0].Name, list[1].Name)
	}
}

func TestListAccountsIsolatedByUser(t *testing.T) {
	repo := testPool(t)
	u1 := testUser(t, repo)
	u2 := testUser(t, repo)
	ctx := context.Background()

	if _, err := repo.CreateAccount(ctx, u1, "u1-account", AccountTypeAsset, "EUR"); err != nil {
		t.Fatalf("u1 create: %v", err)
	}

	list, err := repo.ListAccountsByUser(ctx, u2)
	if err != nil {
		t.Fatalf("u2 list: %v", err)
	}
	if len(list) != 0 {
		t.Fatalf("u2 sees %d accounts, want 0 — isolation broken", len(list))
	}
}

func TestListAccountsEmptyReturnsEmptySlice(t *testing.T) {
	repo := testPool(t)
	userID := testUser(t, repo)
	list, err := repo.ListAccountsByUser(context.Background(), userID)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if list == nil {
		t.Fatal("list is nil; want non-nil empty slice (JSON must be [], not null)")
	}
	if len(list) != 0 {
		t.Fatalf("len = %d, want 0", len(list))
	}
}
