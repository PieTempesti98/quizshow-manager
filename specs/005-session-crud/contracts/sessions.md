# API Contracts: Sessions

**Feature**: 005-session-crud  
**Date**: 2026-05-04  
**Base path**: `/api/v1/sessions`  
**Auth**: All endpoints require `Authorization: Bearer <admin_jwt>`

---

## POST /api/v1/sessions

Create a new session in `draft` status.

### Request

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

| Field | Required | Validation |
|---|---|---|
| `name` | Yes | Non-empty string |
| `category_ids` | Yes | Array of UUIDs, at least one, all must reference active (non-deleted) categories |
| `question_count` | Yes | Integer 1–50 |
| `time_per_question_s` | Yes | One of: 10, 20, 30, 60 |
| `points_per_answer` | No | Positive integer; default 100 |
| `speed_bonus_enabled` | No | Boolean; default false |

### Response 201 — Created

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

**With warning** (when `available_questions < question_count`):

```json
{
  "data": {
    "id": "uuid",
    ...
    "available_questions": 15,
    "player_count": 0,
    "warning": "Only 15 questions available, session will use all of them",
    "created_at": "2025-04-20T09:00:00Z"
  }
}
```

### Response 422 — Validation Error

```json
{
  "error": {
    "code": "VALIDATION_ERROR",
    "message": "category_ids must contain at least one valid category"
  }
}
```

---

## GET /api/v1/sessions

List sessions with optional status filter and pagination.

### Query Parameters

| Param | Type | Default | Description |
|---|---|---|---|
| `status` | string | — | Comma-separated status values: `draft`, `lobby`, `active`, `completed`, `cancelled` |
| `page` | int | 1 | 1-based page number |
| `per_page` | int | 20 | Items per page |

### Response 200

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
    "pagination": {
      "page": 1,
      "per_page": 20,
      "total": 8,
      "total_pages": 1
    }
  }
}
```

---

## GET /api/v1/sessions/:id

Session detail including categories array.

### Response 200

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
    "speed_bonus_enabled": false,
    "player_count": 0,
    "categories": [
      { "id": "uuid", "name": "Storia" },
      { "id": "uuid", "name": "Scienza" }
    ],
    "started_at": null,
    "ended_at": null,
    "created_at": "2025-04-20T09:00:00Z"
  }
}
```

### Response 404

```json
{ "error": { "code": "NOT_FOUND", "message": "session not found" } }
```

---

## PATCH /api/v1/sessions/:id

Partial update of a draft session. Only allowed when `status = "draft"`.

### Request

Any subset of:

```json
{
  "name": "New name",
  "category_ids": ["uuid-1", "uuid-3"],
  "question_count": 15,
  "time_per_question_s": 20,
  "points_per_answer": 200,
  "speed_bonus_enabled": false
}
```

- All fields optional.
- If `category_ids` is present it must be a non-empty array of valid category UUIDs; providing an empty array is a validation error.
- When `category_ids` is updated, `available_questions` is recomputed against the new set.

### Response 200 — Updated session

Same shape as POST 201 response (includes `available_questions` and optional `warning`).

### Response 409 — Not Draft

```json
{ "error": { "code": "SESSION_NOT_DRAFT", "message": "session cannot be modified in its current status" } }
```

### Response 422 — Validation Error

```json
{ "error": { "code": "VALIDATION_ERROR", "message": "..." } }
```

---

## DELETE /api/v1/sessions/:id

Soft-delete a draft session. Only allowed when `status = "draft"`.

### Response 200

```json
{ "data": { "ok": true } }
```

### Response 409 — Not Draft

```json
{ "error": { "code": "SESSION_NOT_DRAFT", "message": "session cannot be deleted in its current status" } }
```

### Response 404

```json
{ "error": { "code": "NOT_FOUND", "message": "session not found" } }
```

---

## Notes

- `available_questions` is a computed value — not stored in the database. It is returned only on POST 201 and PATCH 200 responses.
- `warning` is an optional field present only when `available_questions < question_count`.
- `player_count` is derived at query time via COUNT from the `players` table.
- Null timestamps (`started_at`, `ended_at`) are omitted or returned as JSON `null`.
