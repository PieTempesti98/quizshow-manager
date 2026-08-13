# Implementation Plan: Feature #7 — Presenter Controls & Scoring Engine

**Branch**: `007-presenter-controls` | **Date**: 2026-08-13 | **Spec**: [spec.md](spec.md)  
**Input**: Feature specification from `/specs/007-presenter-controls/spec.md`

## Summary

Implement 5 new REST endpoints for presenter control during an active quiz session, alongside the authoritative pure scoring calculation engine:

1. `POST /api/v1/sessions/:id/next-question` — sequential question activation (sets `session_questions.asked_at`, returns sanitized question without `correct_index`).
2. `POST /api/v1/sessions/:id/pause-timer` — pauses active question countdown with in-memory state tracking.
3. `POST /api/v1/sessions/:id/resume-timer` — resumes countdown and shifts `asked_at` in DB by the pause duration to eliminate time calculation drift.
4. `POST /api/v1/sessions/:id/reveal` — reveals correct answer, calculates scores using `ScoreAnswer()` (with speed bonus logic), updates `answers` and `players.total_score`, computes answer distribution and top 5 leaderboard.
5. `POST /api/v1/sessions/:id/end` — concludes the session, transitioning status to `completed` and setting `ended_at`.

All work extends `backend/internal/session/` (plus `scoring.go` and unit tests in `scoring_test.go`). No database migrations are required. The `SessionEventBroadcaster` interface is expanded to define all presenter event hooks with typed payloads.

---

## Technical Context

**Language/Version**: Go 1.25  
**Primary Dependencies**: `gofiber/fiber` v2, `jackc/pgx` v5, `google/uuid` v1, `github.com/skip2/go-qrcode`  
**Storage**: PostgreSQL — tables `sessions`, `session_questions`, `answers`, `players` (existing from migration 001)  
**Testing**: Go `testing` package with table-driven tests for `ScoreAnswer()` and pause/resume logic (`internal/session/scoring_test.go`)  
**Target Platform**: Linux server (Docker Compose)  
**Project Type**: Web service (REST API)  
**Performance Goals**: `next-question` < 100ms, `reveal` (up to 100 answers) < 250ms, `pause`/`resume` < 50ms  
**Constraints**: Pure Go SQL queries with `pgx` (no ORM), standard API error envelopes, WebSocket hub decoupled via interface stub, no frontend dependencies.  
**Scale/Scope**: MVP — single presenter per session, up to 50 questions and up to 100 concurrent players per session.

---

## Constitution Check

### Principle 1.1 & 1.2 — Spec-First & MVP Discipline ✅
- Feature spec exists at [specs/007-presenter-controls/spec.md](spec.md).
- Implements only MVP user stories US-P02, US-P03, US-P04, US-P05, US-P06. R2+ features (e.g. difficulty multipliers on points) are excluded.

### Principle 2.1 & 2.2 — Package Structure & No Global State ✅
- All logic lives in `internal/session/`.
- Repositories, services, and pause trackers are instantiated and passed via constructor injection.

### Principle 2.3 & 2.4 — Database Access & Migrations ✅
- Uses `pgx` directly with explicit SQL queries.
- No new database schema changes needed (all tables exist from `001_initial_schema.up.sql`).

### Principle 4.3 & 5.2–5.4 — API Envelope, UUIDs, UTC & No Business Logic in SQL ✅
- Standard success envelope `{ "data": ... }` and error envelope `{ "error": ... }`.
- UUID v4 for all entity IDs; timestamps in UTC (`TIMESTAMPTZ`).
- Scoring business logic implemented purely in Go (`ScoreAnswer`).

### Principle 6.1 — Unit Tests for Business Logic ✅
- Table-driven unit tests for scoring engine covering 100% of mathematical boundary cases in `scoring_test.go`.

**Gate result**: PASS — proceed to implementation.

---

## Project Structure

### Documentation (this feature)

```text
specs/007-presenter-controls/
├── plan.md              ← this file
├── spec.md
├── research.md          ← Phase 0 output
├── data-model.md        ← Phase 1 output
├── quickstart.md        ← Phase 1 output
├── contracts/
│   └── endpoints.md     ← Phase 1 output
└── checklists/
    └── requirements.md
```

### Source Code

```text
backend/
├── cmd/server/main.go                          ← Register 5 new presenter routes on protected router
└── internal/
    └── session/
        ├── scoring.go                          ← NEW: Pure ScoreAnswer function & calculation logic
        ├── scoring_test.go                     ← NEW: Table-driven unit tests for scoring
        ├── pause_tracker.go                    ← NEW: In-memory thread-safe pause tracker
        ├── models.go                           ← Result structs, presenter errors, and expanded Broadcaster interface
        ├── repository.go                       ← NextQuestion, ShiftQuestionAskedAt, Reveal, EndSession repo methods
        ├── service.go                          ← Presenter business service orchestration
        └── handler.go                          ← 5 HTTP presenter handlers with parameter validation
```

---

## Complexity Tracking

No constitution violations. No complexity justification required.

---

## Implementation Steps

### Step 1 — Pure Scoring Engine (`internal/session/scoring.go` & `scoring_test.go`)
- Implement pure function `ScoreAnswer(pointsPerAnswer int, isCorrect bool, speedBonusEnabled bool, timeRemainingMs int64, totalTimeMs int64) int`.
- Write comprehensive table-driven tests in `scoring_test.go` covering:
  - Correct answer with speed bonus disabled (flat points).
  - Correct answer with speed bonus enabled (immediate submission = 1.5x, half-time = 1.25x, last millisecond = 1.0x).
  - Incorrect answer (0 points).
  - Unanswered / timeout (0 points).
  - Boundary clamps (negative time remaining, time remaining exceeding total question time).

### Step 2 — In-Memory Pause Tracker (`internal/session/pause_tracker.go`)
- Create `PauseTracker` struct with `sync.RWMutex` and `map[uuid.UUID]time.Time`.
- Methods:
  - `Pause(sessionID uuid.UUID, now time.Time) error` (returns `ErrTimerAlreadyPaused` if active).
  - `Resume(sessionID uuid.UUID, now time.Time) (time.Duration, error)` (returns `ErrTimerNotPaused` if not paused).
  - `Clear(sessionID uuid.UUID)`.
  - `IsPaused(sessionID uuid.UUID) bool`.

### Step 3 — Extend Models & Sentinel Errors (`internal/session/models.go`)
- Sentinel Errors:
  - `ErrSessionNotActive` (`"session is not in active status"`)
  - `ErrQuestionNotRevealed` (`"current question must be revealed before advancing"`)
  - `ErrNoMoreQuestions` (`"all questions have been asked"`)
  - `ErrNoActiveQuestion` (`"no question currently in progress"`)
  - `ErrQuestionAlreadyRevealed` (`"question has already been revealed"`)
  - `ErrTimerAlreadyPaused` (`"timer is already paused"`)
  - `ErrTimerNotPaused` (`"timer is not paused"`)
  - `ErrSessionAlreadyEnded` (`"session has already ended"`)
- Result Structs:
  - `NextQuestionResult`, `PauseTimerResult`, `ResumeTimerResult`, `RevealResult`, `EndSessionResult`, `AnswerDistributionItem`, `LeaderboardEntry`.
- Expand `SessionEventBroadcaster` interface:
  - `BroadcastQuestionStarted(sessionID string, sessionQuestionID string, position int, total int, question QuestionSummary, timeLimitS int, askedAt time.Time)`
  - `BroadcastTimerPaused(sessionID string, pausedAt time.Time, timeRemainingMs int64)`
  - `BroadcastTimerResumed(sessionID string, resumedAt time.Time, timeRemainingMs int64)`
  - `BroadcastQuestionRevealed(sessionID string, sessionQuestionID string, correctIndex int, distribution []AnswerDistributionItem, top5 []LeaderboardEntry)`
  - `BroadcastSessionEnded(sessionID string, reason string)`
- Update `noopBroadcaster` to implement all methods.

### Step 4 — Implement Repository Methods (`internal/session/repository.go`)
- `NextQuestion(ctx context.Context, sessionID uuid.UUID) (NextQuestionData, error)`:
  - Verifies session is `active`.
  - Checks if an active unrevealed question exists -> `ErrQuestionNotRevealed`.
  - Finds next unasked question (lowest position where `asked_at IS NULL`). If none -> `ErrNoMoreQuestions`.
  - Sets `session_questions.asked_at = now()`.
  - Returns question data (without `correct_index`), position, and total question count.
- `GetActiveQuestion(ctx context.Context, sessionID uuid.UUID) (ActiveQuestionData, error)`:
  - Fetches current question where `asked_at IS NOT NULL AND revealed_at IS NULL`.
- `ShiftQuestionAskedAt(ctx context.Context, sessionQuestionID uuid.UUID, delta time.Duration) error`:
  - `UPDATE session_questions SET asked_at = asked_at + $1 WHERE id = $2`.
- `RevealQuestion(ctx context.Context, sessionID uuid.UUID) (RevealData, error)` (in single `tx`):
  - Fetches active question and question's `correct_index` with session settings (`points_per_answer`, `time_per_question_s`, `speed_bonus_enabled`).
  - Sets `session_questions.revealed_at = now()`.
  - Fetches submitted answers `FOR UPDATE`.
  - Computes `is_correct`, `answer_time_ms`, `points_awarded` using `ScoreAnswer()`.
  - Updates `answers` rows and increments `players.total_score`.
  - Computes `answer_distribution` (indices 0..3 with count and percentage).
  - Fetches top 5 players sorted by `total_score DESC, joined_at ASC, id ASC`.
- `EndSession(ctx context.Context, sessionID uuid.UUID, reason string) (Session, error)`:
  - Verifies status is not `completed` or `cancelled` -> `ErrSessionAlreadyEnded`.
  - Updates `sessions SET status = 'completed', ended_at = now(), updated_at = now() WHERE id = $1`.

### Step 5 — Implement Service Methods (`internal/session/service.go`)
- Add methods to `Service` interface:
  - `NextQuestion(ctx context.Context, sessionID uuid.UUID) (NextQuestionResult, error)`
  - `PauseTimer(ctx context.Context, sessionID uuid.UUID) (PauseTimerResult, error)`
  - `ResumeTimer(ctx context.Context, sessionID uuid.UUID) (ResumeTimerResult, error)`
  - `Reveal(ctx context.Context, sessionID uuid.UUID) (RevealResult, error)`
  - `End(ctx context.Context, sessionID uuid.UUID, reason string) (EndSessionResult, error)`
- Integrate `pauseTracker` and `broadcaster` in `service` struct.

### Step 6 — Implement Handlers & Route Registration (`internal/session/handler.go` & `cmd/server/main.go`)
- Add HTTP handlers:
  - `NextQuestion`: parses UUID, calls service, maps domain errors (404, 409 `SESSION_NOT_ACTIVE`, 409 `QUESTION_NOT_REVEALED`, 409 `NO_MORE_QUESTIONS`), returns 200.
  - `PauseTimer`: parses UUID, calls service, maps domain errors (404, 409 `TIMER_ALREADY_PAUSED`, 409 `NO_ACTIVE_QUESTION`), returns 200.
  - `ResumeTimer`: parses UUID, calls service, maps domain errors (404, 409 `TIMER_NOT_PAUSED`), returns 200.
  - `Reveal`: parses UUID, calls service, maps domain errors (404, 409 `QUESTION_ALREADY_REVEALED`, 409 `NO_ACTIVE_QUESTION`), returns 200.
  - `End`: parses UUID, optional body `reason`, maps domain errors (404, 409 `SESSION_ALREADY_ENDED`), returns 200.
- Register 5 routes under Fiber protected group (`protected.Post("/sessions/:id/next-question", ...)`).

---

## Verification & Acceptance Testing

1. Run automated unit tests: `go test -v ./internal/session/...`
2. Perform smoke test sequence as outlined in [quickstart.md](quickstart.md):
   - Admin Login -> Create Session -> Open Lobby -> Launch.
   - Advance to first question -> verify 200 with sanitized question.
   - Try to advance again before reveal -> verify 409 `QUESTION_NOT_REVEALED`.
   - Pause timer -> verify 200; pause again -> verify 409 `TIMER_ALREADY_PAUSED`.
   - Resume timer -> verify 200.
   - Reveal question -> verify 200 with correct index, answer distribution, and top 5.
   - End session -> verify 200 with status `completed`.
