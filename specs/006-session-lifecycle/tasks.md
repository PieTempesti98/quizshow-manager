# Tasks: Session Lifecycle — draft → lobby → active

**Input**: Design documents from `/specs/006-session-lifecycle/`  
**Prerequisites**: plan.md ✅, spec.md ✅, research.md ✅, data-model.md ✅, contracts/ ✅, quickstart.md ✅

**Organization**: Tasks are grouped by user story to enable independent implementation and testing of each story.  
**Tests**: No test tasks — not requested in the spec. Manual smoke tests are in `quickstart.md`.

## Format: `[ID] [P?] [Story] Description`

- **[P]**: Can run in parallel (different files, no dependencies on incomplete tasks)
- **[Story]**: Which user story this task belongs to (US1 = open-lobby, US2 = QR code, US3 = launch)

---

## Phase 1: Setup

**Purpose**: Add the new QR dependency before any code changes.

- [x] T001 Run `cd backend && go get github.com/skip2/go-qrcode` to add QR library to `backend/go.mod` and `backend/go.sum`

---

## Phase 2: Foundational (Blocking Prerequisites)

**Purpose**: Core types, errors, and JWT extension that all three user stories depend on. Must be complete before any story work begins.

**⚠️ CRITICAL**: No user story work can begin until this phase is complete.

- [x] T002 Add `ErrSessionNotInLobby` and `ErrInsufficientQuestions` sentinel errors, plus `OpenLobbyResult`, `LaunchResult`, `SessionEventBroadcaster` interface, and `NoopBroadcaster` implementation to `backend/internal/session/models.go`
- [x] T003 [P] Add `projectionClaims` struct and `IssueProjectionToken(sessionID uuid.UUID, cfg Config) (string, error)` function to `backend/internal/auth/token.go` (does not touch existing functions)
- [x] T004 [P] Add `OpenLobby(ctx context.Context, id uuid.UUID) (Session, error)` and `Launch(ctx context.Context, id uuid.UUID) (Session, int, error)` to the `SessionRepo` interface in `backend/internal/session/repository.go`

**Checkpoint**: Foundational types and interfaces defined — user story phases can proceed.

---

## Phase 3: User Story 1 — Open Lobby (Priority: P1) 🎯

**Goal**: `POST /sessions/:id/open-lobby` transitions a draft session to lobby status, returning the PIN and QR code URL.

**Independent Test**: Call `POST /api/v1/sessions/:id/open-lobby` (with admin token) on a draft session. Verify HTTP 200 with `status: "lobby"`, `pin`, and `qr_code_url`. Call again on the same session and verify HTTP 409. Call on a non-existent ID and verify HTTP 404.

### Implementation for User Story 1

- [x] T005 [US1] Implement `SessionRepository.OpenLobby` in `backend/internal/session/repository.go`: UPDATE sessions SET status='lobby' WHERE id=$1 AND status='draft' AND deleted_at IS NULL RETURNING ...; if 0 rows affected, check existence to distinguish 404 vs 409 (`ErrSessionNotFound` or `ErrSessionNotDraft`)
- [x] T006 [US1] Extend `Service` interface in `backend/internal/session/service.go` to add `OpenLobby(ctx context.Context, id uuid.UUID) (OpenLobbyResult, error)` and update `service` struct to hold `authCfg auth.Config`, `playerURL string`, and `broadcaster SessionEventBroadcaster`; update `NewService` signature to accept these new parameters
- [x] T007 [US1] Implement `service.OpenLobby` in `backend/internal/session/service.go`: call `s.repo.OpenLobby`, propagate sentinel errors, return `OpenLobbyResult{SessionID, PIN, QRCodeURL: "/api/v1/sessions/"+id+"/qr", Status}`
- [x] T008 [US1] Add `Handler.OpenLobby` method in `backend/internal/session/handler.go`: parse `:id` UUID (422 on bad format), call `h.svc.OpenLobby`, map `ErrSessionNotFound`→404, `ErrSessionNotDraft`→409 `SESSION_NOT_DRAFT`, success→200 JSON with `{session_id, pin, qr_code_url, status}`
- [x] T009 [US1] Update `backend/cmd/server/main.go`: read `PLAYER_APP_BASE_URL` env var (default `"http://localhost:5173"` — add inline placeholder comment), update `session.NewService(sessionRepo, cfg, playerURL, session.NewNoopBroadcaster())`, register `protected.Post("/sessions/:id/open-lobby", sessionHandler.OpenLobby)`

**Checkpoint**: `POST /open-lobby` is fully functional. Smoke test: steps 1–4 from quickstart.md.

---

## Phase 4: User Story 2 — QR Code (Priority: P2)

**Goal**: `GET /sessions/:id/qr` serves a PNG QR code encoding the player join URL (`{PLAYER_APP_BASE_URL}/join?pin={pin}`). No auth required.

**Independent Test**: Call `GET /api/v1/sessions/:id/qr` (no auth token). Verify HTTP 200 `Content-Type: image/png`. Save PNG and confirm it is a decodable QR code encoding the expected URL. Call with a non-existent ID and verify HTTP 404.

### Implementation for User Story 2

- [x] T010 [US2] Add `GetQR(ctx context.Context, id uuid.UUID) ([]byte, error)` to the `Service` interface in `backend/internal/session/service.go`
- [x] T011 [US2] Implement `service.GetQR` in `backend/internal/session/service.go`: call `s.repo.FindByID(ctx, id)` (propagate `ErrSessionNotFound`), build URL as `strings.TrimRight(s.playerURL,"/")+"/join?pin="+detail.PIN`, call `qrcode.Encode(url, qrcode.Medium, 256)` and return the PNG bytes; add `buildURL` private helper to avoid repeating TrimRight logic
- [x] T012 [US2] Add `Handler.GetQR` method in `backend/internal/session/handler.go`: parse `:id` UUID (422 on bad format), call `h.svc.GetQR`, map `ErrSessionNotFound`→404, success→`c.Set("Content-Type","image/png"); return c.Status(200).Send(png)`
- [x] T013 [US2] Register the QR endpoint on the public (non-protected) group in `backend/cmd/server/main.go`: `v1.Get("/sessions/:id/qr", sessionHandler.GetQR)` — must be on `v1`, not `protected`, to skip `RequireAdmin`

**Checkpoint**: `GET /sessions/:id/qr` serves a valid PNG. Smoke test: step 5 from quickstart.md.

---

## Phase 5: User Story 3 — Launch Session (Priority: P1)

**Goal**: `POST /sessions/:id/launch` transitions a lobby session to active: draws questions randomly into `session_questions`, sets `started_at`, issues a 12-hour projection JWT, and calls the no-op broadcaster.

**Independent Test**: Call `POST /api/v1/sessions/:id/launch` (admin token) on a lobby session with available questions. Verify HTTP 200 with `status: "active"`, a non-empty `projection_token`, `started_at`, and `question_count > 0`. Query DB to confirm `session_questions` rows exist. Call launch again and verify HTTP 409. Call on a session with an empty question pool and verify HTTP 422 `INSUFFICIENT_QUESTIONS`.

### Implementation for User Story 3

- [x] T014 [US3] Implement `SessionRepository.Launch` in `backend/internal/session/repository.go` as a single pgx transaction:
  1. `BEGIN`
  2. `SELECT status::text, question_count FROM sessions WHERE id=$1 AND deleted_at IS NULL FOR UPDATE` — no rows→`ErrSessionNotFound`, status!='lobby'→`ErrSessionNotInLobby`
  3. `SELECT q.id FROM questions q JOIN session_categories sc ON sc.category_id=q.category_id WHERE sc.session_id=$1 AND q.deleted_at IS NULL ORDER BY RANDOM() LIMIT $2` — empty→`ErrInsufficientQuestions` (rollback)
  4. Loop over drawn IDs: `INSERT INTO session_questions (session_id, question_id, position) VALUES ($1,$2,$3)` with position 1..N
  5. `UPDATE sessions SET status='active', started_at=now(), updated_at=now() WHERE id=$1 RETURNING ...`
  6. `COMMIT` — return `(Session, drawnCount, nil)`
- [x] T015 [US3] Add `Launch(ctx context.Context, id uuid.UUID) (LaunchResult, error)` to the `Service` interface in `backend/internal/session/service.go`
- [x] T016 [US3] Implement `service.Launch` in `backend/internal/session/service.go`: call `s.repo.Launch`, propagate sentinel errors, call `auth.IssueProjectionToken(sess.ID, s.authCfg)`, build `projectionURL` using `buildURL`, call `s.broadcaster.BroadcastSessionStarted(...)`, return `LaunchResult{SessionID, Status, QuestionCount: drawn, ProjectionToken, ProjectionURL, StartedAt: *sess.StartedAt}`
- [x] T017 [US3] Add `Handler.Launch` method in `backend/internal/session/handler.go`: parse `:id` UUID (422 on bad format), call `h.svc.Launch`, map `ErrSessionNotFound`→404, `ErrSessionNotInLobby`→409 `SESSION_NOT_IN_LOBBY`, `ErrInsufficientQuestions`→422 `INSUFFICIENT_QUESTIONS`, success→200 JSON with `{session_id, status, question_count, projection_token, projection_url, started_at}`
- [x] T018 [US3] Register `protected.Post("/sessions/:id/launch", sessionHandler.Launch)` in `backend/cmd/server/main.go`

**Checkpoint**: `POST /launch` is fully functional. Smoke test: steps 6–8 from quickstart.md plus DB verification.

---

## Phase 6: Polish & Cross-Cutting Concerns

**Purpose**: Verify end-to-end correctness and update project tracking.

- [x] T019 Run full smoke test sequence from `specs/006-session-lifecycle/quickstart.md` against live Docker DB; confirm all 8 steps pass
- [x] T020 [P] Verify `projection_token` JWT structure: decode and confirm `session_id`, `role: "projection"`, and ~12h expiry claims are present
- [x] T021 [P] Verify partial pool tolerance: create a session with `question_count: 10` but only 6 questions available in the categories; confirmed `POST /launch` returns 200 with `question_count: 6`
- [x] T022 Update `PLAN.md` to reflect feature #6 completion: mark `open-lobby`, `qr`, and `launch` endpoints as done in section 1.4

---

## Dependencies & Execution Order

### Phase Dependencies

- **Setup (Phase 1)**: No dependencies — start immediately.
- **Foundational (Phase 2)**: Depends on Phase 1. T003 and T004 can run in parallel after T002. BLOCKS all user story phases.
- **US1 — Open Lobby (Phase 3)**: Depends on Phase 2. T005 → T006 → T007 → T008 → T009 (sequential within story).
- **US2 — QR Code (Phase 4)**: Depends on Phase 2, and on T006/T009 for the updated service struct and `main.go` wiring. Can start once US1's T006 and T009 are done.
- **US3 — Launch (Phase 5)**: Depends on Phase 2, and on T006/T009 for the updated service struct. Can start once US1's T006 and T009 are done. T014, T015, T016, T017 are sequential; T018 requires T009 (main.go already updated).
- **Polish (Phase 6)**: Depends on all three user stories being complete.

### User Story Dependencies

- **US1 (P1)**: Foundational only. No dependency on US2 or US3.
- **US2 (P2)**: Depends on US1's T006 (service interface/struct) and T009 (main.go PLAYER_APP_BASE_URL + service construction). Can be worked in parallel with US3 once US1's service groundwork is laid.
- **US3 (P1)**: Depends on US1's T006 and T009 for the same reason. Can be worked in parallel with US2.

### Within Each User Story

- Repository → Service interface extension → Service implementation → Handler → main.go registration

### Parallel Opportunities

Within **Phase 2**: T003 (`auth/token.go`) and T004 (repo interface) can run in parallel after T002 (models.go) completes.

Within **Phase 4 + Phase 5**: Once US1's service layer (T006) and main.go (T009) are done, US2 and US3 implementation tasks can proceed in parallel.

---

## Parallel Example: Foundational Phase

```
After T002 completes:
  └── T003: auth/token.go — IssueProjectionToken
  └── T004: repository.go — interface additions (no conflicts)
```

## Parallel Example: US2 + US3 (after US1 T006 + T009)

```
US2:                                US3:
T010 → T011 → T012 → T013          T014 → T015 → T016 → T017 → T018
(service.go, handler.go, main.go)   (repository.go, service.go, handler.go, main.go)
```

Note: T013 (US2) and T018 (US3) both touch `main.go` — do NOT run these in parallel; merge into one edit pass.

---

## Implementation Strategy

### MVP First (US1 only)

1. Complete Phase 1: Setup (T001)
2. Complete Phase 2: Foundational (T002, T003, T004)
3. Complete Phase 3: US1 — Open Lobby (T005–T009)
4. **STOP and VALIDATE**: `POST /open-lobby` works end to end
5. Continue to US2 and US3

### Incremental Delivery

1. T001 → T002–T004 → Foundation ready
2. T005–T009 → Open Lobby live → demo-able
3. T010–T013 → QR code live → demo-able
4. T014–T018 → Launch live → full session lifecycle complete
5. T019–T022 → Polish, validate, update PLAN.md

---

## Notes

- All tasks follow: `- [ ] [TaskID] [P?] [Story?] Description with file path`
- T003 and T004 are marked [P] — they touch different files (`auth/token.go` vs `session/repository.go`)
- T013 and T018 both modify `main.go` — handle in one combined edit to avoid conflicts
- No test tasks generated (not requested in spec); smoke tests are in `quickstart.md`
- `buildURL` helper (T011) is reused by `GetQR` (US2) and `Launch` (US3) — implement once in service.go
