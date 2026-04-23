# Research: Categories CRUD (002-categories-crud)

## Decision 1: Slug Generation Strategy

**Decision**: Simple in-process string transformation — lowercase name, replace one-or-more whitespace characters and hyphens with a single hyphen, strip any characters that are not lowercase ASCII letters, digits, or hyphens.

**Rationale**: The MVP operates on Italian-language category names that are ASCII-safe. `golang.org/x/text` is already present in `go.mod` (pulled transitively), but using `unicode.ToLower` + a regexp is sufficient and avoids an explicit dependency on the `transform` and `norm` sub-packages. The partial unique index `categories_slug_unique` (non-deleted rows only) enforces uniqueness at the database level; the service layer surfaces a `CONFLICT` error if the generated slug collides.

**Alternatives considered**:
- `golang.org/x/text/transform` + ICU-style transliteration: handles accented characters (è → e, ò → o) correctly but adds complexity for a first-cut MVP. Can be added as a transparent improvement later.
- Pre-generating a random suffix on collision: rejected — slug should remain deterministic from the name.

---

## Decision 2: Package Layout — `internal/category`

**Decision**: New Go package at `backend/internal/category/` with four files: `models.go`, `repository.go`, `service.go`, `handler.go`.

**Rationale**: Mirrors the existing `internal/auth/` pattern exactly (per spec requirement). The package name `category` (singular) is idiomatic Go. Using `internal/question/` for category code would be misleading; `internal/question/` will be the future home for question CRUD.

**Alternatives considered**:
- `internal/categories/` (plural): non-idiomatic for Go package names; Go convention is singular.
- Merging category and question into a single `internal/content/` package: premature — questions are a separate, more complex feature.

---

## Decision 3: `question_count` Computation

**Decision**: Computed at query time via a `LEFT JOIN` + `COUNT` in the `LIST` query. The `CREATE` response uses a hard-coded `0` (newly created categories cannot have questions).

**Rationale**: Constitution Principle III prohibits premature complexity. A denormalized counter column would require trigger maintenance or application-level bookkeeping. At MVP scale (tens of categories, hundreds to low thousands of questions), computing the count in the same query is negligible.

**SQL pattern** (used in repository `List`):
```sql
SELECT c.id, c.name, c.slug, c.created_at,
       COUNT(q.id) AS question_count
FROM categories c
LEFT JOIN questions q
       ON q.category_id = c.id AND q.deleted_at IS NULL
WHERE c.deleted_at IS NULL
GROUP BY c.id
ORDER BY c.name
```

---

## Decision 4: Database Migration

**Decision**: No new migration required. The `categories` table (with `categories_name_unique` and `categories_slug_unique` partial indexes) is already defined in `backend/migrations/001_initial_schema.up.sql`. The `questions` table with `category_id FK` is also present.

**Rationale**: All required schema is in place from the initial migration authored as part of the project scaffolding.

---

## Decision 5: Error Sentinel Pattern

**Decision**: Follow the same pattern as `internal/auth/service.go` — define package-level sentinel errors (`ErrCategoryNotFound`, `ErrCategoryNameConflict`, `ErrCategoryHasQuestions`) in `service.go`. Handler maps these to HTTP status codes.

**Rationale**: Consistent with existing code. Sentinel errors allow `errors.Is()` checks in tests and callers without depending on error message strings.

---

## Decision 6: Route Registration

**Decision**: All four category routes are registered on the existing `protected` group in `cmd/server/main.go` (already guarded by `auth.RequireAdmin(cfg)`). No new middleware required.

**Rationale**: `RequireAdmin` is already the correct guard. Adding category routes under the same `protected` group is the minimal change; no new Fiber group needed.
