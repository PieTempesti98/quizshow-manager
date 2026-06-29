# Implementation Plan: Admin Authentication

**Branch**: `001-admin-auth` | **Date**: 2026-04-20 | **Spec**: [spec.md](spec.md)  
**Input**: Feature specification from `/specs/001-admin-auth/spec.md`

## Summary

Implement admin authentication for the QuizShow backend: login with email/password (US-A01), token refresh and logout with server-side revocation (US-A02), and a JWT auth middleware that guards all admin endpoints. Access tokens are JWTs (15-min TTL, `admin` role claim). Refresh tokens are JWTs (7-day TTL) stored as SHA-256 hashes in `refresh_tokens` and delivered via an httpOnly cookie named `qz_refresh`. The middleware reads the accepted JWT issuer from `JWT_ISSUER` env — no code change needed to swap to Keycloak in R2.

## Technical Context

**Language/Version**: Go 1.25  
**Primary Dependencies**: gofiber/fiber v2, golang-jwt/jwt v5, jackc/pgx v5, google/uuid v1, golang.org/x/crypto (bcrypt — **missing from go.mod, must be added**)  
**Storage**: PostgreSQL — tables `admins`, `refresh_tokens` (migration 001 already covers both)  
**Testing**: Go test — unit tests for token helpers and service logic; integration tests for repositories against real PostgreSQL  
**Target Platform**: Linux server (Docker Compose)  
**Project Type**: REST API web service  
**Performance Goals**: Login response < 2s; auth middleware overhead < 5ms p99  
**Constraints**: No ORM, explicit pgx SQL; dependency injection (no global state); no handler tests in MVP  
**Scale/Scope**: Single admin account in MVP; table supports multiple rows for R2

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

| Rule | Status | Notes |
|------|--------|-------|
| 1.1 Spec-first | ✅ PASS | `specs/001-admin-auth/spec.md` exists |
| 1.2 MVP discipline | ✅ PASS | Token rotation, multi-admin, Keycloak all explicitly out of scope |
| 2.1 Package structure | ✅ PASS | All code in `internal/auth/`; `cmd/server/` only wires routes |
| 2.2 No global state | ✅ PASS | Config, DB pool, and services injected via constructors |
| 2.3 Database access | ✅ PASS | pgx directly; SQL in repository structs |
| 2.4 Migrations | ✅ PASS | `001_initial_schema` already contains `admins` + `refresh_tokens`; no new migration needed |
| 2.6 Auth middleware | ✅ PASS | Stateless JWT validation; `JWT_ISSUER` from env |
| 2.7 Preferred libraries | ⚠️ GAP | `golang.org/x/crypto` (bcrypt) missing from `go.mod` — must run `go get` before implementation |
| 4.2 Versioning | ✅ PASS | All routes under `/api/v1/auth/` |
| 4.3 Response envelope | ✅ PASS | All handlers return `{ data }` or `{ error }` |
| 4.4 Authentication | ✅ PASS | Bearer token; 401 for missing/expired; 403 for wrong role |
| 5.1 UUIDs | ✅ PASS | All IDs use google/uuid |
| 5.2 Soft deletes | ✅ PASS | `admins.deleted_at` checked before issuing tokens |
| 5.3 UTC timestamps | ✅ PASS | All timestamps stored/returned UTC ISO 8601 |
| 6.1 Unit tests | ✅ PASS | Token helpers + service logic covered |
| 6.2 Integration tests | ✅ PASS | Repository functions tested against real PostgreSQL |
| 6.3 No handler tests | ✅ PASS | HTTP handler tests deferred to R2 |

**Post-design re-check**: pending (will revisit after Phase 1 contracts).

## Project Structure

### Documentation (this feature)

```text
specs/001-admin-auth/
├── plan.md              # This file
├── research.md          # Phase 0 — decisions and rationale
├── data-model.md        # Phase 1 — auth entities
├── quickstart.md        # Phase 1 — dev setup
├── contracts/
│   └── auth-api.md      # Phase 1 — endpoint + middleware contracts
└── tasks.md             # Phase 2 — /speckit.tasks output
```

### Source Code

```text
backend/
├── cmd/
│   └── server/
│       └── main.go                   # wire: config, DB pool, Fiber app, auth routes
├── internal/
│   └── auth/
│       ├── config.go                 # AuthConfig struct (JWT_SECRET, JWT_ISSUER, TTLs)
│       ├── token.go                  # IssueAccessToken, IssueRefreshToken, ValidateClaims
│       ├── repository.go             # AdminRepository + RefreshTokenRepository (pgx)
│       ├── service.go                # AuthService interface + Login/Refresh/Logout logic
│       ├── handler.go                # Fiber handlers: POST /login, /refresh, /logout
│       ├── middleware.go             # RequireAdmin fiber middleware
│       └── auth_test.go              # unit tests (token helpers, service)
├── internal/
│   └── auth/
│       └── integration_test.go       # integration tests (repositories vs real PG)
├── migrations/
│   ├── 001_initial_schema.up.sql     ✓ already exists (admins + refresh_tokens included)
│   └── 001_initial_schema.down.sql   ✓ already exists
├── go.mod                            ✓ exists — needs `go get golang.org/x/crypto`
└── go.sum                            ✓ exists
```

**Structure Decision**: Backend-only feature, single `internal/auth` package. No frontend work (blocked per CLAUDE.md until `docs/06-ui-flows.md` exists). The `cmd/server/main.go` is the only file outside `internal/` touched by this feature.

## Complexity Tracking

No constitution violations requiring justification.
