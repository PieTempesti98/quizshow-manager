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

// projectionClaims is the JWT payload for a projection screen token.
type projectionClaims struct {
	SessionID string `json:"session_id"`
	Role      string `json:"role"`
	jwt.RegisteredClaims
}

// IssueProjectionToken issues a 12-hour JWT for the projection screen of a specific session.
// PLACEHOLDER: the projection frontend does not exist yet; token will be used in feature #10.
func IssueProjectionToken(sessionID uuid.UUID, cfg Config) (string, error) {
	now := time.Now().UTC()
	claims := projectionClaims{
		SessionID: sessionID.String(),
		Role:      "projection",
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    cfg.JWTIssuer,
			Subject:   sessionID.String(),
			ExpiresAt: jwt.NewNumericDate(now.Add(12 * time.Hour)),
			IssuedAt:  jwt.NewNumericDate(now),
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signed, err := token.SignedString(cfg.JWTSecret)
	if err != nil {
		return "", fmt.Errorf("issue projection token: %w", err)
	}
	return signed, nil
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

// PlayerClaims holds the validated player claims injected into request context by RequirePlayer.
type PlayerClaims struct {
	PlayerID  uuid.UUID
	SessionID uuid.UUID
	Role      string
	Issuer    string
}

// playerClaimsContextKey is the typed key used to store PlayerClaims in Fiber's Locals.
type playerClaimsContextKey struct{}

// PlayerClaimsKey is the key under which PlayerClaims are stored in the Fiber context.
var PlayerClaimsKey = playerClaimsContextKey{}

// playerJwtClaims is the JWT payload for a player token.
type playerJwtClaims struct {
	SessionID string `json:"session_id"`
	Role      string `json:"role"`
	jwt.RegisteredClaims
}

// IssuePlayerToken issues a 4-hour JWT for an ephemeral player in a specific session.
func IssuePlayerToken(playerID uuid.UUID, sessionID uuid.UUID, cfg Config) (string, time.Time, error) {
	now := time.Now().UTC()
	exp := now.Add(4 * time.Hour)
	claims := playerJwtClaims{
		SessionID: sessionID.String(),
		Role:      "player",
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    cfg.JWTIssuer,
			Subject:   playerID.String(),
			ExpiresAt: jwt.NewNumericDate(exp),
			IssuedAt:  jwt.NewNumericDate(now),
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signed, err := token.SignedString(cfg.JWTSecret)
	if err != nil {
		return "", time.Time{}, fmt.Errorf("issue player token: %w", err)
	}
	return signed, exp, nil
}

// ValidatePlayerClaims parses and validates a Player JWT, returning the embedded PlayerClaims.
func ValidatePlayerClaims(tokenString string, cfg Config) (PlayerClaims, error) {
	var c playerJwtClaims
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
		return PlayerClaims{}, fmt.Errorf("invalid token: %w", err)
	}
	playerID, err := uuid.Parse(c.Subject)
	if err != nil {
		return PlayerClaims{}, fmt.Errorf("invalid player subject claim: %w", err)
	}
	var sessionID uuid.UUID
	if c.SessionID != "" {
		sessionID, err = uuid.Parse(c.SessionID)
		if err != nil {
			return PlayerClaims{}, fmt.Errorf("invalid session_id claim: %w", err)
		}
	}
	return PlayerClaims{
		PlayerID:  playerID,
		SessionID: sessionID,
		Role:      c.Role,
		Issuer:    c.Issuer,
	}, nil
}


