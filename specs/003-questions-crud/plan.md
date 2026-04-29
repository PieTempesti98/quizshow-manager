# Implementation Plan: Questions CRUD

**Branch**: `003-questions-crud` | **Date**: 2026-04-24 | **Spec**: [spec.md](spec.md)
**Input**: Feature specification from `/specs/003-questions-crud/spec.md`

## Summary

Implement the full CRUD lifecycle for questions in the quiz question bank: paginated/filtered list (GET), create (POST), partial update (PATCH), and soft-delete (DELETE). All four endpoints live under `/api/v1/questions`, require admin JWT, and follow the envelope response pattern already established by the categories feature. The implementation adds a new `internal/question/` package (models, repository, service, handler) wired into `cmd/server/main.go`. No database migration is needed — the `questions` and `session_questions` tables already exist from migration 001.

## Technical Context

**Language/Version**: Go 1.25
**Primary Dependencies**: gofiber/fiber v2, jackc/pgx v5, google/uuid v1
**Storage**: PostgreSQL — `questions`, `categories`, `session_questions`, `sessions` tables (all in migration 001, no new migration required)
**Testing**: Go standard `testing` package + `net/http/httptest` (consistent with existing auth package tests)
**Target Platform**: Linux server via Docker Compose
**Project Type**: web-service (backend REST API only; frontend blocked per Constitution Principle I — `docs/06-ui-flows.md` absent)
**Performance Goals**: List endpoint responds in < 300ms for up to 10,000 questions under standard operating load (per US-Q05 acceptance criteria)
**Constraints**: Soft-deletes only (no hard deletes); UUID v4 IDs; envelope responses; no ORM (raw pgx); per-page maximum of 100
**Scale/Scope**: Single admin user, up to ~10,000 questions in MVP question bank

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

| Principle | Status | Notes |
|-----------|--------|-------|
| I. Backend-First | ✅ PASS | Pure backend plan. `docs/06-ui-flows.md` absent — no frontend work included or planned. |
| II. Spec-Driven | ✅ PASS | `specs/003-questions-crud/spec.md` exists, validated, all 16 checklist items green. |
| III. YAGNI | ✅ PASS | No ORM; raw pgx queries throughout. No message broker. No Keycloak. Soft deletes only. Partial update via pointer-field struct (simplest viable approach). |
| IV. Data Integrity | ✅ PASS | `questions` table already has `deleted_at`. All queries filter `WHERE deleted_at IS NULL`. UUID v4 IDs. Envelope responses. No new migration needed. |
| V. Real-Time Isolation | ✅ PASS | This feature is purely REST CRUD. No WebSocket involvement. |

**Post-design re-check**: All principles hold through Phase 1 design. No violations. Complexity Tracking table is empty (no justified violations needed).

## Project Structure

### Documentation (this feature)

```text
specs/003-questions-crud/
├── plan.md              # This file
├── research.md          # Phase 0 output
├── data-model.md        # Phase 1 output
├── quickstart.md        # Phase 1 output
├── contracts/
│   └── rest-api.md      # Phase 1 output
└── tasks.md             # Phase 2 output (/speckit.tasks — not yet created)
```

### Source Code (repository root)

```text
backend/
├── cmd/server/
│   └── main.go                     # Updated: wire question package
└── internal/
    ├── api/
    │   └── envelope.go             # Unchanged
    ├── auth/                       # Unchanged
    ├── category/                   # Unchanged (reference implementation)
    ├── db/                         # Unchanged
    └── question/                   # NEW package
        ├── models.go               # Question, CategoryRef, QuestionFilter, QuestionUpdate, error sentinels
        ├── repository.go           # QuestionRepo interface + pgx implementation
        ├── service.go              # Service interface + implementation
        └── handler.go              # List, Create, Update, Delete HTTP handlers
```

**Structure Decision**: Single new package `internal/question/` mirroring the 4-file layout of `internal/category/`. Wired in `main.go` following the identical constructor chain: `repo → service → handler → routes on protected group`.

## Complexity Tracking

> No violations. All constitution principles satisfied by the plan as designed.
