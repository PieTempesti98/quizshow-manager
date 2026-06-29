# Data Model: Session Lifecycle (006)

## No new migrations required

All tables used by this feature exist in migration 001: `sessions`, `session_categories`, `session_questions`. No schema changes needed.

---

## `session_questions` (read/write — first time populated by this feature)

Previously defined but never written to. This feature performs the first writes.

| Column       | Type         | Written by this feature? | Notes                                  |
|--------------|--------------|--------------------------|----------------------------------------|
| `id`         | UUID PK      | Yes — `gen_random_uuid()`| Auto-generated                         |
| `session_id` | UUID FK      | Yes                      | FK → `sessions.id`                     |
| `question_id`| UUID FK      | Yes                      | FK → `questions.id` (drawn at launch)  |
| `position`   | SMALLINT     | Yes — 1..N               | 1-based draw order                     |
| `asked_at`   | TIMESTAMPTZ  | NULL (set in feature #7) | Set by presenter `/next-question`       |
| `revealed_at`| TIMESTAMPTZ  | NULL (set in feature #7) | Set by presenter `/reveal`              |

Unique constraint: `(session_id, position)` — enforced by DB, also enforced by insert logic (positions are computed 1..len(drawn)).

---

## `sessions` — fields written by this feature

| Field        | Written on          | Value                    |
|--------------|---------------------|--------------------------|
| `status`     | `/open-lobby`       | `'draft'` → `'lobby'`   |
| `status`     | `/launch`           | `'lobby'` → `'active'`  |
| `started_at` | `/launch`           | `now()` UTC              |
| `updated_at` | Both endpoints      | `now()` UTC (trigger)    |

---

## New Go types (no DB schema change)

### `OpenLobbyResult` (session package)
```
SessionID  uuid.UUID
PIN        string
QRCodeURL  string   // "/api/v1/sessions/{id}/qr"
Status     string   // "lobby"
```

### `LaunchResult` (session package)
```
SessionID       uuid.UUID
Status          string      // "active"
QuestionCount   int         // actual drawn count
ProjectionToken string      // JWT, 12h TTL
ProjectionURL   string      // PLAYER_APP_BASE_URL + /projection?session=...&token=...
StartedAt       time.Time
```

### `SessionEventBroadcaster` interface (session package)
```go
type SessionEventBroadcaster interface {
    BroadcastSessionStarted(sessionID string, totalQuestions int)
}
```
No-op implementation injected in MVP; real hub injected in feature #10.

### `projectionClaims` (auth package — unexported)
```
SessionID string  `json:"session_id"`
Role      string  `json:"role"`         // "projection"
jwt.RegisteredClaims
```
Signed with the same HMAC-SHA256 + `JWT_SECRET` used for admin tokens. Subject = session ID string.

---

## New sentinel errors (session package)

| Error                    | HTTP status | Used by                         |
|--------------------------|-------------|---------------------------------|
| `ErrSessionNotInLobby`   | 409         | `Launch` — session not in lobby |
| `ErrInsufficientQuestions` | 422       | `Launch` — pool is empty        |

---

## Key queries

### OpenLobby — atomic status transition
```sql
UPDATE sessions
SET status = 'lobby', updated_at = now()
WHERE id = $1
  AND status = 'draft'
  AND deleted_at IS NULL
RETURNING id, name, pin, status::text, question_count, time_per_question_s,
          points_per_answer, speed_bonus_enabled, started_at, ended_at,
          created_by, created_at, updated_at, deleted_at
```
If `0 rows affected`: check existence separately to distinguish 404 vs 409.

### Launch — draw available questions (inside transaction)
```sql
SELECT q.id
FROM questions q
JOIN session_categories sc ON sc.category_id = q.category_id
WHERE sc.session_id = $1
  AND q.deleted_at IS NULL
ORDER BY RANDOM()
LIMIT $2
```
`$2` = `session.question_count`. If result set is empty → `ErrInsufficientQuestions`.

### Launch — insert session_questions (inside transaction, per drawn question)
```sql
INSERT INTO session_questions (session_id, question_id, position)
VALUES ($1, $2, $3)
```
`$3` = loop index 1..N.

### Launch — activate session (inside transaction)
```sql
UPDATE sessions
SET status = 'active', started_at = now(), updated_at = now()
WHERE id = $1
  AND status = 'lobby'
  AND deleted_at IS NULL
RETURNING id, name, pin, status::text, question_count, time_per_question_s,
          points_per_answer, speed_bonus_enabled, started_at, ended_at,
          created_by, created_at, updated_at, deleted_at
```
