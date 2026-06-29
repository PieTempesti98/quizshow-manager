# Data Model: Session Create and Configure

**Feature**: 005-session-crud  
**Date**: 2026-05-04

## Entities (Go structs)

### Session

```go
// Session is the core entity — corresponds to the sessions table.
type Session struct {
    ID                 uuid.UUID  // PK
    Name               string
    PIN                string     // CHAR(6), numeric
    Status             string     // "draft" | "lobby" | "active" | "completed" | "cancelled"
    QuestionCount      int16
    TimePerQuestionS   int16
    PointsPerAnswer    int
    SpeedBonusEnabled  bool
    StartedAt          *time.Time // nullable
    EndedAt            *time.Time // nullable
    CreatedBy          *uuid.UUID // nullable FK → admins.id
    CreatedAt          time.Time
    UpdatedAt          time.Time
    DeletedAt          *time.Time // nullable; soft-delete
}
```

### SessionCategory (join record, not directly returned in responses)

```go
type SessionCategory struct {
    SessionID  uuid.UUID
    CategoryID uuid.UUID
}
```

### CategoryRef (embedded in detail response)

```go
type CategoryRef struct {
    ID   uuid.UUID
    Name string
}
```

---

## Query / Input Types

### SessionFilter (for list endpoint)

```go
type SessionFilter struct {
    Statuses []string // comma-split from query param; empty = no filter
    Page     int      // 1-based; default 1
    PerPage  int      // default 20
}
```

### SessionListResult (paginated list response data)

```go
type SessionListResult struct {
    Sessions   []SessionListItem
    Total      int
    Page       int
    PerPage    int
    TotalPages int
}

// SessionListItem is one row in the list — includes player_count from JOIN.
type SessionListItem struct {
    Session
    PlayerCount int
}
```

### SessionDetail (single-session response data)

```go
type SessionDetail struct {
    Session
    Categories  []CategoryRef
    PlayerCount int
}
```

### SessionCreate (input to POST /sessions)

```go
type SessionCreate struct {
    Name               string      // required
    CategoryIDs        []uuid.UUID // required, ≥1
    QuestionCount      int16       // required, 1–50
    TimePerQuestionS   int16       // required, one of 10/20/30/60
    PointsPerAnswer    *int        // optional, default 100
    SpeedBonusEnabled  *bool       // optional, default false
}
```

### SessionUpdate (input to PATCH /sessions/:id)

```go
// All fields are pointers; nil means "do not update".
type SessionUpdate struct {
    Name               *string
    CategoryIDs        []uuid.UUID // nil slice = do not update; empty slice is invalid
    QuestionCount      *int16
    TimePerQuestionS   *int16
    PointsPerAnswer    *int
    SpeedBonusEnabled  *bool
}
```

---

## Sentinel Errors

```go
var (
    ErrSessionNotFound  = errors.New("session not found")
    ErrSessionNotDraft  = errors.New("session is not in draft status")
)
```

---

## Existing Tables Used (no schema changes)

### `sessions`
All columns already exist in migration 001. The partial unique index `sessions_pin_active_unique` enforces PIN uniqueness among `lobby`/`active` sessions.

### `session_categories`
Composite PK `(session_id, category_id)`, FK cascade delete from sessions.

### `players`
Read-only from this feature; `COUNT(id)` grouped by `session_id` used for `player_count`.

### `categories`
Read-only; joined for name resolution and ID validation.

---

## Key Invariants

- `status` starts at `"draft"` on creation; this feature does not change status (status transitions belong to later features).
- `created_by` is set from JWT claims; it is nullable in the schema (legacy rows) but always populated by this feature.
- `available_questions` is a computed value returned in responses — it is **not** stored in the database.
- `deleted_at IS NULL` must appear in every query except intentional history reads.
- PIN generation: random 6-digit string; retry on `23505` constraint violation (max 10 attempts).
