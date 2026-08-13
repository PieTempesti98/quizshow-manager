package auth

import (
	"strings"

	"github.com/PieTempesti98/quizshow/internal/api"
	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
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

// RequirePlayer returns a Fiber middleware that validates the player Bearer token.
// Injects PlayerClaims into the context under PlayerClaimsKey on success.
// Returns 401 for missing/invalid/expired tokens; 403 for non-player role.
func RequirePlayer(cfg Config) fiber.Handler {
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

		claims, err := ValidatePlayerClaims(parts[1], cfg)
		if err != nil {
			return c.Status(fiber.StatusUnauthorized).JSON(api.ErrorResponse{
				Error: api.ErrorDetail{Code: "UNAUTHORIZED", Message: "Missing or invalid token"},
			})
		}

		if claims.Role != "player" || claims.SessionID == uuid.Nil {
			return c.Status(fiber.StatusForbidden).JSON(api.ErrorResponse{
				Error: api.ErrorDetail{Code: "FORBIDDEN", Message: "Insufficient permissions"},
			})
		}


		c.Locals(PlayerClaimsKey, claims)
		return c.Next()
	}
}

