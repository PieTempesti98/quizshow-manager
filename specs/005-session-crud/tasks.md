# Tasks: Session Create and Configure

**Input**: Design documents from `/specs/005-session-crud/`  
**Prerequisites**: plan.md ✅, spec.md ✅, research.md ✅, data-model.md ✅, contracts/ ✅

**Organization**: Tasks are grouped by user story to enable independent implementation and testing of each story. No tests were requested in the spec, so no test tasks are included.

## Format: `[ID] [P?] [Story] Description`

- **[P]**: Can run in parallel (independent of other in-progress tasks)
- **[Story]**: Which user story this task belongs to (US1–US4)

---

## Phase 1: Setup

**Purpose**: Package skeleton and shared types — no new migration required; all tables exist in migration 001.

- [x] T001 Create `backend/internal/session/` directory and `backend/internal/session/models.go` with all types: `Session`, `CategoryRef`, `SessionFilter`, `SessionListItem`, `SessionListResult`, `SessionDetail`, `SessionCreate`, `SessionUpdate`, and sentinel errors `ErrSessionNotFound` / `ErrSessionNotDraft`
- [x] T002 [P] Create `backend/internal/session/repository.go` with the `SessionRepo` interface stub (method signatures only, no implementation bodies): `Create`, `List`, `FindByID`, `Update`, `Delete`, `CountAvailableQuestions`, `ValidateCategoryIDs`
- [x] T003 [P] Create `backend/internal/session/service.go` with the `Service` interface stub (method signatures only, no implementation bodies): `Create`, `List`, `FindByID`, `Update`, `Delete`

**Checkpoint**: Package skeleton compiles. No logic yet.

---

## Phase 2: Foundational (Blocking Prerequisites)

**Purpose**: Handler struct and constructor wiring — required before any HTTP handler can be registered.

**⚠️ CRITICAL**: Must be complete before route registration in any user story phase.

- [x] T004 Create `backend/internal/session/handler.go` with the `Handler` struct, `NewHandler(svc Service) *Handler` constructor, and empty stubs for each method: `Create`, `List`, `FindByID`, `Update`, `Delete`
- [x] T005 Add `SessionService` and `SessionHandler` instantiation and variable declarations to `backend/cmd/server/main.go` (no routes registered yet; just ensure the package compiles end-to-end)

**Checkpoint**: Backend compiles with the new session package imported. All routes still return 404.

---

## Phase 3: User Story 1 — Create a Configured Session (Priority: P1) 🎯 MVP

**Goal**: `POST /api/v1/sessions` — admin creates a session with configuration; system generates PIN, computes `available_questions`, and returns a warning when the pool is smaller than requested.

**Independent Test**: `POST /api/v1/sessions` with valid payload returns 201 with a 6-digit PIN, `status: "draft"`, and correct `available_questions`. Submitting fewer questions in the pool than `question_count` returns a `warning` field. Invalid payloads return 422.

- [x] T006 [US1] Implement `SessionRepository.ValidateCategoryIDs(ctx, ids []uuid.UUID) error` in `backend/internal/session/repository.go` — queries `categories` for all IDs, returns `VALIDATION_ERROR` if any ID is missing or soft-deleted
- [x] T007 [US1] Implement `SessionRepository.CountAvailableQuestions(ctx, sessionID uuid.UUID) (int, error)` in `backend/internal/session/repository.go` — COUNT JOIN on `questions` through `session_categories` scoped to the session
- [x] T008 [US1] Implement `SessionRepository.Create(ctx, s SessionCreate, adminID uuid.UUID) (Session, int, error)` in `backend/internal/session/repository.go` — inserts into `sessions` with PIN generation/retry loop (max 10 attempts on `23505` constraint), bulk-inserts into `session_categories`, returns the new session and `available_questions` count
- [x] T009 [US1] Implement `SessionService.Create(ctx, s SessionCreate, adminID uuid.UUID) (Session, int, string, error)` in `backend/internal/session/service.go` — validates fields (name non-empty, category_ids ≥1, question_count 1–50, time_per_question_s in {10,20,30,60}, points_per_answer >0), calls `repo.ValidateCategoryIDs`, calls `repo.Create`, returns session + available_questions + warning string (non-empty when pool < requested)
- [x] T010 [US1] Implement `Handler.Create(c *fiber.Ctx) error` in `backend/internal/session/handler.go` — parses JSON body, extracts admin ID from `auth.ClaimsKey` context, calls `svc.Create`, returns 201 with envelope; includes `warning` field when non-empty; returns 422 on validation errors
- [x] T011 [US1] Register `POST /sessions` route under the protected group in `backend/cmd/server/main.go`

**Checkpoint**: `POST /api/v1/sessions` fully functional. Verify with quickstart.md steps 1.

---

## Phase 4: User Story 2 — View and List Sessions (Priority: P2)

**Goal**: `GET /api/v1/sessions` and `GET /api/v1/sessions/:id` — paginated list with optional status filter and `player_count`; detail includes `categories` array.

**Independent Test**: `GET /api/v1/sessions` returns paginated list with `player_count`. `GET /api/v1/sessions?status=draft` filters correctly. `GET /api/v1/sessions/:id` returns `categories` array. Unknown ID returns 404.

- [x] T012 [P] [US2] Implement `SessionRepository.List(ctx, f SessionFilter) (SessionListResult, error)` in `backend/internal/session/repository.go` — SELECT with optional `status IN (...)` WHERE clause, `LEFT JOIN players … COUNT` subquery for `player_count`, soft-delete filter, pagination (LIMIT/OFFSET), total COUNT
- [x] T013 [P] [US2] Implement `SessionRepository.FindByID(ctx, id uuid.UUID) (SessionDetail, error)` in `backend/internal/session/repository.go` — SELECT session row + `player_count` COUNT + `session_categories JOIN categories` for `categories` array; returns `ErrSessionNotFound` when soft-deleted or absent
- [x] T014 [US2] Implement `SessionService.List(ctx, f SessionFilter) (SessionListResult, error)` and `SessionService.FindByID(ctx, id uuid.UUID) (SessionDetail, error)` in `backend/internal/session/service.go` — delegate to repo; apply default pagination values (page=1, per_page=20) in service layer
- [x] T015 [US2] Implement `Handler.List(c *fiber.Ctx) error` and `Handler.FindByID(c *fiber.Ctx) error` in `backend/internal/session/handler.go` — parse query params (status comma-split, page, per_page), call respective service methods, return 200 envelope; map `ErrSessionNotFound` → 404
- [x] T016 [US2] Register `GET /sessions` and `GET /sessions/:id` routes under the protected group in `backend/cmd/server/main.go`

**Checkpoint**: All three read endpoints functional. Verify with quickstart.md steps 2–3.

---

## Phase 5: User Story 3 — Edit a Draft Session (Priority: P3)

**Goal**: `PATCH /api/v1/sessions/:id` — partial update of configurable fields; blocked with 409 when session is not `draft`; recomputes `available_questions` when `category_ids` changes.

**Independent Test**: PATCH a draft session → 200 with updated fields. PATCH a non-draft session → 409 with `SESSION_NOT_DRAFT`. PATCH with new `category_ids` below `question_count` → 200 with `warning`.

- [x] T017 [US3] Implement `SessionRepository.Update(ctx, id uuid.UUID, u SessionUpdate) (Session, error)` in `backend/internal/session/repository.go` — builds dynamic SET clause for non-nil pointer fields; when `u.CategoryIDs` is non-nil, replaces `session_categories` rows in the same transaction (DELETE old rows, INSERT new rows); returns updated session row; returns `ErrSessionNotFound` when absent/deleted
- [x] T018 [US3] Implement `SessionService.Update(ctx, id uuid.UUID, u SessionUpdate) (Session, int, string, error)` in `backend/internal/session/service.go` — loads session, checks `status == "draft"` (returns `ErrSessionNotDraft` otherwise), validates non-nil update fields (same rules as Create), calls `repo.ValidateCategoryIDs` if `u.CategoryIDs` is non-nil, calls `repo.Update`, calls `repo.CountAvailableQuestions`, returns session + available_questions + warning
- [x] T019 [US3] Implement `Handler.Update(c *fiber.Ctx) error` in `backend/internal/session/handler.go` — parses partial JSON body using pointer fields, calls `svc.Update`, returns 200 envelope with updated session (including `available_questions` and optional `warning`); maps `ErrSessionNotDraft` → 409 `SESSION_NOT_DRAFT`; maps `ErrSessionNotFound` → 404
- [x] T020 [US3] Register `PATCH /sessions/:id` route under the protected group in `backend/cmd/server/main.go`

**Checkpoint**: PATCH endpoint fully functional. Verify with quickstart.md step 4.

---

## Phase 6: User Story 4 — Delete a Draft Session (Priority: P4)

**Goal**: `DELETE /api/v1/sessions/:id` — soft-delete a draft session; blocked with 409 for any other status.

**Independent Test**: DELETE a draft session → 200 `{ "data": { "ok": true } }` and session disappears from list. DELETE a non-draft session → 409. DELETE unknown ID → 404.

- [x] T021 [US4] Implement `SessionRepository.Delete(ctx, id uuid.UUID) error` in `backend/internal/session/repository.go` — fetches current status, returns `ErrSessionNotDraft` if not `draft`, then sets `deleted_at = now()` on the `sessions` row; returns `ErrSessionNotFound` when absent/already deleted
- [x] T022 [US4] Implement `SessionService.Delete(ctx, id uuid.UUID) error` in `backend/internal/session/service.go` — delegates to `repo.Delete`
- [x] T023 [US4] Implement `Handler.Delete(c *fiber.Ctx) error` in `backend/internal/session/handler.go` — calls `svc.Delete`, returns 200 `{ "data": { "ok": true } }`; maps `ErrSessionNotDraft` → 409; maps `ErrSessionNotFound` → 404
- [x] T024 [US4] Register `DELETE /sessions/:id` route under the protected group in `backend/cmd/server/main.go`

**Checkpoint**: DELETE endpoint fully functional. Verify with quickstart.md step 5 and edge-case tests.

---

## Phase 7: Polish & Cross-Cutting Concerns

- [x] T025 Update `CLAUDE.md` Active Technologies section to add `internal/session/` package entry (Go 1.25 + gofiber/fiber v2, jackc/pgx v5, google/uuid v1, math/rand)
- [x] T026 Update `CLAUDE.md` Recent Changes section with entry for `005-session-crud`
- [x] T027 Run all quickstart.md smoke tests end-to-end (create, list, detail, update, delete) and confirm all edge cases pass (warning on shortfall, 409 on non-draft PATCH/DELETE, 404 on unknown ID, status filter)

---

## Dependencies & Execution Order

### Phase Dependencies

- **Phase 1 (Setup)**: No dependencies — start immediately
- **Phase 2 (Foundational)**: Depends on Phase 1 — blocks all user story phases
- **Phase 3 (US1 — Create)**: Depends on Phase 2
- **Phase 4 (US2 — List/Detail)**: Depends on Phase 2; independent of Phase 3
- **Phase 5 (US3 — Update)**: Depends on Phase 3 (needs a created session to update); should run after US1
- **Phase 6 (US4 — Delete)**: Depends on Phase 3 (needs a created session to delete); should run after US1
- **Phase 7 (Polish)**: Depends on all user story phases

### User Story Dependencies

- **US1 (P1)**: No dependencies on other stories — implement first
- **US2 (P2)**: Independent of US1, but practically needs US1 to have test data
- **US3 (P3)**: Requires US1 to create a draft session for update testing
- **US4 (P4)**: Requires US1 to create a draft session for delete testing

### Within Each Phase

- T006/T007 can run in parallel (different methods, same file — write sequentially in practice)
- T012/T013 are marked [P] — independent method implementations
- All other tasks within a phase are sequential (each depends on the prior)

### Parallel Opportunities

```bash
# Phase 1: T002 and T003 can run in parallel (different files)
Task T002: "Create repository.go interface stub"
Task T003: "Create service.go interface stub"

# Phase 4: T012 and T013 can run in parallel (independent query methods)
Task T012: "Implement SessionRepository.List()"
Task T013: "Implement SessionRepository.FindByID()"
```

---

## Implementation Strategy

### MVP First (User Story 1 Only)

1. Complete Phase 1: Setup (T001–T003)
2. Complete Phase 2: Foundational (T004–T005)
3. Complete Phase 3: User Story 1 (T006–T011)
4. **STOP and VALIDATE**: `POST /api/v1/sessions` returns 201 with PIN, status, available_questions, and warning when needed
5. Demo to stakeholders if needed

### Incremental Delivery

1. Complete Setup + Foundational → package compiles, backend starts
2. Add US1 (Create) → `POST /sessions` working → MVP!
3. Add US2 (List/Detail) → `GET /sessions` + `GET /sessions/:id` working
4. Add US3 (Update) → `PATCH /sessions/:id` working
5. Add US4 (Delete) → `DELETE /sessions/:id` working
6. Polish → CLAUDE.md updated, smoke tests verified

---

## Notes

- No new migration needed — all tables and indexes already exist in migration 001
- PIN generation retry loop goes in the repository layer (T008), not the service layer
- `available_questions` is never stored — always computed live (T007 + called from T009 and T018)
- The `status` guard for PATCH and DELETE lives in the repository layer to keep the check atomic with the query (no TOCTOU race)
- `session_categories` replacement in PATCH is transactional — delete old rows, insert new, update session row, all in one transaction (T017)
- Admin ID extraction: `c.Locals(auth.ClaimsKey).(auth.AdminClaims).AdminID` — same pattern as other handlers
