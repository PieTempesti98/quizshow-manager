package auth

import (
	"errors"
	"os"
	"strconv"
	"time"
)

// Config holds all auth-related configuration, loaded once at startup.
type Config struct {
	JWTSecret    []byte
	JWTIssuer    string
	AccessTTL    time.Duration
	RefreshTTL   time.Duration
	CookieSecure bool
}

// LoadConfig reads auth configuration from environment variables.
// Returns an error if JWT_SECRET or JWT_ISSUER are absent.
func LoadConfig() (Config, error) {
	secret := os.Getenv("JWT_SECRET")
	if secret == "" {
		return Config{}, errors.New("JWT_SECRET must be set")
	}
	issuer := os.Getenv("JWT_ISSUER")
	if issuer == "" {
		return Config{}, errors.New("JWT_ISSUER must be set")
	}

	cookieSecure := true
	if v := os.Getenv("COOKIE_SECURE"); v != "" {
		if parsed, err := strconv.ParseBool(v); err == nil {
			cookieSecure = parsed
		}
	}

	return Config{
		JWTSecret:    []byte(secret),
		JWTIssuer:    issuer,
		AccessTTL:    15 * time.Minute,
		RefreshTTL:   7 * 24 * time.Hour,
		CookieSecure: cookieSecure,
	}, nil
}
