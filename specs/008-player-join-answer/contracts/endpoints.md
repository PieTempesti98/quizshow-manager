# API Contracts: Feature #8 — Player Join & Answer Submission

## Base URL
`/api/v1`

---

## 1. Join Session (Public)

### `POST /api/v1/sessions/:id/join`

Registers an ephemeral player in a session lobby, assigns an avatar color, and issues a 4-hour Player JWT.

- **Authentication**: None (Public)
- **Path Parameters**:
  - `id` (`string`, UUID): Target session UUID.

#### Request Body
```json
{
  "pin": "482910",
  "nickname": "Mario"
}
```

#### Validation Rules
- `pin`: Exactly 6 numeric characters. Must match session's active PIN.
- `nickname`: String, 2 to 20 characters, non-empty, stripped of leading/trailing whitespace. Must be unique within the session.

#### Response 201 Created
```json
{
  "data": {
    "player_id": "8fa84381-8153-4f24-ba25-9c9c1071221b",
    "nickname": "Mario",
    "avatar_color": "#E85D24",
    "session_id": "b3e0c0fa-d475-4d0d-9e0c-99cfae5da98e",
    "session_name": "Quiz aziendale Q2",
    "player_token": "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9...",
    "token_expires_at": "2026-08-14T04:15:00Z"
  }
}
```

#### Error Responses
- `404 NOT_FOUND` / `INVALID_PIN`:
  ```json
  { "error": { "code": "INVALID_PIN", "message": "Session not found or PIN does not match" } }
  ```
- `409 SESSION_NOT_IN_LOBBY`:
  ```json
  { "error": { "code": "SESSION_NOT_IN_LOBBY", "message": "Session is not open for player join" } }
  ```
- `409 NICKNAME_TAKEN` / `CONFLICT`:
  ```json
  { "error": { "code": "NICKNAME_TAKEN", "message": "Nickname already taken in this session" } }
  ```
- `422 VALIDATION_ERROR`:
  ```json
  { "error": { "code": "VALIDATION_ERROR", "message": "Nickname must be between 2 and 20 characters" } }
  ```

---

## 2. Submit Answer (Player Protected)

### `POST /api/v1/sessions/:session_id/answers`

Submits a player's answer for the currently active question. Validates timer and provides idempotent handling for repeat submissions.

- **Authentication**: `Authorization: Bearer <player_jwt>` (Requires `role: "player"`)
- **Path Parameters**:
  - `session_id` (`string`, UUID): Session UUID. Must match the `session_id` in the JWT claims.

#### Request Body
```json
{
  "session_question_id": "76df529c-6020-4e3f-b8eb-9dbe637ff007",
  "chosen_index": 2
}
```

#### Validation Rules
- `session_question_id`: Valid UUID string. Must belong to the session and be the active asked question.
- `chosen_index`: Integer between 0 and 3 inclusive.

#### Response 201 Created (First Submission)
```json
{
  "data": {
    "answer_id": "52cbeec4-4e78-43bb-81ef-eb569a910ecb",
    "chosen_index": 2,
    "answered_at": "2026-08-14T00:15:08Z"
  }
}
```

#### Response 200 OK (Idempotent Resubmission)
```json
{
  "data": {
    "answer_id": "52cbeec4-4e78-43bb-81ef-eb569a910ecb",
    "chosen_index": 2,
    "answered_at": "2026-08-14T00:15:08Z"
  }
}
```

#### Error Responses
- `401 UNAUTHORIZED`:
  ```json
  { "error": { "code": "UNAUTHORIZED", "message": "Missing or invalid player token" } }
  ```
- `403 FORBIDDEN`:
  ```json
  { "error": { "code": "FORBIDDEN", "message": "Player token not valid for this session" } }
  ```
- `404 NOT_FOUND`:
  ```json
  { "error": { "code": "NOT_FOUND", "message": "Question not found in this session" } }
  ```
- `409 QUESTION_CLOSED`:
  ```json
  { "error": { "code": "QUESTION_CLOSED", "message": "Timer has expired or question has been revealed" } }
  ```
- `422 VALIDATION_ERROR`:
  ```json
  { "error": { "code": "VALIDATION_ERROR", "message": "chosen_index must be between 0 and 3" } }
  ```

---

## 3. Session Broadcaster Event Contract (Internal Hooks)

| Method | Parameters | Description |
|---|---|---|
| `BroadcastPlayerJoined` | `sessionID string, playerID string, nickname string, avatarColor string, totalPlayers int` | Dispatched to presenter/projection when a player joins lobby |
| `BroadcastAnswerCountUpdated` | `sessionID string, sessionQuestionID string, answeredCount int, totalPlayers int` | Dispatched to presenter/projection when an answer is received |
