package auth

import (
	"encoding/json"
	"io"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

// newTestApp wires a minimal Fiber app with RequireAdmin protecting GET /protected.
func newTestApp(cfg Config) *fiber.App {
	app := fiber.New(fiber.Config{ErrorHandler: func(c *fiber.Ctx, _ error) error {
		return c.Status(500).SendString("internal error")
	}})
	app.Get("/protected", RequireAdmin(cfg), func(c *fiber.Ctx) error {
		claims := c.Locals(ClaimsKey).(AdminClaims)
		return c.Status(200).JSON(fiber.Map{"admin_id": claims.AdminID})
	})
	return app
}

func doGet(app *fiber.App, authHeader string) *httptest.ResponseRecorder {
	req := httptest.NewRequest("GET", "/protected", nil)
	if authHeader != "" {
		req.Header.Set("Authorization", authHeader)
	}
	resp, _ := app.Test(req)
	_ = resp
	return nil // used inline below
}

func testMiddleware(t *testing.T, authHeader string, expectedStatus int) {
	t.Helper()
	app := newTestApp(testCfg)
	req := httptest.NewRequest("GET", "/protected", nil)
	if authHeader != "" {
		req.Header.Set("Authorization", authHeader)
	}
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("app.Test: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != expectedStatus {
		body, _ := io.ReadAll(resp.Body)
		t.Errorf("expected status %d, got %d; body: %s", expectedStatus, resp.StatusCode, body)
	}
}

func TestRequireAdmin_NoHeader(t *testing.T) {
	testMiddleware(t, "", 401)
}

func TestRequireAdmin_MalformedHeader(t *testing.T) {
	testMiddleware(t, "NotBearer token", 401)
}

func TestRequireAdmin_ExpiredToken(t *testing.T) {
	adminID := uuid.New()
	claims := jwtClaims{
		Role: "admin",
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    testCfg.JWTIssuer,
			Subject:   adminID.String(),
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(-1 * time.Minute)),
			IssuedAt:  jwt.NewNumericDate(time.Now().Add(-2 * time.Minute)),
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signed, _ := token.SignedString(testCfg.JWTSecret)
	testMiddleware(t, "Bearer "+signed, 401)
}

func TestRequireAdmin_WrongIssuer(t *testing.T) {
	wrongCfg := testCfg
	wrongCfg.JWTIssuer = "https://attacker.local"
	adminID := uuid.New()
	token, _, _ := IssueAccessToken(adminID, wrongCfg)
	testMiddleware(t, "Bearer "+token, 401)
}

func TestRequireAdmin_PlayerRole(t *testing.T) {
	adminID := uuid.New()
	// craft a token with role=player
	claims := jwtClaims{
		Role: "player",
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    testCfg.JWTIssuer,
			Subject:   adminID.String(),
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(15 * time.Minute)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signed, _ := token.SignedString(testCfg.JWTSecret)
	testMiddleware(t, "Bearer "+signed, 403)
}

func TestRequireAdmin_ValidToken(t *testing.T) {
	adminID := uuid.New()
	token, _, _ := IssueAccessToken(adminID, testCfg)

	app := newTestApp(testCfg)
	req := httptest.NewRequest("GET", "/protected", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("app.Test: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("expected 200, got %d; body: %s", resp.StatusCode, body)
	}

	var result struct {
		AdminID string `json:"admin_id"`
	}
	body, _ := io.ReadAll(resp.Body)
	if err := json.Unmarshal(body, &result); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if result.AdminID != adminID.String() {
		t.Errorf("expected admin_id %s, got %s", adminID, result.AdminID)
	}
}
