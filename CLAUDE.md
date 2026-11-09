# CLAUDE.md — project conventions

Guidance for AI assistants (and humans) working in this repository. Update
this file when a convention changes, not on every commit.

## What this project is

A Go + PostgreSQL financial-tracking backend. It implements double-entry
accounting with database-enforced invariants. It is a learning project, not
a production system, and the code reflects the intent to teach correctness
over speed of delivery.

## Environment expectations

- Primary shell: **Git Bash** on Windows. All examples use POSIX syntax.
- `go vet` is blocked on one developer's machine by Windows AppLocker.
  Commands in this project use `go test -vet=off` to work around it.
  **On Linux/macOS/CI, drop the flag** — the deployment pipeline runs `vet`.
- PostgreSQL 18 server at `localhost:5432`, database `financial_app`.
- Docker is not available locally. Deployments happen via Hugging Face
  Docker Spaces building the image remotely.

## Running things

```bash
go run ./cmd/migrate             # apply migrations
go run ./cmd/api                 # start the server
go test -vet=off -count=1 -short ./...    # fast tests (no DB)
set -a; . ./.env; set +a
go test -vet=off -count=1 ./...           # full tests (needs DB)
gofmt -l .                       # list files needing formatting
go build ./...                   # compile every package
```

## Architecture

Layers, from innermost to outermost:

1. **Domain** — `internal/money`, `internal/ledger`. No imports of `pgx`,
   `net/http`, or `config`. Pure Go logic and business rules.
2. **Application ports** — the function types in `httpapi.Deps` (e.g.
   `CreateUser`, `CreateAccount`). Handlers depend on these signatures,
   not on concrete repository types.
3. **Adapters** — `internal/users`, `internal/sessions`, `internal/ledger`
   (repository), `internal/db`. Talk to PostgreSQL via pgx.
4. **Transport** — `internal/httpapi`. HTTP handlers, middleware, JSON.
5. **Composition root** — `cmd/api/main.go`. Wires everything together.

Dependencies point **inward**. The domain never imports an adapter. If a
business rule needs data from the database, the adapter passes it to the
domain as a plain value; the domain does not call the database.

## Code conventions

### Errors

- **Sentinel errors** for conditions callers must distinguish:
  `money.ErrUnsupportedCurrency`, `users.ErrDuplicateEmail`,
  `ledger.ErrUnbalanced`, `sessions.ErrNotFound`. Use `errors.New` at
  package level.
- **Wrap with context** when returning to a caller who needs the detail:
  `fmt.Errorf("create user: %w", err)`. Use `%w`, not `%v`, for anything
  the caller might check with `errors.Is` or `errors.As`.
- **Do not format error strings for display**. Error messages are for
  developers reading logs; API responses use a structured body
  (`{"code": "...", "message": "..."}`) built by the handler.
- **Never include the raw error text in an HTTP 500 response.** Log it,
  do not return it.

### HTTP handlers

- Each handler is a function factory: `handleX(deps...) http.HandlerFunc`.
  Dependencies arrive as captured function values, not concrete types.
- Always start with `UserIDFromContext` for protected routes. Never
  accept `user_id` from a request body or query parameter.
- Always set `Content-Type` **before** `WriteHeader` **before** writing
  the body.
- Always `http.MaxBytesReader` on the body, then
  `json.NewDecoder(...).DisallowUnknownFields()`.
- Validation lives in the handler; the domain assumes valid input
  (ownership check excepted — that is the handler's responsibility).

### Database

- All queries use parameter binding (`$1`, `$2`, ...). **Never** build SQL
  by string concatenation. No exceptions.
- Repository methods take `context.Context` as the first argument.
- `SELECT` returning zero rows means `pgx.ErrNoRows`; the caller decides
  whether that is a 404, a 401, or something else.
- **Never** call `db.Pool().Exec` from a handler. Repository methods are
  the only interface to raw SQL, and only from the adapter layer.
- Migrations are forward-only. Do not add `down` migrations. To fix a
  schema problem, add a new migration.
- Every migration is idempotent-safe: running `go run ./cmd/migrate` with
  no new files is a no-op.

### Money

- Store as `int64` **minor units** (cents), never `float64`. Never.
- Currency is always explicit, stored alongside the amount.
- Display formatting goes through `money.Format`; comparisons and
  arithmetic use integers.

### Ledger

- A `ledger.Transaction` is valid iff `sum(AmountMinor) == 0` and all
  postings share a currency and there are at least two postings and no
  posting is zero. `Validate` enforces this.
- The database enforces the same invariants via deferred constraint
  trigger and composite FK. **Do not bypass either layer**; the two are
  intentionally independent.
- Posted rows are immutable. Corrections are compensating transactions.
  `UPDATE` and `DELETE` on `journal_transactions` and `postings` are
  blocked by trigger. The domain has no `Update` method; do not add one.

### Auth

- Passwords hashed with bcrypt, cost 10, via `auth.HashPassword`.
- Session tokens are 32 random bytes, hex-encoded; only `sha256(token)`
  is stored. See ADR-0003.
- Login always returns the same response for "wrong password" and "user
  not found" — 401, `INVALID_CREDENTIALS`. Do not distinguish.
- Protected routes are wrapped individually with `requireAuth(...)`.
  `grep "auth(" internal/httpapi/router.go` is the complete list of
  protected endpoints.

### Testing

- Unit tests (fast, no I/O) go in `_test.go` next to the code.
- Integration tests (need PostgreSQL) skip under `-short` and when
  `APP_DB_HOST` is unset.
- Tests describe **intended behavior**, not current behavior. When a
  test fails, the question is "what behavior did I want?", not "how do I
  make it pass?".
- A test that would pass under a broken implementation is not a test.
- Use `t.TempDir()` for test file I/O; never write into the repo.

## Git workflow

- Conventional Commits: `type(scope): subject` — `feat`, `fix`, `refactor`,
  `docs`, `test`, `chore`, `style`, `perf`.
- Subject in imperative mood, lowercase after colon, no trailing period.
- **`git add <specific paths>`** — never `git add .`. This is how `.env`
  would leak.
- `.env` is in `.gitignore`. **Never** remove that line.
- Push after every completed checkpoint, not at the end of a phase.

## What NOT to do

- Do not commit `.env`, `*.exe`, `*.test`, `*.out`, or anything generated
  by `go test` or `go build`.
- Do not disable `vet` in CI. Use `-vet=off` locally only.
- Do not add a new dependency without editing the same commit to include
  `go.mod` and `go.sum`.
- Do not write a `down` migration. No `.down.sql` files.
- Do not import an adapter package from the domain (`ledger`, `money`).
- Do not accept `user_id` from a request body or path.
- Do not log a password, session token, or `.env` value.
- Do not return `err.Error()` in an HTTP 500 response.
- Do not use `panic()` for anything that can happen at runtime.

## Where things live

- "How do I run X?" → README.md
- "Why is the code arranged this way?" → docs/architecture/overview.md
- "What are the tables?" → docs/architecture/data-model.md
- "Why did we pick Y?" → docs/adr/
- "What's next?" → docs/ROADMAP.md