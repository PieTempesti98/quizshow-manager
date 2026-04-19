# QuizShow — Project Overview

## Vision

QuizShow is a web application for hosting live, interactive quiz shows. It replaces tools like Kahoot with a self-hosted, fully customizable platform designed for events, team building, and competitions.

The system consists of three distinct browser views that operate simultaneously during a live session:

- **Admin Panel** — the operator's back-office for managing questions, sessions, and viewing results
- **Presenter Mode + Projection Screen** — the live control room and the audience-facing display
- **Player View** — the mobile-first interface for quiz participants

## Release plan

### MVP (Release 1) — current scope

Core quiz loop: admin creates session → players join via PIN → presenter runs the quiz live → results visible at the end.

Full scope: see `docs/01-mvp-requirements.md`.

### Release 2

- Keycloak integration for SSO and persistent player accounts
- Tournament / bracket management
- Advanced statistics and PDF export
- Team/squad mode

### Release 3+

- Media (images, audio) in questions
- AI-assisted question generation
- Public leaderboards and championships
- Mobile app (React Native)

## Stack decisions

### Frontend: React + Vite + TypeScript

Chosen over Angular for lower boilerplate and faster iteration speed in a solo/small-team context. Three independent Vite apps share a local `ui-kit` package. TypeScript strict mode enforced throughout.

### Backend: Go + Fiber

Go's native concurrency model (goroutines + channels) is the right fit for managing multiple simultaneous live sessions, each with N WebSocket clients. The package-based structure maps cleanly to a future microservices decomposition. Fiber provides a fast, Express-like API surface without abstraction overhead.

### Realtime: WebSocket (native Go)

No message broker (Redis Pub/Sub, Kafka) in MVP. Each session has a single hub goroutine managing its clients. This is sufficient for the expected scale (hundreds of concurrent players per session, tens of concurrent sessions). A broker can be added in R2 if horizontal scaling is needed.

### Database: PostgreSQL

Relational model fits the domain well (questions → categories, sessions → players → answers). `pgx` driver used directly — no ORM — for full control over queries and migrations.

### Auth: JWT (self-issued, MVP) → Keycloak (R2)

The JWT validation middleware is issuer-agnostic. In MVP, the Go backend issues and validates its own tokens. In R2, Keycloak becomes the issuer — no middleware code changes required, only environment variable reconfiguration.

**Why not Keycloak in MVP?** Operational overhead is not justified when there is a single admin user and no persistent player accounts. The architecture is designed so the upgrade is seamless.

## Architecture decision records (ADR)

### ADR-001: Player identity is ephemeral in MVP

**Decision:** Players join a session with a nickname and PIN. No account creation required. Player state (nickname, score, answers) lives only for the session duration.

**Rationale:** Removes registration friction for participants. The primary value of MVP is validating the live quiz experience, not user management.

**Consequence:** No match history or persistent leaderboard across sessions in MVP.

---

### ADR-002: Questions are selected at session launch, not at session creation

**Decision:** When admin creates a session, they configure categories and question count. The actual question selection (random draw from the pool) happens when the session is launched live.

**Rationale:** Allows the question pool to be updated between creation and launch without invalidating the session configuration.

**Consequence:** Two admins could theoretically launch the same session template with different question draws.

---

### ADR-003: Soft deletes for all entities

**Decision:** No entity is ever hard-deleted from the database. All tables have `deleted_at TIMESTAMPTZ NULL`.

**Rationale:** Preserves referential integrity for historical session data (a deleted question should still appear in past session results). Supports future audit requirements.

**Consequence:** All queries must include `WHERE deleted_at IS NULL` unless intentionally querying deleted records.

---

### ADR-004: No ORM

**Decision:** Use `pgx` directly. Write explicit SQL in repository functions.

**Rationale:** Avoids hidden query generation, N+1 issues, and ORM-specific migration patterns. SQL is readable and reviewable. The domain model is not complex enough to justify an ORM's abstractions.

**Consequence:** More verbose repository code. Offset by clarity and control.

---

### ADR-005: WebSocket for push only, REST for CRUD

**Decision:** WebSocket events are exclusively used for server-to-client push during a live session (and client answer submission). All data management operations use REST.

**Rationale:** Clean separation of concerns. REST endpoints are independently testable and cacheable. WebSocket state is scoped to the session lifecycle.

**Consequence:** Clients must manage two connection types. The tradeoff is worth it for architectural clarity.
