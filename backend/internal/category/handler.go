package category

import (
	"errors"
	"fmt"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"github.com/PieTempesti98/quizshow/internal/api"
)

// Handler holds the HTTP handlers for category endpoints.
type Handler struct {
	svc Service
}

// NewHandler constructs a Handler.
func NewHandler(svc Service) *Handler {
	return &Handler{svc: svc}
}

type categoryRequest struct {
	Name string `json:"name"`
}

type categoryResponse struct {
	ID            string `json:"id"`
	Name          string `json:"name"`
	Slug          string `json:"slug"`
	QuestionCount int    `json:"question_count"`
	CreatedAt     string `json:"created_at"`
}

func toCategoryResponse(c CategoryWithCount) categoryResponse {
	return categoryResponse{
		ID:            c.ID.String(),
		Name:          c.Name,
		Slug:          c.Slug,
		QuestionCount: c.QuestionCount,
		CreatedAt:     c.CreatedAt.UTC().Format("2006-01-02T15:04:05Z"),
	}
}

func validateName(name string) *api.ErrorResponse {
	if name == "" {
		return &api.ErrorResponse{Error: api.ErrorDetail{
			Code:    "VALIDATION_ERROR",
			Message: "name: must not be empty",
		}}
	}
	if len([]rune(name)) > 50 {
		return &api.ErrorResponse{Error: api.ErrorDetail{
			Code:    "VALIDATION_ERROR",
			Message: "name: must not exceed 50 characters",
		}}
	}
	return nil
}

// List handles GET /api/v1/categories.
func (h *Handler) List(c *fiber.Ctx) error {
	categories, err := h.svc.List(c.Context())
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(api.ErrorResponse{
			Error: api.ErrorDetail{Code: "INTERNAL_ERROR", Message: "an unexpected error occurred"},
		})
	}

	resp := make([]categoryResponse, len(categories))
	for i, cat := range categories {
		resp[i] = toCategoryResponse(cat)
	}

	return c.Status(fiber.StatusOK).JSON(api.DataResponse{
		Data: map[string]any{"categories": resp},
	})
}

// Create handles POST /api/v1/categories.
func (h *Handler) Create(c *fiber.Ctx) error {
	var req categoryRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusUnprocessableEntity).JSON(api.ErrorResponse{
			Error: api.ErrorDetail{Code: "VALIDATION_ERROR", Message: "invalid request body"},
		})
	}

	if errResp := validateName(req.Name); errResp != nil {
		return c.Status(fiber.StatusUnprocessableEntity).JSON(errResp)
	}

	cat, err := h.svc.Create(c.Context(), req.Name)
	if err != nil {
		if errors.Is(err, ErrCategoryNameConflict) {
			return c.Status(fiber.StatusConflict).JSON(api.ErrorResponse{
				Error: api.ErrorDetail{Code: "CONFLICT", Message: "A category with this name already exists"},
			})
		}
		return c.Status(fiber.StatusInternalServerError).JSON(api.ErrorResponse{
			Error: api.ErrorDetail{Code: "INTERNAL_ERROR", Message: "an unexpected error occurred"},
		})
	}

	return c.Status(fiber.StatusCreated).JSON(api.DataResponse{
		Data: toCategoryResponse(*cat),
	})
}

// Rename handles PATCH /api/v1/categories/:id.
func (h *Handler) Rename(c *fiber.Ctx) error {
	id, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return c.Status(fiber.StatusUnprocessableEntity).JSON(api.ErrorResponse{
			Error: api.ErrorDetail{Code: "VALIDATION_ERROR", Message: "id: must be a valid UUID"},
		})
	}

	var req categoryRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusUnprocessableEntity).JSON(api.ErrorResponse{
			Error: api.ErrorDetail{Code: "VALIDATION_ERROR", Message: "invalid request body"},
		})
	}

	if errResp := validateName(req.Name); errResp != nil {
		return c.Status(fiber.StatusUnprocessableEntity).JSON(errResp)
	}

	cat, err := h.svc.Rename(c.Context(), id, req.Name)
	if err != nil {
		if errors.Is(err, ErrCategoryNotFound) {
			return c.Status(fiber.StatusNotFound).JSON(api.ErrorResponse{
				Error: api.ErrorDetail{Code: "NOT_FOUND", Message: "Category not found"},
			})
		}
		if errors.Is(err, ErrCategoryNameConflict) {
			return c.Status(fiber.StatusConflict).JSON(api.ErrorResponse{
				Error: api.ErrorDetail{Code: "CONFLICT", Message: "A category with this name already exists"},
			})
		}
		return c.Status(fiber.StatusInternalServerError).JSON(api.ErrorResponse{
			Error: api.ErrorDetail{Code: "INTERNAL_ERROR", Message: "an unexpected error occurred"},
		})
	}

	return c.Status(fiber.StatusOK).JSON(api.DataResponse{
		Data: toCategoryResponse(*cat),
	})
}

// Delete handles DELETE /api/v1/categories/:id.
func (h *Handler) Delete(c *fiber.Ctx) error {
	id, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return c.Status(fiber.StatusUnprocessableEntity).JSON(api.ErrorResponse{
			Error: api.ErrorDetail{Code: "VALIDATION_ERROR", Message: "id: must be a valid UUID"},
		})
	}

	if err := h.svc.Delete(c.Context(), id); err != nil {
		if errors.Is(err, ErrCategoryNotFound) {
			return c.Status(fiber.StatusNotFound).JSON(api.ErrorResponse{
				Error: api.ErrorDetail{Code: "NOT_FOUND", Message: "Category not found"},
			})
		}
		var hasQ ErrCategoryHasQuestions
		if errors.As(err, &hasQ) {
			return c.Status(fiber.StatusConflict).JSON(api.ErrorResponse{
				Error: api.ErrorDetail{
					Code:    "CATEGORY_HAS_QUESTIONS",
					Message: fmt.Sprintf("Cannot delete category with %d active questions", hasQ.Count),
				},
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
