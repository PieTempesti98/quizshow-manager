# QuizShow — Project Constitution

> This document is the authoritative source of principles for all development on this project.
> Claude Code must adhere to these principles when generating plans, tasks, and implementation.

## 1. Core principles

### 1.1 Spec-first, always
Never implement a feature without a spec file in `specs/`. If a spec does not exist, create it with `/speckit.specify` before writing any code.

### 1.2 MVP discipline
The MVP scope is fixed in `docs/01-mvp-requirements.md`. Do not implement features marked as R2+ even if they seem small or convenient. Flag scope creep explicitly and ask before proceeding.

### 1.3 Correctness over speed
Prefer simple, correct implementations over clever or premature optimizations. The codebase will grow into microservices — write clean, well-bounded packages from the start.

### 1.4 Fail loudly
Errors must be explicit and structured. Never swallow errors silently. All API errors return the standard envelope with a machine-readable `code` and a human-readable `message`.

## 2. Backend (Go)

### 2.1 Package structure
Follow standard Go layout. Business logic lives in `internal/`. No logic in `cmd/`. Each domain (auth, question, session, player, websocket) is its own package with clear interfaces.

### 2.2 No global state
Pass dependencies explicitly (repositories, services, config) via constructor injection. No init() side effects, no package-level singletons.

### 2.3 Database access
Use `pgx` directly — no ORM. Write explicit SQL. Queries live in repository structs, not scattered across handlers. Use prepared statements for queries executed at high frequency.

### 2.4 Migrations
All schema changes go through `golang-migrate`. Never modify the database schema manually. Migration files are named `{version}_{description}.up.sql` / `.down.sql`.

### 2.5 WebSocket
Each active session has exactly one hub goroutine. Clients register/unregister via channels. The hub never touches HTTP — it only speaks to handlers through typed messages. No goroutine leaks: always handle client disconnection.

### 2.6 Auth middleware
JWT validation is stateless. The middleware reads the `Authorization: Bearer <token>` header, validates signature and expiry, and injects claims into the request context. The issuer URL is read from environment — never hardcoded.

### 2.7 Prefer well-established libraries
Do not implement cryptography, JWT handling, UUID generation, or database drivers from scratch. Use: `golang-jwt/jwt`, `google/uuid`, `jackc/pgx`, `gofiber/fiber`.

## 3. Frontend (React)

### 3.1 Three separate apps
`frontend/admin`, `frontend/presenter`, `frontend/player` are independent Vite apps. They share types and UI components via a local `frontend/ui-kit` package (symlinked or via workspace).

### 3.2 TypeScript strict mode
All frontend code uses TypeScript with `strict: true`. No `any` types. API response types are generated from or manually mirrored against the Go structs.

### 3.3 State management
Use React Query (TanStack Query) for server state. Use Zustand for client-only UI state (e.g., current session phase, presenter controls). Do not use Redux.

### 3.4 WebSocket in React
WebSocket connections are managed via a custom hook (`useSessionSocket`). Components subscribe to typed events — they never interact with the raw WebSocket. Reconnection logic is handled in the hook.

### 3.5 Mobile-first for player view
The player app targets mobile devices. All interactive elements (answer buttons) must be at least 48x48px tap target. No hover-only interactions. Test at 375px viewport width.

### 3.6 Accessibility
Projection Screen must have high contrast (WCAG AA minimum) and large font sizes — it will be displayed on a projector from a distance.

## 4. API design

### 4.1 REST for CRUD, WebSocket for realtime
HTTP REST endpoints handle all create/read/update/delete operations. WebSocket is exclusively for push events during a live session. Do not use WebSocket for data fetching.

### 4.2 Versioning
All REST endpoints are prefixed with `/api/v1/`. This is not negotiable — it enables future non-breaking additions.

### 4.3 Response envelope
```json
// Success
{ "data": { ... } }

// Error  
{ "error": { "code": "SESSION_NOT_FOUND", "message": "Session with ID ... not found" } }
```
Never return a non-2xx status with a success body, or a 2xx status with an error body.

### 4.4 Authentication
Protected endpoints require `Authorization: Bearer <token>`. Unauthenticated requests return 401. Insufficient permissions return 403. The token carries the role (`admin` or `player`).

## 5. Data

### 5.1 UUIDs everywhere
All primary keys are UUID v4. No integer sequences as public IDs.

### 5.2 Soft deletes
All entities have `deleted_at TIMESTAMPTZ`. Queries always filter `WHERE deleted_at IS NULL` unless explicitly retrieving deleted records.

### 5.3 UTC timestamps
All timestamps are stored and returned in UTC. Clients handle timezone conversion for display.

### 5.4 No business logic in SQL
SQL is for data retrieval and persistence. Aggregations and computed fields that require business rules live in Go, not in SQL views or triggers.

## 6. Testing

### 6.1 Unit tests for business logic
Every function in `internal/` that contains business logic (scoring, PIN generation, question selection) must have unit tests. Table-driven tests are preferred.

### 6.2 Integration tests for repositories
Repository functions are tested against a real PostgreSQL instance (use Docker in CI). Use `testcontainers-go` or a dedicated test DB.

### 6.3 No tests for HTTP handlers in MVP
Handler tests are deferred to R2. Focus test coverage on the domain logic and repositories in MVP.

## 7. Infrastructure

### 7.1 Docker Compose for everything
All services (Go backend, PostgreSQL, frontend dev server) run via Docker Compose. A single `docker-compose up` must start the entire development environment.

### 7.2 Environment variables only
No configuration files with secrets. All environment-specific values (DB connection string, JWT secret, allowed origins) are injected via environment variables. A `.env.example` file documents all required variables with placeholder values.

### 7.3 CORS
The backend explicitly configures CORS allowed origins from an environment variable. In development, `localhost:*` is allowed. In production, only the specific frontend domains.