# Tasks: Feature #8 — Player Join & Answer Submission

**Input**: Design documents from `/specs/008-player-join-answer/`  
**Prerequisites**: plan.md ✅, spec.md ✅, research.md ✅, data-model.md ✅, contracts/ ✅, quickstart.md ✅

**Organization**: Tasks are grouped by foundational auth/domain infrastructure and user stories to enable independent implementation and testing.  
**Tests**: Unit tests for player join, answer timer calculations, and auth token issuance are included per Constitution Principle 6.1. End-to-end smoke verification steps are defined in `quickstart.md`.

## Format: `[ID] [P?] [Story] Description`

- **[P]**: Can run in parallel (different files, no dependencies on incomplete tasks)
- **[Story]**: Which user story this task belongs to (US1 = player join lobby, US2 = submit answer)

---

## Phase 1: Setup & Auth Extensions

**Purpose**: Implement Player JWT token issuance, validation, player auth middleware, and avatar color assignment helper.

- [x] T001 Implement `PlayerClaims`, `IssuePlayerToken(playerID, sessionID uuid.UUID, cfg Config)`, and `ValidatePlayerClaims(tokenString, cfg Config)` in `backend/internal/auth/token.go`
- [x] T002 [P] Implement `RequirePlayer(cfg Config) fiber.Handler` middleware injecting `PlayerClaims` into context in `backend/internal/auth/middleware.go`
- [x] T003 [P] Implement curated hex color palette and `AssignAvatarColor(nickname string) string` helper in `backend/internal/session/avatar.go`

---

## Phase 2: Foundational (Blocking Prerequisites)

**Purpose**: Core player/answer models, sentinel errors, DTOs, repository & service interface extensions, and expanded broadcaster hooks.

**⚠️ CRITICAL**: Must be complete before user story implementation begins.

- [x] T004 [P] Add player/answer models (`Player`, `Answer`), result DTOs (`PlayerJoinResult`, `AnswerSubmitResult`), sentinel errors (`ErrNicknameTaken`, `ErrInvalidPIN`, `ErrQuestionClosed`, `ErrQuestionNotFound`, `ErrForbiddenSession`), and extend `SessionEventBroadcaster` with `BroadcastPlayerJoined` and `BroadcastAnswerCountUpdated` (and update `noopBroadcaster`) in `backend/internal/session/models.go`
- [x] T005 Extend `SessionRepo` interface in `backend/internal/session/repository.go` and `Service` interface in `backend/internal/session/service.go` with `JoinPlayer` / `Join` and `SubmitAnswer` method signatures

**Checkpoint**: Foundation ready — user story phases can proceed.

---

## Phase 3: User Story 1 — Join Session Lobby (Priority: P1) 🎯

**Goal**: Public endpoint `POST /api/v1/sessions/:id/join` validates PIN, checks `lobby` status, prevents nickname duplication, registers the player, issues a 4-hour Player JWT, and emits `BroadcastPlayerJoined`.

**Independent Test**: Call `POST /api/v1/sessions/:id/join` with valid PIN and unique nickname on a session in `lobby` status. Verify HTTP 201 returning `player_id`, `nickname`, `avatar_color`, `session_id`, `session_name`, `player_token`, and `token_expires_at`. Calling with duplicate nickname returns HTTP 409 `NICKNAME_TAKEN`. Calling on draft/active session returns HTTP 409 `SESSION_NOT_IN_LOBBY`.

### Implementation for User Story 1

- [x] T006 [US1] Implement `SessionRepository.JoinPlayer` in `backend/internal/session/repository.go`: query session (verify status is `lobby` and PIN matches), insert into `players` (`id`, `session_id`, `nickname`, `avatar_color`, `total_score`, `joined_at`), catch unique violation on `(session_id, nickname)` returning `ErrNicknameTaken`, and count `totalPlayers`
- [x] T007 [US1] Implement `service.Join` in `backend/internal/session/service.go`: validate PIN format (6 digits) and nickname length (2–20 chars), generate `avatar_color` via `AssignAvatarColor`, call `s.repo.JoinPlayer`, issue player JWT via `auth.IssuePlayerToken`, dispatch `s.broadcaster.BroadcastPlayerJoined(...)`, and return `PlayerJoinResult`
- [x] T008 [US1] Implement `Handler.Join` in `backend/internal/session/handler.go`: parse `:id` UUID, decode JSON request body `{pin, nickname}`, call `service.Join`, map `ErrSessionNotFound` / `ErrInvalidPIN` (404), `ErrSessionNotInLobby` (409 `SESSION_NOT_IN_LOBBY`), `ErrNicknameTaken` (409 `NICKNAME_TAKEN`), and return HTTP 201 with standard data envelope

**Checkpoint**: `POST /sessions/:id/join` is functional and testable.

---

## Phase 4: User Story 2 — Submit Answer (Priority: P1) 🎯

**Goal**: Protected endpoint `POST /api/v1/sessions/:session_id/answers` validates player credentials, enforces question timer on the server, calculates `answer_time_ms`, idempotently handles duplicate submissions, persists the answer, and emits `BroadcastAnswerCountUpdated`.

**Independent Test**: Advance an active session to a question. Submit answer as Player 1 (`POST /api/v1/sessions/:session_id/answers`). Verify HTTP 201 returning `answer_id`, `chosen_index`, and `answered_at`. Resubmit the same question -> verify HTTP 200 with the identical `answer_id` (idempotent). Submit after timer expiration -> verify HTTP 409 `QUESTION_CLOSED`.

### Implementation for User Story 2

- [x] T009 [US2] Implement `SessionRepository.SubmitAnswer` in `backend/internal/session/repository.go`:
  - Check if player already answered `(player_id, session_question_id)`: if found, return existing answer with `isDuplicate = true`
  - Query active question and session settings, verify question belongs to session, `asked_at IS NOT NULL`, and `revealed_at IS NULL`
  - Calculate `elapsedMs = now.Sub(asked_at).Milliseconds()`; if `elapsedMs > time_per_question_s * 1000`, return `ErrQuestionClosed`
  - Insert row into `answers (id, player_id, session_question_id, chosen_index, is_correct, points_awarded, answer_time_ms, answered_at)` with `answer_time_ms = max(0, int(elapsedMs))`
  - Query `answeredCount` and `totalPlayers`, returning `(answer, isDuplicate, answeredCount, totalPlayers, nil)`
- [x] T010 [US2] Implement `service.SubmitAnswer` in `backend/internal/session/service.go`: call `s.repo.SubmitAnswer`, if `!isDuplicate` dispatch `s.broadcaster.BroadcastAnswerCountUpdated(...)`, return `AnswerSubmitResult`
- [x] T011 [US2] Implement `Handler.SubmitAnswer` in `backend/internal/session/handler.go`: parse `:session_id` UUID, extract `PlayerClaims` from context, verify `claims.SessionID == sessionID` (HTTP 403 `FORBIDDEN`), decode `{session_question_id, chosen_index}` (validating chosen index 0..3), call `service.SubmitAnswer`, map `ErrQuestionClosed` (409 `QUESTION_CLOSED`), `ErrQuestionNotFound` (404), return HTTP 201 for new submissions or HTTP 200 for idempotent duplicates

**Checkpoint**: `POST /sessions/:session_id/answers` is functional, idempotent, and testable.

---

## Phase 5: Router Integration

**Purpose**: Register public and player-protected routes on the Fiber server.

- [x] T012 Register public `POST /sessions/:id/join` route and protected `POST /sessions/:session_id/answers` route with `auth.RequirePlayer(cfg.Auth)` middleware in `backend/cmd/server/main.go`

---

## Phase 6: Polish, Testing & Verification

**Purpose**: Automated unit tests, integration verification, smoke testing, and updating project memory.

- [x] T013 [P] Implement unit tests for Player JWT claims in `backend/internal/auth/token_test.go` and player join / answer submission logic in `backend/internal/session/player_test.go`
- [x] T014 Run backend automated test suite (`cd backend && go test -v ./internal/session/... ./internal/auth/...`)
- [x] T015 Execute end-to-end smoke test sequence from `specs/008-player-join-answer/quickstart.md`
- [x] T016 [P] Update `GEMINI.md` Recent Changes Log with Feature 008 summary

---

## Dependencies & Execution Order

### Phase Dependencies

- **Setup & Auth Extensions (Phase 1)**: Can start immediately.
- **Foundational (Phase 2)**: Depends on Phase 1.
- **User Story 1 (Phase 3)**: Depends on Phase 2.
- **User Story 2 (Phase 4)**: Depends on Phase 2 & Phase 3.
- **Router Integration (Phase 5)**: Depends on Phases 3 & 4.
- **Polish & Verification (Phase 6)**: Depends on Phase 5.

### Parallel Opportunities

- Within **Phase 1**: T001, T002, T003 can be written in parallel across their respective files.
- Within **Phase 2**: T004 (`models.go`) and T005 (`repository.go`/`service.go` interfaces) can proceed quickly.
- Within **Phase 6**: T013 (`player_test.go`) and T016 (`GEMINI.md`) can be prepared in parallel.

---

## Implementation Strategy

### MVP First (Phases 1, 2, 3, 4, 5)

1. Complete Setup (auth extensions + avatar color helper).
2. Complete Foundational (models, errors, interface definitions).
3. Complete US1 (`POST /join`) -> verify player joins lobby, receives JWT, avatar color assigned.
4. Complete US2 (`POST /answers`) -> verify answer submission, idempotency, timer enforcement.
5. Register routes on Fiber router (Phase 5).
6. Run unit tests & end-to-end smoke tests (Phase 6).
