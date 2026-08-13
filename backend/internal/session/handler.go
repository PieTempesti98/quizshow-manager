package session

import (
	"errors"
	"strings"

	"github.com/PieTempesti98/quizshow/internal/api"
	"github.com/PieTempesti98/quizshow/internal/auth"
	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
)

// Handler holds the HTTP handlers for session endpoints.
type Handler struct {
	svc Service
}

// NewHandler constructs a Handler.
func NewHandler(svc Service) *Handler {
	return &Handler{svc: svc}
}

// --- response shapes ---

type categoryRefResponse struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type sessionResponse struct {
	ID                string  `json:"id"`
	Name              string  `json:"name"`
	PIN               string  `json:"pin"`
	Status            string  `json:"status"`
	QuestionCount     int16   `json:"question_count"`
	TimePerQuestionS  int16   `json:"time_per_question_s"`
	PointsPerAnswer   int     `json:"points_per_answer"`
	SpeedBonusEnabled bool    `json:"speed_bonus_enabled"`
	PlayerCount       int     `json:"player_count"`
	StartedAt         *string `json:"started_at"`
	EndedAt           *string `json:"ended_at"`
	CreatedAt         string  `json:"created_at"`
}

type sessionCreateResponse struct {
	sessionResponse
	AvailableQuestions int    `json:"available_questions"`
	Warning            string `json:"warning,omitempty"`
}

type sessionDetailResponse struct {
	sessionResponse
	Categories []categoryRefResponse `json:"categories"`
}

type paginationResponse struct {
	Page       int `json:"page"`
	PerPage    int `json:"per_page"`
	Total      int `json:"total"`
	TotalPages int `json:"total_pages"`
}

func toSessionResponse(s Session, playerCount int) sessionResponse {
	r := sessionResponse{
		ID:                s.ID.String(),
		Name:              s.Name,
		PIN:               s.PIN,
		Status:            s.Status,
		QuestionCount:     s.QuestionCount,
		TimePerQuestionS:  s.TimePerQuestionS,
		PointsPerAnswer:   s.PointsPerAnswer,
		SpeedBonusEnabled: s.SpeedBonusEnabled,
		PlayerCount:       playerCount,
		CreatedAt:         s.CreatedAt.UTC().Format("2006-01-02T15:04:05Z"),
	}
	if s.StartedAt != nil {
		t := s.StartedAt.UTC().Format("2006-01-02T15:04:05Z")
		r.StartedAt = &t
	}
	if s.EndedAt != nil {
		t := s.EndedAt.UTC().Format("2006-01-02T15:04:05Z")
		r.EndedAt = &t
	}
	return r
}

// --- request shapes ---

type createRequest struct {
	Name              string   `json:"name"`
	CategoryIDs       []string `json:"category_ids"`
	QuestionCount     *int16   `json:"question_count"`
	TimePerQuestionS  *int16   `json:"time_per_question_s"`
	PointsPerAnswer   *int     `json:"points_per_answer"`
	SpeedBonusEnabled *bool    `json:"speed_bonus_enabled"`
}

type updateRequest struct {
	Name              *string  `json:"name"`
	CategoryIDs       []string `json:"category_ids"`
	QuestionCount     *int16   `json:"question_count"`
	TimePerQuestionS  *int16   `json:"time_per_question_s"`
	PointsPerAnswer   *int     `json:"points_per_answer"`
	SpeedBonusEnabled *bool    `json:"speed_bonus_enabled"`
}

type endRequest struct {
	Reason string `json:"reason"`
}

// --- handlers ---

// Create handles POST /api/v1/sessions.
func (h *Handler) Create(c *fiber.Ctx) error {
	var req createRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusUnprocessableEntity).JSON(api.ErrorResponse{
			Error: api.ErrorDetail{Code: "VALIDATION_ERROR", Message: "invalid request body"},
		})
	}

	if req.QuestionCount == nil {
		return c.Status(fiber.StatusUnprocessableEntity).JSON(api.ErrorResponse{
			Error: api.ErrorDetail{Code: "VALIDATION_ERROR", Message: "question_count: required"},
		})
	}
	if req.TimePerQuestionS == nil {
		return c.Status(fiber.StatusUnprocessableEntity).JSON(api.ErrorResponse{
			Error: api.ErrorDetail{Code: "VALIDATION_ERROR", Message: "time_per_question_s: required"},
		})
	}

	catIDs, err := parseCategoryIDs(req.CategoryIDs)
	if err != nil {
		return c.Status(fiber.StatusUnprocessableEntity).JSON(api.ErrorResponse{
			Error: api.ErrorDetail{Code: "VALIDATION_ERROR", Message: err.Error()},
		})
	}

	claims := c.Locals(auth.ClaimsKey).(auth.AdminClaims)

	sc := SessionCreate{
		Name:              req.Name,
		CategoryIDs:       catIDs,
		QuestionCount:     *req.QuestionCount,
		TimePerQuestionS:  *req.TimePerQuestionS,
		PointsPerAnswer:   req.PointsPerAnswer,
		SpeedBonusEnabled: req.SpeedBonusEnabled,
	}

	sess, available, warning, err := h.svc.Create(c.Context(), sc, claims.AdminID)
	if err != nil {
		if isValidationError(err) {
			return c.Status(fiber.StatusUnprocessableEntity).JSON(api.ErrorResponse{
				Error: api.ErrorDetail{Code: "VALIDATION_ERROR", Message: trimValidationPrefix(err)},
			})
		}
		return c.Status(fiber.StatusInternalServerError).JSON(api.ErrorResponse{
			Error: api.ErrorDetail{Code: "INTERNAL_ERROR", Message: "an unexpected error occurred"},
		})
	}

	return c.Status(fiber.StatusCreated).JSON(api.DataResponse{
		Data: sessionCreateResponse{
			sessionResponse:    toSessionResponse(sess, 0),
			AvailableQuestions: available,
			Warning:            warning,
		},
	})
}

// List handles GET /api/v1/sessions.
func (h *Handler) List(c *fiber.Ctx) error {
	page := c.QueryInt("page", 1)
	perPage := c.QueryInt("per_page", 20)

	var statuses []string
	if raw := c.Query("status"); raw != "" {
		for _, s := range strings.Split(raw, ",") {
			s = strings.TrimSpace(s)
			if s != "" {
				statuses = append(statuses, s)
			}
		}
	}

	result, err := h.svc.List(c.Context(), SessionFilter{
		Statuses: statuses,
		Page:     page,
		PerPage:  perPage,
	})
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(api.ErrorResponse{
			Error: api.ErrorDetail{Code: "INTERNAL_ERROR", Message: "an unexpected error occurred"},
		})
	}

	sessions := make([]sessionResponse, len(result.Sessions))
	for i, item := range result.Sessions {
		sessions[i] = toSessionResponse(item.Session, item.PlayerCount)
	}

	return c.Status(fiber.StatusOK).JSON(api.DataResponse{
		Data: map[string]any{
			"sessions": sessions,
			"pagination": paginationResponse{
				Page:       result.Page,
				PerPage:    result.PerPage,
				Total:      result.Total,
				TotalPages: result.TotalPages,
			},
		},
	})
}

// FindByID handles GET /api/v1/sessions/:id.
func (h *Handler) FindByID(c *fiber.Ctx) error {
	id, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return c.Status(fiber.StatusUnprocessableEntity).JSON(api.ErrorResponse{
			Error: api.ErrorDetail{Code: "VALIDATION_ERROR", Message: "id: must be a valid UUID"},
		})
	}

	detail, err := h.svc.FindByID(c.Context(), id)
	if err != nil {
		if errors.Is(err, ErrSessionNotFound) {
			return c.Status(fiber.StatusNotFound).JSON(api.ErrorResponse{
				Error: api.ErrorDetail{Code: "NOT_FOUND", Message: "session not found"},
			})
		}
		return c.Status(fiber.StatusInternalServerError).JSON(api.ErrorResponse{
			Error: api.ErrorDetail{Code: "INTERNAL_ERROR", Message: "an unexpected error occurred"},
		})
	}

	cats := make([]categoryRefResponse, len(detail.Categories))
	for i, cat := range detail.Categories {
		cats[i] = categoryRefResponse{ID: cat.ID.String(), Name: cat.Name}
	}

	return c.Status(fiber.StatusOK).JSON(api.DataResponse{
		Data: sessionDetailResponse{
			sessionResponse: toSessionResponse(detail.Session, detail.PlayerCount),
			Categories:      cats,
		},
	})
}

// Update handles PATCH /api/v1/sessions/:id.
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

	u := SessionUpdate{
		Name:              req.Name,
		QuestionCount:     req.QuestionCount,
		TimePerQuestionS:  req.TimePerQuestionS,
		PointsPerAnswer:   req.PointsPerAnswer,
		SpeedBonusEnabled: req.SpeedBonusEnabled,
	}

	if req.CategoryIDs != nil {
		catIDs, err := parseCategoryIDs(req.CategoryIDs)
		if err != nil {
			return c.Status(fiber.StatusUnprocessableEntity).JSON(api.ErrorResponse{
				Error: api.ErrorDetail{Code: "VALIDATION_ERROR", Message: err.Error()},
			})
		}
		u.CategoryIDs = catIDs
	}

	sess, available, warning, err := h.svc.Update(c.Context(), id, u)
	if err != nil {
		if errors.Is(err, ErrSessionNotFound) {
			return c.Status(fiber.StatusNotFound).JSON(api.ErrorResponse{
				Error: api.ErrorDetail{Code: "NOT_FOUND", Message: "session not found"},
			})
		}
		if errors.Is(err, ErrSessionNotDraft) {
			return c.Status(fiber.StatusConflict).JSON(api.ErrorResponse{
				Error: api.ErrorDetail{Code: "SESSION_NOT_DRAFT", Message: "session cannot be modified in its current status"},
			})
		}
		if isValidationError(err) {
			return c.Status(fiber.StatusUnprocessableEntity).JSON(api.ErrorResponse{
				Error: api.ErrorDetail{Code: "VALIDATION_ERROR", Message: trimValidationPrefix(err)},
			})
		}
		return c.Status(fiber.StatusInternalServerError).JSON(api.ErrorResponse{
			Error: api.ErrorDetail{Code: "INTERNAL_ERROR", Message: "an unexpected error occurred"},
		})
	}

	return c.Status(fiber.StatusOK).JSON(api.DataResponse{
		Data: sessionCreateResponse{
			sessionResponse:    toSessionResponse(sess, 0),
			AvailableQuestions: available,
			Warning:            warning,
		},
	})
}

// Delete handles DELETE /api/v1/sessions/:id.
func (h *Handler) Delete(c *fiber.Ctx) error {
	id, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return c.Status(fiber.StatusUnprocessableEntity).JSON(api.ErrorResponse{
			Error: api.ErrorDetail{Code: "VALIDATION_ERROR", Message: "id: must be a valid UUID"},
		})
	}

	if err := h.svc.Delete(c.Context(), id); err != nil {
		if errors.Is(err, ErrSessionNotFound) {
			return c.Status(fiber.StatusNotFound).JSON(api.ErrorResponse{
				Error: api.ErrorDetail{Code: "NOT_FOUND", Message: "session not found"},
			})
		}
		if errors.Is(err, ErrSessionNotDraft) {
			return c.Status(fiber.StatusConflict).JSON(api.ErrorResponse{
				Error: api.ErrorDetail{Code: "SESSION_NOT_DRAFT", Message: "session cannot be deleted in its current status"},
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

// OpenLobby handles POST /api/v1/sessions/:id/open-lobby.
func (h *Handler) OpenLobby(c *fiber.Ctx) error {
	id, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return c.Status(fiber.StatusUnprocessableEntity).JSON(api.ErrorResponse{
			Error: api.ErrorDetail{Code: "VALIDATION_ERROR", Message: "id: must be a valid UUID"},
		})
	}

	result, err := h.svc.OpenLobby(c.Context(), id)
	if err != nil {
		if errors.Is(err, ErrSessionNotFound) {
			return c.Status(fiber.StatusNotFound).JSON(api.ErrorResponse{
				Error: api.ErrorDetail{Code: "NOT_FOUND", Message: "session not found"},
			})
		}
		if errors.Is(err, ErrSessionNotDraft) {
			return c.Status(fiber.StatusConflict).JSON(api.ErrorResponse{
				Error: api.ErrorDetail{Code: "SESSION_NOT_DRAFT", Message: "session cannot be opened in its current status"},
			})
		}
		return c.Status(fiber.StatusInternalServerError).JSON(api.ErrorResponse{
			Error: api.ErrorDetail{Code: "INTERNAL_ERROR", Message: "an unexpected error occurred"},
		})
	}

	return c.Status(fiber.StatusOK).JSON(api.DataResponse{
		Data: map[string]string{
			"session_id":  result.SessionID,
			"pin":         result.PIN,
			"qr_code_url": result.QRCodeURL,
			"status":      result.Status,
		},
	})
}

// GetQR handles GET /api/v1/sessions/:id/qr.
func (h *Handler) GetQR(c *fiber.Ctx) error {
	id, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return c.Status(fiber.StatusUnprocessableEntity).JSON(api.ErrorResponse{
			Error: api.ErrorDetail{Code: "VALIDATION_ERROR", Message: "id: must be a valid UUID"},
		})
	}

	png, err := h.svc.GetQR(c.Context(), id)
	if err != nil {
		if errors.Is(err, ErrSessionNotFound) {
			return c.Status(fiber.StatusNotFound).JSON(api.ErrorResponse{
				Error: api.ErrorDetail{Code: "NOT_FOUND", Message: "session not found"},
			})
		}
		return c.Status(fiber.StatusInternalServerError).JSON(api.ErrorResponse{
			Error: api.ErrorDetail{Code: "INTERNAL_ERROR", Message: "an unexpected error occurred"},
		})
	}

	c.Set("Content-Type", "image/png")
	return c.Status(fiber.StatusOK).Send(png)
}

// Launch handles POST /api/v1/sessions/:id/launch.
func (h *Handler) Launch(c *fiber.Ctx) error {
	id, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return c.Status(fiber.StatusUnprocessableEntity).JSON(api.ErrorResponse{
			Error: api.ErrorDetail{Code: "VALIDATION_ERROR", Message: "id: must be a valid UUID"},
		})
	}

	result, err := h.svc.Launch(c.Context(), id)
	if err != nil {
		if errors.Is(err, ErrSessionNotFound) {
			return c.Status(fiber.StatusNotFound).JSON(api.ErrorResponse{
				Error: api.ErrorDetail{Code: "NOT_FOUND", Message: "session not found"},
			})
		}
		if errors.Is(err, ErrSessionNotInLobby) {
			return c.Status(fiber.StatusConflict).JSON(api.ErrorResponse{
				Error: api.ErrorDetail{Code: "SESSION_NOT_IN_LOBBY", Message: "session cannot be launched in its current status"},
			})
		}
		if errors.Is(err, ErrInsufficientQuestions) {
			return c.Status(fiber.StatusUnprocessableEntity).JSON(api.ErrorResponse{
				Error: api.ErrorDetail{Code: "INSUFFICIENT_QUESTIONS", Message: "no questions available in the configured categories"},
			})
		}
		return c.Status(fiber.StatusInternalServerError).JSON(api.ErrorResponse{
			Error: api.ErrorDetail{Code: "INTERNAL_ERROR", Message: "an unexpected error occurred"},
		})
	}

	return c.Status(fiber.StatusOK).JSON(api.DataResponse{
		Data: map[string]any{
			"session_id":       result.SessionID,
			"status":           result.Status,
			"question_count":   result.QuestionCount,
			"projection_token": result.ProjectionToken,
			"projection_url":   result.ProjectionURL,
			"started_at":       result.StartedAt.UTC().Format("2006-01-02T15:04:05Z"),
		},
	})
}

// NextQuestion handles POST /api/v1/sessions/:id/next-question.
func (h *Handler) NextQuestion(c *fiber.Ctx) error {
	id, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return c.Status(fiber.StatusUnprocessableEntity).JSON(api.ErrorResponse{
			Error: api.ErrorDetail{Code: "VALIDATION_ERROR", Message: "id: must be a valid UUID"},
		})
	}

	result, err := h.svc.NextQuestion(c.Context(), id)
	if err != nil {
		if errors.Is(err, ErrSessionNotFound) {
			return c.Status(fiber.StatusNotFound).JSON(api.ErrorResponse{
				Error: api.ErrorDetail{Code: "NOT_FOUND", Message: "session not found"},
			})
		}
		if errors.Is(err, ErrSessionNotActive) {
			return c.Status(fiber.StatusConflict).JSON(api.ErrorResponse{
				Error: api.ErrorDetail{Code: "SESSION_NOT_ACTIVE", Message: "session must be active to advance questions"},
			})
		}
		if errors.Is(err, ErrQuestionNotRevealed) {
			return c.Status(fiber.StatusConflict).JSON(api.ErrorResponse{
				Error: api.ErrorDetail{Code: "QUESTION_NOT_REVEALED", Message: "current question must be revealed before advancing"},
			})
		}
		if errors.Is(err, ErrNoMoreQuestions) {
			return c.Status(fiber.StatusConflict).JSON(api.ErrorResponse{
				Error: api.ErrorDetail{Code: "NO_MORE_QUESTIONS", Message: "all questions have been asked; end session instead"},
			})
		}
		return c.Status(fiber.StatusInternalServerError).JSON(api.ErrorResponse{
			Error: api.ErrorDetail{Code: "INTERNAL_ERROR", Message: "an unexpected error occurred"},
		})
	}

	return c.Status(fiber.StatusOK).JSON(api.DataResponse{
		Data: map[string]any{
			"session_question_id": result.SessionQuestionID,
			"position":          result.Position,
			"total":             result.Total,
			"question":          result.Question,
			"time_limit_s":      result.TimeLimitS,
			"asked_at":          result.AskedAt.UTC().Format("2006-01-02T15:04:05Z"),
		},
	})
}

// PauseTimer handles POST /api/v1/sessions/:id/pause-timer.
func (h *Handler) PauseTimer(c *fiber.Ctx) error {
	id, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return c.Status(fiber.StatusUnprocessableEntity).JSON(api.ErrorResponse{
			Error: api.ErrorDetail{Code: "VALIDATION_ERROR", Message: "id: must be a valid UUID"},
		})
	}

	result, err := h.svc.PauseTimer(c.Context(), id)
	if err != nil {
		if errors.Is(err, ErrSessionNotFound) {
			return c.Status(fiber.StatusNotFound).JSON(api.ErrorResponse{
				Error: api.ErrorDetail{Code: "NOT_FOUND", Message: "session not found"},
			})
		}
		if errors.Is(err, ErrSessionNotActive) {
			return c.Status(fiber.StatusConflict).JSON(api.ErrorResponse{
				Error: api.ErrorDetail{Code: "SESSION_NOT_ACTIVE", Message: "session must be active to pause timer"},
			})
		}
		if errors.Is(err, ErrNoActiveQuestion) {
			return c.Status(fiber.StatusConflict).JSON(api.ErrorResponse{
				Error: api.ErrorDetail{Code: "NO_ACTIVE_QUESTION", Message: "no question currently in progress"},
			})
		}
		if errors.Is(err, ErrTimerAlreadyPaused) {
			return c.Status(fiber.StatusConflict).JSON(api.ErrorResponse{
				Error: api.ErrorDetail{Code: "TIMER_ALREADY_PAUSED", Message: "timer is already paused"},
			})
		}
		return c.Status(fiber.StatusInternalServerError).JSON(api.ErrorResponse{
			Error: api.ErrorDetail{Code: "INTERNAL_ERROR", Message: "an unexpected error occurred"},
		})
	}

	return c.Status(fiber.StatusOK).JSON(api.DataResponse{
		Data: map[string]any{
			"ok":        result.Ok,
			"paused_at": result.PausedAt.UTC().Format("2006-01-02T15:04:05Z"),
		},
	})
}

// ResumeTimer handles POST /api/v1/sessions/:id/resume-timer.
func (h *Handler) ResumeTimer(c *fiber.Ctx) error {
	id, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return c.Status(fiber.StatusUnprocessableEntity).JSON(api.ErrorResponse{
			Error: api.ErrorDetail{Code: "VALIDATION_ERROR", Message: "id: must be a valid UUID"},
		})
	}

	result, err := h.svc.ResumeTimer(c.Context(), id)
	if err != nil {
		if errors.Is(err, ErrSessionNotFound) {
			return c.Status(fiber.StatusNotFound).JSON(api.ErrorResponse{
				Error: api.ErrorDetail{Code: "NOT_FOUND", Message: "session not found"},
			})
		}
		if errors.Is(err, ErrSessionNotActive) {
			return c.Status(fiber.StatusConflict).JSON(api.ErrorResponse{
				Error: api.ErrorDetail{Code: "SESSION_NOT_ACTIVE", Message: "session must be active to resume timer"},
			})
		}
		if errors.Is(err, ErrNoActiveQuestion) {
			return c.Status(fiber.StatusConflict).JSON(api.ErrorResponse{
				Error: api.ErrorDetail{Code: "NO_ACTIVE_QUESTION", Message: "no question currently in progress"},
			})
		}
		if errors.Is(err, ErrTimerNotPaused) {
			return c.Status(fiber.StatusConflict).JSON(api.ErrorResponse{
				Error: api.ErrorDetail{Code: "TIMER_NOT_PAUSED", Message: "timer is not paused"},
			})
		}
		return c.Status(fiber.StatusInternalServerError).JSON(api.ErrorResponse{
			Error: api.ErrorDetail{Code: "INTERNAL_ERROR", Message: "an unexpected error occurred"},
		})
	}

	return c.Status(fiber.StatusOK).JSON(api.DataResponse{
		Data: map[string]any{
			"ok":         result.Ok,
			"resumed_at": result.ResumedAt.UTC().Format("2006-01-02T15:04:05Z"),
		},
	})
}

// Reveal handles POST /api/v1/sessions/:id/reveal.
func (h *Handler) Reveal(c *fiber.Ctx) error {
	id, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return c.Status(fiber.StatusUnprocessableEntity).JSON(api.ErrorResponse{
			Error: api.ErrorDetail{Code: "VALIDATION_ERROR", Message: "id: must be a valid UUID"},
		})
	}

	result, err := h.svc.Reveal(c.Context(), id)
	if err != nil {
		if errors.Is(err, ErrSessionNotFound) {
			return c.Status(fiber.StatusNotFound).JSON(api.ErrorResponse{
				Error: api.ErrorDetail{Code: "NOT_FOUND", Message: "session not found"},
			})
		}
		if errors.Is(err, ErrSessionNotActive) {
			return c.Status(fiber.StatusConflict).JSON(api.ErrorResponse{
				Error: api.ErrorDetail{Code: "SESSION_NOT_ACTIVE", Message: "session must be active to reveal answers"},
			})
		}
		if errors.Is(err, ErrNoActiveQuestion) {
			return c.Status(fiber.StatusConflict).JSON(api.ErrorResponse{
				Error: api.ErrorDetail{Code: "NO_ACTIVE_QUESTION", Message: "no question currently in progress"},
			})
		}
		if errors.Is(err, ErrQuestionAlreadyRevealed) {
			return c.Status(fiber.StatusConflict).JSON(api.ErrorResponse{
				Error: api.ErrorDetail{Code: "QUESTION_ALREADY_REVEALED", Message: "question has already been revealed"},
			})
		}
		return c.Status(fiber.StatusInternalServerError).JSON(api.ErrorResponse{
			Error: api.ErrorDetail{Code: "INTERNAL_ERROR", Message: "an unexpected error occurred"},
		})
	}

	return c.Status(fiber.StatusOK).JSON(api.DataResponse{
		Data: map[string]any{
			"session_question_id": result.SessionQuestionID,
			"correct_index":       result.CorrectIndex,
			"answer_distribution": result.AnswerDistribution,
			"top5":                result.Top5,
			"revealed_at":         result.RevealedAt.UTC().Format("2006-01-02T15:04:05Z"),
		},
	})
}

// End handles POST /api/v1/sessions/:id/end.
func (h *Handler) End(c *fiber.Ctx) error {
	id, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return c.Status(fiber.StatusUnprocessableEntity).JSON(api.ErrorResponse{
			Error: api.ErrorDetail{Code: "VALIDATION_ERROR", Message: "id: must be a valid UUID"},
		})
	}

	var req endRequest
	_ = c.BodyParser(&req)

	result, err := h.svc.End(c.Context(), id, req.Reason)
	if err != nil {
		if errors.Is(err, ErrSessionNotFound) {
			return c.Status(fiber.StatusNotFound).JSON(api.ErrorResponse{
				Error: api.ErrorDetail{Code: "NOT_FOUND", Message: "session not found"},
			})
		}
		if errors.Is(err, ErrSessionAlreadyEnded) {
			return c.Status(fiber.StatusConflict).JSON(api.ErrorResponse{
				Error: api.ErrorDetail{Code: "SESSION_ALREADY_ENDED", Message: "session has already ended"},
			})
		}
		return c.Status(fiber.StatusInternalServerError).JSON(api.ErrorResponse{
			Error: api.ErrorDetail{Code: "INTERNAL_ERROR", Message: "an unexpected error occurred"},
		})
	}

	return c.Status(fiber.StatusOK).JSON(api.DataResponse{
		Data: map[string]any{
			"session_id": result.SessionID,
			"status":     result.Status,
			"ended_at":   result.EndedAt.UTC().Format("2006-01-02T15:04:05Z"),
		},
	})
}

// --- helpers ---

func parseCategoryIDs(raw []string) ([]uuid.UUID, error) {
	ids := make([]uuid.UUID, 0, len(raw))
	for _, s := range raw {
		id, err := uuid.Parse(strings.TrimSpace(s))
		if err != nil {
			return nil, errors.New("category_ids: all values must be valid UUIDs")
		}
		ids = append(ids, id)
	}
	return ids, nil
}

func isValidationError(err error) bool {
	return strings.HasPrefix(err.Error(), "VALIDATION_ERROR:")
}

func trimValidationPrefix(err error) string {
	return strings.TrimPrefix(err.Error(), "VALIDATION_ERROR: ")
}
