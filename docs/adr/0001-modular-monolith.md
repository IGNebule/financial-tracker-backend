# ADR-0001: Modular monolith over microservices

- **Status:** Accepted
- **Date:** 2026-11-10
- **Deciders:** project author

## Context

The project is a personal financial-tracking backend, built as a learning
exercise in Go, PostgreSQL, and production-oriented practices. The brief
mentions microservices, gRPC, and distributed systems as long-term goals,
but requires they be justified by a real operational need rather than
adopted for their own sake.

Constraints:

- One developer, one local machine, one database.
- Deployment target is Hugging Face Docker Spaces (a single container).
- Correctness of the ledger matters more than horizontal scale.
- Every pattern added must have a clear problem it solves.

## Decision

We will build a **modular monolith**: a single Go process with a single
PostgreSQL database, organized into clearly bounded packages. Internal
boundaries are enforced by **package structure** (`internal/`) and by
**dependency direction** (domain ← adapters ← transport ← composition
root), not by network calls.

A gRPC boundary or a separate service will be introduced only when there
is a measured cost to keeping the concern in the same process — for
example, a need to scale a specific operation independently, or a
regulatory requirement for physical isolation.

## Consequences

### Positive

- **Deployment is a single container.** One image, one environment, one
  health check, one log stream. Hugging Face Spaces can run it without
  additional infrastructure.
- **Transactions are real.** A multi-table write uses PostgreSQL's own
  transactional semantics, not a saga. The deferred balance trigger fires
  at COMMIT of a single logical operation.
- **Debugging is local.** A stack trace shows the full path from HTTP to
  SQL. No distributed traces, no service-to-service correlation IDs, no
  cross-process clock skew.
- **Learning value is concentrated on the hard problems.** The hard
  problems in this project are ledger correctness, idempotency, and
  concurrency. Those are hard whether or not services are distributed.
  Splitting into services would add distributed-systems complexity
  without making the core problems easier to learn.

### Negative

- **Scaling is per-process, not per-service.** If accounts queries become
  hot while transactions are cold, both scale together. Mitigation:
  PostgreSQL read replicas (Phase 9) if a real need appears.
- **Internal boundaries can erode.** Without a network boundary forcing
  discipline, a future contributor could `import` a repository from a
  handler and bypass the domain. Mitigation: the "domain does not import
  adapters" rule is documented in `CLAUDE.md` and enforced by directory
  structure; a linter rule would enforce it more strongly, and could be
  added if the project ever has more than one regular contributor.
- **The extraction path is real work later.** If a service must be
  split, the domain would move to a new binary and adapters would need
  to be re-wired. The work is bounded (days), not months, because the
  domain has no dependency on infrastructure.

### Alternatives considered

- **Microservices from day one.** Rejected: no measurable operational
  benefit at current scale, and distributed systems introduce failure
  modes (partial failure, network partitions) that are not part of the
  learning goal at this stage.
- **A "serverless" split (Lambda/Functions).** Rejected: the
  double-entry ledger depends on multi-statement transactions with
  deferred constraint checking at COMMIT. Serverless SQL often lacks
  session affinity or long-lived transactions, and cold starts would
  degrade a financial API's p99 latency.
- **A monolith with a message queue.** Rejected *for now*: no async
  operations justify a queue. Phase 7 (webhooks) will add an outbox
  table, which is not a queue — see that ADR when it exists.