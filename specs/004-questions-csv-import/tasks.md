# Tasks: Questions CSV Import

**Input**: Design documents from `/specs/004-questions-csv-import/`  
**Prerequisites**: plan.md ✅, spec.md ✅, research.md ✅, data-model.md ✅, contracts/http.md ✅, quickstart.md ✅

**Tests**: No automated test tasks — spec does not request TDD. Validation via manual smoke test per `quickstart.md`.

**Organization**: Tasks grouped by user story to enable independent implementation and testing.

## Format: `[ID] [P?] [Story] Description`

- **[P]**: Can run in parallel (different files, no dependencies on incomplete tasks)
- **[Story]**: User story label (US1–US4 from spec.md)

---

## Phase 1: Setup

**Purpose**: Update server configuration required by the import feature before any new code is added.

- [x] T001 Update `fiber.Config{BodyLimit: 6 * 1024 * 1024}` in `backend/cmd/server/main.go` (Fiber default is 4MB; import allows 5MB files plus multipart overhead)

---

## Phase 2: Foundational (Blocking Prerequisites)

**Purpose**: New Go types and repository methods shared by all user stories. Must be complete before any story phase begins.

**⚠️ CRITICAL**: No user story work can begin until this phase is complete.

- [x] T002 Add `ImportRow`, `ImportResult`, `RowError` types and `ErrInvalidOnError` sentinel var to `backend/internal/question/models.go`
- [x] T003 [P] Add `FindCategoryMap(ctx context.Context) (map[string]uuid.UUID, error)` to `QuestionRepo` interface and implement on `QuestionRepository` in `backend/internal/question/repository.go` — query: `SELECT id, LOWER(name) FROM categories WHERE deleted_at IS NULL`
- [x] T004 [P] Add `CheckDuplicate(ctx context.Context, text string, categoryID uuid.UUID) (bool, error)` to `QuestionRepo` interface and implement on `QuestionRepository` in `backend/internal/question/repository.go` — query: `SELECT EXISTS(SELECT 1 FROM questions WHERE LOWER(text)=LOWER($1) AND category_id=$2 AND deleted_at IS NULL)`
- [x] T005 Add `Import(ctx context.Context, r io.Reader, onError string) (ImportResult, error)` to `Service` interface in `backend/internal/question/service.go` (stub implementation only — returns zero ImportResult and nil error)

**Checkpoint**: `go build ./...` passes with no errors. Foundation ready.

---

## Phase 3: User Story 1 — CSV Template Download (Priority: P1) 🎯 MVP

**Goal**: Admin (or anyone) can download a ready-to-use CSV template file.

**Independent Test**: `curl http://localhost:3000/api/v1/questions/import/template -o t.csv && cat t.csv` — returns exactly `text,option_a,option_b,option_c,option_d,correct_index,category_name,difficulty` with no data rows. No auth token needed.

- [x] T006 [US1] Implement `ImportTemplate(c *fiber.Ctx) error` handler in `backend/internal/question/handler.go` — set `Content-Type: text/csv`, `Content-Disposition: attachment; filename="questions_template.csv"`, write header string `"text,option_a,option_b,option_c,option_d,correct_index,category_name,difficulty\n"` to response body
- [x] T007 [US1] Register `v1.Get("/questions/import/template", questionHandler.ImportTemplate)` on the **public** `v1` group (before the `protected` group, no auth) in `backend/cmd/server/main.go`

**Checkpoint**: Template endpoint works unauthenticated, quickstart.md step 1 passes.

---

## Phase 4: User Story 2 — Bulk Import (Abort Mode) (Priority: P1)

**Goal**: Admin uploads a CSV; if any row is invalid, nothing is written to the DB and a full error list is returned. If all rows are valid, all are imported atomically.

**Independent Test**: Upload `sample.csv` with one invalid row → response `{imported:0, skipped:0, errors:[{row:3,...}]}`. Upload an all-valid CSV → `{imported:N, skipped:0, errors:[]}`. Confirm new questions appear via `GET /api/v1/questions`.

- [x] T008 [US2] Add `CreateBatch(ctx context.Context, tx pgx.Tx, questions []Question) error` to `QuestionRepo` interface and implement on `QuestionRepository` in `backend/internal/question/repository.go` — iterates over the slice and executes the same `INSERT INTO questions` as `Create()` but within the supplied transaction (`tx.Exec(...)`)
- [x] T009 [US2] Implement full `Import()` body on `service` in `backend/internal/question/service.go`:
  - Parse `on_error` (must be `"abort"` or `"skip"`; return `ErrInvalidOnError` if invalid)
  - Parse CSV from `r io.Reader` using `encoding/csv`; validate header row has exactly the 8 expected columns
  - Count data rows; return error if > 500
  - Call `FindCategoryMap` once to load `lower(name) → uuid` cache
  - For each row: validate required fields, `correct_index` range, `difficulty` enum (default `"medium"`), resolve `category_name` via map — collect `RowError` on any failure
  - For valid rows: call `CheckDuplicate`; if true, append soft warning to errors (do not count as hard error)
  - Abort mode: if any hard errors present → return `ImportResult{Imported:0, Skipped:0, Errors:hardErrors+warnings}`; else → `pool.Begin` tx → `CreateBatch` → commit → return `ImportResult{Imported:N, Errors:warnings}`
- [x] T010 [US2] Implement `Import(c *fiber.Ctx) error` handler in `backend/internal/question/handler.go`:
  - Read `on_error` form field (default `"abort"`)
  - Get `file` from `c.FormFile("file")`; return 422 if missing
  - Check `header.Size > 5*1024*1024`; return 422 if exceeded
  - Open file, call `h.svc.Import(ctx, file, onError)`
  - On `ErrImportFileInvalid`: return 422 with `VALIDATION_ERROR`
  - On success: return 200 `api.DataResponse{Data: importResult}`
  - On unexpected error: return 500
- [x] T011 [US2] Register `protected.Post("/questions/import", questionHandler.Import)` in `backend/cmd/server/main.go`

**Checkpoint**: Quickstart.md steps 3, 5, 6, 7 pass (abort mode, row limit, file size, 401 check).

---

## Phase 5: User Story 3 — Bulk Import (Skip Mode) (Priority: P2)

**Goal**: Admin uploads a CSV with mixed valid/invalid rows; valid rows are imported individually, invalid rows are skipped and reported.

**Independent Test**: Upload `sample.csv` with 2 valid + 1 invalid row using `on_error=skip` → response `{imported:2, skipped:1, errors:[{row:3,...}]}`.

- [x] T012 [US3] Replace the skip-mode `TODO` stub in `service.Import()` in `backend/internal/question/service.go` with the full skip-mode branch: for each row after validation — if hard error: increment `skipped`, append `RowError`; if valid: call `repo.Create(ctx, q)` (reusing existing method), increment `imported`, append any duplicate warning to `errors`; return `ImportResult{Imported:imported, Skipped:skipped, Errors:allErrors}`

**Checkpoint**: Quickstart.md step 4 passes (skip mode).

---

## Phase 6: User Story 4 — Row-Level Validation Feedback (Priority: P2)

**Goal**: Every error message in the `errors` array clearly names the row number and the specific field or value that caused the failure.

**Independent Test**: For each error type listed in the contract, upload a CSV that triggers it and verify the `message` is specific (names the field or shows the invalid value), not generic.

- [x] T013 [US4] Audit all `RowError` messages produced in `service.Import()` in `backend/internal/question/service.go` and ensure each message matches the expected formats from `specs/004-questions-csv-import/contracts/http.md`: `"text: required"`, `"option_a: required"`, `"correct_index must be between 0 and 3, got: X"`, `"category 'X' not found"`, `"difficulty: invalid value 'X', must be easy, medium, or hard"`, `"duplicate question in category 'X'"` — update any generic messages to be specific

**Checkpoint**: Quickstart.md step 8 (duplicate warning) passes.

---

## Phase 7: Polish & Cross-Cutting Concerns

**Purpose**: Final wiring, edge case hardening, and plan update.

- [x] T014 [P] Verify the `on_error` field missing entirely defaults to `"abort"` (handler reads `c.FormValue("on_error", "abort")`) in `backend/internal/question/handler.go` and confirm empty CSV body (0 data rows) returns `{imported:0, skipped:0, errors:[]}` rather than an error
- [x] T015 Run full smoke test sequence from `specs/004-questions-csv-import/quickstart.md` steps 1–8 against live Docker Compose environment and confirm all pass

---

## Dependencies & Execution Order

### Phase Dependencies

- **Setup (Phase 1)**: No dependencies — start immediately
- **Foundational (Phase 2)**: Depends on Phase 1 — **blocks all user stories**
- **US1 (Phase 3)**: Depends on Phase 2 — no dependency on other stories
- **US2 (Phase 4)**: Depends on Phase 2 — no dependency on US1
- **US3 (Phase 5)**: Depends on Phase 4 (reuses service.Import() skeleton and repo.Create)
- **US4 (Phase 6)**: Depends on Phase 4 (audits messages already written)
- **Polish (Phase 7)**: Depends on Phases 3–6

### User Story Dependencies

- **US1 (P1)**: Independent after Phase 2 — can be done before, after, or in parallel with US2
- **US2 (P1)**: Independent after Phase 2 — the core story
- **US3 (P2)**: Depends on US2 (extends `service.Import()`)
- **US4 (P2)**: Depends on US2 (audits messages written during US2)

### Within Each Phase

- T003 and T004 (Phase 2) can run in parallel — different repository methods, same file
- T006 and T007 (Phase 3) must be sequential — register after implementing
- T008 and T009 should be sequential within Phase 4 — T009 calls T008's method
- T010 and T011 must be sequential — register after implementing

### Parallel Opportunities

- T003 and T004 can run in parallel
- T006 (handler) and foundational completion can overlap if working in separate files

---

## Parallel Example: Phase 2

```bash
# These two repository methods can be implemented in parallel (same file, non-conflicting functions):
Task T003: "Add FindCategoryMap to repository.go"
Task T004: "Add CheckDuplicate to repository.go"
```

---

## Implementation Strategy

### MVP First (US1 + US2 only)

1. Complete Phase 1: Setup (T001)
2. Complete Phase 2: Foundational (T002–T005)
3. Complete Phase 3: US1 Template (T006–T007)
4. Complete Phase 4: US2 Abort mode (T008–T011)
5. **STOP and VALIDATE**: Smoke test abort mode, template, auth
6. Ship — admins can import questions with safety guarantees

### Incremental Delivery

1. Setup + Foundational → `go build` passes
2. US1 (template) → `curl` test passes unauthenticated
3. US2 (abort) → core import works, all-or-nothing
4. US3 (skip) → lenient mode available
5. US4 (validation) → error messages are precise
6. Polish → full smoke test green

---

## Notes

- No new files needed — all additions are to the 4 existing files in `internal/question/` plus `main.go`
- The `io.Reader` passed to `service.Import()` is the opened multipart file — the handler is responsible for `file.Open()` and `defer file.Close()`
- `CreateBatch` uses `pgx.Tx` (not `pgxpool.Pool`) — the batch transaction is started in the service layer, not the repository; the repository only executes statements within the provided transaction
- Row numbers in error messages are 1-based, data rows only (header row excluded)
- Total tasks: 15 (T001–T015)
