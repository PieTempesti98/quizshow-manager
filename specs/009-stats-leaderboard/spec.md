# Feature Specification: Feature #9 — Stats & Leaderboard

**Feature Branch**: `009-stats-leaderboard`  
**Created**: 2026-08-14  
**Status**: Draft  
**Input**: User description: "Feature #9 — Stats + leaderboard (US-ST01, US-ST02, US-S04): GET /api/v1/sessions/:id/leaderboard (status guard completed/cancelled, ranking ordered by total_score DESC, joined_at ASC, id ASC, JSON and CSV export ?format=csv with filename header) and GET /api/v1/sessions/:id/stats (status guard, per-question breakdown ordered by position with correct_count, wrong_count, no_answer_count), session history support, repository aggregation queries and unit/integration tests"

## User Scenarios & Testing *(mandatory)*

### User Story 1 — View and Export Final Session Leaderboard (Priority: P1)

As an administrator or quiz host, I want to retrieve the final ranking of all participants for a completed or cancelled quiz session in JSON format or download it as a CSV spreadsheet, so that I can announce winners, archive session results, and share the scoreboard with players or stakeholders.

**Why this priority**: The final leaderboard is the primary outcome of any competitive quiz show. It provides complete transparency into player standings and allows external reporting and spreadsheet archiving.

**Independent Test**: Call `GET /api/v1/sessions/:id/leaderboard` for a session in `completed` status with multiple players with varying scores. Verify HTTP 200 returning the ordered player list with 1-based ranks. Call `GET /api/v1/sessions/:id/leaderboard?format=csv` and verify HTTP 200 with `Content-Type: text/csv; charset=utf-8`, correct `Content-Disposition` attachment header, and CSV data matching the JSON rankings.

**Acceptance Scenarios**:

1. **Given** a session with status `completed` or `cancelled` containing joined players and recorded scores, **When** the admin requests `GET /api/v1/sessions/:id/leaderboard`, **Then** the system returns HTTP 200 with `session_id`, `session_name`, `ended_at`, and the full list of players ordered by `total_score DESC`, `joined_at ASC`, and `id ASC` with 1-based sequential ranks (`1, 2, 3, ...`).
2. **Given** a session in `completed` or `cancelled` status, **When** the admin requests `GET /api/v1/sessions/:id/leaderboard?format=csv`, **Then** the system returns HTTP 200 with `Content-Type: text/csv; charset=utf-8`, `Content-Disposition: attachment; filename="{sanitized_session_name}-{date}-leaderboard.csv"`, and CSV content containing headers `rank,nickname,total_score` and rows for each participant.
3. **Given** a session that is in `draft`, `lobby`, or `active` status, **When** an admin requests the leaderboard (JSON or CSV), **Then** the system rejects the request with HTTP 409 (`SESSION_NOT_COMPLETED`).
4. **Given** a non-existent session ID or a soft-deleted session, **When** requesting the leaderboard, **Then** the system returns HTTP 404 (`NOT_FOUND`).
5. **Given** a completed session with zero players, **When** requesting the leaderboard, **Then** the system returns HTTP 200 with an empty `leaderboard` array (or empty CSV with just headers).
6. **Given** an unauthenticated request or a request with non-admin credentials, **When** requesting the leaderboard, **Then** the system returns HTTP 401 (`UNAUTHORIZED`) or HTTP 403 (`FORBIDDEN`).

---

### User Story 2 — View Post-Session Per-Question Performance Breakdown (Priority: P1)

As an administrator, I want to inspect a detailed per-question statistical breakdown for a completed or cancelled quiz session, showing question text, correct answer, and counts of correct, incorrect, and unanswered responses, so that I can analyze question difficulty, player engagement, and question quality.

**Why this priority**: Post-session question statistics enable organizers to evaluate whether questions were too hard, too easy, or ambiguous, and assess participant performance across topics.

**Independent Test**: Call `GET /api/v1/sessions/:id/stats` on a completed session where questions have a mix of correct, incorrect, and timed-out responses. Verify HTTP 200 returning questions sorted by session position (1..N) with accurate aggregated counts for correct answers, wrong answers, and unanswered players.

**Acceptance Scenarios**:

1. **Given** a session in `completed` or `cancelled` status with drawn questions and recorded answers, **When** the admin requests `GET /api/v1/sessions/:id/stats`, **Then** the system returns HTTP 200 with `session_id` and a list of questions ordered by `position ASC` (1..N).
2. **Given** each question in the breakdown, **When** rendered, **Then** the system provides `position`, `question_text`, `correct_index`, `correct_count`, `wrong_count`, and `no_answer_count` (calculated as total session players minus players who submitted an answer).
3. **Given** a question where some players answered correctly, some chose wrong options, and some did not answer before the timer expired, **When** querying stats, **Then** `correct_count + wrong_count + no_answer_count` equals the total number of players registered in the session.
4. **Given** a session in `draft`, `lobby`, or `active` status, **When** the admin requests stats, **Then** the system rejects the request with HTTP 409 (`SESSION_NOT_COMPLETED`).
5. **Given** a session cancelled early before all questions were asked, **When** querying stats, **Then** unasked questions are included with 0 correct, 0 wrong, and `no_answer_count` matching total players (or 0 if no players joined).
6. **Given** an unauthenticated or unauthorized request, **When** requesting stats, **Then** the system returns HTTP 401 (`UNAUTHORIZED`) or HTTP 403 (`FORBIDDEN`).

---

### User Story 3 — Browse Historical Sessions (Priority: P2)

As an administrator, I want to filter and browse the list of past finished sessions (completed or cancelled) with their execution dates, participant counts, and status, so that I can locate historical quizzes and navigate to their individual stats and leaderboards.

**Why this priority**: Session history provides the navigation hub in the admin panel to discover and drill down into past quiz results.

**Independent Test**: Call `GET /api/v1/sessions?status=completed,cancelled` with pagination params. Verify HTTP 200 returning only sessions in `completed` or `cancelled` status ordered by creation/start date descending, including participant counts.

**Acceptance Scenarios**:

1. **Given** multiple sessions in various statuses (`draft`, `lobby`, `active`, `completed`, `cancelled`), **When** requesting `GET /api/v1/sessions?status=completed,cancelled`, **Then** the system returns only sessions with status `completed` or `cancelled` with correct pagination metadata and player counts.
2. **Given** an admin selecting a past session from the history list, **When** clicking the session, **Then** the admin can directly access the session's leaderboard and question breakdown.

---

### Edge Cases

- **Tied Scores in Leaderboard**: If two or more players have the exact same `total_score`, ties are broken deterministically by `joined_at ASC` (first to join gets higher rank) and `id ASC` (UUID comparison), guaranteeing stable, non-fluctuating ranks.
- **Session Cancelled with Zero Questions or Zero Answers**: If a session is cancelled during the lobby phase or immediately after launch before any questions are answered, `leaderboard` returns all players with `0` score, and `stats` returns questions with `0` responses and `no_answer_count` equal to the player count.
- **Special Characters in Session Name for CSV Export**: Session names containing commas, quotes, spaces, or non-ASCII characters (e.g. `Quiz "Estate 2026" & Musica!`) must have their exported filename sanitized into URL-safe/filesystem-safe ASCII characters (e.g. `quiz-estate-2026-musica-2026-08-14-leaderboard.csv`), while CSV content must properly escape quotes and commas following RFC 4180.
- **Player Disconnected Mid-Session**: Players who joined and then disconnected remain registered in the `players` table and are included in the final leaderboard and question statistics.
- **Large Participant Count in CSV Export**: When exporting leaderboards with hundreds of participants, CSV serialization streams or writes the output efficiently without buffer exhaustion or memory leaks.

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: System MUST provide endpoint `GET /api/v1/sessions/:id/leaderboard` protected by Admin JWT authentication (`RequireAdmin`).
- **FR-002**: `GET /api/v1/sessions/:id/leaderboard` MUST verify that the session exists and has status `completed` or `cancelled`. If the session is in `draft`, `lobby`, or `active` status, system MUST return HTTP 409 with error code `SESSION_NOT_COMPLETED`.
- **FR-003**: System MUST compute final rankings for all participants registered in the session, ordered deterministically by `total_score DESC`, `joined_at ASC`, `id ASC`.
- **FR-004**: Each participant in the JSON leaderboard response MUST include `rank` (1-based sequential integer), `nickname`, and `total_score` (and optionally `avatar_color`, `correct_answers`, `total_questions`).
- **FR-005**: `GET /api/v1/sessions/:id/leaderboard` MUST support query parameter `format=csv`.
- **FR-006**: When `format=csv` is specified, the system MUST return HTTP 200 with header `Content-Type: text/csv; charset=utf-8` and header `Content-Disposition: attachment; filename="{session-slug}-{date}-leaderboard.csv"`.
- **FR-007**: The exported CSV MUST strictly follow RFC 4180, containing the header line `rank,nickname,total_score` and properly escaped participant records.
- **FR-008**: System MUST provide endpoint `GET /api/v1/sessions/:id/stats` protected by Admin JWT authentication (`RequireAdmin`).
- **FR-009**: `GET /api/v1/sessions/:id/stats` MUST verify that the session exists and has status `completed` or `cancelled`. If not, system MUST return HTTP 409 with error code `SESSION_NOT_COMPLETED`.
- **FR-010**: `GET /api/v1/sessions/:id/stats` MUST aggregate per-question response statistics for all questions assigned to the session, ordered by `position ASC` (1..N).
- **FR-011**: For each question, the statistics breakdown MUST include:
  - `position`: question number in presentation sequence
  - `question_text` (or `text`): text of the question
  - `correct_index`: index of the correct answer (0..3)
  - `correct_count`: count of players who selected the correct answer
  - `wrong_count`: count of players who selected an incorrect answer
  - `no_answer_count`: count of joined session players who did not submit an answer
  - `answer_distribution` (optional): array of `{ index, count }` for all 4 choices
  - `avg_answer_time_ms` (optional): average response time in milliseconds for submitted answers
- **FR-012**: System MUST support querying session history via existing `GET /api/v1/sessions?status=completed,cancelled` with pagination and date descending sort order.
- **FR-013**: All JSON success responses MUST wrap their payload in the standard `{ "data": { ... } }` envelope.
- **FR-014**: All error responses MUST strictly follow the standard format `{ "error": { "code": "...", "message": "..." } }`.

### Key Entities

- **Session**: The completed or cancelled quiz instance (`id`, `name`, `status`, `started_at`, `ended_at`, `question_count`).
- **Player**: Participant with cumulative score (`id`, `session_id`, `nickname`, `avatar_color`, `total_score`, `joined_at`).
- **Leaderboard**: Aggregated ranking entity consisting of ordered player standings with computed 1-based ranks.
- **SessionQuestion**: Snapshot question item (`id`, `session_id`, `question_id`, `position`, `asked_at`, `revealed_at`).
- **QuestionStats**: Aggregated per-question performance metrics (`position`, `question_text`, `correct_index`, `correct_count`, `wrong_count`, `no_answer_count`, `answer_distribution`, `avg_answer_time_ms`).

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: Leaderboard endpoint (`GET /sessions/:id/leaderboard`) calculates and returns complete rankings in under 50ms for sessions with up to 500 participants.
- **SC-002**: Question stats endpoint (`GET /sessions/:id/stats`) calculates and returns aggregated breakdown in under 50ms for sessions with up to 50 questions.
- **SC-003**: 100% of requests to `/leaderboard` or `/stats` on non-completed sessions (`draft`, `lobby`, `active`) receive HTTP 409 `SESSION_NOT_COMPLETED`.
- **SC-004**: CSV export produces 100% valid RFC 4180 CSV files that can be opened without parsing errors in standard spreadsheet applications (Microsoft Excel, Apple Numbers, Google Sheets).
- **SC-005**: For every question in stats breakdown, the sum of `correct_count + wrong_count + no_answer_count` exactly equals the total player count of the session.
- **SC-006**: Unit and integration test coverage for leaderboard calculation, tie-breaking, CSV generation, and SQL stats aggregation exceeds 90%.

## Assumptions

- Leaderboard and question stats are only accessible to administrators in MVP (ADR-001 / `RequireAdmin`). Players receive their personal score and rank through the live reveal / end-of-quiz WebSocket events and screens.
- Total score is read from the denormalized `players.total_score` field, which is updated transactionally at each question reveal (Feature #7).
- No new database tables or schema migrations are required; all queries operate on existing PostgreSQL tables (`sessions`, `players`, `answers`, `session_questions`, `questions`).
- The date string in the CSV export filename uses UTC date format `YYYY-MM-DD` derived from session `ended_at` (or `created_at` fallback).
