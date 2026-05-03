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
var ErrInvalidOnError = errors.New("on_error must be 'abort' or 'skip'")

// ErrImportFileInvalid is returned by Import when the file itself is malformed
// (wrong headers, too many rows, etc.) before any row is processed.
type ErrImportFileInvalid struct{ Msg string }

func (e *ErrImportFileInvalid) Error() string { return e.Msg }

// ImportRow is one parsed data row from an uploaded CSV file.
type ImportRow struct {
	RowNumber    int
	Text         string
	OptionA      string
	OptionB      string
	OptionC      string
	OptionD      string
	CorrectIndex int
	CategoryName string
	Difficulty   string
}

// RowError is a single row-level problem (hard validation failure or soft warning).
type RowError struct {
	Row     int    `json:"row"`
	Message string `json:"message"`
}

// ImportResult is the aggregate outcome of a CSV import operation.
type ImportResult struct {
	Imported int        `json:"imported"`
	Skipped  int        `json:"skipped"`
	Errors   []RowError `json:"errors"`
}
