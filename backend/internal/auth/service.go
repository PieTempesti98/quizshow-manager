package auth

import (
	"context"
	"errors"
	"fmt"
	"time"

	"golang.org/x/crypto/bcrypt"
)

// ErrInvalidCredentials is returned when email/password don't match.
var ErrInvalidCredentials = errors.New("invalid credentials")

// ErrInvalidRefreshToken is returned when the refresh token is missing, expired, or revoked.
var ErrInvalidRefreshToken = errors.New("invalid or expired refresh token")

// Service defines the auth business operations.
type Service interface {
	Login(ctx context.Context, email, password string) (accessToken string, expiresAt time.Time, refreshToken string, err error)
	Refresh(ctx context.Context, rawRefreshJWT string) (accessToken string, expiresAt time.Time, err error)
	Logout(ctx context.Context, rawRefreshJWT string) error
}

type service struct {
	admins AdminRepo
	tokens RefreshTokenRepo
	cfg    Config
}

// NewService constructs an auth Service with the given repositories and config.
func NewService(admins AdminRepo, tokens RefreshTokenRepo, cfg Config) Service {
	return &service{admins: admins, tokens: tokens, cfg: cfg}
}

func (s *service) Login(ctx context.Context, email, password string) (string, time.Time, string, error) {
	admin, err := s.admins.FindByEmail(ctx, email)
	if err != nil {
		return "", time.Time{}, "", fmt.Errorf("login: %w", err)
	}
	if admin == nil {
		return "", time.Time{}, "", ErrInvalidCredentials
	}

	if err := bcrypt.CompareHashAndPassword([]byte(admin.PasswordHash), []byte(password)); err != nil {
		return "", time.Time{}, "", ErrInvalidCredentials
	}

	accessToken, expiresAt, err := IssueAccessToken(admin.ID, s.cfg)
	if err != nil {
		return "", time.Time{}, "", fmt.Errorf("login: %w", err)
	}

	refreshToken, refreshExp, err := IssueRefreshToken(admin.ID, s.cfg)
	if err != nil {
		return "", time.Time{}, "", fmt.Errorf("login: %w", err)
	}

	hash := HashToken(refreshToken)
	if err := s.tokens.Create(ctx, admin.ID, hash, refreshExp); err != nil {
		return "", time.Time{}, "", fmt.Errorf("login: %w", err)
	}

	return accessToken, expiresAt, refreshToken, nil
}

func (s *service) Refresh(ctx context.Context, rawRefreshJWT string) (string, time.Time, error) {
	claims, err := ValidateClaims(rawRefreshJWT, s.cfg)
	if err != nil {
		return "", time.Time{}, ErrInvalidRefreshToken
	}
	if claims.Role != "refresh" {
		return "", time.Time{}, ErrInvalidRefreshToken
	}

	hash := HashToken(rawRefreshJWT)
	record, err := s.tokens.FindByHash(ctx, hash)
	if err != nil {
		return "", time.Time{}, fmt.Errorf("refresh: %w", err)
	}
	if record == nil {
		return "", time.Time{}, ErrInvalidRefreshToken
	}

	// Verify admin is still active before issuing a new access token.
	admin, err := s.admins.FindByID(ctx, record.AdminID)
	if err != nil {
		return "", time.Time{}, fmt.Errorf("refresh: %w", err)
	}
	if admin == nil {
		return "", time.Time{}, ErrInvalidRefreshToken
	}

	accessToken, expiresAt, err := IssueAccessToken(admin.ID, s.cfg)
	if err != nil {
		return "", time.Time{}, fmt.Errorf("refresh: %w", err)
	}
	return accessToken, expiresAt, nil
}

func (s *service) Logout(ctx context.Context, rawRefreshJWT string) error {
	hash := HashToken(rawRefreshJWT)
	return s.tokens.Revoke(ctx, hash)
}
