# Research: Session Create and Configure

**Feature**: 005-session-crud  
**Date**: 2026-05-04

## Findings

### 1. PIN Generation Strategy

**Decision**: Generate a random 6-digit string (`fmt.Sprintf("%06d", rand.Intn(900000)+100000)`) and insert. On `23505` unique-constraint violation (partial index `sessions_pin_active_unique`), retry with a new random value (max 10 attempts before returning an internal error).

**Rationale**: The partial unique index already enforces uniqueness among `lobby`/`active` sessions. The collision probability is negligible in practice (MVP scale, O(10) concurrent active sessions), so a simple retry loop is correct and minimal. No need for a separate PIN counter table or pre-allocated pool.

**Alternatives considered**:
- Sequential PIN from a counter table: more predictable but leaks session volume and adds schema complexity.
- Pre-allocated PIN pool: unnecessary complexity for MVP scale.

### 2. `available_questions` Computation

**Decision**: Run a single COUNT query joining `questions` through `session_categories` scoped to the new session's ID at creation time. For PATCH, recompute against the updated set.

```sql
SELECT COUNT(*)
FROM questions q
JOIN session_categories sc ON sc.category_id = q.category_id
WHERE sc.session_id = $1
  AND q.deleted_at IS NULL;
```

**Rationale**: This query already appears in `docs/02-data-model.md` as a canonical useful query. It is O(questions in selected categories) and runs once per create/update — not on the hot path.

**Alternatives considered**: Denormalizing the count into `sessions.available_questions` column — rejected because it would require keeping the column in sync whenever questions are added/deleted, which goes beyond this feature's scope.

### 3. `player_count` in List Response

**Decision**: Include `player_count` via a `LEFT JOIN … COUNT(p.id)` subquery in the list query, grouped by session. No denormalized column exists in the schema.

**Rationale**: The `players` table has `session_id` indexed (`players_session_id_idx`). A grouped COUNT is efficient and requires no schema change.

**Alternatives considered**: Separate N+1 queries per session — rejected as poor performance at scale. Denormalized column — would require updates at player join/disconnect events that are not part of this feature.

### 4. PATCH `category_ids` Replacement

**Decision**: When a PATCH request includes `category_ids`, the repository deletes all existing `session_categories` rows for the session and inserts the new set — all in a single transaction that also updates the `sessions` row.

**Rationale**: A full replace is simpler and correct for this use case (small set, admin-only). Diff-and-patch logic (compute added/removed IDs) adds complexity with no meaningful benefit at MVP scale.

**Alternatives considered**: Compute delta and only insert/delete changed rows — rejected as premature optimization.

### 5. No New Migration Required

**Decision**: All tables (`sessions`, `session_categories`) and all indexes (including `sessions_pin_active_unique`) are already created in `backend/migrations/001_initial_schema.up.sql`.

**Rationale**: Verified by reading the migration file. The `sessions` table has all required columns (`pin`, `status`, `question_count`, `time_per_question_s`, `points_per_answer`, `speed_bonus_enabled`, `created_by`, `started_at`, `ended_at`, soft-delete timestamps). The `session_categories` join table exists with the composite PK.

### 6. Admin ID Extraction

**Decision**: Use `c.Locals(auth.ClaimsKey).(auth.AdminClaims).AdminID` — the same pattern used in other handlers — to populate `created_by` on session creation.

**Rationale**: The `RequireAdmin` middleware already injects `AdminClaims` into the Fiber context under `auth.ClaimsKey`. No additional middleware or context key needed.

### 7. Validation Rules Summary

| Field | Rule |
|---|---|
| `name` | Required, non-empty string |
| `category_ids` | Required, at least one element, all must reference non-deleted categories |
| `question_count` | Required, integer 1–50 |
| `time_per_question_s` | Required, one of: 10, 20, 30, 60 |
| `points_per_answer` | Optional, positive integer, default 100 |
| `speed_bonus_enabled` | Optional, boolean, default false |

### 8. Error Code Mapping

| Condition | HTTP | Error Code |
|---|---|---|
| Missing/invalid fields | 422 | `VALIDATION_ERROR` |
| Invalid category ID (not found or deleted) | 422 | `VALIDATION_ERROR` |
| PATCH/DELETE on non-draft session | 409 | `SESSION_NOT_DRAFT` |
| Session not found | 404 | `NOT_FOUND` |

Note: `SESSION_NOT_DRAFT` is a new error code not yet listed in `docs/04-api-design.md` but consistent with the existing `SESSION_NOT_IN_LOBBY` and `SESSION_NOT_ACTIVE` codes in that document.
