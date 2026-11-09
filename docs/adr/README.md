# Architecture Decision Records

Lightweight records of decisions that shape the codebase. Each ADR captures
a decision that was non-obvious at the time, so a future contributor can
understand the "why" without reconstructing it from the diff.

## Format

Standard Nygard-style: Context, Decision, Consequences. Consequences are
split into positive, negative, and alternatives considered.

## Statuses

- **Proposed** — under discussion.
- **Accepted** — current direction; changes require a new ADR that
  supersedes this one.
- **Superseded by ADR-NNNN** — replaced; kept for historical context.
- **Deprecated** — no longer applies, but the code it justified has not
  been removed.

## Index

- [ADR-0001](0001-modular-monolith.md) — Modular monolith over microservices
- [ADR-0002](0002-double-entry-ledger.md) — Double-entry ledger with
  database-enforced invariants
- [ADR-0003](0003-opaque-session-tokens.md) — Opaque session tokens
  instead of JWT
- [ADR-0004](0004-integer-minor-units.md) — Integer minor units for money

## Creating a new ADR

1. Copy `0000-template.md`.
2. Renumber and rename: `NNNN-short-title.md`.
3. Add to this index.
4. Commit as `docs: add ADR-NNNN <title>`.