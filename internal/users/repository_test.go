package users

import (
	"context"
	"errors"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"financialtracker/internal/config"
	"financialtracker/internal/db"
)

func newIntegrationRepo(t *testing.T) *Repository {
	t.Helper()
	if testing.Short() {
		t.Skip("integration test skipped in -short mode")
	}
	if os.Getenv("APP_DB_HOST") == "" {
		t.Skip("APP_DB_HOST not set; run `set -a; . ./.env; set +a` before go test")
	}

	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("config.Load: %v", err)
	}
	database, err := db.Open(context.Background(), cfg.DB)
	if err != nil {
		t.Fatalf("db.Open: %v", err)
	}
	t.Cleanup(database.Close)

	return NewRepository(database.Pool())
}

func uniqueEmail(t *testing.T) string {
	t.Helper()
	return "test-" + uuid.NewString() + "@example.invalid"
}

func TestCreateAndFind(t *testing.T) {
	repo := newIntegrationRepo(t)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	email := uniqueEmail(t)
	created, err := repo.Create(ctx, email, "$2a$10$fixed-fake-hash-for-test")
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if created.ID == uuid.Nil {
		t.Fatal("Create returned nil UUID")
	}
	if created.Email != email {
		t.Fatalf("Email = %q, want %q", created.Email, email)
	}
	if created.CreatedAt.IsZero() {
		t.Fatal("CreatedAt is zero")
	}

	found, err := repo.FindByEmail(ctx, email)
	if err != nil {
		t.Fatalf("FindByEmail: %v", err)
	}
	if found.ID != created.ID {
		t.Fatalf("FindByEmail returned ID %v, want %v", found.ID, created.ID)
	}
	if found.PasswordHash != created.PasswordHash {
		t.Fatalf("PasswordHash mismatch")
	}
}

func TestCreateDuplicateEmail(t *testing.T) {
	repo := newIntegrationRepo(t)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	email := uniqueEmail(t)

	if _, err := repo.Create(ctx, email, "hash1"); err != nil {
		t.Fatalf("first Create: %v", err)
	}

	_, err := repo.Create(ctx, email, "hash2")
	if !errors.Is(err, ErrDuplicateEmail) {
		t.Fatalf("second Create = %v, want ErrDuplicateEmail", err)
	}
}

func TestCreateDuplicateEmailCaseInsensitive(t *testing.T) {
	repo := newIntegrationRepo(t)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	email := uniqueEmail(t)
	upper := strings.ToUpper(email)

	if _, err := repo.Create(ctx, email, "hash1"); err != nil {
		t.Fatalf("first Create: %v", err)
	}

	_, err := repo.Create(ctx, upper, "hash2")
	if !errors.Is(err, ErrDuplicateEmail) {
		t.Fatalf("Create with uppercase duplicate = %v, want ErrDuplicateEmail", err)
	}
}

func TestFindByEmailNotFound(t *testing.T) {
	repo := newIntegrationRepo(t)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_, err := repo.FindByEmail(ctx, "definitely-not-registered-"+uuid.NewString()+"@example.invalid")
	if err == nil {
		t.Fatal("FindByEmail returned nil error for unknown email")
	}
	// Do not check the exact sentinel — pgx.ErrNoRows — because callers should
	// use errors.Is, not ==. The important contract is "non-nil error".
}

func TestCreateEmptyEmail(t *testing.T) {
	repo := newIntegrationRepo(t)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	u, err := repo.Create(ctx, "", "hash")
	if err != nil {
		t.Fatalf("create with empty email failed unexpectedlu: %v", err)
	}
	if u.ID == uuid.Nil {
		t.Fatal("create returned nil UUID for empty email")
	}
	if u.Email != "" {
		t.Fatalf("email = %q, want %q", u.Email, "")
	}
}
