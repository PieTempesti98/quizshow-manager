# REST API Contracts: Questions CRUD

**Feature**: 003-questions-crud | **Date**: 2026-04-24
**Base path**: `/api/v1/questions`
**Auth**: All endpoints require `Authorization: Bearer <admin_jwt>`

---

## GET /api/v1/questions

Paginated list of non-deleted questions with optional filters.

### Query Parameters

| Param | Type | Default | Constraint | Description |
|-------|------|---------|------------|-------------|
| `page` | int | `1` | ≥ 1 | Page number (1-based) |
| `per_page` | int | `25` | 1–100 (clamped) | Items per page |
| `category_id` | string | — | Comma-separated UUIDs | Filter by category (union) |
| `difficulty` | string | — | Comma-separated: `easy`, `medium`, `hard` | Filter by difficulty (union) |
| `q` | string | — | Any string | Case-insensitive substring search on question text |

### Response 200

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

Empty list (no matches):
```json
{ "data": { "questions": [], "pagination": { "page": 1, "per_page": 25, "total": 0, "total_pages": 0 } } }
```

### Error Responses

| Status | Code | Condition |
|--------|------|-----------|
| 401 | `UNAUTHORIZED` | Missing or invalid admin JWT |

---

## POST /api/v1/questions

Create a new question.

### Request Body

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

- `difficulty` is optional; defaults to `"medium"` when absent.
- All other fields are required.

### Response 201

Full question object (same shape as list item):
```json
{
  "data": {
    "id": "uuid",
    "category": { "id": "uuid", "name": "Storia" },
    "text": "In che anno è caduta Roma?",
    "option_a": "476 d.C.",
    "option_b": "410 d.C.",
    "option_c": "395 d.C.",
    "option_d": "455 d.C.",
    "correct_index": 0,
    "difficulty": "medium",
    "created_at": "2026-04-24T10:00:00Z"
  }
}
```

### Error Responses

| Status | Code | Condition |
|--------|------|-----------|
| 401 | `UNAUTHORIZED` | Missing or invalid admin JWT |
| 422 | `VALIDATION_ERROR` | Any required field missing or invalid |
| 422 | `VALIDATION_ERROR` | `correct_index` not in 0–3 |
| 422 | `VALIDATION_ERROR` | `difficulty` not in `easy`/`medium`/`hard` |
| 422 | `VALIDATION_ERROR` | `category_id` not a valid UUID or not found |

---

## PATCH /api/v1/questions/:id

Partial update. Only fields present in the request body are updated.

### Request Body

Any subset of question fields (all optional in PATCH):
```json
{
  "text": "Updated question text",
  "correct_index": 1
}
```

Empty body → no-op, returns unchanged question.

### Response 200

Updated question object (same shape as POST 201 response):
```json
{ "data": { ...question object... } }
```

### Error Responses

| Status | Code | Condition |
|--------|------|-----------|
| 401 | `UNAUTHORIZED` | Missing or invalid admin JWT |
| 404 | `NOT_FOUND` | Question does not exist or is soft-deleted |
| 409 | `QUESTION_IN_USE` | Question is referenced in a session with status `active` |
| 422 | `VALIDATION_ERROR` | `:id` is not a valid UUID |
| 422 | `VALIDATION_ERROR` | Any supplied field fails validation |

---

## DELETE /api/v1/questions/:id

Soft-delete a question. Sets `deleted_at`; question is removed from all list responses but session history is preserved.

### Response 200

```json
{ "data": { "ok": true } }
```

### Error Responses

| Status | Code | Condition |
|--------|------|-----------|
| 401 | `UNAUTHORIZED` | Missing or invalid admin JWT |
| 404 | `NOT_FOUND` | Question does not exist or is already soft-deleted |
| 409 | `QUESTION_IN_USE` | Question is referenced in a session with status `active` |
| 422 | `VALIDATION_ERROR` | `:id` is not a valid UUID |

---

## Shared Error Response Shape

All errors follow the standard envelope:
```json
{
  "error": {
    "code": "MACHINE_READABLE_CODE",
    "message": "Human-readable description"
  }
}
```

Example — `QUESTION_IN_USE`:
```json
{
  "error": {
    "code": "QUESTION_IN_USE",
    "message": "Cannot modify a question used in an active session"
  }
}
```
