# Data Model: Questions CSV Import

**Feature**: 004-questions-csv-import  
**Date**: 2026-05-03

## Database Changes

**No new migration required.** This feature inserts into the existing `questions` table using the same schema as the single-question `POST /api/v1/questions` endpoint. The `categories` table is read (not written).

---

## Application-Level Types (in-memory, not persisted)

### `ImportRow`

Represents one parsed and partially-validated data row from the uploaded CSV. Carries the 1-based row number for error reporting.

| Field | Go Type | CSV Column | Notes |
|---|---|---|---|
| `RowNumber` | `int` | — | 1-based, data rows only (header excluded) |
| `Text` | `string` | `text` | Required, non-empty |
| `OptionA` | `string` | `option_a` | Required, non-empty |
| `OptionB` | `string` | `option_b` | Required, non-empty |
| `OptionC` | `string` | `option_c` | Required, non-empty |
| `OptionD` | `string` | `option_d` | Required, non-empty |
| `CorrectIndex` | `int` | `correct_index` | Must be 0–3 |
| `CategoryName` | `string` | `category_name` | Resolved to CategoryID via lookup map |
| `Difficulty` | `string` | `difficulty` | Optional; defaults to `"medium"` if blank |

### `ImportResult`

The aggregate outcome returned to the caller after an import operation completes.

| Field | Go Type | JSON Key | Notes |
|---|---|---|---|
| `Imported` | `int` | `imported` | Count of rows successfully written to DB |
| `Skipped` | `int` | `skipped` | Count of rows not written due to validation errors |
| `Errors` | `[]RowError` | `errors` | Ordered list of per-row problems (hard errors + soft warnings) |

### `RowError`

A single entry in the `errors` array. Used for both hard validation failures (that prevent import) and soft duplicate warnings (that do not).

| Field | Go Type | JSON Key | Notes |
|---|---|---|---|
| `Row` | `int` | `row` | 1-based row number (data rows only) |
| `Message` | `string` | `message` | Human-readable description of the problem |

---

## Sentinel Errors

These are package-level `var` errors in `internal/question/`, following the existing pattern:

| Sentinel | Trigger |
|---|---|
| `ErrInvalidOnError` | `on_error` field is present but not `"abort"` or `"skip"` |

---

## Existing Tables Used (read-only lookups)

### `categories`

| Column | Used For |
|---|---|
| `id` | Target FK when inserting into `questions.category_id` |
| `name` | Matched case-insensitively against `category_name` CSV column |
| `deleted_at` | Must be `NULL` — soft-deleted categories are invisible to import |

### `questions` (read for duplicate check, written for inserts)

| Column | Used For |
|---|---|
| `text` | Duplicate check: `LOWER(text) = LOWER($1)` |
| `category_id` | Duplicate check: combined with text |
| `deleted_at` | Soft-deleted questions excluded from duplicate check |

---

## Validation Rules per Row

| Field | Rule | On Failure |
|---|---|---|
| `text` | Non-empty string | Hard error |
| `option_a` | Non-empty string | Hard error |
| `option_b` | Non-empty string | Hard error |
| `option_c` | Non-empty string | Hard error |
| `option_d` | Non-empty string | Hard error |
| `correct_index` | Parseable integer, value in `{0, 1, 2, 3}` | Hard error |
| `category_name` | Case-insensitive match in category lookup map | Hard error |
| `difficulty` | One of `easy`, `medium`, `hard` (or blank → `medium`) | Hard error if non-blank and invalid |
| Duplicate text+category | EXISTS check against `questions` table | Soft warning (row still imported) |

---

## State Transitions

```
Upload received
  → file size check (> 5MB → reject 422)
  → parse CSV headers (invalid headers → reject 422)
  → count data rows (> 500 → reject 422)
  → load category map (one DB query)
  → validate each row + duplicate check
      [abort mode]
        any hard errors? → return ImportResult{Imported:0, Errors:[...]}
        no hard errors? → BEGIN TX → batch INSERT → COMMIT → return ImportResult{Imported:N}
      [skip mode]
        for each row:
          hard error? → record RowError, increment Skipped
          valid? → repo.Create() → increment Imported (+ optional duplicate warning)
        → return ImportResult{Imported:M, Skipped:K, Errors:[...]}
```
