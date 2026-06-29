package auth

import (
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

var testCfg = Config{
	JWTSecret:    []byte("test-secret-that-is-long-enough-32c"),
	JWTIssuer:    "https://test.local",
	AccessTTL:    15 * time.Minute,
	RefreshTTL:   7 * 24 * time.Hour,
	CookieSecure: false,
}

func TestIssueAccessToken(t *testing.T) {
	adminID := uuid.New()
	token, exp, err := IssueAccessToken(adminID, testCfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if token == "" {
		t.Fatal("expected non-empty token")
	}
	if exp.Before(time.Now()) {
		t.Fatal("expected expiry to be in the future")
	}
	if exp.After(time.Now().Add(16 * time.Minute)) {
		t.Fatal("expected expiry within 16 minutes")
	}

	// Parse and validate claims
	claims, err := ValidateClaims(token, testCfg)
	if err != nil {
		t.Fatalf("valid token should parse: %v", err)
	}
	if claims.AdminID != adminID {
		t.Errorf("expected AdminID %v, got %v", adminID, claims.AdminID)
	}
	if claims.Role != "admin" {
		t.Errorf("expected role 'admin', got %q", claims.Role)
	}
	if claims.Issuer != testCfg.JWTIssuer {
		t.Errorf("expected issuer %q, got %q", testCfg.JWTIssuer, claims.Issuer)
	}
}

func TestIssueRefreshToken(t *testing.T) {
	adminID := uuid.New()
	token, exp, err := IssueRefreshToken(adminID, testCfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if token == "" {
		t.Fatal("expected non-empty token")
	}
	if exp.Before(time.Now().Add(6 * 24 * time.Hour)) {
		t.Fatal("expected expiry around 7 days from now")
	}

	// The refresh token should parse but role must be "refresh", not "admin"
	claims, err := ValidateClaims(token, testCfg)
	if err != nil {
		t.Fatalf("valid refresh token should parse: %v", err)
	}
	if claims.Role != "refresh" {
		t.Errorf("expected role 'refresh', got %q", claims.Role)
	}
}

func TestHashToken(t *testing.T) {
	raw := "some-token-value"
	h1 := HashToken(raw)
	h2 := HashToken(raw)
	if h1 != h2 {
		t.Fatal("HashToken must be deterministic")
	}
	if len(h1) != 64 {
		t.Errorf("expected 64-char hex SHA-256, got len %d", len(h1))
	}
	if HashToken("different") == h1 {
		t.Fatal("different inputs must produce different hashes")
	}
}

func TestValidateClaims_ExpiredToken(t *testing.T) {
	adminID := uuid.New()
	// Craft a token that is already expired
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

	_, err := ValidateClaims(signed, testCfg)
	if err == nil {
		t.Fatal("expected error for expired token")
	}
}

func TestValidateClaims_WrongIssuer(t *testing.T) {
	adminID := uuid.New()
	wrongCfg := testCfg
	wrongCfg.JWTIssuer = "https://other.local"

	token, _, _ := IssueAccessToken(adminID, wrongCfg)
	_, err := ValidateClaims(token, testCfg)
	if err == nil {
		t.Fatal("expected error for wrong issuer")
	}
}

func TestValidateClaims_WrongSignature(t *testing.T) {
	adminID := uuid.New()
	token, _, _ := IssueAccessToken(adminID, testCfg)

	// Tamper with the token
	tampered := token[:len(token)-4] + "XXXX"
	_, err := ValidateClaims(tampered, testCfg)
	if err == nil {
		t.Fatal("expected error for tampered signature")
	}
}
