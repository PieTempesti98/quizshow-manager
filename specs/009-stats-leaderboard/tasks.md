# Tasks: Feature #9 — Stats & Leaderboard

**Input**: Design documents from `/specs/009-stats-leaderboard/`  
**Prerequisites**: plan.md ✅, spec.md ✅, research.md ✅, data-model.md ✅, contracts/ ✅, quickstart.md ✅

**Organization**: Tasks are grouped by foundational data structures, user stories (leaderboard, stats, history), route registration, and test verification.  
**Tests**: Unit tests for CSV formatting, filename slugification, leaderboard tie-breaking, status guards, and SQL stats aggregation are included per Constitution Principle 6.1. End-to-end smoke verification steps are defined in `quickstart.md`.

## Format: `[ID] [P?] [Story] Description`

- **[P]**: Can run in parallel (different files, no dependencies on incomplete tasks)
- **[Story]**: Which user story this task belongs to (US1 = final leaderboard & CSV, US2 = question breakdown stats, US3 = session history & routes)

---

## Phase 1: Setup & Foundational Infrastructure

**Purpose**: Core DTOs, sentinel errors, CSV generator helper, and repository/service interface definitions.

- [x] T001 [P] Define `SessionLeaderboardEntry`, `SessionLeaderboardResult`, `QuestionStatsItem`, `SessionStatsResult`, and sentinel error `ErrSessionNotCompleted` in `backend/internal/session/models.go`
- [x] T002 [P] Implement `GenerateLeaderboardCSV(entries []SessionLeaderboardEntry) ([]byte, error)` and `FormatLeaderboardCSVFilename(name string, endedAt *time.Time, fallbackCreatedAt time.Time) string` in `backend/internal/session/csv.go`
- [x] T003 Extend `SessionRepo` interface in `backend/internal/session/repository.go` and `Service` interface in `backend/internal/session/service.go` with `GetLeaderboard`, `GetLeaderboardCSV`, and `GetStats` method signatures

**Checkpoint**: Foundation ready — user story implementations can proceed.

---

## Phase 2: User Story 1 — Final Leaderboard & CSV Export (Priority: P1) 🎯

**Goal**: Protected endpoint `GET /api/v1/sessions/:id/leaderboard` enforces `completed`/`cancelled` status guard, returns final standings ordered by `total_score DESC, joined_at ASC, id ASC` with 1-based ranks in JSON, and supports `?format=csv` downloading an RFC 4180 spreadsheet.

**Independent Test**: Advance a session to `completed` with multiple players. Call `GET /api/v1/sessions/:id/leaderboard` -> verify HTTP 200 with sequential ranks and scores. Call with `?format=csv` -> verify HTTP 200 with `Content-Type: text/csv; charset=utf-8`, attachment filename header, and CSV data matching JSON ranks. Call on draft/active session -> verify HTTP 409 `SESSION_NOT_COMPLETED`.

### Implementation for User Story 1

- [x] T004 [US1] Implement `SessionRepository.GetLeaderboard` in `backend/internal/session/repository.go`: verify session exists and `status IN ('completed', 'cancelled')` (returning `ErrSessionNotCompleted` otherwise), execute leaderboard query with `ROW_NUMBER() OVER (ORDER BY p.total_score DESC, p.joined_at ASC, p.id ASC)`, and return session details and player entries
- [x] T005 [US1] Implement `service.GetLeaderboard` and `service.GetLeaderboardCSV` in `backend/internal/session/service.go`: call repository `GetLeaderboard`, build `SessionLeaderboardResult` for JSON, and generate CSV bytes and filename for export
- [x] T006 [US1] Implement `Handler.GetLeaderboard` in `backend/internal/session/handler.go`: parse `:id` UUID, detect `?format=csv` query param, call service, set CSV headers (`Content-Type`, `Content-Disposition`) on download or return JSON data envelope, mapping `ErrSessionNotFound` (404) and `ErrSessionNotCompleted` (409 `SESSION_NOT_COMPLETED`)

**Checkpoint**: `GET /api/v1/sessions/:id/leaderboard` (JSON and CSV) is fully functional and testable.

---

## Phase 3: User Story 2 — Post-Session Question Performance Breakdown (Priority: P1) 🎯

**Goal**: Protected endpoint `GET /api/v1/sessions/:id/stats` enforces `completed`/`cancelled` status guard and aggregates question performance breakdown (`position`, `question_text`, `correct_index`, `correct_count`, `wrong_count`, `no_answer_count`, answer distribution, and `avg_answer_time_ms`) ordered by `position ASC`.

**Independent Test**: Advance a session with answers to `completed`. Call `GET /api/v1/sessions/:id/stats` -> verify HTTP 200 returning questions sorted 1..N where `correct_count + wrong_count + no_answer_count == total_players`. Call on draft/active session -> verify HTTP 409 `SESSION_NOT_COMPLETED`.

### Implementation for User Story 2

- [x] T007 [US2] Implement `SessionRepository.GetStats` in `backend/internal/session/repository.go`: verify session exists and `status IN ('completed', 'cancelled')` (returning `ErrSessionNotCompleted` otherwise), query total player count, execute per-question aggregation query joining `session_questions`, `questions`, and `answers`, compute `no_answer_count = max(0, totalPlayers - (correctCount + wrongCount))`, and assemble `QuestionStatsItem` list ordered by `position ASC`
- [x] T008 [US2] Implement `service.GetStats` in `backend/internal/session/service.go`: call repository `GetStats` and return `SessionStatsResult`
- [x] T009 [US2] Implement `Handler.GetStats` in `backend/internal/session/handler.go`: parse `:id` UUID, call service, return HTTP 200 with standard data envelope, mapping `ErrSessionNotFound` (404) and `ErrSessionNotCompleted` (409 `SESSION_NOT_COMPLETED`)

**Checkpoint**: `GET /api/v1/sessions/:id/stats` is fully functional and testable.

---

## Phase 4: User Story 3 — Session History & Route Registration (Priority: P2)

**Goal**: Register leaderboard and stats routes on the protected admin group in the HTTP server, ensuring full compatibility with history filtering (`GET /api/v1/sessions?status=completed,cancelled`).

- [x] T010 [US3] Register `protected.Get("/sessions/:id/leaderboard", sessionHandler.GetLeaderboard)` and `protected.Get("/sessions/:id/stats", sessionHandler.GetStats)` in `backend/cmd/server/main.go` and verify that `GET /api/v1/sessions` correctly lists history when filtered by `status=completed,cancelled`

---

## Phase 5: Polish, Testing & Verification

**Purpose**: Unit tests, integration validation, smoke testing, and updating project documentation.

- [x] T011 [P] Implement unit tests for CSV generation and filename slugification in `backend/internal/session/csv_test.go`
- [x] T012 [P] Implement unit & integration tests for leaderboard ranking, tie-breaking, status guards, and stats aggregation in `backend/internal/session/stats_test.go`
- [x] T013 Run backend automated test suite (`cd backend && go test -v ./internal/session/...`)
- [x] T014 Execute end-to-end smoke test sequence from `specs/009-stats-leaderboard/quickstart.md`
- [x] T015 [P] Update `GEMINI.md` Recent Changes Log and `PLAN.md` with Feature 009 summary

---

## Dependencies & Execution Order

### Phase Dependencies

- **Foundational (Phase 1)**: Can start immediately.
- **User Story 1 (Phase 2)**: Depends on Phase 1.
- **User Story 2 (Phase 3)**: Depends on Phase 1.
- **Session History & Routes (Phase 4)**: Depends on Phases 2 & 3.
- **Polish & Verification (Phase 5)**: Depends on Phase 4.

### Parallel Opportunities

- Within **Phase 1**: T001 (`models.go`) and T002 (`csv.go`) can be written in parallel.
- Across **User Stories**: Phase 2 (US1) and Phase 3 (US2) can be implemented in parallel once Phase 1 is complete.
- Within **Phase 5**: T011 (`csv_test.go`), T012 (`stats_test.go`), and T015 documentation updates can be worked on in parallel.

---

## Implementation Strategy

### MVP First (Phases 1, 2, 3, 4)

1. Setup models, DTOs, errors, and CSV generator helper.
2. Complete US1 (`GET /leaderboard` JSON + CSV) -> verify ranking and file download.
3. Complete US2 (`GET /stats`) -> verify per-question aggregation.
4. Register routes in `main.go`.
5. Execute unit tests and quickstart verification.
