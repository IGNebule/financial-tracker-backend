package sessions

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"

	"financialtracker/internal/auth"
	"financialtracker/internal/config"
	"financialtracker/internal/db"
)

func testRepo(t *testing.T) *Repository {
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
	// Create a user directly via raw SQL — this is a sessions test, not a users test.
	var id uuid.UUID
	err := repo.pool.QueryRow(context.Background(),
		`INSERT INTO users (email, password_hash) VALUES ($1, $2) RETURNING id`,
		"session-test-"+uuid.NewString()+"@example.invalid", "not-real-hash",
	).Scan(&id)
	if err != nil {
		t.Fatalf("insert user: %v", err)
	}
	return id
}

func TestCreateAndFind(t *testing.T) {
	repo := testRepo(t)
	ctx := context.Background()
	userID := testUser(t, repo)

	token, err := repo.Create(ctx, userID)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if len(token) != auth.SessionTokenBytes*2 {
		t.Fatalf("token length = %d, want %d", len(token), auth.SessionTokenBytes*2)
	}

	s, err := repo.FindByToken(ctx, token)
	if err != nil {
		t.Fatalf("FindByToken: %v", err)
	}
	if s.UserID != userID {
		t.Fatalf("UserID = %v, want %v", s.UserID, userID)
	}
	if time.Until(s.ExpiresAt) < DefaultLifetime-time.Minute {
		t.Fatalf("ExpiresAt is too soon: %v", s.ExpiresAt)
	}
}

func TestFindUnknownToken(t *testing.T) {
	repo := testRepo(t)
	_, err := repo.FindByToken(context.Background(),
		"0000000000000000000000000000000000000000000000000000000000000000")
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("FindByToken = %v, want ErrNotFound", err)
	}
}

func TestFindMalformedToken(t *testing.T) {
	repo := testRepo(t)
	_, err := repo.FindByToken(context.Background(), "not-a-token")
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("FindByToken = %v, want ErrNotFound", err)
	}
}

func TestDelete(t *testing.T) {
	repo := testRepo(t)
	ctx := context.Background()
	userID := testUser(t, repo)
	token, _ := repo.Create(ctx, userID)

	if err := repo.Delete(ctx, token); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if _, err := repo.FindByToken(ctx, token); !errors.Is(err, ErrNotFound) {
		t.Fatalf("FindByToken after Delete = %v, want ErrNotFound", err)
	}
}
