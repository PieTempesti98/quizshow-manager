# Feature Specification: Session Lifecycle — draft → lobby → active

**Feature Branch**: `006-session-lifecycle`  
**Created**: 2026-06-29  
**Status**: Draft  
**Input**: User description: "Session state transitions draft → lobby → active: POST /open-lobby, GET /qr, POST /launch"

## User Scenarios & Testing *(mandatory)*

### User Story 1 — Open lobby so players can join (Priority: P1)

The admin has created and configured a session (draft status). Before starting the quiz, they want to open a waiting lobby where players can join using the PIN or QR code. The admin calls "Open Lobby" and the session becomes joinable by players.

**Why this priority**: This is the first gate in the session lifecycle. Without it, no player can ever join. Everything else depends on reaching lobby status.

**Independent Test**: Call `POST /api/v1/sessions/:id/open-lobby` on a session in draft status. Verify the response contains `pin`, `qr_code_url`, and `status: "lobby"`. Verify the session detail endpoint now returns `status: "lobby"`.

**Acceptance Scenarios**:

1. **Given** a session in `draft` status, **When** admin calls `POST /open-lobby`, **Then** session status changes to `lobby`, response returns `{ session_id, pin, qr_code_url, status: "lobby" }` with HTTP 200.
2. **Given** a session in `draft` status, **When** admin calls `POST /open-lobby`, **Then** the PIN previously generated at session creation is returned unchanged (not regenerated).
3. **Given** a session already in `lobby` status, **When** admin calls `POST /open-lobby` again, **Then** the server returns HTTP 409.
4. **Given** a session in `active`, `completed`, or `cancelled` status, **When** admin calls `POST /open-lobby`, **Then** the server returns HTTP 409.
5. **Given** a session ID that does not exist, **When** admin calls `POST /open-lobby`, **Then** the server returns HTTP 404.
6. **Given** an unauthenticated request, **When** admin calls `POST /open-lobby`, **Then** the server returns HTTP 401.

---

### User Story 2 — Retrieve QR code for the player join URL (Priority: P2)

The admin or presenter wants to display a scannable QR code that takes players directly to the join screen with the session PIN pre-filled. This QR code can be shown on a projector or printed.

**Why this priority**: Players can still join manually with the PIN, so the QR code is a convenience enhancement. The core lifecycle works without it, but it significantly lowers friction for player onboarding.

**Independent Test**: Call `GET /api/v1/sessions/:id/qr` (no auth) and verify the response is a valid PNG (`Content-Type: image/png`). Decode the QR and verify the encoded URL contains the session PIN.

**Acceptance Scenarios**:

1. **Given** any session with a known ID, **When** a client calls `GET /sessions/:id/qr` without an auth token, **Then** the server returns HTTP 200 with `Content-Type: image/png`.
2. **Given** the QR code PNG is decoded, **When** the encoded URL is read, **Then** it matches `{PLAYER_APP_BASE_URL}/join?pin={pin}` where `{pin}` is the session's 6-digit PIN.
3. **Given** the env var `PLAYER_APP_BASE_URL` is not set, **When** the QR is generated, **Then** the base URL defaults to `http://localhost:5173`.
4. **Given** a session ID that does not exist, **When** a client calls `GET /sessions/:id/qr`, **Then** the server returns HTTP 404.

---

### User Story 3 — Launch session: draw questions and go active (Priority: P1)

The admin has waited for enough players to join in the lobby and is ready to start. Launching draws a random question set from the configured pool, transitions the session to active, sets `started_at`, and issues a short-lived projection screen token.

**Why this priority**: This is the critical transition that starts the quiz. No question flow, presenter controls, or player answers can operate without reaching active status.

**Independent Test**: Call `POST /api/v1/sessions/:id/launch` on a session in lobby status that has questions available. Verify the response contains `status: "active"`, a decodable `projection_token` JWT, `started_at`, and `question_count`. Verify that `session_questions` rows exist in the database with positions 1..N.

**Acceptance Scenarios**:

1. **Given** a session in `lobby` status with available questions, **When** admin calls `POST /launch`, **Then** session status changes to `active`, `started_at` is set to now, `session_questions` rows are inserted (1-based positions), and the response returns `{ session_id, status: "active", question_count, projection_token, projection_url, started_at }` with HTTP 200.
2. **Given** the question pool has more questions than `question_count`, **When** `POST /launch` is called, **Then** exactly `question_count` questions are randomly selected.
3. **Given** the question pool has fewer questions than `question_count` but at least one, **When** `POST /launch` is called, **Then** all available questions are drawn (no error); the actual drawn count is returned in `question_count`.
4. **Given** the question pool has zero available questions, **When** `POST /launch` is called, **Then** the server returns HTTP 422 with error code `INSUFFICIENT_QUESTIONS` and the session remains in `lobby`.
5. **Given** a session not in `lobby` status (draft, active, completed, cancelled), **When** admin calls `POST /launch`, **Then** the server returns HTTP 409.
6. **Given** a successful launch, **When** the `projection_token` JWT is decoded, **Then** it contains claims `{ session_id, role: "projection" }` and has a 12-hour TTL.
7. **Given** the `projection_url` in the response, **When** the URL is parsed, **Then** it matches `{PLAYER_APP_BASE_URL}/projection?session={session_id}&token={projection_token}`.
8. **Given** a transient DB failure during `session_questions` insert after the draw, **When** the transaction is rolled back, **Then** the session remains in `lobby` and no partial draws are visible.
9. **Given** the WebSocket hub is not yet implemented, **When** launch succeeds, **Then** the no-op `SessionEventBroadcaster.BroadcastSessionStarted()` is called without error.

---

### Edge Cases

- What happens if `POST /open-lobby` is called concurrently twice for the same draft session? The first call wins; the second gets 409 because the status is no longer draft.
- What if the session's configured categories were soft-deleted after session creation? A category cannot be soft-deleted while it has active questions (guarded by `ErrCategoryHasQuestions` in the category service), so questions with `deleted_at IS NULL` are always eligible for draw — the pool shrinks accordingly if questions were soft-deleted.
- What if `PLAYER_APP_BASE_URL` contains a trailing slash? URL construction must normalize the slash to avoid double-slash in the resulting URL.
- Can the QR endpoint be called for a session in draft status (before open-lobby)? Yes — the PIN is set at creation and the QR has no status constraint (see Assumptions for rationale).

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: System MUST transition a session from `draft` to `lobby` when `POST /sessions/:id/open-lobby` is called with a valid admin token.
- **FR-002**: System MUST return the existing session PIN (set at creation) in the open-lobby response without regenerating it.
- **FR-003**: System MUST include `qr_code_url` pointing to `/api/v1/sessions/{id}/qr` in the open-lobby response.
- **FR-004**: System MUST return HTTP 409 for `POST /open-lobby` when the session is not in `draft` status.
- **FR-005**: System MUST serve a valid PNG image at `GET /sessions/:id/qr` with no authentication required.
- **FR-006**: The QR code PNG MUST encode the URL `{PLAYER_APP_BASE_URL}/join?pin={pin}` (placeholder pending frontend deployment).
- **FR-007**: `PLAYER_APP_BASE_URL` MUST be read from an environment variable; the default value when unset is `http://localhost:5173`.
- **FR-008**: System MUST transition a session from `lobby` to `active` when `POST /sessions/:id/launch` is called with a valid admin token.
- **FR-009**: On launch, system MUST randomly draw questions from the pool defined by the session's linked categories, excluding soft-deleted questions.
- **FR-010**: On launch, system MUST insert one `session_questions` row per drawn question with `position` values 1..N (1-based) and `asked_at`/`revealed_at` left NULL.
- **FR-011**: On launch, if the available question pool is empty, system MUST return HTTP 422 with error code `INSUFFICIENT_QUESTIONS` and leave the session in `lobby`.
- **FR-012**: On launch, if the pool is non-zero but smaller than `question_count`, system MUST use all available questions without returning an error.
- **FR-013**: On launch, system MUST set `sessions.started_at` to the current UTC timestamp.
- **FR-014**: On launch, system MUST generate a `projection_token` JWT with claims `{ session_id, role: "projection" }`, TTL 12h, signed with the same secret as admin tokens.
- **FR-015**: JWT signing for `projection_token` MUST reuse/extend the existing `internal/auth` token utility — signing logic must not be duplicated.
- **FR-016**: A `SessionEventBroadcaster` interface with method `BroadcastSessionStarted(sessionID string, totalQuestions int)` MUST be defined and a no-op implementation injected into the session service for this feature.
- **FR-017**: The draw and `session_questions` insert on launch MUST be wrapped in a single database transaction.
- **FR-018**: All three new endpoints MUST have handler methods implemented in `backend/internal/session/handler.go` and be registered on the Fiber router in `backend/cmd/server/main.go` (with `/qr` on the public group and `/open-lobby` & `/launch` on the protected group).

### Key Entities

- **Session**: Core lifecycle entity with a status state machine. This feature drives the `draft → lobby` and `lobby → active` transitions.
- **SessionQuestion**: Frozen snapshot of a drawn question. Created at launch with `session_id`, `question_id`, `position`; `asked_at` and `revealed_at` are populated by feature #7 (presenter controls).
- **ProjectionToken**: A stateless short-lived JWT (12h, role: "projection") issued at launch. Not stored in the database — validated by signature alone.

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: An admin can open the lobby for a configured session in a single call in under 500ms.
- **SC-002**: A session launch (including question draw and DB write) completes in under 1 second for pools up to 500 available questions.
- **SC-003**: The QR code PNG is served in under 300ms and is correctly decodable by standard QR scanner apps.
- **SC-004**: No partial question draws are ever committed: if the launch fails after draw but before full DB write, the session remains in lobby with zero session_questions rows.
- **SC-005**: All JSON responses conform to the standard `{ "data": ... }` / `{ "error": ... }` envelope, while `GET /sessions/:id/qr` returns raw binary PNG (`image/png`) data on HTTP 200 and the standard error envelope on failure.

## Assumptions

- The PIN is generated at session creation (feature #5). `POST /open-lobby` does not generate or alter it — it only transitions the status that makes the PIN active.
- The QR endpoint is available for sessions in any status (including draft) because the PIN exists from creation. This is a deliberate decision: the admin may want to pre-generate a QR code before opening the lobby. Logged in PLAN.md.
- `PLAYER_APP_BASE_URL` is a temporary placeholder because the player frontend does not yet exist (blocked by `docs/06-ui-flows.md`). Code will include a comment marking it as a placeholder to update at frontend deployment.
- The `projection_url` in the launch response uses the same `PLAYER_APP_BASE_URL` placeholder — the projection frontend does not exist yet either.
- The WebSocket `session_started` event (specified in `docs/04-api-design.md`) is NOT sent by this feature. A `SessionEventBroadcaster` interface with a no-op implementation is introduced to keep the door open for feature #10.
- Question randomness uses standard (non-cryptographic) random selection — sufficient for quiz ordering.
- No new database migration is required: `sessions`, `session_categories`, and `session_questions` tables all exist from migration 001.
- The `internal/auth` token utility is extended to support custom claims rather than duplicating JWT signing logic.
- The existing `RequireAdmin` middleware is reused for `/open-lobby` and `/launch`. The `/qr` endpoint has no auth middleware.
