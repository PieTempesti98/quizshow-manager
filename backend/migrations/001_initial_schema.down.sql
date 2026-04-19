-- Migration: 001_initial_schema.down.sql
-- Rollback: drops all tables and types created in 001_initial_schema.up.sql

DROP TRIGGER IF EXISTS sessions_updated_at ON sessions;
DROP TRIGGER IF EXISTS questions_updated_at ON questions;
DROP TRIGGER IF EXISTS categories_updated_at ON categories;
DROP TRIGGER IF EXISTS admins_updated_at ON admins;
DROP FUNCTION IF EXISTS set_updated_at();

DROP TABLE IF EXISTS answers;
DROP TABLE IF EXISTS players;
DROP TABLE IF EXISTS session_questions;
DROP TABLE IF EXISTS session_categories;
DROP TABLE IF EXISTS sessions;
DROP TABLE IF EXISTS questions;
DROP TABLE IF EXISTS categories;
DROP TABLE IF EXISTS refresh_tokens;
DROP TABLE IF EXISTS admins;

DROP TYPE IF EXISTS session_status;
DROP TYPE IF EXISTS difficulty_level;

DROP EXTENSION IF EXISTS "pgcrypto";
