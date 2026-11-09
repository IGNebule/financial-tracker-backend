# ADR-0004: Integer minor units for money

- **Status:** Accepted
- **Date:** 2026-11-10
- **Deciders:** project author

## Context

IEEE 754 floating-point numbers cannot represent most decimal fractions
exactly. `0.1 + 0.2 == 0.30000000000000004` in `float64`. For a single
arithmetic operation, the error is invisible; for a ledger summing
thousands of transactions, the errors accumulate, and balances stop
reconciling.

Financial software has known this for decades, but the mistake still
appears in tutorials and is easy to reproduce accidentally.

## Decision

All monetary amounts are stored and computed as **signed 64-bit integers
in minor units**. For EUR and USD, one minor unit is one cent; the amount
`12.34 EUR` is stored as `1234`. For a hypothetical 3-decimal currency,
one minor unit would be one thousandth; the invariant is that the
**stored integer times the currency's minor-unit scale equals the major
amount**, not that it is always "cents".

- The domain (`internal/money`) accepts `int64` and a currency code.
- The database stores `amount_minor BIGINT` alongside `currency TEXT`.
- Currency is **always** explicit; the code never assumes two decimal
  places without checking.
- Display formatting goes through `money.Format(int64, currency string)`,
  which knows the per-currency exponent (currently 2 for all supported
  currencies; the table will grow for JPY, which has 0, and BHD, which
  has 3, when they are supported).
- The HTTP API accepts and returns `amount_minor` as an integer in JSON,
  not a decimal string. This is deliberate: JSON has no decimal type
  either, and turning a decimal string into a float silently loses
  precision on every intermediary.

## Consequences

### Positive

- **Exact arithmetic.** Addition, subtraction, and comparison of amounts
  in the same currency are exact. No `0.00000000000001` residue.
- **The domain knows its units.** `int64` cannot be accidentally
  interpreted as a float. A future contributor who writes
  `amount / 100.0` to display it will get a compile error, not a silent
  bug — `int64` divided by `float64` does not typecheck.
- **The database stores what the code stores.** `BIGINT` maps to `int64`
  with no conversion, and PostgreSQL sums `BIGINT` exactly.
- **The API is honest about precision.** A client that needs more
  decimal places than the currency has cannot express the amount.
  Precision is bounded by currency, not by transport.

### Negative

- **Every boundary needs explicit conversion.** A mobile app that
  shows "€12.34" must do `amount_minor / 100` at display time. If the
  app is written in JavaScript, it must not use `Number` for amounts
  (which is a `float64`); it must use `BigInt` or a decimal library.
  This is documented for future client developers.
- **JSON does not have an integer type with a defined width.** A number
  larger than 2^53 (about 9 quadrillion) loses precision in JavaScript
  clients that parse JSON with `JSON.parse` into `Number`. The practical
  limit is far beyond any real transaction, but the risk exists for
  amounts above 90 trillion EUR — every JavaScript client must use
  `BigInt` or a streaming parser for such values, or the API must return
  amounts as strings above a threshold. **Current code does not implement
  such a threshold**; if a use case appears, it is a schema-compatible
  change (the JSON string form is a superset of the integer form).
- **Overflow.** `ledger.Validate` sums `int64` values without checking
  for overflow. A transaction with three postings near `math.MaxInt64`
  can wrap to zero in two's complement, producing a **false positive**
  (balanced according to the check but not mathematically). This was
  observed in practice. Mitigation: the handler will cap `amount_minor`
  at a sane maximum (Phase 4e). The domain's `Validate` is not the
  appropriate place for the overflow check — the trust boundary is the
  HTTP input, not the internal call.
- **Slightly less readable in debugging.** A `psql` session shows
  `amount_minor = 1234`, not `12.34`. The `currency` column and context
  make the value interpretable, but it is not human-friendly at a
  glance. This is a small cost.

### Alternatives considered

- **`float64` for amounts.** Rejected: the exact failure mode
  (accumulating rounding error) is the reason financial systems use
  integers. The whole point of this decision is to make it hard to
  reintroduce that bug.
- **`NUMERIC`/`DECIMAL` in PostgreSQL.** Considered: PostgreSQL's
  `NUMERIC` is exact and stores decimal values natively. Rejected for
  the storage layer because it does not solve the *Go* side — the Go
  type would be `pgtype.Numeric` or a string, and every arithmetic
  operation would need a decimal library (e.g. `shopspring/decimal`).
  That adds a dependency and a runtime for every computation. Since
  every currency currently has at most 2 decimals, integers are strictly
  simpler and equally correct. **If a currency with a non-2 exponent is
  added, revisit this decision** — the `money` package's currency table
  is the natural place to make the change.
- **Decimal strings in the API (`"12.34"`).** Considered: avoids the
  JSON integer precision edge and is human-readable. Rejected because
  the server must parse the string either way, and any client that
  later does `parseFloat` will lose precision in exactly the same way.
  Sending `int64` is honest: the client sees the integer and cannot
  pretend it is a decimal.
- **A money library (`shopspring/decimal`, `Rhymond/go-money`).**
  Considered for the domain package. Rejected because the domain needs
  only formatting and (eventually) currency-aware scaling; `int64` plus
  the currency table in `internal/money` does both without a
  dependency. If arithmetic ever needs to be currency-aware (e.g. adding
  EUR to USD requires a rate), a library would help; that day has not
  arrived.