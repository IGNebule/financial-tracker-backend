# Architecture overview

## Shape

A modular monolith: one process, one database, clear internal boundaries
between domain logic and infrastructure. Microservices are deferred until
there is a measurable cost to keeping things together. See ADR-0001.

## Layers

```
┌───────────────────────────────────────────────────────────┐
│                    cmd/api (composition root)             │
│   reads config, opens pool, wires repos into the router   │
└───────────────────────────┬───────────────────────────────┘
                            │
┌───────────────────────────▼───────────────────────────────┐
│                  internal/httpapi (transport)             │
│   handlers, middleware, JSON encoding, error mapping      │
│   depends on: function types declared in Deps             │
└───────────────────────────┬───────────────────────────────┘
                            │  (function values, not types)
┌───────────────────────────▼───────────────────────────────┐
│              internal/{users,sessions,ledger}             │
│                     (adapters)                            │
│   repositories: SQL, transaction boundaries, pgx          │
│   depends on: domain, config, db                          │
└───────────────────────────┬───────────────────────────────┘
                            │
┌───────────────────────────▼───────────────────────────────┐
│                  internal/{money,ledger}                  │
│                     (domain)                              │
│   pure Go: types, invariants, validation                  │
│   depends on: stdlib, uuid (no pgx, no net/http)          │
└───────────────────────────────────────────────────────────┘
```

## Dependency rule

Dependencies point **inward**. The domain does not import any adapter.
`internal/ledger` does not know `internal/db` exists; `internal/db` imports
`internal/ledger` when it needs the `AccountType` string. This is the
distinction between a domain that says "these are the rules" and an adapter
that says "here is how we store the rules".

The test: `grep -r "internal/db" internal/ledger/` should return nothing.
`grep -r "jackc/pgx" internal/ledger/` should return nothing.

## How a request flows

Example: `GET /v1/accounts` with a bearer token.

1. **`net/http`** accepts the connection and dispatches to the mux.
2. **`mux.Handle("GET /v1/accounts", auth(...))`** — the router wrapped this
   route with `requireAuth`, so auth runs first.
3. **`requireAuth`** reads `Authorization: Bearer <token>`, calls
   `findSession(ctx, token)` (a function value wired in `main.go`),
   receives a `*sessions.Session`, and stores `session.UserID` on the
   request context via `WithUserID`.
4. **`handleListAccounts`** runs. It calls `UserIDFromContext` (guaranteed
   present because of step 3), then `listAccounts(ctx, userID)` — another
   function value from `Deps`.
5. **`ledger.Repository.ListAccountsByUser`** issues `SELECT ... WHERE
   user_id = $1 ORDER BY name`, scans into `[]*ledger.Account`, and returns.
6. **`handleListAccounts`** maps `[]*ledger.Account` to
   `[]accountResponse` and JSON-encodes.
7. The response goes back through the middleware stack, which does nothing
   further, and back to `net/http`.

Each step is testable in isolation:

- Step 3 (`requireAuth`) is tested with a fake `findSession`.
- Steps 4–6 (`handleListAccounts`) are tested with a fake `listAccounts`.
- Step 5 (`Repository.ListAccountsByUser`) is tested against a real
  PostgreSQL via integration tests.

No test needs to stand up the whole stack.

## Why function values in `Deps` instead of interfaces

`httpapi.Deps` declares each dependency as a **function value**:

```go
type Deps struct {
    FindSession   func(context.Context, string) (*sessions.Session, error)
    CreateAccount func(context.Context, uuid.UUID, string, ledger.AccountType, string) (*ledger.Account, error)
    // ...
}
```

This is a deliberate choice. An interface would work, but:

- An interface has to be **declared somewhere**, which is another file.
  A function type is declared inline, at the point of use.
- **Only the methods the handler actually calls** need to appear in the
  function type. An interface would need every method on the repository
  to be stubbed by every fake, including ones the handler does not use.
- **Fakes are closures**, not structs. A test writes:

  ```go
  createAcct := func(ctx context.Context, uid uuid.UUID, name string, typ ledger.AccountType, cur string) (*ledger.Account, error) {
      return nil, ledger.ErrDuplicateAccountName
  }
  ```

  That is 3 lines, no struct, no boilerplate. `main.go` wires the real
  method value: `CreateAccount: ledgerRepo.CreateAccount`.

The tradeoff: `Deps` grows a field per dependency, and adding a new
dependency is a breaking change for every caller of `NewRouter`. This is
**intentional**. A handler that suddenly needs a new dependency should
force a review of its dependencies, not get one injected silently.

## What is intentionally not here

- **No microservices.** A single Go process with one database has no
  network boundary to protect, so no gRPC, no service mesh, no distributed
  tracing. Those are Phase 10+ decisions justified by a real operational
  need, not by fashion.
- **No ORM.** Every query is explicit SQL, readable in the repository.
  The double-entry trigger makes SQL part of the domain model; an ORM
  would obscure it.
- **No event sourcing.** The journal table is already a log; we do not
  need a second one.
- **No CQRS split yet.** A single PostgreSQL primary serves reads and
  writes. Read replicas are Phase 9, if at all.