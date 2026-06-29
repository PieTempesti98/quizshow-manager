# Implementation Plan: Session Create and Configure

**Branch**: `005-session-crud` | **Date**: 2026-05-04 | **Spec**: [spec.md](spec.md)  
**Input**: Feature specification from `/specs/005-session-crud/spec.md`

## Summary

Implement the session CRUD lifecycle for the QuizShow admin — create, list, detail, update, and soft-delete — covering US-S01 (create and configure) and US-S02 (PIN generation). The feature lives entirely in a new `internal/session/` Go package following the same four-layer architecture (models → repository → service → handler) already established by `internal/question/`. No new dependencies or migrations are needed; all tables and indexes are already present in migration 001.

## Technical Context

**Language/Version**: Go 1.25  
**Primary Dependencies**: gofiber/fiber v2, jackc/pgx v5, google/uuid v1, math/rand (stdlib)  
**Storage**: PostgreSQL — `sessions` and `session_categories` tables (migration 001, no new migration)  
**Testing**: Go stdlib `testing` + `pgx` test helpers  
**Target Platform**: Linux server (Docker Compose)  
**Project Type**: Web service (REST API)  
**Performance Goals**: Session list response < 300 ms for up to 500 sessions (matches question list SLA)  
**Constraints**: No ORM (pgx direct SQL), soft deletes everywhere, UUID v4 PKs, envelope responses  
**Scale/Scope**: MVP — single admin, O(hundreds) of sessions

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

| Principle | Status | Notes |
|---|---|---|
| I. Backend-First | ✅ PASS | Pure backend feature; no frontend artifacts |
| II. Spec-Driven | ✅ PASS | spec.md exists and is complete |
| III. Architectural Simplicity | ✅ PASS | No ORM, no broker; direct pgx SQL; hub not touched |
| IV. Data Integrity Standards | ✅ PASS | Soft deletes, UUID v4, envelope responses, UTC timestamps |
| V. Real-Time Isolation | ✅ PASS | No WebSocket in this feature; REST only |

No gate violations. Complexity Tracking table not required.

## Project Structure

### Documentation (this feature)

```text
specs/005-session-crud/
├── plan.md              # This file
├── research.md          # Phase 0 output
├── data-model.md        # Phase 1 output
├── quickstart.md        # Phase 1 output
├── contracts/           # Phase 1 output
│   └── sessions.md
└── tasks.md             # Phase 2 output (/speckit.tasks — NOT created here)
```

### Source Code (repository root)

```text
backend/
├── cmd/server/
│   └── main.go                    # register session routes (existing file, modify)
└── internal/
    └── session/                   # new package
        ├── models.go              # Session, SessionFilter, SessionListResult, SessionUpdate, errors
        ├── repository.go          # SessionRepo interface + pgx implementation
        ├── service.go             # Service interface + business logic
        └── handler.go             # Fiber HTTP handlers
```

No new migration files. No frontend changes.
