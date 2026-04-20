package auth

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

// AdminClaims holds the validated claims injected into request context by RequireAdmin.
type AdminClaims struct {
	AdminID uuid.UUID
	Role    string
	Issuer  string
}

// claimsContextKey is the typed key used to store AdminClaims in Fiber's Locals.
type claimsContextKey struct{}

// ClaimsKey is the key under which AdminClaims are stored in the Fiber context.
var ClaimsKey = claimsContextKey{}

// jwtClaims is the internal JWT payload structure.
type jwtClaims struct {
	Role string `json:"role"`
	jwt.RegisteredClaims
}

// IssueAccessToken issues a signed JWT access token for the given admin.
func IssueAccessToken(adminID uuid.UUID, cfg Config) (string, time.Time, error) {
	now := time.Now().UTC()
	exp := now.Add(cfg.AccessTTL)
	claims := jwtClaims{
		Role: "admin",
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    cfg.JWTIssuer,
			Subject:   adminID.String(),
			ExpiresAt: jwt.NewNumericDate(exp),
			IssuedAt:  jwt.NewNumericDate(now),
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signed, err := token.SignedString(cfg.JWTSecret)
	if err != nil {
		return "", time.Time{}, fmt.Errorf("issue access token: %w", err)
	}
	return signed, exp, nil
}

// IssueRefreshToken issues a signed JWT refresh token for the given admin.
func IssueRefreshToken(adminID uuid.UUID, cfg Config) (string, time.Time, error) {
	now := time.Now().UTC()
	exp := now.Add(cfg.RefreshTTL)
	claims := jwtClaims{
		Role: "refresh",
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    cfg.JWTIssuer,
			Subject:   adminID.String(),
			ExpiresAt: jwt.NewNumericDate(exp),
			IssuedAt:  jwt.NewNumericDate(now),
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signed, err := token.SignedString(cfg.JWTSecret)
	if err != nil {
		return "", time.Time{}, fmt.Errorf("issue refresh token: %w", err)
	}
	return signed, exp, nil
}

// HashToken returns the hex-encoded SHA-256 of the raw token string.
func HashToken(raw string) string {
	h := sha256.Sum256([]byte(raw))
	return hex.EncodeToString(h[:])
}

// ValidateClaims parses and validates a JWT, returning the embedded AdminClaims.
// Returns an error if the token is expired, has an invalid signature, or wrong issuer.
func ValidateClaims(tokenString string, cfg Config) (AdminClaims, error) {
	var c jwtClaims
	token, err := jwt.ParseWithClaims(
		tokenString, &c,
		func(t *jwt.Token) (any, error) {
			if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
				return nil, fmt.Errorf("unexpected signing method: %v", t.Header["alg"])
			}
			return cfg.JWTSecret, nil
		},
		jwt.WithValidMethods([]string{"HS256"}),
		jwt.WithIssuer(cfg.JWTIssuer),
		jwt.WithExpirationRequired(),
	)
	if err != nil || !token.Valid {
		return AdminClaims{}, fmt.Errorf("invalid token: %w", err)
	}
	adminID, err := uuid.Parse(c.Subject)
	if err != nil {
		return AdminClaims{}, fmt.Errorf("invalid subject claim: %w", err)
	}
	return AdminClaims{
		AdminID: adminID,
		Role:    c.Role,
		Issuer:  c.Issuer,
	}, nil
}
