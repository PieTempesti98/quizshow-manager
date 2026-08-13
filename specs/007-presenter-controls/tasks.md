# Tasks: Feature #7 — Presenter Controls & Scoring Engine

**Input**: Design documents from `/specs/007-presenter-controls/`  
**Prerequisites**: plan.md ✅, spec.md ✅, research.md ✅, data-model.md ✅, contracts/ ✅, quickstart.md ✅

**Organization**: Tasks are grouped by foundational infrastructure and user stories to enable independent implementation and testing.  
**Tests**: Unit tests for the scoring engine are included in Phase 1 per Constitution Principle 6.1. End-to-end smoke verification steps are in `quickstart.md`.

## Format: `[ID] [P?] [Story] Description`

- **[P]**: Can run in parallel (different files, no dependencies on incomplete tasks)
- **[Story]**: Which user story this task belongs to (US1 = next-question, US2 = reveal & scoring, US3 = pause/resume timer, US4 = end session)

---

## Phase 1: Setup & Pure Scoring Engine

**Purpose**: Implement the pure domain scoring engine and unit test suite before any database or HTTP integration.

- [ ] T001 Implement pure `ScoreAnswer(pointsPerAnswer int, isCorrect bool, speedBonusEnabled bool, timeRemainingMs int64, totalTimeMs int64) int` function in `backend/internal/session/scoring.go`
- [ ] T002 [P] Implement table-driven unit tests for `ScoreAnswer` in `backend/internal/session/scoring_test.go` covering base score, speed bonus multiplier, boundary clamps, wrong answers, and timeouts

---

## Phase 2: Foundational (Blocking Prerequisites)

**Purpose**: Core types, in-memory pause tracker, sentinel errors, DTOs, and expanded broadcaster interface that all user stories depend on.

**⚠️ CRITICAL**: Must be complete before user story implementation begins.

- [ ] T003 [P] Implement thread-safe in-memory `PauseTracker` in `backend/internal/session/pause_tracker.go` with `Pause`, `Resume`, `Clear`, and `IsPaused` methods
- [ ] T004 [P] Add sentinel errors (`ErrSessionNotActive`, `ErrQuestionNotRevealed`, `ErrNoMoreQuestions`, `ErrNoActiveQuestion`, `ErrQuestionAlreadyRevealed`, `ErrTimerAlreadyPaused`, `ErrTimerNotPaused`, `ErrSessionAlreadyEnded`), response DTOs, and expanded `SessionEventBroadcaster` / `noopBroadcaster` in `backend/internal/session/models.go`
- [ ] T005 Extend `SessionRepo` interface in `backend/internal/session/repository.go` and `Service` interface / `service` struct in `backend/internal/session/service.go` with presenter method signatures

**Checkpoint**: Foundation ready — user story phases can proceed.

---

## Phase 3: User Story 1 — Advance Questions Sequentially (Priority: P1) 🎯

**Goal**: `POST /sessions/:id/next-question` advances the active session to the next unasked question, setting `asked_at = now()` and returning sanitized question info without `correct_index`.

**Independent Test**: Call `POST /api/v1/sessions/:id/next-question` on an active session. Verify HTTP 200 with `session_question_id`, `position: 1`, `question` object (without `correct_index`), and `asked_at`. Calling it again before reveal must return HTTP 409 `QUESTION_NOT_REVEALED`.

### Implementation for User Story 1

- [ ] T006 [US1] Implement `SessionRepository.NextQuestion` in `backend/internal/session/repository.go`: verify session is active, ensure no unrevealed active question exists, fetch next unasked question by position, update `asked_at = now()`, return question details without `correct_index`
- [ ] T007 [US1] Implement `service.NextQuestion` in `backend/internal/session/service.go`: call `s.repo.NextQuestion`, invoke `s.broadcaster.BroadcastQuestionStarted(...)`, return `NextQuestionResult`
- [ ] T008 [US1] Implement `Handler.NextQuestion` in `backend/internal/session/handler.go`: parse `:id` UUID, call service, map `ErrSessionNotFound` (404), `ErrSessionNotActive` (409 `SESSION_NOT_ACTIVE`), `ErrQuestionNotRevealed` (409 `QUESTION_NOT_REVEALED`), `ErrNoMoreQuestions` (409 `NO_MORE_QUESTIONS`), return 200 with standard envelope

**Checkpoint**: `POST /next-question` is functional and testable.

---

## Phase 4: User Story 2 — Reveal Answer & Compute Scores (Priority: P1)

**Goal**: `POST /sessions/:id/reveal` reveals the correct answer, evaluates all submitted player answers, computes points via `ScoreAnswer()`, atomically updates `answers` and `players.total_score`, computes answer distribution, and returns top 5 leaderboard.

**Independent Test**: Call `POST /api/v1/sessions/:id/reveal` on an asked question. Verify HTTP 200 with `correct_index`, `answer_distribution` (indices 0..3), `top5` leaderboard, and `revealed_at`. Calling it again must return HTTP 409 `QUESTION_ALREADY_REVEALED`.

### Implementation for User Story 2

- [ ] T009 [US2] Implement `SessionRepository.RevealQuestion` in `backend/internal/session/repository.go`: in a single transaction (`tx`), lock active question, set `revealed_at = now()`, lock answers `FOR UPDATE`, score answers with `ScoreAnswer()`, update `answers` rows (`is_correct`, `points_awarded`, `answer_time_ms`), increment `players.total_score`, compute `answer_distribution` and top 5 leaderboard
- [ ] T010 [US2] Implement `service.Reveal` in `backend/internal/session/service.go`: clear pause state for session in `pauseTracker`, call `s.repo.RevealQuestion`, invoke `s.broadcaster.BroadcastQuestionRevealed(...)`, return `RevealResult`
- [ ] T011 [US2] Implement `Handler.Reveal` in `backend/internal/session/handler.go`: parse `:id` UUID, call service, map `ErrSessionNotFound` (404), `ErrSessionNotActive` (409 `SESSION_NOT_ACTIVE`), `ErrNoActiveQuestion` (409 `NO_ACTIVE_QUESTION`), `ErrQuestionAlreadyRevealed` (409 `QUESTION_ALREADY_REVEALED`), return 200 with standard envelope

**Checkpoint**: `POST /reveal` and scoring engine are functional and testable.

---

## Phase 5: User Story 3 — Pause & Resume Question Timer (Priority: P2)

**Goal**: `POST /sessions/:id/pause-timer` and `POST /sessions/:id/resume-timer` allow presenter to halt countdown and resume it, adjusting `session_questions.asked_at` by the pause duration to eliminate speed bonus calculation drift.

**Independent Test**: Call `POST /pause-timer` on an active question; verify HTTP 200 with `paused_at`. Call `POST /pause-timer` again; verify HTTP 409 `TIMER_ALREADY_PAUSED`. Call `POST /resume-timer`; verify HTTP 200 with `resumed_at` and check that `asked_at` in DB was shifted by the pause duration.

### Implementation for User Story 3

- [ ] T012 [US3] Implement `SessionRepository.GetActiveQuestion` and `SessionRepository.ShiftQuestionAskedAt` in `backend/internal/session/repository.go` to support pause verification and timestamp adjustment (`UPDATE session_questions SET asked_at = asked_at + $1 WHERE id = $2`)
- [ ] T013 [US3] Implement `service.PauseTimer` and `service.ResumeTimer` in `backend/internal/session/service.go`:
  - `PauseTimer`: verify active unrevealed question, record pause in `pauseTracker`, calculate remaining time, invoke `s.broadcaster.BroadcastTimerPaused(...)`
  - `ResumeTimer`: retrieve pause timestamp from `pauseTracker`, calculate pause duration delta, call `s.repo.ShiftQuestionAskedAt`, invoke `s.broadcaster.BroadcastTimerResumed(...)`
- [ ] T014 [US3] Implement `Handler.PauseTimer` and `Handler.ResumeTimer` in `backend/internal/session/handler.go`: parse `:id` UUID, call service, map `ErrSessionNotFound` (404), `ErrSessionNotActive` (409), `ErrNoActiveQuestion` (409), `ErrTimerAlreadyPaused` (409 `TIMER_ALREADY_PAUSED`), `ErrTimerNotPaused` (409 `TIMER_NOT_PAUSED`), return 200 with standard envelope

**Checkpoint**: Timer pause/resume controls are functional and testable.

---

## Phase 6: User Story 4 — End Session & Finalize Results (Priority: P1)

**Goal**: `POST /sessions/:id/end` transitions the session status to `completed`, sets `ended_at = now()`, clears active timers, and broadcasts session conclusion.

**Independent Test**: Call `POST /api/v1/sessions/:id/end` with optional body `{"reason": "completed"}`. Verify HTTP 200 with `status: "completed"` and `ended_at`. Calling subsequent presenter controls on the ended session must return HTTP 409 `SESSION_NOT_ACTIVE`.

### Implementation for User Story 4

- [ ] T015 [US4] Implement `SessionRepository.EndSession` in `backend/internal/session/repository.go`: check session is not already in terminal status (`completed`/`cancelled`), update `sessions SET status = 'completed', ended_at = now(), updated_at = now() WHERE id = $1`
- [ ] T016 [US4] Implement `service.End` in `backend/internal/session/service.go`: clear pause state for session in `pauseTracker`, call `s.repo.EndSession`, invoke `s.broadcaster.BroadcastSessionEnded(...)`, return `EndSessionResult`
- [ ] T017 [US4] Implement `Handler.End` in `backend/internal/session/handler.go`: parse `:id` UUID, parse optional `reason` from request body (default `"completed"`), call service, map `ErrSessionNotFound` (404), `ErrSessionAlreadyEnded` (409 `SESSION_ALREADY_ENDED`), return 200 with standard envelope

**Checkpoint**: `POST /end` is functional and testable.

---

## Phase 7: Router Integration

**Purpose**: Register all 5 new presenter routes on the Fiber server.

- [ ] T018 Register 5 presenter routes on the protected router group in `backend/cmd/server/main.go` (`POST /sessions/:id/next-question`, `POST /sessions/:id/pause-timer`, `POST /sessions/:id/resume-timer`, `POST /sessions/:id/reveal`, `POST /sessions/:id/end`)

---

## Phase 8: Polish & Verification

**Purpose**: Run test suites, execute end-to-end smoke verification, and update project memory.

- [ ] T019 Run automated unit tests in `backend/internal/session/scoring_test.go` (`cd backend && go test -v ./internal/session/...`)
- [ ] T020 Run end-to-end smoke test sequence from `specs/007-presenter-controls/quickstart.md` against live backend instance
- [ ] T021 [P] Update `GEMINI.md` Recent Changes Log with Feature 007 presenter controls summary

---

## Dependencies & Execution Order

### Phase Dependencies

- **Setup & Scoring Engine (Phase 1)**: Can start immediately.
- **Foundational (Phase 2)**: Depends on Phase 1.
- **User Story 1 (Phase 3)**: Depends on Phase 2.
- **User Story 2 (Phase 4)**: Depends on Phase 2 & Phase 3.
- **User Story 3 (Phase 5)**: Depends on Phase 2.
- **User Story 4 (Phase 6)**: Depends on Phase 2.
- **Router Integration (Phase 7)**: Depends on Phases 3, 4, 5, 6 (all handlers implemented).
- **Polish & Verification (Phase 8)**: Depends on Phase 7.

### Parallel Opportunities

- Within **Phase 1**: T001 and T002 can be developed in quick succession or parallel.
- Within **Phase 2**: T003 (`pause_tracker.go`) and T004 (`models.go`) can be written in parallel.
- User Story 3 (Pause/Resume) and User Story 4 (End) can be implemented in parallel after Foundational Phase 2 completes.

---

## Implementation Strategy

### MVP First (Phases 1, 2, 3, 4, 6, 7)

1. Complete Setup (scoring engine + unit tests).
2. Complete Foundational (models, pause tracker, interfaces).
3. Complete US1 (Next Question) + US2 (Reveal & Score) + US4 (End Session).
4. Integrate Router (Phase 7).
5. **Validate**: Full live quiz loop can be stepped through end-to-end.
6. Complete US3 (Pause/Resume controls) and verify timer adjustments.
