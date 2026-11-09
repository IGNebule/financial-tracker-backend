# Roadmap

## Current phase

Phase 3 complete. Phase 4 (double-entry ledger) starting.

## What's implemented
- Config from `.env` with startup validation (fail closed)\
- `pgxpool` connection pool, `Ping` at startup, `/readyz` reports DB reachability
- Versioned SQL migrations with advisory-lock serialization and per-migration transactions (`cmd/migrate`)
- `users` table: UUID PK, case-insensitive UNIQUE on email, `TIMESTAMPTZ`
- bcrypt (cost 10) with `ErrInvalidPassword` / `ErrPasswordTooLong` sentinels
- `users.Repository`: `Create` (INSERT ... RETURNING), `FindByEmail`, translates SQLSTATE 23505 to `ErrDuplicateEmail`
- HTTP: `GET /healthz`, `/version`, `/readyz`; `POST /v1/echo`, `/v1/users`
- Handler tests use fakes (fast, no DB); repository tests are integration (skipped under `-short` or when `APP_DB_HOST` is unset)

## Tests actually run
- `go test -vet=off -count=1 -short ./...` - all 8 packages pass
- `go test -vet=off -count=1 -v ./internal/users` - 5 integration tests pass
- `go test -vet=off -count=1 -v ./internal/httpapi -run 'TestCreateUser|TestIsValidEmail'` - pass
- Manual Postman: 201/409/409/400 verified

## Next step
Phase 4.1: accounting primer + `internal/ledger` domain package
(`Account`, `Posting`, `Transaction` with balance validation), pure Go, no database.