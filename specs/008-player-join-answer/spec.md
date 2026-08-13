# Feature Specification: Feature #8 — Player Join & Answer Submission

**Feature Branch**: `008-player-join-answer`  
**Created**: 2026-08-14  
**Status**: Draft  
**Input**: User description: "Feature #8 — Player join + answer (US-PL01, US-PL03): POST /api/v1/sessions/:id/join (public, PIN + nickname validation, avatar color, JWT issuance, event emission) and POST /api/v1/sessions/:session_id/answers (player auth, question active + timer validation, idempotency on resubmit, answer_time_ms calculation, event emission) with SessionEventBroadcaster decoupling"

## User Scenarios & Testing *(mandatory)*

### User Story 1 — Join Session Lobby via PIN and Nickname (Priority: P1)

As an anonymous player on a mobile or web browser, I want to enter a 6-digit session PIN and choose a nickname so that I can join the quiz session lobby without creating a persistent user account.

**Why this priority**: Joining the session is the essential entry point for all participants. Without this capability, no players can connect, participate in the lobby, or answer quiz questions.

**Independent Test**: Call `POST /api/v1/sessions/:id/join` with valid PIN and unique nickname on a session in `lobby` status. Verify HTTP 201 returning `player_id`, `nickname`, `avatar_color`, `session_id`, `session_name`, `player_token` (JWT with `player` role and 4-hour expiry), and `token_expires_at`. Verify record creation in `players` table and verify that `BroadcastPlayerJoined` is triggered with player details and total connected count.

**Acceptance Scenarios**:

1. **Given** a session in `lobby` status and a valid PIN, **When** a player submits a unique nickname (2–20 characters), **Then** the server registers the player in `players` with an assigned hex `avatar_color`, issues a signed Player JWT (role `player`, TTL 4h), emits `BroadcastPlayerJoined`, and returns HTTP 201 with player credentials.
2. **Given** a session in `lobby` status, **When** a player submits a nickname that is already registered for that session, **Then** the server rejects the request with HTTP 409 (`CONFLICT` / `NICKNAME_TAKEN`) without registering a duplicate.
3. **Given** a session that is in `draft`, `active`, `completed`, or `cancelled` status, **When** a player attempts to join, **Then** the server returns HTTP 409 (`SESSION_NOT_IN_LOBBY`).
4. **Given** an invalid PIN that does not match the session's generated PIN, **When** a player attempts to join, **Then** the server returns HTTP 404 (`NOT_FOUND` / `INVALID_PIN`).
5. **Given** an invalid payload (e.g. nickname < 2 or > 20 characters, missing PIN, malformed JSON), **When** a player attempts to join, **Then** the server returns HTTP 422 (`VALIDATION_ERROR`).

---

### User Story 2 — Submit Answer with Active Timer & Idempotency (Priority: P1)

As a joined player in an active quiz show, I want to submit my selected answer index (0–3) for the current question so that my response is recorded server-side with precise timing before the timer expires.

**Why this priority**: Submitting answers is the core game action for participants during live questions. Real-time recording, timer enforcement, and idempotent retry guarantee fair competition and smooth gameplay over mobile networks.

**Independent Test**: Advance an active session to a question (`asked_at = now()`). Authenticate as a joined player and call `POST /api/v1/sessions/:session_id/answers` with `session_question_id` and `chosen_index` (0..3). Verify HTTP 201 returning `answer_id`, `chosen_index`, and `answered_at`. Verify that `answers` table contains the row with calculated `answer_time_ms` while `is_correct` and `points_awarded` remain unrevealed (`false` and `0`). Call the same endpoint again with the same `session_question_id` and verify HTTP 200 returning the existing answer.

**Acceptance Scenarios**:

1. **Given** an active session with an open question (`asked_at` is set, `revealed_at` is null, and elapsed time <= `time_per_question_s`), **When** an authenticated player submits `session_question_id` and `chosen_index` (0–3), **Then** the server calculates `answer_time_ms` server-side, persists the answer in `answers`, dispatches `BroadcastAnswerCountUpdated`, and returns HTTP 201 with `{ answer_id, chosen_index, answered_at }`.
2. **Given** a question whose timer has expired (`now() - asked_at > time_per_question_s`) or that has already been revealed (`revealed_at IS NOT NULL`), **When** a player submits an answer, **Then** the server rejects the request with HTTP 409 (`QUESTION_CLOSED`).
3. **Given** a player who has already submitted an answer for `session_question_id`, **When** the player submits again for the same question, **Then** the server returns HTTP 200 with the previously saved answer (idempotent behavior) without modifying the original submission or timestamp.
4. **Given** an authenticated player belonging to Session A, **When** attempting to submit an answer for a question in Session B or with a URL mismatch (`:session_id != token.session_id`), **Then** the server rejects the request with HTTP 403 (`FORBIDDEN`).
5. **Given** a question that does not belong to the session or does not exist, **When** submitting an answer, **Then** the server returns HTTP 404 (`NOT_FOUND`).
6. **Given** an invalid `chosen_index` (e.g. < 0 or > 3), **When** submitting an answer, **Then** the server returns HTTP 422 (`VALIDATION_ERROR`).

---

### User Story 3 — Real-Time Participation Event Hooks (Priority: P2)

As a backend developer and frontend consumer, I want player actions (joining lobby, submitting answers) to emit typed broadcaster event hooks (`BroadcastPlayerJoined`, `BroadcastAnswerCountUpdated`) so that Presenter, Projection Screen, and Player waiting rooms receive instantaneous live updates.

**Why this priority**: Decoupled event hooks ensure WebSocket hubs can be connected in subsequent features without refactoring HTTP handlers or repository interactions.

**Independent Test**: Invoke player join and answer submission endpoints against a mocked `SessionEventBroadcaster` and assert that the correct methods and payload parameters are executed.

**Acceptance Scenarios**:

1. **Given** a successful player join, **When** the player record is inserted, **Then** `broadcaster.BroadcastPlayerJoined(sessionID, playerID, nickname, avatarColor, totalPlayers)` is called.
2. **Given** a successful answer submission, **When** the answer record is inserted, **Then** `broadcaster.BroadcastAnswerCountUpdated(sessionID, sessionQuestionID, answeredCount, totalPlayers)` is called.
3. **Given** an idempotent answer resubmission (already answered), **When** HTTP 200 is returned, **Then** `BroadcastAnswerCountUpdated` is NOT re-emitted with duplicate count.

---

### Edge Cases

- **Nickname Race Condition**: Two players submit the same nickname for the same session at the exact same millisecond. The database unique constraint `(session_id, nickname)` rejects the second transaction, and the service converts the error into a clean HTTP 409 `NICKNAME_TAKEN`.
- **Clock Drift & Latency**: `answer_time_ms` is strictly calculated server-side as `answered_at - asked_at`. If `answered_at < asked_at` due to microsecond clock adjustments, `answer_time_ms` is clamped to `0`. If `answered_at > asked_at + time_per_question_s`, it is rejected with HTTP 409 `QUESTION_CLOSED`.
- **Token Tampering / Cross-Session Access**: A player token created for Session A cannot be used to submit answers for Session B. The middleware and handler cross-validate `token.session_id == url.session_id == question.session_id`.
- **Rapid Double Clicks / Network Retries**: Mobile players repeatedly tapping an answer button receive the initial response on first click (201 Created) and the same answer on subsequent retries (200 OK) without creating duplicate rows.
- **Session State Transitions During Join**: If the admin clicks "Launch" (`lobby -> active`) while a player is submitting the join form, the join request fails with HTTP 409 `SESSION_NOT_IN_LOBBY` to avoid orphaned lobby joins.

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: System MUST provide a public endpoint `POST /api/v1/sessions/:id/join` requiring no initial authentication.
- **FR-002**: `POST /api/v1/sessions/:id/join` MUST validate that the request body contains a 6-digit numeric `pin` and a `nickname` between 2 and 20 characters.
- **FR-003**: System MUST verify that the target session exists and has `status = 'lobby'`. If the session is in any other status (`draft`, `active`, `completed`, `cancelled`), system MUST return HTTP 409 with code `SESSION_NOT_IN_LOBBY`.
- **FR-004**: System MUST verify that the provided `pin` matches the session's active PIN. If invalid, system MUST return HTTP 404 with code `INVALID_PIN`.
- **FR-005**: System MUST assign a deterministic or visually distinct hex `avatar_color` for each joined player (e.g. from a curated palette of vibrant accessible colors).
- **FR-006**: System MUST persist the player in `players` table (`id`, `session_id`, `nickname`, `avatar_color`, `total_score = 0`, `joined_at = now()`).
- **FR-007**: System MUST enforce nickname uniqueness per session; if `(session_id, nickname)` already exists, system MUST return HTTP 409 with code `NICKNAME_TAKEN`.
- **FR-008**: System MUST generate and return a signed JWT `player_token` with claims `player_id`, `session_id`, and `role = "player"`, with a TTL of 4 hours.
- **FR-009**: On successful player join, system MUST dispatch `BroadcastPlayerJoined` via `SessionEventBroadcaster` with `player_id`, `nickname`, `avatar_color`, and updated `total_players` count.
- **FR-010**: `POST /api/v1/sessions/:id/join` response MUST return HTTP 201 with payload `{ player_id, nickname, avatar_color, session_id, session_name, player_token, token_expires_at }` wrapped in the standard `{ "data": ... }` envelope.
- **FR-011**: System MUST provide endpoint `POST /api/v1/sessions/:session_id/answers` protected by Player JWT authentication.
- **FR-012**: The player authentication middleware/guard MUST validate that the token carries role `"player"` and inject `player_id` and `session_id` into the request context.
- **FR-013**: System MUST reject `POST /api/v1/sessions/:session_id/answers` with HTTP 403 `FORBIDDEN` if the token's `session_id` does not match the URL parameter `:session_id`.
- **FR-014**: `POST /api/v1/sessions/:session_id/answers` MUST validate that `chosen_index` is an integer between 0 and 3 and `session_question_id` is a valid UUID.
- **FR-015**: System MUST verify that `session_question_id` belongs to the specified `session_id`, has been asked (`asked_at IS NOT NULL`), and has not yet been revealed (`revealed_at IS NULL`).
- **FR-016**: System MUST compute remaining time based on `session_questions.asked_at` and `sessions.time_per_question_s`. If `now() > asked_at + time_per_question_s`, system MUST reject the answer with HTTP 409 code `QUESTION_CLOSED`.
- **FR-017**: If the player has already submitted an answer for `session_question_id`, system MUST return HTTP 200 with the existing answer data (`answer_id`, `chosen_index`, `answered_at`) without modifying database state or re-counting the answer.
- **FR-018**: On a first valid submission, system MUST calculate `answer_time_ms = now() - asked_at` (clamped >= 0) and insert a new row in `answers` with `is_correct = false` and `points_awarded = 0` (to be evaluated later during reveal).
- **FR-019**: On successful answer insertion, system MUST dispatch `BroadcastAnswerCountUpdated` via `SessionEventBroadcaster` with `session_id`, `session_question_id`, `answered_count`, and `total_players`.
- **FR-020**: `POST /api/v1/sessions/:session_id/answers` MUST return HTTP 201 with `{ answer_id, chosen_index, answered_at }` wrapped in the standard `{ "data": ... }` envelope.
- **FR-021**: All error responses MUST strictly follow the standard format `{ "error": { "code": "...", "message": "..." } }`.

### Key Entities

- **Player**: Ephemeral participant record in `players` table associated with a `session_id`, carrying `nickname`, `avatar_color`, `total_score`, and `joined_at`.
- **Answer**: Player response in `answers` table linking `player_id` and `session_question_id`, storing `chosen_index`, `answer_time_ms`, and `answered_at`.
- **Session**: Parent game entity defining `status` (`lobby` required for joins, `active` for answers), `pin`, and `time_per_question_s`.
- **SessionQuestion**: Snapshot question item holding `asked_at` and `revealed_at` timestamps determining active question window.

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: Player join endpoint (`POST /sessions/:id/join`) responds in under 50ms under normal load.
- **SC-002**: Answer submission endpoint (`POST /sessions/:session_id/answers`) records responses in under 50ms.
- **SC-003**: 100% of duplicate nickname attempts in the same session are rejected with HTTP 409 `NICKNAME_TAKEN`.
- **SC-004**: 100% of expired timer submissions are rejected with HTTP 409 `QUESTION_CLOSED`.
- **SC-005**: 100% of repeated submissions for the same question return HTTP 200 with identical answer data.
- **SC-006**: Unit and integration test coverage for player join, token issuance, answer validation, idempotency, and timer boundaries exceeds 90%.

## Assumptions

- Player identities are ephemeral and do not persist across multiple distinct sessions in MVP (ADR-001).
- Avatars are generated automatically on the backend as distinct hex color strings (e.g. from a vibrant 12-color palette) to avoid requiring avatar uploads in MVP.
- Scoring calculation (`is_correct`, `points_awarded`, and `players.total_score` update) occurs strictly when the presenter triggers `POST /reveal` (Feature #7), keeping the answer submission hot path minimal and fast.
- WebSocket distribution is decoupled via `SessionEventBroadcaster` and uses the no-op implementation until WebSocket hub features are wired up.
