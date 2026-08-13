# API Contracts: Presenter Controls Endpoints

**Base Path**: `/api/v1/sessions/:id`  
**Authentication**: Required Bearer token (`role: admin`) for all endpoints.

---

## 1. Advance to Next Question

### `POST /api/v1/sessions/:id/next-question`

Advances the active session to the next question in sequence and marks it as asked.

#### Request
- **Headers**: `Authorization: Bearer <admin_jwt>`
- **Body**: None

#### Responses

##### `200 OK`
```json
{
  "data": {
    "session_question_id": "9b1deb4d-3b7d-4bad-9bdd-2b0d7b3dcb6d",
    "position": 1,
    "total": 10,
    "question": {
      "text": "Qual è la capitale dell'Australia?",
      "option_a": "Sydney",
      "option_b": "Melbourne",
      "option_c": "Canberra",
      "option_d": "Brisbane"
    },
    "time_limit_s": 30,
    "asked_at": "2026-08-13T21:50:00Z"
  }
}
```

##### `401 Unauthorized`
```json
{
  "error": {
    "code": "UNAUTHORIZED",
    "message": "Missing or invalid authorization token"
  }
}
```

##### `404 Not Found`
```json
{
  "error": {
    "code": "NOT_FOUND",
    "message": "session not found"
  }
}
```

##### `409 Conflict`
- `SESSION_NOT_ACTIVE`:
  ```json
  {
    "error": {
      "code": "SESSION_NOT_ACTIVE",
      "message": "session must be active to advance questions"
    }
  }
  ```
- `QUESTION_NOT_REVEALED`:
  ```json
  {
    "error": {
      "code": "QUESTION_NOT_REVEALED",
      "message": "current question must be revealed before advancing"
    }
  }
  ```
- `NO_MORE_QUESTIONS`:
  ```json
  {
    "error": {
      "code": "NO_MORE_QUESTIONS",
      "message": "all questions have been asked; end session instead"
    }
  }
  ```

---

## 2. Pause Timer

### `POST /api/v1/sessions/:id/pause-timer`

Pauses the countdown for the currently active question.

#### Request
- **Headers**: `Authorization: Bearer <admin_jwt>`
- **Body**: None

#### Responses

##### `200 OK`
```json
{
  "data": {
    "ok": true,
    "paused_at": "2026-08-13T21:50:15Z"
  }
}
```

##### `409 Conflict`
- `TIMER_ALREADY_PAUSED`:
  ```json
  {
    "error": {
      "code": "TIMER_ALREADY_PAUSED",
      "message": "timer is already paused"
    }
  }
  ```
- `NO_ACTIVE_QUESTION`:
  ```json
  {
    "error": {
      "code": "NO_ACTIVE_QUESTION",
      "message": "no question currently in progress"
    }
  }
  ```

---

## 3. Resume Timer

### `POST /api/v1/sessions/:id/resume-timer`

Resumes a paused timer and shifts `asked_at` to account for the pause duration.

#### Request
- **Headers**: `Authorization: Bearer <admin_jwt>`
- **Body**: None

#### Responses

##### `200 OK`
```json
{
  "data": {
    "ok": true,
    "resumed_at": "2026-08-13T21:50:35Z"
  }
}
```

##### `409 Conflict`
- `TIMER_NOT_PAUSED`:
  ```json
  {
    "error": {
      "code": "TIMER_NOT_PAUSED",
      "message": "timer is not paused"
    }
  }
  ```

---

## 4. Reveal Correct Answer & Score Round

### `POST /api/v1/sessions/:id/reveal`

Reveals the correct answer, evaluates all submitted answers, computes points and bonuses, updates player cumulative scores, and calculates the top 5 leaderboard.

#### Request
- **Headers**: `Authorization: Bearer <admin_jwt>`
- **Body**: None

#### Responses

##### `200 OK`
```json
{
  "data": {
    "session_question_id": "9b1deb4d-3b7d-4bad-9bdd-2b0d7b3dcb6d",
    "correct_index": 2,
    "answer_distribution": [
      { "index": 0, "count": 2, "percent": 25 },
      { "index": 1, "count": 1, "percent": 13 },
      { "index": 2, "count": 5, "percent": 62 },
      { "index": 3, "count": 0, "percent": 0 }
    ],
    "top5": [
      {
        "rank": 1,
        "nickname": "Alice",
        "total_score": 145,
        "avatar_color": "#FF5733"
      },
      {
        "rank": 2,
        "nickname": "Bob",
        "total_score": 120,
        "avatar_color": "#33C1FF"
      }
    ],
    "revealed_at": "2026-08-13T21:51:00Z"
  }
}
```

##### `409 Conflict`
- `QUESTION_ALREADY_REVEALED`:
  ```json
  {
    "error": {
      "code": "QUESTION_ALREADY_REVEALED",
      "message": "question has already been revealed"
    }
  }
  ```
- `NO_ACTIVE_QUESTION`:
  ```json
  {
    "error": {
      "code": "NO_ACTIVE_QUESTION",
      "message": "no question currently in progress"
    }
  }
  ```

---

## 5. End Session

### `POST /api/v1/sessions/:id/end`

Concludes the quiz session, transitions status to `completed`, and marks `ended_at`.

#### Request
- **Headers**: `Authorization: Bearer <admin_jwt>`
- **Body** (optional):
```json
{
  "reason": "early"
}
```
*(reason can be `"completed"` or `"early"`, defaults to `"completed"` if empty)*

#### Responses

##### `200 OK`
```json
{
  "data": {
    "session_id": "3fa85f64-5717-4562-b3fc-2c963f66afa6",
    "status": "completed",
    "ended_at": "2026-08-13T21:55:00Z"
  }
}
```

##### `409 Conflict`
- `SESSION_ALREADY_ENDED`:
  ```json
  {
    "error": {
      "code": "SESSION_ALREADY_ENDED",
      "message": "session has already ended"
    }
  }
  ```
