# Data Model: Questions CRUD

**Feature**: 003-questions-crud | **Date**: 2026-04-24

No new database migration is required. The `questions`, `categories`, `session_questions`, and `sessions` tables are fully defined in `backend/migrations/001_initial_schema.up.sql`.

---

## Existing Schema Reference (relevant tables)

### `questions` table

```sql
CREATE TABLE questions (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    category_id     UUID NOT NULL REFERENCES categories(id),
    text            TEXT NOT NULL,
    option_a        TEXT NOT NULL,
    option_b        TEXT NOT NULL,
    option_c        TEXT NOT NULL,
    option_d        TEXT NOT NULL,
    correct_index   SMALLINT NOT NULL CHECK (correct_index BETWEEN 0 AND 3),
    difficulty      difficulty_level NOT NULL DEFAULT 'medium',
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    deleted_at      TIMESTAMPTZ
);

-- difficulty_level ENUM: 'easy' | 'medium' | 'hard'
```

Existing index: `questions_category_id_idx` on `(category_id) WHERE deleted_at IS NULL`.

### `session_questions` table (read-only in this feature)

```sql
CREATE TABLE session_questions (
    id          UUID PRIMARY KEY,
    session_id  UUID NOT NULL REFERENCES sessions(id),
    question_id UUID NOT NULL REFERENCES questions(id),
    position    SMALLINT NOT NULL,
    ...
);
```

Used only for the active-session guard check. Not written by this feature.

---

## Go Package Model: `internal/question/`

### Core types (`models.go`)

```go
// Question is the full internal representation of a question row,
// including the joined category name.
type Question struct {
    ID           uuid.UUID
    CategoryID   uuid.UUID
    CategoryName string     // joined from categories.name
    Text         string
    OptionA      string
    OptionB      string
    OptionC      string
    OptionD      string
    CorrectIndex int        // 0–3
    Difficulty   string     // "easy" | "medium" | "hard"
    CreatedAt    time.Time
    UpdatedAt    time.Time
}

// CategoryRef is the nested category object included in API responses.
type CategoryRef struct {
    ID   uuid.UUID
    Name string
}

// QuestionFilter holds the optional query parameters for the list endpoint.
type QuestionFilter struct {
    CategoryIDs  []uuid.UUID // empty = no filter
    Difficulties []string    // empty = no filter
    Search       string      // empty = no filter
    Page         int         // 1-based, minimum 1
    PerPage      int         // 1–100, default 25
}

// QuestionListResult is returned by the list operation.
type QuestionListResult struct {
    Questions  []Question
    Total      int
    Page       int
    PerPage    int
    TotalPages int
}

// QuestionUpdate carries the fields for a partial update (PATCH).
// A nil pointer means the field is absent from the request — do not update.
type QuestionUpdate struct {
    CategoryID   *uuid.UUID
    Text         *string
    OptionA      *string
    OptionB      *string
    OptionC      *string
    OptionD      *string
    CorrectIndex *int
    Difficulty   *string
}
```

### Sentinel errors (`models.go`)

```go
var ErrQuestionNotFound = errors.New("question not found")
var ErrQuestionInUse    = errors.New("question is in use by an active session")
```

---

## Validation Rules

| Field | Create | Update (PATCH) |
|-------|--------|----------------|
| `text` | Required, non-empty | Optional; if present, must be non-empty |
| `option_a/b/c/d` | All required, all non-empty | Optional; if present, must be non-empty |
| `correct_index` | Required, integer 0–3 | Optional; if present, must be 0–3 |
| `category_id` | Required, must reference active category | Optional; if present, must reference active category |
| `difficulty` | Optional; defaults to `"medium"`; if present must be `easy`/`medium`/`hard` | Optional; if present must be `easy`/`medium`/`hard` |

Validation is performed in the handler before the service is called, consistent with the category handler pattern.

---

## API Response Shape

### Question object (in all responses)

```json
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
```

### List response envelope

```json
{
  "data": {
    "questions": [ { ...question object... } ],
    "pagination": {
      "page": 1,
      "per_page": 25,
      "total": 142,
      "total_pages": 6
    }
  }
}
```

### Create / Update response envelope

```json
{ "data": { ...question object... } }
```

### Delete response envelope

```json
{ "data": { "ok": true } }
```

### Error responses

| Scenario | HTTP | `code` |
|---|---|---|
| Missing/invalid token | 401 | `UNAUTHORIZED` |
| Non-UUID path param | 422 | `VALIDATION_ERROR` |
| Missing required field on create | 422 | `VALIDATION_ERROR` |
| Invalid `correct_index` | 422 | `VALIDATION_ERROR` |
| Invalid `difficulty` | 422 | `VALIDATION_ERROR` |
| Question not found | 404 | `NOT_FOUND` |
| Question used in active session | 409 | `QUESTION_IN_USE` |
| Internal error | 500 | `INTERNAL_ERROR` |

---

## State Transitions

Questions have no internal state machine. Their lifecycle is:

```
created → (active, visible in list)
        → soft-deleted (deleted_at set, invisible in list; session_questions snapshot preserved)
```

The only guard on mutation is the active-session check:
- `PATCH /questions/:id` and `DELETE /questions/:id` are blocked when `EXISTS(session_questions WHERE question_id = :id AND sessions.status = 'active')`.
