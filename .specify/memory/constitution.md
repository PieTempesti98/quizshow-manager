<!--
SYNC IMPACT REPORT
==================
Version change: [template] → 1.0.0
Modified principles: N/A (initial population from template)
Added sections:
  - Core Principles (5 principles defined)
  - MVP Scope Boundaries
  - Implementation Order & Workflow
  - Governance
Templates requiring updates:
  - .specify/templates/plan-template.md ✅ (Constitution Check section references these principles)
  - .specify/templates/spec-template.md ✅ (no constitution-specific changes needed)
  - .specify/templates/tasks-template.md ✅ (no constitution-specific changes needed)
Deferred TODOs: none
-->

# QuizShow Constitution

## Core Principles

### I. Backend-First Development

The backend (Go server, migrations, auth, REST endpoints, WebSocket hub) MUST be fully
implemented before any frontend code is generated. No React scaffolding, components, or
pages may be created until `docs/06-ui-flows.md` exists in the repository.

If a spec or plan includes frontend work and `docs/06-ui-flows.md` is absent, frontend
tasks MUST be skipped and flagged as blocked. Backend work continues unblocked.

**Rationale**: UI design decisions (component library, color palette, layout patterns)
have not been finalized. Generating frontend code before that document exists produces
inconsistent artifacts that must be discarded.

### II. Spec-Driven Development (NON-NEGOTIABLE)

Every feature MUST have a spec file under `specs/` before any implementation begins.
The specKit workflow MUST be followed in order:
`/speckit.specify` → `/speckit.plan` → `/speckit.tasks` → `/speckit.implement`

No code may be committed for a feature that lacks a corresponding spec file.

**Rationale**: Spec-first ensures alignment on requirements, data model, and API
contracts before implementation diverges. It also creates a traceable record of design
decisions.

### III. Architectural Simplicity (YAGNI)

The simplest solution that satisfies the current requirement MUST be chosen. Complexity
requires explicit justification tracked in the plan's Complexity Tracking table.

Specific mandates:
- No ORM — use `pgx` directly with explicit SQL in repository functions.
- No message broker (Redis, Kafka) in MVP — one hub goroutine per live session suffices.
- No Keycloak in MVP — the Go backend self-issues JWTs; the middleware is issuer-agnostic.
- No hard deletes — all entities use `deleted_at TIMESTAMPTZ NULL` (soft deletes only).

**Rationale**: Premature abstraction and infrastructure overhead impose maintenance cost
without delivering proportional value at MVP scale.

### IV. Data Integrity Standards

All data operations MUST conform to these non-negotiable conventions:

- **Soft deletes**: every table MUST have `deleted_at TIMESTAMPTZ NULL`; all queries
  MUST include `WHERE deleted_at IS NULL` unless intentionally reading deleted records.
- **Timestamps**: always UTC, ISO 8601 format (`TIMESTAMPTZ` in Postgres).
- **IDs**: UUID v4 for all primary keys.
- **API responses**: MUST follow the envelope `{ "data": ..., "error": null }` or
  `{ "data": null, "error": { "code": "...", "message": "..." } }`. No bare payloads.
- **Migrations**: managed with `golang-migrate`; files live in `backend/migrations/`.

**Rationale**: Consistent conventions prevent silent data loss, simplify auditing, and
make API consumers predictable.

### V. Real-Time Isolation

WebSocket and REST responsibilities MUST remain separated:

- **REST**: all CRUD and data management operations.
- **WebSocket**: server-to-client push events during a live session, plus client answer
  submission. Nothing else.
- Each active session MUST have exactly one hub goroutine managing its connected clients.
  Cross-session communication is not permitted at the hub level.

**Rationale**: Clean separation makes REST endpoints independently testable and
cacheable. Session-scoped hubs bound the blast radius of a session failure to that
session only.

## MVP Scope Boundaries

The following are **in scope** for MVP (Release 1):

- Admin login (JWT, no Keycloak)
- Question CRUD + CSV import + categories
- Session lifecycle: create → configure → launch → complete
- PIN-based player join (no account required)
- Full real-time loop: Presenter controls → Projection Screen displays → Player responds
- Basic post-session stats and leaderboard

The following are **explicitly out of scope** until Release 2+:

- Keycloak / SSO, persistent player accounts, match history
- Tournaments / brackets
- Media (images, audio) in questions
- Advanced analytics and PDF export
- Team/squad mode

Any work touching out-of-scope items MUST be flagged as out-of-scope in the spec and
rejected from the MVP implementation plan.

## Implementation Order & Workflow

Features MUST be implemented in this order:

1. **Backend** — Go server, DB migrations, auth, all REST endpoints, WebSocket hub.
2. **`docs/05-websocket-events.md`** and **`docs/06-ui-flows.md`** — must be authored
   and committed before any frontend work begins.
3. **Frontend** — React apps for Admin, Presenter, and Player views.

Within each feature, the specKit workflow governs task sequencing
(see Principle II). Tasks within a phase marked `[P]` may run in parallel.

## Governance

This constitution supersedes all other practices and conventions within this repository.

**Amendment procedure**: Any change to this constitution MUST:
1. Increment the version following semantic versioning (MAJOR for principle removal or
   redefinition; MINOR for new principle or material expansion; PATCH for wording fixes).
2. Update `LAST_AMENDED_DATE` to the amendment date.
3. Run the consistency propagation checklist against all templates in
   `.specify/templates/` and update any outdated references.
4. Be committed with message `docs: amend constitution to vX.Y.Z (<summary>)`.

**Compliance review**: Every spec review and plan review MUST include a Constitution
Check section that verifies compliance with all five principles before proceeding to
implementation.

**Versioning policy**: `MAJOR.MINOR.PATCH` — see amendment procedure above.

**Version**: 1.0.0 | **Ratified**: 2026-04-20 | **Last Amended**: 2026-04-20
