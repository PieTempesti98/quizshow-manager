# Data Model: Categories CRUD (002-categories-crud)

## Existing Schema (no migration required)

The `categories` table and all required indexes are already defined in `backend/migrations/001_initial_schema.up.sql`.

```sql
CREATE TABLE categories (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name        TEXT NOT NULL,
    slug        TEXT NOT NULL,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    deleted_at  TIMESTAMPTZ
);

-- Uniqueness enforced on non-deleted rows only
CREATE UNIQUE INDEX categories_name_unique
    ON categories (name)
    WHERE deleted_at IS NULL;

CREATE UNIQUE INDEX categories_slug_unique
    ON categories (slug)
    WHERE deleted_at IS NULL;
```

The `categories_updated_at` trigger is also already in place.

The `questions` table (relevant for `question_count` and delete-blocking) is also present:
```sql
-- Relevant column:
-- category_id UUID NOT NULL REFERENCES categories(id)
-- deleted_at  TIMESTAMPTZ
```

---

## Go Model (`internal/category/models.go`)

```go
type Category struct {
    ID            uuid.UUID
    Name          string
    Slug          string
    CreatedAt     time.Time
    UpdatedAt     time.Time
    DeletedAt     *time.Time
}

// CategoryWithCount is Category + the derived question_count field.
type CategoryWithCount struct {
    Category
    QuestionCount int
}
```

---

## Validation Rules

| Field  | Rule                                             | Error code        |
|--------|--------------------------------------------------|-------------------|
| `name` | Required, non-empty                              | `VALIDATION_ERROR` |
| `name` | Max 50 characters                                | `VALIDATION_ERROR` |
| `name` | Unique among non-deleted rows (partial index)    | `CONFLICT`         |
| `slug` | Auto-generated from name; unique (partial index) | `CONFLICT`         |
| `id`   | Must be a valid UUID v4                          | `VALIDATION_ERROR` |

---

## Slug Generation

```
slug = lowercase(name)
slug = replace(/[\s]+/g, "-")        // spaces → single hyphen
slug = replace(/[^a-z0-9\-]/g, "")  // strip non-alphanumeric non-hyphen
slug = replace(/-{2,}/g, "-")       // collapse consecutive hyphens
slug = trim("-")                    // strip leading/trailing hyphens
```

Implemented as a pure function `slugify(name string) string` in `internal/category/models.go`.

---

## Key Queries

### List categories (with question_count)
```sql
SELECT c.id, c.name, c.slug, c.created_at, c.updated_at,
       COUNT(q.id) AS question_count
FROM categories c
LEFT JOIN questions q
       ON q.category_id = c.id AND q.deleted_at IS NULL
WHERE c.deleted_at IS NULL
GROUP BY c.id
ORDER BY c.name ASC
```

### Find by ID (for rename / delete)
```sql
SELECT id, name, slug, created_at, updated_at, deleted_at
FROM categories
WHERE id = $1 AND deleted_at IS NULL
```

### Create
```sql
INSERT INTO categories (name, slug)
VALUES ($1, $2)
RETURNING id, name, slug, created_at, updated_at
```

### Rename (update name + slug + updated_at)
```sql
UPDATE categories
SET name = $1, slug = $2, updated_at = now()
WHERE id = $3 AND deleted_at IS NULL
RETURNING id, name, slug, created_at, updated_at
```

### Count active questions in a category (delete-blocking check)
```sql
SELECT COUNT(*) FROM questions
WHERE category_id = $1 AND deleted_at IS NULL
```

### Soft delete
```sql
UPDATE categories
SET deleted_at = now(), updated_at = now()
WHERE id = $1 AND deleted_at IS NULL
```

---

## State Transitions

Categories are not stateful beyond active / soft-deleted:

```
[active] --soft-delete--> [deleted]
```

There is no restore/undelete operation in MVP.

---

## Conflict Detection

Both `name` and `slug` conflicts are signalled by PostgreSQL raising a unique constraint violation (`pgconn.PgError` with `Code == "23505"`). The repository layer catches this error and returns a sentinel error (`ErrCategoryNameConflict`) that the service and handler translate to HTTP 409.
