package db

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"financialtracker/internal/config"
)

func mustTestPool(t *testing.T) *pgxpool.Pool {
	t.Helper()
	if testing.Short() {
		t.Skip("integration test skipped in -short mode")
	}
	cfg, err := config.Load()
	if err != nil {
		t.Skipf("config unavailable: %v", err)
	}
	database, err := Open(context.Background(), cfg.DB)
	if err != nil {
		t.Skipf("database unavailable: %v", err)
	}
	t.Cleanup(database.Close)
	return database.Pool()
}

func makeUser(t *testing.T, pool *pgxpool.Pool) uuid.UUID {
	t.Helper()
	email := "ledger-test-" + uuid.NewString() + "@example.invalid"
	var id uuid.UUID
	err := pool.QueryRow(context.Background(),
		`INSERT INTO users (email, password_hash) VALUES ($1, $2) RETURNING id`,
		email, "not-a-real-hash",
	).Scan(&id)
	if err != nil {
		t.Fatalf("insert user: %v", err)
	}
	return id
}

func makeAccount(t *testing.T, pool *pgxpool.Pool, userID uuid.UUID, name, typ, currency string) uuid.UUID {
	t.Helper()
	var id uuid.UUID
	err := pool.QueryRow(context.Background(),
		`INSERT INTO accounts (user_id, name, type, currency) VALUES ($1, $2, $3, $4) RETURNING id`,
		userID, name, typ, currency,
	).Scan(&id)
	if err != nil {
		t.Fatalf("insert account %q: %v", name, err)
	}
	return id
}

func tryPostBalanced(t *testing.T, pool *pgxpool.Pool, userID, debitAcct, creditAcct uuid.UUID, amount int64, currency string) error {
	t.Helper()
	ctx := context.Background()
	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatalf("begin: %v", err)
	}
	defer tx.Rollback(ctx)

	var txID uuid.UUID
	if err := tx.QueryRow(ctx,
		`INSERT INTO journal_transactions (user_id, description, occurred_at)
		 VALUES ($1, $2, $3) RETURNING id`,
		userID, "test", time.Now(),
	).Scan(&txID); err != nil {
		t.Fatalf("insert journal_transaction: %v", err)
	}

	if _, err := tx.Exec(ctx,
		`INSERT INTO postings (transaction_id, account_id, amount_minor, currency)
		 VALUES ($1, $2, $3, $4), ($1, $5, $6, $4)`,
		txID, debitAcct, amount, currency, creditAcct, -amount,
	); err != nil {
		t.Fatalf("insert postings: %v", err)
	}

	return tx.Commit(ctx)
}

func TestBalancedTransactionCommits(t *testing.T) {
	pool := mustTestPool(t)
	userID := makeUser(t, pool)
	checking := makeAccount(t, pool, userID, "checking", "asset", "EUR")
	groceries := makeAccount(t, pool, userID, "groceries", "expense", "EUR")

	if err := tryPostBalanced(t, pool, userID, groceries, checking, 5000, "EUR"); err != nil {
		t.Fatalf("balanced transaction failed: %v", err)
	}
}

func TestUnbalancedTransactionRejected(t *testing.T) {
	pool := mustTestPool(t)
	userID := makeUser(t, pool)
	checking := makeAccount(t, pool, userID, "checking", "asset", "EUR")
	groceries := makeAccount(t, pool, userID, "groceries", "expense", "EUR")

	ctx := context.Background()
	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatalf("begin: %v", err)
	}
	defer tx.Rollback(ctx)

	var txID uuid.UUID
	if err := tx.QueryRow(ctx,
		`INSERT INTO journal_transactions (user_id, description, occurred_at)
		 VALUES ($1, $2, $3) RETURNING id`,
		userID, "unbalanced", time.Now(),
	).Scan(&txID); err != nil {
		t.Fatalf("insert journal_transaction: %v", err)
	}

	if _, err := tx.Exec(ctx,
		`INSERT INTO postings (transaction_id, account_id, amount_minor, currency)
		 VALUES ($1, $2, 5000, 'EUR'), ($1, $3, -4000, 'EUR')`,
		txID, groceries, checking,
	); err != nil {
		t.Fatalf("insert postings: %v", err)
	}

	commitErr := tx.Commit(ctx)
	if commitErr == nil {
		t.Fatal("unbalanced transaction committed successfully; trigger did not fire")
	}
	if !strings.Contains(commitErr.Error(), "does not balance") {
		t.Fatalf("commit error = %v, want message containing 'does not balance'", commitErr)
	}

	// Confirm the rollback: the transaction must not exist.
	var count int
	if err := pool.QueryRow(ctx,
		`SELECT COUNT(*) FROM journal_transactions WHERE id = $1`, txID,
	).Scan(&count); err != nil {
		t.Fatalf("count check: %v", err)
	}
	if count != 0 {
		t.Fatalf("transaction %v was persisted despite failed commit", txID)
	}
}

func TestEmptyTransactionRejected(t *testing.T) {
	pool := mustTestPool(t)
	userID := makeUser(t, pool)

	ctx := context.Background()
	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatalf("begin: %v", err)
	}
	defer tx.Rollback(ctx)

	if _, err := tx.Exec(ctx,
		`INSERT INTO journal_transactions (user_id, description, occurred_at)
		 VALUES ($1, $2, $3)`,
		userID, "empty", time.Now(),
	); err != nil {
		t.Fatalf("insert journal_transaction: %v", err)
	}

	commitErr := tx.Commit(ctx)
	if commitErr == nil {
		t.Fatal("empty transaction committed successfully; trigger did not fire")
	}
	if !strings.Contains(commitErr.Error(), "minimum is 2") {
		t.Fatalf("commit error = %v, want message mentioning minimum is 2", commitErr)
	}
}

func TestSinglePostingRejected(t *testing.T) {
	pool := mustTestPool(t)
	userID := makeUser(t, pool)
	checking := makeAccount(t, pool, userID, "checking", "asset", "EUR")

	ctx := context.Background()
	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatalf("begin: %v", err)
	}
	defer tx.Rollback(ctx)

	var txID uuid.UUID
	if err := tx.QueryRow(ctx,
		`INSERT INTO journal_transactions (user_id, description, occurred_at)
		 VALUES ($1, $2, $3) RETURNING id`,
		userID, "single", time.Now(),
	).Scan(&txID); err != nil {
		t.Fatalf("insert journal_transaction: %v", err)
	}

	if _, err := tx.Exec(ctx,
		`INSERT INTO postings (transaction_id, account_id, amount_minor, currency)
		 VALUES ($1, $2, 100, 'EUR')`,
		txID, checking,
	); err != nil {
		t.Fatalf("insert posting: %v", err)
	}

	commitErr := tx.Commit(ctx)
	if commitErr == nil {
		t.Fatal("single-posting transaction committed successfully")
	}
	if !strings.Contains(commitErr.Error(), "minimum is 2") {
		t.Fatalf("commit error = %v, want message mentioning minimum is 2", commitErr)
	}
}

func TestMixedCurrencyRejected(t *testing.T) {
	pool := mustTestPool(t)
	userID := makeUser(t, pool)
	eurAcct := makeAccount(t, pool, userID, "eur-checking", "asset", "EUR")
	usdAcct := makeAccount(t, pool, userID, "usd-savings", "asset", "USD")

	ctx := context.Background()
	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatalf("begin: %v", err)
	}
	defer tx.Rollback(ctx)

	var txID uuid.UUID
	if err := tx.QueryRow(ctx,
		`INSERT INTO journal_transactions (user_id, description, occurred_at)
		 VALUES ($1, $2, $3) RETURNING id`,
		userID, "mixed", time.Now(),
	).Scan(&txID); err != nil {
		t.Fatalf("insert journal_transaction: %v", err)
	}

	// Both postings must satisfy the composite FK (currency matches account),
	// so the ONLY way to attempt mixed currency is to reference accounts of
	// different currencies.
	if _, err := tx.Exec(ctx,
		`INSERT INTO postings (transaction_id, account_id, amount_minor, currency)
		 VALUES ($1, $2, 5000, 'EUR'), ($1, $3, -5000, 'USD')`,
		txID, eurAcct, usdAcct,
	); err != nil {
		// Could fail here if... no, composite FK allows it (each row satisfies
		// its own account). But the balance trigger should catch it at commit.
		t.Fatalf("insert postings: %v", err)
	}

	commitErr := tx.Commit(ctx)
	if commitErr == nil {
		t.Fatal("mixed-currency transaction committed successfully")
	}
	if !strings.Contains(commitErr.Error(), "distinct currencies") {
		t.Fatalf("commit error = %v, want message mentioning distinct currencies", commitErr)
	}
}

func TestPostingCurrencyMustMatchAccount(t *testing.T) {
	pool := mustTestPool(t)
	userID := makeUser(t, pool)
	eurAcct := makeAccount(t, pool, userID, "eur-checking", "asset", "EUR")

	ctx := context.Background()
	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatalf("begin: %v", err)
	}
	defer tx.Rollback(ctx)

	var txID uuid.UUID
	if err := tx.QueryRow(ctx,
		`INSERT INTO journal_transactions (user_id, description, occurred_at)
		 VALUES ($1, $2, $3) RETURNING id`,
		userID, "wrong-currency", time.Now(),
	).Scan(&txID); err != nil {
		t.Fatalf("insert journal_transaction: %v", err)
	}

	// Account is EUR, posting claims USD. Composite FK must reject this at
	// INSERT time (before commit).
	_, err = tx.Exec(ctx,
		`INSERT INTO postings (transaction_id, account_id, amount_minor, currency)
		 VALUES ($1, $2, 100, 'USD')`,
		txID, eurAcct,
	)
	if err == nil {
		t.Fatal("posting with mismatched currency inserted successfully")
	}
	// err must be a foreign key violation (SQLSTATE 23503).
	if !strings.Contains(err.Error(), "23503") &&
		!strings.Contains(strings.ToLower(err.Error()), "foreign key") {
		t.Fatalf("insert error = %v, want foreign key violation", err)
	}
}

func TestUpdatePostingDenied(t *testing.T) {
	pool := mustTestPool(t)
	userID := makeUser(t, pool)
	checking := makeAccount(t, pool, userID, "checking", "asset", "EUR")
	groceries := makeAccount(t, pool, userID, "groceries", "expense", "EUR")

	// Create a valid transaction first.
	if err := tryPostBalanced(t, pool, userID, groceries, checking, 5000, "EUR"); err != nil {
		t.Fatalf("setup transaction: %v", err)
	}

	ctx := context.Background()
	var postingID uuid.UUID
	if err := pool.QueryRow(ctx,
		`SELECT id FROM postings WHERE account_id = $1 LIMIT 1`, checking,
	).Scan(&postingID); err != nil {
		t.Fatalf("find posting: %v", err)
	}

	_, err := pool.Exec(ctx,
		`UPDATE postings SET amount_minor = 9999 WHERE id = $1`, postingID,
	)
	if err == nil {
		t.Fatal("UPDATE postings succeeded; immutability trigger did not fire")
	}
	if !strings.Contains(err.Error(), "append-only") {
		t.Fatalf("update error = %v, want 'append-only'", err)
	}
}

func TestDeletePostingDenied(t *testing.T) {
	pool := mustTestPool(t)
	userID := makeUser(t, pool)
	checking := makeAccount(t, pool, userID, "checking", "asset", "EUR")
	groceries := makeAccount(t, pool, userID, "groceries", "expense", "EUR")

	if err := tryPostBalanced(t, pool, userID, groceries, checking, 5000, "EUR"); err != nil {
		t.Fatalf("setup transaction: %v", err)
	}

	ctx := context.Background()
	var postingID uuid.UUID
	if err := pool.QueryRow(ctx,
		`SELECT id FROM postings WHERE account_id = $1 LIMIT 1`, checking,
	).Scan(&postingID); err != nil {
		t.Fatalf("find posting: %v", err)
	}

	_, err := pool.Exec(ctx, `DELETE FROM postings WHERE id = $1`, postingID)
	if err == nil {
		t.Fatal("DELETE postings succeeded; immutability trigger did not fire")
	}
	if !strings.Contains(err.Error(), "append-only") {
		t.Fatalf("delete error = %v, want 'append-only'", err)
	}
}

func TestUpdateJournalTransactionDenied(t *testing.T) {
	pool := mustTestPool(t)
	userID := makeUser(t, pool)
	checking := makeAccount(t, pool, userID, "checking", "asset", "EUR")
	groceries := makeAccount(t, pool, userID, "groceries", "expense", "EUR")

	if err := tryPostBalanced(t, pool, userID, groceries, checking, 5000, "EUR"); err != nil {
		t.Fatalf("setup transaction: %v", err)
	}

	ctx := context.Background()
	var txID uuid.UUID
	if err := pool.QueryRow(ctx,
		`SELECT id FROM journal_transactions WHERE user_id = $1 LIMIT 1`, userID,
	).Scan(&txID); err != nil {
		t.Fatalf("find transaction: %v", err)
	}

	_, err := pool.Exec(ctx,
		`UPDATE journal_transactions SET description = 'rewritten' WHERE id = $1`, txID,
	)
	if err == nil {
		t.Fatal("UPDATE journal_transactions succeeded; immutability trigger did not fire")
	}
	if !strings.Contains(err.Error(), "append-only") {
		t.Fatalf("update error = %v, want 'append-only'", err)
	}
}
