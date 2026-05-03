# Research: Questions CSV Import

**Feature**: 004-questions-csv-import  
**Date**: 2026-05-03

## Decision 1: CSV Parsing Library

**Decision**: Use Go's standard library `encoding/csv`.

**Rationale**: Zero new dependencies. Handles RFC 4180 CSV (quoted fields, embedded commas, embedded newlines). Already available in the module without `go get`. Consistent with the constitution's YAGNI principle.

**Alternatives considered**:
- `gocarina/gocsv` — struct-tag CSV mapping, convenient but adds a dependency for marginal ergonomic gain
- `jszwec/csvutil` — similar; adds dependency; overkill for 8 fixed columns

---

## Decision 2: Abort Mode — Database Strategy

**Decision**: `pgxpool.Pool.Begin(ctx)` → execute all `INSERT` statements within the transaction → `Commit()` on success or `Rollback()` on any error or validation failure.

**Rationale**: Abort mode requires atomicity — either all rows land or none do. A single transaction is the correct primitive. pgx transactions integrate cleanly with the existing pool.

**Alternatives considered**:
- Individual `Create()` calls followed by a compensating rollback loop — impossible without a real transaction
- Savepoints per row — overkill; not needed for abort mode

**Implementation note**: In abort mode, validation runs over all rows first (before opening the transaction). The transaction only opens if all rows pass. This avoids holding an open transaction during CPU-bound validation work.

---

## Decision 3: Skip Mode — Database Strategy

**Decision**: Reuse the existing `repo.Create(ctx, q)` method per valid row, with no overarching transaction.

**Rationale**: Skip mode is best-effort — each row is independent. Re-using `Create` avoids a parallel code path and keeps the repo surface small. If one insert fails (e.g., a race condition), only that row is skipped and reported.

**Alternatives considered**:
- Batch insert without transaction — still atomic per batch, which is wrong for skip semantics
- A dedicated `CreateNoTx` method — unnecessary; `Create` already works with the pool

---

## Decision 4: Category Resolution

**Decision**: Load all active categories once before row processing into a `map[string]uuid.UUID` keyed by `strings.ToLower(name)`. Resolve `category_name` per row via map lookup.

**Rationale**: Eliminates N+1 DB queries for category resolution. A quiz system will typically have < 100 categories. Loading them all into memory is safe and fast. Case-insensitive matching is implemented via `strings.ToLower` on both sides.

**Alternatives considered**:
- Per-row `SELECT id FROM categories WHERE LOWER(name) = LOWER($1)` — correct but 1 query per row; 500 rows = 500 queries
- Index on `LOWER(name)` + per-row query — still N queries; optimized but not necessary when a single bulk fetch suffices

---

## Decision 5: Duplicate Detection

**Decision**: For each valid row (after all other validation passes), run a single `EXISTS` query against the `questions` table: `WHERE LOWER(text) = LOWER($1) AND category_id = $2 AND deleted_at IS NULL`. A match appends a warning to the `errors` array but does not prevent the row from being imported.

**Rationale**: Matches spec FR-009 exactly. Per-row query is acceptable at ≤ 500 rows (at most 500 queries, each hitting a primary index). No additional data structures needed.

**Alternatives considered**:
- Bulk pre-fetch of all existing (text, category_id) pairs — could be millions of rows; not safe at scale
- Postgres `ON CONFLICT` — does not distinguish "duplicate warning" from a hard constraint violation; semantics differ

---

## Decision 6: File Size Enforcement

**Decision**: Check file size explicitly in the handler after `c.FormFile("file")` by inspecting `header.Size > 5*1024*1024`. Return 422 if exceeded. Additionally, configure `fiber.Config{BodyLimit: 5 * 1024 * 1024 + 1024}` in main to prevent the server from reading oversized bodies at all.

**Rationale**: Belt-and-suspenders. Fiber's body limit provides server-level protection; the explicit handler check provides a user-friendly JSON error. Fiber's default body limit is 4MB which is below our 5MB requirement, so the Fiber config must be updated.

**Alternatives considered**:
- Relying solely on Fiber body limit — returns a raw 413 without JSON envelope; breaks API contract
- Streaming the file and counting bytes — overly complex for synchronous import

---

## Decision 7: Row Limit Enforcement (500 rows)

**Decision**: After parsing the CSV with `encoding/csv`, count data rows. If `len(rows) > 500`, return an immediate 422 before any validation or DB access.

**Rationale**: Simple and correct. `encoding/csv` reads the entire file into memory anyway (the file is ≤ 5MB). Counting after parse is trivial.

---

## Decision 8: Template Endpoint

**Decision**: Inline the CSV header string as a constant and write it directly to the response body with `Content-Type: text/csv` and `Content-Disposition: attachment; filename="questions_template.csv"`. No auth required — registered on the public route group.

**Rationale**: Zero overhead, no file on disk to manage, no caching needed. The template never changes.

---

## Decision 9: Code Location

**Decision**: All new code lives in `internal/question/`:
- `models.go` — add `ImportRow`, `ImportResult`, `RowError` types, `ErrInvalidOnError` sentinel
- `repository.go` — add `FindCategoryMap(ctx) (map[string]uuid.UUID, error)`, `CheckDuplicate(ctx, text string, categoryID uuid.UUID) (bool, error)`, `CreateBatch(ctx, pgx.Tx, []Question) error`
- `service.go` — add `Import(ctx, file multipart.File, onError string) (ImportResult, error)` (or pass parsed rows)
- `handler.go` — add `Import(c *fiber.Ctx) error` and `ImportTemplate(c *fiber.Ctx) error`
- `main.go` — register 2 new routes

**Rationale**: Keeps the feature self-contained within the existing package boundary. No new package needed. Consistent with the 002/003 pattern.

**Note on service layer for Import**: The Import method on the service will receive the multipart file header (or an `io.Reader`) and the `onError` string. CSV parsing, validation, and orchestration of DB operations happen in the service. The handler's responsibility is only HTTP parsing and response formatting.
