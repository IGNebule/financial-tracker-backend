# Data model

## Entity relationships

```
users ─┬─→ accounts ───┐
       │               │
       ├─→ sessions    │
       │               │
       └─→ journal_transactions ──→ postings ──→ (accounts)
```

Postings reference accounts and transactions. Transactions reference users.
Accounts reference users. Sessions reference users and cascade on delete.

The composite FK `postings(account_id, currency) → accounts(id, currency)`
means a posting's currency must match its account's currency. This is why
`accounts` has an extra `UNIQUE (id, currency)` constraint — a redundant
but required-by-Postgres unique on the primary key column plus currency.

## Tables

### `users`

```sql
id            UUID PRIMARY KEY DEFAULT gen_random_uuid()
email         TEXT NOT NULL UNIQUE
password_hash TEXT NOT NULL
created_at    TIMESTAMPTZ NOT NULL DEFAULT NOW()

CREATE UNIQUE INDEX users_email_lower_unique ON users (LOWER(email));
```

- UUIDs, not sequential integers — see ADR-0001.
- `password_hash` holds bcrypt output (cost 10). Never the plaintext.
- Two unique constraints on email: the column-level `UNIQUE` and a
  case-insensitive functional index. Redundant on purpose: the handler
  lowercases email before insert, but a direct SQL insert that does not
  will still be caught by the functional index.

### `accounts`

```sql
id         UUID PRIMARY KEY DEFAULT gen_random_uuid()
user_id    UUID NOT NULL REFERENCES users(id)
name       TEXT NOT NULL
type       TEXT NOT NULL CHECK (type IN ('asset','liability','equity','income','expense'))
currency   TEXT NOT NULL
created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()

UNIQUE (user_id, name)
UNIQUE (id, currency)   -- required for the composite FK on postings
```

- `type` is one of the five account types from double-entry accounting.
  The CHECK clause is the final guard against an invalid type; the
  `ledger.AccountType.Valid()` method is the friendly guard in Go.
- `UNIQUE (user_id, name)` — per-user name uniqueness. Two users may both
  have a "Checking" account.
- `UNIQUE (id, currency)` — required by PostgreSQL so that `postings` can
  reference both columns in a composite FK. Technically redundant with the
  primary key, but PostgreSQL enforces the constraint explicitly.

### `journal_transactions`

```sql
id          UUID PRIMARY KEY DEFAULT gen_random_uuid()
user_id     UUID NOT NULL REFERENCES users(id)
description TEXT NOT NULL
occurred_at TIMESTAMPTZ NOT NULL
created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
```

- One row per financial event. A €50 grocery expense is one
  `journal_transactions` row plus two `postings` rows.
- `occurred_at` (when it happened) is distinct from `created_at` (when we
  recorded it). A transaction can be recorded retroactively.
- Immutable: `UPDATE` and `DELETE` blocked by trigger.

### `postings`

```sql
id             UUID PRIMARY KEY DEFAULT gen_random_uuid()
transaction_id UUID NOT NULL REFERENCES journal_transactions(id)
account_id     UUID NOT NULL
amount_minor   BIGINT NOT NULL CHECK (amount_minor <> 0)
currency       TEXT NOT NULL
created_at     TIMESTAMPTZ NOT NULL DEFAULT NOW()

FOREIGN KEY (account_id, currency) REFERENCES accounts(id, currency)
```

- `amount_minor` is **signed**: positive = debit, negative = credit. This
  is the encoding that makes the balance check a single
  `SUM(amount_minor) = 0`.
- `CHECK (amount_minor <> 0)` — a zero posting is meaningless; rejected at
  the row level.
- The composite FK enforces `posting.currency == account.currency` at
  INSERT time (before the balance trigger, which is deferred).
- Immutable: `UPDATE` and `DELETE` blocked by trigger.

### `sessions`

```sql
token_hash BYTEA PRIMARY KEY
user_id    UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE
created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
expires_at TIMESTAMPTZ NOT NULL
```

- `token_hash` is `SHA-256(<token>)`. The raw token is never stored.
- `ON DELETE CASCADE` means deleting a user deletes their sessions.
- Expiry is enforced at lookup (`expires_at > NOW()`). Expired rows are
  not deleted automatically yet; Phase 8 adds a maintenance job.

### `schema_migrations`

```sql
version    TEXT PRIMARY KEY
applied_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
```

- Maintained by `internal/db.Migrate`. Do not edit by hand.
- A migration is skipped when its numeric prefix is already in this table.
- To force re-application of a migration (for development only), delete
  its row and drop the objects it created.

## Invariants

These are properties the system guarantees. Most are enforced by the
database, not just by Go code — a direct `psql` session cannot violate them.

| Invariant | Enforced by |
|---|---|
| A user's email is unique (case-insensitive) | `UNIQUE` + functional unique index |
| A user's account names are unique | `UNIQUE (user_id, name)` |
| A posting's currency matches its account's currency | Composite FK |
| Every transaction has ≥ 2 postings | Deferred constraint trigger |
| Every transaction uses exactly one currency | Deferred constraint trigger |
| Every transaction's postings sum to zero | Deferred constraint trigger |
| Journal transactions and postings are immutable | `BEFORE UPDATE/DELETE` triggers |
| A session token is unique and stored hashed | `BYTEA PRIMARY KEY`, application-side SHA-256 |
| `amount_minor` is non-zero | `CHECK (amount_minor <> 0)` |

The three ledger invariants are enforced **twice** — once in the Go domain
(`ledger.Validate`) and once in PostgreSQL. This is not redundancy for its
own sake; it is defence in depth. A bug in a future handler cannot produce
an unbalanced ledger, because the database will reject it at COMMIT.

## What is not modelled

- **Multi-currency transactions.** Deferred. A EUR→USD conversion needs a
  rate, a base currency, and gain/loss accounts. Doing it wrong produces
  silently wrong books.
- **Balances as a stored column.** Balances are derived (`SUM(amount_minor)
  GROUP BY account_id`). A cached balance is a Phase 9 optimization, not
  a Phase 4 feature; if added, it must be reconcilable against the ledger.
- **Account hierarchy.** Assets, liabilities, etc., are flat. Sub-accounts
  (Checking under Assets) are a Phase 9 reporting concern.
- **Scheduled/future transactions.** `occurred_at` is a fact, not a
  schedule. Scheduling is a separate feature.