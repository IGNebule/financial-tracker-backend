# Roadmap

## Current phase

Phase 4c complete. Phase 4d (accounts endpoints) about to start.

## What's implemented

### Infrastructure

- Config from `.env` with startup validation; missing variables fail start.
- pgxpool connection pool; `Ping` at startup; `/readyz` reports reachability.
- Versioned SQL migrations with advisory-lock serialization and per-migration
  transactions; files embedded in the binary via `//go:embed`.

### Auth

- bcrypt (cost 10) with `ErrInvalidPassword` / `ErrPasswordTooLong` sentinels.
- `POST /v1/users` (registration) and `POST /v1/login` (session issuance).
- Session tokens: 32 random bytes, hex-encoded, SHA-256 at rest, 24-hour
  lifetime. `Authorization: Bearer <token>` reads them.
- `requireAuth` middleware puts the authenticated user ID on the request
  context. Unknown email and wrong password return identical 401s.

### Ledger

- `internal/ledger`: `Account`, `Posting`, `Transaction` with `Validate`
  enforcing balance, minimum postings, single currency, non-zero amounts.
- `accounts`, `journal_transactions`, `postings` tables with FKs.
- Deferred constraint trigger enforces the three transaction-level
  invariants at COMMIT.
- `BEFORE UPDATE/DELETE` triggers enforce immutability.
- Composite FK enforces `posting.currency = account.currency`.

### HTTP surface

| Method | Path | Auth | Status |
|---|---|---|---|
| GET  | `/healthz` | — | done |
| GET  | `/version` | — | done |
| GET  | `/readyz`  | — | done |
| POST | `/v1/echo`  | — | done (learning endpoint) |
| POST | `/v1/users` | — | done |
| POST | `/v1/login` | — | done |
| POST | `/v1/accounts` | ✓ | **next (4d)** |
| GET  | `/v1/accounts` | ✓ | **next (4d)** |
| POST | `/v1/transactions` | ✓ | **planned (4e)** |

## Tests actually run

Most recent full run:

- `go test -vet=off -count=1 -short ./...` — all 8 packages pass.
- Integration tests (with `.env` exported):
  `internal/users` (5), `internal/sessions` (4), `internal/ledger` (5),
  `internal/db` (9).
- Postman manual checks: registration, login, duplicate, wrong password,
  unknown email, no-token-401.

## Known issues

- **`go vet` blocked** on this machine by Windows AppLocker. Workaround:
  `-vet=off` locally. Deployment pipeline runs `vet` on Linux.
- **Test suite is slow on Windows** (~2–7s per package) — likely antivirus
  or disk. Not a code issue.
- **Overflow limit** in `ledger.Validate`: amounts near `int64` max can
  overflow the sum. Documented in ADR-0004; handler-side bound (Phase 4e)
  will cap amounts far below the limit.

## Next step

**Phase 4d** — `POST /v1/accounts` and `GET /v1/accounts`:
- Ownership via `UserIDFromContext` (never from the request body).
- Duplicate name → 409; invalid type/currency → 400.
- Empty list serializes as `[]`, not `null`.

Then **Phase 4e** — `POST /v1/transactions`:
- Uses `ledger.NewTransaction` for validation.
- Ownership check: user must own every account in the postings.
- Persists via explicit `BEGIN ... COMMIT` so the deferred trigger fires.
- Maps `ErrUnbalanced`, `ErrMixedCurrency`, `ErrEmptyTransaction`,
  `ErrZeroAmount` to 400 with specific codes.