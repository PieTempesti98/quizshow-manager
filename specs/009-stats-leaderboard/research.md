# Research & Technical Decisions: Feature #9 — Stats & Leaderboard

**Branch**: `009-stats-leaderboard` | **Date**: 2026-08-14 | **Spec**: [spec.md](spec.md)

## Summary of Decisions

| Area | Decision | Rationale | Alternatives Considered |
|---|---|---|---|
| **Status Guard** | Enforce `status IN ('completed', 'cancelled')` via repository/service guard, returning `ErrSessionNotCompleted` (HTTP 409 `SESSION_NOT_COMPLETED`). | Prevents premature disclosure of leaderboard and statistics while session is still in `draft`, `lobby`, or `active`. | Allowing partial stats during active session (deferred to presenter reveal events). |
| **Leaderboard Tie-Breaking** | `ORDER BY total_score DESC, joined_at ASC, id ASC` with sequential `ROW_NUMBER() OVER (...)` rank. | Guarantees deterministic, stable ranking across calls and CSV exports without rank gaps. | `RANK()` or `DENSE_RANK()` with shared ties (more complex for consumer UI and prize distribution). |
| **CSV Streaming & Encoding** | Go standard library `encoding/csv` writing directly into `bytes.Buffer` / response writer. | RFC 4180 compliant, standard library, zero external dependencies, robust quote escaping. | Third-party CSV libraries (unnecessary overhead). |
| **CSV Filename Sanitization** | Lowercase alphanumeric slug from session name + UTC date: `{slug}-{YYYY-MM-DD}-leaderboard.csv`. | Safe for all filesystems, browsers, and HTTP `Content-Disposition` headers; handles non-ASCII and special characters gracefully. | Raw session name in filename (causes HTTP header parsing errors with quotes/commas/spaces). |
| **Per-Question Aggregation** | Single aggregated SQL query joining `session_questions`, `questions`, and `answers` with `FILTER (WHERE ...)` and `total_players` subquery. | Computes `correct_count`, `wrong_count`, `no_answer_count`, answer distribution, and `avg_answer_time_ms` in a single fast query (<10ms). | Multiple N+1 queries per question (slow and inefficient). |

---

## Detailed Research & Architecture Notes

### 1. Leaderboard Computation (`GET /api/v1/sessions/:id/leaderboard`)

- **Precondition**: Session exists, `deleted_at IS NULL`, `status IN ('completed', 'cancelled')`.
- **Query Strategy**:
  ```sql
  SELECT
      ROW_NUMBER() OVER (ORDER BY p.total_score DESC, p.joined_at ASC, p.id ASC) AS rank,
      p.id AS player_id,
      p.nickname,
      p.total_score,
      p.avatar_color,
      COALESCE(COUNT(a.id) FILTER (WHERE a.is_correct), 0) AS correct_answers,
      s.question_count AS total_questions
  FROM players p
  JOIN sessions s ON s.id = p.session_id
  LEFT JOIN session_questions sq ON sq.session_id = s.id
  LEFT JOIN answers a ON a.session_question_id = sq.id AND a.player_id = p.id
  WHERE p.session_id = $1
  GROUP BY p.id, p.nickname, p.total_score, p.avatar_color, p.joined_at, s.question_count
  ORDER BY rank ASC;
  ```
- **Performance**: Ephemeral session size is typically 5–200 players; the query executes in ~2–5ms with the existing index on `players (session_id)`.

---

### 2. CSV Export Generation (`GET /api/v1/sessions/:id/leaderboard?format=csv`)

- **Format Specification**:
  - `Content-Type`: `text/csv; charset=utf-8`
  - `Content-Disposition`: `attachment; filename="quiz-aziendale-q2-2026-08-14-leaderboard.csv"`
  - Header Row: `rank,nickname,total_score`
  - Data Rows: `1,Mario,1250`
- **Filename Slugification**:
  - Converts non-alphanumeric characters to `-`, strips redundant hyphens, truncates to 50 characters.
  - Date format: `ended_at.Format("2006-01-02")` (or `now` if `ended_at` is null).
- **RFC 4180 Escaping**: Handled automatically by Go `encoding/csv.Writer`.

---

### 3. Per-Question Performance Breakdown (`GET /api/v1/sessions/:id/stats`)

- **Precondition**: Session exists, `deleted_at IS NULL`, `status IN ('completed', 'cancelled')`.
- **Aggregated SQL Query**:
  ```sql
  SELECT
      sq.id AS session_question_id,
      sq.position,
      q.text AS question_text,
      q.correct_index,
      q.difficulty::text,
      COALESCE(COUNT(a.id) FILTER (WHERE a.is_correct), 0) AS correct_count,
      COALESCE(COUNT(a.id) FILTER (WHERE NOT a.is_correct AND a.chosen_index IS NOT NULL), 0) AS wrong_count,
      COALESCE(COUNT(a.id) FILTER (WHERE a.chosen_index = 0), 0) AS count_0,
      COALESCE(COUNT(a.id) FILTER (WHERE a.chosen_index = 1), 0) AS count_1,
      COALESCE(COUNT(a.id) FILTER (WHERE a.chosen_index = 2), 0) AS count_2,
      COALESCE(COUNT(a.id) FILTER (WHERE a.chosen_index = 3), 0) AS count_3,
      COALESCE(AVG(a.answer_time_ms) FILTER (WHERE a.answer_time_ms IS NOT NULL), 0)::int AS avg_answer_time_ms
  FROM session_questions sq
  JOIN questions q ON q.id = sq.question_id
  LEFT JOIN answers a ON a.session_question_id = sq.id
  WHERE sq.session_id = $1
  GROUP BY sq.id, sq.position, q.text, q.correct_index, q.difficulty
  ORDER BY sq.position ASC;
  ```
- **No-Answer Count Computation**:
  - Given `total_players` count for the session, `no_answer_count = max(0, total_players - (correct_count + wrong_count))`.
  - For unasked or early-cancelled questions, `correct_count = 0`, `wrong_count = 0`, `no_answer_count = total_players`.
