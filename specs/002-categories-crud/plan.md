# Implementation Plan: Categories CRUD

**Branch**: `002-categories-crud` | **Date**: 2026-04-21 | **Spec**: [spec.md](spec.md)  
**Input**: Feature specification from `/specs/002-categories-crud/spec.md`

## Summary

Implement the four category management endpoints (list, create, rename, soft-delete) for the QuizShow admin panel, as defined in US-Q04. The implementation follows the existing `internal/auth/` package pattern: a new `internal/category/` package with `models.go`, `repository.go`, `service.go`, and `handler.go`. No database migration is required — the `categories` table and its partial unique indexes are already present in migration 001. Routes are registered on the existing `protected` Fiber group in `cmd/server/main.go`.

## Technical Context

**Language/Version**: Go 1.25  
**Primary Dependencies**: gofiber/fiber v2, jackc/pgx v5, google/uuid v1 (all already in `go.mod`)  
**Storage**: PostgreSQL — `categories` table + `questions` table (both in migration 001)  
**Testing**: `go test ./...` — unit tests for service logic, integration tests for repository  
**Target Platform**: Linux server (Docker Compose)  
**Project Type**: Web service (REST API)  
**Performance Goals**: Standard web latency — no special targets for category endpoints at MVP scale  
**Constraints**: Must follow envelope response format; all queries must filter `deleted_at IS NULL`  
**Scale/Scope**: Tens of categories, hundreds to low thousands of questions

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-checked after Phase 1 design.*

| Principle | Status | Notes |
|-----------|--------|-------|
| I. Backend-First | ✅ PASS | This feature is backend-only. `docs/06-ui-flows.md` does not exist; no frontend work included. |
| II. Spec-Driven | ✅ PASS | `specs/002-categories-crud/spec.md` exists and is complete. specKit workflow in order. |
| III. Architectural Simplicity (YAGNI) | ✅ PASS | No ORM. Raw pgx queries. No new infrastructure. Slug via simple string transform. |
| IV. Data Integrity | ✅ PASS | Soft deletes (`deleted_at`). UUID v4 PKs. UTC timestamps. Envelope responses. Migration-managed schema. |
| V. Real-Time Isolation | ✅ PASS | Feature is pure REST. No WebSocket involvement. |

**Verdict**: All gates pass. No complexity tracking required.

## Project Structure

### Documentation (this feature)

```text
specs/002-categories-crud/
├── plan.md              ← this file
├── research.md          ← Phase 0 output
├── data-model.md        ← Phase 1 output
├── contracts/
│   └── categories.md    ← Phase 1 output
├── checklists/
│   └── requirements.md  ← spec quality checklist
└── tasks.md             ← Phase 2 output (created by /speckit.tasks)
```

### Source Code (repository root)

```text
backend/
├── cmd/server/
│   └── main.go                   ← register 4 new routes on existing protected group
├── internal/
│   ├── api/
│   │   └── envelope.go           ← unchanged (DataResponse / ErrorResponse already defined)
│   ├── auth/
│   │   └── middleware.go         ← unchanged (RequireAdmin already works)
│   └── category/                 ← NEW package
│       ├── models.go             ← Category, CategoryWithCount structs; slugify()
│       ├── repository.go         ← CategoryRepo interface + CategoryRepository (pgx)
│       ├── service.go            ← Service interface + service struct; sentinel errors
│       └── handler.go            ← HTTP handlers for GET/POST/PATCH/DELETE
└── migrations/
    └── 001_initial_schema.up.sql ← already has categories table; no new migration needed
```

**Structure Decision**: Single `internal/category/` package, mirroring `internal/auth/`. No new top-level directories. All four files follow the same interface-based design (public interface + private concrete struct).

## Complexity Tracking

> No violations. All gates pass. Table not needed.

---

## Phase 0: Research

**Status**: Complete — see [research.md](research.md)

Key decisions resolved:

| Unknown | Resolution |
|---------|-----------|
| Slug generation | Simple ASCII transform: lowercase + spaces→hyphens + strip non-alphanum; see `research.md` Decision 1 |
| Package name | `internal/category/` (singular, idiomatic Go); see Decision 2 |
| `question_count` computation | LEFT JOIN + COUNT at query time; CREATE returns hard-coded `0`; see Decision 3 |
| Migration required? | No — schema complete in migration 001; see Decision 4 |
| Error sentinel pattern | Package-level `var Err...` sentinels, matching auth pattern; see Decision 5 |
| Route registration | Add to existing `protected` group in `main.go`; see Decision 6 |

---

## Phase 1: Design & Contracts

**Status**: Complete

### Data Model
See [data-model.md](data-model.md) for:
- Go struct definitions (`Category`, `CategoryWithCount`)
- `slugify()` algorithm
- All SQL queries (LIST, FIND BY ID, CREATE, RENAME, COUNT QUESTIONS, SOFT DELETE)
- Conflict detection via `pgconn.PgError` code `"23505"`

### API Contracts
See [contracts/categories.md](contracts/categories.md) for:
- `GET  /api/v1/categories`     → 200 with `{ categories: [...] }`
- `POST /api/v1/categories`     → 201 with created category object
- `PATCH /api/v1/categories/:id` → 200 with updated category object
- `DELETE /api/v1/categories/:id` → 200 `{ ok: true }` or 409 `CATEGORY_HAS_QUESTIONS`

All endpoints: 401 on missing JWT, 403 on non-admin role, 422 on validation failure, 404 on unknown ID.

---

## Implementation Guide for `/speckit.tasks`

The tasks command should generate tasks in this order:

1. **Create `internal/category/models.go`**  
   - `Category` struct (ID, Name, Slug, CreatedAt, UpdatedAt, DeletedAt)  
   - `CategoryWithCount` struct (embeds Category + QuestionCount int)  
   - `slugify(name string) string` function  
   - Sentinel errors: `ErrCategoryNotFound`, `ErrCategoryNameConflict`, `ErrCategoryHasQuestions`

2. **Create `internal/category/repository.go`**  
   - `CategoryRepo` interface with: `List`, `FindByID`, `Create`, `Update`, `Delete`, `CountActiveQuestions`  
   - `CategoryRepository` struct (holds `*pgxpool.Pool`)  
   - `NewCategoryRepository(pool) *CategoryRepository`  
   - All six query implementations (see data-model.md for SQL)  
   - Map `pgconn.PgError` code `"23505"` → `ErrCategoryNameConflict`

3. **Create `internal/category/service.go`**  
   - `Service` interface with: `List`, `Create`, `Rename`, `Delete`  
   - Private `service` struct holding `CategoryRepo`  
   - `NewService(repo CategoryRepo) Service`  
   - Business logic: call `slugify`, call repo, map sentinel errors  
   - `Delete` calls `CountActiveQuestions` first; returns `ErrCategoryHasQuestions{Count: n}` if n > 0

4. **Create `internal/category/handler.go`**  
   - `Handler` struct holding `Service`  
   - `NewHandler(svc Service) *Handler`  
   - `List(c *fiber.Ctx) error` — GET, returns DataResponse with `categories` array  
   - `Create(c *fiber.Ctx) error` — POST, parse body, validate name, call svc, return 201  
   - `Rename(c *fiber.Ctx) error` — PATCH `:id`, parse UUID, parse body, call svc, return 200  
   - `Delete(c *fiber.Ctx) error` — DELETE `:id`, parse UUID, call svc, return 200 or 409  
   - Map sentinel errors to HTTP codes per contracts/categories.md

5. **Register routes in `cmd/server/main.go`**  
   - Import `internal/category`  
   - Instantiate `categoryRepo`, `categorySvc`, `categoryHandler`  
   - Add to `protected` group:  
     ```
     protected.Get("/categories", categoryHandler.List)
     protected.Post("/categories", categoryHandler.Create)
     protected.Patch("/categories/:id", categoryHandler.Rename)
     protected.Delete("/categories/:id", categoryHandler.Delete)
     ```

6. **Update CLAUDE.md** — add `internal/category/` to Active Technologies section

7. **Tests**  
   - `internal/category/service_test.go` — unit tests with mock repo for all service methods  
   - `internal/category/repository_test.go` — integration tests (requires live DB or testcontainers pattern)  
   - Test cases must cover: happy path, not-found, name conflict, slug conflict, delete-blocked-by-questions
