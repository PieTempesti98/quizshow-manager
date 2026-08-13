# API Contracts: Feature #9 — Stats & Leaderboard

**Branch**: `009-stats-leaderboard` | **Date**: 2026-08-14 | **Spec**: [spec.md](../spec.md)

All endpoints require Admin authentication:
```http
Authorization: Bearer <admin_jwt>
```

---

## 1. GET /api/v1/sessions/:id/leaderboard

Retrieves the final leaderboard ranking of all participants in a completed or cancelled session.

### Request

- **Method**: `GET`
- **Path**: `/api/v1/sessions/:id/leaderboard`
- **Path Params**:
  - `id` (UUID, required): The ID of the session.
- **Query Params**:
  - `format` (string, optional): `"csv"` to export as a downloadable CSV file.

### Response 200 OK (Default / JSON)

- **Content-Type**: `application/json`

```json
{
  "data": {
    "session_id": "018f2d5e-7a42-78d1-9f23-8c4b12345678",
    "session_name": "Quiz Aziendale Q2",
    "ended_at": "2026-08-14T10:45:00Z",
    "leaderboard": [
      {
        "rank": 1,
        "player_id": "018f2d5e-7a42-78d1-9f23-8c4b12345679",
        "nickname": "Mario",
        "total_score": 1250,
        "avatar_color": "#E85D24",
        "correct_answers": 8,
        "total_questions": 10
      },
      {
        "rank": 2,
        "player_id": "018f2d5e-7a42-78d1-9f23-8c4b12345680",
        "nickname": "Luigi",
        "total_score": 980,
        "avatar_color": "#3B8BD4",
        "correct_answers": 6,
        "total_questions": 10
      }
    ]
  }
}
```

### Response 200 OK (`?format=csv`)

- **Content-Type**: `text/csv; charset=utf-8`
- **Content-Disposition**: `attachment; filename="quiz-aziendale-q2-2026-08-14-leaderboard.csv"`

```csv
rank,nickname,total_score
1,Mario,1250
2,Luigi,980
```

### Response 404 NOT_FOUND

```json
{
  "error": {
    "code": "NOT_FOUND",
    "message": "Session not found"
  }
}
```

### Response 409 SESSION_NOT_COMPLETED

Returned if the session is currently in `draft`, `lobby`, or `active` status:

```json
{
  "error": {
    "code": "SESSION_NOT_COMPLETED",
    "message": "Session is not completed or cancelled"
  }
}
```

---

## 2. GET /api/v1/sessions/:id/stats

Retrieves the aggregated per-question statistical breakdown for a completed or cancelled session.

### Request

- **Method**: `GET`
- **Path**: `/api/v1/sessions/:id/stats`
- **Path Params**:
  - `id` (UUID, required): The ID of the session.

### Response 200 OK

- **Content-Type**: `application/json`

```json
{
  "data": {
    "session_id": "018f2d5e-7a42-78d1-9f23-8c4b12345678",
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
          { "index": 0, "count": 9, "percent": 64 },
          { "index": 1, "count": 2, "percent": 14 },
          { "index": 2, "count": 1, "percent": 7 },
          { "index": 3, "count": 1, "percent": 7 }
        ],
        "avg_answer_time_ms": 8420
      },
      {
        "position": 2,
        "text": "Qual è il pianeta più grande del sistema solare?",
        "difficulty": "easy",
        "correct_index": 1,
        "correct_count": 14,
        "wrong_count": 0,
        "no_answer_count": 0,
        "answer_distribution": [
          { "index": 0, "count": 0, "percent": 0 },
          { "index": 1, "count": 14, "percent": 100 },
          { "index": 2, "count": 0, "percent": 0 },
          { "index": 3, "count": 0, "percent": 0 }
        ],
        "avg_answer_time_ms": 4250
      }
    ]
  }
}
```

### Response 404 NOT_FOUND

```json
{
  "error": {
    "code": "NOT_FOUND",
    "message": "Session not found"
  }
}
```

### Response 409 SESSION_NOT_COMPLETED

```json
{
  "error": {
    "code": "SESSION_NOT_COMPLETED",
    "message": "Session is not completed or cancelled"
  }
}
```

---

## 3. GET /api/v1/sessions?status=completed,cancelled

Session history query listing past completed or cancelled sessions.

### Request

- **Method**: `GET`
- **Path**: `/api/v1/sessions`
- **Query Params**:
  - `status`: `completed,cancelled`
  - `page`: 1
  - `per_page`: 20

### Response 200 OK

```json
{
  "data": {
    "sessions": [
      {
        "id": "018f2d5e-7a42-78d1-9f23-8c4b12345678",
        "name": "Quiz Aziendale Q2",
        "pin": "482910",
        "status": "completed",
        "question_count": 10,
        "time_per_question_s": 30,
        "points_per_answer": 100,
        "speed_bonus_enabled": true,
        "player_count": 14,
        "started_at": "2026-08-14T10:00:00Z",
        "ended_at": "2026-08-14T10:45:00Z",
        "created_at": "2026-08-14T09:00:00Z"
      }
    ],
    "pagination": {
      "page": 1,
      "per_page": 20,
      "total": 1,
      "total_pages": 1
    }
  }
}
```
