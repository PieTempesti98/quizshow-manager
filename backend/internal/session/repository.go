package session

import (
	"context"
	"errors"
	"fmt"
	"math"
	"math/rand"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

// NextQuestionData holds the question payload for advancing to the next question.
type NextQuestionData struct {
	SessionQuestionID uuid.UUID
	Position          int
	Total             int
	Question          QuestionSummary
	TimeLimitS        int
	AskedAt           time.Time
}

// ActiveQuestionData holds metadata about the question currently in progress.
type ActiveQuestionData struct {
	SessionQuestionID uuid.UUID
	SessionID         uuid.UUID
	TimeLimitS        int
	AskedAt           time.Time
}

// RevealData holds the payload returned after scoring a question reveal.
type RevealData struct {
	SessionQuestionID  uuid.UUID
	CorrectIndex       int
	AnswerDistribution []AnswerDistributionItem
	Top5               []LeaderboardEntry
	RevealedAt         time.Time
}

// SessionRepo defines the persistence operations for sessions.
type SessionRepo interface {
	Create(ctx context.Context, s SessionCreate, adminID uuid.UUID) (Session, int, error)
	List(ctx context.Context, f SessionFilter) (SessionListResult, error)
	FindByID(ctx context.Context, id uuid.UUID) (SessionDetail, error)
	Update(ctx context.Context, id uuid.UUID, u SessionUpdate) (Session, error)
	Delete(ctx context.Context, id uuid.UUID) error
	CountAvailableQuestions(ctx context.Context, sessionID uuid.UUID) (int, error)
	ValidateCategoryIDs(ctx context.Context, ids []uuid.UUID) error
	OpenLobby(ctx context.Context, id uuid.UUID) (Session, error)
	Launch(ctx context.Context, id uuid.UUID) (Session, int, error)
	NextQuestion(ctx context.Context, id uuid.UUID) (NextQuestionData, error)
	GetActiveQuestion(ctx context.Context, id uuid.UUID) (ActiveQuestionData, error)
	ShiftQuestionAskedAt(ctx context.Context, sessionQuestionID uuid.UUID, delta time.Duration) error
	RevealQuestion(ctx context.Context, id uuid.UUID) (RevealData, error)
	EndSession(ctx context.Context, id uuid.UUID, reason string) (Session, error)
	JoinPlayer(ctx context.Context, sessionID uuid.UUID, pin string, nickname string, avatarColor string) (Player, Session, int, error)
	SubmitAnswer(ctx context.Context, sessionID uuid.UUID, playerID uuid.UUID, sessionQuestionID uuid.UUID, chosenIndex int16, now time.Time) (Answer, bool, int, int, error)
}


// SessionRepository implements SessionRepo using pgxpool.
type SessionRepository struct {
	pool *pgxpool.Pool
}

// NewRepository constructs a SessionRepository.
func NewRepository(pool *pgxpool.Pool) *SessionRepository {
	return &SessionRepository{pool: pool}
}

func (r *SessionRepository) ValidateCategoryIDs(ctx context.Context, ids []uuid.UUID) error {
	const q = `SELECT COUNT(*) FROM categories WHERE id = ANY($1) AND deleted_at IS NULL`
	var found int
	if err := r.pool.QueryRow(ctx, q, ids).Scan(&found); err != nil {
		return fmt.Errorf("session repo: validate category ids: %w", err)
	}
	if found != len(ids) {
		return fmt.Errorf("VALIDATION_ERROR: one or more category_ids do not exist or are deleted")
	}
	return nil
}

func (r *SessionRepository) CountAvailableQuestions(ctx context.Context, sessionID uuid.UUID) (int, error) {
	const q = `
		SELECT COUNT(DISTINCT q.id)
		FROM questions q
		JOIN session_categories sc ON sc.category_id = q.category_id
		WHERE sc.session_id = $1
		  AND q.deleted_at IS NULL`

	var count int
	if err := r.pool.QueryRow(ctx, q, sessionID).Scan(&count); err != nil {
		return 0, fmt.Errorf("session repo: count available questions: %w", err)
	}
	return count, nil
}

func generatePIN() string {
	return fmt.Sprintf("%06d", rand.Intn(1_000_000))
}

func (r *SessionRepository) Create(ctx context.Context, s SessionCreate, adminID uuid.UUID) (Session, int, error) {
	pointsPerAnswer := 100
	if s.PointsPerAnswer != nil {
		pointsPerAnswer = *s.PointsPerAnswer
	}
	speedBonus := false
	if s.SpeedBonusEnabled != nil {
		speedBonus = *s.SpeedBonusEnabled
	}

	const insertSession = `
		INSERT INTO sessions (name, pin, question_count, time_per_question_s, points_per_answer, speed_bonus_enabled, created_by)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		RETURNING id, name, pin, status::text, question_count, time_per_question_s,
		          points_per_answer, speed_bonus_enabled, started_at, ended_at,
		          created_by, created_at, updated_at, deleted_at`

	var sess Session
	var err error
	for attempt := 0; attempt < 10; attempt++ {
		pin := generatePIN()
		err = r.pool.QueryRow(ctx, insertSession,
			s.Name, pin, s.QuestionCount, s.TimePerQuestionS,
			pointsPerAnswer, speedBonus, adminID,
		).Scan(
			&sess.ID, &sess.Name, &sess.PIN, &sess.Status,
			&sess.QuestionCount, &sess.TimePerQuestionS,
			&sess.PointsPerAnswer, &sess.SpeedBonusEnabled,
			&sess.StartedAt, &sess.EndedAt,
			&sess.CreatedBy, &sess.CreatedAt, &sess.UpdatedAt, &sess.DeletedAt,
		)
		if err == nil {
			break
		}
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			continue
		}
		return Session{}, 0, fmt.Errorf("session repo: create: %w", err)
	}
	if err != nil {
		return Session{}, 0, fmt.Errorf("session repo: create: pin exhausted: %w", err)
	}

	const insertCats = `INSERT INTO session_categories (session_id, category_id) VALUES ($1, $2)`
	for _, catID := range s.CategoryIDs {
		if _, err := r.pool.Exec(ctx, insertCats, sess.ID, catID); err != nil {
			return Session{}, 0, fmt.Errorf("session repo: create categories: %w", err)
		}
	}

	available, err := r.CountAvailableQuestions(ctx, sess.ID)
	if err != nil {
		return Session{}, 0, err
	}

	return sess, available, nil
}

func (r *SessionRepository) List(ctx context.Context, f SessionFilter) (SessionListResult, error) {
	clauses := []string{"s.deleted_at IS NULL"}
	args := []any{}
	n := 1

	if len(f.Statuses) > 0 {
		clauses = append(clauses, fmt.Sprintf("s.status::text = ANY($%d)", n))
		args = append(args, f.Statuses)
		n++
	}

	where := strings.Join(clauses, " AND ")

	var total int
	countQ := fmt.Sprintf(`SELECT COUNT(*) FROM sessions s WHERE %s`, where)
	if err := r.pool.QueryRow(ctx, countQ, args...).Scan(&total); err != nil {
		return SessionListResult{}, fmt.Errorf("session repo: list count: %w", err)
	}

	totalPages := 0
	if total > 0 {
		totalPages = (total + f.PerPage - 1) / f.PerPage
	}
	offset := (f.Page - 1) * f.PerPage

	dataQ := fmt.Sprintf(`
		SELECT s.id, s.name, s.pin, s.status::text,
		       s.question_count, s.time_per_question_s, s.points_per_answer,
		       s.speed_bonus_enabled, s.started_at, s.ended_at,
		       s.created_by, s.created_at, s.updated_at, s.deleted_at,
		       COALESCE(p.player_count, 0)
		FROM sessions s
		LEFT JOIN (
			SELECT session_id, COUNT(*) AS player_count
			FROM players
			GROUP BY session_id
		) p ON p.session_id = s.id
		WHERE %s
		ORDER BY s.created_at DESC
		LIMIT $%d OFFSET $%d`, where, n, n+1)

	dataArgs := make([]any, len(args)+2)
	copy(dataArgs, args)
	dataArgs[len(args)] = f.PerPage
	dataArgs[len(args)+1] = offset

	rows, err := r.pool.Query(ctx, dataQ, dataArgs...)
	if err != nil {
		return SessionListResult{}, fmt.Errorf("session repo: list: %w", err)
	}
	defer rows.Close()

	items := make([]SessionListItem, 0)
	for rows.Next() {
		var item SessionListItem
		if err := rows.Scan(
			&item.ID, &item.Name, &item.PIN, &item.Status,
			&item.QuestionCount, &item.TimePerQuestionS, &item.PointsPerAnswer,
			&item.SpeedBonusEnabled, &item.StartedAt, &item.EndedAt,
			&item.CreatedBy, &item.CreatedAt, &item.UpdatedAt, &item.DeletedAt,
			&item.PlayerCount,
		); err != nil {
			return SessionListResult{}, fmt.Errorf("session repo: list scan: %w", err)
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return SessionListResult{}, fmt.Errorf("session repo: list: %w", err)
	}

	return SessionListResult{
		Sessions:   items,
		Total:      total,
		Page:       f.Page,
		PerPage:    f.PerPage,
		TotalPages: totalPages,
	}, nil
}

func (r *SessionRepository) FindByID(ctx context.Context, id uuid.UUID) (SessionDetail, error) {
	const sessQ = `
		SELECT s.id, s.name, s.pin, s.status::text,
		       s.question_count, s.time_per_question_s, s.points_per_answer,
		       s.speed_bonus_enabled, s.started_at, s.ended_at,
		       s.created_by, s.created_at, s.updated_at, s.deleted_at,
		       COALESCE(p.player_count, 0)
		FROM sessions s
		LEFT JOIN (
			SELECT session_id, COUNT(*) AS player_count
			FROM players
			GROUP BY session_id
		) p ON p.session_id = s.id
		WHERE s.id = $1 AND s.deleted_at IS NULL`

	var detail SessionDetail
	err := r.pool.QueryRow(ctx, sessQ, id).Scan(
		&detail.ID, &detail.Name, &detail.PIN, &detail.Status,
		&detail.QuestionCount, &detail.TimePerQuestionS, &detail.PointsPerAnswer,
		&detail.SpeedBonusEnabled, &detail.StartedAt, &detail.EndedAt,
		&detail.CreatedBy, &detail.CreatedAt, &detail.UpdatedAt, &detail.DeletedAt,
		&detail.PlayerCount,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return SessionDetail{}, ErrSessionNotFound
	}
	if err != nil {
		return SessionDetail{}, fmt.Errorf("session repo: find by id: %w", err)
	}

	const catsQ = `
		SELECT c.id, c.name
		FROM categories c
		JOIN session_categories sc ON sc.category_id = c.id
		WHERE sc.session_id = $1 AND c.deleted_at IS NULL
		ORDER BY c.name`

	rows, err := r.pool.Query(ctx, catsQ, id)
	if err != nil {
		return SessionDetail{}, fmt.Errorf("session repo: find by id categories: %w", err)
	}
	defer rows.Close()

	detail.Categories = make([]CategoryRef, 0)
	for rows.Next() {
		var cat CategoryRef
		if err := rows.Scan(&cat.ID, &cat.Name); err != nil {
			return SessionDetail{}, fmt.Errorf("session repo: find by id categories scan: %w", err)
		}
		detail.Categories = append(detail.Categories, cat)
	}
	if err := rows.Err(); err != nil {
		return SessionDetail{}, fmt.Errorf("session repo: find by id categories: %w", err)
	}

	return detail, nil
}

func (r *SessionRepository) Update(ctx context.Context, id uuid.UUID, u SessionUpdate) (Session, error) {
	if u.CategoryIDs != nil {
		tx, err := r.pool.Begin(ctx)
		if err != nil {
			return Session{}, fmt.Errorf("session repo: update begin tx: %w", err)
		}
		defer func() { _ = tx.Rollback(ctx) }()

		if _, err := tx.Exec(ctx, `DELETE FROM session_categories WHERE session_id = $1`, id); err != nil {
			return Session{}, fmt.Errorf("session repo: update delete categories: %w", err)
		}
		for _, catID := range u.CategoryIDs {
			if _, err := tx.Exec(ctx, `INSERT INTO session_categories (session_id, category_id) VALUES ($1, $2)`, id, catID); err != nil {
				return Session{}, fmt.Errorf("session repo: update insert categories: %w", err)
			}
		}

		sess, err := r.updateSessionFields(ctx, tx, id, u)
		if err != nil {
			return Session{}, err
		}
		if err := tx.Commit(ctx); err != nil {
			return Session{}, fmt.Errorf("session repo: update commit: %w", err)
		}
		return sess, nil
	}

	return r.updateSessionFields(ctx, r.pool, id, u)
}

type queryRunner interface {
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}

func (r *SessionRepository) updateSessionFields(ctx context.Context, qr queryRunner, id uuid.UUID, u SessionUpdate) (Session, error) {
	setClauses := []string{}
	args := []any{}
	n := 1

	if u.Name != nil {
		setClauses = append(setClauses, fmt.Sprintf("name = $%d", n))
		args = append(args, *u.Name)
		n++
	}
	if u.QuestionCount != nil {
		setClauses = append(setClauses, fmt.Sprintf("question_count = $%d", n))
		args = append(args, *u.QuestionCount)
		n++
	}
	if u.TimePerQuestionS != nil {
		setClauses = append(setClauses, fmt.Sprintf("time_per_question_s = $%d", n))
		args = append(args, *u.TimePerQuestionS)
		n++
	}
	if u.PointsPerAnswer != nil {
		setClauses = append(setClauses, fmt.Sprintf("points_per_answer = $%d", n))
		args = append(args, *u.PointsPerAnswer)
		n++
	}
	if u.SpeedBonusEnabled != nil {
		setClauses = append(setClauses, fmt.Sprintf("speed_bonus_enabled = $%d", n))
		args = append(args, *u.SpeedBonusEnabled)
		n++
	}
	setClauses = append(setClauses, "updated_at = now()")

	args = append(args, id)
	q := fmt.Sprintf(`
		UPDATE sessions
		SET %s
		WHERE id = $%d AND deleted_at IS NULL
		RETURNING id, name, pin, status::text,
		          question_count, time_per_question_s, points_per_answer,
		          speed_bonus_enabled, started_at, ended_at,
		          created_by, created_at, updated_at, deleted_at`,
		strings.Join(setClauses, ", "), n)

	var sess Session
	err := qr.QueryRow(ctx, q, args...).Scan(
		&sess.ID, &sess.Name, &sess.PIN, &sess.Status,
		&sess.QuestionCount, &sess.TimePerQuestionS, &sess.PointsPerAnswer,
		&sess.SpeedBonusEnabled, &sess.StartedAt, &sess.EndedAt,
		&sess.CreatedBy, &sess.CreatedAt, &sess.UpdatedAt, &sess.DeletedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return Session{}, ErrSessionNotFound
	}
	if err != nil {
		return Session{}, fmt.Errorf("session repo: update fields: %w", err)
	}
	return sess, nil
}

func (r *SessionRepository) OpenLobby(ctx context.Context, id uuid.UUID) (Session, error) {
	const q = `
		UPDATE sessions
		SET status = 'lobby', updated_at = now()
		WHERE id = $1
		  AND status = 'draft'
		  AND deleted_at IS NULL
		RETURNING id, name, pin, status::text, question_count, time_per_question_s,
		          points_per_answer, speed_bonus_enabled, started_at, ended_at,
		          created_by, created_at, updated_at, deleted_at`

	var sess Session
	err := r.pool.QueryRow(ctx, q, id).Scan(
		&sess.ID, &sess.Name, &sess.PIN, &sess.Status,
		&sess.QuestionCount, &sess.TimePerQuestionS, &sess.PointsPerAnswer,
		&sess.SpeedBonusEnabled, &sess.StartedAt, &sess.EndedAt,
		&sess.CreatedBy, &sess.CreatedAt, &sess.UpdatedAt, &sess.DeletedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		var exists bool
		_ = r.pool.QueryRow(ctx,
			`SELECT EXISTS(SELECT 1 FROM sessions WHERE id = $1 AND deleted_at IS NULL)`, id,
		).Scan(&exists)
		if !exists {
			return Session{}, ErrSessionNotFound
		}
		return Session{}, ErrSessionNotDraft
	}
	if err != nil {
		return Session{}, fmt.Errorf("session repo: open lobby: %w", err)
	}
	return sess, nil
}

func (r *SessionRepository) Launch(ctx context.Context, id uuid.UUID) (Session, int, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return Session{}, 0, fmt.Errorf("session repo: launch begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var status string
	var questionCount int16
	err = tx.QueryRow(ctx,
		`SELECT status::text, question_count FROM sessions WHERE id = $1 AND deleted_at IS NULL FOR UPDATE`,
		id,
	).Scan(&status, &questionCount)
	if errors.Is(err, pgx.ErrNoRows) {
		return Session{}, 0, ErrSessionNotFound
	}
	if err != nil {
		return Session{}, 0, fmt.Errorf("session repo: launch lock: %w", err)
	}
	if status != "lobby" {
		return Session{}, 0, ErrSessionNotInLobby
	}

	const drawQ = `
		SELECT q.id
		FROM questions q
		JOIN session_categories sc ON sc.category_id = q.category_id
		WHERE sc.session_id = $1
		  AND q.deleted_at IS NULL
		ORDER BY RANDOM()
		LIMIT $2`

	rows, err := tx.Query(ctx, drawQ, id, questionCount)
	if err != nil {
		return Session{}, 0, fmt.Errorf("session repo: launch draw: %w", err)
	}
	var questionIDs []uuid.UUID
	for rows.Next() {
		var qID uuid.UUID
		if err := rows.Scan(&qID); err != nil {
			rows.Close()
			return Session{}, 0, fmt.Errorf("session repo: launch draw scan: %w", err)
		}
		questionIDs = append(questionIDs, qID)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return Session{}, 0, fmt.Errorf("session repo: launch draw: %w", err)
	}
	if len(questionIDs) == 0 {
		return Session{}, 0, ErrInsufficientQuestions
	}

	const insertSQ = `INSERT INTO session_questions (session_id, question_id, position) VALUES ($1, $2, $3)`
	for i, qID := range questionIDs {
		if _, err := tx.Exec(ctx, insertSQ, id, qID, i+1); err != nil {
			return Session{}, 0, fmt.Errorf("session repo: launch insert question %d: %w", i+1, err)
		}
	}

	const activateQ = `
		UPDATE sessions
		SET status = 'active', started_at = now(), updated_at = now()
		WHERE id = $1
		RETURNING id, name, pin, status::text, question_count, time_per_question_s,
		          points_per_answer, speed_bonus_enabled, started_at, ended_at,
		          created_by, created_at, updated_at, deleted_at`

	var sess Session
	err = tx.QueryRow(ctx, activateQ, id).Scan(
		&sess.ID, &sess.Name, &sess.PIN, &sess.Status,
		&sess.QuestionCount, &sess.TimePerQuestionS, &sess.PointsPerAnswer,
		&sess.SpeedBonusEnabled, &sess.StartedAt, &sess.EndedAt,
		&sess.CreatedBy, &sess.CreatedAt, &sess.UpdatedAt, &sess.DeletedAt,
	)
	if err != nil {
		return Session{}, 0, fmt.Errorf("session repo: launch activate: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return Session{}, 0, fmt.Errorf("session repo: launch commit: %w", err)
	}
	return sess, len(questionIDs), nil
}

func (r *SessionRepository) Delete(ctx context.Context, id uuid.UUID) error {
	var status string
	err := r.pool.QueryRow(ctx,
		`SELECT status::text FROM sessions WHERE id = $1 AND deleted_at IS NULL`, id,
	).Scan(&status)
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrSessionNotFound
	}
	if err != nil {
		return fmt.Errorf("session repo: delete status check: %w", err)
	}
	if status != "draft" {
		return ErrSessionNotDraft
	}

	_, err = r.pool.Exec(ctx,
		`UPDATE sessions SET deleted_at = now(), updated_at = now() WHERE id = $1`, id,
	)
	if err != nil {
		return fmt.Errorf("session repo: delete: %w", err)
	}
	return nil
}

// NextQuestion advances the active session to the next unasked question.
func (r *SessionRepository) NextQuestion(ctx context.Context, id uuid.UUID) (NextQuestionData, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return NextQuestionData{}, fmt.Errorf("session repo: next-question begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var status string
	var timeLimitS int16
	err = tx.QueryRow(ctx,
		`SELECT status::text, time_per_question_s FROM sessions WHERE id = $1 AND deleted_at IS NULL FOR UPDATE`,
		id,
	).Scan(&status, &timeLimitS)
	if errors.Is(err, pgx.ErrNoRows) {
		return NextQuestionData{}, ErrSessionNotFound
	}
	if err != nil {
		return NextQuestionData{}, fmt.Errorf("session repo: next-question lock session: %w", err)
	}
	if status != "active" {
		return NextQuestionData{}, ErrSessionNotActive
	}

	// Guard: Ensure no question is currently active and unrevealed.
	var hasUnrevealed bool
	err = tx.QueryRow(ctx,
		`SELECT EXISTS(
			SELECT 1 FROM session_questions
			WHERE session_id = $1 AND asked_at IS NOT NULL AND revealed_at IS NULL
		)`, id,
	).Scan(&hasUnrevealed)
	if err != nil {
		return NextQuestionData{}, fmt.Errorf("session repo: next-question check unrevealed: %w", err)
	}
	if hasUnrevealed {
		return NextQuestionData{}, ErrQuestionNotRevealed
	}

	// Fetch next unasked question (lowest position where asked_at IS NULL).
	const selectNext = `
		SELECT sq.id, sq.position, q.text, q.option_a, q.option_b, q.option_c, q.option_d
		FROM session_questions sq
		JOIN questions q ON q.id = sq.question_id
		WHERE sq.session_id = $1 AND sq.asked_at IS NULL
		ORDER BY sq.position ASC
		LIMIT 1
		FOR UPDATE OF sq`

	var sqID uuid.UUID
	var position int16
	var qText, optA, optB, optC, optD string
	err = tx.QueryRow(ctx, selectNext, id).Scan(
		&sqID, &position, &qText, &optA, &optB, &optC, &optD,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return NextQuestionData{}, ErrNoMoreQuestions
	}
	if err != nil {
		return NextQuestionData{}, fmt.Errorf("session repo: next-question select: %w", err)
	}

	// Mark question as asked.
	var askedAt time.Time
	err = tx.QueryRow(ctx,
		`UPDATE session_questions SET asked_at = now() WHERE id = $1 RETURNING asked_at`,
		sqID,
	).Scan(&askedAt)
	if err != nil {
		return NextQuestionData{}, fmt.Errorf("session repo: next-question update asked_at: %w", err)
	}

	// Get total questions in session.
	var total int
	err = tx.QueryRow(ctx,
		`SELECT COUNT(*) FROM session_questions WHERE session_id = $1`, id,
	).Scan(&total)
	if err != nil {
		return NextQuestionData{}, fmt.Errorf("session repo: next-question count total: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return NextQuestionData{}, fmt.Errorf("session repo: next-question commit: %w", err)
	}

	return NextQuestionData{
		SessionQuestionID: sqID,
		Position:          int(position),
		Total:             total,
		Question: QuestionSummary{
			Text:    qText,
			OptionA: optA,
			OptionB: optB,
			OptionC: optC,
			OptionD: optD,
		},
		TimeLimitS: int(timeLimitS),
		AskedAt:    askedAt,
	}, nil
}

// GetActiveQuestion retrieves metadata for the question currently in progress.
func (r *SessionRepository) GetActiveQuestion(ctx context.Context, id uuid.UUID) (ActiveQuestionData, error) {
	const q = `
		SELECT s.status::text, s.time_per_question_s, sq.id, sq.asked_at
		FROM sessions s
		JOIN session_questions sq ON sq.session_id = s.id
		WHERE s.id = $1 AND s.deleted_at IS NULL AND sq.asked_at IS NOT NULL AND sq.revealed_at IS NULL`

	var status string
	var timeLimitS int16
	var sqID uuid.UUID
	var askedAt time.Time

	err := r.pool.QueryRow(ctx, q, id).Scan(&status, &timeLimitS, &sqID, &askedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		// Distinguish 404 vs 409 vs no active question.
		var sStatus string
		checkErr := r.pool.QueryRow(ctx,
			`SELECT status::text FROM sessions WHERE id = $1 AND deleted_at IS NULL`, id,
		).Scan(&sStatus)
		if errors.Is(checkErr, pgx.ErrNoRows) {
			return ActiveQuestionData{}, ErrSessionNotFound
		}
		if checkErr == nil && sStatus != "active" {
			return ActiveQuestionData{}, ErrSessionNotActive
		}
		return ActiveQuestionData{}, ErrNoActiveQuestion
	}
	if err != nil {
		return ActiveQuestionData{}, fmt.Errorf("session repo: get active question: %w", err)
	}
	if status != "active" {
		return ActiveQuestionData{}, ErrSessionNotActive
	}

	return ActiveQuestionData{
		SessionQuestionID: sqID,
		SessionID:         id,
		TimeLimitS:        int(timeLimitS),
		AskedAt:           askedAt,
	}, nil
}

// ShiftQuestionAskedAt adjusts asked_at by adding the pause duration delta.
func (r *SessionRepository) ShiftQuestionAskedAt(ctx context.Context, sessionQuestionID uuid.UUID, delta time.Duration) error {
	micros := delta.Microseconds()
	const q = `
		UPDATE session_questions
		SET asked_at = asked_at + $1 * interval '1 microsecond'
		WHERE id = $2`

	res, err := r.pool.Exec(ctx, q, micros, sessionQuestionID)
	if err != nil {
		return fmt.Errorf("session repo: shift asked_at: %w", err)
	}
	if res.RowsAffected() == 0 {
		return ErrNoActiveQuestion
	}
	return nil
}

type answerRecord struct {
	id          uuid.UUID
	playerID    uuid.UUID
	chosenIndex *int16
	answeredAt  time.Time
}

// RevealQuestion scores all answers for the current active question and computes round stats.
func (r *SessionRepository) RevealQuestion(ctx context.Context, id uuid.UUID) (RevealData, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return RevealData{}, fmt.Errorf("session repo: reveal begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	// Lock session and active question.
	const selectActive = `
		SELECT s.status::text, s.points_per_answer, s.time_per_question_s, s.speed_bonus_enabled,
		       sq.id, sq.asked_at, q.correct_index
		FROM sessions s
		JOIN session_questions sq ON sq.session_id = s.id
		JOIN questions q ON q.id = sq.question_id
		WHERE s.id = $1 AND s.deleted_at IS NULL AND sq.asked_at IS NOT NULL AND sq.revealed_at IS NULL
		FOR UPDATE OF s, sq`

	var status string
	var pointsPerAnswer int
	var timeLimitS int16
	var speedBonusEnabled bool
	var sqID uuid.UUID
	var askedAt time.Time
	var correctIndex int16

	err = tx.QueryRow(ctx, selectActive, id).Scan(
		&status, &pointsPerAnswer, &timeLimitS, &speedBonusEnabled,
		&sqID, &askedAt, &correctIndex,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		var sStatus string
		checkErr := tx.QueryRow(ctx,
			`SELECT status::text FROM sessions WHERE id = $1 AND deleted_at IS NULL`, id,
		).Scan(&sStatus)
		if errors.Is(checkErr, pgx.ErrNoRows) {
			return RevealData{}, ErrSessionNotFound
		}
		if checkErr == nil && sStatus != "active" {
			return RevealData{}, ErrSessionNotActive
		}

		// Check if the most recent question was already revealed
		var hasAnyAsked bool
		_ = tx.QueryRow(ctx,
			`SELECT EXISTS(SELECT 1 FROM session_questions WHERE session_id = $1 AND asked_at IS NOT NULL)`, id,
		).Scan(&hasAnyAsked)
		if hasAnyAsked {
			return RevealData{}, ErrQuestionAlreadyRevealed
		}
		return RevealData{}, ErrNoActiveQuestion
	}
	if err != nil {
		return RevealData{}, fmt.Errorf("session repo: reveal lock active: %w", err)
	}
	if status != "active" {
		return RevealData{}, ErrSessionNotActive
	}

	// Mark question revealed_at = now()
	var revealedAt time.Time
	err = tx.QueryRow(ctx,
		`UPDATE session_questions SET revealed_at = now() WHERE id = $1 RETURNING revealed_at`,
		sqID,
	).Scan(&revealedAt)
	if err != nil {
		return RevealData{}, fmt.Errorf("session repo: reveal update revealed_at: %w", err)
	}

	// Fetch all answers for this question FOR UPDATE
	const selectAnswers = `
		SELECT id, player_id, chosen_index, answered_at
		FROM answers
		WHERE session_question_id = $1
		FOR UPDATE`

	rows, err := tx.Query(ctx, selectAnswers, sqID)
	if err != nil {
		return RevealData{}, fmt.Errorf("session repo: reveal select answers: %w", err)
	}

	var answers []answerRecord
	counts := [4]int{0, 0, 0, 0}

	for rows.Next() {
		var ans answerRecord
		if err := rows.Scan(&ans.id, &ans.playerID, &ans.chosenIndex, &ans.answeredAt); err != nil {
			rows.Close()
			return RevealData{}, fmt.Errorf("session repo: reveal scan answer: %w", err)
		}
		answers = append(answers, ans)
		if ans.chosenIndex != nil && *ans.chosenIndex >= 0 && *ans.chosenIndex <= 3 {
			counts[*ans.chosenIndex]++
		}
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return RevealData{}, fmt.Errorf("session repo: reveal answers error: %w", err)
	}

	// Score answers and update player scores
	totalTimeMs := int64(timeLimitS) * 1000
	for _, ans := range answers {
		isCorrect := (ans.chosenIndex != nil && *ans.chosenIndex == correctIndex)
		elapsedMs := ans.answeredAt.Sub(askedAt).Milliseconds()
		timeRemainingMs := totalTimeMs - elapsedMs
		if timeRemainingMs < 0 {
			timeRemainingMs = 0
		}
		if timeRemainingMs > totalTimeMs {
			timeRemainingMs = totalTimeMs
		}
		answerTimeMs := int(elapsedMs)
		if answerTimeMs < 0 {
			answerTimeMs = 0
		}

		pointsAwarded := ScoreAnswer(pointsPerAnswer, isCorrect, speedBonusEnabled, timeRemainingMs, totalTimeMs)

		_, err := tx.Exec(ctx,
			`UPDATE answers SET is_correct = $1, points_awarded = $2, answer_time_ms = $3 WHERE id = $4`,
			isCorrect, pointsAwarded, answerTimeMs, ans.id,
		)
		if err != nil {
			return RevealData{}, fmt.Errorf("session repo: reveal update answer %s: %w", ans.id, err)
		}

		if pointsAwarded > 0 {
			_, err := tx.Exec(ctx,
				`UPDATE players SET total_score = total_score + $1 WHERE id = $2`,
				pointsAwarded, ans.playerID,
			)
			if err != nil {
				return RevealData{}, fmt.Errorf("session repo: reveal update player score %s: %w", ans.playerID, err)
			}
		}
	}

	// Compute distribution
	totalAnswers := len(answers)
	distribution := make([]AnswerDistributionItem, 4)
	for i := 0; i < 4; i++ {
		pct := 0
		if totalAnswers > 0 {
			pct = int(math.Round(float64(counts[i]) / float64(totalAnswers) * 100))
		}
		distribution[i] = AnswerDistributionItem{
			Index:   i,
			Count:   counts[i],
			Percent: pct,
		}
	}

	// Fetch Top 5 leaderboard
	const selectTop5 = `
		SELECT nickname, total_score, avatar_color
		FROM players
		WHERE session_id = $1
		ORDER BY total_score DESC, joined_at ASC, id ASC
		LIMIT 5`

	topRows, err := tx.Query(ctx, selectTop5, id)
	if err != nil {
		return RevealData{}, fmt.Errorf("session repo: reveal query top5: %w", err)
	}
	defer topRows.Close()

	var top5 []LeaderboardEntry
	rank := 1
	for topRows.Next() {
		var entry LeaderboardEntry
		if err := topRows.Scan(&entry.Nickname, &entry.TotalScore, &entry.AvatarColor); err != nil {
			return RevealData{}, fmt.Errorf("session repo: reveal scan top5: %w", err)
		}
		entry.Rank = rank
		top5 = append(top5, entry)
		rank++
	}
	if err := topRows.Err(); err != nil {
		return RevealData{}, fmt.Errorf("session repo: reveal top5 error: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return RevealData{}, fmt.Errorf("session repo: reveal commit: %w", err)
	}

	return RevealData{
		SessionQuestionID:  sqID,
		CorrectIndex:       int(correctIndex),
		AnswerDistribution: distribution,
		Top5:               top5,
		RevealedAt:         revealedAt,
	}, nil
}

// EndSession marks an active, lobby, or draft session as completed.
func (r *SessionRepository) EndSession(ctx context.Context, id uuid.UUID, reason string) (Session, error) {
	var status string
	err := r.pool.QueryRow(ctx,
		`SELECT status::text FROM sessions WHERE id = $1 AND deleted_at IS NULL`, id,
	).Scan(&status)
	if errors.Is(err, pgx.ErrNoRows) {
		return Session{}, ErrSessionNotFound
	}
	if err != nil {
		return Session{}, fmt.Errorf("session repo: end status check: %w", err)
	}
	if status == "completed" || status == "cancelled" {
		return Session{}, ErrSessionAlreadyEnded
	}

	const q = `
		UPDATE sessions
		SET status = 'completed', ended_at = now(), updated_at = now()
		WHERE id = $1
		RETURNING id, name, pin, status::text, question_count, time_per_question_s,
		          points_per_answer, speed_bonus_enabled, started_at, ended_at,
		          created_by, created_at, updated_at, deleted_at`

	var sess Session
	err = r.pool.QueryRow(ctx, q, id).Scan(
		&sess.ID, &sess.Name, &sess.PIN, &sess.Status,
		&sess.QuestionCount, &sess.TimePerQuestionS, &sess.PointsPerAnswer,
		&sess.SpeedBonusEnabled, &sess.StartedAt, &sess.EndedAt,
		&sess.CreatedBy, &sess.CreatedAt, &sess.UpdatedAt, &sess.DeletedAt,
	)
	if err != nil {
		return Session{}, fmt.Errorf("session repo: end session: %w", err)
	}
	return sess, nil
}

// JoinPlayer verifies session status and PIN, persists the player record, and returns the player with total count.
func (r *SessionRepository) JoinPlayer(ctx context.Context, sessionID uuid.UUID, pin string, nickname string, avatarColor string) (Player, Session, int, error) {
	const selectSess = `
		SELECT id, name, pin, status::text, question_count, time_per_question_s,
		       points_per_answer, speed_bonus_enabled, started_at, ended_at,
		       created_by, created_at, updated_at, deleted_at
		FROM sessions
		WHERE id = $1 AND deleted_at IS NULL`

	var sess Session
	err := r.pool.QueryRow(ctx, selectSess, sessionID).Scan(
		&sess.ID, &sess.Name, &sess.PIN, &sess.Status,
		&sess.QuestionCount, &sess.TimePerQuestionS, &sess.PointsPerAnswer,
		&sess.SpeedBonusEnabled, &sess.StartedAt, &sess.EndedAt,
		&sess.CreatedBy, &sess.CreatedAt, &sess.UpdatedAt, &sess.DeletedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return Player{}, Session{}, 0, ErrSessionNotFound
	}
	if err != nil {
		return Player{}, Session{}, 0, fmt.Errorf("session repo: join player find session: %w", err)
	}

	if sess.PIN != pin {
		return Player{}, Session{}, 0, ErrInvalidPIN
	}
	if sess.Status != "lobby" {
		return Player{}, Session{}, 0, ErrSessionNotInLobby
	}

	const insertPlayer = `
		INSERT INTO players (session_id, nickname, avatar_color, total_score, joined_at)
		VALUES ($1, $2, $3, 0, now())
		RETURNING id, session_id, nickname, avatar_color, total_score, joined_at, disconnected_at`

	var player Player
	err = r.pool.QueryRow(ctx, insertPlayer, sessionID, nickname, avatarColor).Scan(
		&player.ID, &player.SessionID, &player.Nickname, &player.AvatarColor,
		&player.TotalScore, &player.JoinedAt, &player.DisconnectedAt,
	)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return Player{}, Session{}, 0, ErrNicknameTaken
		}
		return Player{}, Session{}, 0, fmt.Errorf("session repo: join player insert: %w", err)
	}

	var totalPlayers int
	err = r.pool.QueryRow(ctx, `SELECT COUNT(*) FROM players WHERE session_id = $1`, sessionID).Scan(&totalPlayers)
	if err != nil {
		return Player{}, Session{}, 0, fmt.Errorf("session repo: join player count total: %w", err)
	}

	return player, sess, totalPlayers, nil
}

// SubmitAnswer persists an answer, enforcing timer deadlines and returning existing answers idempotently.
func (r *SessionRepository) SubmitAnswer(ctx context.Context, sessionID uuid.UUID, playerID uuid.UUID, sessionQuestionID uuid.UUID, chosenIndex int16, now time.Time) (Answer, bool, int, int, error) {
	// 1. Check if the player already submitted an answer for this question (Idempotency)
	const selectExisting = `
		SELECT id, player_id, session_question_id, chosen_index, is_correct, points_awarded, answer_time_ms, answered_at
		FROM answers
		WHERE player_id = $1 AND session_question_id = $2`

	var existing Answer
	err := r.pool.QueryRow(ctx, selectExisting, playerID, sessionQuestionID).Scan(
		&existing.ID, &existing.PlayerID, &existing.SessionQuestionID,
		&existing.ChosenIndex, &existing.IsCorrect, &existing.PointsAwarded,
		&existing.AnswerTimeMs, &existing.AnsweredAt,
	)
	if err == nil {
		// Existing answer found -> return idempotent success
		var answeredCount, totalPlayers int
		_ = r.pool.QueryRow(ctx, `SELECT COUNT(*) FROM answers WHERE session_question_id = $1`, sessionQuestionID).Scan(&answeredCount)
		_ = r.pool.QueryRow(ctx, `SELECT COUNT(*) FROM players WHERE session_id = $1`, sessionID).Scan(&totalPlayers)
		return existing, true, answeredCount, totalPlayers, nil
	} else if !errors.Is(err, pgx.ErrNoRows) {
		return Answer{}, false, 0, 0, fmt.Errorf("session repo: submit answer check existing: %w", err)
	}

	// 2. Validate question belongs to active session and timer has not expired
	const selectQuestion = `
		SELECT s.status::text, s.time_per_question_s, sq.asked_at, sq.revealed_at
		FROM session_questions sq
		JOIN sessions s ON s.id = sq.session_id
		WHERE sq.id = $1 AND sq.session_id = $2 AND s.deleted_at IS NULL`

	var status string
	var timeLimitS int16
	var askedAt *time.Time
	var revealedAt *time.Time

	err = r.pool.QueryRow(ctx, selectQuestion, sessionQuestionID, sessionID).Scan(
		&status, &timeLimitS, &askedAt, &revealedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return Answer{}, false, 0, 0, ErrQuestionNotFound
	}
	if err != nil {
		return Answer{}, false, 0, 0, fmt.Errorf("session repo: submit answer find question: %w", err)
	}

	if status != "active" {
		return Answer{}, false, 0, 0, ErrSessionNotActive
	}
	if askedAt == nil || revealedAt != nil {
		return Answer{}, false, 0, 0, ErrQuestionClosed
	}

	totalTimeMs := int64(timeLimitS) * 1000
	elapsedMs := now.Sub(*askedAt).Milliseconds()
	if elapsedMs > totalTimeMs {
		return Answer{}, false, 0, 0, ErrQuestionClosed
	}

	answerTimeMs := int(elapsedMs)
	if answerTimeMs < 0 {
		answerTimeMs = 0
	}

	// 3. Insert answer record
	const insertAnswer = `
		INSERT INTO answers (player_id, session_question_id, chosen_index, is_correct, points_awarded, answer_time_ms, answered_at)
		VALUES ($1, $2, $3, false, 0, $4, $5)
		ON CONFLICT (player_id, session_question_id) DO NOTHING
		RETURNING id, player_id, session_question_id, chosen_index, is_correct, points_awarded, answer_time_ms, answered_at`

	var newAns Answer
	err = r.pool.QueryRow(ctx, insertAnswer, playerID, sessionQuestionID, chosenIndex, answerTimeMs, now).Scan(
		&newAns.ID, &newAns.PlayerID, &newAns.SessionQuestionID,
		&newAns.ChosenIndex, &newAns.IsCorrect, &newAns.PointsAwarded,
		&newAns.AnswerTimeMs, &newAns.AnsweredAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		// Conflict on concurrent insert -> re-query existing
		err = r.pool.QueryRow(ctx, selectExisting, playerID, sessionQuestionID).Scan(
			&existing.ID, &existing.PlayerID, &existing.SessionQuestionID,
			&existing.ChosenIndex, &existing.IsCorrect, &existing.PointsAwarded,
			&existing.AnswerTimeMs, &existing.AnsweredAt,
		)
		if err != nil {
			return Answer{}, false, 0, 0, fmt.Errorf("session repo: submit answer fetch after conflict: %w", err)
		}
		var answeredCount, totalPlayers int
		_ = r.pool.QueryRow(ctx, `SELECT COUNT(*) FROM answers WHERE session_question_id = $1`, sessionQuestionID).Scan(&answeredCount)
		_ = r.pool.QueryRow(ctx, `SELECT COUNT(*) FROM players WHERE session_id = $1`, sessionID).Scan(&totalPlayers)
		return existing, true, answeredCount, totalPlayers, nil
	}
	if err != nil {
		return Answer{}, false, 0, 0, fmt.Errorf("session repo: submit answer insert: %w", err)
	}

	var answeredCount, totalPlayers int
	err = r.pool.QueryRow(ctx, `SELECT COUNT(*) FROM answers WHERE session_question_id = $1`, sessionQuestionID).Scan(&answeredCount)
	if err != nil {
		return Answer{}, false, 0, 0, fmt.Errorf("session repo: submit answer count answered: %w", err)
	}
	err = r.pool.QueryRow(ctx, `SELECT COUNT(*) FROM players WHERE session_id = $1`, sessionID).Scan(&totalPlayers)
	if err != nil {
		return Answer{}, false, 0, 0, fmt.Errorf("session repo: submit answer count total players: %w", err)
	}

	return newAns, false, answeredCount, totalPlayers, nil
}

