# QuizShow — Scoring Mechanics

This document defines all scoring rules, bonus logic, and question selection algorithms.
It is the authoritative reference for the Go scoring service.

---

## 1. Base scoring

Every session is configured with a `points_per_answer` value (default: 100).

| Outcome | Points |
|---|---|
| Correct answer | `points_per_answer` (× multiplier if speed bonus active) |
| Wrong answer | 0 |
| No answer (timeout) | 0 |

Points are calculated server-side at reveal time and stored in `answers.points_awarded`.
The player's `total_score` is updated atomically after each reveal.

---

## 2. Speed bonus (MVP)

Enabled per-session via `sessions.speed_bonus_enabled`.

### Formula

```
multiplier = 1.0 + (time_remaining_ms / total_time_ms) × 0.5
points_awarded = ROUND(points_per_answer × multiplier)
```

- `time_remaining_ms`: milliseconds left on the timer when the player submitted
- `total_time_ms`: `time_per_question_s × 1000`
- Result is rounded to the nearest integer (`math.Round` in Go)
- Only applied to **correct** answers — wrong answers always score 0 regardless

### Boundary values

| When player answers | `time_remaining_ms / total_time_ms` | Multiplier | Points (base 100) |
|---|---|---|---|
| Immediately (1ms in) | ≈ 1.0 | ≈ 1.5× | 150 |
| At exactly half timer | 0.5 | 1.25× | 125 |
| Last millisecond | ≈ 0.0 | ≈ 1.0× | 100 |

### Computing `time_remaining_ms` server-side

The server records `session_questions.asked_at` when the question is broadcast.
When an answer arrives, `time_remaining_ms` is calculated as:

```go
totalMs := int64(session.TimePerQuestionS) * 1000
elapsedMs := answer.AnsweredAt.Sub(sessionQuestion.AskedAt).Milliseconds()
timeRemainingMs := totalMs - elapsedMs

// Clamp to valid range
if timeRemainingMs < 0 { timeRemainingMs = 0 }
if timeRemainingMs > totalMs { timeRemainingMs = totalMs }
```

> **Important:** never trust the client-reported time. Always compute server-side
> from `asked_at` and the answer receipt timestamp.

### Why linear over other approaches

- **vs. binary (first 50% = 1.5×):** no cliff edge — answering at 50.1% vs 49.9% of the timer produces almost identical scores, which is fair
- **vs. rank-based (top 3 fastest = 1.5×):** independent of other players — your bonus is entirely determined by your own speed, not how fast others happened to be; also scales correctly to any session size
- **vs. tiered:** smoother experience, same implementation complexity

---

## 3. Difficulty (MVP)

In MVP, difficulty is a **filter and label only**. It has no effect on points.

| Difficulty | Points impact |
|---|---|
| `easy` | None — same as `medium` and `hard` |
| `medium` | None |
| `hard` | None |

Difficulty is used for:
- Admin UI filtering when browsing the question bank
- Post-session statistics (breakdown by difficulty)
- Manual session curation ("I want harder questions for this group")

---

## 4. Planned mechanics (R2+)

The following are **not implemented in MVP** but are captured here to inform schema and architectural decisions. No code should implement these until the relevant release.

### 4a. Difficulty multiplier on points

Apply a per-question multiplier based on difficulty:

```
easy   → 1.0× points_per_answer
medium → 1.5× points_per_answer
hard   → 2.0× points_per_answer
```

Combined with speed bonus (if both active):
```
points = ROUND(points_per_answer × difficulty_multiplier × speed_multiplier)
```

Implementation note: `answers.points_awarded` already stores the final computed value,
so no schema change is needed — only the scoring function in Go changes.

The admin UI will need to display the point value of each question before the session starts
so players understand what is at stake.

### 4b. Balanced question pool selection

When launching a session, instead of pure random draw from the selected categories,
the system can generate a pool that maintains a target difficulty balance.

**Difficulty coefficient:**

| Difficulty | Coefficient |
|---|---|
| `easy` | 1 |
| `medium` | 2 |
| `hard` | 3 |

**Target score:** `question_count × 2` (equivalent to an all-medium pool)

**Algorithm (greedy approximation):**

```
1. Shuffle all available questions from the selected categories
2. Greedily pick questions, tracking running coefficient sum
3. Prefer questions that bring the sum closest to the target
4. Stop when question_count is reached
5. If exact target is impossible (insufficient questions of a difficulty),
   accept the closest achievable sum and surface a warning to the admin
```

**Example — 20 questions, target coefficient sum = 40:**

| Mix | Coefficient sum | Delta from target |
|---|---|---|
| 20 medium | 40 | 0 (perfect) |
| 10 easy + 10 hard | 40 | 0 (perfect) |
| 5 easy + 10 medium + 5 hard | 40 | 0 (perfect) |
| 20 easy | 20 | −20 (too easy) |
| 20 hard | 60 | +20 (too hard) |

The admin UI will show the resulting difficulty distribution before confirming the launch
(e.g. "7 easy / 9 medium / 4 hard — coefficient balance: 39/40").

**Schema readiness:** no changes needed. `questions.difficulty` and
`session_questions.question_id` already carry all required information.

---

## 5. Scoring summary table (MVP)

| Scenario | Speed bonus off | Speed bonus on |
|---|---|---|
| Correct, answered immediately | 100 | ~150 |
| Correct, answered at half timer | 100 | 125 |
| Correct, answered last second | 100 | 100 |
| Wrong answer | 0 | 0 |
| No answer | 0 | 0 |

All values assume `points_per_answer = 100`. Scale linearly for other base values.

---

## 6. Go implementation reference

```go
// ScoreAnswer computes points for a single answer.
// Called at reveal time for each player's answer.
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
    // Clamp ratio to [0, 1]
    if ratio < 0 { ratio = 0 }
    if ratio > 1 { ratio = 1 }

    multiplier := 1.0 + ratio*0.5
    return int(math.Round(float64(pointsPerAnswer) * multiplier))
}
```

This function is pure (no side effects, no DB access) and trivially unit-testable.
