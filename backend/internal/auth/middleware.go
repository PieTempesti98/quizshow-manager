package auth

import (
	"strings"

	"github.com/gofiber/fiber/v2"
	"github.com/PieTempesti98/quizshow/internal/api"
)

// RequireAdmin returns a Fiber middleware that validates the admin Bearer token.
// Injects AdminClaims into the context under ClaimsKey on success.
// Returns 401 for missing/invalid/expired tokens; 403 for non-admin role.
func RequireAdmin(cfg Config) fiber.Handler {
	return func(c *fiber.Ctx) error {
		authHeader := c.Get("Authorization")
		if authHeader == "" {
			return c.Status(fiber.StatusUnauthorized).JSON(api.ErrorResponse{
				Error: api.ErrorDetail{Code: "UNAUTHORIZED", Message: "Missing or invalid token"},
			})
		}

		parts := strings.SplitN(authHeader, " ", 2)
		if len(parts) != 2 || parts[0] != "Bearer" || parts[1] == "" {
			return c.Status(fiber.StatusUnauthorized).JSON(api.ErrorResponse{
				Error: api.ErrorDetail{Code: "UNAUTHORIZED", Message: "Missing or invalid token"},
			})
		}

		claims, err := ValidateClaims(parts[1], cfg)
		if err != nil {
			return c.Status(fiber.StatusUnauthorized).JSON(api.ErrorResponse{
				Error: api.ErrorDetail{Code: "UNAUTHORIZED", Message: "Missing or invalid token"},
			})
		}

		if claims.Role != "admin" {
			return c.Status(fiber.StatusForbidden).JSON(api.ErrorResponse{
				Error: api.ErrorDetail{Code: "FORBIDDEN", Message: "Insufficient permissions"},
			})
		}

		c.Locals(ClaimsKey, claims)
		return c.Next()
	}
}
