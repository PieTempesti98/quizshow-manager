# Phase 0: Research & Technical Decisions — Presenter Controls & Scoring Engine

**Feature**: Feature #7 — Presenter Controls (US-P02, US-P03, US-P04, US-P05, US-P06)  
**Branch**: `007-presenter-controls`  
**Date**: 2026-08-13  

---

## 1. Scoring Calculation & Speed Bonus Algorithm

### Decision
Implement the pure scoring calculation function `ScoreAnswer` in `backend/internal/session/scoring.go` following the exact mathematical specification from `docs/03-scoring-mechanics.md`:

```go
func ScoreAnswer(
    pointsPerAnswer int,
    isCorrect bool,
    speedBonusEnabled bool,
    timeRemainingMs int64,
    totalTimeMs int64,
) int {
    if !isCorrect {
        return 0
    }
    if !speedBonusEnabled || totalTimeMs <= 0 {
        return pointsPerAnswer
    }

    ratio := float64(timeRemainingMs) / float64(totalTimeMs)
    if ratio < 0 {
        ratio = 0
    }
    if ratio > 1 {
        ratio = 1
    }

    multiplier := 1.0 + ratio*0.5
    return int(math.Round(float64(pointsPerAnswer) * multiplier))
}
```

### Rationale
- Pure function with zero external side-effects or DB dependencies, making it directly unit-testable against all boundary cases (0ms, 50% timer, full timer, clamped bounds).
- Server-side authoritative: `timeRemainingMs` is computed strictly using server timestamps (`answer.AnsweredAt.Sub(sessionQuestion.AskedAt)`), never relying on client-supplied time.

### Alternatives Considered
- **SQL-based score calculation (Stored Procedures/Triggers)**: Rejected by Constitution Principle 5.4 ("No business logic in SQL — SQL is for data retrieval and persistence").
- **Separate `scoring` package**: Rejected to keep session domain cohesive. `scoring.go` inside `internal/session/` keeps model, service, and tests colocated while maintaining modularity.

---

## 2. Timer Pause & Resume State Management

### Decision
Implement an in-memory `PauseTracker` (thread-safe map guarded by `sync.RWMutex`) in the session service to track pause start timestamps per session (`session_id -> paused_at`).

When `POST /pause-timer` is invoked:
1. Verify session is `active` and has a currently active unrevealed question (`asked_at IS NOT NULL AND revealed_at IS NULL`).
2. Record `pausedAt = now.UTC()` in the pause tracker.
3. Calculate current `timeRemainingMs = (timePerQuestionS * 1000) - (now - asked_at)`.
4. Return `paused_at` and trigger `BroadcastTimerPaused(sessionID, pausedAt, timeRemainingMs)`.

When `POST /resume-timer` is invoked:
1. Retrieve `pausedAt` from the tracker (error 409 `TIMER_NOT_PAUSED` if not found).
2. Calculate pause duration: `pauseDuration = now.Sub(pausedAt)`.
3. Transactionally shift `asked_at` in the database: `UPDATE session_questions SET asked_at = asked_at + $1 WHERE id = $2`.
4. Delete session from pause tracker.
5. Trigger `BroadcastTimerResumed(sessionID, resumedAt, timeRemainingMs)`.

### Rationale
- **Zero Database Schema Changes**: Avoids adding new columns to `session_questions` or creating migrations for ephemeral pause state.
- **Accurate Stateless Answer Scoring**: Shifting `asked_at` forward by the pause duration makes all future elapsed time calculations (`answered_at - asked_at`) completely accurate without needing special case logic in answers or scoring.

### Alternatives Considered
- **Add `paused_at` and `total_pause_duration` columns to `session_questions` table**: Overcomplicates schema for ephemeral live session state during MVP.
- **Redis / External State Store**: Unnecessary external dependency during MVP (violates Principle III / YAGNI).

---

## 3. Sequential Question Progression & State Machine

### Decision
Enforce strict question sequencing in repository transactions:
- Current question state is determined by inspecting `session_questions`:
  - Active question: `asked_at IS NOT NULL AND revealed_at IS NULL`.
  - Revealed questions: `asked_at IS NOT NULL AND revealed_at IS NOT NULL`.
  - Unasked questions: `asked_at IS NULL`.
- `POST /next-question` rules:
  - Session status must be `active`.
  - If any question in the session has `asked_at IS NOT NULL AND revealed_at IS NULL`, return `ErrQuestionNotRevealed` (HTTP 409 `QUESTION_NOT_REVEALED`).
  - Next question is the unasked question with lowest `position`. If none remain, return `ErrNoMoreQuestions` (HTTP 409 `NO_MORE_QUESTIONS`).
  - Set `asked_at = now()`.
  - Response payload includes sanitized question info (`text`, options A-D) and **explicitly excludes** `correct_index`.

### Rationale
- Prevents skipping questions without revealing answers or scoring.
- Prevents cheating: `correct_index` is never transmitted across the network during `POST /next-question` or `question_started`.

---

## 4. Answer Reveal, Score Processing & Top-5 Leaderboard

### Decision
`POST /reveal` executes within a single database transaction (`FOR UPDATE` on `session_questions` and `answers`):
1. Lock active question and fetch `correct_index`, `points_per_answer`, `time_per_question_s`, `speed_bonus_enabled`.
2. Update `session_questions.revealed_at = now()`.
3. Fetch all `answers` for the question.
4. For each answer:
   - Determine `is_correct = (chosen_index != nil && *chosen_index == correct_index)`.
   - Calculate `answer_time_ms = clamp(answered_at - asked_at)`.
   - Calculate `points_awarded = ScoreAnswer(...)`.
   - Update `answers SET is_correct = $1, points_awarded = $2, answer_time_ms = $3 WHERE id = $4`.
   - Update `players SET total_score = total_score + $1 WHERE id = $2`.
5. Compute `answer_distribution` (counts and integer percentage for indices 0, 1, 2, 3).
6. Query top 5 players for intermediate leaderboard: `SELECT rank, nickname, total_score, avatar_color FROM players WHERE session_id = $1 ORDER BY total_score DESC, joined_at ASC, id ASC LIMIT 5`.

### Rationale
- Atomic update ensures no partial score increments occur if a crash occurs mid-calculation.
- Tie-breaking in leaderboard ordering (`ORDER BY total_score DESC, joined_at ASC, id ASC`) guarantees deterministic leaderboard output.

---

## 5. Broadcaster Decoupling

### Decision
Expand `SessionEventBroadcaster` interface in `internal/session/models.go` to support all presenter events with exact payload types matching `docs/04-api-design.md`:
- `BroadcastQuestionStarted`
- `BroadcastTimerPaused`
- `BroadcastTimerResumed`
- `BroadcastQuestionRevealed`
- `BroadcastSessionEnded`

Maintain `noopBroadcaster` implementation for this feature until WebSocket Hub is implemented in Feature #10.
