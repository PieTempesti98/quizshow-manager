# HTTP Contracts: Questions CSV Import

**Feature**: 004-questions-csv-import  
**Date**: 2026-05-03  
**Base path**: `/api/v1`

All responses follow the project envelope: `{ "data": ..., "error": null }` or `{ "data": null, "error": { "code": "...", "message": "..." } }`.

---

## GET /api/v1/questions/import/template

**Auth**: None  
**Description**: Returns a downloadable CSV template with column headers and no data rows.

### Response — 200 OK

```
Content-Type: text/csv
Content-Disposition: attachment; filename="questions_template.csv"

text,option_a,option_b,option_c,option_d,correct_index,category_name,difficulty
```

No JSON envelope — this is a raw file download.

---

## POST /api/v1/questions/import

**Auth**: Admin Bearer JWT (same `RequireAdmin` middleware as other protected routes)  
**Content-Type**: `multipart/form-data`

### Request Fields

| Field | Type | Required | Description |
|---|---|---|---|
| `file` | File (CSV) | Yes | The CSV file to import. Max 5 MB. |
| `on_error` | String | No | `"abort"` (default) or `"skip"` |

### CSV Format

```
text,option_a,option_b,option_c,option_d,correct_index,category_name,difficulty
"In che anno cadde l'Impero Romano d'Occidente?","476 d.C.","410 d.C.","395 d.C.","455 d.C.",0,Storia,medium
"Chi dipinse la Sistina?","Leonardo","Michelangelo","Raffaello","Botticelli",1,Arte,easy
```

Column rules:
- `correct_index`: integer, must be 0, 1, 2, or 3
- `difficulty`: `easy`, `medium`, or `hard`; omit or leave blank to default to `medium`
- `category_name`: must match an existing, non-deleted category (case-insensitive)
- All other columns: non-empty strings

### Response — 200 OK (import completed, with or without errors)

```json
{
  "data": {
    "imported": 48,
    "skipped": 2,
    "errors": [
      {
        "row": 12,
        "message": "category 'Sport' not found"
      },
      {
        "row": 34,
        "message": "correct_index must be between 0 and 3, got: 5"
      }
    ]
  },
  "error": null
}
```

Notes:
- `errors` may include soft duplicate warnings even when `imported > 0`
- In `abort` mode with any hard errors: `imported: 0`, `skipped: 0`, full error list
- In `abort` mode with no hard errors: `imported: N`, `skipped: 0`, `errors` may still contain warnings

### Response — 422 Unprocessable Entity (pre-import rejection)

Used when the file or request itself is invalid before any row processing:

```json
{
  "data": null,
  "error": {
    "code": "VALIDATION_ERROR",
    "message": "file exceeds maximum size of 5MB"
  }
}
```

Other 422 triggers:
- `on_error` is present but not `"abort"` or `"skip"`
- `file` field missing from the request
- CSV header row missing or has unrecognized columns
- More than 500 data rows in the file

### Response — 401 Unauthorized

```json
{
  "data": null,
  "error": {
    "code": "UNAUTHORIZED",
    "message": "missing or invalid token"
  }
}
```

---

## Error Codes Reference

| Code | HTTP Status | Description |
|---|---|---|
| `VALIDATION_ERROR` | 422 | File-level or request-level validation failure (pre-row-processing) |
| `UNAUTHORIZED` | 401 | Missing or invalid admin JWT |
| `INTERNAL_ERROR` | 500 | Unexpected server error |

Row-level errors are not HTTP errors — they are returned as entries in the `errors` array within a 200 response.
