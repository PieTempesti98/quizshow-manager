# Quickstart: Session Create and Configure

**Feature**: 005-session-crud  
**Date**: 2026-05-04

## Prerequisites

- Backend running (Docker Compose or `go run ./cmd/server`)
- At least one category and some questions in the database
- Valid admin JWT (obtain via `POST /api/v1/auth/login`)

## Smoke Test — Happy Path

### 1. Create a session

```bash
curl -s -X POST http://localhost:3000/api/v1/sessions \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "Test Quiz",
    "category_ids": ["<category-uuid>"],
    "question_count": 5,
    "time_per_question_s": 30,
    "points_per_answer": 100,
    "speed_bonus_enabled": false
  }' | jq .
```

Expected: `201` with `status: "draft"`, a 6-digit `pin`, and `available_questions`.

### 2. List sessions

```bash
curl -s http://localhost:3000/api/v1/sessions \
  -H "Authorization: Bearer $TOKEN" | jq .
```

Expected: `200` with the session in the `sessions` array and correct pagination.

### 3. Get session detail

```bash
curl -s http://localhost:3000/api/v1/sessions/<session-id> \
  -H "Authorization: Bearer $TOKEN" | jq .
```

Expected: `200` with a `categories` array populated.

### 4. Update the session

```bash
curl -s -X PATCH http://localhost:3000/api/v1/sessions/<session-id> \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"question_count": 3}' | jq .
```

Expected: `200` with `question_count: 3`.

### 5. Delete the session

```bash
curl -s -X DELETE http://localhost:3000/api/v1/sessions/<session-id> \
  -H "Authorization: Bearer $TOKEN" | jq .
```

Expected: `200` with `{ "data": { "ok": true } }`.

## Edge Case Tests

### Warning on insufficient questions

Create a session with `question_count` higher than available questions in the selected categories. Expect `201` response that includes a `warning` field.

### PATCH on non-draft session

Manually set a session's status to `active` in the DB, then attempt PATCH. Expect `409` with `SESSION_NOT_DRAFT`.

### Delete on non-draft session

Same as above for DELETE.

### Filter by status

```bash
curl -s "http://localhost:3000/api/v1/sessions?status=draft,active" \
  -H "Authorization: Bearer $TOKEN" | jq .
```

Expected: only `draft` and `active` sessions returned.
