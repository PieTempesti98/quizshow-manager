package question

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

// QuestionRepo defines the persistence operations for questions.
type QuestionRepo interface {
	List(ctx context.Context, filter QuestionFilter) (QuestionListResult, error)
	Create(ctx context.Context, q Question) (*Question, error)
	FindByID(ctx context.Context, id uuid.UUID) (*Question, error)
	IsInActiveSession(ctx context.Context, id uuid.UUID) (bool, error)
	Update(ctx context.Context, id uuid.UUID, u QuestionUpdate) (*Question, error)
	Delete(ctx context.Context, id uuid.UUID) error
}

// QuestionRepository implements QuestionRepo using a pgxpool.Pool.
type QuestionRepository struct {
	pool *pgxpool.Pool
}

// NewRepository constructs a QuestionRepository.
func NewRepository(pool *pgxpool.Pool) *QuestionRepository {
	return &QuestionRepository{pool: pool}
}

func (r *QuestionRepository) List(ctx context.Context, filter QuestionFilter) (QuestionListResult, error) {
	if filter.Page < 1 {
		filter.Page = 1
	}
	if filter.PerPage < 1 {
		filter.PerPage = 25
	}
	if filter.PerPage > 100 {
		filter.PerPage = 100
	}

	clauses := []string{"q.deleted_at IS NULL"}
	args := []any{}
	n := 1

	if len(filter.CategoryIDs) > 0 {
		clauses = append(clauses, fmt.Sprintf("q.category_id = ANY($%d)", n))
		args = append(args, filter.CategoryIDs)
		n++
	}
	if len(filter.Difficulties) > 0 {
		clauses = append(clauses, fmt.Sprintf("q.difficulty::text = ANY($%d)", n))
		args = append(args, filter.Difficulties)
		n++
	}
	if filter.Search != "" {
		clauses = append(clauses, fmt.Sprintf("q.text ILIKE $%d", n))
		args = append(args, "%"+filter.Search+"%")
		n++
	}

	where := strings.Join(clauses, " AND ")

	countQ := fmt.Sprintf(`SELECT COUNT(*) FROM questions q WHERE %s`, where)
	var total int
	if err := r.pool.QueryRow(ctx, countQ, args...).Scan(&total); err != nil {
		return QuestionListResult{}, fmt.Errorf("question repo: list count: %w", err)
	}

	totalPages := 0
	if total > 0 {
		totalPages = (total + filter.PerPage - 1) / filter.PerPage
	}
	offset := (filter.Page - 1) * filter.PerPage

	dataQ := fmt.Sprintf(`
		SELECT q.id, q.category_id, COALESCE(c.name, ''), q.text,
		       q.option_a, q.option_b, q.option_c, q.option_d,
		       q.correct_index, q.difficulty::text, q.created_at, q.updated_at
		FROM questions q
		LEFT JOIN categories c ON c.id = q.category_id AND c.deleted_at IS NULL
		WHERE %s
		ORDER BY q.created_at DESC
		LIMIT $%d OFFSET $%d`, where, n, n+1)

	dataArgs := make([]any, len(args)+2)
	copy(dataArgs, args)
	dataArgs[len(args)] = filter.PerPage
	dataArgs[len(args)+1] = offset

	rows, err := r.pool.Query(ctx, dataQ, dataArgs...)
	if err != nil {
		return QuestionListResult{}, fmt.Errorf("question repo: list: %w", err)
	}
	defer rows.Close()

	questions := make([]Question, 0)
	for rows.Next() {
		var q Question
		if err := rows.Scan(
			&q.ID, &q.CategoryID, &q.CategoryName, &q.Text,
			&q.OptionA, &q.OptionB, &q.OptionC, &q.OptionD,
			&q.CorrectIndex, &q.Difficulty, &q.CreatedAt, &q.UpdatedAt,
		); err != nil {
			return QuestionListResult{}, fmt.Errorf("question repo: list scan: %w", err)
		}
		questions = append(questions, q)
	}
	if err := rows.Err(); err != nil {
		return QuestionListResult{}, fmt.Errorf("question repo: list: %w", err)
	}

	return QuestionListResult{
		Questions:  questions,
		Total:      total,
		Page:       filter.Page,
		PerPage:    filter.PerPage,
		TotalPages: totalPages,
	}, nil
}

func (r *QuestionRepository) Create(ctx context.Context, q Question) (*Question, error) {
	const insertQ = `
		INSERT INTO questions (category_id, text, option_a, option_b, option_c, option_d, correct_index, difficulty)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8::difficulty_level)
		RETURNING id, category_id, text, option_a, option_b, option_c, option_d,
		          correct_index, difficulty::text, created_at, updated_at`

	var result Question
	err := r.pool.QueryRow(ctx, insertQ,
		q.CategoryID, q.Text, q.OptionA, q.OptionB, q.OptionC, q.OptionD,
		q.CorrectIndex, q.Difficulty,
	).Scan(
		&result.ID, &result.CategoryID, &result.Text,
		&result.OptionA, &result.OptionB, &result.OptionC, &result.OptionD,
		&result.CorrectIndex, &result.Difficulty, &result.CreatedAt, &result.UpdatedAt,
	)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23503" {
			return nil, ErrQuestionNotFound
		}
		return nil, fmt.Errorf("question repo: create: %w", err)
	}

	const catQ = `SELECT name FROM categories WHERE id = $1 AND deleted_at IS NULL`
	if err := r.pool.QueryRow(ctx, catQ, result.CategoryID).Scan(&result.CategoryName); err != nil {
		if !errors.Is(err, pgx.ErrNoRows) {
			return nil, fmt.Errorf("question repo: create category fetch: %w", err)
		}
	}

	return &result, nil
}

func (r *QuestionRepository) FindByID(ctx context.Context, id uuid.UUID) (*Question, error) {
	const q = `
		SELECT q.id, q.category_id, COALESCE(c.name, ''), q.text,
		       q.option_a, q.option_b, q.option_c, q.option_d,
		       q.correct_index, q.difficulty::text, q.created_at, q.updated_at
		FROM questions q
		LEFT JOIN categories c ON c.id = q.category_id AND c.deleted_at IS NULL
		WHERE q.id = $1 AND q.deleted_at IS NULL`

	var result Question
	err := r.pool.QueryRow(ctx, q, id).Scan(
		&result.ID, &result.CategoryID, &result.CategoryName, &result.Text,
		&result.OptionA, &result.OptionB, &result.OptionC, &result.OptionD,
		&result.CorrectIndex, &result.Difficulty, &result.CreatedAt, &result.UpdatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("question repo: find by id: %w", err)
	}
	return &result, nil
}

func (r *QuestionRepository) IsInActiveSession(ctx context.Context, id uuid.UUID) (bool, error) {
	const q = `
		SELECT EXISTS (
			SELECT 1
			FROM session_questions sq
			JOIN sessions s ON s.id = sq.session_id
			WHERE sq.question_id = $1
			  AND s.status = 'active'
			  AND s.deleted_at IS NULL
		)`

	var exists bool
	if err := r.pool.QueryRow(ctx, q, id).Scan(&exists); err != nil {
		return false, fmt.Errorf("question repo: is in active session: %w", err)
	}
	return exists, nil
}

func (r *QuestionRepository) Update(ctx context.Context, id uuid.UUID, u QuestionUpdate) (*Question, error) {
	setClauses := []string{}
	args := []any{}
	n := 1

	if u.CategoryID != nil {
		setClauses = append(setClauses, fmt.Sprintf("category_id = $%d", n))
		args = append(args, *u.CategoryID)
		n++
	}
	if u.Text != nil {
		setClauses = append(setClauses, fmt.Sprintf("text = $%d", n))
		args = append(args, *u.Text)
		n++
	}
	if u.OptionA != nil {
		setClauses = append(setClauses, fmt.Sprintf("option_a = $%d", n))
		args = append(args, *u.OptionA)
		n++
	}
	if u.OptionB != nil {
		setClauses = append(setClauses, fmt.Sprintf("option_b = $%d", n))
		args = append(args, *u.OptionB)
		n++
	}
	if u.OptionC != nil {
		setClauses = append(setClauses, fmt.Sprintf("option_c = $%d", n))
		args = append(args, *u.OptionC)
		n++
	}
	if u.OptionD != nil {
		setClauses = append(setClauses, fmt.Sprintf("option_d = $%d", n))
		args = append(args, *u.OptionD)
		n++
	}
	if u.CorrectIndex != nil {
		setClauses = append(setClauses, fmt.Sprintf("correct_index = $%d", n))
		args = append(args, *u.CorrectIndex)
		n++
	}
	if u.Difficulty != nil {
		setClauses = append(setClauses, fmt.Sprintf("difficulty = $%d::difficulty_level", n))
		args = append(args, *u.Difficulty)
		n++
	}
	setClauses = append(setClauses, "updated_at = now()")

	args = append(args, id)
	q := fmt.Sprintf(`
		UPDATE questions
		SET %s
		WHERE id = $%d AND deleted_at IS NULL
		RETURNING id, category_id, text, option_a, option_b, option_c, option_d,
		          correct_index, difficulty::text, created_at, updated_at`,
		strings.Join(setClauses, ", "), n)

	var result Question
	err := r.pool.QueryRow(ctx, q, args...).Scan(
		&result.ID, &result.CategoryID, &result.Text,
		&result.OptionA, &result.OptionB, &result.OptionC, &result.OptionD,
		&result.CorrectIndex, &result.Difficulty, &result.CreatedAt, &result.UpdatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrQuestionNotFound
	}
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23503" {
			return nil, ErrQuestionNotFound
		}
		return nil, fmt.Errorf("question repo: update: %w", err)
	}

	const catQ = `SELECT name FROM categories WHERE id = $1 AND deleted_at IS NULL`
	if err := r.pool.QueryRow(ctx, catQ, result.CategoryID).Scan(&result.CategoryName); err != nil {
		if !errors.Is(err, pgx.ErrNoRows) {
			return nil, fmt.Errorf("question repo: update category fetch: %w", err)
		}
	}

	return &result, nil
}

func (r *QuestionRepository) Delete(ctx context.Context, id uuid.UUID) error {
	const q = `
		UPDATE questions
		SET deleted_at = now(), updated_at = now()
		WHERE id = $1 AND deleted_at IS NULL`

	ct, err := r.pool.Exec(ctx, q, id)
	if err != nil {
		return fmt.Errorf("question repo: delete: %w", err)
	}
	if ct.RowsAffected() == 0 {
		return ErrQuestionNotFound
	}
	return nil
}
