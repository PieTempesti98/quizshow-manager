# QuizShow — API Design

## Conventions

### Base URL
All REST endpoints are prefixed with `/api/v1/`.

### Response envelope
```json
// Success
{ "data": { ... } }

// Error
{ "error": { "code": "MACHINE_READABLE_CODE", "message": "Human readable message" } }
```
Never mix: a non-2xx status always has `error`, a 2xx always has `data`.

### Authentication
Three token types, each with a distinct role claim:

| Token type | Issued by | TTL | Role claim | Used for |
|---|---|---|---|---|
| Admin access token | `POST /auth/login` | 15 min | `admin` | All admin REST endpoints |
| Admin refresh token | `POST /auth/login` | 7 days | — | `POST /auth/refresh` only |
| Player token | `POST /sessions/:id/join` | 4 hours | `player` | WebSocket auth + answer submission |
| Projection token | `POST /sessions/:id/launch` | 12 hours | `projection` | Projection Screen WebSocket only |

All tokens are JWTs signed with `JWT_SECRET`. Protected endpoints require:
```
Authorization: Bearer <token>
```
Refresh tokens are stored in httpOnly cookies and also in the `refresh_tokens` table for server-side revocation.

### Error codes (standard set)
| Code | HTTP status | Meaning |
|---|---|---|
| `UNAUTHORIZED` | 401 | Missing or invalid token |
| `FORBIDDEN` | 403 | Token valid but insufficient role |
| `NOT_FOUND` | 404 | Resource does not exist |
| `CONFLICT` | 409 | Duplicate resource (e.g. nickname taken) |
| `VALIDATION_ERROR` | 422 | Request body failed validation |
| `SESSION_NOT_IN_LOBBY` | 409 | Player tried to join a non-lobby session |
| `SESSION_NOT_ACTIVE` | 409 | Presenter command on non-active session |
| `QUESTION_IN_USE` | 409 | Attempted to delete a question used in active session |
| `CATEGORY_HAS_QUESTIONS` | 409 | Attempted to delete category with active questions |
| `INSUFFICIENT_QUESTIONS` | 422 | Not enough questions in pool to fill session |

### Timestamps
All timestamps are UTC ISO 8601: `"2025-04-20T10:30:00Z"`.

### IDs
All IDs are UUID v4 strings.

---

## Auth

### POST /api/v1/auth/login
Admin login. No auth required.

**Request**
```json
{
  "email": "admin@quizshow.local",
  "password": "secret"
}
```

**Response 200**
```json
{
  "data": {
    "access_token": "<jwt>",
    "expires_at": "2025-04-20T10:45:00Z"
  }
}
```
Refresh token set as httpOnly cookie `qz_refresh`.

**Response 401**
```json
{ "error": { "code": "UNAUTHORIZED", "message": "Invalid credentials" } }
```
Note: never indicate which field is wrong.

---

### POST /api/v1/auth/refresh
Exchange refresh token for a new access token. No Bearer required — uses httpOnly cookie.

**Response 200**
```json
{
  "data": {
    "access_token": "<jwt>",
    "expires_at": "2025-04-20T11:00:00Z"
  }
}
```

**Response 401** — cookie missing, token expired, or revoked.

---

### POST /api/v1/auth/logout
Revoke current refresh token. Requires admin Bearer.

**Response 200**
```json
{ "data": { "ok": true } }
```
Clears `qz_refresh` cookie and marks token as revoked in DB.

---

## Categories

### GET /api/v1/categories
List all active categories. Requires admin Bearer.

**Response 200**
```json
{
  "data": {
    "categories": [
      {
        "id": "uuid",
        "name": "Storia",
        "slug": "storia",
        "question_count": 42,
        "created_at": "2025-04-01T09:00:00Z"
      }
    ]
  }
}
```
`question_count` = non-deleted questions in this category.

---

### POST /api/v1/categories
Create a category. Requires admin Bearer.

**Request**
```json
{ "name": "Scienza" }
```
Slug is auto-generated from name (lowercase, spaces → hyphens).

**Response 201**
```json
{
  "data": {
    "id": "uuid",
    "name": "Scienza",
    "slug": "scienza",
    "question_count": 0,
    "created_at": "2025-04-20T10:00:00Z"
  }
}
```

**Response 409** — category name already exists.

---

### PATCH /api/v1/categories/:id
Rename a category. Requires admin Bearer.

**Request**
```json
{ "name": "Scienze naturali" }
```

**Response 200** — updated category object.
**Response 409** — new name conflicts with existing category.

---

### DELETE /api/v1/categories/:id
Soft-delete a category. Requires admin Bearer.

**Response 200**
```json
{ "data": { "ok": true } }
```

**Response 409 CATEGORY_HAS_QUESTIONS**
```json
{
  "error": {
    "code": "CATEGORY_HAS_QUESTIONS",
    "message": "Cannot delete category with 12 active questions"
  }
}
```

---

## Questions

### GET /api/v1/questions
Paginated list with filters. Requires admin Bearer.

**Query params**
| Param | Type | Default | Description |
|---|---|---|---|
| `page` | int | 1 | Page number (1-based) |
| `per_page` | int | 25 | Max 100 |
| `category_id` | uuid[] | — | Comma-separated, multi-select |
| `difficulty` | string[] | — | `easy`, `medium`, `hard` (comma-separated) |
| `q` | string | — | Full-text search on question text |

**Response 200**
```json
{
  "data": {
    "questions": [
      {
        "id": "uuid",
        "category": { "id": "uuid", "name": "Storia" },
        "text": "In che anno è caduta Roma?",
        "option_a": "476 d.C.",
        "option_b": "410 d.C.",
        "option_c": "395 d.C.",
        "option_d": "455 d.C.",
        "correct_index": 0,
        "difficulty": "medium",
        "created_at": "2025-04-01T09:00:00Z"
      }
    ],
    "pagination": {
      "page": 1,
      "per_page": 25,
      "total": 142,
      "total_pages": 6
    }
  }
}
```

---

### POST /api/v1/questions
Create a question. Requires admin Bearer.

**Request**
```json
{
  "category_id": "uuid",
  "text": "In che anno è caduta Roma?",
  "option_a": "476 d.C.",
  "option_b": "410 d.C.",
  "option_c": "395 d.C.",
  "option_d": "455 d.C.",
  "correct_index": 0,
  "difficulty": "medium"
}
```
`difficulty` optional, defaults to `"medium"`.

**Response 201** — full question object (same shape as list item).
**Response 422** — validation error (missing fields, invalid correct_index, etc.).

---

### PATCH /api/v1/questions/:id
Update a question. Requires admin Bearer. All fields optional (partial update).

**Request** — any subset of question fields.

**Response 200** — updated question object.
**Response 409 QUESTION_IN_USE** — question is in an active session.

---

### DELETE /api/v1/questions/:id
Soft-delete. Requires admin Bearer.

**Response 200** `{ "data": { "ok": true } }`
**Response 409 QUESTION_IN_USE** — question is in an active session.

---

### POST /api/v1/questions/import
Bulk import from CSV. Requires admin Bearer.

**Request** — `multipart/form-data`
| Field | Type | Description |
|---|---|---|
| `file` | file | CSV file, max 5MB |
| `on_error` | string | `"abort"` (default) or `"skip"` — abort = reject all on any error, skip = import valid rows |

**CSV format**
```
text,option_a,option_b,option_c,option_d,correct_index,category_name,difficulty
"In che anno...","476 d.C.","410 d.C.","395 d.C.","455 d.C.",0,Storia,medium
```
- `correct_index`: integer 0–3
- `difficulty`: `easy` / `medium` / `hard` (optional, defaults to `medium`)
- `category_name`: must match an existing active category name (case-insensitive)
- Max 500 rows per import

**Response 200**
```json
{
  "data": {
    "imported": 48,
    "skipped": 2,
    "errors": [
      {
        "row": 12,
        "message": "Category 'Sport' not found"
      },
      {
        "row": 34,
        "message": "correct_index must be between 0 and 3, got: 5"
      }
    ]
  }
}
```
If `on_error: "abort"` and any errors exist: `imported: 0`, full error list returned, nothing written to DB.

**GET /api/v1/questions/import/template**
Returns the CSV template file. No auth required.

---

## Sessions

### GET /api/v1/sessions
List sessions. Requires admin Bearer.

**Query params**
| Param | Type | Default | Description |
|---|---|---|---|
| `status` | string[] | — | Filter by status (comma-separated) |
| `page` | int | 1 | |
| `per_page` | int | 20 | |

**Response 200**
```json
{
  "data": {
    "sessions": [
      {
        "id": "uuid",
        "name": "Quiz aziendale Q2",
        "pin": "482910",
        "status": "completed",
        "question_count": 20,
        "time_per_question_s": 30,
        "points_per_answer": 100,
        "speed_bonus_enabled": true,
        "player_count": 14,
        "started_at": "2025-04-20T10:00:00Z",
        "ended_at": "2025-04-20T10:45:00Z",
        "created_at": "2025-04-19T15:00:00Z"
      }
    ],
    "pagination": { "page": 1, "per_page": 20, "total": 8, "total_pages": 1 }
  }
}
```

---

### POST /api/v1/sessions
Create a session. Requires admin Bearer.

**Request**
```json
{
  "name": "Quiz aziendale Q2",
  "category_ids": ["uuid-1", "uuid-2"],
  "question_count": 20,
  "time_per_question_s": 30,
  "points_per_answer": 100,
  "speed_bonus_enabled": true
}
```

**Response 201**
```json
{
  "data": {
    "id": "uuid",
    "name": "Quiz aziendale Q2",
    "pin": "482910",
    "status": "draft",
    "question_count": 20,
    "time_per_question_s": 30,
    "points_per_answer": 100,
    "speed_bonus_enabled": true,
    "available_questions": 63,
    "player_count": 0,
    "created_at": "2025-04-20T09:00:00Z"
  }
}
```
`available_questions` = number of non-deleted questions in the selected categories.
If `available_questions < question_count`, session is created anyway but a `warning` field is added:
```json
"warning": "Only 15 questions available, session will use all of them"
```

**Response 422 VALIDATION_ERROR** — missing fields, invalid values.

---

### GET /api/v1/sessions/:id
Session detail. Requires admin Bearer.

**Response 200** — full session object (same as list item) plus `categories` array:
```json
{
  "data": {
    "id": "uuid",
    "name": "...",
    "pin": "482910",
    "status": "draft",
    "categories": [
      { "id": "uuid", "name": "Storia" },
      { "id": "uuid", "name": "Scienza" }
    ],
    ...
  }
}
```

---

### PATCH /api/v1/sessions/:id
Update session configuration. Requires admin Bearer. Only allowed when `status = draft`.

**Request** — any subset of: `name`, `category_ids`, `question_count`, `time_per_question_s`, `points_per_answer`, `speed_bonus_enabled`.

**Response 200** — updated session object.
**Response 409** — session is not in `draft` status.

---

### DELETE /api/v1/sessions/:id
Soft-delete. Requires admin Bearer. Only allowed when `status = draft`.

**Response 200** `{ "data": { "ok": true } }`
**Response 409** — session is not in `draft` status.

---

### POST /api/v1/sessions/:id/open-lobby
Transition session from `draft` → `lobby`. PIN becomes active, players can join.
Requires admin Bearer.

**Response 200**
```json
{
  "data": {
    "session_id": "uuid",
    "pin": "482910",
    "qr_code_url": "/api/v1/sessions/uuid/qr",
    "status": "lobby"
  }
}
```
**Response 409** — session not in `draft` status.

---

### GET /api/v1/sessions/:id/qr
Returns QR code image (PNG) encoding the player join URL.
No auth required — URL is already scoped to session ID.

**Response 200** — `Content-Type: image/png`

---

### POST /api/v1/sessions/:id/launch
Transition `lobby` → `active`. Draws questions, opens presenter and projection access.
Requires admin Bearer.

**Response 200**
```json
{
  "data": {
    "session_id": "uuid",
    "status": "active",
    "question_count": 20,
    "projection_token": "<jwt>",
    "projection_url": "https://quizshow.local/projection?session=uuid&token=<jwt>",
    "started_at": "2025-04-20T10:00:00Z"
  }
}
```
`projection_token` — JWT with `{ session_id, role: "projection" }`, TTL 12h.
Questions are randomly drawn from pool at this moment and written to `session_questions`.

**Response 409** — session not in `lobby` status.
**Response 422 INSUFFICIENT_QUESTIONS** — pool is empty.

---

### POST /api/v1/sessions/:id/next-question
Advance to next question. Requires admin Bearer. Session must be `active`.

**Response 200**
```json
{
  "data": {
    "session_question_id": "uuid",
    "position": 3,
    "total": 20,
    "question": {
      "text": "In che anno è caduta Roma?",
      "option_a": "476 d.C.",
      "option_b": "410 d.C.",
      "option_c": "395 d.C.",
      "option_d": "455 d.C."
    },
    "asked_at": "2025-04-20T10:15:00Z"
  }
}
```
Note: `correct_index` is **not** included — it is only sent in the reveal event.
Triggers WebSocket event `question_started` to all session clients.

**Response 409** — no more questions (use `/end` instead), or session not active.

---

### POST /api/v1/sessions/:id/pause-timer
Pause the current question timer. Requires admin Bearer. Session must be `active`.

**Response 200**
```json
{ "data": { "ok": true, "paused_at": "2025-04-20T10:16:05Z" } }
```
Triggers WebSocket event `timer_paused` to all clients.

---

### POST /api/v1/sessions/:id/resume-timer
Resume paused timer. Requires admin Bearer.

**Response 200**
```json
{ "data": { "ok": true, "resumed_at": "2025-04-20T10:16:20Z" } }
```
Triggers WebSocket event `timer_resumed` to all clients.
Server adjusts `asked_at` to account for pause duration, keeping `answer_time_ms` accurate.

---

### POST /api/v1/sessions/:id/reveal
Reveal the correct answer for the current question and compute scores.
Requires admin Bearer. Can be called manually or triggered automatically by timer expiry.

**Response 200**
```json
{
  "data": {
    "session_question_id": "uuid",
    "correct_index": 0,
    "answer_distribution": [
      { "index": 0, "count": 9, "percent": 64 },
      { "index": 1, "count": 2, "percent": 14 },
      { "index": 2, "count": 3, "percent": 22 },
      { "index": 3, "count": 0, "percent": 0 }
    ],
    "top5": [
      { "rank": 1, "nickname": "Mario", "total_score": 425, "avatar_color": "#E85D24" },
      { "rank": 2, "nickname": "Luigi", "total_score": 380, "avatar_color": "#3B8BD4" }
    ],
    "revealed_at": "2025-04-20T10:16:30Z"
  }
}
```
Triggers WebSocket event `question_revealed` to all clients.

---

### POST /api/v1/sessions/:id/end
End the session (all questions done, or early termination).
Requires admin Bearer. Session must be `active`.

**Request** (optional)
```json
{ "reason": "early" }
```

**Response 200**
```json
{
  "data": {
    "session_id": "uuid",
    "status": "completed",
    "ended_at": "2025-04-20T10:45:00Z"
  }
}
```
Triggers WebSocket event `session_ended` to all clients.

---

### GET /api/v1/sessions/:id/leaderboard
Final leaderboard. Requires admin Bearer. Session must be `completed` or `cancelled`.

**Response 200**
```json
{
  "data": {
    "session_id": "uuid",
    "session_name": "Quiz aziendale Q2",
    "ended_at": "2025-04-20T10:45:00Z",
    "leaderboard": [
      {
        "rank": 1,
        "nickname": "Mario",
        "total_score": 1250,
        "avatar_color": "#E85D24",
        "correct_answers": 17,
        "total_questions": 20
      }
    ]
  }
}
```

**GET /api/v1/sessions/:id/leaderboard?format=csv**
Returns CSV file. `Content-Disposition: attachment; filename="<session-name>-leaderboard.csv"`.

---

### GET /api/v1/sessions/:id/stats
Per-question breakdown. Requires admin Bearer. Session must be `completed` or `cancelled`.

**Response 200**
```json
{
  "data": {
    "session_id": "uuid",
    "questions": [
      {
        "position": 1,
        "text": "In che anno è caduta Roma?",
        "difficulty": "medium",
        "correct_index": 0,
        "correct_count": 9,
        "wrong_count": 4,
        "no_answer_count": 1,
        "answer_distribution": [
          { "index": 0, "count": 9 },
          { "index": 1, "count": 2 },
          { "index": 2, "count": 1 },
          { "index": 3, "count": 1 }
        ],
        "avg_answer_time_ms": 8420
      }
    ]
  }
}
```

---

## Players (public — no admin auth)

### POST /api/v1/sessions/:id/join
Player joins a session. No auth required.

**Request**
```json
{
  "pin": "482910",
  "nickname": "Mario"
}
```

**Response 201**
```json
{
  "data": {
    "player_id": "uuid",
    "nickname": "Mario",
    "avatar_color": "#E85D24",
    "session_id": "uuid",
    "session_name": "Quiz aziendale Q2",
    "player_token": "<jwt>",
    "token_expires_at": "2025-04-20T14:00:00Z"
  }
}
```
`player_token` — JWT with `{ player_id, session_id, role: "player" }`, TTL 4h.
Used to authenticate the WebSocket connection.

**Response 404** — PIN not found or session not in `lobby` status.
**Response 409 CONFLICT** — nickname already taken in this session.
**Response 409 SESSION_NOT_IN_LOBBY** — session is active, completed, or draft.

---

### POST /api/v1/sessions/:session_id/answers
Submit an answer. Requires player Bearer (player token).

**Request**
```json
{
  "session_question_id": "uuid",
  "chosen_index": 2
}
```

**Response 201**
```json
{
  "data": {
    "answer_id": "uuid",
    "chosen_index": 2,
    "answered_at": "2025-04-20T10:15:08Z"
  }
}
```
Note: `is_correct` and `points_awarded` are **not** returned here — only at reveal.
If the player already answered this question: **200** (idempotent, returns existing answer — no double submission).
If timer has expired: **409** `{ "error": { "code": "QUESTION_CLOSED", "message": "Timer has expired" } }`.

---

## WebSocket

### Connection

**Admin / Presenter**
```
WS /ws/sessions/:id?token=<admin_jwt>
```

**Projection Screen**
```
WS /ws/sessions/:id?token=<projection_token>
```

**Player**
```
WS /ws/sessions/:id?token=<player_token>
```

The server reads the `token` query param, validates it, and assigns the connection to the correct role. All three roles connect to the same hub but receive different event subsets.

On successful connection, server sends:
```json
{
  "event": "connected",
  "data": {
    "session_id": "uuid",
    "role": "player",
    "player_id": "uuid"
  }
}
```

---

### Server → Client events

All events follow the envelope:
```json
{ "event": "<event_name>", "data": { ... } }
```

**`player_joined`** — broadcast to admin/presenter when a player connects to lobby.
```json
{
  "event": "player_joined",
  "data": {
    "player_id": "uuid",
    "nickname": "Mario",
    "avatar_color": "#E85D24",
    "total_players": 8
  }
}
```

**`player_disconnected`** — broadcast to admin/presenter.
```json
{
  "event": "player_disconnected",
  "data": { "player_id": "uuid", "nickname": "Mario", "total_players": 7 }
}
```

**`session_started`** — broadcast to all roles when session transitions `lobby → active`.
```json
{
  "event": "session_started",
  "data": { "session_id": "uuid", "total_questions": 20 }
}
```

**`question_started`** — broadcast to all roles. Triggered by `POST /next-question`.
```json
{
  "event": "question_started",
  "data": {
    "session_question_id": "uuid",
    "position": 3,
    "total": 20,
    "question": {
      "text": "In che anno è caduta Roma?",
      "option_a": "476 d.C.",
      "option_b": "410 d.C.",
      "option_c": "395 d.C.",
      "option_d": "455 d.C."
    },
    "time_limit_s": 30,
    "asked_at": "2025-04-20T10:15:00Z"
  }
}
```
`correct_index` never included — revealed only in `question_revealed`.

**`answer_count_updated`** — broadcast to admin/presenter only (not players, not projection).
```json
{
  "event": "answer_count_updated",
  "data": {
    "session_question_id": "uuid",
    "answered": 6,
    "total": 14
  }
}
```

**`timer_paused`** — broadcast to all roles.
```json
{
  "event": "timer_paused",
  "data": { "paused_at": "2025-04-20T10:16:05Z", "time_remaining_ms": 18500 }
}
```

**`timer_resumed`** — broadcast to all roles.
```json
{
  "event": "timer_resumed",
  "data": { "resumed_at": "2025-04-20T10:16:20Z", "time_remaining_ms": 18500 }
}
```

**`question_revealed`** — broadcast to all roles. Triggered by `POST /reveal`.
```json
{
  "event": "question_revealed",
  "data": {
    "session_question_id": "uuid",
    "correct_index": 0,
    "answer_distribution": [
      { "index": 0, "count": 9, "percent": 64 },
      { "index": 1, "count": 2, "percent": 14 },
      { "index": 2, "count": 3, "percent": 22 },
      { "index": 3, "count": 0, "percent": 0 }
    ],
    "top5": [
      { "rank": 1, "nickname": "Mario", "total_score": 425, "avatar_color": "#E85D24" }
    ]
  }
}
```

**`player_result`** — sent to a **specific player only** after reveal.
```json
{
  "event": "player_result",
  "data": {
    "chosen_index": 0,
    "is_correct": true,
    "points_awarded": 137,
    "total_score": 425,
    "rank": 1,
    "total_players": 14
  }
}
```

**`session_ended`** — broadcast to all roles.
```json
{
  "event": "session_ended",
  "data": {
    "session_id": "uuid",
    "reason": "completed",
    "leaderboard": [
      { "rank": 1, "nickname": "Mario", "total_score": 1250, "avatar_color": "#E85D24" }
    ]
  }
}
```
`reason`: `"completed"` (all questions done) or `"early"` (presenter ended manually).

---

### Client → Server messages

Only players send messages to the server via WebSocket.
Answer submission is handled via REST (`POST /answers`) — WebSocket is push-only for admin/presenter/projection.

Players may send a **heartbeat** ping to keep the connection alive:
```json
{ "event": "ping" }
```
Server responds:
```json
{ "event": "pong" }
```

---

## Endpoint summary

### Admin endpoints (require admin JWT)

| Method | Path | Description |
|---|---|---|
| POST | `/auth/login` | Login |
| POST | `/auth/refresh` | Refresh access token |
| POST | `/auth/logout` | Logout |
| GET | `/categories` | List categories |
| POST | `/categories` | Create category |
| PATCH | `/categories/:id` | Rename category |
| DELETE | `/categories/:id` | Delete category |
| GET | `/questions` | List questions (paginated + filtered) |
| POST | `/questions` | Create question |
| PATCH | `/questions/:id` | Update question |
| DELETE | `/questions/:id` | Delete question |
| POST | `/questions/import` | Bulk import from CSV |
| GET | `/questions/import/template` | Download CSV template |
| GET | `/sessions` | List sessions |
| POST | `/sessions` | Create session |
| GET | `/sessions/:id` | Session detail |
| PATCH | `/sessions/:id` | Update session (draft only) |
| DELETE | `/sessions/:id` | Delete session (draft only) |
| POST | `/sessions/:id/open-lobby` | Open lobby (draft → lobby) |
| GET | `/sessions/:id/qr` | QR code PNG |
| POST | `/sessions/:id/launch` | Launch session (lobby → active) |
| POST | `/sessions/:id/next-question` | Advance to next question |
| POST | `/sessions/:id/pause-timer` | Pause timer |
| POST | `/sessions/:id/resume-timer` | Resume timer |
| POST | `/sessions/:id/reveal` | Reveal answer + compute scores |
| POST | `/sessions/:id/end` | End session |
| GET | `/sessions/:id/leaderboard` | Final leaderboard (+ ?format=csv) |
| GET | `/sessions/:id/stats` | Per-question stats |

### Player endpoints (public or player JWT)

| Method | Path | Auth | Description |
|---|---|---|---|
| POST | `/sessions/:id/join` | None | Join session |
| POST | `/sessions/:session_id/answers` | Player JWT | Submit answer |

### WebSocket

| Path | Auth | Role |
|---|---|---|
| `/ws/sessions/:id?token=<jwt>` | Admin JWT | Presenter |
| `/ws/sessions/:id?token=<projection_token>` | Projection JWT | Projection screen |
| `/ws/sessions/:id?token=<player_token>` | Player JWT | Player |
