# ADR-0002: Double-entry ledger with database-enforced invariants

- **Status:** Accepted
- **Date:** 2026-11-10
- **Deciders:** project author

## Context

A financial tracker must not lose or invent money. A single-column
`balance` on an `accounts` table is easy to write and easy to get wrong:
concurrent updates race, a buggy handler writes an incorrect delta, and
there is no way to reconstruct what happened.

The core requirement is that **every recorded financial event is a
movement of value between at least two accounts, and the movements sum to
zero**. This is the double-entry principle, and it is the minimum bar for
a system that wants to reconcile.

A second requirement from the project brief: correctness must hold even
if the Go code has a bug. A compromised application process must not be
able to corrupt the ledger.

## Decision

We will implement a **double-entry ledger** with the following shape:

- `accounts` — user-owned buckets, each tagged with an account type
  (asset, liability, equity, income, expense) and a currency.
- `journal_transactions` — one row per financial event, with a
  description and an `occurred_at` timestamp.
- `postings` — one row per (transaction, account) pair, with a signed
  `amount_minor` (positive = debit, negative = credit).

The invariant is `SUM(amount_minor) = 0` per transaction, and all postings
in a transaction must share a currency.

The invariant is enforced in **two independent places**:

1. **Go domain** (`ledger.Validate`) — runs in the handler before any SQL
   is issued. Provides fast feedback and a well-typed error for the API.
2. **PostgreSQL** — a `DEFERRABLE INITIALLY DEFERRED` constraint trigger
   that fires at COMMIT and raises if the sum is non-zero, if the
   currency count exceeds one, or if there are fewer than two postings.

**Posted rows are immutable.** A `BEFORE UPDATE OR DELETE` trigger on
both `journal_transactions` and `postings` raises and aborts. Corrections
are made by posting compensating transactions — a new transaction whose
postings negate the error, optionally followed by a correct one.

## Consequences

### Positive

- **The ledger is auditable.** Every financial state is the sum of a
  history of postings, and the history cannot be rewritten. A bug in Go
  code produces a rejected transaction, not a corrupted balance.
- **A direct `psql` session cannot corrupt the ledger.** This was tested:
  `INSERT journal_transactions` + `INSERT postings` with a non-zero sum
  fails at `COMMIT` with `transaction does not balance`, and the whole
  transaction rolls back. See `internal/db/ledger_constraints_test.go`.
- **Concurrency is handled by PostgreSQL.** A `SELECT ... FOR UPDATE` on
  the account rows (Phase 4e, Phase 6) plus the deferred check means
  concurrent writes cannot interleave to produce an unbalanced ledger.
- **Currency safety is enforced independently.** A posting whose currency
  does not match its account is rejected at INSERT by composite FK, not
  only at commit. Two layers, two failure modes caught.
- **Reversibility is real.** "Undo last transaction" is a compensating
  posting, not an undo. The original stays. The audit trail is intact.

### Negative

- **Two implementations of the same rule.** The Go and SQL checks must
  agree. If someone adds a new rule to `Validate` and forgets the
  trigger, the database will accept what Go rejects — and vice versa.
  Mitigation: the integration tests in `internal/db` and the domain tests
  in `internal/ledger` cover the same cases; a divergence makes one of
  them fail.
- **Corrections require two writes.** "Edit last transaction" is not
  possible; the user must post a reversal, then a corrected transaction.
  This is a UX cost, not a correctness cost. Phase 9's reporting will
  make reversal transactions visually distinguishable.
- **The trigger fires once per row, not per statement.** A 100-posting
  transaction runs the balance query 100 times. Mitigation: transactions
  in this app are small (typically 2–4 postings). For pathological cases,
  the trigger could be rewritten as a statement-level trigger with a
  temporary table, but that is not justified yet.
- **Multi-currency is impossible without a redesign.** A EUR→USD transfer
  cannot be modelled. This is a feature, not a bug — the prompt explicitly
  defers FX. When it arrives, it will need a base currency, an exchange
  rate table, and a gain/loss account per currency pair.

### Alternatives considered

- **A `balance` column updated on every write.** Rejected: it is a
  denormalization that introduces a new invariant (balance = sum of
  historical postings) that must be maintained transactionally on every
  write. The double-entry model makes that invariant structural.
- **Domain-only enforcement (no database triggers).** Rejected: a bug in
  a handler, a migration script, or a compromised application would not
  be caught. The whole point is that the storage layer enforces the
  invariant.
- **Database-only enforcement (no domain `Validate`).** Rejected: the
  handler needs to map "unbalanced" to a 400 with a specific error code
  before opening a database transaction. Waiting for COMMIT to fail is
  slower and produces an error the handler cannot distinguish from a
  connection failure.
- **Event sourcing.** Rejected as overkill: the journal is already a log;
  a second log of events would be redundant. If event-driven analytics
  becomes a need, it is a projection over the existing tables, not a
  replacement.