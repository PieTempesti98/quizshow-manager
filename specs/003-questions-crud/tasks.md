# Tasks: Questions CRUD

**Input**: Design documents from `/specs/003-questions-crud/`
**Prerequisites**: plan.md ✅, spec.md ✅, research.md ✅, data-model.md ✅, contracts/rest-api.md ✅, quickstart.md ✅

**Tests**: Not requested — no test tasks generated.

**Organization**: Tasks are grouped by user story to enable independent implementation and testing. Each story adds incrementally to the four files in `backend/internal/question/` and one route registration in `backend/cmd/server/main.go`.

## Format: `[ID] [P?] [Story] Description`

- **[P]**: Can run in parallel (different files, no dependencies at that moment)
- **[Story]**: Which user story (US1–US4)

## Path Conventions

All source code paths are relative to the repository root. The new package lives at `backend/internal/question/`.

---

## Phase 1: Setup (Shared Foundation)

**Purpose**: Create the one type-definitions file that every subsequent task depends on. No new dependencies; no project-initialization steps needed — the Go backend is already running with all required libraries.

- [ ] T001 Create `backend/internal/question/models.go` with: `Question` struct (all fields including `CategoryID uuid.UUID` + `CategoryName string` for JOIN), `CategoryRef` struct (`ID`, `Name`), `QuestionFilter` struct (`CategoryIDs []uuid.UUID`, `Difficulties []string`, `Search string`, `Page int`, `PerPage int`), `QuestionListResult` struct (`Questions []Question`, `Total int`, `Page int`, `PerPage int`, `TotalPages int`), `QuestionUpdate` struct (all pointer fields: `CategoryID *uuid.UUID`, `Text *string`, `OptionA/B/C/D *string`, `CorrectIndex *int`, `Difficulty *string`), sentinel errors `ErrQuestionNotFound` and `ErrQuestionInUse`

**Checkpoint**: models.go compiles (`go build ./internal/question/`). All downstream tasks unblocked.

---

## Phase 2: Foundational (Blocking Prerequisites)

No tasks. The Go backend project is fully initialized and `models.go` (Phase 1) is the only shared prerequisite for all user stories.

**⚠️ CRITICAL**: Phase 1 `models.go` must be complete before any Phase 3+ task begins.

---

## Phase 3: User Story 1 — Browse and Find Questions (Priority: P1) 🎯 MVP

**Goal**: Admin can call `GET /api/v1/questions` with any combination of filters and receive a paginated, nested-category response.

**Independent Test**: With questions seeded in the database, `GET /api/v1/questions?difficulty=medium` returns only medium-difficulty questions with `category: {id, name}` in each item and correct `pagination` metadata. Verified with curl per `quickstart.md` step 2–3.

### Implementation for User Story 1

- [ ] T002 [US1] Create `backend/internal/question/repository.go` with: `QuestionRepo` interface (one method: `List(ctx, QuestionFilter) (QuestionListResult, error)`), `QuestionRepository` struct, `NewRepository(pool *pgxpool.Pool) *QuestionRepository` constructor, and `List` implementation — dynamic WHERE clause builder (`clauses []string`, `args []any`) appending `q.deleted_at IS NULL` always, `q.category_id = ANY($N)` when `CategoryIDs` non-empty, `q.difficulty = ANY($N)` when `Difficulties` non-empty, `q.text ILIKE $N` when `Search` non-empty; always `LEFT JOIN categories c ON c.id = q.category_id AND c.deleted_at IS NULL`; sort `ORDER BY q.created_at DESC`; separate `SELECT COUNT(*)` query (same WHERE, no LIMIT/OFFSET); result query with `LIMIT $N OFFSET $M`; scan into `Question` including `c.id` and `c.name` for `CategoryID` and `CategoryName`; clamp `PerPage` to 100, default to 25; compute `TotalPages = ceil(Total/PerPage)`

- [ ] T003 [US1] Create `backend/internal/question/service.go` with: `Service` interface (one method: `List(ctx, QuestionFilter) (QuestionListResult, error)`), `service` struct, `NewService(repo QuestionRepo) Service` constructor, and `List` implementation that calls `repo.List` directly (no business logic at list level)

- [ ] T004 [US1] Create `backend/internal/question/handler.go` with: `Handler` struct (`svc Service`), `NewHandler(svc Service) *Handler`, `categoryResponse` struct (`ID string`, `Name string`), `questionResponse` struct (all question fields with JSON tags, `Category categoryResponse`), `toQuestionResponse(q Question) questionResponse` mapper (formats `CreatedAt` as UTC ISO 8601), `toPaginationResponse` helper, and `List(c *fiber.Ctx) error` handler — parse `page` (default 1), `per_page` (default 25, max 100), `category_id` (split comma-separated string into `[]uuid.UUID`, skip invalid UUIDs with 422), `difficulty` (split comma-separated), `q` (search string); build `QuestionFilter`; call `h.svc.List`; return `api.DataResponse{Data: map[string]any{"questions": resp, "pagination": pagination}}` with HTTP 200

- [ ] T005 [US1] Update `backend/cmd/server/main.go` to: import `"github.com/PieTempesti98/quizshow/internal/question"`, add `questionRepo := question.NewRepository(pool)`, `questionSvc := question.NewService(questionRepo)`, `questionHandler := question.NewHandler(questionSvc)`, register `protected.Get("/questions", questionHandler.List)`

**Checkpoint**: `GET /api/v1/questions` responds with 200, empty or populated list, correct pagination. `GET /api/v1/questions?difficulty=medium,hard&q=Roma` filters correctly. US1 independently functional.

---

## Phase 4: User Story 2 — Create a Question (Priority: P2)

**Goal**: Admin can `POST /api/v1/questions` with all required fields and receive the new question object (HTTP 201). Omitting `difficulty` defaults to `"medium"`. Validation errors return HTTP 422.

**Independent Test**: POST a valid question body → HTTP 201 with nested category object and `difficulty: "medium"` (when omitted). POST with `correct_index: 5` → HTTP 422. POST with empty `option_a` → HTTP 422.

### Implementation for User Story 2

- [ ] T006 [US2] Add to `backend/internal/question/repository.go`: expand `QuestionRepo` interface with `Create(ctx, Question) (*Question, error)` (where input `Question` has `CategoryID`, `Text`, `OptionA/B/C/D`, `CorrectIndex`, `Difficulty` populated); implement `Create` with `INSERT INTO questions (category_id, text, option_a, option_b, option_c, option_d, correct_index, difficulty) VALUES ($1...$8) RETURNING id, category_id, text, option_a, option_b, option_c, option_d, correct_index, difficulty, created_at, updated_at`; then fetch category name with `SELECT id, name FROM categories WHERE id = $1 AND deleted_at IS NULL` to populate `CategoryName` on the returned `*Question`; handle FK violation (category not found) as `ErrQuestionNotFound`

- [ ] T007 [US2] Add to `backend/internal/question/service.go`: expand `Service` interface with `Create(ctx, Question) (*Question, error)`; implement `Create` — validate `CorrectIndex` 0–3, validate `Difficulty` in {"easy","medium","hard"} (default "medium" if empty), call `repo.Create`; map `ErrQuestionNotFound` from repo to a category-not-found context

- [ ] T008 [US2] Add to `backend/internal/question/handler.go`: add `createRequest` struct (`CategoryID string`, `Text string`, `OptionA/B/C/D string`, `CorrectIndex int`, `Difficulty *string` as pointer); implement `Create(c *fiber.Ctx) error` — parse body, validate all required fields non-empty, validate `CategoryID` parses as UUID, validate `CorrectIndex` 0–3, validate `Difficulty` (if provided) in allowed set, call `h.svc.Create`, map `ErrQuestionNotFound` to HTTP 422 `VALIDATION_ERROR` ("category not found"), return HTTP 201 with `api.DataResponse{Data: toQuestionResponse(*q)}`

- [ ] T009 [US2] Update `backend/cmd/server/main.go`: register `protected.Post("/questions", questionHandler.Create)`

**Checkpoint**: `POST /api/v1/questions` with valid body → HTTP 201, full question object with nested category. Validation errors return 422. US2 independently functional alongside US1.

---

## Phase 5: User Story 3 — Edit a Question (Priority: P3)

**Goal**: Admin can `PATCH /api/v1/questions/:id` with any subset of fields. Only supplied fields are updated; absent fields are unchanged. Request blocked with 409 if question is in an active session.

**Independent Test**: PATCH only `{"text": "updated"}` → response shows new text, all other fields identical to original. PATCH a question linked to an active-status session → HTTP 409 `QUESTION_IN_USE`. PATCH non-existent ID → HTTP 404.

### Implementation for User Story 3

- [ ] T010 [US3] Add to `backend/internal/question/repository.go`: expand `QuestionRepo` interface with `FindByID(ctx, uuid.UUID) (*Question, error)`, `IsInActiveSession(ctx, uuid.UUID) (bool, error)`, `Update(ctx, uuid.UUID, QuestionUpdate) (*Question, error)`; implement `IsInActiveSession` with `SELECT EXISTS (SELECT 1 FROM session_questions sq JOIN sessions s ON s.id = sq.session_id WHERE sq.question_id = $1 AND s.status = 'active' AND s.deleted_at IS NULL)`; implement `FindByID` selecting all question fields + `JOIN categories c` for name, `WHERE q.id = $1 AND q.deleted_at IS NULL`, returns `nil, nil` on `pgx.ErrNoRows`; implement `Update` building a dynamic SET clause: iterate `QuestionUpdate` pointer fields, append `col = $N` only for non-nil fields, always append `updated_at = now()`, append `WHERE id = $N AND deleted_at IS NULL RETURNING ...`, then re-fetch category name for the returned question; if UPDATE affects zero rows return `ErrQuestionNotFound`

- [ ] T011 [US3] Add to `backend/internal/question/service.go`: expand `Service` interface with `Update(ctx, uuid.UUID, QuestionUpdate) (*Question, error)`; implement `Update` — call `repo.IsInActiveSession`; if true return `ErrQuestionInUse`; call `repo.FindByID`; if nil return `ErrQuestionNotFound`; validate non-nil fields in `QuestionUpdate` (`CorrectIndex` 0–3 if non-nil, `Difficulty` in allowed set if non-nil, non-empty strings if non-nil); call `repo.Update`

- [ ] T012 [US3] Add to `backend/internal/question/handler.go`: add `updateRequest` struct with all pointer fields (`CategoryID *string`, `Text *string`, `OptionA/B/C/D *string`, `CorrectIndex *int`, `Difficulty *string`); implement `Update(c *fiber.Ctx) error` — parse `:id` as UUID (422 on failure), parse body into `updateRequest`, convert non-nil string `CategoryID` to `*uuid.UUID` (422 on invalid UUID), validate non-nil fields, build `QuestionUpdate`, call `h.svc.Update`, map errors: `ErrQuestionNotFound` → 404, `ErrQuestionInUse` → 409 `QUESTION_IN_USE`, return 200 with updated question

- [ ] T013 [US3] Update `backend/cmd/server/main.go`: register `protected.Patch("/questions/:id", questionHandler.Update)`

**Checkpoint**: `PATCH /questions/:id` with `{"difficulty":"hard"}` → only difficulty changes. Active-session guard returns 409. US3 independently functional.

---

## Phase 6: User Story 4 — Delete a Question (Priority: P4)

**Goal**: Admin can `DELETE /api/v1/questions/:id` to soft-delete a question. Question disappears from list; session history unchanged. Blocked with 409 if question is in an active session.

**Independent Test**: Create a question, DELETE it → HTTP 200 `{"ok":true}`, question absent from list. DELETE a question linked to an active session → HTTP 409. DELETE non-existent ID → HTTP 404.

### Implementation for User Story 4

- [ ] T014 [US4] Add to `backend/internal/question/repository.go`: expand `QuestionRepo` interface with `Delete(ctx, uuid.UUID) error`; implement `Delete` with `UPDATE questions SET deleted_at = now(), updated_at = now() WHERE id = $1 AND deleted_at IS NULL`; check `RowsAffected()` — if 0 rows affected return `ErrQuestionNotFound` (question was already deleted or never existed)

- [ ] T015 [US4] Add to `backend/internal/question/service.go`: expand `Service` interface with `Delete(ctx, uuid.UUID) error`; implement `Delete` — call `repo.IsInActiveSession`; if true return `ErrQuestionInUse`; call `repo.Delete` (which returns `ErrQuestionNotFound` if missing)

- [ ] T016 [US4] Add to `backend/internal/question/handler.go`: implement `Delete(c *fiber.Ctx) error` — parse `:id` as UUID (422 on failure), call `h.svc.Delete`, map `ErrQuestionNotFound` → 404, `ErrQuestionInUse` → 409 `QUESTION_IN_USE`, success → 200 `{"data": {"ok": true}}`

- [ ] T017 [US4] Update `backend/cmd/server/main.go`: register `protected.Delete("/questions/:id", questionHandler.Delete)`

**Checkpoint**: All four endpoints functional. Full CRUD lifecycle works. Run `go build ./...` — must compile cleanly.

---

## Phase 7: Polish & Cross-Cutting Concerns

**Purpose**: Validate end-to-end correctness, verify edge cases, update project documentation.

- [ ] T018 Run full smoke test against `quickstart.md` scenarios: create question (default difficulty), list with all filter combos, PATCH partial update, DELETE, unauthenticated request → 401, `per_page=200` → clamped to 100

- [ ] T019 [P] Update `CLAUDE.md` Active Technologies section: add `internal/question/` package description; update Recent Changes to note feature 003 implementation

- [ ] T020 [P] Verify `go vet ./...` and `go build ./...` pass cleanly for the entire backend

---

## Dependencies & Execution Order

### Phase Dependencies

- **Phase 1** (models.go): No dependencies — start immediately
- **Phase 3** (US1): Requires T001 complete — creates repository.go, service.go, handler.go from scratch
- **Phase 4** (US2): Requires Phase 3 complete — adds Create method to existing files
- **Phase 5** (US3): Requires Phase 4 complete — adds FindByID, IsInActiveSession, Update to existing files
- **Phase 6** (US4): Requires Phase 5 complete — reuses IsInActiveSession, adds Delete
- **Phase 7** (Polish): Requires all user stories complete

### User Story Dependencies

- **US1 (P1)**: Depends only on T001 (models.go). Start here.
- **US2 (P2)**: Depends on US1 (extends same files; category existence check requires working repository).
- **US3 (P3)**: Depends on US2 (IsInActiveSession is introduced here; tests need existing questions).
- **US4 (P4)**: Depends on US3 (reuses IsInActiveSession from repository).

### Within Each User Story

1. Repository method → Service method → Handler method → Route registration
2. Each layer depends on the previous layer in the same story

### Parallel Opportunities

Within Phase 3 (US1), T002 (repository) and T003 (service) both create new files and could be written simultaneously by different developers — but T003 references the `QuestionRepo` interface from T002, so sequential is safer for a single developer.

T019 and T020 in Phase 7 touch different files and can run in parallel.

---

## Parallel Example: User Story 1

```
# Sequential for single developer (interface dependency: service.go references QuestionRepo from repository.go):
T002 → T003 → T004 → T005

# For parallel team:
T002 (repository.go)   ←─────────────────┐
                                          ↓
T003 (service.go) — can start writing interface + constructor, await T002 for impl
T004 (handler.go) — can start writing struct + response types, await T003 for impl
```

---

## Implementation Strategy

### MVP First (User Story 1 Only)

1. Complete Phase 1: Create `models.go` (T001)
2. Complete Phase 3: US1 (T002–T005) — list with filters and pagination
3. **STOP and VALIDATE**: `GET /api/v1/questions` works with all filter combinations
4. Admin can browse the question bank immediately

### Incremental Delivery

1. T001 → models.go ready
2. T002–T005 → `GET /questions` works (**US1 complete**)
3. T006–T009 → `POST /questions` works (**US2 complete**)
4. T010–T013 → `PATCH /questions/:id` works (**US3 complete**)
5. T014–T017 → `DELETE /questions/:id` works (**US4 complete**)
6. T018–T020 → Polish and verify

Each story adds endpoints without breaking previously working endpoints.

---

## Notes

- All 4 user stories extend the same 4 files (`models.go`, `repository.go`, `service.go`, `handler.go`) and `main.go` — one route registration per story
- `IsInActiveSession` repository method is introduced in US3 (T010) and reused without change in US4 (T015)
- The `QuestionRepo` interface grows incrementally across phases — Go's structural typing means partial interface satisfaction is valid at each stage
- Commit after each checkpoint (T005, T009, T013, T017) to keep git history aligned with story boundaries
- `per_page` clamping (max 100) is enforced in the handler, not the repository
- Empty PATCH body → `QuestionUpdate` with all nil fields → `UPDATE ... SET updated_at = now() WHERE ...` → returns unchanged question
