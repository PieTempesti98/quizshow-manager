# Data Model: Feature #8 — Player Join & Answer Submission

## Existing Database Tables Utilized

No new database schema migrations are required for Feature #8. The tables `players`, `answers`, `sessions`, and `session_questions` are already present in PostgreSQL schema `001_initial_schema.up.sql`.

### `players` Table Reference

| Column | Type | Constraints | Description |
|---|---|---|---|
| `id` | `UUID` | `PRIMARY KEY DEFAULT gen_random_uuid()` | Unique identifier for ephemeral player |
| `session_id` | `UUID` | `NOT NULL REFERENCES sessions(id) ON DELETE CASCADE` | Associated session |
| `nickname` | `TEXT` | `NOT NULL` | Player display name (2–20 chars) |
| `avatar_color` | `TEXT` | `NOT NULL DEFAULT '#888888'` | Hex color assigned upon join |
| `total_score` | `INT` | `NOT NULL DEFAULT 0` | Running score (updated on reveal) |
| `joined_at` | `TIMESTAMPTZ` | `NOT NULL DEFAULT now()` | Timestamp when player joined |
| `disconnected_at` | `TIMESTAMPTZ` | `NULL` | Future tracking for disconnects |

**Constraints & Indexes**:
- `UNIQUE (session_id, nickname)`: Prevents duplicate nicknames in the same session.
- `CREATE INDEX players_session_id_idx ON players (session_id)`

---

### `answers` Table Reference

| Column | Type | Constraints | Description |
|---|---|---|---|
| `id` | `UUID` | `PRIMARY KEY DEFAULT gen_random_uuid()` | Unique answer record |
| `player_id` | `UUID` | `NOT NULL REFERENCES players(id) ON DELETE CASCADE` | Submitting player |
| `session_question_id` | `UUID` | `NOT NULL REFERENCES session_questions(id) ON DELETE CASCADE` | Target session question |
| `chosen_index` | `SMALLINT` | `CHECK (chosen_index BETWEEN 0 AND 3)` | 0=A, 1=B, 2=C, 3=D |
| `is_correct` | `BOOLEAN` | `NOT NULL DEFAULT false` | Computed on reveal (Feature #7) |
| `points_awarded` | `INT` | `NOT NULL DEFAULT 0` | Calculated on reveal (Feature #7) |
| `answer_time_ms` | `INT` | `CHECK (answer_time_ms >= 0)` | Milliseconds elapsed from `asked_at` |
| `answered_at` | `TIMESTAMPTZ` | `NOT NULL DEFAULT now()` | Server submission timestamp |

**Constraints & Indexes**:
- `UNIQUE (player_id, session_question_id)`: Enforces one answer per player per question (enables safe idempotency).
- `CREATE INDEX answers_session_question_id_idx ON answers (session_question_id)`
- `CREATE INDEX answers_player_id_idx ON answers (player_id)`

---

## Domain Entities & Go Structs

```go
// Player represents an ephemeral participant in an active/lobby session.
type Player struct {
	ID             uuid.UUID  `json:"id"`
	SessionID      uuid.UUID  `json:"session_id"`
	Nickname       string     `json:"nickname"`
	AvatarColor    string     `json:"avatar_color"`
	TotalScore     int        `json:"total_score"`
	JoinedAt       time.Time  `json:"joined_at"`
	DisconnectedAt *time.Time `json:"disconnected_at,omitempty"`
}

// Answer represents a participant's submission for a question.
type Answer struct {
	ID                uuid.UUID `json:"id"`
	PlayerID          uuid.UUID `json:"player_id"`
	SessionQuestionID uuid.UUID `json:"session_question_id"`
	ChosenIndex       int16     `json:"chosen_index"`
	IsCorrect         bool      `json:"is_correct"`
	PointsAwarded     int       `json:"points_awarded"`
	AnswerTimeMs      int       `json:"answer_time_ms"`
	AnsweredAt        time.Time `json:"answered_at"`
}

// PlayerJoinResult holds data returned upon successful join.
type PlayerJoinResult struct {
	PlayerID       string    `json:"player_id"`
	Nickname       string    `json:"nickname"`
	AvatarColor    string    `json:"avatar_color"`
	SessionID      string    `json:"session_id"`
	SessionName    string    `json:"session_name"`
	PlayerToken    string    `json:"player_token"`
	TokenExpiresAt time.Time `json:"token_expires_at"`
}

// AnswerSubmitResult holds data returned upon answer submission.
type AnswerSubmitResult struct {
	AnswerID    string    `json:"answer_id"`
	ChosenIndex int16     `json:"chosen_index"`
	AnsweredAt  time.Time `json:"answered_at"`
	IsDuplicate bool      `json:"-"`
}
```
