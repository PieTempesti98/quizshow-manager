package auth

import (
	"time"

	"github.com/google/uuid"
)

// Admin represents a row in the admins table.
type Admin struct {
	ID           uuid.UUID
	Email        string
	PasswordHash string
	Name         string
	CreatedAt    time.Time
	UpdatedAt    time.Time
	DeletedAt    *time.Time
}

// RefreshToken represents a row in the refresh_tokens table.
type RefreshToken struct {
	ID        uuid.UUID
	AdminID   uuid.UUID
	TokenHash string
	ExpiresAt time.Time
	CreatedAt time.Time
	RevokedAt *time.Time
}
