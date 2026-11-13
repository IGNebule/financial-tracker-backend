package ledger

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

var ErrDuplicateAccountName = errors.New("ledger: account name already exists for this user")

type Repository struct {
	pool *pgxpool.Pool
}

func NewRepository(pool *pgxpool.Pool) *Repository {
	return &Repository{pool: pool}
}

func (r *Repository) CreateAccount(ctx context.Context, userID uuid.UUID, name string, typ AccountType, currency string) (*Account, error) {
	const q = `
		INSERT INTO accounts (user_id, name, type, currency)
		VALUES ($1, $2, $3, $4)
		RETURNING id, user_id, name, type, currency, created_at
	`
	a := &Account{}
	var typStr string
	err := r.pool.QueryRow(ctx, q, userID, name, string(typ), currency).Scan(
		&a.ID, &a.UserID, &a.Name, &typStr, &a.Currency, &a.CreatedAt,
	)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return nil, ErrDuplicateAccountName
		}
		return nil, fmt.Errorf("insert account: %w", err)
	}
	a.Type = AccountType(typStr)
	return a, nil
}

func (r *Repository) ListAccountByUser(ctx context.Context, userID uuid.UUID, name string, typ AccountType, currency string) (*Account, error) {
	const q = `
		INSERT INTO accounts (user_id, name, type, currency)
		VALUES ($1, $2, $3, $4)
		RETURNING id, user_id, name, type, currency, created_at
	`
	a := &Account{}
	var typStr string
	err := r.pool.QueryRow(ctx, q, userID, name, string(typ), currency).Scan(&a.ID, &a.UserID, &a.Name, &typStr, &a.Currency, &a.CreatedAt)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return nil, ErrDuplicateAccountName
		}
		return nil, fmt.Errorf("insert account: %w", err)
	}
	a.Type = AccountType(typStr)
	return a, nil
}

func (r *Repository) ListAccountsByUser(ctx context.Context, userID uuid.UUID) ([]*Account, error) {
	const q = `
		SELECT id, user_id, name, type, currency, created_at
		FROM accounts
		WHERE user_id = $1
		ORDER BY name
	`
	rows, err := r.pool.Query(ctx, q, userID)
	if err != nil {
		return nil, fmt.Errorf("query accounts: %w", err)
	}
	defer rows.Close()

	out := []*Account{}
	for rows.Next() {
		a := &Account{}
		var typStr string
		if err := rows.Scan(&a.ID, &a.UserID, &a.Name, &typStr, &a.Currency, &a.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan account: %w", err)
		}
		a.Type = AccountType(typStr)
		out = append(out, a)
	}
	return out, rows.Err()
}
