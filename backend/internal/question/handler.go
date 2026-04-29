package question

import (
	"errors"
	"fmt"
	"strings"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"github.com/PieTempesti98/quizshow/internal/api"
)

// Handler holds the HTTP handlers for question endpoints.
type Handler struct {
	svc Service
}

// NewHandler constructs a Handler.
func NewHandler(svc Service) *Handler {
	return &Handler{svc: svc}
}

type categoryResponse struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type questionResponse struct {
	ID           string           `json:"id"`
	Category     categoryResponse `json:"category"`
	Text         string           `json:"text"`
	OptionA      string           `json:"option_a"`
	OptionB      string           `json:"option_b"`
	OptionC      string           `json:"option_c"`
	OptionD      string           `json:"option_d"`
	CorrectIndex int              `json:"correct_index"`
	Difficulty   string           `json:"difficulty"`
	CreatedAt    string           `json:"created_at"`
}

type paginationResponse struct {
	Page       int `json:"page"`
	PerPage    int `json:"per_page"`
	Total      int `json:"total"`
	TotalPages int `json:"total_pages"`
}

func toQuestionResponse(q Question) questionResponse {
	return questionResponse{
		ID: q.ID.String(),
		Category: categoryResponse{
			ID:   q.CategoryID.String(),
			Name: q.CategoryName,
		},
		Text:         q.Text,
		OptionA:      q.OptionA,
		OptionB:      q.OptionB,
		OptionC:      q.OptionC,
		OptionD:      q.OptionD,
		CorrectIndex: q.CorrectIndex,
		Difficulty:   q.Difficulty,
		CreatedAt:    q.CreatedAt.UTC().Format("2006-01-02T15:04:05Z"),
	}
}

var validDifficulties = map[string]bool{
	"easy":   true,
	"medium": true,
	"hard":   true,
}

// List handles GET /api/v1/questions.
func (h *Handler) List(c *fiber.Ctx) error {
	page := c.QueryInt("page", 1)
	if page < 1 {
		page = 1
	}
	perPage := c.QueryInt("per_page", 25)
	if perPage < 1 {
		perPage = 25
	}
	if perPage > 100 {
		perPage = 100
	}

	var categoryIDs []uuid.UUID
	if raw := c.Query("category_id"); raw != "" {
		for _, s := range strings.Split(raw, ",") {
			s = strings.TrimSpace(s)
			if s == "" {
				continue
			}
			id, err := uuid.Parse(s)
			if err != nil {
				return c.Status(fiber.StatusUnprocessableEntity).JSON(api.ErrorResponse{
					Error: api.ErrorDetail{
						Code:    "VALIDATION_ERROR",
						Message: fmt.Sprintf("category_id: invalid UUID %q", s),
					},
				})
			}
			categoryIDs = append(categoryIDs, id)
		}
	}

	var difficulties []string
	if raw := c.Query("difficulty"); raw != "" {
		for _, s := range strings.Split(raw, ",") {
			s = strings.TrimSpace(s)
			if s == "" {
				continue
			}
			if !validDifficulties[s] {
				return c.Status(fiber.StatusUnprocessableEntity).JSON(api.ErrorResponse{
					Error: api.ErrorDetail{
						Code:    "VALIDATION_ERROR",
						Message: fmt.Sprintf("difficulty: invalid value %q, must be easy, medium, or hard", s),
					},
				})
			}
			difficulties = append(difficulties, s)
		}
	}

	filter := QuestionFilter{
		CategoryIDs:  categoryIDs,
		Difficulties: difficulties,
		Search:       c.Query("q"),
		Page:         page,
		PerPage:      perPage,
	}

	result, err := h.svc.List(c.Context(), filter)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(api.ErrorResponse{
			Error: api.ErrorDetail{Code: "INTERNAL_ERROR", Message: "an unexpected error occurred"},
		})
	}

	resp := make([]questionResponse, len(result.Questions))
	for i, q := range result.Questions {
		resp[i] = toQuestionResponse(q)
	}

	return c.Status(fiber.StatusOK).JSON(api.DataResponse{
		Data: map[string]any{
			"questions": resp,
			"pagination": paginationResponse{
				Page:       result.Page,
				PerPage:    result.PerPage,
				Total:      result.Total,
				TotalPages: result.TotalPages,
			},
		},
	})
}

type createRequest struct {
	CategoryID   string  `json:"category_id"`
	Text         string  `json:"text"`
	OptionA      string  `json:"option_a"`
	OptionB      string  `json:"option_b"`
	OptionC      string  `json:"option_c"`
	OptionD      string  `json:"option_d"`
	CorrectIndex *int    `json:"correct_index"`
	Difficulty   *string `json:"difficulty"`
}

// Create handles POST /api/v1/questions.
func (h *Handler) Create(c *fiber.Ctx) error {
	var req createRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusUnprocessableEntity).JSON(api.ErrorResponse{
			Error: api.ErrorDetail{Code: "VALIDATION_ERROR", Message: "invalid request body"},
		})
	}

	if req.CategoryID == "" {
		return c.Status(fiber.StatusUnprocessableEntity).JSON(api.ErrorResponse{
			Error: api.ErrorDetail{Code: "VALIDATION_ERROR", Message: "category_id: required"},
		})
	}
	catID, err := uuid.Parse(req.CategoryID)
	if err != nil {
		return c.Status(fiber.StatusUnprocessableEntity).JSON(api.ErrorResponse{
			Error: api.ErrorDetail{Code: "VALIDATION_ERROR", Message: "category_id: must be a valid UUID"},
		})
	}
	if req.Text == "" {
		return c.Status(fiber.StatusUnprocessableEntity).JSON(api.ErrorResponse{
			Error: api.ErrorDetail{Code: "VALIDATION_ERROR", Message: "text: required"},
		})
	}
	if req.OptionA == "" || req.OptionB == "" || req.OptionC == "" || req.OptionD == "" {
		return c.Status(fiber.StatusUnprocessableEntity).JSON(api.ErrorResponse{
			Error: api.ErrorDetail{Code: "VALIDATION_ERROR", Message: "option_a, option_b, option_c, option_d: all required and non-empty"},
		})
	}
	if req.CorrectIndex == nil {
		return c.Status(fiber.StatusUnprocessableEntity).JSON(api.ErrorResponse{
			Error: api.ErrorDetail{Code: "VALIDATION_ERROR", Message: "correct_index: required"},
		})
	}
	if *req.CorrectIndex < 0 || *req.CorrectIndex > 3 {
		return c.Status(fiber.StatusUnprocessableEntity).JSON(api.ErrorResponse{
			Error: api.ErrorDetail{Code: "VALIDATION_ERROR", Message: "correct_index: must be between 0 and 3"},
		})
	}

	difficulty := "medium"
	if req.Difficulty != nil {
		if !validDifficulties[*req.Difficulty] {
			return c.Status(fiber.StatusUnprocessableEntity).JSON(api.ErrorResponse{
				Error: api.ErrorDetail{Code: "VALIDATION_ERROR", Message: "difficulty: must be easy, medium, or hard"},
			})
		}
		difficulty = *req.Difficulty
	}

	q := Question{
		CategoryID:   catID,
		Text:         req.Text,
		OptionA:      req.OptionA,
		OptionB:      req.OptionB,
		OptionC:      req.OptionC,
		OptionD:      req.OptionD,
		CorrectIndex: *req.CorrectIndex,
		Difficulty:   difficulty,
	}

	result, err := h.svc.Create(c.Context(), q)
	if err != nil {
		if errors.Is(err, ErrQuestionNotFound) {
			return c.Status(fiber.StatusUnprocessableEntity).JSON(api.ErrorResponse{
				Error: api.ErrorDetail{Code: "VALIDATION_ERROR", Message: "category_id: category not found"},
			})
		}
		return c.Status(fiber.StatusInternalServerError).JSON(api.ErrorResponse{
			Error: api.ErrorDetail{Code: "INTERNAL_ERROR", Message: "an unexpected error occurred"},
		})
	}

	return c.Status(fiber.StatusCreated).JSON(api.DataResponse{
		Data: toQuestionResponse(*result),
	})
}

type updateRequest struct {
	CategoryID   *string `json:"category_id"`
	Text         *string `json:"text"`
	OptionA      *string `json:"option_a"`
	OptionB      *string `json:"option_b"`
	OptionC      *string `json:"option_c"`
	OptionD      *string `json:"option_d"`
	CorrectIndex *int    `json:"correct_index"`
	Difficulty   *string `json:"difficulty"`
}

// Update handles PATCH /api/v1/questions/:id.
func (h *Handler) Update(c *fiber.Ctx) error {
	id, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return c.Status(fiber.StatusUnprocessableEntity).JSON(api.ErrorResponse{
			Error: api.ErrorDetail{Code: "VALIDATION_ERROR", Message: "id: must be a valid UUID"},
		})
	}

	var req updateRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusUnprocessableEntity).JSON(api.ErrorResponse{
			Error: api.ErrorDetail{Code: "VALIDATION_ERROR", Message: "invalid request body"},
		})
	}

	var update QuestionUpdate

	if req.CategoryID != nil {
		catID, err := uuid.Parse(*req.CategoryID)
		if err != nil {
			return c.Status(fiber.StatusUnprocessableEntity).JSON(api.ErrorResponse{
				Error: api.ErrorDetail{Code: "VALIDATION_ERROR", Message: "category_id: must be a valid UUID"},
			})
		}
		update.CategoryID = &catID
	}
	if req.Text != nil {
		if *req.Text == "" {
			return c.Status(fiber.StatusUnprocessableEntity).JSON(api.ErrorResponse{
				Error: api.ErrorDetail{Code: "VALIDATION_ERROR", Message: "text: must not be empty"},
			})
		}
		update.Text = req.Text
	}
	if req.OptionA != nil {
		if *req.OptionA == "" {
			return c.Status(fiber.StatusUnprocessableEntity).JSON(api.ErrorResponse{
				Error: api.ErrorDetail{Code: "VALIDATION_ERROR", Message: "option_a: must not be empty"},
			})
		}
		update.OptionA = req.OptionA
	}
	if req.OptionB != nil {
		if *req.OptionB == "" {
			return c.Status(fiber.StatusUnprocessableEntity).JSON(api.ErrorResponse{
				Error: api.ErrorDetail{Code: "VALIDATION_ERROR", Message: "option_b: must not be empty"},
			})
		}
		update.OptionB = req.OptionB
	}
	if req.OptionC != nil {
		if *req.OptionC == "" {
			return c.Status(fiber.StatusUnprocessableEntity).JSON(api.ErrorResponse{
				Error: api.ErrorDetail{Code: "VALIDATION_ERROR", Message: "option_c: must not be empty"},
			})
		}
		update.OptionC = req.OptionC
	}
	if req.OptionD != nil {
		if *req.OptionD == "" {
			return c.Status(fiber.StatusUnprocessableEntity).JSON(api.ErrorResponse{
				Error: api.ErrorDetail{Code: "VALIDATION_ERROR", Message: "option_d: must not be empty"},
			})
		}
		update.OptionD = req.OptionD
	}
	if req.CorrectIndex != nil {
		if *req.CorrectIndex < 0 || *req.CorrectIndex > 3 {
			return c.Status(fiber.StatusUnprocessableEntity).JSON(api.ErrorResponse{
				Error: api.ErrorDetail{Code: "VALIDATION_ERROR", Message: "correct_index: must be between 0 and 3"},
			})
		}
		update.CorrectIndex = req.CorrectIndex
	}
	if req.Difficulty != nil {
		if !validDifficulties[*req.Difficulty] {
			return c.Status(fiber.StatusUnprocessableEntity).JSON(api.ErrorResponse{
				Error: api.ErrorDetail{Code: "VALIDATION_ERROR", Message: "difficulty: must be easy, medium, or hard"},
			})
		}
		update.Difficulty = req.Difficulty
	}

	result, err := h.svc.Update(c.Context(), id, update)
	if err != nil {
		if errors.Is(err, ErrQuestionNotFound) {
			return c.Status(fiber.StatusNotFound).JSON(api.ErrorResponse{
				Error: api.ErrorDetail{Code: "NOT_FOUND", Message: "Question not found"},
			})
		}
		if errors.Is(err, ErrQuestionInUse) {
			return c.Status(fiber.StatusConflict).JSON(api.ErrorResponse{
				Error: api.ErrorDetail{Code: "QUESTION_IN_USE", Message: "Cannot modify a question used in an active session"},
			})
		}
		return c.Status(fiber.StatusInternalServerError).JSON(api.ErrorResponse{
			Error: api.ErrorDetail{Code: "INTERNAL_ERROR", Message: "an unexpected error occurred"},
		})
	}

	return c.Status(fiber.StatusOK).JSON(api.DataResponse{
		Data: toQuestionResponse(*result),
	})
}

// Delete handles DELETE /api/v1/questions/:id.
func (h *Handler) Delete(c *fiber.Ctx) error {
	id, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return c.Status(fiber.StatusUnprocessableEntity).JSON(api.ErrorResponse{
			Error: api.ErrorDetail{Code: "VALIDATION_ERROR", Message: "id: must be a valid UUID"},
		})
	}

	if err := h.svc.Delete(c.Context(), id); err != nil {
		if errors.Is(err, ErrQuestionNotFound) {
			return c.Status(fiber.StatusNotFound).JSON(api.ErrorResponse{
				Error: api.ErrorDetail{Code: "NOT_FOUND", Message: "Question not found"},
			})
		}
		if errors.Is(err, ErrQuestionInUse) {
			return c.Status(fiber.StatusConflict).JSON(api.ErrorResponse{
				Error: api.ErrorDetail{Code: "QUESTION_IN_USE", Message: "Cannot delete a question used in an active session"},
			})
		}
		return c.Status(fiber.StatusInternalServerError).JSON(api.ErrorResponse{
			Error: api.ErrorDetail{Code: "INTERNAL_ERROR", Message: "an unexpected error occurred"},
		})
	}

	return c.Status(fiber.StatusOK).JSON(api.DataResponse{
		Data: map[string]bool{"ok": true},
	})
}
