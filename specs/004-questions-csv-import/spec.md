# Feature Specification: Questions CSV Import

**Feature Branch**: `004-questions-csv-import`  
**Created**: 2026-05-03  
**Status**: Draft  
**Input**: User description: "Implement questions CSV import (US-Q03): bulk import from multipart/form-data CSV upload, synchronous processing with per-row validation and report."

## User Scenarios & Testing *(mandatory)*

### User Story 1 — Download CSV Template (Priority: P1)

An admin wants to populate the question bank in bulk. Before uploading, they need to know the exact file format. They navigate to the import page, download the CSV template, open it in a spreadsheet app, fill in their questions, and save.

**Why this priority**: The template is a prerequisite for any successful import. Without it, users cannot produce a correctly-formatted file. It is also the simplest endpoint (no auth, no DB writes) and can be delivered and tested independently.

**Independent Test**: Download the template file, verify it contains exactly the expected column headers and no data rows, and confirm it is served as a downloadable CSV attachment.

**Acceptance Scenarios**:

1. **Given** any unauthenticated or authenticated request, **When** calling `GET /api/v1/questions/import/template`, **Then** the server returns a `200 OK` with `Content-Type: text/csv`, `Content-Disposition: attachment; filename="questions_template.csv"`, and a single header row: `text,option_a,option_b,option_c,option_d,correct_index,category_name,difficulty`.
2. **Given** the downloaded template, **When** the admin opens it in a spreadsheet tool, **Then** exactly 8 columns are present and no data rows exist.

---

### User Story 2 — Bulk Import with Abort Mode (Priority: P1)

An admin has prepared a CSV file with many questions. They upload it expecting an all-or-nothing result: either every valid row is imported or nothing is written to the database, so the question bank stays consistent.

**Why this priority**: This is the core import flow. Abort mode is the default and the safest option — it prevents partial imports that would leave the database in an inconsistent state.

**Independent Test**: Upload a CSV with all valid rows in abort mode and verify that all rows appear in the question bank. Then upload a CSV with one invalid row and verify that zero rows were imported and a full error list is returned.

**Acceptance Scenarios**:

1. **Given** a valid CSV with ≤ 500 rows and `on_error=abort` (or omitted), **When** the admin uploads the file, **Then** all rows are imported atomically, the response contains `imported: N, skipped: 0, errors: []`, and `N` new questions appear in the question bank.
2. **Given** a CSV where at least one row has a validation error and `on_error=abort`, **When** the admin uploads the file, **Then** zero rows are written to the database, the response contains `imported: 0, skipped: 0`, and `errors` lists every failing row with its row number and a human-readable reason.
3. **Given** a CSV with more than 500 data rows, **When** the admin uploads the file, **Then** the entire file is rejected before any row processing with a clear error message, and nothing is written to the database.
4. **Given** a CSV file exceeding 5 MB, **When** the admin uploads the file, **Then** the server rejects it immediately with a descriptive error.

---

### User Story 3 — Bulk Import with Skip Mode (Priority: P2)

An admin has a large CSV where some rows may be imperfect but wants to import as many valid questions as possible without discarding the entire batch.

**Why this priority**: Skip mode is the more lenient alternative. It is valuable for large datasets but less critical than abort mode for safety. It can be delivered after abort mode is working.

**Independent Test**: Upload a CSV with a mix of valid and invalid rows using `on_error=skip` and verify that only the valid rows are imported, the invalid ones are reported, and the final counts are correct.

**Acceptance Scenarios**:

1. **Given** a CSV with some valid and some invalid rows and `on_error=skip`, **When** the admin uploads the file, **Then** only the valid rows are imported, the response contains the correct `imported` and `skipped` counts, and `errors` describes each skipped row.
2. **Given** a CSV where all rows are invalid and `on_error=skip`, **When** the admin uploads the file, **Then** the response contains `imported: 0, skipped: N, errors: [...]` with all rows listed.

---

### User Story 4 — Row-Level Validation Feedback (Priority: P2)

An admin uploads a file with various mistakes (unknown category, wrong `correct_index`, missing required field). They need precise feedback to know which rows to fix and why.

**Why this priority**: Quality error messages reduce the admin's iteration cycle. Without them, the feature is usable but painful.

**Independent Test**: Upload a CSV with rows containing each type of validation error and verify that each row error message clearly identifies the row number and the specific problem.

**Acceptance Scenarios**:

1. **Given** a row where `category_name` does not match any active category (case-insensitive), **When** the file is processed, **Then** the error for that row reads something like `"Category 'X' not found"`.
2. **Given** a row where `correct_index` is not an integer in 0–3, **When** the file is processed, **Then** the error identifies the invalid value.
3. **Given** a row where a required field (`text`, `option_a`–`option_d`, `correct_index`, `category_name`) is empty, **When** the file is processed, **Then** the error names the missing field.
4. **Given** a row where `difficulty` is omitted or blank, **When** the file is processed, **Then** the row is treated as `medium` difficulty with no error.
5. **Given** a row whose `text` is identical to an existing question in the same category, **When** the file is processed, **Then** a warning is included in the error list for that row (import still succeeds for that row unless another hard error exists).

---

### Edge Cases

- What happens when the uploaded file is not a CSV (e.g., `.xlsx`, `.txt`)? → Rejected with a descriptive content-type or parse error before row processing.
- What happens when the CSV has the correct headers but zero data rows? → Response: `imported: 0, skipped: 0, errors: []`.
- What happens when the `on_error` field is omitted entirely? → Defaults to `abort`.
- What happens when `on_error` has an invalid value (not `abort` or `skip`)? → Request rejected with a validation error before file processing.
- What happens when the file field is missing from the multipart request? → Request rejected with a descriptive error.
- What happens when the CSV header row is missing or has unrecognized column names? → File rejected with a descriptive parse error before any row is processed.

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: The system MUST expose `GET /api/v1/questions/import/template` that returns a downloadable CSV file containing exactly the 8 column headers (`text,option_a,option_b,option_c,option_d,correct_index,category_name,difficulty`) and no data rows. This endpoint requires no authentication.
- **FR-002**: The system MUST expose `POST /api/v1/questions/import` that accepts a `multipart/form-data` request with a `file` field (CSV, max 5 MB) and an optional `on_error` field (`"abort"` or `"skip"`, default `"abort"`). This endpoint requires admin authentication.
- **FR-003**: The system MUST reject any import file containing more than 500 data rows before processing any row.
- **FR-004**: The system MUST validate each row for: all required fields present and non-empty (`text`, `option_a`, `option_b`, `option_c`, `option_d`, `correct_index`, `category_name`); `correct_index` is an integer in range 0–3; `category_name` matches an existing, non-deleted category (case-insensitive match on category `name`); `difficulty` is one of `easy`, `medium`, `hard` when present.
- **FR-005**: The system MUST default `difficulty` to `medium` when the field is absent or blank in a row.
- **FR-006**: In `abort` mode, the system MUST write zero rows to the database if any row fails validation, and MUST return the complete list of row errors.
- **FR-007**: In `skip` mode, the system MUST import each valid row and skip each invalid row, and MUST return accurate `imported` and `skipped` counts along with the list of skipped-row errors.
- **FR-008**: The system MUST return a structured response with `imported` (integer), `skipped` (integer), and `errors` (array of `{ row: integer, message: string }`) for every import request.
- **FR-009**: The system MUST detect duplicate questions (identical `text` in the same category) and include a warning entry in the `errors` array for the affected row without treating it as a hard validation failure.
- **FR-010**: The system MUST run abort-mode imports inside a single database transaction so that a failure at any point leaves the database unchanged.

### Key Entities

- **ImportRow**: A parsed row from the uploaded CSV, containing `text`, `option_a`–`option_d`, `correct_index`, `category_name`, `difficulty`, and a 1-based `row_number` for error reporting.
- **ImportResult**: The aggregate outcome of an import operation — `imported` count, `skipped` count, and the ordered list of `RowError` entries.
- **RowError**: A single row-level problem — `row` (1-based integer, data rows only, header excluded) and `message` (human-readable description of the failure or warning).

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: An admin can download the CSV template, fill in question data, and successfully upload it in a single attempt without consulting external documentation.
- **SC-002**: A valid 500-row CSV file import completes within 10 seconds from upload submission to final response.
- **SC-003**: Every row error response includes the exact row number and a message that identifies the specific field or value that caused the failure — zero ambiguous or generic error messages.
- **SC-004**: In abort mode, a file with any invalid row produces exactly zero new questions in the question bank.
- **SC-005**: In skip mode, the sum of `imported` + `skipped` in the response always equals the total number of data rows in the uploaded file.

## Assumptions

- The admin interface is not yet built; these endpoints will initially be exercised via direct HTTP calls (curl, Postman) or automated tests.
- The `categories` table is already populated with active categories before any import is attempted; the import feature does not create new categories on the fly.
- The CSV file is expected to use standard comma delimiter and UTF-8 encoding. Other encodings or delimiters are out of scope for MVP.
- Row numbers in error messages are 1-based and refer to data rows only (the header row is row 0 and excluded from numbering, so the first data row is row 1).
- The `on_error` field is sent as a plain form field alongside the file in the multipart body; it is not a query parameter.
- Duplicate detection (FR-009) is a soft warning: the duplicate row is still imported unless another hard validation error is present on the same row.
- The import endpoint lives in `internal/question/` alongside the existing questions CRUD code, reusing the existing repository and service patterns.
