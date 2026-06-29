# Research: Admin Authentication

**Feature**: 001-admin-auth  
**Date**: 2026-04-20

No NEEDS CLARIFICATION markers were raised in the spec. This document records the key design decisions made during planning and the rationale behind each.

---

## Decision 1: Refresh Token Storage — SHA-256 hash only

**Decision**: Store `SHA-256(raw_refresh_jwt)` in `refresh_tokens.token_hash`. Never persist the raw token string.

**Rationale**: If the database is compromised, an attacker cannot directly use the token hashes to authenticate — they would need to reverse the hash (infeasible for SHA-256 over a high-entropy JWT). This is the standard approach for storing bearer tokens server-side (analogous to password hashing but lighter since JWTs are already high-entropy).

**Alternatives considered**:
- Store raw token: rejected — database breach = instant session hijack.
- Encrypt token (AES-GCM): rejected — adds complexity; encryption is reversible, so a key breach is equivalent to storing plaintext. SHA-256 is sufficient.
- Store token ID claim (jti): considered — would require adding a `jti` claim to the refresh JWT. Rejected for simplicity: hashing the full token string is equivalent and avoids an extra JWT claim to manage.

---

## Decision 2: Refresh Token is a JWT

**Decision**: The refresh token is a JWT signed with `JWT_SECRET`, with claims `{ sub: admin_id, role: "refresh", exp: +7days }`. The cookie value is the raw JWT string.

**Rationale**: The API design doc specifies "All tokens are JWTs signed with JWT_SECRET." Using a JWT for the refresh token means the server can do a cheap signature + expiry check before hitting the database, avoiding unnecessary DB reads for obviously invalid tokens (expired, wrong signature).

**Alternatives considered**:
- Opaque random token (crypto/rand): simpler to generate; would require the DB lookup for every refresh call (cannot pre-validate). Rejected for consistency with the existing token design.

---

## Decision 3: No Token Rotation in MVP

**Decision**: The refresh endpoint issues a new access token but does NOT invalidate the old refresh token. The same refresh JWT stays valid until natural expiry or explicit logout.

**Rationale**: Spec assumption explicitly out-of-scopes token rotation. For a single-admin MVP application, the complexity-to-security tradeoff is not justified. Logout covers the revocation path for intentional sessions.

**Alternatives considered**:
- Rotate on every refresh: would require updating `token_hash` in DB on each refresh call and handling the race condition (two concurrent refreshes — one succeeds, one fails). Deferred to R2.

---

## Decision 4: Cookie Scope — Path `/api/v1/auth/`

**Decision**: The `qz_refresh` cookie is scoped to `Path=/api/v1/auth/` so the browser only sends it to the auth endpoints, not to every API call.

**Rationale**: Minimises the exposure window of the refresh credential. If any non-auth endpoint is vulnerable to CSRF or a response-header injection, the refresh cookie is not exposed. Access tokens (Bearer header) are sent explicitly by application code — they are not cookies and are not subject to this scope.

**Additional cookie flags**: `HttpOnly=true`, `SameSite=Strict`, `Secure=true` in production. In development (HTTP), `Secure` can be `false` via an env flag.

---

## Decision 5: Concurrent Refresh — Non-Issue for MVP

**Decision**: Concurrent refresh requests with the same refresh token are allowed to both succeed in MVP (both return a valid new access token).

**Rationale**: Without token rotation, there is no state change on refresh — the refresh token remains valid. Two concurrent refreshes simply return two equal (or near-equal) access tokens. The risk (a very brief window where two valid access tokens exist) is acceptable for a single-admin MVP.

**Future**: When token rotation is introduced in R2, this becomes a write-once race condition that must be handled with a DB-level compare-and-swap (UPDATE ... WHERE revoked_at IS NULL RETURNING).

---

## Decision 6: Soft-Deleted Admin Handling

**Decision**: On login, check `WHERE email = $1 AND deleted_at IS NULL`. On refresh, look up the admin via the `admin_id` claim in the refresh JWT and check `deleted_at IS NULL` before issuing a new access token.

**Rationale**: An admin soft-deleted while logged in will have their existing access token remain valid for up to 15 minutes (natural expiry). On the next refresh attempt, the check fails and the admin is effectively locked out. This is the simplest correct approach without requiring access token blacklisting.

---

## Decision 7: bcrypt Cost Factor

**Decision**: Use bcrypt cost factor 12.

**Rationale**: At cost 12, bcrypt on modern hardware takes ~250–300ms per hash — high enough to slow brute-force attacks significantly, low enough that a login call completes well within the 2-second SLA (the rest of the login is DB lookups and JWT signing, each <5ms). Cost 10 (Go's default) is acceptable but 12 is the current OWASP recommendation for bcrypt.

---

## Decision 8: JWT Claims Structure

**Access token** (TTL: 15 min):
```json
{
  "iss": "<JWT_ISSUER env value>",
  "sub": "<admin_id UUID>",
  "role": "admin",
  "exp": <unix timestamp>,
  "iat": <unix timestamp>
}
```

**Refresh token** (TTL: 7 days):
```json
{
  "iss": "<JWT_ISSUER env value>",
  "sub": "<admin_id UUID>",
  "role": "refresh",
  "exp": <unix timestamp>,
  "iat": <unix timestamp>
}
```

**Rationale**: Keeping the role claim in the refresh token allows the middleware to quickly reject it if somehow presented to a non-refresh endpoint (wrong role → 403 rather than silently proceeding). The `iss` claim enables the issuer-agnostic middleware validation.

---

## Decision 9: Middleware Claims Injection

**Decision**: After successful JWT validation, the middleware injects a typed `Claims` struct into the Fiber request context using a typed key (not a string key) to prevent accidental shadowing.

```go
type contextKey struct{}
// Usage: c.Locals(contextKey{}, claims)
```

**Rationale**: Typed context keys prevent key collisions between packages and make accidental reads from wrong handlers impossible at compile time.
