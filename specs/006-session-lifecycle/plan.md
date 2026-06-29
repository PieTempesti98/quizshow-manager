# Implementation Plan: Session Lifecycle — draft → lobby → active

**Branch**: `006-session-lifecycle` | **Date**: 2026-06-29 | **Spec**: [spec.md](spec.md)  
**Input**: Feature specification from `/specs/006-session-lifecycle/spec.md`

## Summary

Implement three new REST endpoints to drive the session state machine from `draft` → `lobby` → `active`:

1. `POST /sessions/:id/open-lobby` — transition draft → lobby (activates the PIN)
2. `GET /sessions/:id/qr` — serve a PNG QR code encoding the player join URL
3. `POST /sessions/:id/launch` — transition lobby → active: randomly draw questions into `session_questions`, set `started_at`, issue a 12-hour projection JWT

All work extends the existing `backend/internal/session/` package (4-file layout: models, repository, service, handler). No new DB migration is needed — `session_questions` already exists from migration 001. One new library (`github.com/skip2/go-qrcode`) is required. The existing `internal/auth/token.go` is extended with `IssueProjectionToken`.

---

## Technical Context

**Language/Version**: Go 1.25  
**Primary Dependencies**: gofiber/fiber v2, golang-jwt/jwt v5, jackc/pgx v5, google/uuid v1, `github.com/skip2/go-qrcode` (new)  
**Storage**: PostgreSQL — `sessions`, `session_categories`, `session_questions` (migration 001, no new migration)  
**Testing**: Manual smoke tests (no automated test suite in this project yet)  
**Target Platform**: Linux server (Docker Compose)  
**Project Type**: Web service (REST API)  
**Performance Goals**: Open-lobby < 500ms, Launch < 1s for pools up to 500 questions, QR < 300ms  
**Constraints**: No ORM (pgx direct), no new migrations, no WebSocket in this feature, no frontend  
**Scale/Scope**: MVP — single admin, up to 50 questions per session

---

## Constitution Check

### Principle I — Backend-First Development ✅
This feature is backend-only. `docs/06-ui-flows.md` does not exist; no frontend work is included or implied.

### Principle II — Spec-Driven Development ✅
Spec file exists at `specs/006-session-lifecycle/spec.md`. Plan follows spec.

### Principle III — Architectural Simplicity (YAGNI) ✅
- QR PNG generated server-side using `skip2/go-qrcode` — minimal dependency, no render service.
- `SessionEventBroadcaster` is a one-method interface with a no-op implementation — no infrastructure added.
- No message broker, no Redis, no async workers.
- `IssueProjectionToken` added to existing `auth/token.go` — no new package.

### Principle IV — Data Integrity Standards ✅
- All queries include `WHERE deleted_at IS NULL` where applicable.
- Timestamps UTC via `TIMESTAMPTZ`.
- IDs are UUID v4.
- All responses use the `{ "data": ... }` / `{ "error": ... }` envelope.
- Launch transaction uses `FOR UPDATE` to prevent concurrent double-launch.

### Principle V — Real-Time Isolation ✅
- WebSocket is explicitly deferred to feature #10.
- The `SessionEventBroadcaster` no-op implementation separates the REST endpoint from the future WebSocket hub cleanly.
- No cross-session state.

**Gate result**: PASS — proceed to implementation.

---

## Project Structure

### Documentation (this feature)

```text
specs/006-session-lifecycle/
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
├── go.mod                                  ← add github.com/skip2/go-qrcode
├── cmd/server/main.go                      ← update session.NewService() call + 3 new routes + PLAYER_APP_BASE_URL env read
└── internal/
    ├── auth/
    │   └── token.go                        ← add IssueProjectionToken + projectionClaims
    └── session/
        ├── models.go                       ← add ErrSessionNotInLobby, ErrInsufficientQuestions, OpenLobbyResult, LaunchResult, SessionEventBroadcaster
        ├── repository.go                   ← add OpenLobby, Launch (transactional) to interface + impl
        ├── service.go                      ← add OpenLobby, GetQR, Launch methods; update service struct + constructor
        └── handler.go                      ← add OpenLobby, GetQR, Launch handlers
```

---

## Complexity Tracking

No constitution violations. No complexity justification required.

---

## Implementation Steps

### Step 1 — Add QR code dependency
`cd backend && go get github.com/skip2/go-qrcode`

### Step 2 — Extend `internal/auth/token.go`

Add (do not modify existing functions):

```go
// projectionClaims is the JWT payload for a projection screen token.
type projectionClaims struct {
    SessionID string `json:"session_id"`
    Role      string `json:"role"`
    jwt.RegisteredClaims
}

// IssueProjectionToken issues a 12-hour JWT for the projection screen of a specific session.
// PLACEHOLDER: projection frontend does not exist yet; token is issued to prepare for feature #10.
func IssueProjectionToken(sessionID uuid.UUID, cfg Config) (string, error) {
    now := time.Now().UTC()
    claims := projectionClaims{
        SessionID: sessionID.String(),
        Role:      "projection",
        RegisteredClaims: jwt.RegisteredClaims{
            Issuer:    cfg.JWTIssuer,
            Subject:   sessionID.String(),
            ExpiresAt: jwt.NewNumericDate(now.Add(12 * time.Hour)),
            IssuedAt:  jwt.NewNumericDate(now),
        },
    }
    token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
    signed, err := token.SignedString(cfg.JWTSecret)
    if err != nil {
        return "", fmt.Errorf("issue projection token: %w", err)
    }
    return signed, nil
}
```

### Step 3 — Update `internal/session/models.go`

Add sentinel errors, result types, and the broadcaster interface:

```go
// New sentinel errors
var (
    ErrSessionNotFound       = errors.New("session not found")   // already exists
    ErrSessionNotDraft       = errors.New("session is not in draft status")  // already exists
    ErrSessionNotInLobby     = errors.New("session is not in lobby status")
    ErrInsufficientQuestions = errors.New("no questions available in the configured categories")
)

// OpenLobbyResult is returned by Service.OpenLobby.
type OpenLobbyResult struct {
    SessionID uuid.UUID
    PIN       string
    QRCodeURL string
    Status    string
}

// LaunchResult is returned by Service.Launch.
type LaunchResult struct {
    SessionID       uuid.UUID
    Status          string
    QuestionCount   int
    ProjectionToken string
    ProjectionURL   string
    StartedAt       time.Time
}

// SessionEventBroadcaster notifies connected clients of session state changes.
// NoopBroadcaster is used until the WebSocket hub is implemented in feature #10.
type SessionEventBroadcaster interface {
    BroadcastSessionStarted(sessionID string, totalQuestions int)
}

type noopBroadcaster struct{}

func (noopBroadcaster) BroadcastSessionStarted(string, int) {}

// NewNoopBroadcaster returns a SessionEventBroadcaster that does nothing.
func NewNoopBroadcaster() SessionEventBroadcaster { return noopBroadcaster{} }
```

### Step 4 — Update `internal/session/repository.go`

Extend `SessionRepo` interface:
```go
OpenLobby(ctx context.Context, id uuid.UUID) (Session, error)
Launch(ctx context.Context, id uuid.UUID) (Session, int, error)
```

Implement `OpenLobby`:
- `UPDATE sessions SET status = 'lobby', updated_at = now() WHERE id = $1 AND status = 'draft' AND deleted_at IS NULL RETURNING ...`
- 0 rows affected → check existence; if not found → `ErrSessionNotFound`, else → `ErrSessionNotDraft`

Implement `Launch` (single transaction):
1. `BEGIN`
2. `SELECT status::text, question_count FROM sessions WHERE id = $1 AND deleted_at IS NULL FOR UPDATE`
   - no rows → `ErrSessionNotFound`
   - status != 'lobby' → `ErrSessionNotInLobby`
3. Draw questions: `SELECT q.id FROM questions q JOIN session_categories sc ON sc.category_id = q.category_id WHERE sc.session_id = $1 AND q.deleted_at IS NULL ORDER BY RANDOM() LIMIT $2`
   - empty result → `ErrInsufficientQuestions`, rollback
4. Insert `session_questions` rows one by one (position 1..N)
5. `UPDATE sessions SET status = 'active', started_at = now(), updated_at = now() WHERE id = $1 RETURNING ...`
6. `COMMIT`
7. Return `(Session, drawnCount, nil)`

### Step 5 — Update `internal/session/service.go`

Extend `Service` interface:
```go
OpenLobby(ctx context.Context, id uuid.UUID) (OpenLobbyResult, error)
GetQR(ctx context.Context, id uuid.UUID) ([]byte, error)
Launch(ctx context.Context, id uuid.UUID) (LaunchResult, error)
```

Update `service` struct:
```go
type service struct {
    repo        SessionRepo
    authCfg     auth.Config
    playerURL   string   // PLAYER_APP_BASE_URL; placeholder until frontend is deployed
    broadcaster SessionEventBroadcaster
}

func NewService(repo SessionRepo, authCfg auth.Config, playerURL string, broadcaster SessionEventBroadcaster) Service {
    return &service{repo: repo, authCfg: authCfg, playerURL: playerURL, broadcaster: broadcaster}
}
```

Implement `OpenLobby`:
```
sess, err := s.repo.OpenLobby(ctx, id)
// propagate ErrSessionNotFound, ErrSessionNotDraft
return OpenLobbyResult{
    SessionID: sess.ID,
    PIN:       sess.PIN,
    QRCodeURL: "/api/v1/sessions/" + sess.ID.String() + "/qr",
    Status:    sess.Status,
}, nil
```

Implement `GetQR`:
```
detail, err := s.repo.FindByID(ctx, id)
// propagate ErrSessionNotFound
url := buildURL(s.playerURL, "/join?pin="+detail.PIN)
png, err := qrcode.Encode(url, qrcode.Medium, 256)
return png, err
```

Implement `Launch`:
```
sess, drawn, err := s.repo.Launch(ctx, id)
// propagate ErrSessionNotFound, ErrSessionNotInLobby, ErrInsufficientQuestions
token, err := auth.IssueProjectionToken(sess.ID, s.authCfg)
projURL := buildURL(s.playerURL, "/projection?session="+sess.ID.String()+"&token="+token)
s.broadcaster.BroadcastSessionStarted(sess.ID.String(), drawn)
return LaunchResult{
    SessionID:       sess.ID,
    Status:          sess.Status,
    QuestionCount:   drawn,
    ProjectionToken: token,
    ProjectionURL:   projURL,
    StartedAt:       *sess.StartedAt,
}, nil
```

Helper:
```go
func buildURL(base, path string) string {
    return strings.TrimRight(base, "/") + path
}
```

### Step 6 — Update `internal/session/handler.go`

Add three handler methods:

**`OpenLobby`** (no request body):
- Parse `:id` UUID (422 on bad format)
- Call `h.svc.OpenLobby(ctx, id)`
- Map `ErrSessionNotFound` → 404, `ErrSessionNotDraft` → 409, else 200

**`GetQR`** (no auth):
- Parse `:id` UUID (422 on bad format)
- Call `h.svc.GetQR(ctx, id)`
- Map `ErrSessionNotFound` → 404
- On success: `c.Set("Content-Type", "image/png"); return c.Status(200).Send(png)`

**`Launch`** (no request body):
- Parse `:id` UUID (422 on bad format)
- Call `h.svc.Launch(ctx, id)`
- Map `ErrSessionNotFound` → 404, `ErrSessionNotInLobby` → 409 `SESSION_NOT_IN_LOBBY`, `ErrInsufficientQuestions` → 422 `INSUFFICIENT_QUESTIONS`
- On success: 200 with `LaunchResult` fields

### Step 7 — Update `cmd/server/main.go`

Read env var at startup:
```go
playerURL := os.Getenv("PLAYER_APP_BASE_URL")
if playerURL == "" {
    playerURL = "http://localhost:5173" // PLACEHOLDER: update when player frontend is deployed
}
```

Update service construction:
```go
sessionSvc := session.NewService(sessionRepo, cfg, playerURL, session.NewNoopBroadcaster())
```

Register new routes (after existing session routes):
```go
protected.Post("/sessions/:id/open-lobby", sessionHandler.OpenLobby)
protected.Post("/sessions/:id/launch", sessionHandler.Launch)
// QR is public — no RequireAdmin middleware:
v1.Get("/sessions/:id/qr", sessionHandler.GetQR)
```

Note: `v1.Get("/sessions/:id/qr", ...)` must be registered **before** the protected group catches `/sessions/:id` patterns, or registered explicitly on the public `v1` group (not `protected`). Looking at the existing code, this is fine — `v1.Get(...)` and `protected.Get(...)` are separate Fiber groups.

---

## Smoke Test Plan

After implementation, run against the live Docker DB:

```bash
# 1. Login to get admin token
TOKEN=$(curl -s -X POST http://localhost:3000/api/v1/auth/login \
  -H 'Content-Type: application/json' \
  -d '{"email":"admin@quizshow.local","password":"..."}' | jq -r .data.access_token)

# 2. Create a session (needs category with questions)
SESSION_ID=$(curl -s -X POST http://localhost:3000/api/v1/sessions \
  -H "Authorization: Bearer $TOKEN" \
  -H 'Content-Type: application/json' \
  -d '{"name":"Smoke Test","category_ids":["<cat-id>"],"question_count":5,"time_per_question_s":30}' \
  | jq -r .data.id)

# 3. Open lobby — expect 200 status=lobby
curl -s -X POST http://localhost:3000/api/v1/sessions/$SESSION_ID/open-lobby \
  -H "Authorization: Bearer $TOKEN" | jq .

# 4. Call open-lobby again — expect 409
curl -s -X POST http://localhost:3000/api/v1/sessions/$SESSION_ID/open-lobby \
  -H "Authorization: Bearer $TOKEN" | jq .

# 5. Get QR code (no auth) — expect PNG
curl -s http://localhost:3000/api/v1/sessions/$SESSION_ID/qr -o /tmp/test.png
file /tmp/test.png   # should say "PNG image data"

# 6. Launch — expect 200 status=active, projection_token present
curl -s -X POST http://localhost:3000/api/v1/sessions/$SESSION_ID/launch \
  -H "Authorization: Bearer $TOKEN" | jq .

# 7. Call launch again — expect 409
curl -s -X POST http://localhost:3000/api/v1/sessions/$SESSION_ID/launch \
  -H "Authorization: Bearer $TOKEN" | jq .

# 8. Verify session_questions in DB
# psql: SELECT count(*) FROM session_questions WHERE session_id = '<SESSION_ID>';
```

---

## Out of Scope

- `POST /sessions/:id/next-question`, `/pause-timer`, `/resume-timer`, `/reveal`, `/end` — feature #7
- `POST /sessions/:id/join` — feature #8
- WebSocket `session_started` event broadcast — feature #10
- Frontend (admin, presenter, player) — blocked on `docs/06-ui-flows.md`
- Projection frontend — blocked on `docs/06-ui-flows.md`
