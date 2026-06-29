package auth

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Integration tests require a real PostgreSQL database.
// Set TEST_DATABASE_URL to run them; they are skipped otherwise.
//
// Example:
//   TEST_DATABASE_URL="postgres://quizshow:quizshow@localhost:5432/quizshow?sslmode=disable" \
//     go test ./internal/auth/... -run TestIntegration

func integrationPool(t *testing.T) *pgxpool.Pool {
	t.Helper()
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("TEST_DATABASE_URL not set — skipping integration test")
	}
	pool, err := pgxpool.New(context.Background(), dsn)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	if err := pool.Ping(context.Background()); err != nil {
		pool.Close()
		t.Fatalf("ping: %v", err)
	}
	t.Cleanup(pool.Close)
	return pool
}

func seedAdmin(t *testing.T, pool *pgxpool.Pool) *Admin {
	t.Helper()
	id := uuid.New()
	_, err := pool.Exec(context.Background(),
		`INSERT INTO admins (id, email, password_hash, name) VALUES ($1, $2, $3, $4)`,
		id, "test-"+id.String()+"@example.com", "$2a$12$placeholder", "Test Admin",
	)
	if err != nil {
		t.Fatalf("seed admin: %v", err)
	}
	t.Cleanup(func() {
		pool.Exec(context.Background(), `DELETE FROM admins WHERE id = $1`, id)
	})
	return &Admin{ID: id, Email: "test-" + id.String() + "@example.com"}
}

// --- AdminRepository ---

func TestIntegration_AdminRepository_FindByEmail_Found(t *testing.T) {
	pool := integrationPool(t)
	admin := seedAdmin(t, pool)
	repo := NewAdminRepository(pool)

	got, err := repo.FindByEmail(context.Background(), admin.Email)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got == nil {
		t.Fatal("expected admin, got nil")
	}
	if got.ID != admin.ID {
		t.Errorf("expected ID %v, got %v", admin.ID, got.ID)
	}
}

func TestIntegration_AdminRepository_FindByEmail_NotFound(t *testing.T) {
	pool := integrationPool(t)
	repo := NewAdminRepository(pool)

	got, err := repo.FindByEmail(context.Background(), "nobody@example.com")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != nil {
		t.Fatal("expected nil for unknown email")
	}
}

func TestIntegration_AdminRepository_FindByID(t *testing.T) {
	pool := integrationPool(t)
	admin := seedAdmin(t, pool)
	repo := NewAdminRepository(pool)

	got, err := repo.FindByID(context.Background(), admin.ID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got == nil {
		t.Fatal("expected admin, got nil")
	}
	if got.Email != admin.Email {
		t.Errorf("expected email %s, got %s", admin.Email, got.Email)
	}
}

// --- RefreshTokenRepository ---

func TestIntegration_RefreshTokenRepository_CreateAndFind(t *testing.T) {
	pool := integrationPool(t)
	admin := seedAdmin(t, pool)
	repo := NewRefreshTokenRepository(pool)

	hash := HashToken("test-refresh-jwt-" + uuid.NewString())
	exp := time.Now().Add(7 * 24 * time.Hour)

	if err := repo.Create(context.Background(), admin.ID, hash, exp); err != nil {
		t.Fatalf("create: %v", err)
	}
	t.Cleanup(func() {
		pool.Exec(context.Background(), `DELETE FROM refresh_tokens WHERE token_hash = $1`, hash)
	})

	got, err := repo.FindByHash(context.Background(), hash)
	if err != nil {
		t.Fatalf("find: %v", err)
	}
	if got == nil {
		t.Fatal("expected record, got nil")
	}
	if got.AdminID != admin.ID {
		t.Errorf("expected AdminID %v, got %v", admin.ID, got.AdminID)
	}
}

func TestIntegration_RefreshTokenRepository_FindByHash_NotFound(t *testing.T) {
	pool := integrationPool(t)
	repo := NewRefreshTokenRepository(pool)

	got, err := repo.FindByHash(context.Background(), "nonexistent-hash")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != nil {
		t.Fatal("expected nil for unknown hash")
	}
}

func TestIntegration_RefreshTokenRepository_Revoke(t *testing.T) {
	pool := integrationPool(t)
	admin := seedAdmin(t, pool)
	repo := NewRefreshTokenRepository(pool)

	hash := HashToken("revoke-test-" + uuid.NewString())
	exp := time.Now().Add(7 * 24 * time.Hour)

	if err := repo.Create(context.Background(), admin.ID, hash, exp); err != nil {
		t.Fatalf("create: %v", err)
	}
	t.Cleanup(func() {
		pool.Exec(context.Background(), `DELETE FROM refresh_tokens WHERE token_hash = $1`, hash)
	})

	// Should be findable before revocation
	before, _ := repo.FindByHash(context.Background(), hash)
	if before == nil {
		t.Fatal("expected record before revoke")
	}

	if err := repo.Revoke(context.Background(), hash); err != nil {
		t.Fatalf("revoke: %v", err)
	}

	// Should NOT be findable after revocation
	after, err := repo.FindByHash(context.Background(), hash)
	if err != nil {
		t.Fatalf("find after revoke: %v", err)
	}
	if after != nil {
		t.Fatal("expected nil after revoke — token should not be findable")
	}
}

func TestIntegration_RefreshTokenRepository_Revoke_Idempotent(t *testing.T) {
	pool := integrationPool(t)
	admin := seedAdmin(t, pool)
	repo := NewRefreshTokenRepository(pool)

	hash := HashToken("idempotent-" + uuid.NewString())
	exp := time.Now().Add(7 * 24 * time.Hour)

	if err := repo.Create(context.Background(), admin.ID, hash, exp); err != nil {
		t.Fatalf("create: %v", err)
	}
	t.Cleanup(func() {
		pool.Exec(context.Background(), `DELETE FROM refresh_tokens WHERE token_hash = $1`, hash)
	})

	if err := repo.Revoke(context.Background(), hash); err != nil {
		t.Fatalf("first revoke: %v", err)
	}
	if err := repo.Revoke(context.Background(), hash); err != nil {
		t.Fatalf("second revoke (should be idempotent): %v", err)
	}
}
