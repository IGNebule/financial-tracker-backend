# Financial Tracker Backend

A Go + PostgreSQL backend for personal financial tracking, built incrementally
as a learning project. Implements double-entry accounting with database-enforced
invariants, session-based authentication, and a REST API.

**Status:** early. Auth, users, and the ledger schema are complete; financial
endpoints (`POST /v1/transactions`) are in progress. See `docs/ROADMAP.md`.

## Prerequisites

- Go 1.27+ (`go version`)
- PostgreSQL 16+ (local development uses 18; `psql --version`)
- Git Bash on Windows (POSIX shell; PowerShell works but examples assume bash)

## Quick start

```bash
# 1. Configure
cp .env.example .env
# Edit .env — set APP_DB_PASSWORD to a password for a PostgreSQL role you own.

# 2. Create the role and database once (as a PostgreSQL superuser)
psql -h localhost -U postgres -d postgres
#   CREATE ROLE <role> WITH LOGIN;
#   \password <role>
#   CREATE DATABASE financial_app OWNER <role>;
#   \q

# 3. Apply migrations
go run ./cmd/migrate

# 4. Start the server
go run ./cmd/api
# → listening on :8080
```

Test the first request:

```bash
curl -i http://localhost:8080/healthz
# HTTP/1.1 200 OK
# {"status":"ok"}
```

## What works today

| Method | Path | Auth | Purpose |
|---|---|---|---|
| `GET`  | `/healthz` | — | Process is alive |
| `GET`  | `/version` | — | Build version |
| `GET`  | `/readyz`  | — | Database reachable |
| `POST` | `/v1/echo`  | — | Format a money value (learning endpoint) |
| `POST` | `/v1/users` | — | Register |
| `POST` | `/v1/login` | — | Issue a session token |

**Planned (Phase 4d+):** `POST /v1/accounts`, `GET /v1/accounts`,
`POST /v1/transactions`. See `docs/ROADMAP.md` for the full plan.

## Authentication

Register, log in, then send the token as `Authorization: Bearer <token>`:

```bash
curl -s -X POST http://localhost:8080/v1/users \
  -H 'Content-Type: application/json' \
  -d '{"email":"alice@example.com","password":"correct horse battery staple"}'

TOKEN=$(curl -s -X POST http://localhost:8080/v1/login \
  -H 'Content-Type: application/json' \
  -d '{"email":"alice@example.com","password":"correct horse battery staple"}' \
  | jq -r .token)

curl -s http://localhost:8080/v1/accounts -H "Authorization: Bearer $TOKEN"
```

Session tokens are opaque, random (256 bits), and stored server-side as
SHA-256 hashes. They expire 24 hours after issuance. See ADR-0003.

## Project layout

```
cmd/
  api/        — the HTTP server binary (composition root)
  migrate/    — the migration runner binary
internal/
  auth/       — bcrypt password hashing and session token generation
  config/     — .env loader and startup validation
  db/         — pgxpool wrapper and migration runner
  httpapi/    — HTTP handlers, middleware, request/response shapes
  ledger/     — double-entry domain model and (soon) repository
  money/      — money formatting and currency validation
  sessions/   — session persistence (PostgreSQL)
  users/      — user persistence (PostgreSQL)
migrations/   — versioned SQL files, embedded at build time
docs/         — architecture notes and ADRs
```

Everything under `internal/` is importable only within this module. That is
Go's built-in encapsulation; see `docs/architecture/overview.md`.

## Running tests

```bash
# Fast (fakes only, no database) — always safe
go test -vet=off -count=1 -short ./...

# Full (includes integration tests requiring PostgreSQL)
set -a; . ./.env; set +a
go test -vet=off -count=1 ./...
```

`-vet=off` is required on some Windows machines where Windows Application
Control blocks `vet.exe`. On Linux/macOS/CI, drop the flag. See
`CLAUDE.md` for details.

## Migrations

```bash
go run ./cmd/migrate     # apply pending migrations
```

Migrations are forward-only and idempotent. A migration that fails midway
rolls back cleanly — the runner wraps each file in a transaction and takes
an advisory lock so concurrent invocations serialize. See ADR-0002.

## Design notes

- **Money is stored as integer minor units** (cents), never floats. See
  ADR-0004.
- **The ledger's balance invariant is enforced by the database**, not only
  by Go code. A `psql` session cannot insert an unbalanced transaction.
- **Posted journal entries are immutable**; corrections are compensating
  transactions, not edits.
- **User ID always comes from the session**, never from a request body or
  query parameter.

## Known limitations

- No rate limiting yet (Phase 8). Login is currently vulnerable to offline
  brute-force attempts bounded only by bcrypt's cost factor.
- No graceful shutdown (Phase 8). Ctrl+C kills in-flight requests.
- No pagination. `GET /v1/accounts` returns all accounts.
- Currency support is limited to EUR, USD, GBP. Foreign-exchange
  transactions are deferred entirely.
- Single-instance only. `pg_advisory_lock` and connection pooling assume
  one process; the database itself handles multiple app instances, but the
  session/migration stories have not been tested under multi-instance load.

## License

Not yet chosen.