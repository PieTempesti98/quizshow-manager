# Data Model & DTOs: Feature #9 — Stats & Leaderboard

**Branch**: `009-stats-leaderboard` | **Date**: 2026-08-14 | **Spec**: [spec.md](spec.md)

## Entities & Database Schemas (Read-Only)

No schema migrations or alterations are required. Queries operate on existing PostgreSQL tables:

### 1. `sessions`
- `id` (UUID PK)
- `name` (TEXT)
- `status` (`session_status`: `draft`, `lobby`, `active`, `completed`, `cancelled`)
- `question_count` (SMALLINT)
- `started_at` (TIMESTAMPTZ)
- `ended_at` (TIMESTAMPTZ)
- `deleted_at` (TIMESTAMPTZ)

### 2. `players`
- `id` (UUID PK)
- `session_id` (UUID FK)
- `nickname` (TEXT)
- `avatar_color` (TEXT)
- `total_score` (INT)
- `joined_at` (TIMESTAMPTZ)

### 3. `answers`
- `id` (UUID PK)
- `player_id` (UUID FK)
- `session_question_id` (UUID FK)
- `chosen_index` (SMALLINT NULL)
- `is_correct` (BOOLEAN)
- `points_awarded` (INT)
- `answer_time_ms` (INT)
- `answered_at` (TIMESTAMPTZ)

### 4. `session_questions`
- `id` (UUID PK)
- `session_id` (UUID FK)
- `question_id` (UUID FK)
- `position` (SMALLINT)
- `asked_at` (TIMESTAMPTZ)
- `revealed_at` (TIMESTAMPTZ)

### 5. `questions`
- `id` (UUID PK)
- `text` (TEXT)
- `option_a`, `option_b`, `option_c`, `option_d` (TEXT)
- `correct_index` (SMALLINT)
- `difficulty` (`difficulty_level`)

---

## Domain Models & DTOs (`internal/session/models.go`)

### Leaderboard Types

```go
// SessionLeaderboardEntry represents one player's row in the final session leaderboard.
type SessionLeaderboardEntry struct {
	Rank           int       `json:"rank"`
	PlayerID       uuid.UUID `json:"player_id"`
	Nickname       string    `json:"nickname"`
	TotalScore     int       `json:"total_score"`
	AvatarColor    string    `json:"avatar_color"`
	CorrectAnswers int       `json:"correct_answers"`
	TotalQuestions int       `json:"total_questions"`
}

// SessionLeaderboardResult represents the response payload for GET /api/v1/sessions/:id/leaderboard.
type SessionLeaderboardResult struct {
	SessionID   string                    `json:"session_id"`
	SessionName string                    `json:"session_name"`
	EndedAt     *time.Time                `json:"ended_at"`
	Leaderboard []SessionLeaderboardEntry `json:"leaderboard"`
}
```

### Stats Types

```go
// QuestionStatsItem represents the aggregated statistics for a single question in a finished session.
type QuestionStatsItem struct {
	Position           int                      `json:"position"`
	Text               string                   `json:"text"`
	Difficulty         string                   `json:"difficulty"`
	CorrectIndex       int                      `json:"correct_index"`
	CorrectCount       int                      `json:"correct_count"`
	WrongCount         int                      `json:"wrong_count"`
	NoAnswerCount      int                      `json:"no_answer_count"`
	AnswerDistribution []AnswerDistributionItem `json:"answer_distribution"`
	AvgAnswerTimeMs    int                      `json:"avg_answer_time_ms"`
}

// SessionStatsResult represents the response payload for GET /api/v1/sessions/:id/stats.
type SessionStatsResult struct {
	SessionID string              `json:"session_id"`
	Questions []QuestionStatsItem `json:"questions"`
}
```

### Sentinel Errors

```go
var (
	ErrSessionNotCompleted = errors.New("session is not completed or cancelled")
)
```
