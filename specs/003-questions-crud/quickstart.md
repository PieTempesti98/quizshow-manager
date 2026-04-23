# Quickstart: Questions CRUD

**Feature**: 003-questions-crud | **Date**: 2026-04-24

---

## Prerequisites

- Docker Compose running: `docker compose up -d` (starts Postgres + Go server)
- Admin JWT in hand: call `POST /api/v1/auth/login` and extract `data.access_token`
- At least one active category created (`POST /api/v1/categories`)

---

## Local Development

### Run the server

```sh
cd backend
go run ./cmd/server
```

Or via Docker:
```sh
docker compose up --build
```

Server listens on `PORT` env var (default `3000`).

### Run tests

```sh
cd backend
go test ./internal/question/...
```

---

## Manual Smoke Test (curl)

Substitute `<TOKEN>` with your admin JWT and `<CATEGORY_ID>` with a valid category UUID.

### 1. Create a question

```sh
curl -s -X POST http://localhost:3000/api/v1/questions \
  -H "Authorization: Bearer <TOKEN>" \
  -H "Content-Type: application/json" \
  -d '{
    "category_id": "<CATEGORY_ID>",
    "text": "In che anno è caduta Roma?",
    "option_a": "476 d.C.",
    "option_b": "410 d.C.",
    "option_c": "395 d.C.",
    "option_d": "455 d.C.",
    "correct_index": 0
  }' | jq .
# → HTTP 201, full question object with difficulty: "medium" (default)
```

### 2. List questions

```sh
curl -s http://localhost:3000/api/v1/questions \
  -H "Authorization: Bearer <TOKEN>" | jq .
# → HTTP 200, questions array + pagination
```

### 3. Filter by category and difficulty

```sh
curl -s "http://localhost:3000/api/v1/questions?category_id=<CATEGORY_ID>&difficulty=medium,hard&q=Roma" \
  -H "Authorization: Bearer <TOKEN>" | jq .
```

### 4. Partial update (PATCH)

```sh
curl -s -X PATCH http://localhost:3000/api/v1/questions/<QUESTION_ID> \
  -H "Authorization: Bearer <TOKEN>" \
  -H "Content-Type: application/json" \
  -d '{"text": "Quando cadde Roma?", "difficulty": "hard"}' | jq .
# → HTTP 200, only text and difficulty changed
```

### 5. Delete a question

```sh
curl -s -X DELETE http://localhost:3000/api/v1/questions/<QUESTION_ID> \
  -H "Authorization: Bearer <TOKEN>" | jq .
# → HTTP 200, {"data": {"ok": true}}
```

### 6. Verify deletion

```sh
curl -s http://localhost:3000/api/v1/questions \
  -H "Authorization: Bearer <TOKEN>" | jq '.data.questions | length'
# → one fewer than before
```

---

## Key Behaviours to Verify

| Scenario | Expected |
|---|---|
| Create without `difficulty` | Defaults to `"medium"` |
| Create with `correct_index: 5` | HTTP 422 `VALIDATION_ERROR` |
| Create with empty `option_a` | HTTP 422 `VALIDATION_ERROR` |
| PATCH only `text` | Only text changes; all other fields unchanged |
| DELETE question in active session | HTTP 409 `QUESTION_IN_USE` |
| GET without auth header | HTTP 401 `UNAUTHORIZED` |
| `per_page=200` | Clamped to 100; 100 results returned |

---

## Package Wiring Reference

```go
// cmd/server/main.go additions
questionRepo    := question.NewRepository(pool)
questionSvc     := question.NewService(questionRepo)
questionHandler := question.NewHandler(questionSvc)

protected.Get("/questions",       questionHandler.List)
protected.Post("/questions",      questionHandler.Create)
protected.Patch("/questions/:id", questionHandler.Update)
protected.Delete("/questions/:id",questionHandler.Delete)
```
