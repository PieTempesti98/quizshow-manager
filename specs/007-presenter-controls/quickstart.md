# Quickstart & Verification Guide: Presenter Controls

This guide provides end-to-end verification steps for Feature #7 (Presenter Controls & Scoring Engine) using `curl` against a running backend instance.

---

## 1. Prerequisites

1. PostgreSQL database running and migrated to migration `001_initial_schema`.
2. Backend Go server running on `http://localhost:3000`.
3. Admin account created (e.g. `admin@quizshow.local`).

---

## 2. Step-by-Step Test Sequence

### Step 1: Admin Login
```bash
TOKEN=$(curl -s -X POST http://localhost:3000/api/v1/auth/login \
  -H 'Content-Type: application/json' \
  -d '{"email":"admin@quizshow.local","password":"secretpassword"}' \
  | jq -r .data.access_token)

echo "Admin Token: $TOKEN"
```

### Step 2: Setup Session & Launch
```bash
# 1. Get a category ID with questions
CAT_ID=$(curl -s -H "Authorization: Bearer $TOKEN" http://localhost:3000/api/v1/categories | jq -r '.data.categories[0].id')

# 2. Create session with speed bonus enabled
SESSION_ID=$(curl -s -X POST http://localhost:3000/api/v1/sessions \
  -H "Authorization: Bearer $TOKEN" \
  -H 'Content-Type: application/json' \
  -d "{\"name\":\"Presenter Smoke Test\",\"category_ids\":[\"$CAT_ID\"],\"question_count\":3,\"time_per_question_s\":30,\"points_per_answer\":100,\"speed_bonus_enabled\":true}" \
  | jq -r .data.id)

# 3. Open Lobby
curl -s -X POST "http://localhost:3000/api/v1/sessions/$SESSION_ID/open-lobby" \
  -H "Authorization: Bearer $TOKEN" | jq .

# 4. Launch Session (moves to active, draws questions)
curl -s -X POST "http://localhost:3000/api/v1/sessions/$SESSION_ID/launch" \
  -H "Authorization: Bearer $TOKEN" | jq .
```

### Step 3: Advance to First Question (`POST /next-question`)
```bash
curl -s -X POST "http://localhost:3000/api/v1/sessions/$SESSION_ID/next-question" \
  -H "Authorization: Bearer $TOKEN" | jq .
```
**Expected Outcome**:
- Status `200 OK`.
- JSON contains `session_question_id`, `position: 1`, `total: 3`, `question` object without `correct_index`, `asked_at`.

### Step 4: Verify Guard against Skipping Question
```bash
# Calling next-question again before reveal must return 409
curl -s -X POST "http://localhost:3000/api/v1/sessions/$SESSION_ID/next-question" \
  -H "Authorization: Bearer $TOKEN" | jq .
```
**Expected Outcome**:
- Status `409 Conflict` with `code: "QUESTION_NOT_REVEALED"`.

### Step 5: Pause and Resume Timer
```bash
# 1. Pause timer
curl -s -X POST "http://localhost:3000/api/v1/sessions/$SESSION_ID/pause-timer" \
  -H "Authorization: Bearer $TOKEN" | jq .
# Expected: 200 OK with paused_at

# 2. Pause timer again (must fail 409)
curl -s -X POST "http://localhost:3000/api/v1/sessions/$SESSION_ID/pause-timer" \
  -H "Authorization: Bearer $TOKEN" | jq .
# Expected: 409 Conflict with TIMER_ALREADY_PAUSED

# 3. Resume timer
curl -s -X POST "http://localhost:3000/api/v1/sessions/$SESSION_ID/resume-timer" \
  -H "Authorization: Bearer $TOKEN" | jq .
# Expected: 200 OK with resumed_at
```

### Step 6: Reveal Answer & Score Round
```bash
curl -s -X POST "http://localhost:3000/api/v1/sessions/$SESSION_ID/reveal" \
  -H "Authorization: Bearer $TOKEN" | jq .
```
**Expected Outcome**:
- Status `200 OK`.
- JSON contains `session_question_id`, `correct_index` (0..3), `answer_distribution` (array of 4 items), `top5` array, and `revealed_at`.

### Step 7: End Session Early / Conclude Quiz
```bash
curl -s -X POST "http://localhost:3000/api/v1/sessions/$SESSION_ID/end" \
  -H "Authorization: Bearer $TOKEN" \
  -H 'Content-Type: application/json' \
  -d '{"reason":"completed"}' | jq .
```
**Expected Outcome**:
- Status `200 OK`.
- JSON contains `session_id`, `status: "completed"`, `ended_at`.

---

## 3. Automated Unit Testing
Run pure scoring tests:
```bash
cd backend
go test -v ./internal/session/...
```
