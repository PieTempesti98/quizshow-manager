# Quickstart & Validation Guide: Feature #8 — Player Join & Answer Submission

## Overview

This guide describes how to run automated unit tests and perform a complete live smoke test of player join, token validation, answer submission, idempotency, and timer expiration.

---

## 1. Automated Unit & Integration Tests

Run backend unit tests for session package (including player join, answer timer calculations, and auth token issuance):

```bash
cd backend
go test -v ./internal/session/... ./internal/auth/...
```

---

## 2. End-to-End Smoke Test Flow

### Step 1: Admin Login & Session Setup

1. **Admin Login**:
   ```bash
   curl -X POST http://localhost:8080/api/v1/auth/login \
     -H "Content-Type: application/json" \
     -d '{"email": "admin@quizshow.local", "password": "adminpassword"}'
   ```
   *Save `$ADMIN_TOKEN` from response.*

2. **Create Session**:
   ```bash
   curl -X POST http://localhost:8080/api/v1/sessions \
     -H "Authorization: Bearer $ADMIN_TOKEN" \
     -H "Content-Type: application/json" \
     -d '{"name": "Live Test Session", "category_ids": ["<CATEGORY_ID>"], "question_count": 5, "time_per_question_s": 20, "points_per_answer": 100, "speed_bonus_enabled": true}'
   ```
   *Save `$SESSION_ID` and `$PIN` from response.*

3. **Open Lobby**:
   ```bash
   curl -X POST http://localhost:8080/api/v1/sessions/$SESSION_ID/open-lobby \
     -H "Authorization: Bearer $ADMIN_TOKEN"
   ```

---

### Step 2: Player Join

4. **Join as Player 1 (Mario)**:
   ```bash
   curl -X POST http://localhost:8080/api/v1/sessions/$SESSION_ID/join \
     -H "Content-Type: application/json" \
     -d '{"pin": "'"$PIN"'", "nickname": "Mario"}'
   ```
   *Expected: HTTP 201 with `player_id`, `avatar_color`, and `player_token`. Save `$PLAYER1_TOKEN`.*

5. **Join Duplicate Nickname (Mario again)**:
   ```bash
   curl -X POST http://localhost:8080/api/v1/sessions/$SESSION_ID/join \
     -H "Content-Type: application/json" \
     -d '{"pin": "'"$PIN"'", "nickname": "Mario"}'
   ```
   *Expected: HTTP 409 `NICKNAME_TAKEN`.*

6. **Join as Player 2 (Luigi)**:
   ```bash
   curl -X POST http://localhost:8080/api/v1/sessions/$SESSION_ID/join \
     -H "Content-Type: application/json" \
     -d '{"pin": "'"$PIN"'", "nickname": "Luigi"}'
   ```
   *Save `$PLAYER2_TOKEN`.*

---

### Step 3: Launch Session & Advance Question

7. **Launch Session**:
   ```bash
   curl -X POST http://localhost:8080/api/v1/sessions/$SESSION_ID/launch \
     -H "Authorization: Bearer $ADMIN_TOKEN"
   ```

8. **Advance to Question 1**:
   ```bash
   curl -X POST http://localhost:8080/api/v1/sessions/$SESSION_ID/next-question \
     -H "Authorization: Bearer $ADMIN_TOKEN"
   ```
   *Save `$SESSION_QUESTION_ID` from response.*

---

### Step 4: Submit Answers & Test Idempotency / Expiration

9. **Player 1 Submits Answer (Valid)**:
   ```bash
   curl -X POST http://localhost:8080/api/v1/sessions/$SESSION_ID/answers \
     -H "Authorization: Bearer $PLAYER1_TOKEN" \
     -H "Content-Type: application/json" \
     -d '{"session_question_id": "'"$SESSION_QUESTION_ID"'", "chosen_index": 2}'
   ```
   *Expected: HTTP 201 with `answer_id`, `chosen_index: 2`, `answered_at`.*

10. **Player 1 Submits Again (Idempotency Test)**:
    ```bash
    curl -X POST http://localhost:8080/api/v1/sessions/$SESSION_ID/answers \
      -H "Authorization: Bearer $PLAYER1_TOKEN" \
      -H "Content-Type: application/json" \
      -d '{"session_question_id": "'"$SESSION_QUESTION_ID"'", "chosen_index": 2}'
    ```
    *Expected: HTTP 200 with identical `answer_id`.*

11. **Wait > 20s (Timer Expiry) & Player 2 Submits**:
    ```bash
    sleep 22
    curl -X POST http://localhost:8080/api/v1/sessions/$SESSION_ID/answers \
      -H "Authorization: Bearer $PLAYER2_TOKEN" \
      -H "Content-Type: application/json" \
      -d '{"session_question_id": "'"$SESSION_QUESTION_ID"'", "chosen_index": 1}'
    ```
    *Expected: HTTP 409 `QUESTION_CLOSED`.*

12. **Presenter Reveal & Verify Scores**:
    ```bash
    curl -X POST http://localhost:8080/api/v1/sessions/$SESSION_ID/reveal \
      -H "Authorization: Bearer $ADMIN_TOKEN"
    ```
    *Expected: HTTP 200 with `correct_index`, `answer_distribution`, and leaderboard reflecting Player 1's score.*
