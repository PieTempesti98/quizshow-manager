# QuizShow — Data Model

## Overview

The data model is built around three core concepts:

- **Content** (`categories`, `questions`) — the question bank, managed by the admin and reusable across sessions
- **Session lifecycle** (`sessions`, `session_categories`, `session_questions`) — the configuration and runtime snapshot of a live quiz
- **Participation** (`players`, `answers`) — ephemeral player identity and their responses within a session

---

## Design decisions

### Session snapshot pattern
`session_questions` captures the exact set of questions drawn for a session at launch time, in order. This decouples the live session from the question bank: questions can be edited or soft-deleted after the session without corrupting historical results. The `position` column preserves presentation order.

### Ephemeral players (MVP)
`players` has no foreign key to an `admins` or `users` table. A player exists only for the duration of a session — identified by nickname + session PIN. This is intentional for MVP (ADR-001). In R2, a `users` table will be introduced and `players.user_id` will become an optional FK.

### Soft deletes everywhere
All content tables (`admins`, `categories`, `questions`, `sessions`) carry `deleted_at TIMESTAMPTZ NULL`. Queries must always filter `WHERE deleted_at IS NULL` unless explicitly querying history.

### Score denormalization
`players.total_score` is a running total maintained server-side at each reveal. It is also derivable by summing `answers.points_awarded` for the player. The denormalized column exists for fast leaderboard queries during the live session — no aggregation needed on the hot path.

### Answer time in milliseconds
`answers.answer_time_ms` stores how many milliseconds elapsed between the question being displayed and the player submitting. Used for speed bonus calculation and future analytics.

### PIN uniqueness scope
`sessions.pin` is unique among sessions with `status IN ('lobby', 'active')`. Completed sessions can reuse PINs. Enforced by a partial unique index.

---

## Entity reference

### `admins`
The operator account. Single-user for MVP; the table supports multiple rows for future multi-admin support.

| Column | Type | Notes |
|---|---|---|
| `id` | `UUID` | PK, default `gen_random_uuid()` |
| `email` | `TEXT` | Unique, used for login |
| `password_hash` | `TEXT` | bcrypt hash |
| `name` | `TEXT` | Display name |
| `created_at` | `TIMESTAMPTZ` | Default `now()` |
| `updated_at` | `TIMESTAMPTZ` | Updated on every change |
| `deleted_at` | `TIMESTAMPTZ` | Null = active |

### `refresh_tokens`
Stores issued refresh tokens for JWT rotation and revocation.

| Column | Type | Notes |
|---|---|---|
| `id` | `UUID` | PK |
| `admin_id` | `UUID` | FK → `admins.id` |
| `token_hash` | `TEXT` | SHA-256 of the raw token |
| `expires_at` | `TIMESTAMPTZ` | 7 days from issuance |
| `created_at` | `TIMESTAMPTZ` | |
| `revoked_at` | `TIMESTAMPTZ` | Null = still valid |

### `categories`
Groups of questions. Used to configure which topics a session draws from.

| Column | Type | Notes |
|---|---|---|
| `id` | `UUID` | PK |
| `name` | `TEXT` | Display name, unique among non-deleted |
| `slug` | `TEXT` | URL-safe identifier, unique |
| `created_at` | `TIMESTAMPTZ` | |
| `updated_at` | `TIMESTAMPTZ` | |
| `deleted_at` | `TIMESTAMPTZ` | |

### `questions`
The question bank. Multiple-choice, always 4 options.

| Column | Type | Notes |
|---|---|---|
| `id` | `UUID` | PK |
| `category_id` | `UUID` | FK → `categories.id` |
| `text` | `TEXT` | Question body |
| `option_a` | `TEXT` | Answer option A |
| `option_b` | `TEXT` | Answer option B |
| `option_c` | `TEXT` | Answer option C |
| `option_d` | `TEXT` | Answer option D |
| `correct_index` | `SMALLINT` | 0=A, 1=B, 2=C, 3=D |
| `difficulty` | `TEXT` | enum: `easy`, `medium`, `hard` |
| `created_at` | `TIMESTAMPTZ` | |
| `updated_at` | `TIMESTAMPTZ` | |
| `deleted_at` | `TIMESTAMPTZ` | |

### `sessions`
A quiz session, from draft through completion.

| Column | Type | Notes |
|---|---|---|
| `id` | `UUID` | PK |
| `name` | `TEXT` | Display name |
| `pin` | `CHAR(6)` | 6-digit numeric PIN |
| `status` | `TEXT` | enum: `draft`, `lobby`, `active`, `completed`, `cancelled` |
| `question_count` | `SMALLINT` | How many questions to draw |
| `time_per_question_s` | `SMALLINT` | Timer in seconds (10/20/30/60) |
| `points_per_answer` | `INT` | Base points for correct answer |
| `speed_bonus_enabled` | `BOOLEAN` | Default false |
| `started_at` | `TIMESTAMPTZ` | Set when status → active |
| `ended_at` | `TIMESTAMPTZ` | Set when status → completed/cancelled |
| `created_by` | `UUID` | FK → `admins.id`, nullable — null = legacy/seeded row |
| `created_at` | `TIMESTAMPTZ` | |
| `updated_at` | `TIMESTAMPTZ` | |
| `deleted_at` | `TIMESTAMPTZ` | |

#### Session status machine

```
draft → lobby → active → completed
                        → cancelled
draft → cancelled
```

- `draft`: created, not yet open to players
- `lobby`: PIN active, players can join, quiz not started
- `active`: quiz is running, questions being asked
- `completed`: all questions done, results final
- `cancelled`: terminated early or abandoned

### `session_categories`
Many-to-many join: which categories a session draws questions from.

| Column | Type | Notes |
|---|---|---|
| `session_id` | `UUID` | FK → `sessions.id` |
| `category_id` | `UUID` | FK → `categories.id` |

Composite PK: `(session_id, category_id)`.

### `session_questions`
The frozen snapshot of questions drawn for a session, in order.

| Column | Type | Notes |
|---|---|---|
| `id` | `UUID` | PK |
| `session_id` | `UUID` | FK → `sessions.id` |
| `question_id` | `UUID` | FK → `questions.id` |
| `position` | `SMALLINT` | 1-based display order |
| `asked_at` | `TIMESTAMPTZ` | When presenter advanced to this question |
| `revealed_at` | `TIMESTAMPTZ` | When answer was revealed |

Unique constraint: `(session_id, position)`.

### `players`
A participant in a session. Ephemeral for MVP — no persistent account.

| Column | Type | Notes |
|---|---|---|
| `id` | `UUID` | PK |
| `session_id` | `UUID` | FK → `sessions.id` |
| `nickname` | `TEXT` | Unique within session |
| `avatar_color` | `TEXT` | Hex color, generated on join |
| `total_score` | `INT` | Running total, updated at each reveal |
| `joined_at` | `TIMESTAMPTZ` | |
| `disconnected_at` | `TIMESTAMPTZ` | Null = currently connected |

Unique constraint: `(session_id, nickname)`.

### `answers`
A player's response to a single question in a session.

| Column | Type | Notes |
|---|---|---|
| `id` | `UUID` | PK |
| `player_id` | `UUID` | FK → `players.id` |
| `session_question_id` | `UUID` | FK → `session_questions.id` |
| `chosen_index` | `SMALLINT` | 0–3, or NULL if no answer submitted |
| `is_correct` | `BOOLEAN` | Computed at answer time |
| `points_awarded` | `INT` | 0 if wrong/no answer, includes bonus if applicable |
| `answer_time_ms` | `INT` | Milliseconds from question display to submission |
| `answered_at` | `TIMESTAMPTZ` | |

Unique constraint: `(player_id, session_question_id)` — one answer per player per question.

---

## SQL schema (migration 001)

```sql
-- Enable UUID generation
CREATE EXTENSION IF NOT EXISTS "pgcrypto";

-- ─── admins ───────────────────────────────────────────────────────────────────

CREATE TABLE admins (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    email           TEXT NOT NULL,
    password_hash   TEXT NOT NULL,
    name            TEXT NOT NULL,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    deleted_at      TIMESTAMPTZ
);

CREATE UNIQUE INDEX admins_email_unique
    ON admins (email)
    WHERE deleted_at IS NULL;

-- ─── refresh_tokens ───────────────────────────────────────────────────────────

CREATE TABLE refresh_tokens (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    admin_id    UUID NOT NULL REFERENCES admins(id) ON DELETE CASCADE,
    token_hash  TEXT NOT NULL UNIQUE,
    expires_at  TIMESTAMPTZ NOT NULL,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    revoked_at  TIMESTAMPTZ
);

CREATE INDEX refresh_tokens_admin_id_idx ON refresh_tokens (admin_id);

-- ─── categories ───────────────────────────────────────────────────────────────

CREATE TABLE categories (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name        TEXT NOT NULL,
    slug        TEXT NOT NULL,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    deleted_at  TIMESTAMPTZ
);

CREATE UNIQUE INDEX categories_name_unique
    ON categories (name)
    WHERE deleted_at IS NULL;

CREATE UNIQUE INDEX categories_slug_unique
    ON categories (slug)
    WHERE deleted_at IS NULL;

-- ─── questions ────────────────────────────────────────────────────────────────

CREATE TYPE difficulty_level AS ENUM ('easy', 'medium', 'hard');

CREATE TABLE questions (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    category_id     UUID NOT NULL REFERENCES categories(id),
    text            TEXT NOT NULL,
    option_a        TEXT NOT NULL,
    option_b        TEXT NOT NULL,
    option_c        TEXT NOT NULL,
    option_d        TEXT NOT NULL,
    correct_index   SMALLINT NOT NULL CHECK (correct_index BETWEEN 0 AND 3),
    difficulty      difficulty_level NOT NULL DEFAULT 'medium',
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    deleted_at      TIMESTAMPTZ
);

CREATE INDEX questions_category_id_idx ON questions (category_id)
    WHERE deleted_at IS NULL;

-- ─── sessions ─────────────────────────────────────────────────────────────────

CREATE TYPE session_status AS ENUM ('draft', 'lobby', 'active', 'completed', 'cancelled');

CREATE TABLE sessions (
    id                      UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name                    TEXT NOT NULL,
    pin                     CHAR(6) NOT NULL,
    status                  session_status NOT NULL DEFAULT 'draft',
    question_count          SMALLINT NOT NULL CHECK (question_count BETWEEN 1 AND 50),
    time_per_question_s     SMALLINT NOT NULL CHECK (time_per_question_s IN (10, 20, 30, 60)),
    points_per_answer       INT NOT NULL DEFAULT 100 CHECK (points_per_answer > 0),
    speed_bonus_enabled     BOOLEAN NOT NULL DEFAULT false,
    started_at              TIMESTAMPTZ,
    ended_at                TIMESTAMPTZ,
    created_by              UUID REFERENCES admins(id) ON DELETE SET NULL,
    created_at              TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at              TIMESTAMPTZ NOT NULL DEFAULT now(),
    deleted_at              TIMESTAMPTZ
);

-- PIN must be unique among active/lobby sessions only
CREATE UNIQUE INDEX sessions_pin_active_unique
    ON sessions (pin)
    WHERE status IN ('lobby', 'active') AND deleted_at IS NULL;

CREATE INDEX sessions_status_idx ON sessions (status)
    WHERE deleted_at IS NULL;

-- ─── session_categories ───────────────────────────────────────────────────────

CREATE TABLE session_categories (
    session_id   UUID NOT NULL REFERENCES sessions(id) ON DELETE CASCADE,
    category_id  UUID NOT NULL REFERENCES categories(id),
    PRIMARY KEY (session_id, category_id)
);

-- ─── session_questions ────────────────────────────────────────────────────────

CREATE TABLE session_questions (
    id           UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    session_id   UUID NOT NULL REFERENCES sessions(id) ON DELETE CASCADE,
    question_id  UUID NOT NULL REFERENCES questions(id),
    position     SMALLINT NOT NULL CHECK (position >= 1),
    asked_at     TIMESTAMPTZ,
    revealed_at  TIMESTAMPTZ,
    UNIQUE (session_id, position)
);

CREATE INDEX session_questions_session_id_idx ON session_questions (session_id);

-- ─── players ──────────────────────────────────────────────────────────────────

CREATE TABLE players (
    id               UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    session_id       UUID NOT NULL REFERENCES sessions(id) ON DELETE CASCADE,
    nickname         TEXT NOT NULL,
    avatar_color     TEXT NOT NULL DEFAULT '#888888',
    total_score      INT NOT NULL DEFAULT 0,
    joined_at        TIMESTAMPTZ NOT NULL DEFAULT now(),
    disconnected_at  TIMESTAMPTZ,
    UNIQUE (session_id, nickname)
);

CREATE INDEX players_session_id_idx ON players (session_id);

-- ─── answers ──────────────────────────────────────────────────────────────────

CREATE TABLE answers (
    id                   UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    player_id            UUID NOT NULL REFERENCES players(id) ON DELETE CASCADE,
    session_question_id  UUID NOT NULL REFERENCES session_questions(id) ON DELETE CASCADE,
    chosen_index         SMALLINT CHECK (chosen_index BETWEEN 0 AND 3),
    is_correct           BOOLEAN NOT NULL DEFAULT false,
    points_awarded       INT NOT NULL DEFAULT 0,
    answer_time_ms       INT CHECK (answer_time_ms >= 0),
    answered_at          TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (player_id, session_question_id)
);

CREATE INDEX answers_session_question_id_idx ON answers (session_question_id);
CREATE INDEX answers_player_id_idx ON answers (player_id);

-- ─── updated_at trigger ───────────────────────────────────────────────────────

CREATE OR REPLACE FUNCTION set_updated_at()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = now();
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER admins_updated_at
    BEFORE UPDATE ON admins
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();

CREATE TRIGGER categories_updated_at
    BEFORE UPDATE ON categories
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();

CREATE TRIGGER questions_updated_at
    BEFORE UPDATE ON questions
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();

CREATE TRIGGER sessions_updated_at
    BEFORE UPDATE ON sessions
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();
```

---

## Useful queries

### Leaderboard for a session
```sql
SELECT
    nickname,
    total_score,
    RANK() OVER (ORDER BY total_score DESC) AS rank
FROM players
WHERE session_id = $1
ORDER BY total_score DESC;
```

### Per-question answer breakdown (post-session stats)
```sql
SELECT
    sq.position,
    q.text AS question_text,
    q.correct_index,
    COUNT(a.id) FILTER (WHERE a.is_correct) AS correct_count,
    COUNT(a.id) FILTER (WHERE NOT a.is_correct AND a.chosen_index IS NOT NULL) AS wrong_count,
    COUNT(p.id) - COUNT(a.id) AS no_answer_count
FROM session_questions sq
JOIN questions q ON q.id = sq.question_id
JOIN players p ON p.session_id = sq.session_id
LEFT JOIN answers a ON a.session_question_id = sq.id AND a.player_id = p.id
WHERE sq.session_id = $1
GROUP BY sq.position, q.text, q.correct_index
ORDER BY sq.position;
```

### Answer distribution per question (for projection screen bar chart)
```sql
SELECT
    chosen_index,
    COUNT(*) AS count
FROM answers
WHERE session_question_id = $1
  AND chosen_index IS NOT NULL
GROUP BY chosen_index
ORDER BY chosen_index;
```

### Available questions for a session (before draw)
```sql
SELECT COUNT(*)
FROM questions q
JOIN session_categories sc ON sc.category_id = q.category_id
WHERE sc.session_id = $1
  AND q.deleted_at IS NULL;
```

---

## Indexes summary

| Table | Index | Type | Purpose |
|---|---|---|---|
| `admins` | `admins_email_unique` | Partial unique | Login lookup |
| `refresh_tokens` | `refresh_tokens_admin_id_idx` | B-tree | Token lookup by admin |
| `categories` | `categories_name_unique` | Partial unique | Dedup on create |
| `categories` | `categories_slug_unique` | Partial unique | URL routing |
| `questions` | `questions_category_id_idx` | Partial B-tree | Filter by category |
| `sessions` | `sessions_pin_active_unique` | Partial unique | PIN validation on join |
| `sessions` | `sessions_status_idx` | Partial B-tree | Filter active sessions |
| `sessions` | `sessions_created_by_idx` | Partial B-tree | Filter sessions by admin (R2) |
| `session_questions` | `session_questions_session_id_idx` | B-tree | Load session questions |
| `players` | `players_session_id_idx` | B-tree | Load session players |
| `answers` | `answers_session_question_id_idx` | B-tree | Load answers for question |
| `answers` | `answers_player_id_idx` | B-tree | Load player history |

---

## R2 extensions (not in MVP)

When persistent player accounts are introduced in R2:

```sql
-- New table
CREATE TABLE users (
    id           UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    email        TEXT NOT NULL,
    display_name TEXT NOT NULL,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    deleted_at   TIMESTAMPTZ
);

-- players gains an optional FK
ALTER TABLE players ADD COLUMN user_id UUID REFERENCES users(id);
```

No existing queries break — `user_id` is nullable and existing code ignores it.

When multi-admin session visibility is introduced in R2:

`sessions.created_by` is already present and indexed. Enabling per-admin filtering
requires only a query change — no schema migration needed:

```sql
-- R2: filter sessions by owning admin
SELECT * FROM sessions
WHERE created_by = $1
  AND deleted_at IS NULL;
```