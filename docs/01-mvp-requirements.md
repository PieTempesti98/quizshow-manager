# QuizShow — MVP Requirements

## Summary

| View | Stories | Status |
|---|---|---|
| Admin Panel | 11 | Defined |
| Presenter Mode | 6 | Defined |
| Projection Screen | 4 | Defined |
| Player View | 5 | Defined |
| **Total** | **26** | |

---

## Admin Panel

### Authentication

**US-A01 — Login**
As an admin, I want to log in with email and password so that I can access the management panel.

Acceptance criteria:
- Email + password form with basic validation (non-empty, valid email format)
- On success: receive JWT access token (15min TTL) and refresh token (7 days TTL), stored in httpOnly cookies
- On failure: generic error message — do not indicate whether email or password is wrong
- Redirect to dashboard on success

**US-A02 — Logout and token refresh**
As an admin, I want to log out and have my session invalidated server-side so that no active sessions are left open.

Acceptance criteria:
- Logout call revokes the refresh token in the database
- Access token expires naturally (short TTL); no client-side blacklisting needed
- Redirect to login page on logout
- Expired access tokens are transparently refreshed using the refresh token cookie

---

### Question Management

**US-Q01 — Create question**
As an admin, I want to create a question with: question text, 4 answer options, the index of the correct answer, a category, and a difficulty level so that it is available for quiz sessions.

Acceptance criteria:
- All fields required except difficulty (default: medium)
- Difficulty options: easy / medium / hard
- Answer options: exactly 4, each non-empty
- Correct answer: one of the 4 options, indicated by index (0–3)
- Validation on both client and server
- On success: question appears immediately in the question list

**US-Q02 — Edit / delete question**
As an admin, I want to edit an existing question or delete it so that I can keep the question bank accurate.

Acceptance criteria:
- All fields editable, same validation as creation
- Deletion is soft (sets `deleted_at`); question disappears from the list but is preserved in historical session data
- A question used in an active (in-progress) session cannot be deleted — show a clear error message
- Confirmation dialog before deletion

**US-Q03 — Import questions from CSV**
As an admin, I want to upload a CSV file to bulk-import questions so that I can populate the question bank efficiently.

Acceptance criteria:
- A downloadable CSV template is available in the UI
- CSV columns: `text`, `option_a`, `option_b`, `option_c`, `option_d`, `correct_index` (0–3), `category_name`, `difficulty` (easy/medium/hard)
- Validation is performed row by row; results report shows which rows succeeded and which failed with reason
- Import is transactional: either all valid rows are imported or none (if any row fails validation, the entire import is rejected — configurable toggle)
- Maximum 500 questions per import
- Duplicate detection: a question with identical text in the same category is flagged as a warning (not a hard error)

**US-Q04 — Manage categories and difficulty**
As an admin, I want to create, rename, and delete categories so that I can organize the question bank.

Acceptance criteria:
- Categories are global (not per-session)
- Category name: unique, max 50 characters
- A category with active (non-deleted) questions cannot be deleted — show count of blocking questions
- Difficulty is a fixed enum (easy / medium / hard) — not configurable per category

**US-Q05 — List and search questions**
As an admin, I want to browse the question bank with filters and search so that I can find and manage specific questions.

Acceptance criteria:
- Server-side pagination: 25 questions per page
- Filters: category (multi-select), difficulty (multi-select), free text search on question text
- Filters are combinable
- Response time < 300ms for standard queries
- Column sort: by creation date (default desc), by category

---

### Session Management

**US-S01 — Create and configure session**
As an admin, I want to create a quiz session with configurable parameters so that I can prepare it before going live.

Acceptance criteria:
- Parameters: session name (required), categories to include (at least one), number of questions (min 1, max 50), time per question in seconds (options: 10 / 20 / 30 / 60), points per correct answer (default: 100), speed bonus enabled/disabled
- Speed bonus rule (when enabled): players who answer in the first 50% of the timer receive 1.5× points
- If the selected categories contain fewer questions than requested, show a warning but allow saving (the session will use all available questions)
- Session is saved in `draft` status

**US-S02 — Generate PIN and QR code**
As an admin, I want each session to have a unique 6-digit PIN and corresponding QR code so that players can join easily.

Acceptance criteria:
- PIN is generated automatically when the session is created
- PIN is unique among all currently active sessions (not globally unique across history)
- QR code encodes the player join URL with the PIN pre-filled
- PIN and QR code are displayed prominently in the session detail view
- PIN becomes invalid when the session moves to `completed` or `cancelled` status

**US-S03 — Launch live session**
As an admin, I want to launch a session live so that it transitions to the Presenter Mode and becomes joinable by players.

Acceptance criteria:
- "Launch" button available only for sessions in `draft` status
- On launch: session status changes to `active`, questions are randomly selected from the configured pool
- Presenter Mode opens in the same tab (or a new tab — TBD during implementation)
- A shareable link for the Projection Screen is displayed, intended to be opened in a separate browser window for projection
- Players who are already waiting in the lobby (connected via WebSocket) receive a `session_started` event

**US-S04 — Session history**
As an admin, I want to see a list of past sessions so that I can review previous quiz results.

Acceptance criteria:
- Shows only sessions in `completed` or `cancelled` status
- Columns: session name, date/time, participant count, top score, status
- Sorted by date descending
- Clicking a session opens the post-session statistics view (US-ST01 / US-ST02)

---

### Statistics (MVP-lite)

**US-ST01 — Post-session question breakdown**
As an admin, I want to see a per-question breakdown of a completed session so that I can understand which questions were easy or hard.

Acceptance criteria:
- For each question: question text, correct answer, count of correct answers, count of wrong answers, count of no-answers (player did not respond in time)
- Sorted by question order in the session
- Available immediately after the session is completed

**US-ST02 — Final leaderboard**
As an admin, I want to see and export the final leaderboard of a session so that I can share results.

Acceptance criteria:
- Columns: rank, nickname, total score
- All participants listed (not just top N)
- Export as CSV: one click, filename `{session-name}-{date}-leaderboard.csv`

---

## Presenter Mode

**US-P01 — Lobby view**
As a presenter, I want to see the waiting lobby with the list of connected players and the session PIN/QR code so that I can wait for participants to join before starting.

Acceptance criteria:
- Player list updates in real-time via WebSocket (no page refresh)
- Each player shown with nickname and a generated avatar (initials-based color avatar)
- PIN displayed large and clearly; QR code visible alongside
- "Start Quiz" button enabled only when at least 1 player is connected
- Player count shown (e.g., "8 players connected")

**US-P02 — Advance questions manually**
As a presenter, I want to manually advance to the next question after the reveal so that I control the quiz pace.

Acceptance criteria:
- "Next Question" button appears after the reveal phase of each question
- Pressing it broadcasts a `question_start` event to all clients simultaneously
- The button is disabled during the active timer phase (cannot skip a question mid-timer... unless using US-P03 pause)
- On the last question, the button label changes to "End Quiz" and triggers session completion

**US-P03 — Timer control**
As a presenter, I want to see the countdown for the current question and be able to pause it if needed so that I can manage unexpected interruptions.

Acceptance criteria:
- Timer displayed prominently as a countdown (seconds remaining)
- Pause button: halts the timer on all clients simultaneously (`timer_paused` event)
- Resume button: restarts the timer from where it paused (`timer_resumed` event)
- When the timer reaches zero, the reveal phase starts automatically (no manual trigger needed)
- Pausing does not reset the timer

**US-P04 — Live response counter**
As a presenter, I want to see how many players have already submitted an answer for the current question so that I know when participation is complete.

Acceptance criteria:
- Counter displayed as "X / Y have answered" (X = answered, Y = total connected players)
- Updates in real-time via WebSocket on every answer received
- Does not reveal which answer was selected by whom before the reveal

**US-P05 — Reveal answer and intermediate leaderboard**
As a presenter, I want the correct answer to be revealed and the top 5 leaderboard shown after each question so that the session has drama and flow.

Acceptance criteria:
- Reveal triggered automatically at timer end, or manually via a "Reveal" button (available after timer ends)
- Reveal broadcasts `question_reveal` event to all clients simultaneously
- Top 5 leaderboard shown in the presenter view and on the Projection Screen
- Scores are calculated server-side at reveal time

**US-P06 — End session early**
As a presenter, I want to be able to end the session at any time so that I can handle technical issues or time constraints.

Acceptance criteria:
- "End Session" button always visible in the presenter toolbar
- Confirmation dialog required before ending
- On confirm: session status set to `completed`, `session_ended` event broadcast to all clients
- All clients redirect to the final results screen

---

## Projection Screen

> The Projection Screen is a read-only view. It receives events via WebSocket and renders them. No user interaction. Optimized for full-screen display on a projector.

**US-PR01 — Lobby display**
The screen shows the session PIN large and prominently, the QR code, and the growing list of joined player nicknames while waiting for the presenter to start.

Acceptance criteria:
- Font size: PIN at minimum 96px, QR code at minimum 200×200px
- Player nicknames auto-scroll if the list exceeds the screen
- Updates in real-time via WebSocket

**US-PR02 — Question display**
The screen shows the current question text, four answer options labeled A / B / C / D with distinct background colors, the question number (e.g., "Question 3 of 10"), and an animated countdown timer.

Acceptance criteria:
- Answer option colors: A = blue, B = orange, C = purple, D = green (consistent with player view)
- Countdown timer: large, animated (shrinks or changes color in last 5 seconds)
- Question text: minimum 36px, centered, readable from 5 meters
- Options: minimum 28px

**US-PR03 — Reveal display**
After the reveal event, the screen highlights the correct answer, shows a bar chart of answer distribution (% who chose A, B, C, D), and displays the top 5 leaderboard.

Acceptance criteria:
- Correct answer option highlighted in green; incorrect options dimmed
- Distribution bars animated (grow from 0 to final value)
- Leaderboard: rank, nickname, score — top 5 only

**US-PR04 — Final results display**
At session end, the screen shows the full final leaderboard with rank, nickname, and score.

Acceptance criteria:
- Top 3 visually distinguished (podium style: 1st largest, 2nd/3rd smaller)
- Remains on screen until the presenter closes or navigates away
- Full leaderboard scrollable if more than ~15 players

---

## Player View

> Mobile-first. Minimum supported viewport: 375px wide. All interactive elements minimum 48×48px tap target.

**US-PL01 — Join session**
As a player, I want to enter the 6-digit PIN and a nickname to join a quiz session without registering an account.

Acceptance criteria:
- PIN: numeric input, 6 digits
- Nickname: 2–20 characters, alphanumeric + spaces + common punctuation
- Nickname must be unique within the session — if taken, show error and prompt to choose another
- If PIN does not exist or session is not in `active`/`lobby` status: clear error message
- If session is already in progress (past lobby phase): joining is not allowed in MVP

**US-PL02 — Lobby waiting screen**
As a player, I want to see a waiting screen with the list of other participants after joining so that I know the quiz hasn't started yet.

Acceptance criteria:
- "Waiting for the quiz to start..." message prominent
- List of connected player nicknames visible
- Player's own nickname highlighted
- Updates in real-time when new players join

**US-PL03 — Answer a question**
As a player, I want to see the current question and tap one of the four answer buttons so that I can participate in the quiz.

Acceptance criteria:
- Question text and 4 answer buttons (A / B / C / D) with matching colors as Projection Screen
- Buttons fill most of the mobile screen (minimum 64px height each)
- After tapping: selected button shows a "selected" state, all buttons disabled (no double answer)
- The correct/incorrect result is NOT shown at this point — only after the presenter triggers reveal
- If the timer expires before the player answers: all buttons are disabled, question is marked as unanswered

**US-PL04 — Post-reveal feedback**
As a player, I want to see whether my answer was correct, how many points I earned, and my current rank after each question's reveal so that I stay engaged.

Acceptance criteria:
- Correct answer highlighted; player's chosen answer highlighted (green if correct, red if wrong)
- Points earned this round shown with animation (e.g., "+100" or "+150" with bonus)
- Current total score and rank shown (e.g., "Score: 750 — You are #3")
- Rank calculated server-side

**US-PL05 — Final results screen**
As a player, I want to see the final leaderboard at the end of the session so that I know how I finished.

Acceptance criteria:
- Full leaderboard with rank, nickname, score
- Player's own row highlighted
- A "Play Again" or "Join Another Quiz" button (links back to the PIN entry screen)
