package auth

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// AdminRepo is the interface for admin persistence operations.
type AdminRepo interface {
	FindByEmail(ctx context.Context, email string) (*Admin, error)
	FindByID(ctx context.Context, id uuid.UUID) (*Admin, error)
}

// RefreshTokenRepo is the interface for refresh token persistence operations.
type RefreshTokenRepo interface {
	Create(ctx context.Context, adminID uuid.UUID, tokenHash string, expiresAt time.Time) error
	FindByHash(ctx context.Context, hash string) (*RefreshToken, error)
	Revoke(ctx context.Context, hash string) error
}

// AdminRepository implements AdminRepo using a pgxpool.Pool.
type AdminRepository struct {
	pool *pgxpool.Pool
}

func NewAdminRepository(pool *pgxpool.Pool) *AdminRepository {
	return &AdminRepository{pool: pool}
}

func (r *AdminRepository) FindByEmail(ctx context.Context, email string) (*Admin, error) {
	const q = `
		SELECT id, email, password_hash, name, created_at, updated_at, deleted_at
		FROM admins
		WHERE email = $1 AND deleted_at IS NULL`

	var a Admin
	err := r.pool.QueryRow(ctx, q, email).Scan(
		&a.ID, &a.Email, &a.PasswordHash, &a.Name,
		&a.CreatedAt, &a.UpdatedAt, &a.DeletedAt,
	)
	if err == pgx.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("admin repo: find by email: %w", err)
	}
	return &a, nil
}

func (r *AdminRepository) FindByID(ctx context.Context, id uuid.UUID) (*Admin, error) {
	const q = `
		SELECT id, email, password_hash, name, created_at, updated_at, deleted_at
		FROM admins
		WHERE id = $1 AND deleted_at IS NULL`

	var a Admin
	err := r.pool.QueryRow(ctx, q, id).Scan(
		&a.ID, &a.Email, &a.PasswordHash, &a.Name,
		&a.CreatedAt, &a.UpdatedAt, &a.DeletedAt,
	)
	if err == pgx.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("admin repo: find by id: %w", err)
	}
	return &a, nil
}

// RefreshTokenRepository implements RefreshTokenRepo using a pgxpool.Pool.
type RefreshTokenRepository struct {
	pool *pgxpool.Pool
}

func NewRefreshTokenRepository(pool *pgxpool.Pool) *RefreshTokenRepository {
	return &RefreshTokenRepository{pool: pool}
}

func (r *RefreshTokenRepository) Create(ctx context.Context, adminID uuid.UUID, tokenHash string, expiresAt time.Time) error {
	const q = `
		INSERT INTO refresh_tokens (admin_id, token_hash, expires_at)
		VALUES ($1, $2, $3)`
	_, err := r.pool.Exec(ctx, q, adminID, tokenHash, expiresAt)
	if err != nil {
		return fmt.Errorf("refresh token repo: create: %w", err)
	}
	return nil
}

func (r *RefreshTokenRepository) FindByHash(ctx context.Context, hash string) (*RefreshToken, error) {
	const q = `
		SELECT id, admin_id, token_hash, expires_at, created_at, revoked_at
		FROM refresh_tokens
		WHERE token_hash = $1
		  AND revoked_at IS NULL
		  AND expires_at > now()`

	var t RefreshToken
	err := r.pool.QueryRow(ctx, q, hash).Scan(
		&t.ID, &t.AdminID, &t.TokenHash, &t.ExpiresAt, &t.CreatedAt, &t.RevokedAt,
	)
	if err == pgx.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("refresh token repo: find by hash: %w", err)
	}
	return &t, nil
}

func (r *RefreshTokenRepository) Revoke(ctx context.Context, hash string) error {
	const q = `
		UPDATE refresh_tokens
		SET revoked_at = now()
		WHERE token_hash = $1 AND revoked_at IS NULL`
	_, err := r.pool.Exec(ctx, q, hash)
	if err != nil {
		return fmt.Errorf("refresh token repo: revoke: %w", err)
	}
	return nil
}
