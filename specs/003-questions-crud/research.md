# Research: Questions CRUD

**Feature**: 003-questions-crud | **Date**: 2026-04-24

No "NEEDS CLARIFICATION" unknowns exist in the technical context — all technology choices are determined by the existing codebase. This research document records three non-obvious implementation decisions and validates the chosen approaches against the existing category package pattern.

---

## Decision 1: Partial Update (PATCH) with pgx

**Problem**: Go's `encoding/json` cannot distinguish between an absent JSON field and a field explicitly set to the zero value. A plain struct like `type Req struct { Text string }` will unmarshal a missing `text` field as an empty string — indistinguishable from the caller deliberately blanking it.

**Decision**: Use pointer fields in the PATCH request struct (`*string`, `*int`). A nil pointer means the field was absent; a non-nil pointer means the caller supplied a value. The repository function accepts a `QuestionUpdate` struct with all pointer fields and builds the SQL SET clause dynamically, skipping nil fields.

```go
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

The repository iterates the non-nil fields to build a slice of `SET col = $N` clauses and a matching args slice, then issues `UPDATE questions SET ... WHERE id = $N AND deleted_at IS NULL RETURNING ...`.

**Rationale**: Pointer-field approach is idiomatic Go, requires no extra dependencies, and is the same pattern used in the Go standard library's `database/sql` optional scanning. It keeps the repository function signature explicit and type-safe.

**Alternatives considered**:
- `map[string]any` from raw JSON body: flexible but loses type safety and complicates validation.
- Separate endpoints per field: overly verbose; conflicts with the API design doc specifying a single PATCH endpoint.

---

## Decision 2: Active-Session Guard (QUESTION_IN_USE check)

**Problem**: A question must not be edited or deleted if it is referenced in a session currently in `active` status. The check must be fast and correct — a race between the check and the mutation is acceptable at MVP scale (single admin, low concurrency).

**Decision**: A single `EXISTS` query against `session_questions` joined to `sessions`:

```sql
SELECT EXISTS (
    SELECT 1
    FROM session_questions sq
    JOIN sessions s ON s.id = sq.session_id
    WHERE sq.question_id = $1
      AND s.status = 'active'
      AND s.deleted_at IS NULL
)
```

This is called by the service before any update or delete. If true, the service returns `ErrQuestionInUse`. The handler maps this to HTTP 409 with code `QUESTION_IN_USE`.

**Rationale**: Single round-trip; database enforces the join condition. No locking needed at MVP scale. The `session_questions` table has an index on `session_id`; the join to `sessions` on `id` (PK) is always fast. No transaction wrapper needed because the window between the check and the write is negligible at single-admin MVP scale.

**Alternatives considered**:
- SELECT + lock: overkill for MVP single-admin scenario.
- Check at the DB constraint level: not expressible as a declarative constraint in Postgres without a trigger.

---

## Decision 3: Paginated List with Filters (pgx dynamic query)

**Problem**: The list endpoint accepts multiple optional filters (category_id[], difficulty[], free-text search), must return pagination metadata, and must sort by `created_at DESC`. Building a safe parameterized query with a variable number of WHERE clauses requires care.

**Decision**: Build the WHERE clause dynamically in Go, appending clauses and args to slices as filters are present. Use a separate `COUNT(*)` query (not `COUNT(*) OVER()`) to get the total — two queries, both fast with indexes.

Pattern:
```go
clauses := []string{"q.deleted_at IS NULL"}
args := []any{}
n := 1

if len(filter.CategoryIDs) > 0 {
    clauses = append(clauses, fmt.Sprintf("q.category_id = ANY($%d)", n))
    args = append(args, filter.CategoryIDs)
    n++
}
if len(filter.Difficulties) > 0 {
    clauses = append(clauses, fmt.Sprintf("q.difficulty = ANY($%d)", n))
    args = append(args, filter.Difficulties)
    n++
}
if filter.Search != "" {
    clauses = append(clauses, fmt.Sprintf("q.text ILIKE $%d", n))
    args = append(args, "%"+filter.Search+"%")
    n++
}
```

The category JOIN (`LEFT JOIN categories c ON c.id = q.category_id AND c.deleted_at IS NULL`) is always present so the response always includes `category: {id, name}`.

**Rationale**: This approach is the standard pgx pattern for optional filters. Using `ANY($1)` with a `[]uuid.UUID` or `[]string` slice is natively supported by pgx and avoids SQL injection. Two queries (count + fetch) is simpler than `COUNT(*) OVER()` which changes the query shape significantly and is harder to reason about for pagination correctness.

**Alternatives considered**:
- `COUNT(*) OVER()` window function: one query, but more complex and returns the total on every row requiring a scan of the first row before building the pagination struct.
- Query builder library (sqlc, squirrel): adds a dependency; YAGNI per Constitution Principle III.

---

## Decision 4: Difficulty Validation

**Problem**: `difficulty` is a PostgreSQL ENUM (`difficulty_level`: easy/medium/hard). pgx will reject an invalid value at the database level, but the error message from Postgres is not user-friendly.

**Decision**: Validate `difficulty` in the handler before hitting the database. Accept `"easy"`, `"medium"`, `"hard"` only. Return HTTP 422 `VALIDATION_ERROR` for any other value. Default to `"medium"` on create when the field is absent.

**Rationale**: Consistent with how the category handler validates `name` before calling the service. Keeps error messages user-friendly rather than exposing raw Postgres error text.

---

## Summary of All Decisions

| # | Topic | Decision |
|---|-------|----------|
| 1 | PATCH partial update | Pointer-field `QuestionUpdate` struct; dynamic SET clause in repository |
| 2 | Active-session guard | EXISTS query on session_questions JOIN sessions WHERE status='active' |
| 3 | Paginated filtered list | Dynamic WHERE slice + `ANY($N)` + separate COUNT query |
| 4 | Difficulty validation | Validate in handler; default to "medium" on absent field |
