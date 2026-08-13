# Quickstart & Verification Guide: Feature #9 — Stats & Leaderboard

**Branch**: `009-stats-leaderboard` | **Date**: 2026-08-14 | **Spec**: [spec.md](spec.md)

## Automated Tests

Run unit and integration tests across the session package:

```bash
# 1. Run unit tests for leaderboard ranking, tie-breaking, and CSV formatting
go test -v ./internal/session/... -run "TestLeaderboard|TestStats|TestCSV"

# 2. Run all backend tests
go test -v ./...
```

---

## Manual Smoke Test Sequence

### Prerequisites
- Docker Compose running PostgreSQL (`docker compose up -d postgres`)
- Server running on `http://localhost:8080`

### 1. Authenticate as Admin
```bash
ADMIN_TOKEN=$(curl -s -X POST http://localhost:8080/api/v1/auth/login \
  -H "Content-Type: application/json" \
  -d '{"email":"admin@quizshow.local","password":"secret"}' \
  | jq -r '.data.access_token')
```

### 2. Verify Status Guard on Active/Draft Session
```bash
# Query leaderboard for a draft session -> Expected HTTP 409 SESSION_NOT_COMPLETED
curl -s -w "\nHTTP: %{http_code}\n" -X GET http://localhost:8080/api/v1/sessions/<DRAFT_SESSION_ID>/leaderboard \
  -H "Authorization: Bearer $ADMIN_TOKEN"
```

### 3. Verify Leaderboard on Completed Session (JSON)
```bash
curl -s -X GET http://localhost:8080/api/v1/sessions/<COMPLETED_SESSION_ID>/leaderboard \
  -H "Authorization: Bearer $ADMIN_TOKEN" | jq .
```
- Assert `status: 200`
- Assert `leaderboard` entries have sequential 1-based ranks (`1, 2, 3...`) sorted by `total_score DESC`.

### 4. Verify CSV Export
```bash
curl -i -X GET "http://localhost:8080/api/v1/sessions/<COMPLETED_SESSION_ID>/leaderboard?format=csv" \
  -H "Authorization: Bearer $ADMIN_TOKEN"
```
- Assert `Content-Type: text/csv; charset=utf-8`
- Assert `Content-Disposition: attachment; filename="..."`
- Assert CSV header `rank,nickname,total_score` and valid rows.

### 5. Verify Per-Question Stats
```bash
curl -s -X GET http://localhost:8080/api/v1/sessions/<COMPLETED_SESSION_ID>/stats \
  -H "Authorization: Bearer $ADMIN_TOKEN" | jq .
```
- Assert `questions` array is ordered by `position` (1..N).
- Assert each question has `correct_count`, `wrong_count`, `no_answer_count`, and answer distribution.
- Assert `correct_count + wrong_count + no_answer_count == total_players`.

### 6. Verify Session History Query
```bash
curl -s -X GET "http://localhost:8080/api/v1/sessions?status=completed,cancelled" \
  -H "Authorization: Bearer $ADMIN_TOKEN" | jq .
```
- Assert returned sessions only have `status: completed` or `status: cancelled`.
