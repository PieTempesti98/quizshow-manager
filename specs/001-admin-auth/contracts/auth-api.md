# API Contracts: Admin Authentication

**Feature**: 001-admin-auth  
**Date**: 2026-04-20  
**Base URL**: `/api/v1`  
**Response envelope**: all responses use `{ "data": ... }` on success or `{ "error": { "code": "...", "message": "..." } }` on failure.

---

## POST /api/v1/auth/login

Admin login. No authentication required.

### Request

```
Content-Type: application/json
```

```json
{
  "email": "admin@quizshow.local",
  "password": "secret"
}
```

**Validation**:
- `email`: non-empty, valid email format (RFC 5322)
- `password`: non-empty

### Responses

**200 OK** — credentials valid

```json
{
  "data": {
    "access_token": "<jwt>",
    "expires_at": "2026-04-20T10:45:00Z"
  }
}
```

Set-Cookie header (always accompanied with 200):
```
Set-Cookie: qz_refresh=<refresh_jwt>; Path=/api/v1/auth/; HttpOnly; SameSite=Strict; Secure; Max-Age=604800
```
(`Secure` flag omitted in development when `COOKIE_SECURE=false`)

**401 Unauthorized** — wrong credentials (email not found OR password mismatch — same response for both)

```json
{
  "error": {
    "code": "UNAUTHORIZED",
    "message": "Invalid credentials"
  }
}
```

**422 Unprocessable Entity** — validation failure

```json
{
  "error": {
    "code": "VALIDATION_ERROR",
    "message": "email: must be a valid email address"
  }
}
```

---

## POST /api/v1/auth/refresh

Exchange the `qz_refresh` httpOnly cookie for a new access token. No `Authorization` header required.

### Request

No body. The `qz_refresh` cookie must be present.

### Responses

**200 OK** — refresh successful

```json
{
  "data": {
    "access_token": "<jwt>",
    "expires_at": "2026-04-20T11:00:00Z"
  }
}
```

**401 Unauthorized** — cookie missing, JWT signature invalid, token expired, or token revoked in DB

```json
{
  "error": {
    "code": "UNAUTHORIZED",
    "message": "Refresh token is invalid or expired"
  }
}
```

---

## POST /api/v1/auth/logout

Revoke the current refresh token. Requires a valid admin access token.

### Request

```
Authorization: Bearer <access_token>
```

No body. The `qz_refresh` cookie must be present.

### Responses

**200 OK** — logout successful

```json
{
  "data": { "ok": true }
}
```

Set-Cookie header (always accompanied with 200 to clear the cookie):
```
Set-Cookie: qz_refresh=; Path=/api/v1/auth/; HttpOnly; SameSite=Strict; Max-Age=0
```

**401 Unauthorized** — missing or invalid access token, or missing refresh cookie

```json
{
  "error": {
    "code": "UNAUTHORIZED",
    "message": "Missing or invalid token"
  }
}
```

---

## Auth Middleware Contract

The `RequireAdmin` Fiber middleware guards all admin endpoints. It is applied at the route group level to all routes under `/api/v1/` except `/auth/login` and `/auth/refresh`.

### Behaviour

1. Read the `Authorization` header. If absent → 401.
2. Extract the Bearer token. If malformed → 401.
3. Parse and validate the JWT:
   - Signature must be valid (signed with `JWT_SECRET`)
   - `iss` claim must equal `JWT_ISSUER` env value
   - `exp` claim must be in the future
   - If any check fails → 401.
4. Check the `role` claim. If not `"admin"` → 403.
5. Inject the parsed claims into the Fiber context for downstream handlers.

### Claims injected into context

```go
// Accessible in handlers via:
claims := c.Locals(auth.ClaimsKey).(auth.AdminClaims)
```

```go
type AdminClaims struct {
    AdminID uuid.UUID // from "sub" claim
    Role    string    // always "admin" past the middleware
    Issuer  string    // from "iss" claim
}
```

### Error responses

**401** — missing header, malformed token, invalid signature, expired token, wrong issuer

```json
{
  "error": {
    "code": "UNAUTHORIZED",
    "message": "Missing or invalid token"
  }
}
```

**403** — valid token but role is not `admin`

```json
{
  "error": {
    "code": "FORBIDDEN",
    "message": "Insufficient permissions"
  }
}
```

---

## Environment Variables

| Variable | Required | Description |
|----------|----------|-------------|
| `JWT_SECRET` | Yes | HMAC-SHA256 signing key for all JWTs. Min 32 bytes recommended. |
| `JWT_ISSUER` | Yes | Expected `iss` claim value (e.g., `https://quizshow.local`). Server refuses to start if unset. |
| `COOKIE_SECURE` | No | Set to `false` in local dev to allow cookies over HTTP. Defaults to `true`. |
| `DATABASE_URL` | Yes | PostgreSQL connection string (`postgres://user:pass@host:port/db`) |
