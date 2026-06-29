# Data Model: Admin Authentication

**Feature**: 001-admin-auth  
**Date**: 2026-04-20

Both tables are defined in `backend/migrations/001_initial_schema.up.sql`. No new migration is required for this feature.

---

## Entity: `admins`

Represents an operator account. Single row in MVP; table supports multiple rows for R2 multi-admin.

| Column | Type | Nullable | Notes |
|--------|------|----------|-------|
| `id` | `UUID` | No | PK, `gen_random_uuid()` |
| `email` | `TEXT` | No | Unique among non-deleted rows (partial unique index) |
| `password_hash` | `TEXT` | No | bcrypt hash, cost 12 |
| `name` | `TEXT` | No | Display name |
| `created_at` | `TIMESTAMPTZ` | No | Default `now()` |
| `updated_at` | `TIMESTAMPTZ` | No | Updated by trigger on every write |
| `deleted_at` | `TIMESTAMPTZ` | Yes | Null = active; soft-delete pattern |

**Indexes**:
- `admins_email_unique` — partial unique on `(email) WHERE deleted_at IS NULL` — used by login lookup

**Validation rules**:
- Email must be non-empty and pass RFC 5322 format check before DB lookup
- `deleted_at IS NULL` must be true for login and refresh to succeed

**Go struct**:
```go
type Admin struct {
    ID           uuid.UUID
    Email        string
    PasswordHash string
    Name         string
    CreatedAt    time.Time
    UpdatedAt    time.Time
    DeletedAt    *time.Time
}
```

---

## Entity: `refresh_tokens`

Server-side record of every issued refresh token. Enables revocation without a blacklist on access tokens.

| Column | Type | Nullable | Notes |
|--------|------|----------|-------|
| `id` | `UUID` | No | PK |
| `admin_id` | `UUID` | No | FK → `admins.id` ON DELETE CASCADE |
| `token_hash` | `TEXT` | No | `SHA-256(raw_refresh_jwt)`, hex-encoded; unique |
| `expires_at` | `TIMESTAMPTZ` | No | `now() + 7 days` at issuance |
| `created_at` | `TIMESTAMPTZ` | No | Default `now()` |
| `revoked_at` | `TIMESTAMPTZ` | Yes | Null = still valid; set by logout |

**Indexes**:
- `refresh_tokens_admin_id_idx` — B-tree on `admin_id` — used for cleanup queries (R2)

**Validation rules (service-layer)**:
- Token is valid iff: `revoked_at IS NULL AND expires_at > now()`
- `token_hash` lookup uses the SHA-256 of the raw cookie value; no constant-time comparison needed (hash is not sensitive by itself)

**Go struct**:
```go
type RefreshToken struct {
    ID        uuid.UUID
    AdminID   uuid.UUID
    TokenHash string
    ExpiresAt time.Time
    CreatedAt time.Time
    RevokedAt *time.Time
}
```

---

## Relationships

```
admins (1) ──── (0..N) refresh_tokens
```

- An admin can have multiple outstanding refresh tokens (e.g., logged in from two browsers simultaneously). All are invalidated when logout is called from either session — the logout endpoint revokes the specific token from the cookie, not all tokens for the admin.
- `ON DELETE CASCADE` on `refresh_tokens.admin_id` means hard-deleting an admin (not used in MVP — soft-delete only) would clean up tokens automatically.

---

## Not In Scope

The `sessions.created_by` column (FK → `admins.id`, nullable) was added to the migration as part of the data model update on 2026-04-20. It is populated during session creation (a future feature spec), not during auth. The auth package does not read or write this column.
