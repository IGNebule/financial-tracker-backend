// Package db wraps a PostgreSQL connection pool and exposes the operations the
// rest of the application needs: a ready-to-use pool, a health check, and
// orderly shutdown.
package db

import (
	"context"
	"fmt"
	"net"
	"net/url"
	"strconv"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"financialtracker/internal/config"
)

// DB is a PostgreSQL connection pool with a small, explicit surface. Handlers
// and use cases should never reach past this type to *pgxpool.Pool; if a new
// operation needs raw pool access, add a method here.
type DB struct {
	pool *pgxpool.Pool
}

// Open creates the pool and verifies it with a Ping. It returns an error if
// the pool cannot be created or the server cannot be reached within five
// seconds. The caller must Close the returned DB.
func Open(ctx context.Context, cfg config.DBConfig) (*DB, error) {
	dsn := buildDSN(cfg)

	poolCfg, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		return nil, fmt.Errorf("parse pool config: %w", err)
	}
	poolCfg.MaxConns = 10
	poolCfg.MinConns = 1
	poolCfg.MaxConnLifetime = 30 * time.Minute
	poolCfg.MaxConnIdleTime = 5 * time.Minute
	poolCfg.HealthCheckPeriod = 30 * time.Second

	pool, err := pgxpool.NewWithConfig(ctx, poolCfg)
	if err != nil {
		return nil, fmt.Errorf("create pool: %w", err)
	}

	pingCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	if err := pool.Ping(pingCtx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("ping database: %w", err)
	}

	return &DB{pool: pool}, nil
}

// Ping verifies the pool can reach the server. It is called by GET /readyz.
func (d *DB) Ping(ctx context.Context) error {
	return d.pool.Ping(ctx)
}

// Close releases all pooled connections. Safe to call more than once.
func (d *DB) Close() {
	d.pool.Close()
}

func (d *DB) Pool() *pgxpool.Pool {
	return d.pool
}

// buildDSN assembles a PostgreSQL connection string from structured settings.
// It uses url.URL so that a username or password containing reserved
// characters (@, :, /, %, ...) is encoded correctly rather than corrupting
// the DSN. A connection string built by naive string concatenation silently
// breaks for exactly those passwords.
func buildDSN(cfg config.DBConfig) string {
	u := &url.URL{
		Scheme: "postgres",
		User:   url.UserPassword(cfg.User, cfg.Password),
		Host:   net.JoinHostPort(cfg.Host, strconv.Itoa(cfg.Port)),
		Path:   "/" + cfg.Name,
	}
	q := url.Values{}
	q.Set("sslmode", cfg.SSLMode)
	u.RawQuery = q.Encode()
	return u.String()
}
