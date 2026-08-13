# Implementation Plan: Feature #9 — Stats & Leaderboard

**Branch**: `009-stats-leaderboard` | **Date**: 2026-08-14 | **Spec**: [spec.md](spec.md)  
**Input**: Feature specification from `/specs/009-stats-leaderboard/spec.md`

## Summary

Implement the backend REST endpoints and aggregation queries for post-session statistics, final leaderboard ranking, CSV export, and session history:

1. `GET /api/v1/sessions/:id/leaderboard` (Admin protected):
   - Enforces status guard: session must be in `completed` or `cancelled` status (otherwise HTTP 409 `SESSION_NOT_COMPLETED`).
   - Retrieves all participants ordered deterministically by `total_score DESC, joined_at ASC, id ASC`.
   - Computes sequential 1-based ranks (`1, 2, 3...`).
   - Supports `?format=csv` query parameter:
     - Formats output as RFC 4180 CSV with columns `rank,nickname,total_score`.
     - Sets headers `Content-Type: text/csv; charset=utf-8` and `Content-Disposition: attachment; filename="{session-slug}-{date}-leaderboard.csv"`.
   - Returns standard JSON response by default.

2. `GET /api/v1/sessions/:id/stats` (Admin protected):
   - Enforces status guard: session must be in `completed` or `cancelled` status (otherwise HTTP 409 `SESSION_NOT_COMPLETED`).
   - Aggregates per-question breakdown ordered by `position ASC` (1..N).
   - Computes `correct_count`, `wrong_count`, and `no_answer_count` (total players - answered count), along with answer distribution and average answer time.
   - Returns standard JSON response.

3. `GET /api/v1/sessions?status=completed,cancelled` (Admin protected):
   - Validates session history querying with terminal statuses, pagination, and date descending ordering.

---

## Technical Context

**Language/Version**: Go 1.25  
**Primary Dependencies**: `gofiber/fiber` v2, `jackc/pgx` v5, `google/uuid` v1, standard library `encoding/csv`  
**Storage**: PostgreSQL — tables `sessions`, `players`, `answers`, `session_questions`, `questions` (schema `001_initial_schema.up.sql`)  
**Testing**: Go `testing` package with unit tests for ranking/tie-breaking/CSV serialization and integration tests for SQL aggregations  
**Target Platform**: Linux server (Docker Compose)  
**Project Type**: Web service (REST API)  
**Performance Goals**: Leaderboard query < 50ms, Stats query < 50ms  
**Constraints**: Admin JWT authentication (`RequireAdmin`), standard API response envelope, RFC 4180 CSV export  
**Scale/Scope**: MVP — up to 500 players per session, up to 50 questions per session.

---

## Constitution Check

### Principle 1.1 & 1.2 — Spec-First & MVP Discipline ✅
- Feature spec exists at [specs/009-stats-leaderboard/spec.md](spec.md).
- Covers MVP user stories US-ST01 (question breakdown), US-ST02 (final leaderboard + CSV export), and US-S04 (session history). Advanced analytics and PDF exports are deferred to R2.

### Principle 2.1 & 2.2 — Package Structure & No Global State ✅
- Extends `internal/session/` maintaining clean separation: models, repository, service, handler.
- Dependencies injected explicitly via constructors.

### Principle 2.3 & 2.4 — Database Access & Migrations ✅
- Direct `pgx` queries with explicit SQL and aggregation functions (`FILTER (WHERE ...)`).
- All required tables exist in `001_initial_schema.up.sql`. No migration required.

### Principle 4.3 & 5.2–5.4 — API Envelope, UUIDs, UTC & No Business Logic in SQL ✅
- Standard JSON envelope `{ "data": ... }` and `{ "error": ... }`.
- Binary/CSV streaming handles file downloads cleanly with standard headers.
- UUID v4 for all entity IDs; timestamps in UTC ISO 8601.
- Business rules (status checking, CSV serialization, filename slugification) handled in Go service.

### Principle 6.1 — Unit Tests for Business Logic ✅
- Unit tests for leaderboard ranking, tie-breaking, CSV formatting, and filename generation in `internal/session/`.

**Gate result**: PASS — proceed to implementation planning.

---

## Project Structure

### Documentation (this feature)

```text
specs/009-stats-leaderboard/
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
├── cmd/server/main.go                          ← Register new endpoints on protected admin group
└── internal/
    └── session/
        ├── models.go                           ← SessionLeaderboardEntry, QuestionStatsItem, ErrSessionNotCompleted
        ├── csv.go                              ← NEW: CSV formatting and filename slug helper
        ├── csv_test.go                         ← NEW: Unit tests for CSV generation and slugification
        ├── repository.go                       ← GetLeaderboard, GetStats repository queries
        ├── service.go                          ← GetLeaderboard, GetLeaderboardCSV, GetStats service methods
        ├── handler.go                          ← GetLeaderboard, GetStats HTTP handlers
        └── stats_test.go                       ← NEW: Unit & integration tests for leaderboard & stats
```

---

## Complexity Tracking

No constitution violations. No complexity justification required.

---

## Implementation Steps

### Step 1 — Models, DTOs & Sentinel Errors (`internal/session/models.go`)
- Define `SessionLeaderboardEntry`:
  ```go
  type SessionLeaderboardEntry struct {
      Rank           int       `json:"rank"`
      PlayerID       uuid.UUID `json:"player_id"`
      Nickname       string    `json:"nickname"`
      TotalScore     int       `json:"total_score"`
      AvatarColor    string    `json:"avatar_color"`
      CorrectAnswers int       `json:"correct_answers"`
      TotalQuestions int       `json:"total_questions"`
  }
  ```
- Define `SessionLeaderboardResult`:
  ```go
  type SessionLeaderboardResult struct {
      SessionID   string                    `json:"session_id"`
      SessionName string                    `json:"session_name"`
      EndedAt     *time.Time                `json:"ended_at"`
      Leaderboard []SessionLeaderboardEntry `json:"leaderboard"`
  }
  ```
- Define `QuestionStatsItem` and `SessionStatsResult`:
  ```go
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
  type SessionStatsResult struct {
      SessionID string              `json:"session_id"`
      Questions []QuestionStatsItem `json:"questions"`
  }
  ```
- Define Sentinel Error:
  ```go
  var ErrSessionNotCompleted = errors.New("session is not completed or cancelled")
  ```

### Step 2 — CSV Formatting & Filename Slugification (`internal/session/csv.go`)
- Implement `GenerateLeaderboardCSV(entries []SessionLeaderboardEntry) ([]byte, error)`:
  - Uses `encoding/csv.Writer`.
  - Writes header: `[]string{"rank", "nickname", "total_score"}`.
  - Iterates entries writing string slices `[]string{strconv.Itoa(e.Rank), e.Nickname, strconv.Itoa(e.TotalScore)}`.
- Implement `FormatLeaderboardCSVFilename(sessionName string, endedAt *time.Time, fallbackCreatedAt time.Time) string`:
  - Slugifies `sessionName` (converts spaces/punctuation to `-`, trims non-alphanumeric, limits length).
  - Formats date as `YYYY-MM-DD`.
  - Returns `"{slug}-{date}-leaderboard.csv"`.

### Step 3 — Repository Aggregations (`internal/session/repository.go`)
- Implement `GetLeaderboard(ctx context.Context, sessionID uuid.UUID) (Session, []SessionLeaderboardEntry, error)`:
  - Fetches session details (verifying existence and `deleted_at IS NULL`).
  - Verifies `status IN ('completed', 'cancelled')` -> returns `ErrSessionNotCompleted` if not.
  - Executes leaderboard aggregation query with `ROW_NUMBER() OVER (ORDER BY p.total_score DESC, p.joined_at ASC, p.id ASC)`.
  - Returns session and player rankings.
- Implement `GetStats(ctx context.Context, sessionID uuid.UUID) (Session, []QuestionStatsItem, error)`:
  - Fetches session details and total players count.
  - Verifies `status IN ('completed', 'cancelled')` -> returns `ErrSessionNotCompleted` if not.
  - Executes per-question aggregation query joining `session_questions`, `questions`, and `answers`.
  - Computes `no_answer_count = max(0, totalPlayers - (correctCount + wrongCount))`.
  - Assembles `AnswerDistribution` with percentage calculations.
  - Returns session and question stats.

### Step 4 — Service Layer (`internal/session/service.go`)
- Implement `GetLeaderboard(ctx context.Context, sessionID uuid.UUID) (SessionLeaderboardResult, error)`:
  - Calls `r.GetLeaderboard`.
  - Returns `SessionLeaderboardResult`.
- Implement `GetLeaderboardCSV(ctx context.Context, sessionID uuid.UUID) (filename string, data []byte, err error)`:
  - Calls `r.GetLeaderboard`.
  - Generates CSV bytes via `GenerateLeaderboardCSV`.
  - Generates filename via `FormatLeaderboardCSVFilename`.
  - Returns `(filename, data, nil)`.
- Implement `GetStats(ctx context.Context, sessionID uuid.UUID) (SessionStatsResult, error)`:
  - Calls `r.GetStats`.
  - Returns `SessionStatsResult`.

### Step 5 — HTTP Handlers & Route Registration (`internal/session/handler.go` & `cmd/server/main.go`)
- In `internal/session/handler.go`:
  - `GetLeaderboard`: Parses `:id`, checks `c.Query("format") == "csv"`.
    - If CSV: calls `s.GetLeaderboardCSV`, sets `Content-Type: text/csv; charset=utf-8`, `Content-Disposition: attachment; filename="{filename}"`, sends bytes.
    - If JSON: calls `s.GetLeaderboard`, returns HTTP 200 with `{ "data": ... }`.
    - Maps errors: `ErrSessionNotFound` -> 404, `ErrSessionNotCompleted` -> 409 `SESSION_NOT_COMPLETED`.
  - `GetStats`: Parses `:id`, calls `s.GetStats`, returns HTTP 200 with `{ "data": ... }`.
    - Maps errors: `ErrSessionNotFound` -> 404, `ErrSessionNotCompleted` -> 409 `SESSION_NOT_COMPLETED`.
- In `cmd/server/main.go`:
  - Register endpoints under the protected admin group:
    ```go
    protected.Get("/sessions/:id/leaderboard", sessionHandler.GetLeaderboard)
    protected.Get("/sessions/:id/stats", sessionHandler.GetStats)
    ```

### Step 6 — Testing (`internal/session/csv_test.go` & `internal/session/stats_test.go`)
- Unit tests for `GenerateLeaderboardCSV` (valid rows, special characters, empty entries).
- Unit tests for `FormatLeaderboardCSVFilename` (spaces, accents, fallback dates).
- Unit/integration tests for `GetLeaderboard` (status guards, tie-breaking order, correct rank numbering).
- Unit/integration tests for `GetStats` (status guards, accurate counts, question ordering).

---

## Verification & Acceptance Testing

1. Run automated test suite: `go test -v ./internal/session/...`
2. Follow manual verification steps in [quickstart.md](quickstart.md).
