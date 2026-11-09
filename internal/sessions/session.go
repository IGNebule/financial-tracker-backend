// Package sessions manages the lifecycle of authenticated sessions: creation
// on login, lookup on each request, and optional revocation.
package sessions

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"financialtracker/internal/auth"
)

// DefaultLifetime is how long a session remains valid after issuance.
// 24 hours is a reasonable compromise: long enough for a workday, short
// enough that a stolen token expires quickly. The value is fixed here
// rather than configurable so that it cannot drift between environments;
// Phase 13 could make it a config value if needed.
const DefaultLifetime = 24 * time.Hour

// ErrNotFound is returned by FindByToken when no live session matches. It
// conflates "token does not exist", "token expired", and "token was revoked"
// on purpose: a client should not be able to distinguish these cases.
var ErrNotFound = errors.New("sessions: no active session")

// Session is a live authentication session.
type Session struct {
	TokenHash [32]byte // never exposed to clients
	UserID    uuid.UUID
	CreatedAt time.Time
	ExpiresAt time.Time
}

// Repository persists sessions.
type Repository struct {
	pool *pgxpool.Pool
}

func NewRepository(pool *pgxpool.Pool) *Repository {
	return &Repository{pool: pool}
}

// Create issues a new session for userID and returns the raw token. The
// token is what the caller sends to the client; the database stores only
// its hash. The caller must not log or store the returned token.
func (r *Repository) Create(ctx context.Context, userID uuid.UUID) (string, error) {
	token, err := auth.GenerateToken()
	if err != nil {
		return "", fmt.Errorf("generate token: %w", err)
	}
	hash, err := auth.HashToken(token)
	if err != nil {
		return "", fmt.Errorf("hash token: %w", err)
	}

	const q = `
		INSERT INTO sessions (token_hash, user_id, expires_at)
		VALUES ($1, $2, NOW() + $3)
	`
	if _, err := r.pool.Exec(ctx, q, hash[:], userID, DefaultLifetime); err != nil {
		return "", fmt.Errorf("insert session: %w", err)
	}
	return token, nil
}

// FindByToken returns the live session for the given raw token. The raw
// token is hashed here; callers never see the hash. Expired sessions are
// treated as not found.
func (r *Repository) FindByToken(ctx context.Context, token string) (*Session, error) {
	hash, err := auth.HashToken(token)
	if err != nil {
		return nil, ErrNotFound
	}

	const q = `
		SELECT token_hash, user_id, created_at, expires_at
		FROM sessions
		WHERE token_hash = $1 AND expires_at > NOW()
	`
	s := &Session{}
	var rawHash []byte
	err = r.pool.QueryRow(ctx, q, hash[:]).Scan(&rawHash, &s.UserID, &s.CreatedAt, &s.ExpiresAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("lookup session: %w", err)
	}
	copy(s.TokenHash[:], rawHash)
	return s, nil
}

// Delete removes a session, immediately invalidating its token. Called by
// a future logout endpoint; not yet exposed.
func (r *Repository) Delete(ctx context.Context, token string) error {
	hash, err := auth.HashToken(token)
	if err != nil {
		return nil // invalid format = nothing to delete
	}
	const q = `DELETE FROM sessions WHERE token_hash = $1`
	if _, err := r.pool.Exec(ctx, q, hash[:]); err != nil {
		return fmt.Errorf("delete session: %w", err)
	}
	return nil
}
