# Research: Feature #8 — Player Join & Answer Submission

## Research Questions & Architectural Decisions

### 1. Player Authentication & Ephemeral Identity

- **Decision**: Issue a signed JWT `player_token` with claims `{ player_id: UUID, session_id: UUID, role: "player" }` and 4-hour TTL upon successful join.
- **Rationale**: In accordance with ADR-001 (Ephemeral Player), players do not have persistent accounts in MVP. A stateless JWT allows the player client to authenticate subsequent HTTP REST actions (`POST /api/v1/sessions/:session_id/answers`) and WebSocket connections (`/ws/sessions/:id?token=...`) without database session lookup per request.
- **Alternatives considered**:
  - Session cookies: Rejected because mobile clients and WebSocket connections in future apps handle Bearer tokens more cleanly across domains without CSRF vulnerabilities.
  - Opaque tokens in DB/Redis: Rejected to avoid unnecessary stateful lookup overhead on high-frequency live events.

### 2. Avatar Generation Strategy

- **Decision**: Assign a vibrant hex color on join chosen deterministically or cyclically from a curated accessible palette (e.g. 12 distinct high-contrast colors).
- **Rationale**: ADR-001 specifies no user account or image upload for MVP. Initials or simple avatars with vivid colors (`#E85D24`, `#3B8BD4`, `#7B2CBF`, `#2A9D8F`, etc.) provide instant visual identity on Presenter and Projection views without external asset storage.
- **Alternatives considered**:
  - User-selected avatar colors: Increases join friction on mobile join form.
  - Image generation: Adds unnecessary latency and complexity during join spikes.

### 3. Server-Side Timer & Answer Time Enforcement

- **Decision**: Answer validity and timing (`answer_time_ms`) are strictly evaluated on the server:
  - `time_limit_ms = session.time_per_question_s * 1000`
  - `elapsed_ms = now.Sub(session_question.asked_at).Milliseconds()`
  - If `elapsed_ms > time_limit_ms` or `revealed_at IS NOT NULL`: return HTTP 409 `QUESTION_CLOSED`.
  - `answer_time_ms = max(0, int(elapsed_ms))`
- **Rationale**: Prevents client clock tampering. All participants are judged against the authoritative server timestamp. If the timer was paused and resumed via Feature #7, `asked_at` was already shifted forward by the pause duration, ensuring `elapsed_ms` remains completely accurate without extra branching logic.
- **Alternatives considered**:
  - Client-reported answer time: Insecure and susceptible to spoofing.

### 4. Idempotency on Answer Submission

- **Decision**: Handle duplicate answer submissions by checking for existing records on `(player_id, session_question_id)`. If an answer is already present, return HTTP 200 with the original answer without re-recording or updating values.
- **Rationale**: Mobile players frequently experience flaky connections or double-tap buttons. Idempotent 200 OK prevents double-voting while providing a seamless UX.
- **Alternatives considered**:
  - Returning HTTP 409 CONFLICT on resubmission: Creates client-side error handling complexity on benign retry attempts.

### 5. Decoupled WebSocket Event Distribution

- **Decision**: Extend `SessionEventBroadcaster` with:
  - `BroadcastPlayerJoined(sessionID string, playerID string, nickname string, avatarColor string, totalPlayers int)`
  - `BroadcastAnswerCountUpdated(sessionID string, sessionQuestionID string, answeredCount int, totalPlayers int)`
- **Rationale**: Keeps REST domain services clean and completely decoupled from WebSocket transport logic. Allows unit testing and stubbing until Feature #10 activates the real hub.
