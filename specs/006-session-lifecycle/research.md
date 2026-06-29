# Research: Session Lifecycle (006)

## Decision 1 — QR Code library

**Decision**: Use `github.com/skip2/go-qrcode`  
**Rationale**: Pure Go (no CGO), actively maintained, generates PNG bytes directly from a string, minimal API (`qrcode.Encode(content, qrcode.Medium, 256)` returns `[]byte`). Already used in similar Go quiz/event projects. Size is ~30KB uncompressed.  
**Alternatives considered**:  
- `github.com/boombuler/barcode` — more general barcode library but more complex API for simple QR PNG use case.  
- Rendering QR on the frontend — rejected because the spec explicitly defines `GET /sessions/:id/qr` as a server-side PNG endpoint.

---

## Decision 2 — `PLAYER_APP_BASE_URL` handling

**Decision**: Read from environment variable at startup; default to `http://localhost:5173`. Normalize by trimming trailing slashes before appending paths.  
**Rationale**: The player frontend does not exist yet. The env var makes the URL configurable without code changes when the frontend is deployed. `localhost:5173` is the Vite dev server default port — the natural default for development.  
**Code comment required**: Mark this as a placeholder in both service and the `main.go` read; note it must be updated when the player app is deployed.

---

## Decision 3 — Projection token design

**Decision**: Add `IssueProjectionToken(sessionID uuid.UUID, cfg auth.Config) (string, error)` to `internal/auth/token.go`. Use a new `projectionClaims` struct embedding `jwt.RegisteredClaims` with a `session_id` custom claim and `role: "projection"`. TTL: 12h. Subject: session ID string.  
**Rationale**: Reuses the existing HMAC-SHA256 signing path and `auth.Config.JWTSecret`. No duplication of signing logic. The new struct is unexported (package-private) since it is only needed within the token issuance path. `Subject` doubles as session ID for possible future validation without schema change.  
**Alternatives considered**:  
- Passing a `func(sessionID) string` closure to the service — cleaner decoupling but unnecessary indirection given that `session.Handler` already imports `auth`.  
- Storing the projection token in the DB — rejected (stateless JWT is sufficient; TTL is enforced by signature expiry).

---

## Decision 4 — `SessionEventBroadcaster` interface placement

**Decision**: Define `SessionEventBroadcaster` and `NoopBroadcaster` in `backend/internal/session/models.go`.  
**Rationale**: The interface belongs with the session domain. Feature #10 (WebSocket hub) will provide a real implementation that gets injected at startup in `main.go`. Keeping it in the `session` package avoids circular imports and keeps feature #10's hub cleanly separate.  
**Alternatives considered**:  
- Defining it in a shared `internal/events` package — overkill for one interface; premature abstraction per Constitution Principle III.

---

## Decision 5 — QR endpoint status constraint

**Decision**: The `GET /sessions/:id/qr` endpoint has **no session status constraint**. It returns 200 for any existing session, 404 for unknown IDs.  
**Rationale**: The PIN is generated at session creation (feature #5) and never changes. An admin may want to print or share the QR code before opening the lobby. Restricting to lobby/active would add complexity with no security benefit (the URL is already scoped to a session ID). Documented in spec Assumptions.

---

## Decision 6 — Partial pool tolerance on Launch

**Decision**: If `available_questions < question_count` but `available_questions > 0`, draw all available questions and return them in `question_count` of the response. No error.  
**Rationale**: Consistent with the behavior already established in US-S01 (session creation warns but proceeds). The admin already saw a warning at creation time. Failing the launch would be a regression in UX. Spec FR-012 makes this explicit.  
**Zero pool case**: Returns HTTP 422 `INSUFFICIENT_QUESTIONS` — no partial state.

---

## Decision 7 — Transaction scope for Launch

**Decision**: The entire Launch operation (check status, count pool, draw questions, insert `session_questions`, update session status) runs inside a single `pgx` transaction with `SELECT ... FOR UPDATE` on the session row to prevent concurrent launches.  
**Rationale**: Prevents two simultaneous `/launch` calls from both succeeding and inserting duplicate `session_questions` rows. The partial unique index `(session_id, position)` would catch it, but a clean transaction is better than relying on constraint violations for flow control.

---

## Decision 8 — Service constructor signature change

**Decision**: Change `session.NewService(repo)` to `session.NewService(repo, authCfg, playerURL, broadcaster)`.  
**Rationale**: The service needs `auth.Config` for token signing, `playerURL` for URL construction in QR and launch responses, and `broadcaster` for the future WebSocket hook. All three are injected at startup in `main.go`. This follows the same dependency-injection pattern used throughout the codebase.
