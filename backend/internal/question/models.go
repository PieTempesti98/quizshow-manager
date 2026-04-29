package question

import (
	"errors"
	"time"

	"github.com/google/uuid"
)

// Question is the full internal representation of a question row,
// including the joined category name.
type Question struct {
	ID           uuid.UUID
	CategoryID   uuid.UUID
	CategoryName string
	Text         string
	OptionA      string
	OptionB      string
	OptionC      string
	OptionD      string
	CorrectIndex int
	Difficulty   string
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

// QuestionFilter holds the optional query parameters for the list endpoint.
type QuestionFilter struct {
	CategoryIDs  []uuid.UUID
	Difficulties []string
	Search       string
	Page         int
	PerPage      int
}

// QuestionListResult is returned by the list operation.
type QuestionListResult struct {
	Questions  []Question
	Total      int
	Page       int
	PerPage    int
	TotalPages int
}

// QuestionUpdate carries the fields for a partial update (PATCH).
// A nil pointer means the field is absent from the request — do not update.
type QuestionUpdate struct {
	CategoryID   *uuid.UUID
	Text         *string
	OptionA      *string
	OptionB      *string
	OptionC      *string
	OptionD      *string
	CorrectIndex *int
	Difficulty   *string
}

var ErrQuestionNotFound = errors.New("question not found")
var ErrQuestionInUse = errors.New("question is in use by an active session")
