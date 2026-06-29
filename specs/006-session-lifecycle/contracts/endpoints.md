# API Contracts: Session Lifecycle (006)

All endpoints follow the existing `{ "data": ... }` / `{ "error": ... }` envelope. Source of truth: `docs/04-api-design.md`.

---

## POST /api/v1/sessions/:id/open-lobby

**Auth**: Admin Bearer (`RequireAdmin` middleware)  
**Body**: None  

### Response 200
```json
{
  "data": {
    "session_id": "uuid",
    "pin": "482910",
    "qr_code_url": "/api/v1/sessions/uuid/qr",
    "status": "lobby"
  }
}
```

### Response 404 — session not found
```json
{ "error": { "code": "NOT_FOUND", "message": "session not found" } }
```

### Response 409 — session not in draft
```json
{ "error": { "code": "SESSION_NOT_DRAFT", "message": "session cannot be opened in its current status" } }
```

### Response 401 — missing/invalid token
```json
{ "error": { "code": "UNAUTHORIZED", "message": "missing or invalid token" } }
```

---

## GET /api/v1/sessions/:id/qr

**Auth**: None (public endpoint)  
**Body**: None  

### Response 200
```
Content-Type: image/png
[binary PNG data]
```
The QR code encodes the URL: `{PLAYER_APP_BASE_URL}/join?pin={pin}`  
Default `PLAYER_APP_BASE_URL` = `http://localhost:5173` (placeholder — update when player frontend is deployed).

### Response 404 — session not found
```json
{ "error": { "code": "NOT_FOUND", "message": "session not found" } }
```

---

## POST /api/v1/sessions/:id/launch

**Auth**: Admin Bearer (`RequireAdmin` middleware)  
**Body**: None  

### Response 200
```json
{
  "data": {
    "session_id": "uuid",
    "status": "active",
    "question_count": 20,
    "projection_token": "<jwt>",
    "projection_url": "http://localhost:5173/projection?session=uuid&token=<jwt>",
    "started_at": "2025-04-20T10:00:00Z"
  }
}
```
`projection_token` is a JWT with claims `{ session_id, role: "projection" }`, TTL 12h, signed with `JWT_SECRET`.  
`projection_url` uses `PLAYER_APP_BASE_URL` (placeholder until projection frontend is deployed).  
`question_count` reflects the **actual** number of questions drawn (may be less than configured if pool was smaller).

### Response 404 — session not found
```json
{ "error": { "code": "NOT_FOUND", "message": "session not found" } }
```

### Response 409 — session not in lobby
```json
{ "error": { "code": "SESSION_NOT_IN_LOBBY", "message": "session cannot be launched in its current status" } }
```

### Response 422 — question pool empty
```json
{ "error": { "code": "INSUFFICIENT_QUESTIONS", "message": "no questions available in the configured categories" } }
```

### Response 401 — missing/invalid token
```json
{ "error": { "code": "UNAUTHORIZED", "message": "missing or invalid token" } }
```
