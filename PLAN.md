# QuizShow — Implementation Plan

This file tracks the current implementation status and next actions.
Update it at the end of every Claude Code session.

---

## Current status

**Phase:** Backend implementation in progress — auth complete
**Last updated:** 2025-04-20
**Active branch:** `feature/US-A01-A02-auth` (ready to merge into develop)

---

## Design documents

| Document | Status | Notes |
|---|---|---|
| `docs/00-overview.md` | Done | Vision, stack, ADR |
| `docs/01-mvp-requirements.md` | Done | 26 user stories across 4 views |
| `docs/02-data-model.md` | Done | ERD, SQL schema, migration 001 |
| `docs/03-scoring-mechanics.md` | Done | Speed bonus formula, difficulty R2 plan |
| `docs/04-api-design.md` | Done | All REST endpoints + WebSocket events |
| `docs/05-websocket-events.md` | Not started | Generate in Claude Web before WebSocket hub work |
| `docs/06-ui-flows.md` | Not started | Generate in Claude Web before any frontend work |
| `.specify/constitution.md` | Done | Consolidated via `/speckit.constitution` |

---

## Implementation phases

### Phase 1 — Backend core (current)

Goal: working Go server with all REST endpoints, database, and auth. No frontend.

#### 1.1 Project bootstrap
- [x] `specify init . --ai claude` — initialize specKit
- [x] `/speckit.constitution` — consolidate principles from `.specify/constitution.md`
- [x] Go module init (`go mod init github.com/yourname/quizshow`)
- [x] Fiber server skeleton in `backend/cmd/server/`
- [x] Docker Compose wired up: backend + PostgreSQL
- [ ] `golang-migrate` configured, migration 001 runs cleanly — pending live DB (T034)

#### 1.2 Auth (US-A01, US-A02)
- [x] `POST /api/v1/auth/login`
- [x] `POST /api/v1/auth/refresh`
- [x] `POST /api/v1/auth/logout`
- [x] JWT middleware (admin role)
- [x] Admin seed on first run (from env vars)
- [ ] T034 end-to-end smoke test — pending live DB

#### 1.3 Question management (US-Q01–US-Q05)
- [ ] `GET /api/v1/categories`
- [ ] `POST /api/v1/categories`
- [ ] `PATCH /api/v1/categories/:id`
- [ ] `DELETE /api/v1/categories/:id`
- [ ] `GET /api/v1/questions` (paginated + filtered)
- [ ] `POST /api/v1/questions`
- [ ] `PATCH /api/v1/questions/:id`
- [ ] `DELETE /api/v1/questions/:id`
- [ ] `POST /api/v1/questions/import` (CSV, sincrono, max 500 righe)
- [ ] `GET /api/v1/questions/import/template`

#### 1.4 Session lifecycle (US-S01–US-S04)
- [ ] `POST /api/v1/sessions`
- [ ] `GET /api/v1/sessions`
- [ ] `GET /api/v1/sessions/:id`
- [ ] `PATCH /api/v1/sessions/:id`
- [ ] `DELETE /api/v1/sessions/:id`
- [ ] `POST /api/v1/sessions/:id/open-lobby`
- [ ] `GET /api/v1/sessions/:id/qr`
- [ ] `POST /api/v1/sessions/:id/launch` (question draw + projection token)

#### 1.5 Live session — presenter controls (US-P02–US-P06)
- [ ] `POST /api/v1/sessions/:id/next-question`
- [ ] `POST /api/v1/sessions/:id/pause-timer`
- [ ] `POST /api/v1/sessions/:id/resume-timer`
- [ ] `POST /api/v1/sessions/:id/reveal` (scoring via `ScoreAnswer()`)
- [ ] `POST /api/v1/sessions/:id/end`

#### 1.6 Player endpoints (US-PL01, US-PL03)
- [ ] `POST /api/v1/sessions/:id/join` (PIN + nickname → player JWT)
- [ ] `POST /api/v1/sessions/:session_id/answers`

#### 1.7 Stats endpoints (US-ST01, US-ST02)
- [ ] `GET /api/v1/sessions/:id/leaderboard` (+ `?format=csv`)
- [ ] `GET /api/v1/sessions/:id/stats`

#### 1.8 WebSocket hub
- [ ] Generate `docs/05-websocket-events.md` (do this in Claude Web first)
- [ ] Hub goroutine per session
- [ ] Role-based event routing (admin / player / projection)
- [ ] Events: `connected`, `player_joined`, `player_disconnected`
- [ ] Events: `session_started`, `question_started`, `answer_count_updated`
- [ ] Events: `timer_paused`, `timer_resumed`, `question_revealed`, `player_result`
- [ ] Events: `session_ended`
- [ ] Ping/pong heartbeat

---

### Phase 2 — Frontend (blocked until `docs/06-ui-flows.md` exists)

Do not start this phase until:
1. `docs/06-ui-flows.md` is generated (Claude Web session)
2. `docs/05-websocket-events.md` is generated (Claude Web session)
3. Phase 1 backend is functionally complete

#### 2.1 Design system + ui-kit
- [ ] Component library decision (shadcn/ui recommended — decide in Claude Web)
- [ ] Color palette and typography
- [ ] Shared `frontend/ui-kit` package scaffold

#### 2.2 Admin app (`frontend/admin`)
- [ ] Login page
- [ ] Question bank (list, create, edit, import)
- [ ] Categories management
- [ ] Session create + configure
- [ ] Session detail (PIN, QR code, lobby)
- [ ] Session history
- [ ] Post-session stats + leaderboard

#### 2.3 Presenter + Projection app (`frontend/presenter`)
- [ ] Lobby view (player list, PIN, QR)
- [ ] Live question view (timer, response counter, controls)
- [ ] Reveal view (distribution chart, top 5)
- [ ] Projection screen (read-only, full-screen optimized)
- [ ] Final results screen

#### 2.4 Player app (`frontend/player`)
- [ ] Join screen (PIN + nickname)
- [ ] Lobby waiting screen
- [ ] Answer screen (4 buttons, mobile-first)
- [ ] Post-reveal feedback (score, rank, points animation)
- [ ] Final results screen

---

### Phase 3 — Integration and QA

- [ ] End-to-end flow test: admin → presenter → projection → player
- [ ] Mobile testing (player view, 375px viewport)
- [ ] WebSocket reconnection handling (player drops and rejoins)
- [ ] Timer accuracy validation (pause/resume, server-side computation)
- [ ] Score validation (`ScoreAnswer()` unit tests pass)
- [ ] CSV import edge cases (500 rows, bad data, abort vs skip)

---

## specKit feature backlog

Ordered by recommended implementation sequence.
Each item maps to one `/speckit.specify` invocation.

| # | Feature | User stories | Phase | Status |
|---|---|---|---|---|
| 1 | Auth admin | US-A01, US-A02 | 1.2 | Done — 28/28 tests pass, T034 pending live DB |
| 2 | Categories CRUD | US-Q04 | 1.3 | Not started |
| 3 | Questions CRUD | US-Q01, US-Q02, US-Q05 | 1.3 | Not started |
| 4 | Questions CSV import | US-Q03 | 1.3 | Not started |
| 5 | Session create + configure | US-S01, US-S02 | 1.4 | Not started |
| 6 | Session lifecycle (lobby → active) | US-S03 | 1.4 | Not started |
| 7 | Presenter controls | US-P02, US-P03, US-P04, US-P05, US-P06 | 1.5 | Not started |
| 8 | Player join + answer | US-PL01, US-PL03 | 1.6 | Not started |
| 9 | Stats + leaderboard | US-ST01, US-ST02, US-S04 | 1.7 | Not started |
| 10 | WebSocket hub | US-P01, US-PL02, US-PR01–04 | 1.8 | Not started |

---

## Decisions log

| Date | Decision | Rationale |
|---|---|---|
| 2025-04-20 | Go + Fiber for backend | Native concurrency for WebSocket hubs, clean path to microservices |
| 2025-04-20 | JWT self-issued in MVP, Keycloak in R2 | Issuer-agnostic middleware — upgrade requires only env var change |
| 2025-04-20 | Player identity ephemeral in MVP | No registration friction; token issued on join, TTL 4h |
| 2025-04-20 | Projection Screen gets temporary token at launch | Security without operational complexity of full auth |
| 2025-04-20 | REST for presenter commands, WebSocket push-only | Idempotency and explicit error handling for state mutations |
| 2025-04-20 | Speed bonus linear (not tiered, not rank-based) | No cliff edges, independent of other players, trivially unit-testable |
| 2025-04-20 | Difficulty = filter only in MVP | UX simplicity; multiplier + balanced pool planned for R2 |
| 2025-04-20 | CSV import synchronous, max 500 rows | Sufficient for MVP scale; async job queue not justified |
| 2025-04-20 | Stats on dedicated endpoints, not nested in session detail | Separation of concerns; avoids aggregation on every session fetch |
| 2025-04-20 | `sessions.created_by` nullable FK to admins | MVP has one admin so visibility is global; field ready for R2 multi-admin filtering without migration |

---

## Next session checklist

Before opening Claude Code:
1. Merge `feature/US-A01-A02-auth` → `develop`
2. `docker-compose up db` — run T034 smoke test with live DB
3. `git checkout -b feature/US-Q04-categories` from develop
4. `/speckit.specify` for feature #2 (categories CRUD, US-Q04)