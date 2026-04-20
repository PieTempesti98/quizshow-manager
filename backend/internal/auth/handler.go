package auth

import (
	"errors"
	"regexp"

	"github.com/gofiber/fiber/v2"
	"github.com/PieTempesti98/quizshow/internal/api"
)

var emailRegex = regexp.MustCompile(`^[a-zA-Z0-9._%+\-]+@[a-zA-Z0-9.\-]+\.[a-zA-Z]{2,}$`)

// Handler holds the HTTP handlers for auth endpoints.
type Handler struct {
	svc Service
	cfg Config
}

// NewHandler constructs a Handler.
func NewHandler(svc Service, cfg Config) *Handler {
	return &Handler{svc: svc, cfg: cfg}
}

type loginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type tokenResponse struct {
	AccessToken string `json:"access_token"`
	ExpiresAt   string `json:"expires_at"`
}

// Login handles POST /api/v1/auth/login.
func (h *Handler) Login(c *fiber.Ctx) error {
	var req loginRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusUnprocessableEntity).JSON(api.ErrorResponse{
			Error: api.ErrorDetail{Code: "VALIDATION_ERROR", Message: "invalid request body"},
		})
	}

	if req.Email == "" || !emailRegex.MatchString(req.Email) {
		return c.Status(fiber.StatusUnprocessableEntity).JSON(api.ErrorResponse{
			Error: api.ErrorDetail{Code: "VALIDATION_ERROR", Message: "email: must be a valid email address"},
		})
	}
	if req.Password == "" {
		return c.Status(fiber.StatusUnprocessableEntity).JSON(api.ErrorResponse{
			Error: api.ErrorDetail{Code: "VALIDATION_ERROR", Message: "password: must not be empty"},
		})
	}

	accessToken, expiresAt, refreshToken, err := h.svc.Login(c.Context(), req.Email, req.Password)
	if err != nil {
		if errors.Is(err, ErrInvalidCredentials) {
			return c.Status(fiber.StatusUnauthorized).JSON(api.ErrorResponse{
				Error: api.ErrorDetail{Code: "UNAUTHORIZED", Message: "Invalid credentials"},
			})
		}
		return c.Status(fiber.StatusInternalServerError).JSON(api.ErrorResponse{
			Error: api.ErrorDetail{Code: "INTERNAL_ERROR", Message: "an unexpected error occurred"},
		})
	}

	maxAge := int(h.cfg.RefreshTTL.Seconds())
	c.Cookie(&fiber.Cookie{
		Name:     "qz_refresh",
		Value:    refreshToken,
		Path:     "/api/v1/auth/",
		HTTPOnly: true,
		SameSite: "Strict",
		Secure:   h.cfg.CookieSecure,
		MaxAge:   maxAge,
	})

	return c.Status(fiber.StatusOK).JSON(api.DataResponse{
		Data: tokenResponse{
			AccessToken: accessToken,
			ExpiresAt:   expiresAt.Format("2006-01-02T15:04:05Z"),
		},
	})
}

// Refresh handles POST /api/v1/auth/refresh.
func (h *Handler) Refresh(c *fiber.Ctx) error {
	rawCookie := c.Cookies("qz_refresh")
	if rawCookie == "" {
		return c.Status(fiber.StatusUnauthorized).JSON(api.ErrorResponse{
			Error: api.ErrorDetail{Code: "UNAUTHORIZED", Message: "Refresh token is invalid or expired"},
		})
	}

	accessToken, expiresAt, err := h.svc.Refresh(c.Context(), rawCookie)
	if err != nil {
		if errors.Is(err, ErrInvalidRefreshToken) {
			return c.Status(fiber.StatusUnauthorized).JSON(api.ErrorResponse{
				Error: api.ErrorDetail{Code: "UNAUTHORIZED", Message: "Refresh token is invalid or expired"},
			})
		}
		return c.Status(fiber.StatusInternalServerError).JSON(api.ErrorResponse{
			Error: api.ErrorDetail{Code: "INTERNAL_ERROR", Message: "an unexpected error occurred"},
		})
	}

	return c.Status(fiber.StatusOK).JSON(api.DataResponse{
		Data: tokenResponse{
			AccessToken: accessToken,
			ExpiresAt:   expiresAt.Format("2006-01-02T15:04:05Z"),
		},
	})
}

// Logout handles POST /api/v1/auth/logout. Requires RequireAdmin middleware.
func (h *Handler) Logout(c *fiber.Ctx) error {
	rawCookie := c.Cookies("qz_refresh")
	if rawCookie == "" {
		return c.Status(fiber.StatusUnauthorized).JSON(api.ErrorResponse{
			Error: api.ErrorDetail{Code: "UNAUTHORIZED", Message: "Missing or invalid token"},
		})
	}

	if err := h.svc.Logout(c.Context(), rawCookie); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(api.ErrorResponse{
			Error: api.ErrorDetail{Code: "INTERNAL_ERROR", Message: "an unexpected error occurred"},
		})
	}

	c.Cookie(&fiber.Cookie{
		Name:     "qz_refresh",
		Value:    "",
		Path:     "/api/v1/auth/",
		HTTPOnly: true,
		SameSite: "Strict",
		MaxAge:   0,
	})

	return c.Status(fiber.StatusOK).JSON(api.DataResponse{
		Data: map[string]bool{"ok": true},
	})
}
