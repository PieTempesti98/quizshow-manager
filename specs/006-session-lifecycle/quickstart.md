# Quickstart: Session Lifecycle (006)

## Prerequisites

- Docker Compose running (`docker compose up -d`)
- Admin account seeded (from env vars on first run)
- At least one category with at least one question in the DB

## New environment variable

| Var                  | Default                   | Notes                                                             |
|----------------------|---------------------------|-------------------------------------------------------------------|
| `PLAYER_APP_BASE_URL`| `http://localhost:5173`   | PLACEHOLDER — update to real player app URL when frontend ships  |

## New dependency

```bash
cd backend
go get github.com/skip2/go-qrcode
```

## New endpoints

| Method | Path                          | Auth  | Description                          |
|--------|-------------------------------|-------|--------------------------------------|
| POST   | `/api/v1/sessions/:id/open-lobby` | Admin | Transition draft → lobby         |
| GET    | `/api/v1/sessions/:id/qr`     | None  | PNG QR code for player join URL      |
| POST   | `/api/v1/sessions/:id/launch` | Admin | Transition lobby → active, draw Qs  |

## Session state machine (relevant transitions)

```
draft ──[open-lobby]──▶ lobby ──[launch]──▶ active
```

## Quick smoke test

```bash
# Prerequisites: $TOKEN set to admin JWT, $CAT_ID set to a category UUID

SESSION=$(curl -s -X POST http://localhost:3000/api/v1/sessions \
  -H "Authorization: Bearer $TOKEN" -H 'Content-Type: application/json' \
  -d "{\"name\":\"Test\",\"category_ids\":[\"$CAT_ID\"],\"question_count\":3,\"time_per_question_s\":30}" \
  | jq -r .data.id)

# Open lobby
curl -s -X POST http://localhost:3000/api/v1/sessions/$SESSION/open-lobby \
  -H "Authorization: Bearer $TOKEN" | jq .data.status
# → "lobby"

# QR code
curl -s http://localhost:3000/api/v1/sessions/$SESSION/qr -o /tmp/qr.png && file /tmp/qr.png
# → "PNG image data"

# Launch
curl -s -X POST http://localhost:3000/api/v1/sessions/$SESSION/launch \
  -H "Authorization: Bearer $TOKEN" | jq '{status: .data.status, qcount: .data.question_count}'
# → {"status": "active", "qcount": 3}
```
