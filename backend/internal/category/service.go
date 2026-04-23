package category

import (
	"context"
	"fmt"

	"github.com/google/uuid"
)

// Service defines the category business operations.
type Service interface {
	List(ctx context.Context) ([]CategoryWithCount, error)
	Create(ctx context.Context, name string) (*CategoryWithCount, error)
	Rename(ctx context.Context, id uuid.UUID, name string) (*CategoryWithCount, error)
	Delete(ctx context.Context, id uuid.UUID) error
}

type service struct {
	repo CategoryRepo
}

// NewService constructs a category Service.
func NewService(repo CategoryRepo) Service {
	return &service{repo: repo}
}

func (s *service) List(ctx context.Context) ([]CategoryWithCount, error) {
	return s.repo.List(ctx)
}

func (s *service) Create(ctx context.Context, name string) (*CategoryWithCount, error) {
	slug := slugify(name)
	cat, err := s.repo.Create(ctx, name, slug)
	if err != nil {
		return nil, err
	}
	return &CategoryWithCount{Category: *cat, QuestionCount: 0}, nil
}

func (s *service) Rename(ctx context.Context, id uuid.UUID, name string) (*CategoryWithCount, error) {
	existing, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("rename: %w", err)
	}
	if existing == nil {
		return nil, ErrCategoryNotFound
	}

	slug := slugify(name)
	cat, err := s.repo.Update(ctx, id, name, slug)
	if err != nil {
		return nil, err
	}

	count, err := s.repo.CountActiveQuestions(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("rename: %w", err)
	}

	return &CategoryWithCount{Category: *cat, QuestionCount: count}, nil
}

func (s *service) Delete(ctx context.Context, id uuid.UUID) error {
	existing, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return fmt.Errorf("delete: %w", err)
	}
	if existing == nil {
		return ErrCategoryNotFound
	}

	count, err := s.repo.CountActiveQuestions(ctx, id)
	if err != nil {
		return fmt.Errorf("delete: %w", err)
	}
	if count > 0 {
		return ErrCategoryHasQuestions{Count: count}
	}

	return s.repo.Delete(ctx, id)
}
