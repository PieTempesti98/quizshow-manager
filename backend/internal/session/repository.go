package session

import (
	"context"
	"errors"
	"fmt"
	"math/rand"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

// SessionRepo defines the persistence operations for sessions.
type SessionRepo interface {
	Create(ctx context.Context, s SessionCreate, adminID uuid.UUID) (Session, int, error)
	List(ctx context.Context, f SessionFilter) (SessionListResult, error)
	FindByID(ctx context.Context, id uuid.UUID) (SessionDetail, error)
	Update(ctx context.Context, id uuid.UUID, u SessionUpdate) (Session, error)
	Delete(ctx context.Context, id uuid.UUID) error
	CountAvailableQuestions(ctx context.Context, sessionID uuid.UUID) (int, error)
	ValidateCategoryIDs(ctx context.Context, ids []uuid.UUID) error
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
	// Handle category replacement in a transaction when CategoryIDs is non-nil.
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
