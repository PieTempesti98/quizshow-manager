# Tasks: Categories CRUD

**Input**: Design documents from `/specs/002-categories-crud/`  
**Prerequisites**: plan.md ✅ spec.md ✅ research.md ✅ data-model.md ✅ contracts/ ✅  
**Tests**: Not explicitly requested in spec. No test tasks generated.

## Format: `[ID] [P?] [Story] Description`

- **[P]**: Can run in parallel (different files, no dependencies)
- **[Story]**: Which user story this task belongs to
- All paths relative to repository root

---

## Phase 1: Setup

**Purpose**: Create the new Go package skeleton before any implementation.

- [x] T001 Create directory `backend/internal/category/` (empty, just mkdir — Go package will be added in Foundational phase)

---

## Phase 2: Foundational (Blocking Prerequisites)

**Purpose**: Shared types, data access layer, and wiring that all three user stories depend on. All tasks in this phase must complete before any Phase 3+ work begins.

**⚠️ CRITICAL**: All user story phases depend on this phase being complete.

- [x] T002 Create `backend/internal/category/models.go` — define `Category` struct (ID, Name, Slug, CreatedAt, UpdatedAt, DeletedAt), `CategoryWithCount` struct (embeds `Category` + `QuestionCount int`), `slugify(name string) string` pure function (lowercase, spaces→hyphens, strip non-[a-z0-9-], collapse runs of hyphens, trim leading/trailing hyphens), and package-level sentinel errors: `ErrCategoryNotFound`, `ErrCategoryNameConflict`, `ErrCategoryHasQuestions` (struct type with `Count int` field for the blocking question count)

- [x] T003 Create `backend/internal/category/repository.go` — define `CategoryRepo` interface with methods: `List(ctx) ([]CategoryWithCount, error)`, `FindByID(ctx, uuid.UUID) (*Category, error)`, `Create(ctx, name, slug string) (*Category, error)`, `Update(ctx, id uuid.UUID, name, slug string) (*Category, error)`, `CountActiveQuestions(ctx, id uuid.UUID) (int, error)`, `Delete(ctx, id uuid.UUID) error`; implement `CategoryRepository` struct holding `*pgxpool.Pool`; implement `NewCategoryRepository(pool *pgxpool.Pool) *CategoryRepository`; implement all six methods using the SQL from `specs/002-categories-crud/data-model.md`; map `pgconn.PgError` with `Code == "23505"` to `ErrCategoryNameConflict` in `Create` and `Update`; map `pgx.ErrNoRows` to `nil` return in `FindByID` (caller checks nil)

---

## Phase 3: User Story 1 — List and Create Categories (Priority: P1) 🎯 MVP

**Goal**: Admin can list all existing categories (with question counts) and create new categories. Both `GET /api/v1/categories` and `POST /api/v1/categories` are live and auth-protected.

**Independent Test**: Start the server, obtain an admin JWT via `POST /auth/login`, then:
1. `GET /api/v1/categories` → 200 with `{ "data": { "categories": [] } }` (empty DB)
2. `POST /api/v1/categories` body `{"name":"Storia"}` → 201 with `{ "data": { "id": "...", "name": "Storia", "slug": "storia", "question_count": 0, "created_at": "..." } }`
3. `POST /api/v1/categories` body `{"name":"Storia"}` again → 409 CONFLICT
4. `GET /api/v1/categories` → 200 with one category, `question_count: 0`
5. `GET /api/v1/categories` without Authorization header → 401

- [x] T004 [US1] Create `backend/internal/category/service.go` — define `Service` interface with methods: `List(ctx) ([]CategoryWithCount, error)`, `Create(ctx, name string) (*CategoryWithCount, error)`, `Rename(ctx, id uuid.UUID, name string) (*CategoryWithCount, error)`, `Delete(ctx, id uuid.UUID) error`; implement private `service` struct holding `CategoryRepo`; implement `NewService(repo CategoryRepo) Service`; implement `List`: delegates to `repo.List`; implement `Create`: validate name non-empty and ≤50 chars (return `ErrCategoryNameConflict` on length? no — return a validation error type), call `slugify(name)`, call `repo.Create`, return `CategoryWithCount` with `QuestionCount: 0`; note: `Create` returns `ErrCategoryNameConflict` if repo returns that sentinel; stub out `Rename` and `Delete` with `return nil, errors.New("not implemented")` / `return errors.New("not implemented")`

- [x] T005 [US1] Create `backend/internal/category/handler.go` — define `Handler` struct holding `Service`; implement `NewHandler(svc Service) *Handler`; implement `List(c *fiber.Ctx) error`: call `svc.List`, return 200 `api.DataResponse{Data: map[string]any{"categories": results}}`; implement `Create(c *fiber.Ctx) error`: parse body into struct `{Name string json:"name"}`, validate `Name` non-empty and ≤50 chars returning 422 on failure, call `svc.Create`, return 201 on success, map `ErrCategoryNameConflict` → 409 `CONFLICT`; stub `Rename` and `Delete` as `return c.SendStatus(fiber.StatusNotImplemented)` for now

- [x] T006 [US1] Wire category package into `backend/cmd/server/main.go` — import `internal/category`, instantiate `categoryRepo := category.NewCategoryRepository(pool)`, `categorySvc := category.NewService(categoryRepo)`, `categoryHandler := category.NewHandler(categorySvc)`, add to the existing `protected` group: `protected.Get("/categories", categoryHandler.List)` and `protected.Post("/categories", categoryHandler.Create)`

**Checkpoint**: `GET /api/v1/categories` and `POST /api/v1/categories` fully functional. User Story 1 independently testable.

---

## Phase 4: User Story 2 — Rename a Category (Priority: P2)

**Goal**: Admin can rename a non-deleted category. Slug is regenerated from the new name. `PATCH /api/v1/categories/:id` is live.

**Independent Test**: Using the server from Phase 3:
1. Create a category "Storia" (via POST)
2. `PATCH /api/v1/categories/{id}` body `{"name":"Storia Moderna"}` → 200 with updated name `"Storia Moderna"` and slug `"storia-moderna"`
3. Create another category "Arte"
4. `PATCH /api/v1/categories/{storia-id}` body `{"name":"Arte"}` → 409 CONFLICT
5. `PATCH /api/v1/categories/00000000-0000-0000-0000-000000000000` body `{"name":"X"}` → 404
6. `PATCH /api/v1/categories/{id}` body `{"name":""}` → 422

- [x] T007 [US2] Implement `Rename` in `backend/internal/category/service.go` — replace the stub: validate name non-empty and ≤50 chars, call `repo.FindByID`, if nil return `ErrCategoryNotFound`, call `slugify(name)`, call `repo.Update(ctx, id, name, slug)`, map `pgx.ErrNoRows` from Update to `ErrCategoryNotFound`, return `CategoryWithCount` built from updated `Category` + `repo.CountActiveQuestions` for current count; map `ErrCategoryNameConflict` from repo through to caller

- [x] T008 [US2] Implement `Rename` handler in `backend/internal/category/handler.go` — replace the stub: parse `:id` path param as `uuid.UUID` (return 422 on parse error), parse body `{Name string}`, validate non-empty and ≤50 chars returning 422, call `svc.Rename`, map `ErrCategoryNotFound` → 404 `NOT_FOUND`, map `ErrCategoryNameConflict` → 409 `CONFLICT`, return 200 with updated category object (`id`, `name`, `slug`, `question_count`, `created_at`)

- [x] T009 [US2] Register rename route in `backend/cmd/server/main.go` — add `protected.Patch("/categories/:id", categoryHandler.Rename)` alongside the existing category routes

**Checkpoint**: `PATCH /api/v1/categories/:id` fully functional. User Stories 1 and 2 independently testable.

---

## Phase 5: User Story 3 — Soft-Delete a Category (Priority: P3)

**Goal**: Admin can soft-delete a category that has no active questions. Deletion is blocked (409) when active questions exist. `DELETE /api/v1/categories/:id` is live.

**Independent Test**: Using the server from Phases 3–4:
1. Create a category "ToDelete" (via POST)
2. `DELETE /api/v1/categories/{id}` → 200 `{ "data": { "ok": true } }`
3. `GET /api/v1/categories` → category no longer appears
4. `DELETE /api/v1/categories/{id}` (same id, now deleted) → 404
5. Create a category "HasQuestions"; insert a non-deleted question referencing it (direct DB seed or via future question API)
6. `DELETE /api/v1/categories/{hasQuestions-id}` → 409 `CATEGORY_HAS_QUESTIONS` with message "Cannot delete category with N active questions"
7. `DELETE /api/v1/categories/00000000-0000-0000-0000-000000000000` → 404

- [x] T010 [US3] Implement `Delete` in `backend/internal/category/service.go` — replace the stub: parse id (already a uuid.UUID from caller), call `repo.FindByID`, if nil return `ErrCategoryNotFound`, call `repo.CountActiveQuestions(ctx, id)`, if count > 0 return `ErrCategoryHasQuestions{Count: count}`, call `repo.Delete(ctx, id)`, return nil on success

- [x] T011 [US3] Implement `Delete` handler in `backend/internal/category/handler.go` — replace the stub: parse `:id` path param as `uuid.UUID` (return 422 on parse error), call `svc.Delete`, map `ErrCategoryNotFound` → 404 `NOT_FOUND`, map `ErrCategoryHasQuestions` → 409 with code `CATEGORY_HAS_QUESTIONS` and message `fmt.Sprintf("Cannot delete category with %d active questions", err.Count)`, return 200 `{ "data": { "ok": true } }` on success

- [x] T012 [US3] Register delete route in `backend/cmd/server/main.go` — add `protected.Delete("/categories/:id", categoryHandler.Delete)` alongside the existing category routes

**Checkpoint**: All four category endpoints fully functional. All three user stories independently testable.

---

## Phase 6: Polish & Cross-Cutting Concerns

**Purpose**: Finalize wiring, update project memory, and verify all stories work together.

- [x] T013 [P] Update `CLAUDE.md` Active Technologies and Recent Changes sections — confirm `internal/category/` package is listed; remove stale note about `golang.org/x/crypto` being missing from go.mod (it is present in go.sum/go.mod at v0.50.0)

- [x] T014 End-to-end smoke test of all four endpoints in sequence — run the server locally (`docker compose up` or `go run ./cmd/server`), obtain admin JWT, execute the full sequence: list (empty), create "Storia", create "Arte", list (2 items), rename "Storia" → "Storia Antica", list (updated slug visible), attempt delete "Arte" with mock question (confirm 409), delete "Storia Antica" (confirm 200), list (1 item remaining)

---

## Dependencies & Execution Order

### Phase Dependencies

- **Setup (Phase 1)**: No dependencies — start immediately
- **Foundational (Phase 2)**: Depends on Phase 1 — **BLOCKS** all user story phases
- **US1 Phase (Phase 3)**: Depends on Phase 2 completion
- **US2 Phase (Phase 4)**: Depends on Phase 2 completion (can overlap with Phase 3 if `service.go` stub is already created)
- **US3 Phase (Phase 5)**: Depends on Phase 2 completion (can overlap with Phases 3–4)
- **Polish (Phase 6)**: Depends on all story phases completing

### Within Each User Story

```
models.go + repository.go (T002, T003)
       ↓
   service.go stub (T004)
       ↓
   handler.go stub (T005)
       ↓
   main.go wiring (T006)
       ↓ [US1 DONE]

   service Rename (T007) → handler Rename (T008) → main.go route (T009)
       [US2 DONE — depends on T004 service interface only]

   service Delete (T010) → handler Delete (T011) → main.go route (T012)
       [US3 DONE — depends on T004 service interface only]
```

### Parallel Opportunities

Within Phase 2: T002 and T003 touch different files — they can be written in parallel.

Within Phase 3: T004 and T005 touch different files — handler stub can be written while service is being implemented, as long as the Service interface is defined first in T004.

US2 (Phase 4) and US3 (Phase 5) can be worked in parallel once the service interface stub (T004) exists — they implement different service methods and different handler methods.

---

## Parallel Execution Examples

### Phase 2 — Parallel

```bash
# These two files have no dependency on each other:
Task T002: "Create backend/internal/category/models.go"
Task T003: "Create backend/internal/category/repository.go"
```

### Phases 4 + 5 — Parallel (after T004 completes)

```bash
# After service interface is defined in T004:
Task T007: "Implement Rename in service.go"    ← US2
Task T010: "Implement Delete in service.go"    ← US3
# (different methods, same file — serialize if single developer)

# After their respective service methods:
Task T008: "Implement Rename handler"          ← US2
Task T011: "Implement Delete handler"          ← US3
# (different methods, same file — serialize if single developer)
```

---

## Implementation Strategy

### MVP First (User Story 1 Only)

1. Complete Phase 1 + Phase 2 (T001–T003)
2. Complete Phase 3 (T004–T006)
3. **STOP and VALIDATE**: `GET /api/v1/categories` and `POST /api/v1/categories` work correctly
4. Deploy / demo the partial feature

### Incremental Delivery

1. Phase 1 + 2 → Foundation ready
2. Phase 3 (US1) → List + Create live → validate → demo
3. Phase 4 (US2) → Rename live → validate
4. Phase 5 (US3) → Delete live → validate
5. Phase 6 → Polish + full smoke test → commit

### Single-Developer Sequence

```
T001 → T002 → T003 → T004 → T005 → T006
                                     ↓ [US1 complete]
                        T007 → T008 → T009
                                     ↓ [US2 complete]
                        T010 → T011 → T012
                                     ↓ [US3 complete]
                        T013 → T014
                                     ↓ [Done]
```

---

## Notes

- [P] tasks write to different files and have no dependency on each other's output
- Sentinel error `ErrCategoryHasQuestions` is a struct (not a simple `var`) so it can carry `Count int` — use `errors.As` in the handler, not `errors.Is`
- The `protected` group in `main.go` already applies `RequireAdmin(cfg)` middleware — no additional auth work needed
- The `categories_name_unique` and `categories_slug_unique` partial indexes enforce uniqueness at the DB level; the application maps the `23505` Postgres error code to `ErrCategoryNameConflict`
- No migration is needed — `categories` table is fully defined in `backend/migrations/001_initial_schema.up.sql`
- Stub methods in Phase 3 (`Rename` returning `errors.New("not implemented")`, `Delete` returning same) allow the package to compile during US1 verification before US2/US3 are implemented
