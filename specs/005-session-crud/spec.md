# Feature Specification: Session Create and Configure

**Feature Branch**: `005-session-crud`  
**Created**: 2026-05-04  
**Status**: Draft  
**Input**: User description: "Implement session create and configure (US-S01, US-S02): create a session with configuration, list sessions, get session detail, update session, soft-delete session."

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Create a Configured Session (Priority: P1)

An admin opens the session management screen and creates a new quiz session by providing a name, selecting one or more question categories, and setting parameters such as the number of questions, time per question, base points, and whether a speed bonus is active. The system auto-generates a 6-digit PIN, persists the session in draft status, and immediately tells the admin how many questions are available across the chosen categories — plus a warning if that number falls short of the requested count.

**Why this priority**: Without the ability to create a session, no other session management operation is possible. This is the entry point for the entire quiz lifecycle.

**Independent Test**: Can be fully tested by sending a valid POST request and confirming the response contains the new session ID, the generated PIN, `status: "draft"`, and `available_questions`. Delivers value by allowing admins to prepare sessions ahead of time.

**Acceptance Scenarios**:

1. **Given** a valid admin token and all required fields, **When** the admin submits a create-session request with `category_ids`, `question_count: 10`, `time_per_question_s: 30`, `points_per_answer: 100`, `speed_bonus_enabled: false`, **Then** the system returns HTTP 201 with the session object including a 6-character numeric PIN, `status: "draft"`, and `available_questions` reflecting the non-deleted question count across the selected categories.

2. **Given** a request where the selected categories contain only 7 non-deleted questions but `question_count` is 10, **When** the session is submitted, **Then** the system creates the session anyway (HTTP 201) and includes a `warning` field in the response with a message indicating the available count.

3. **Given** a request with `category_ids: []` (empty), **When** submitted, **Then** the system returns HTTP 422 with `VALIDATION_ERROR`.

4. **Given** a request with `question_count: 0` or `question_count: 51`, **When** submitted, **Then** the system returns HTTP 422 with `VALIDATION_ERROR`.

5. **Given** a request with `time_per_question_s: 45` (not in allowed set), **When** submitted, **Then** the system returns HTTP 422 with `VALIDATION_ERROR`.

---

### User Story 2 - View and List Sessions (Priority: P2)

An admin navigates to the sessions screen and sees a paginated list of sessions. They can filter by one or more statuses to quickly locate draft sessions awaiting launch, or completed sessions for review. Clicking into a session shows its full detail including the categories it draws from.

**Why this priority**: Listing and viewing sessions is needed for an admin to manage and monitor the quiz lifecycle. It is also required before editing or deleting a session.

**Independent Test**: Can be fully tested independently by reading sessions from the API — list and detail endpoints return data without needing the create or edit flow.

**Acceptance Scenarios**:

1. **Given** multiple sessions in various statuses, **When** the admin calls the list endpoint with no filters, **Then** the response returns a paginated list (default 20 per page) with each session showing `player_count` alongside configuration fields.

2. **Given** sessions in `draft`, `active`, and `completed` statuses, **When** the admin filters by `status=draft,active`, **Then** only sessions in those two statuses are returned.

3. **Given** a session ID, **When** the admin requests the detail endpoint, **Then** the response includes a `categories` array of `{ id, name }` objects in addition to all list fields.

4. **Given** a non-existent session ID, **When** detail is requested, **Then** the system returns HTTP 404.

---

### User Story 3 - Edit a Draft Session (Priority: P3)

An admin reviews a draft session and wants to adjust the question count, swap categories, or change the timer settings before going live. Any subset of configurable fields can be updated. The edit is blocked if the session is no longer in draft status.

**Why this priority**: Editing ensures sessions can be refined before launch. It is strictly scoped to draft status, so it cannot interfere with live sessions.

**Independent Test**: Can be tested by creating a draft session, sending a PATCH with changed fields, and confirming the response reflects the updates.

**Acceptance Scenarios**:

1. **Given** a session in `draft` status, **When** the admin sends a PATCH with a new `name` and updated `category_ids`, **Then** the system returns HTTP 200 with the updated session object.

2. **Given** a session in `active` status, **When** the admin attempts a PATCH, **Then** the system returns HTTP 409.

3. **Given** a session in `completed` status, **When** the admin attempts a PATCH, **Then** the system returns HTTP 409.

4. **Given** a PATCH that updates `category_ids` where available questions in the new set fall below `question_count`, **When** submitted, **Then** the system applies the update and includes a `warning` field in the response.

---

### User Story 4 - Delete a Draft Session (Priority: P4)

An admin decides to discard a session they created by mistake or no longer need. The system soft-deletes the session, removing it from standard listings. Deletion is blocked for sessions that are not in draft status.

**Why this priority**: Clean-up is a basic administrative need but is lower risk and lower priority than creation, viewing, and editing.

**Independent Test**: Can be tested independently by creating a draft session, deleting it, then confirming it no longer appears in the list.

**Acceptance Scenarios**:

1. **Given** a session in `draft` status, **When** the admin sends a DELETE request, **Then** the system returns HTTP 200 with `{ "data": { "ok": true } }` and the session is no longer returned by the list or detail endpoints.

2. **Given** a session in `lobby` status, **When** deletion is attempted, **Then** the system returns HTTP 409.

3. **Given** a session in `active` status, **When** deletion is attempted, **Then** the system returns HTTP 409.

4. **Given** an already-deleted session ID, **When** deletion is attempted again, **Then** the system returns HTTP 404.

---

### Edge Cases

- What happens when two admins simultaneously create sessions and generate the same PIN? The PIN uniqueness constraint is enforced at the database level via a partial unique index; one of the two inserts will fail and the application must retry with a freshly generated PIN.
- What happens when a category referenced in `category_ids` does not exist or is soft-deleted? The system returns HTTP 422 — all category IDs must resolve to active categories.
- What happens when `available_questions` is zero? The session is still created, with the warning message indicating zero available questions. The admin must add questions to the selected categories before the session can be launched.
- What happens when paginating beyond the last page? The system returns an empty `sessions` array with correct pagination metadata (`total` and `total_pages` unaffected).

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: System MUST allow an authenticated admin to create a session providing: name (required), at least one category ID (required), question count 1–50 (required), time per question in seconds — one of 10/20/30/60 (required), points per correct answer as a positive integer defaulting to 100 (optional), speed bonus enabled flag defaulting to false (optional).
- **FR-002**: System MUST auto-generate a unique 6-digit numeric PIN at session creation; uniqueness is scoped to sessions currently in `lobby` or `active` status.
- **FR-003**: System MUST compute `available_questions` at create time as the count of non-deleted questions across all selected categories and include it in the creation response.
- **FR-004**: System MUST create the session in `draft` status even when `available_questions < question_count`, and MUST include a human-readable `warning` field in the response in that case.
- **FR-005**: System MUST populate `created_by` from the authenticated admin identity carried in the JWT.
- **FR-006**: System MUST write all selected `category_ids` to the `session_categories` join table at creation time.
- **FR-007**: System MUST return a paginated list of sessions (default 20 per page) filterable by one or more statuses (comma-separated), including `player_count` per session.
- **FR-008**: System MUST return full session detail including a `categories` array of `{ id, name }` objects for the detail endpoint.
- **FR-009**: System MUST allow partial updates (any subset of configurable fields) on sessions with `status = "draft"` only; attempts to update sessions in any other status MUST return HTTP 409.
- **FR-010**: System MUST soft-delete (set `deleted_at`) sessions with `status = "draft"` only; attempts to delete sessions in any other status MUST return HTTP 409.
- **FR-011**: Soft-deleted sessions MUST be excluded from all standard list and detail queries.
- **FR-012**: System MUST validate that all provided `category_ids` resolve to active (non-deleted) categories; any invalid ID MUST cause HTTP 422.
- **FR-013**: When a PATCH supplies `category_ids`, the system MUST replace the existing `session_categories` rows entirely (delete old, insert new) within a single transaction.

### Key Entities

- **Session**: The primary entity representing a quiz session. Holds configuration (name, PIN, question count, timer, points, speed bonus), lifecycle status (`draft` → `lobby` → `active` → `completed`/`cancelled`), timestamps, and a reference to the creating admin.
- **SessionCategory**: Join record linking a session to each of its selected categories. Written in bulk on create; fully replaced on PATCH when `category_ids` is provided.
- **Category**: Referenced by ID and name in session detail responses; not modified by session operations.
- **Player**: Counted per session (as `player_count`) in list responses via an aggregate over the players table.

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: An admin can create a fully configured session, view it in the list, open its detail, edit it, and delete it — all completing successfully with no data loss.
- **SC-002**: Session list responds within 300 ms for a library of up to 500 sessions under normal load (consistent with the existing question list SLA in the requirements).
- **SC-003**: PIN collisions are prevented deterministically — no two sessions in `lobby` or `active` status share the same PIN at any point in time, even under concurrent creation.
- **SC-004**: The `available_questions` value in the creation response is accurate: it exactly matches the count of non-deleted questions in the selected categories at the moment of creation.
- **SC-005**: Every invalid request (missing required fields, out-of-range values, invalid category IDs, wrong-status PATCH/DELETE) is rejected with an appropriate HTTP 4xx response and no partial writes reach the database.

## Assumptions

- The `sessions` and `session_categories` tables are already present in the database via migration 001 — no new migration is required for this feature.
- The partial unique index `sessions_pin_active_unique` on `(pin) WHERE status IN ('lobby', 'active') AND deleted_at IS NULL` already exists and handles PIN uniqueness at the database level; the application generates a random PIN and retries on conflict.
- `player_count` in list responses is derived by counting rows in the `players` table per session; no denormalized column exists for this value in MVP.
- Speed bonus semantics (the linear formula in docs/03-scoring-mechanics.md) inform the `speed_bonus_enabled` flag meaning but score computation is not part of this feature — it is applied at reveal time in a later feature.
- `time_per_question_s` accepts exactly four values: 10, 20, 30, 60. Any other value is rejected as a validation error.
- `points_per_answer` must be a positive integer; there is no defined upper bound in MVP.
- The admin authentication middleware already places the admin ID in the request context (as established in the 001-admin-auth feature); this feature reads it without modification.
