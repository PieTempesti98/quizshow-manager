# Feature Specification: Feature #7 — Presenter Controls & Scoring Engine

**Feature Branch**: `007-presenter-controls`  
**Created**: 2026-08-13  
**Status**: Draft  
**Input**: User description: "Feature #7 — Presenter controls (US-P02, US-P03, US-P04, US-P05, US-P06): POST /next-question, POST /pause-timer, POST /resume-timer, POST /reveal, POST /end with ScoreAnswer scoring mechanics and SessionEventBroadcaster decoupling"

## User Scenarios & Testing *(mandatory)*

### User Story 1 — Advance questions sequentially during active session (Priority: P1)

As a presenter, once the session is active, I want to advance to the next question so that all participants and the projection screen receive the new question simultaneously.

**Why this priority**: Without question progression, the live quiz cannot run. This is the core forward driver of an active quiz show.

**Independent Test**: Call `POST /api/v1/sessions/:id/next-question` on an active session. Verify HTTP 200 with `session_question_id`, 1-based `position`, `total`, question details (`text`, `option_a`, `option_b`, `option_c`, `option_d` — without `correct_index`), and `asked_at` timestamp. Verify in DB that `session_questions.asked_at` is set to the current timestamp.

**Acceptance Scenarios**:

1. **Given** a session in `active` status with drawn questions where no question has been asked yet, **When** admin calls `POST /next-question`, **Then** the first question (position 1) is activated, its `asked_at` timestamp is set to UTC now, and the response returns question payload (excluding `correct_index`) with HTTP 200.
2. **Given** an active session where question `K` has been revealed, **When** admin calls `POST /next-question`, **Then** question `K+1` is activated, its `asked_at` timestamp is set to UTC now, and the response returns question `K+1` details with HTTP 200.
3. **Given** an active session where the current question has NOT yet been revealed (`revealed_at IS NULL`), **When** admin calls `POST /next-question`, **Then** the server returns HTTP 409 conflict (`QUESTION_NOT_REVEALED`).
4. **Given** an active session where all drawn questions have already been asked and revealed, **When** admin calls `POST /next-question`, **Then** the server returns HTTP 409 conflict (`NO_MORE_QUESTIONS`).
5. **Given** a session not in `active` status (e.g. `draft`, `lobby`, `completed`, `cancelled`), **When** admin calls `POST /next-question`, **Then** the server returns HTTP 409 (`SESSION_NOT_ACTIVE`).
6. **Given** an unauthenticated request or a request with an invalid/expired token, **When** calling `POST /next-question`, **Then** the server returns HTTP 401.

---

### User Story 2 — Reveal correct answer and compute player scores (Priority: P1)

As a presenter, after the timer expires or when all players have answered, I want to trigger the answer reveal to display the correct answer, show the answer distribution across options, calculate points earned by each participant, and view the intermediate top-5 leaderboard.

**Why this priority**: Revealing answers and scoring is the central game loop payoff. It computes points, updates player rankings, and provides dramatic feedback to the presenter, projection screen, and players.

**Independent Test**: Call `POST /api/v1/sessions/:id/reveal` on an active session with an unrevealed asked question. Verify HTTP 200 returning `session_question_id`, `correct_index`, `answer_distribution` (counts and percentages for options 0..3), `top5` leaderboard, and `revealed_at`. Verify that submitted player answers have `is_correct`, `points_awarded`, and `answer_time_ms` calculated according to scoring rules, and `players.total_score` is atomically updated.

**Acceptance Scenarios**:

1. **Given** an active session with an asked question where `revealed_at IS NULL`, **When** admin calls `POST /reveal`, **Then** the server sets `session_questions.revealed_at = now()`, evaluates all submitted answers for that question, calculates points using the authoritative `ScoreAnswer()` formula, updates `answers` rows and `players.total_score`, and returns `{ session_question_id, correct_index, answer_distribution, top5, revealed_at }` with HTTP 200.
2. **Given** a session with speed bonus disabled (`speed_bonus_enabled = false`), **When** an answer is correct, **Then** `points_awarded` equals `session.points_per_answer` (default 100) regardless of answer submission time.
3. **Given** a session with speed bonus enabled (`speed_bonus_enabled = true`), **When** an answer is correct, **Then** `points_awarded` is calculated as `ROUND(points_per_answer * (1.0 + (time_remaining_ms / total_time_ms) * 0.5))` where `time_remaining_ms = total_time_ms - (answered_at - asked_at)` clamped between 0 and `total_time_ms`.
4. **Given** an incorrect answer or a missing answer, **When** scored, **Then** `points_awarded` is 0 and `is_correct` is false.
5. **Given** multiple players answering the question, **When** reveal completes, **Then** `answer_distribution` contains exactly 4 entries (indices 0..3) with accurate counts and integer percentage shares summing to 100% (or 0% if no answers), and `top5` contains up to 5 highest-ranking players sorted by `total_score` DESC.
6. **Given** an active session where the current question is already revealed, **When** admin calls `POST /reveal`, **Then** the server returns HTTP 409 (`QUESTION_ALREADY_REVEALED`).
7. **Given** an active session where no question has been asked yet, **When** admin calls `POST /reveal`, **Then** the server returns HTTP 409 (`NO_ACTIVE_QUESTION`).

---

### User Story 3 — Pause and resume question timer (Priority: P2)

As a presenter, I want to pause the question countdown during unexpected disruptions and resume it later, with the server adjusting the time calculation so players still have their full remaining time without speed-bonus penalty.

**Why this priority**: Real-world live events frequently experience interruptions (noise, presenter remarks, technical checks). Pausing and resuming maintains fairness and live flow control.

**Independent Test**: Call `POST /api/v1/sessions/:id/pause-timer` on an active unrevealed question, verify HTTP 200 with `paused_at`. Then call `POST /api/v1/sessions/:id/resume-timer`, verify HTTP 200 with `resumed_at`. Check that the elapsed pause duration is added to `session_questions.asked_at` so future answer speed calculations remain accurate.

**Acceptance Scenarios**:

1. **Given** an active session with an unrevealed asked question that is not paused, **When** admin calls `POST /pause-timer`, **Then** server marks the question/timer as paused at current UTC time and returns `{ ok: true, paused_at }` with HTTP 200.
2. **Given** a paused question timer, **When** admin calls `POST /resume-timer`, **Then** server calculates the pause duration `delta = resumed_at - paused_at`, shifts `session_questions.asked_at` forward by `delta`, clears the paused state, and returns `{ ok: true, resumed_at }` with HTTP 200.
3. **Given** a question that is already paused, **When** admin calls `POST /pause-timer` again, **Then** server returns HTTP 409 (`TIMER_ALREADY_PAUSED`).
4. **Given** a question that is not currently paused, **When** admin calls `POST /resume-timer`, **Then** server returns HTTP 409 (`TIMER_NOT_PAUSED`).
5. **Given** a session with no active unrevealed question, **When** admin calls `POST /pause-timer` or `POST /resume-timer`, **Then** server returns HTTP 409.

---

### User Story 4 — End session and finalize results (Priority: P1)

As a presenter, I want to conclude the session — either after completing all questions or early due to time constraints — so that the session transitions to `completed`, PIN access expires, and final leaderboards are prepared.

**Why this priority**: Concluding the session is essential to release resources, finalize match status, invalidate the PIN for new joins, and transition all screens to post-game state.

**Independent Test**: Call `POST /api/v1/sessions/:id/end` (with optional JSON body `{"reason": "early"}`). Verify HTTP 200 returning `session_id`, `status: "completed"`, and `ended_at`. Verify that the session record in DB has `status = 'completed'` and `ended_at` populated.

**Acceptance Scenarios**:

1. **Given** an active session, **When** admin calls `POST /end`, **Then** session status transitions to `completed`, `ended_at` is set to UTC now, and the response returns `{ session_id, status: "completed", ended_at }` with HTTP 200.
2. **Given** a session in `lobby` or `draft` status, **When** admin calls `POST /end` with reason or without, **Then** status transitions to `cancelled` or `completed` as appropriate (or HTTP 409 if status is already terminal `completed`/`cancelled`).
3. **Given** a session already in `completed` or `cancelled` status, **When** admin calls `POST /end`, **Then** server returns HTTP 409 (`SESSION_ALREADY_ENDED`).
4. **Given** an active session ended via `POST /end`, **When** any player or presenter subsequently attempts question or answer operations, **Then** the server rejects them with HTTP 409 (`SESSION_NOT_ACTIVE`).

---

### User Story 5 — Live response tracking & Broadcaster event hooks (Priority: P2)

As a presenter and system architect, I want the backend presenter operations to emit typed lifecycle event hooks (`question_started`, `timer_paused`, `timer_resumed`, `question_revealed`, `session_ended`) via an abstract broadcaster interface so that real-time WebSocket distribution can be plugged in seamlessly in subsequent features without modifying core domain logic.

**Why this priority**: Clean event abstraction keeps the Go domain architecture decoupled and testable while ensuring all event payloads match the WebSocket contract defined in `docs/04-api-design.md`.

**Independent Test**: Execute each presenter endpoint (`/next-question`, `/pause-timer`, `/resume-timer`, `/reveal`, `/end`) and verify through unit tests and mock broadcaster invocations that each method dispatches the expected typed event with exact parameters.

**Acceptance Scenarios**:

1. **Given** a successful `POST /next-question` execution, **When** the transaction commits, **Then** `broadcaster.BroadcastQuestionStarted(sessionID, sessionQuestionID, position, total, questionData, timeLimitS, askedAt)` is called.
2. **Given** a successful `POST /pause-timer` execution, **When** the timer is paused, **Then** `broadcaster.BroadcastTimerPaused(sessionID, pausedAt, timeRemainingMs)` is called.
3. **Given** a successful `POST /resume-timer` execution, **When** the timer is resumed, **Then** `broadcaster.BroadcastTimerResumed(sessionID, resumedAt, timeRemainingMs)` is called.
4. **Given** a successful `POST /reveal` execution, **When** answers and scores are committed, **Then** `broadcaster.BroadcastQuestionRevealed(sessionID, sessionQuestionID, correctIndex, distribution, top5)` is called.
5. **Given** a successful `POST /end` execution, **When** the session completes, **Then** `broadcaster.BroadcastSessionEnded(sessionID, reason)` is called.

---

### Edge Cases

- **Concurrent / Fast Double Clicks**: What if the presenter clicks "Next Question" or "Reveal" twice simultaneously? Database transactions and optimistic/state guards (`WHERE status = 'active' AND revealed_at IS NULL`) ensure only one request succeeds and the second returns HTTP 409.
- **Zero Answers Submitted**: What if a question timer expires and zero players submitted answers before `POST /reveal` is called? The reveal succeeds gracefully: `answer_distribution` counts are all 0, percentages 0%, `top5` reflects existing cumulative scores with 0 delta.
- **Sub-Millisecond / Clamped Speeds**: What if network latency produces an answer timestamp with `timeRemainingMs < 0` or `timeRemainingMs > totalTimeMs`? The scoring service strictly clamps `timeRemainingMs` between `0` and `totalTimeMs` before calculating multiplier.
- **Pause Duration Exceeds Time Limit**: What happens if the timer was paused with 5 seconds remaining, and resumed 20 minutes later? Shifting `asked_at` by the pause duration preserves the exact remaining 5 seconds regardless of how long the pause lasted.
- **Tie Breakers in Leaderboard**: If multiple players have the same `total_score`, they are ordered deterministically by `total_score DESC, joined_at ASC, id ASC`.

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: System MUST provide endpoint `POST /api/v1/sessions/:id/next-question` protected by admin authentication.
- **FR-002**: Calling `POST /next-question` on an active session MUST advance to the next unasked question in 1-based order, set its `session_questions.asked_at` timestamp to UTC now, and return the question details without exposing `correct_index`.
- **FR-003**: System MUST prevent advancing to the next question if the currently active question has not yet been revealed (`revealed_at IS NULL`), returning HTTP 409 with code `QUESTION_NOT_REVEALED`.
- **FR-004**: System MUST return HTTP 409 with code `NO_MORE_QUESTIONS` if `POST /next-question` is called after all drawn questions for the session have already been asked and revealed.
- **FR-005**: System MUST provide endpoint `POST /api/v1/sessions/:id/reveal` protected by admin authentication.
- **FR-006**: Calling `POST /reveal` MUST set `session_questions.revealed_at` to UTC now for the current asked question.
- **FR-007**: When `POST /reveal` is executed, system MUST evaluate all answers for that question against the question's `correct_index` and calculate points using the authoritative `ScoreAnswer()` logic.
- **FR-008**: The `ScoreAnswer()` service function MUST apply the speed bonus formula `ROUND(points_per_answer * (1.0 + (time_remaining_ms / total_time_ms) * 0.5))` when `sessions.speed_bonus_enabled` is true, and flat `points_per_answer` when false (0 for incorrect/unanswered).
- **FR-009**: The scoring logic MUST compute `time_remaining_ms` strictly server-side based on `session_questions.asked_at` and `answers.answered_at`, clamped to `[0, total_time_ms]`.
- **FR-010**: Calling `POST /reveal` MUST atomically update `answers.is_correct`, `answers.points_awarded`, `answers.answer_time_ms`, and increment each player's `players.total_score` in a single transaction.
- **FR-011**: Calling `POST /reveal` MUST return the question's `correct_index`, `answer_distribution` (for options 0..3), `top5` players leaderboard, and `revealed_at` timestamp.
- **FR-012**: System MUST provide endpoint `POST /api/v1/sessions/:id/pause-timer` protected by admin authentication.
- **FR-013**: System MUST provide endpoint `POST /api/v1/sessions/:id/resume-timer` protected by admin authentication, which adjusts `asked_at` by adding the elapsed pause duration (`resumed_at - paused_at`).
- **FR-014**: System MUST return HTTP 409 if `POST /pause-timer` is called when timer is already paused, or if `POST /resume-timer` is called when timer is not paused.
- **FR-015**: System MUST provide endpoint `POST /api/v1/sessions/:id/end` protected by admin authentication, accepting an optional `reason` parameter (defaults to `"completed"`, or `"early"`).
- **FR-016**: Calling `POST /end` MUST update `sessions.status = 'completed'` (or `'cancelled'` if ending a non-active session) and set `sessions.ended_at` to UTC now.
- **FR-017**: All presenter endpoints MUST return HTTP 409 `SESSION_NOT_ACTIVE` if invoked on a session that is not in `active` status (except `/end` on draft/lobby).
- **FR-018**: System MUST expand the `SessionEventBroadcaster` interface with typed methods for `BroadcastQuestionStarted`, `BroadcastTimerPaused`, `BroadcastTimerResumed`, `BroadcastQuestionRevealed`, and `BroadcastSessionEnded`.
- **FR-019**: A default no-op broadcaster implementation MUST be provided to decouple REST execution from the WebSocket hub until feature #10.
- **FR-020**: All JSON responses MUST strictly adhere to the standard API response envelopes: `{ "data": ... }` for HTTP 2xx and `{ "error": { "code": "...", "message": "..." } }` for errors.

### Key Entities

- **Session**: State machine entity (`active` status during presenter operations; transitions to `completed` upon `/end`).
- **SessionQuestion**: Association entity between `sessions` and `questions`. Carries `position`, `asked_at`, and `revealed_at` timestamps determining question lifecycle.
- **Answer**: Player response record linking `players` and `session_questions`. Updated during `/reveal` with computed `is_correct`, `points_awarded`, and `answer_time_ms`.
- **Player**: Participant entity with cumulative `total_score` and `avatar_color`.

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: Advancing to the next question (`POST /next-question`) completes within 100ms.
- **SC-002**: Revealing answers and scoring up to 100 player submissions (`POST /reveal`) executes atomically in under 250ms.
- **SC-003**: Pure unit tests for `ScoreAnswer()` cover 100% of mathematical boundary cases (0ms, half-time, full-time, clamped overflow, speed bonus on/off, wrong answer).
- **SC-004**: Timer pause and resume operations preserve accurate remaining question time with zero millisecond drift for subsequent score calculations.
- **SC-005**: 100% of responses adhere to the standard error/data envelopes, returning appropriate HTTP status codes (200, 401, 404, 409, 422).

## Assumptions

- Admin authentication (JWT Bearer token) is required for all presenter control endpoints.
- Scoring is performed synchronously during the `POST /reveal` request to guarantee transactional consistency before broadcasting results to clients.
- Pause state can be tracked via an in-memory session lock/state or session question timestamps; when resuming, `asked_at` in the database is adjusted by adding the pause duration so that `time_remaining_ms` math remains simple and stateless for answers.
- `docs/06-ui-flows.md` is not required for backend REST API endpoints and domain services; frontend implementation will follow once UI specs are published.
- The WebSocket hub is not yet active in this feature; broadcaster method invocations are executed via no-op stub.
