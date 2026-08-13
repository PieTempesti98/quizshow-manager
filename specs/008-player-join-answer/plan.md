# Implementation Plan: Feature #8 — Player Join & Answer Submission

**Branch**: `008-player-join-answer` | **Date**: 2026-08-14 | **Spec**: [spec.md](spec.md)  
**Input**: Feature specification from `/specs/008-player-join-answer/spec.md`

## Summary

Implement the backend REST endpoints and domain logic for ephemeral player participation during quiz sessions:

1. `POST /api/v1/sessions/:id/join` (Public, no auth):
   - Validates 6-digit numeric PIN and unique nickname (2–20 chars).
   - Verifies session is in `lobby` status (rejects draft/active/completed with 409 `SESSION_NOT_IN_LOBBY`).
   - Assigns a vibrant hex `avatar_color` from a curated palette.
   - Inserts record into `players` table, handling nickname collisions with HTTP 409 `NICKNAME_TAKEN`.
   - Generates and signs a Player JWT (claims `{ player_id, session_id, role: "player" }`, TTL 4h).
   - Dispatches `BroadcastPlayerJoined` event via `SessionEventBroadcaster`.
   - Returns HTTP 201 with player details and token.

2. `POST /api/v1/sessions/:session_id/answers` (Player JWT protected):
   - Authenticated via `RequirePlayer` middleware extracting `player_id` and `session_id`.
   - Rejects token/URL mismatch with HTTP 403 `FORBIDDEN`.
   - Validates question belongs to active session, has been asked, and is not yet revealed.
   - Calculates server-side elapsed time: rejects expired submissions with HTTP 409 `QUESTION_CLOSED`.
   - Handles resubmission idempotently: returns HTTP 200 with existing answer if already submitted.
   - Computes `answer_time_ms` server-side and persists answer row (with `is_correct` and `points_awarded` left for reveal).
   - Dispatches `BroadcastAnswerCountUpdated` event via `SessionEventBroadcaster`.
   - Returns HTTP 201 with answer ID and timestamp.

---

## Technical Context

**Language/Version**: Go 1.25  
**Primary Dependencies**: `gofiber/fiber` v2, `golang-jwt/jwt` v5, `jackc/pgx` v5, `google/uuid` v1  
**Storage**: PostgreSQL — tables `players`, `answers`, `sessions`, `session_questions` (schema `001_initial_schema.up.sql`)  
**Testing**: Go `testing` package with unit tests for timer/answer calculations and integration tests for join/answers  
**Target Platform**: Linux server (Docker Compose)  
**Project Type**: Web service (REST API)  
**Performance Goals**: Join < 50ms, Submit answer < 50ms  
**Constraints**: Ephemeral player model (ADR-001), REST for actions (ADR-005), standard API response envelope, decoupled WebSocket broadcaster  
**Scale/Scope**: MVP — up to 100 concurrent players per session, 50 questions per session.

---

## Constitution Check

### Principle 1.1 & 1.2 — Spec-First & MVP Discipline ✅
- Feature spec exists at [specs/008-player-join-answer/spec.md](spec.md).
- Implements only MVP user stories US-PL01 and US-PL03. Persistent accounts (R2+) and team modes are deferred.

### Principle 2.1 & 2.2 — Package Structure & No Global State ✅
- Auth extensions in `internal/auth/` (Player JWT issue & `RequirePlayer` middleware).
- Player domain logic in `internal/session/` (extending repository, service, and handlers).
- Dependencies injected explicitly via constructors.

### Principle 2.3 & 2.4 — Database Access & Migrations ✅
- Direct `pgx` queries with explicit SQL.
- All tables (`players`, `answers`, `sessions`, `session_questions`) exist in `001_initial_schema.up.sql`. No migration needed.

### Principle 4.3 & 5.2–5.4 — API Envelope, UUIDs, UTC & No Business Logic in SQL ✅
- Strict envelope `{ "data": ... }` and `{ "error": ... }`.
- UUID v4 for all entity IDs; timestamps in UTC.
- Timer checks and answer time calculations performed in Go.

### Principle 6.1 — Unit Tests for Business Logic ✅
- Unit tests for answer timing, validation, and token claims in `internal/session/` and `internal/auth/`.

**Gate result**: PASS — proceed to implementation.

---

## Project Structure

### Documentation (this feature)

```text
specs/008-player-join-answer/
├── plan.md              ← this file
├── spec.md
├── research.md          ← Phase 0 output
├── data-model.md        ← Phase 1 output
├── quickstart.md        ← Phase 1 output
├── contracts/
│   └── endpoints.md     ← Phase 1 output
└── checklists/
    └── requirements.md
```

### Source Code

```text
backend/
├── cmd/server/main.go                          ← Register public join route and protected player answers route
└── internal/
    ├── auth/
    │   ├── token.go                            ← PlayerClaims, IssuePlayerToken, ValidatePlayerClaims
    │   └── middleware.go                       ← RequirePlayer middleware injecting PlayerClaims
    └── session/
        ├── avatar.go                           ← NEW: Hex color palette and avatar assignment helper
        ├── models.go                           ← Player/Answer models, results, errors, broadcaster hooks
        ├── repository.go                       ← JoinPlayer, SubmitAnswer repo methods with constraints & counts
        ├── service.go                          ← Join, SubmitAnswer service methods with timer logic & events
        ├── handler.go                          ← Join, SubmitAnswer HTTP handlers with input validation
        └── player_test.go                      ← NEW: Unit tests for player join & answer submission
```

---

## Complexity Tracking

No constitution violations. No complexity justification required.

---

## Implementation Steps

### Step 1 — Auth Extensions (`internal/auth/token.go` & `internal/auth/middleware.go`)
- In `internal/auth/token.go`:
  - Define `PlayerClaims` struct with `PlayerID uuid.UUID`, `SessionID uuid.UUID`, `Role string`, `Issuer string`.
  - Define `playerClaimsContextKey{}` and `PlayerClaimsKey`.
  - Implement `IssuePlayerToken(playerID uuid.UUID, sessionID uuid.UUID, cfg Config) (string, time.Time, error)` with 4-hour TTL and role `"player"`.
  - Implement `ValidatePlayerClaims(tokenString string, cfg Config) (PlayerClaims, error)`.
- In `internal/auth/middleware.go`:
  - Implement `RequirePlayer(cfg Config) fiber.Handler` that extracts Bearer token, validates claims, ensures `role == "player"`, and sets `c.Locals(PlayerClaimsKey, claims)`.

### Step 2 — Avatar Color Palette (`internal/session/avatar.go`)
- Create curated list of 12 vibrant high-contrast hex colors:
  `{"#E85D24", "#3B8BD4", "#7B2CBF", "#2A9D8F", "#E76F51", "#264653", "#F4A261", "#E63946", "#457B9D", "#1D3557", "#06D6A0", "#118AB2"}`.
- Implement `AssignAvatarColor(nickname string) string` (deterministic modulo hash or random).

### Step 3 — Extend Models, Sentinel Errors & Broadcaster (`internal/session/models.go`)
- Define Models & DTOs:
  - `Player`, `Answer`, `PlayerJoinResult`, `AnswerSubmitResult`.
- Define Sentinel Errors:
  - `ErrNicknameTaken`, `ErrInvalidPIN`, `ErrQuestionClosed`, `ErrQuestionNotFound`, `ErrForbiddenSession`.
- Expand `SessionEventBroadcaster` interface:
  - `BroadcastPlayerJoined(sessionID string, playerID string, nickname string, avatarColor string, totalPlayers int)`
  - `BroadcastAnswerCountUpdated(sessionID string, sessionQuestionID string, answeredCount int, totalPlayers int)`
- Update `noopBroadcaster` to implement the new methods.

### Step 4 — Implement Repository Methods (`internal/session/repository.go`)
- `JoinPlayer(ctx context.Context, sessionID uuid.UUID, pin string, nickname string, avatarColor string) (Player, Session, int, error)`:
  - Verifies session exists, `deleted_at IS NULL`, status is `lobby` (`ErrSessionNotInLobby`), and `session.PIN == pin` (`ErrInvalidPIN`).
  - Inserts new row into `players (id, session_id, nickname, avatar_color, total_score, joined_at)`.
  - Catches PostgreSQL unique violation (code `23505`) on `(session_id, nickname)` -> returns `ErrNicknameTaken`.
  - Counts total players for the session and returns `(player, session, totalPlayers, nil)`.
- `SubmitAnswer(ctx context.Context, sessionID uuid.UUID, playerID uuid.UUID, sessionQuestionID uuid.UUID, chosenIndex int16, now time.Time) (Answer, bool, int, int, error)`:
  - Queries existing answer for `(player_id, session_question_id)`.
  - If existing answer is found: returns `(existingAnswer, true, answeredCount, totalPlayers, nil)` without re-inserting (Idempotent 200 OK).
  - Validates session is `active`, question belongs to session, `asked_at IS NOT NULL`, and `revealed_at IS NULL`.
  - Calculates `elapsedMs = now.Sub(*sq.asked_at).Milliseconds()`.
  - If `elapsedMs > session.time_per_question_s * 1000`: returns `ErrQuestionClosed`.
  - Inserts new row into `answers` with `is_correct = false`, `points_awarded = 0`, `answer_time_ms = max(0, int(elapsedMs))`.
  - Handles concurrent insert conflict on `(player_id, session_question_id)` gracefully.
  - Queries `answeredCount` and `totalPlayers`, returning `(newAnswer, false, answeredCount, totalPlayers, nil)`.

### Step 5 — Implement Service Methods (`internal/session/service.go`)
- Implement `Join(ctx context.Context, sessionID uuid.UUID, pin string, nickname string) (PlayerJoinResult, error)`:
  - Validates PIN & nickname format.
  - Generates avatar color.
  - Calls repository `JoinPlayer`.
  - Issues player JWT token via `auth.IssuePlayerToken`.
  - Dispatches `broadcaster.BroadcastPlayerJoined(...)`.
  - Returns `PlayerJoinResult`.
- Implement `SubmitAnswer(ctx context.Context, sessionID uuid.UUID, playerID uuid.UUID, sessionQuestionID uuid.UUID, chosenIndex int16) (AnswerSubmitResult, error)`:
  - Calls repository `SubmitAnswer`.
  - If not duplicate (`isDuplicate == false`): dispatches `broadcaster.BroadcastAnswerCountUpdated(...)`.
  - Returns `AnswerSubmitResult`.

### Step 6 — Implement Handlers & Routing (`internal/session/handler.go` & `cmd/server/main.go`)
- In `internal/session/handler.go`:
  - `Join`: Parses `session_id` from URL param `:id`, decodes JSON body `{pin, nickname}`, calls service, maps domain errors (404 `INVALID_PIN`, 409 `SESSION_NOT_IN_LOBBY`, 409 `NICKNAME_TAKEN`, 422 `VALIDATION_ERROR`), returns HTTP 201.
  - `SubmitAnswer`: Parses `session_id` from URL param `:session_id`, extracts `PlayerClaims` from context, verifies `claims.SessionID == sessionID` (HTTP 403 `FORBIDDEN`), decodes JSON body `{session_question_id, chosen_index}`, calls service, maps domain errors (404 `NOT_FOUND`, 409 `QUESTION_CLOSED`, 422 `VALIDATION_ERROR`), returns HTTP 201 for new answer or HTTP 200 for idempotent duplicate.
- In `cmd/server/main.go`:
  - Register public route: `v1.Post("/sessions/:id/join", sessionHandler.Join)`.
  - Register protected player route:
    ```go
    playerSession := v1.Group("/sessions/:session_id", auth.RequirePlayer(cfg.Auth))
    playerSession.Post("/answers", sessionHandler.SubmitAnswer)
    ```

### Step 7 — Unit & Integration Testing (`internal/session/player_test.go`)
- Test avatar assignment determinism / format.
- Test player token issuance and validation.
- Test join input validation and error mappings.
- Test answer submission timer calculation, expiration edge cases, and idempotency logic.

---

## Verification & Acceptance Testing

1. Run automated unit tests: `go test -v ./internal/session/... ./internal/auth/...`
2. Perform smoke test sequence as outlined in [quickstart.md](quickstart.md).
