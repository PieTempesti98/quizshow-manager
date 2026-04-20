package auth

import (
	"testing"
)

func TestLoadConfig_MissingJWTSecret(t *testing.T) {
	t.Setenv("JWT_SECRET", "")
	t.Setenv("JWT_ISSUER", "https://test.local")

	_, err := LoadConfig()
	if err == nil {
		t.Fatal("expected error when JWT_SECRET is absent")
	}
}

func TestLoadConfig_MissingJWTIssuer(t *testing.T) {
	t.Setenv("JWT_SECRET", "some-secret-that-is-long-enough-32c")
	t.Setenv("JWT_ISSUER", "")

	_, err := LoadConfig()
	if err == nil {
		t.Fatal("expected error when JWT_ISSUER is absent")
	}
}

func TestLoadConfig_Defaults(t *testing.T) {
	t.Setenv("JWT_SECRET", "some-secret-that-is-long-enough-32c")
	t.Setenv("JWT_ISSUER", "https://test.local")
	t.Setenv("COOKIE_SECURE", "")

	cfg, err := LoadConfig()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !cfg.CookieSecure {
		t.Error("expected CookieSecure to default to true")
	}
	if cfg.AccessTTL.Minutes() != 15 {
		t.Errorf("expected 15m AccessTTL, got %v", cfg.AccessTTL)
	}
	if cfg.RefreshTTL.Hours() != 168 {
		t.Errorf("expected 168h RefreshTTL, got %v", cfg.RefreshTTL)
	}
}

func TestLoadConfig_CookieSecureFalse(t *testing.T) {
	t.Setenv("JWT_SECRET", "some-secret-that-is-long-enough-32c")
	t.Setenv("JWT_ISSUER", "https://test.local")
	t.Setenv("COOKIE_SECURE", "false")

	cfg, err := LoadConfig()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.CookieSecure {
		t.Error("expected CookieSecure=false when COOKIE_SECURE=false")
	}
}
