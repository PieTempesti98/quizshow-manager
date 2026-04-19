# QuizShow — Claude Code Memory

## Project overview

A web application for managing and running live quiz shows. Three distinct browser views (Admin Panel, Presenter Mode + Projection Screen, Player View) communicate in real-time via WebSocket.

## Stack

| Layer | Technology |
|---|---|
| Frontend | React + Vite + TypeScript |
| Styling | Tailwind CSS |
| Backend | Go + Fiber |
| Realtime | WebSocket (native Go, no external broker) |
| Database | PostgreSQL |
| Auth (MVP) | JWT self-issued by Go backend |
| Auth (R2+) | Keycloak (same JWT validation contract, different issuer) |
| Infrastructure | Docker Compose |

## Monorepo structure

```
quizshow/
├── CLAUDE.md
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
├── specs/                         # specKit output, one file per feature
├── frontend/
│   ├── admin/                     # React app — Admin Panel
│   ├── presenter/                 # React app — Presenter Mode + Projection Screen
│   └── player/                   # React app — Player View (mobile-first)
├── backend/
│   ├── cmd/server/
│   ├── internal/
│   │   ├── auth/
│   │   ├── question/
│   │   ├── session/
│   │   ├── player/
│   │   └── websocket/
│   └── migrations/
└── docker-compose.yml
```

## Key architectural decisions

- **WebSocket hub per session**: each active session has its own Go goroutine managing connected clients. Messages are broadcast to all clients in the same session hub.
- **Auth is JWT-agnostic**: the Go middleware validates JWTs by checking signature + claims. The issuer URL is configurable via environment variable — swapping to Keycloak in R2 requires no code change.
- **Player identity is ephemeral in MVP**: players join with nickname + PIN, no persistent account. Player state lives only for the session duration.
- **Questions are selected at session start**: when admin launches a live session, the backend randomly selects questions from the configured categories/pool. The selection is fixed for the duration of the session.
- **Soft deletes**: all entities use `deleted_at` timestamp. Nothing is hard-deleted.

## Development conventions

- Go packages follow standard layout (`cmd/`, `internal/`). No `pkg/` for now.
- Database migrations managed with `golang-migrate`. Migration files in `backend/migrations/`.
- Frontend apps share a common `ui-kit` package (Tailwind components, types).
- All API responses follow the envelope: `{ "data": ..., "error": null }` or `{ "data": null, "error": { "code": "...", "message": "..." } }`.
- Timestamps are always UTC, ISO 8601 format.
- IDs are UUIDs v4.

## MVP scope

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

## Implementation order — IMPORTANT

**Do not generate any frontend code until `docs/06-ui-flows.md` exists.**

The frontend design system, component library choices, color palette, and UI flows for all three apps (admin, presenter, player) have not been defined yet. Generating React scaffolding, components, or pages before that document is present will produce inconsistent results that will need to be discarded.

The correct implementation order is:

1. **Backend first** — Go server, migrations, auth, all REST endpoints, WebSocket hub
2. **`docs/05-websocket-events.md` and `docs/06-ui-flows.md`** will be added before any frontend work begins
3. **Frontend second** — React apps, only after the UI design document is in place

If a specKit task or plan includes frontend work and `docs/06-ui-flows.md` does not exist yet: skip the frontend tasks, flag them as blocked, and continue with backend-only tasks.

---

## specKit workflow

This project uses spec-driven development via specKit. Before implementing any feature:

1. Run `/speckit.specify` to generate the feature spec from the relevant user stories in `docs/01-mvp-requirements.md`
2. Run `/speckit.plan` to generate the implementation plan
3. Run `/speckit.tasks` to get the task breakdown
4. Review tasks, then run `/speckit.implement`

Spec files live in `specs/`. Never implement a feature without a spec file present.