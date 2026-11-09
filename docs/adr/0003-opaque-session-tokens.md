# ADR-0003: Opaque session tokens instead of JWT

- **Status:** Accepted
- **Date:** 2026-11-10
- **Deciders:** project author

## Context

The API needs to authenticate requests. Two broad approaches exist:

- **Signed tokens (JWT, PASETO).** The client holds a token that the
  server can verify without a database lookup. The payload is visible to
  the client (base64-encoded, not encrypted, unless JWE). Revocation is
  difficult — once issued, a JWT is valid until its `exp`.
- **Opaque tokens.** The client holds an arbitrary string. The server
  looks it up in storage on every request. Revocation is a `DELETE`.

Constraints for this project:

- Single-service backend, single database.
- Learning goal: understand the mechanics of session management, not
  the mechanics of JWT signing.
- The deployment target is a single container; there is no second service
  that would need to verify tokens without a shared database.
- Session revocation is a hard requirement (a user must be able to log
  out, and an administrator must be able to invalidate a compromised
  session).

## Decision

We will use **opaque session tokens**:

- The token is **32 random bytes**, hex-encoded (64 characters). Generated
  by `crypto/rand`, unique across calls.
- The database stores only `SHA-256(token)`, never the token itself.
- A `sessions` table maps `token_hash → user_id, created_at, expires_at`.
- Lookup on every request: hash the presented token, `SELECT ... WHERE
  token_hash = $1 AND expires_at > NOW()`.
- Expiry is 24 hours from issuance. A session can be revoked by
  `DELETE FROM sessions WHERE token_hash = $1`, effective immediately.
- The token is transmitted as `Authorization: Bearer <token>`.

## Consequences

### Positive

- **Revocation is immediate.** A single `DELETE` invalidates a session.
  There is no window during which a compromised JWT remains valid.
- **A database leak does not expose live sessions.** The token hash is
  stored, not the token. SHA-256 of a 256-bit random value is
  preimage-resistant: an attacker cannot recover the token from the
  hash.
- **No signing keys to manage.** There is no `JWT_SECRET` to rotate, no
  algorithm confusion vulnerability, no key-ID rotation dance. Key
  management is a source of production incidents; opaque tokens avoid
  it entirely for this project's scale.
- **Token contents are private by construction.** Even if a token is
  logged in transit (which we avoid, but assume happens), it reveals
  nothing about the user. A JWT payload is base64 (readable).
- **The implementation matches the learning goal.** Understanding
  "hash the token, look it up" teaches the shape of session management.
  Understanding JWT requires learning signing algorithms, claim
  validation, and revocation workarounds — valuable but orthogonal.

### Negative

- **A database lookup per request.** Every authenticated request issues
  a `SELECT` on `sessions`. At this project's scale, this is not a
  bottleneck. If it ever becomes one, a cache (Redis, Phase 8/9) would
  sit in front. Adding a cache introduces the cache-invalidation problem,
  which is genuinely hard; we defer it until measurement justifies it.
- **Stateful.** The service is not stateless; a session is a row. This
  means horizontal scaling requires all instances to share the sessions
  table (they do, via the shared PostgreSQL). It also means session
  cleanup (deletion of expired rows) is a background job, not optional
  housekeeping. That job arrives in Phase 8.
- **No offline verification.** A third-party service cannot verify a
  session token without calling our API. This is not a requirement
  here, but it would be if we ever needed a second service to trust
  tokens without a shared database.

### Alternatives considered

- **JWT with a short expiry and a revocation list.** Partially solves
  revocation (short `exp` limits the window) but requires a revocation
  list in storage anyway — which is the same state, plus a signature
  check. Strictly more complex than opaque tokens for the same
  guarantees.
- **JWT with no revocation.** Rejected: "you cannot log out" is a
  security defect.
- **Cookies with `HttpOnly`, `Secure`, `SameSite`.** The right answer for
  a browser-based single-page app. Rejected here because the API is
  intended for non-browser clients (mobile, scripts), where the
  `Authorization` header is idiomatic. A browser client could be added
  later with a parallel cookie-based flow.
- **HTTP Basic with the user's password on every request.** Rejected:
  sends the password on every request, requires the server to verify
  the bcrypt hash on every request (60ms+), and cannot be revoked
  without changing the password.