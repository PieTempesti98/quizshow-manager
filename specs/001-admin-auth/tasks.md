# Tasks: Admin Authentication

**Input**: Design documents from `/specs/001-admin-auth/`  
**Prerequisites**: plan.md ✓, spec.md ✓, research.md ✓, data-model.md ✓, contracts/auth-api.md ✓, quickstart.md ✓

**Tests**: Unit tests for token helpers and service logic; integration tests for repositories. No HTTP handler tests (deferred to R2 per constitution §6.3).

**Organization**: Tasks grouped by user story to enable independent implementation and testing.

## Format: `[ID] [P?] [Story] Description`

- **[P]**: Can run in parallel (different files, no blocking dependencies)
- **[Story]**: Which user story this task belongs to (US1–US4)

---

## Phase 1: Setup (Shared Infrastructure)

**Purpose**: Create the directory structure and document the environment contract before writing any Go code.

- [x] T001 Create directory tree: `backend/internal/auth/`, `backend/internal/db/`, `backend/internal/api/`, `backend/cmd/server/`
- [x] T002 [P] Create `backend/.env.example` documenting all required env vars: `JWT_SECRET`, `JWT_ISSUER`, `DATABASE_URL`, `COOKIE_SECURE`, `PORT`

---

## Phase 2: Foundational (Blocking Prerequisites)

**Purpose**: Core shared infrastructure that MUST be complete before any user story can compile or run.

**⚠️ CRITICAL**: No user story work can begin until this phase is complete.

- [x] T003 Implement pgx connection pool constructor + ping-on-startup in `backend/internal/db/db.go`
- [x] T004 [P] Implement `DataResponse` and `ErrorResponse` JSON helpers (matching `{"data":…}` / `{"error":{"code":"…","message":"…"}}` envelope) in `backend/internal/api/envelope.go`
- [x] T005 [P] Implement `AuthConfig` struct in `backend/internal/auth/config.go` — reads `JWT_SECRET`, `JWT_ISSUER`, `ACCESS_TTL` (15m), `REFRESH_TTL` (7d), `COOKIE_SECURE` from env; exits with error if `JWT_SECRET` or `JWT_ISSUER` are unset
- [x] T006 [P] Define `Admin` and `RefreshToken` Go structs (matching `admins` and `refresh_tokens` table columns) in `backend/internal/auth/models.go`
- [x] T007 Bootstrap Fiber app in `backend/cmd/server/main.go` — load `AuthConfig`, connect DB pool, mount `/api/v1/auth` route group, configure graceful shutdown; no individual routes yet

**Checkpoint**: Foundation ready — all user story phases can now begin.

---

## Phase 3: User Story 1 — Admin Login (Priority: P1) 🎯 MVP

**Goal**: A valid admin can POST to `/api/v1/auth/login` and receive a 15-min access token in the response body plus a 7-day `qz_refresh` httpOnly cookie. Invalid credentials return a generic 401.

**Independent Test**: `POST /api/v1/auth/login` with valid credentials → 200 + `access_token` + `Set-Cookie: qz_refresh`. Invalid credentials → 401 `UNAUTHORIZED` (same message for wrong email or wrong password). Empty/malformed email → 422.

- [x] T008 [P] [US1] Implement `IssueAccessToken(adminID, config)` and `IssueRefreshToken(adminID, config)` JWT constructors (claims: `iss`, `sub`, `role`, `iat`, `exp`) in `backend/internal/auth/token.go`
- [x] T009 [P] [US1] Implement `AdminRepository` struct and `FindByEmail(ctx, email)` method (pgx query on `admins` where `deleted_at IS NULL`) in `backend/internal/auth/repository.go`
- [x] T010 [US1] Add `RefreshTokenRepository` struct and `Create(ctx, adminID, tokenHash, expiresAt)` method to `backend/internal/auth/repository.go`
- [x] T011 [US1] Define `AuthService` interface and implement `Login(ctx, email, password)` in `backend/internal/auth/service.go` — bcrypt compare (cost 12), issue both tokens, compute SHA-256 hash of refresh JWT, store hash via repository, return tokens
- [x] T012 [US1] Implement `LoginHandler` in `backend/internal/auth/handler.go` — validate request body, call service, write `access_token` + `expires_at` to JSON response, set `qz_refresh` cookie with `HttpOnly`, `SameSite=Strict`, `Secure` (from config), `Path=/api/v1/auth/`, `Max-Age=604800`
- [x] T013 [US1] Register `POST /api/v1/auth/login` → `LoginHandler` in `backend/cmd/server/main.go`
- [x] T014 [P] [US1] Write unit tests for `IssueAccessToken` and `IssueRefreshToken` in `backend/internal/auth/token_test.go` — verify claims (`iss`, `sub`, `role`, `exp` within tolerance), verify signature validation, verify expired token rejection
- [x] T015 [US1] Write integration tests for `AdminRepository.FindByEmail` and `RefreshTokenRepository.Create` against real PostgreSQL in `backend/internal/auth/repository_test.go`

**Checkpoint**: `POST /api/v1/auth/login` is fully functional. Login, wrong-password, and validation-error flows all work.

---

## Phase 4: User Story 2 — Transparent Token Refresh (Priority: P1)

**Goal**: A client holding a valid non-revoked `qz_refresh` cookie can POST to `/api/v1/auth/refresh` and receive a fresh access token without re-entering credentials.

**Independent Test**: Login → wait for access token to expire (or shorten TTL in test) → `POST /api/v1/auth/refresh` with cookie → 200 + new `access_token`. Then manually revoke the refresh token in DB and retry → 401.

- [x] T016 [US2] Add `RefreshTokenRepository.FindByHash(ctx, hash)` method to `backend/internal/auth/repository.go` — returns token record only if `revoked_at IS NULL AND expires_at > now()`
- [x] T017 [US2] Implement `AuthService.Refresh(ctx, rawRefreshJWT)` in `backend/internal/auth/service.go` — parse + verify JWT signature/expiry/issuer, SHA-256 hash the raw token, call `FindByHash`, look up admin (check `deleted_at IS NULL`), issue and return new access token
- [x] T018 [US2] Implement `RefreshHandler` in `backend/internal/auth/handler.go` — read `qz_refresh` cookie, call service, return new `access_token` + `expires_at`; 401 if cookie missing or service returns error
- [x] T019 [US2] Register `POST /api/v1/auth/refresh` → `RefreshHandler` in `backend/cmd/server/main.go`
- [x] T020 [P] [US2] Write unit tests for `AuthService.Refresh()` in `backend/internal/auth/service_test.go` — valid token, expired JWT, revoked DB record, soft-deleted admin
- [x] T021 [US2] Add integration tests for `RefreshTokenRepository.FindByHash` (valid, expired, revoked) in `backend/internal/auth/repository_test.go`

**Checkpoint**: Token refresh flow works end-to-end. Admin can work beyond 15-min access token TTL without logging in again.

---

## Phase 5: User Story 4 — Auth Middleware (Priority: P1)

**Goal**: All admin endpoints (outside `/auth/login` and `/auth/refresh`) require a valid `admin`-role Bearer token. Wrong/missing/expired tokens → 401. Wrong role → 403. Claims injected into Fiber context.

**Independent Test**: Call any admin route (e.g., `GET /api/v1/categories` once implemented) with: no header → 401; expired token → 401; player-role token → 403; valid admin token → 200/404 (not auth error).

- [x] T022 [US4] Add `ValidateClaims(tokenString, config)`, `AdminClaims` struct, and typed `ClaimsKey` context key to `backend/internal/auth/token.go`
- [x] T023 [US4] Implement `RequireAdmin` Fiber middleware in `backend/internal/auth/middleware.go` — extract Bearer token, call `ValidateClaims`, check `iss == JWT_ISSUER`, check `role == "admin"`, inject `AdminClaims` via `c.Locals(ClaimsKey, claims)`, return 401/403 with standard error envelope on failure
- [x] T024 [US4] Apply `RequireAdmin` middleware to the admin route group in `backend/cmd/server/main.go` — `/api/v1/` group (excluding `/auth/login` and `/auth/refresh` which stay public)
- [x] T025 [P] [US4] Write unit tests for `RequireAdmin` in `backend/internal/auth/middleware_test.go` — no header, malformed token, expired token, wrong issuer, player-role token, valid admin token (verify claims injected)

**Checkpoint**: All admin routes are protected. The auth system is fully operational for other features to build on.

---

## Phase 6: User Story 3 — Logout with Server-Side Invalidation (Priority: P2)

**Goal**: An authenticated admin can POST to `/api/v1/auth/logout` to revoke their refresh token in the database and clear the cookie. Subsequent refresh attempts with the same cookie return 401.

**Independent Test**: Login → save cookie → logout → attempt `POST /api/v1/auth/refresh` with saved cookie → must return 401. Access token remains valid (not blacklisted) until natural expiry.

- [x] T026 [US3] Add `RefreshTokenRepository.Revoke(ctx, hash)` method to `backend/internal/auth/repository.go` — sets `revoked_at = now()` where `token_hash = $1 AND revoked_at IS NULL`
- [x] T027 [US3] Implement `AuthService.Logout(ctx, rawRefreshJWT)` in `backend/internal/auth/service.go` — hash the cookie value, call `Revoke`; no error if already revoked (idempotent)
- [x] T028 [US3] Implement `LogoutHandler` in `backend/internal/auth/handler.go` — read `qz_refresh` cookie, call service, clear cookie (`Max-Age=0`), return `{"data":{"ok":true}}`; requires `RequireAdmin` middleware (Bearer token)
- [x] T029 [US3] Register `POST /api/v1/auth/logout` → `LogoutHandler` (inside the `RequireAdmin`-protected group) in `backend/cmd/server/main.go`
- [x] T030 [P] [US3] Write unit tests for `AuthService.Logout()` in `backend/internal/auth/service_test.go` — valid revocation, already-revoked token (idempotent), missing cookie
- [x] T031 [US3] Add integration test for full revocation cycle: Create → FindByHash (valid) → Revoke → FindByHash (nil/not found) in `backend/internal/auth/repository_test.go`

**Checkpoint**: All four user stories complete. Full auth lifecycle — login → refresh → protected access → logout — works end-to-end.

---

## Phase 7: Polish & Cross-Cutting Concerns

- [x] T032 [P] Audit all error responses in `backend/internal/auth/handler.go` — verify every non-2xx path returns the standard `{"error":{"code":"…","message":"…"}}` envelope with the correct HTTP status code per `contracts/auth-api.md`
- [x] T033 [P] Verify fail-fast startup behaviour in `backend/internal/auth/config.go` — confirm server exits with a clear message when `JWT_SECRET` or `JWT_ISSUER` are absent; add test to `backend/internal/auth/config_test.go`
- [x] T034 Validate `quickstart.md` end-to-end: run migration, insert seed admin, login, refresh, logout, confirm revoked refresh returns 401

---

## Dependencies & Execution Order

### Phase Dependencies

- **Phase 1 (Setup)**: No dependencies — start immediately
- **Phase 2 (Foundational)**: Depends on Phase 1 — **BLOCKS all user stories**
- **Phase 3 (US1 Login)**: Depends on Phase 2
- **Phase 4 (US2 Refresh)**: Depends on Phase 3 (needs repository + service from US1)
- **Phase 5 (US4 Middleware)**: Depends on Phase 3 (`token.go` issue functions must exist before `ValidateClaims` is added)
- **Phase 6 (US3 Logout)**: Depends on Phases 3–5 (logout handler requires `RequireAdmin` middleware; revoke uses same repository file)
- **Phase 7 (Polish)**: Depends on all prior phases

### User Story Dependencies

- **US1 (Login)**: No story dependencies — first to implement
- **US2 (Refresh)**: Depends on US1 repository infrastructure (`FindByHash` extends the same file)
- **US4 (Middleware)**: Depends on US1 token infrastructure (`ValidateClaims` extends `token.go`)
- **US3 (Logout)**: Depends on US1 + US4 (logout route sits inside the middleware-protected group)

### Within Each Story

- Repository methods before service methods
- Service methods before handlers
- Handlers before route registration
- Unit tests can be written alongside or immediately after source code (not required to be written first — no TDD mandate in spec)
- Integration tests can run after repository implementation

### Parallel Opportunities

Within Phase 2: T003, T004, T005, T006 can all run in parallel (different files).  
Within Phase 3: T008 (token.go) and T009 (AdminRepository) can run in parallel — different concerns in the same file requires care; split by method if pair-programming.  
Within Phase 5: T022 and T023 can run in parallel (different files).

---

## Parallel Example: Phase 2 (Foundational)

```text
# All four can start simultaneously (different files, no inter-dependencies):
T003 → backend/internal/db/db.go
T004 → backend/internal/api/envelope.go
T005 → backend/internal/auth/config.go
T006 → backend/internal/auth/models.go

# T007 (main.go bootstrap) depends on T003 + T005 completing first.
```

## Parallel Example: Phase 3 (US1)

```text
# T008 and T009 can start in parallel after Phase 2:
T008 → backend/internal/auth/token.go  (IssueAccessToken, IssueRefreshToken)
T009 → backend/internal/auth/repository.go  (AdminRepository.FindByEmail)

# T010 must wait for T009 (same file):
T010 → backend/internal/auth/repository.go  (add RefreshTokenRepository.Create)

# T011 waits for T008 + T010:
T011 → backend/internal/auth/service.go  (AuthService.Login)

# T012 waits for T011; T014 can run in parallel with T012 (different file):
T012 → backend/internal/auth/handler.go  (LoginHandler)
T014 → backend/internal/auth/token_test.go  (unit tests — parallel with T012)
```

---

## Implementation Strategy

### MVP First (US1 Login only)

1. Complete Phase 1 (Setup)
2. Complete Phase 2 (Foundational) — T003–T007
3. Complete Phase 3 (US1 Login) — T008–T015
4. **STOP and VALIDATE**: `POST /api/v1/auth/login` works; unit + integration tests pass
5. Other features can now consume `RequireAdmin` once Phase 5 completes

### Incremental Delivery

1. Setup + Foundational → package compiles, server starts
2. US1 (Login) → login endpoint live; tokens issued
3. US2 (Refresh) → seamless session continuation
4. US4 (Middleware) → admin endpoints protectable; unblocks all other feature work
5. US3 (Logout) → full auth lifecycle complete

---

## Summary

| Phase | Stories | Tasks | Parallelizable |
|-------|---------|-------|----------------|
| 1 Setup | — | 2 | 1 |
| 2 Foundational | — | 5 | 4 |
| 3 US1 Login (P1) | US1 | 8 | 3 |
| 4 US2 Refresh (P1) | US2 | 6 | 1 |
| 5 US4 Middleware (P1) | US4 | 4 | 2 |
| 6 US3 Logout (P2) | US3 | 6 | 1 |
| 7 Polish | — | 3 | 2 |
| **Total** | | **34** | **14** |
