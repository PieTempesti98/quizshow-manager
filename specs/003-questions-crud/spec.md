# Feature Specification: Questions CRUD

**Feature Branch**: `003-questions-crud`
**Created**: 2026-04-24
**Status**: Draft
**Input**: User description: "Implement questions CRUD (US-Q01, US-Q02, US-Q05): paginated list with filters, create a question, partial update, soft-delete."

## User Scenarios & Testing *(mandatory)*

### User Story 1 — Browse and Find Questions (Priority: P1)

An admin needs to find specific questions in the question bank before editing, auditing, or verifying content. They open the question list and apply filters — by category, by difficulty, or by typing a search term — to narrow down the results. The list paginates automatically so large banks remain navigable.

**Why this priority**: Without the ability to list and search questions, the admin cannot discover what already exists, cannot avoid duplicates, and cannot locate questions for editing or deletion. This is the foundational read capability the entire CRUD feature depends on.

**Independent Test**: Can be fully tested by seeding several questions across categories and difficulties, then requesting the list endpoint with various filter combinations and verifying the correct subset is returned with accurate pagination metadata.

**Acceptance Scenarios**:

1. **Given** the admin is authenticated, **When** they request the question list with no filters, **Then** they receive up to 25 questions sorted by creation date descending, with pagination metadata (current page, per-page count, total questions, total pages).
2. **Given** questions exist across multiple categories, **When** the admin filters by one or more category IDs (comma-separated), **Then** only questions belonging to those categories are returned.
3. **Given** questions with mixed difficulties exist, **When** the admin filters by one or more difficulty values (e.g. `easy,hard`), **Then** only questions with those difficulty levels are returned.
4. **Given** questions with varied text exist, **When** the admin provides a search term via the `q` parameter, **Then** only questions whose text contains that term (case-insensitive) are returned.
5. **Given** multiple active filters, **When** the admin combines category, difficulty, and text search simultaneously, **Then** only questions matching all applied filters are returned.
6. **Given** a large question bank, **When** the admin requests page 2 with per_page=10, **Then** the correct slice of results is returned along with updated pagination metadata.
7. **Given** the admin requests per_page=150, **Then** the system caps the value to 100 and returns at most 100 results.
8. **Given** there are no questions matching the applied filters, **Then** an empty list is returned with total=0 — not an error.
9. **Given** each question in the response, **Then** the category field is a nested object containing `id` and `name` — not just the category ID.

---

### User Story 2 — Create a Question (Priority: P2)

An admin wants to add a new multiple-choice question to the question bank so it can be included in future quiz sessions. They provide the question text, four distinct answer options, the index of the correct answer (0–3), an existing category, and optionally a difficulty level.

**Why this priority**: Creating questions is the primary content management action; the bank cannot grow without it. It is independently useful alongside listing.

**Independent Test**: Can be fully tested by submitting a valid create request and then verifying the new question appears in the question list with correct data including the nested category object and the defaulted difficulty.

**Acceptance Scenarios**:

1. **Given** a valid request with all required fields, **When** the admin creates a question, **Then** the question is persisted and the full question object (including nested category) is returned with HTTP 201.
2. **Given** the `difficulty` field is omitted from the request, **When** the question is created, **Then** difficulty defaults to `medium`.
3. **Given** a `correct_index` value outside 0–3 is submitted, **Then** the system rejects it with a validation error.
4. **Given** any of the four answer option fields is empty or absent, **Then** the system rejects the request with a validation error.
5. **Given** the question text is empty or absent, **Then** the system rejects the request with a validation error.
6. **Given** an invalid or non-existent category ID is provided, **Then** the system rejects the request with an appropriate error.
7. **Given** an invalid difficulty value (not `easy`, `medium`, or `hard`) is provided, **Then** the system rejects the request with a validation error.
8. **Given** a successfully created question, **When** the admin lists questions with no filters, **Then** the new question appears at the top of the default-sorted list.

---

### User Story 3 — Edit a Question (Priority: P3)

An admin wants to correct a mistake or update content in an existing question. They may change any subset of fields without resubmitting all fields. The edit is rejected if the question is currently part of an ongoing live session.

**Why this priority**: Editing preserves question bank accuracy over time. It can be delivered after create/list and before deletion, and is independently demonstrable.

**Independent Test**: Can be fully tested by creating a question, PATCHing only one or two fields, and verifying the response reflects the new values while all unchanged fields are preserved exactly.

**Acceptance Scenarios**:

1. **Given** an existing question, **When** the admin PATCHes with only the `text` field, **Then** only the text is updated; all other fields remain unchanged.
2. **Given** any valid subset of fields in the PATCH body, **When** submitted, **Then** only those fields are updated; the rest retain their current values.
3. **Given** the question is referenced in a session with `active` status, **When** the admin attempts to PATCH it, **Then** the request is rejected with a `QUESTION_IN_USE` error.
4. **Given** the question is referenced only in sessions with `completed`, `draft`, `lobby`, or `cancelled` status, **When** the admin PATCHes it, **Then** the update succeeds.
5. **Given** invalid values in the PATCH body (e.g. `correct_index: 5`), **Then** the request is rejected with a validation error.
6. **Given** a non-existent question ID, **When** the admin PATCHes, **Then** the system responds with a not-found error.

---

### User Story 4 — Delete a Question (Priority: P4)

An admin wants to remove an outdated or incorrect question from the question bank. The deletion is soft — the question disappears from all listings but its data is preserved in the historical record of past sessions. Deletion is blocked if the question is part of an active live session.

**Why this priority**: Deletion rounds out full CRUD. The soft-delete pattern is a hard requirement for historical session integrity. It is independently testable after the other three operations exist.

**Independent Test**: Can be fully tested by creating a question, deleting it, verifying it no longer appears in any list response, and confirming that a question linked to an active session cannot be deleted.

**Acceptance Scenarios**:

1. **Given** an existing question not referenced in any active session, **When** the admin deletes it, **Then** HTTP 200 is returned and the question no longer appears in the question list.
2. **Given** the question is referenced in a session with `active` status, **When** the admin attempts to delete it, **Then** the request is rejected with a `QUESTION_IN_USE` error.
3. **Given** the question was used in a `completed` session, **When** the admin deletes it, **Then** the deletion succeeds and historical session data referencing the question remains intact.
4. **Given** a non-existent question ID, **When** the admin attempts to delete, **Then** the system responds with a not-found error.
5. **Given** a successfully deleted question, **When** the admin lists questions with any combination of filters, **Then** the deleted question never appears in the results.

---

### Edge Cases

- What happens when all filters produce zero results? → An empty list is returned with `total: 0` and no error.
- What happens when `per_page` exceeds 100? → The value is silently clamped to 100.
- What happens when a question is soft-deleted and a completed session that used it is queried later? → The session history (stored as a snapshot in `session_questions`) continues to reference the question's data captured at session launch; the soft delete does not corrupt or affect historical records.
- What happens when the admin tries to update `category_id` to a soft-deleted or non-existent category? → The request is rejected with a validation error.
- What happens when the admin PATCHes a question with an empty body? → The question is returned unchanged (no-op); no fields are modified.
- What happens when an unauthenticated request reaches any of the four endpoints? → The request is rejected immediately with an authentication error.

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: The system MUST provide a paginated list of non-deleted questions, defaulting to 25 results per page with a maximum of 100 per page, sorted by creation date descending.
- **FR-002**: The list endpoint MUST support filtering by one or more category IDs supplied as comma-separated values; results are restricted to questions belonging to any of the specified categories.
- **FR-003**: The list endpoint MUST support filtering by one or more difficulty values (`easy`, `medium`, `hard`) supplied as comma-separated values; results are restricted to questions at the matching difficulty levels.
- **FR-004**: The list endpoint MUST support case-insensitive free-text search over the question text field; this filter is combinable with category and difficulty filters.
- **FR-005**: Every question object in list and create/update responses MUST include the category as a nested object containing both the category `id` and `name`.
- **FR-006**: The list response MUST include a pagination envelope with: current page number, items per page, total matching question count, and total page count.
- **FR-007**: The system MUST allow admins to create a question by supplying: question text (required, non-empty), four answer options (all required and non-empty), correct answer index 0–3 (required), a valid active category ID (required), and difficulty (optional, defaults to `medium`).
- **FR-008**: The system MUST reject any create or update request where `correct_index` is outside the range 0–3, returning a validation error.
- **FR-009**: The system MUST reject any create request where any of the four answer option fields is absent or empty, returning a validation error.
- **FR-010**: The update endpoint MUST support partial updates — only the fields present in the request body are changed; absent fields retain their current values.
- **FR-011**: The system MUST block both edit (PATCH) and delete (DELETE) operations on a question that is referenced in any session currently in `active` status, returning a `QUESTION_IN_USE` error with HTTP 409.
- **FR-012**: Deleting a question MUST be a soft delete — the record is marked as deleted and disappears from all listings, but the underlying data and its association with historical session snapshots are preserved.
- **FR-013**: All four endpoints (list, create, update, delete) MUST require a valid admin JWT; requests without a valid token MUST be rejected with an authentication error.

### Key Entities

- **Question**: The core content unit in the question bank. Identified by a UUID. Fields: id, category (nested: id + name), question text, four answer options (A/B/C/D), correct answer index (0–3), difficulty (easy/medium/hard), creation timestamp. A question is active when its `deleted_at` field is not set.
- **Category**: A topic grouping for questions, managed separately. A question always belongs to exactly one active category. Exposed in question responses as a nested object (id + name only — not the full category record).
- **Session**: A live quiz event that references questions via a snapshot taken at launch. A question is considered "in use" when it appears in a session currently in `active` status. Historical references via session snapshots are preserved even after the question is soft-deleted.

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: Admins can retrieve a filtered, paginated question list in under 300ms for a question bank of up to 10,000 questions under normal operating conditions (per US-Q05 acceptance criteria).
- **SC-002**: A newly created question appears in the question list within the same request-response cycle — no additional refresh or delay is required.
- **SC-003**: Every attempt to edit or delete a question that is used in an active session is blocked with a clear, unambiguous error message; zero cases of accidental modification during a live quiz.
- **SC-004**: Soft-deleted questions are absent from 100% of list responses regardless of the filter combination applied — no deleted question ever appears in results.
- **SC-005**: Partial updates preserve all unspecified fields unchanged in 100% of cases — no silent data loss from omitted fields.
- **SC-006**: All four endpoints enforce admin authentication — 100% of unauthenticated requests receive a rejection response.

## Assumptions

- Questions are listed in default descending `created_at` order; secondary sort by category is also available (both are defined in US-Q05).
- Only `active` session status blocks question edits and deletes; questions referenced in `draft`, `lobby`, `completed`, or `cancelled` sessions can be freely edited and deleted.
- The category referenced in a create or update request must already exist and not be soft-deleted; this feature does not create categories (covered by US-Q04, already implemented as feature 002).
- The `session_questions` snapshot pattern — questions are drawn and stored at session launch time — ensures that soft-deleting a question after a session is launched does not affect any in-progress or completed session's data.
- In MVP, difficulty is a label and filter only; it has no effect on scoring (confirmed in scoring mechanics documentation §3).
- There is no dedicated single-question GET endpoint (`GET /questions/:id`) in this feature scope; individual question data is accessed through the list or returned as the response body of create/update operations.
- An empty PATCH body is treated as a no-op: the question is returned unchanged rather than raising an error.
