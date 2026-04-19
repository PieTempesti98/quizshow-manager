-- Migration: 001_initial_schema.up.sql
-- QuizShow — initial database schema
-- Run with: golang-migrate up

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
    created_at              TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at              TIMESTAMPTZ NOT NULL DEFAULT now(),
    deleted_at              TIMESTAMPTZ
);

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
