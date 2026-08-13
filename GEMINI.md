# QuizShow — Project Instructions & Antigravity Guidelines

## Project Overview

QuizShow is a web application for managing and running live quiz shows. Three distinct browser views (**Admin Panel**, **Presenter Mode + Projection Screen**, and **Player View**) communicate in real-time via WebSocket.

## Stack

| Layer | Technology |
|---|---|
| Frontend | React + Vite + TypeScript |
| Styling | Tailwind CSS |
| Backend | Go 1.25 + Fiber (v2) |
| Realtime | WebSocket (native Go, no external broker) |
| Database | PostgreSQL (pgx/v5) |
| Auth (MVP) | JWT self-issued by Go backend |
| Auth (R2+) | Keycloak (same JWT validation contract, different issuer) |
| Infrastructure | Docker Compose |

## Monorepo Structure

```
quizshow/
├── GEMINI.md                   # Antigravity project memory & instructions
├── CLAUDE.md                   # Claude Code project memory
├── .agents/
│   └── skills/                 # Workspace specKit skills
├── .specify/
│   └── constitution.md
├── docs/
│   ├── 00-overview.md
│   ├── 01-mvp-requirements.md
│   ├── 02-data-model.md
│   ├── 03-scoring-mechanics.md
│   ├── 04-api-design.md
│   ├── 05-websocket-events.md    # TODO
│   └── 06-ui-flows.md            # TODO
├── specs/                         # specKit feature specifications and plans
├── frontend/
│   ├── admin/                     # React app — Admin Panel
│   ├── presenter/                 # React app — Presenter Mode + Projection Screen
│   └── player/                   # React app — Player View (mobile-first)
├── backend/
│   ├── cmd/server/
│   ├── internal/
│   │   ├── auth/
│   │   ├── category/
│   │   ├── question/
│   │   ├── session/
│   │   ├── player/
│   │   └── websocket/
│   └── migrations/
└── docker-compose.yaml
```

## Key Architectural Decisions

- **WebSocket hub per session**: Each active session has its own Go goroutine managing connected clients. Messages are broadcast to all clients in the same session hub.
- **Auth is JWT-agnostic**: The Go middleware validates JWTs by checking signature + claims. The issuer URL is configurable via environment variable — swapping to Keycloak in R2 requires no code change.
- **Player identity is ephemeral in MVP**: Players join with nickname + PIN, no persistent account. Player state lives only for the session duration.
- **Questions are selected at session start**: When admin launches a live session, the backend randomly selects questions from the configured categories/pool. The selection is fixed for the duration of the session.
- **Soft deletes**: All entities use `deleted_at` timestamp. Nothing is hard-deleted.

## Development Conventions

- **Go Layout**: Follow standard layout (`cmd/`, `internal/`). No `pkg/` package for now.
- **Database Migrations**: Managed with `golang-migrate`. Migration files reside in `backend/migrations/`.
- **Frontend Kit**: Frontend apps share a common `ui-kit` package (Tailwind components, types).
- **API Response Envelope**: All API responses follow the envelope:
  - Success: `{ "data": ..., "error": null }`
  - Error: `{ "data": null, "error": { "code": "...", "message": "..." } }`
- **Data Formats**:
  - Timestamps: UTC, ISO 8601 format.
  - Identifiers: UUIDs v4.

## MVP Scope

See `docs/01-mvp-requirements.md` for the full user story list (26 stories across 4 views).

**In MVP:**
- Admin login (JWT, no Keycloak)
- Question CRUD + CSV import + categories
- Session lifecycle (create → configure → launch → complete)
- PIN-based player join (no account required)
- Full realtime loop: Presenter controls → Projection Screen displays → Player responds
- Basic post-session stats and leaderboard

**Out of MVP (R2+):**
- Keycloak / SSO
- Persistent player accounts and match history
- Tournaments / brackets
- Media (images) in questions
- Advanced analytics and PDF export
- Team/squad mode

## Implementation Order — CRITICAL

> [!IMPORTANT]
> **Do not generate any frontend code until `docs/06-ui-flows.md` exists.**

The frontend design system, component library choices, color palette, and UI flows for all three apps (admin, presenter, player) have not been defined yet. Generating React scaffolding, components, or pages before that document is present will produce inconsistent results that will need to be discarded.

The correct implementation order is:
1. **Backend first** — Go server, migrations, auth, all REST endpoints, WebSocket hub.
2. **`docs/05-websocket-events.md` and `docs/06-ui-flows.md`** will be added before any frontend work begins.
3. **Frontend second** — React apps, only after the UI design document is in place.

If a specKit task or plan includes frontend work and `docs/06-ui-flows.md` does not exist yet: skip the frontend tasks, flag them as blocked, and continue with backend-only tasks.

---

## SpecKit Workflow (Antigravity Skills)

This project uses spec-driven development via SpecKit skills located in `.agents/skills/`. Before implementing any feature:

1. **Specify**: Activate `speckit-specify` to generate/update the feature spec from relevant user stories in `docs/01-mvp-requirements.md`.
2. **Clarify (optional)**: Activate `speckit-clarify` to identify and resolve underspecified areas.
3. **Plan**: Activate `speckit-plan` to generate design artifacts and implementation plan.
4. **Tasks**: Activate `speckit-tasks` to create dependency-ordered tasks in `tasks.md`.
5. **Analyze / Checklist (optional)**: Activate `speckit-analyze` or `speckit-checklist` for consistency validation.
6. **Implement**: Activate `speckit-implement` to execute the tasks systematically.

Spec files live in `specs/`. Never implement a feature without a spec file present.

---

## Active Technologies & Modules

- **Go 1.25 + Fiber v2**, `golang-jwt/jwt` v5, `jackc/pgx` v5, `google/uuid` v1, `golang.org/x/crypto` (bcrypt), `github.com/skip2/go-qrcode`
- **PostgreSQL**: tables `admins`, `refresh_tokens`, `categories`, `questions`, `sessions`, `session_categories`, `session_questions` (all in migration `001_initial_schema.up.sql`)
- **Backend Internal Packages**:
  - `internal/auth/`: JWT issue/validation, password hashing, RequireAdmin middleware, token generation (Admin, Projection).
  - `internal/category/`: Models, repo (pgx), service, handlers (GET/POST/PATCH/DELETE).
  - `internal/question/`: Models, repo with dynamic filtering/pagination, active-session guard, CSV import service, handlers.
  - `internal/session/`: Models, PIN generation with retry loop, CRUD handlers, lifecycle endpoints (`/open-lobby`, `/qr`, `/launch`), question draw with `FOR UPDATE` lock.

## Recent Changes Log

- **001-admin-auth**: Added Go 1.25 + gofiber/fiber v2, golang-jwt/jwt v5, jackc/pgx v5, google/uuid v1, golang.org/x/crypto (bcrypt).
- **002-categories-crud**: Implemented `internal/category/` package; registered 4 routes on protected group; no new migration required.
- **003-questions-crud**: Implemented `internal/question/` package; registered 4 routes (GET/POST/PATCH/DELETE `/questions`) on protected group; dynamic WHERE filters, pointer-field PATCH, EXISTS guard for active sessions.
- **004-questions-csv-import**: CSV import endpoint for bulk question loading with validation against categories.
- **005-session-crud**: Implemented `internal/session/` package; registered 5 routes (POST/GET/GET/:id/PATCH/:id/DELETE/:id `/sessions`) on protected group; PIN generation with 10-attempt retry loop on unique constraint, dynamic SET for PATCH, transactional category replacement, soft-delete with status guard.
- **006-session-lifecycle**: Extended `internal/session/` with 3 new endpoints (POST `/open-lobby`, GET `/qr`, POST `/launch`); QR PNG generation via `skip2/go-qrcode`; transactional question draw with `FOR UPDATE` lock; `IssueProjectionToken` added to `internal/auth/token.go`; `SessionEventBroadcaster` no-op interface; public routes registered before protected group in Fiber.
- **007-presenter-controls**: Implemented 5 presenter control endpoints (POST `/next-question`, POST `/pause-timer`, POST `/resume-timer`, POST `/reveal`, POST `/end`) in `internal/session/`; pure `ScoreAnswer` scoring engine with speed bonus formula and comprehensive table-driven tests; thread-safe in-memory `PauseTracker` with `asked_at` timestamp adjustment on resume; transactional reveal logic updating `answers.points_awarded`, `answers.is_correct`, `answers.answer_time_ms`, and `players.total_score` with answer distribution and top 5 leaderboard; expanded `SessionEventBroadcaster` with typed lifecycle hooks and no-op stub.
- **008-player-join-answer**: Implemented player join (POST `/sessions/:id/join`) and answer submission (POST `/sessions/:session_id/answers`); player JWT issuance (`IssuePlayerToken` with 4h TTL) and `RequirePlayer` middleware in `internal/auth/`; curated 12-color hex avatar palette in `internal/session/avatar.go`; server-side question timer validation and idempotent duplicate answer handling; expanded `SessionEventBroadcaster` with `BroadcastPlayerJoined` and `BroadcastAnswerCountUpdated` lifecycle hooks.

