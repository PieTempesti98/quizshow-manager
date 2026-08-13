# Phase 1: Data Model & State Transitions — Presenter Controls & Scoring Engine

**Feature**: Feature #7 — Presenter Controls  
**Branch**: `007-presenter-controls`  
**Date**: 2026-08-13  

---

## 1. Entities & Schema Reference

No database migration is required. All tables exist from migration `001_initial_schema.up.sql`.

### 1.1 `sessions`
Represents the quiz show session instance.

| Column | Type | Nullable | Description |
|---|---|---|---|
| `id` | UUID | NO | Primary key |
| `name` | TEXT | NO | Display name of the session |
| `pin` | CHAR(6) | NO | 6-digit PIN |
| `status` | session_status | NO | Enum: `draft`, `lobby`, `active`, `completed`, `cancelled` |
| `question_count` | SMALLINT | NO | Total questions configured (1..50) |
| `time_per_question_s` | SMALLINT | NO | Seconds per question (10, 20, 30, 60) |
| `points_per_answer` | INT | NO | Base points for a correct answer (default: 100) |
| `speed_bonus_enabled` | BOOLEAN | NO | Enables multiplier up to 1.5x for faster answers |
| `started_at` | TIMESTAMPTZ | YES | Set on session launch |
| `ended_at` | TIMESTAMPTZ | YES | **Updated on `POST /end`** |
| `created_by` | UUID | YES | Foreign key to `admins(id)` |
| `created_at` | TIMESTAMPTZ | NO | Timestamp created |
| `updated_at` | TIMESTAMPTZ | NO | Timestamp updated |
| `deleted_at` | TIMESTAMPTZ | YES | Soft-delete timestamp |

---

### 1.2 `session_questions`
Represents the ordered questions drawn for an active session.

| Column | Type | Nullable | Description |
|---|---|---|---|
| `id` | UUID | NO | Primary key |
| `session_id` | UUID | NO | Foreign key to `sessions(id)` ON DELETE CASCADE |
| `question_id` | UUID | NO | Foreign key to `questions(id)` |
| `position` | SMALLINT | NO | 1-based order index (1..N) |
| `asked_at` | TIMESTAMPTZ | YES | **Set on `POST /next-question`**; **adjusted on `POST /resume-timer`** |
| `revealed_at` | TIMESTAMPTZ | YES | **Set on `POST /reveal`** |

**Constraints**: `UNIQUE (session_id, position)`

---

### 1.3 `answers`
Represents a player's answer submission for a given question.

| Column | Type | Nullable | Description |
|---|---|---|---|
| `id` | UUID | NO | Primary key |
| `player_id` | UUID | NO | Foreign key to `players(id)` ON DELETE CASCADE |
| `session_question_id` | UUID | NO | Foreign key to `session_questions(id)` ON DELETE CASCADE |
| `chosen_index` | SMALLINT | YES | 0..3 index chosen by player (NULL if timeout) |
| `is_correct` | BOOLEAN | NO | **Evaluated on `POST /reveal`** (default: false) |
| `points_awarded` | INT | NO | **Computed on `POST /reveal` via `ScoreAnswer()`** (default: 0) |
| `answer_time_ms` | INT | YES | **Computed on `POST /reveal` as `answered_at - asked_at`** |
| `answered_at` | TIMESTAMPTZ | NO | Submission timestamp recorded by server |

**Constraints**: `UNIQUE (player_id, session_question_id)`

---

### 1.4 `players`
Represents a participant in the session.

| Column | Type | Nullable | Description |
|---|---|---|---|
| `id` | UUID | NO | Primary key |
| `session_id` | UUID | NO | Foreign key to `sessions(id)` |
| `nickname` | TEXT | NO | Player nickname (unique per session) |
| `avatar_color` | TEXT | NO | Hex color code |
| `total_score` | INT | NO | **Incremented atomically on `POST /reveal`** (default: 0) |
| `joined_at` | TIMESTAMPTZ | NO | Timestamp joined |
| `disconnected_at` | TIMESTAMPTZ | YES | Disconnection timestamp |

---

## 2. In-Memory Pause State

```go
type PauseTracker struct {
    mu     sync.RWMutex
    pauses map[uuid.UUID]time.Time // key: session_id, val: paused_at
}
```

- **Set**: on `POST /pause-timer` (if not already paused).
- **Check/Get/Delete**: on `POST /resume-timer` (calculates `now - paused_at`).
- **Cleaned**: on `POST /reveal` or `POST /end`.

---

## 3. Question State Transitions

```
[Unasked Question] ──(POST /next-question)──> [Active / In Progress]
                                                    │
                                   ┌────────────────┴────────────────┐
                         (POST /pause-timer)                (POST /reveal)
                                   │                                 │
                                   ▼                                 ▼
                           [Paused Question]                   [Revealed]
                                   │                                 │
                        (POST /resume-timer)                         │
                                   │                                 │
                                   └──────────────> [Active] ────────┘
```

| Question State | `asked_at` | `revealed_at` | `PauseTracker` | Allowed Actions |
|---|---|---|---|---|
| **Unasked** | `NULL` | `NULL` | Not Present | `POST /next-question` (if prior revealed) |
| **In Progress** | Set | `NULL` | Not Present | `POST /pause-timer`, `POST /reveal` |
| **Paused** | Set | `NULL` | Present (`paused_at`) | `POST /resume-timer`, `POST /reveal` |
| **Revealed** | Set | Set | Not Present | `POST /next-question`, `POST /end` |
