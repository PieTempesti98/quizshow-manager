package question

import (
	"context"
	"encoding/csv"
	"fmt"
	"io"
	"strconv"
	"strings"

	"github.com/google/uuid"
)

// Service defines the question business operations.
type Service interface {
	List(ctx context.Context, filter QuestionFilter) (QuestionListResult, error)
	Create(ctx context.Context, q Question) (*Question, error)
	Update(ctx context.Context, id uuid.UUID, u QuestionUpdate) (*Question, error)
	Delete(ctx context.Context, id uuid.UUID) error
	Import(ctx context.Context, r io.Reader, onError string) (ImportResult, error)
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

var csvExpectedHeaders = []string{
	"text", "option_a", "option_b", "option_c", "option_d",
	"correct_index", "category_name", "difficulty",
}

func (s *service) Import(ctx context.Context, r io.Reader, onError string) (ImportResult, error) {
	if onError != "abort" && onError != "skip" {
		return ImportResult{}, ErrInvalidOnError
	}

	reader := csv.NewReader(r)
	headers, err := reader.Read()
	if err != nil {
		return ImportResult{}, &ErrImportFileInvalid{Msg: "cannot read CSV header row"}
	}
	if len(headers) != len(csvExpectedHeaders) {
		return ImportResult{}, &ErrImportFileInvalid{
			Msg: fmt.Sprintf("CSV must have exactly %d columns", len(csvExpectedHeaders)),
		}
	}
	for i, h := range headers {
		if strings.TrimSpace(strings.ToLower(h)) != csvExpectedHeaders[i] {
			return ImportResult{}, &ErrImportFileInvalid{
				Msg: fmt.Sprintf("unexpected column %q at position %d, expected %q", h, i+1, csvExpectedHeaders[i]),
			}
		}
	}

	allRecords, err := reader.ReadAll()
	if err != nil {
		return ImportResult{}, &ErrImportFileInvalid{Msg: "cannot parse CSV rows"}
	}
	if len(allRecords) > 500 {
		return ImportResult{}, &ErrImportFileInvalid{
			Msg: fmt.Sprintf("file contains %d rows; maximum is 500", len(allRecords)),
		}
	}

	catMap, err := s.repo.FindCategoryMap(ctx)
	if err != nil {
		return ImportResult{}, fmt.Errorf("import: load categories: %w", err)
	}

	type validatedRow struct {
		question Question
		rowNum   int
	}

	var (
		valid      []validatedRow
		hardErrors []RowError
		warnings   []RowError
	)

	for i, rec := range allRecords {
		rowNum := i + 1
		text := strings.TrimSpace(rec[0])
		optA := strings.TrimSpace(rec[1])
		optB := strings.TrimSpace(rec[2])
		optC := strings.TrimSpace(rec[3])
		optD := strings.TrimSpace(rec[4])
		ciRaw := strings.TrimSpace(rec[5])
		catName := strings.TrimSpace(rec[6])
		diff := strings.TrimSpace(rec[7])

		var rowErrs []string

		if text == "" {
			rowErrs = append(rowErrs, "text: required")
		}
		if optA == "" {
			rowErrs = append(rowErrs, "option_a: required")
		}
		if optB == "" {
			rowErrs = append(rowErrs, "option_b: required")
		}
		if optC == "" {
			rowErrs = append(rowErrs, "option_c: required")
		}
		if optD == "" {
			rowErrs = append(rowErrs, "option_d: required")
		}
		if catName == "" {
			rowErrs = append(rowErrs, "category_name: required")
		}

		ci, ciErr := strconv.Atoi(ciRaw)
		if ciErr != nil || ci < 0 || ci > 3 {
			rowErrs = append(rowErrs, fmt.Sprintf("correct_index must be between 0 and 3, got: %q", ciRaw))
		}

		if diff == "" {
			diff = "medium"
		} else if diff != "easy" && diff != "medium" && diff != "hard" {
			rowErrs = append(rowErrs, fmt.Sprintf("difficulty: invalid value %q, must be easy, medium, or hard", diff))
		}

		catID, catFound := catMap[strings.ToLower(catName)]
		if catName != "" && !catFound {
			rowErrs = append(rowErrs, fmt.Sprintf("category '%s' not found", catName))
		}

		if len(rowErrs) > 0 {
			hardErrors = append(hardErrors, RowError{Row: rowNum, Message: strings.Join(rowErrs, "; ")})
			continue
		}

		isDup, err := s.repo.CheckDuplicate(ctx, text, catID)
		if err != nil {
			return ImportResult{}, fmt.Errorf("import: duplicate check row %d: %w", rowNum, err)
		}
		if isDup {
			warnings = append(warnings, RowError{Row: rowNum, Message: fmt.Sprintf("duplicate question in category '%s'", catName)})
		}

		valid = append(valid, validatedRow{
			rowNum: rowNum,
			question: Question{
				CategoryID:   catID,
				Text:         text,
				OptionA:      optA,
				OptionB:      optB,
				OptionC:      optC,
				OptionD:      optD,
				CorrectIndex: ci,
				Difficulty:   diff,
			},
		})
	}

	allErrors := append(hardErrors, warnings...)

	if onError == "abort" {
		if len(hardErrors) > 0 {
			return ImportResult{Imported: 0, Skipped: 0, Errors: allErrors}, nil
		}

		questions := make([]Question, len(valid))
		for i, v := range valid {
			questions[i] = v.question
		}

		tx, err := s.repo.BeginTx(ctx)
		if err != nil {
			return ImportResult{}, fmt.Errorf("import: begin tx: %w", err)
		}
		if err := s.repo.CreateBatch(ctx, tx, questions); err != nil {
			_ = tx.Rollback(ctx)
			return ImportResult{}, fmt.Errorf("import: create batch: %w", err)
		}
		if err := tx.Commit(ctx); err != nil {
			return ImportResult{}, fmt.Errorf("import: commit: %w", err)
		}

		return ImportResult{Imported: len(valid), Skipped: 0, Errors: warnings}, nil
	}

	// skip mode
	var (
		imported int
		skipped  int
		skipErrs []RowError
	)

	validIdx := 0
	for i := range allRecords {
		rowNum := i + 1
		isHardError := false
		for _, he := range hardErrors {
			if he.Row == rowNum {
				skipErrs = append(skipErrs, he)
				skipped++
				isHardError = true
				break
			}
		}
		if isHardError {
			continue
		}

		v := valid[validIdx]
		validIdx++
		if _, err := s.repo.Create(ctx, v.question); err != nil {
			skipErrs = append(skipErrs, RowError{Row: rowNum, Message: fmt.Sprintf("insert failed: %v", err)})
			skipped++
			continue
		}
		imported++

		for _, w := range warnings {
			if w.Row == rowNum {
				skipErrs = append(skipErrs, w)
				break
			}
		}
	}

	return ImportResult{Imported: imported, Skipped: skipped, Errors: skipErrs}, nil
}
