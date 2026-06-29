# API Contracts: Categories (002-categories-crud)

All endpoints require `Authorization: Bearer <admin_jwt>`.  
All responses use the envelope: `{ "data": ... }` on success, `{ "error": { "code": "...", "message": "..." } }` on failure.

---

## GET /api/v1/categories

List all non-deleted categories with question counts.

**Auth**: Admin Bearer  
**Request body**: none

**Response 200**
```json
{
  "data": {
    "categories": [
      {
        "id": "550e8400-e29b-41d4-a716-446655440000",
        "name": "Storia",
        "slug": "storia",
        "question_count": 42,
        "created_at": "2026-04-01T09:00:00Z"
      }
    ]
  }
}
```
- `question_count`: count of non-deleted questions in this category.
- Sorted by `name` ascending.
- No pagination (full list).

**Response 401** — missing or invalid token
```json
{ "error": { "code": "UNAUTHORIZED", "message": "Missing or invalid token" } }
```

---

## POST /api/v1/categories

Create a new category.

**Auth**: Admin Bearer  
**Request body**
```json
{ "name": "Scienza" }
```

| Field  | Type   | Required | Constraints             |
|--------|--------|----------|-------------------------|
| `name` | string | yes      | Non-empty, max 50 chars |

**Response 201**
```json
{
  "data": {
    "id": "550e8400-e29b-41d4-a716-446655440001",
    "name": "Scienza",
    "slug": "scienza",
    "question_count": 0,
    "created_at": "2026-04-20T10:00:00Z"
  }
}
```

**Response 409** — name already used by a non-deleted category
```json
{ "error": { "code": "CONFLICT", "message": "A category with this name already exists" } }
```

**Response 422** — validation failure (empty name, name > 50 chars, missing field)
```json
{ "error": { "code": "VALIDATION_ERROR", "message": "name: must not be empty" } }
```

---

## PATCH /api/v1/categories/:id

Rename an existing category. Regenerates slug from the new name.

**Auth**: Admin Bearer  
**Path param**: `id` — UUID v4 of the category  
**Request body**
```json
{ "name": "Scienze naturali" }
```

| Field  | Type   | Required | Constraints             |
|--------|--------|----------|-------------------------|
| `name` | string | yes      | Non-empty, max 50 chars |

**Response 200**
```json
{
  "data": {
    "id": "550e8400-e29b-41d4-a716-446655440001",
    "name": "Scienze naturali",
    "slug": "scienze-naturali",
    "question_count": 7,
    "created_at": "2026-04-20T10:00:00Z"
  }
}
```
Note: the rename response includes `question_count` for UI convenience (same shape as list item).

**Response 404** — category does not exist or is soft-deleted
```json
{ "error": { "code": "NOT_FOUND", "message": "Category not found" } }
```

**Response 409** — new name conflicts with an existing non-deleted category
```json
{ "error": { "code": "CONFLICT", "message": "A category with this name already exists" } }
```

**Response 422** — validation failure
```json
{ "error": { "code": "VALIDATION_ERROR", "message": "name: must not be empty" } }
```

---

## DELETE /api/v1/categories/:id

Soft-delete a category. Blocked if the category has active (non-deleted) questions.

**Auth**: Admin Bearer  
**Path param**: `id` — UUID v4 of the category  
**Request body**: none

**Response 200**
```json
{ "data": { "ok": true } }
```

**Response 404** — category does not exist or is already soft-deleted
```json
{ "error": { "code": "NOT_FOUND", "message": "Category not found" } }
```

**Response 409** — category has active questions
```json
{
  "error": {
    "code": "CATEGORY_HAS_QUESTIONS",
    "message": "Cannot delete category with 12 active questions"
  }
}
```
The count in the message is the exact count of non-deleted questions in the category at the time of the request.

---

## Error Code Reference (this feature)

| Code                    | HTTP | Trigger                                           |
|-------------------------|------|---------------------------------------------------|
| `UNAUTHORIZED`          | 401  | Missing or invalid admin JWT                      |
| `FORBIDDEN`             | 403  | JWT valid but role is not `admin`                 |
| `NOT_FOUND`             | 404  | Category ID not found or already soft-deleted     |
| `CONFLICT`              | 409  | Duplicate category name (or slug)                 |
| `CATEGORY_HAS_QUESTIONS`| 409  | Delete attempted on category with active questions |
| `VALIDATION_ERROR`      | 422  | Name empty, too long, body missing, invalid UUID  |
