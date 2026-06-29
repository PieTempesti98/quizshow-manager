package category

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

// CategoryRepo defines the persistence operations for categories.
type CategoryRepo interface {
	List(ctx context.Context) ([]CategoryWithCount, error)
	FindByID(ctx context.Context, id uuid.UUID) (*Category, error)
	Create(ctx context.Context, name, slug string) (*Category, error)
	Update(ctx context.Context, id uuid.UUID, name, slug string) (*Category, error)
	CountActiveQuestions(ctx context.Context, id uuid.UUID) (int, error)
	Delete(ctx context.Context, id uuid.UUID) error
}

// CategoryRepository implements CategoryRepo using a pgxpool.Pool.
type CategoryRepository struct {
	pool *pgxpool.Pool
}

// NewCategoryRepository constructs a CategoryRepository.
func NewCategoryRepository(pool *pgxpool.Pool) *CategoryRepository {
	return &CategoryRepository{pool: pool}
}

func (r *CategoryRepository) List(ctx context.Context) ([]CategoryWithCount, error) {
	const q = `
		SELECT c.id, c.name, c.slug, c.created_at, c.updated_at,
		       COUNT(q.id) AS question_count
		FROM categories c
		LEFT JOIN questions q
		       ON q.category_id = c.id AND q.deleted_at IS NULL
		WHERE c.deleted_at IS NULL
		GROUP BY c.id
		ORDER BY c.name ASC`

	rows, err := r.pool.Query(ctx, q)
	if err != nil {
		return nil, fmt.Errorf("category repo: list: %w", err)
	}
	defer rows.Close()

	results := make([]CategoryWithCount, 0)
	for rows.Next() {
		var c CategoryWithCount
		if err := rows.Scan(
			&c.ID, &c.Name, &c.Slug, &c.CreatedAt, &c.UpdatedAt,
			&c.QuestionCount,
		); err != nil {
			return nil, fmt.Errorf("category repo: list scan: %w", err)
		}
		results = append(results, c)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("category repo: list: %w", err)
	}
	return results, nil
}

func (r *CategoryRepository) FindByID(ctx context.Context, id uuid.UUID) (*Category, error) {
	const q = `
		SELECT id, name, slug, created_at, updated_at, deleted_at
		FROM categories
		WHERE id = $1 AND deleted_at IS NULL`

	var c Category
	err := r.pool.QueryRow(ctx, q, id).Scan(
		&c.ID, &c.Name, &c.Slug, &c.CreatedAt, &c.UpdatedAt, &c.DeletedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("category repo: find by id: %w", err)
	}
	return &c, nil
}

func (r *CategoryRepository) Create(ctx context.Context, name, slug string) (*Category, error) {
	const q = `
		INSERT INTO categories (name, slug)
		VALUES ($1, $2)
		RETURNING id, name, slug, created_at, updated_at`

	var c Category
	err := r.pool.QueryRow(ctx, q, name, slug).Scan(
		&c.ID, &c.Name, &c.Slug, &c.CreatedAt, &c.UpdatedAt,
	)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return nil, ErrCategoryNameConflict
		}
		return nil, fmt.Errorf("category repo: create: %w", err)
	}
	return &c, nil
}

func (r *CategoryRepository) Update(ctx context.Context, id uuid.UUID, name, slug string) (*Category, error) {
	const q = `
		UPDATE categories
		SET name = $1, slug = $2, updated_at = now()
		WHERE id = $3 AND deleted_at IS NULL
		RETURNING id, name, slug, created_at, updated_at`

	var c Category
	err := r.pool.QueryRow(ctx, q, name, slug, id).Scan(
		&c.ID, &c.Name, &c.Slug, &c.CreatedAt, &c.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrCategoryNotFound
		}
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return nil, ErrCategoryNameConflict
		}
		return nil, fmt.Errorf("category repo: update: %w", err)
	}
	return &c, nil
}

func (r *CategoryRepository) CountActiveQuestions(ctx context.Context, id uuid.UUID) (int, error) {
	const q = `
		SELECT COUNT(*) FROM questions
		WHERE category_id = $1 AND deleted_at IS NULL`

	var count int
	if err := r.pool.QueryRow(ctx, q, id).Scan(&count); err != nil {
		return 0, fmt.Errorf("category repo: count active questions: %w", err)
	}
	return count, nil
}

func (r *CategoryRepository) Delete(ctx context.Context, id uuid.UUID) error {
	const q = `
		UPDATE categories
		SET deleted_at = now(), updated_at = now()
		WHERE id = $1 AND deleted_at IS NULL`

	if _, err := r.pool.Exec(ctx, q, id); err != nil {
		return fmt.Errorf("category repo: delete: %w", err)
	}
	return nil
}
