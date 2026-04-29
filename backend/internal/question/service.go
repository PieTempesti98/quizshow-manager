package question

import (
	"context"
	"fmt"

	"github.com/google/uuid"
)

// Service defines the question business operations.
type Service interface {
	List(ctx context.Context, filter QuestionFilter) (QuestionListResult, error)
	Create(ctx context.Context, q Question) (*Question, error)
	Update(ctx context.Context, id uuid.UUID, u QuestionUpdate) (*Question, error)
	Delete(ctx context.Context, id uuid.UUID) error
}

type service struct {
	repo QuestionRepo
}

// NewService constructs a question Service.
func NewService(repo QuestionRepo) Service {
	return &service{repo: repo}
}

func (s *service) List(ctx context.Context, filter QuestionFilter) (QuestionListResult, error) {
	return s.repo.List(ctx, filter)
}

func (s *service) Create(ctx context.Context, q Question) (*Question, error) {
	return s.repo.Create(ctx, q)
}

func (s *service) Update(ctx context.Context, id uuid.UUID, u QuestionUpdate) (*Question, error) {
	inUse, err := s.repo.IsInActiveSession(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("update: %w", err)
	}
	if inUse {
		return nil, ErrQuestionInUse
	}

	existing, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("update: %w", err)
	}
	if existing == nil {
		return nil, ErrQuestionNotFound
	}

	return s.repo.Update(ctx, id, u)
}

func (s *service) Delete(ctx context.Context, id uuid.UUID) error {
	inUse, err := s.repo.IsInActiveSession(ctx, id)
	if err != nil {
		return fmt.Errorf("delete: %w", err)
	}
	if inUse {
		return ErrQuestionInUse
	}
	return s.repo.Delete(ctx, id)
}
