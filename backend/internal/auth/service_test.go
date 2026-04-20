package auth

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
)

// --- mock repositories ---

type mockAdminRepo struct {
	admin *Admin
	err   error
}

func (m *mockAdminRepo) FindByEmail(_ context.Context, _ string) (*Admin, error) {
	return m.admin, m.err
}
func (m *mockAdminRepo) FindByID(_ context.Context, _ uuid.UUID) (*Admin, error) {
	return m.admin, m.err
}

type mockTokenRepo struct {
	record    *RefreshToken
	createErr error
	findErr   error
	revokeErr error
}

func (m *mockTokenRepo) Create(_ context.Context, _ uuid.UUID, _ string, _ time.Time) error {
	return m.createErr
}
func (m *mockTokenRepo) FindByHash(_ context.Context, _ string) (*RefreshToken, error) {
	return m.record, m.findErr
}
func (m *mockTokenRepo) Revoke(_ context.Context, _ string) error {
	return m.revokeErr
}

// --- helpers ---

func hashPassword(t *testing.T, pw string) string {
	t.Helper()
	h, err := bcrypt.GenerateFromPassword([]byte(pw), bcrypt.MinCost)
	if err != nil {
		t.Fatalf("bcrypt: %v", err)
	}
	return string(h)
}

// --- Login tests ---

func TestService_Login_Success(t *testing.T) {
	adminID := uuid.New()
	admin := &Admin{ID: adminID, Email: "a@b.com", PasswordHash: hashPassword(t, "secret")}
	svc := NewService(&mockAdminRepo{admin: admin}, &mockTokenRepo{}, testCfg)

	at, exp, rt, err := svc.Login(context.Background(), "a@b.com", "secret")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if at == "" || rt == "" {
		t.Fatal("expected non-empty tokens")
	}
	if exp.Before(time.Now()) {
		t.Fatal("expected future expiry")
	}
}

func TestService_Login_WrongPassword(t *testing.T) {
	admin := &Admin{ID: uuid.New(), Email: "a@b.com", PasswordHash: hashPassword(t, "secret")}
	svc := NewService(&mockAdminRepo{admin: admin}, &mockTokenRepo{}, testCfg)

	_, _, _, err := svc.Login(context.Background(), "a@b.com", "wrong")
	if !errors.Is(err, ErrInvalidCredentials) {
		t.Fatalf("expected ErrInvalidCredentials, got %v", err)
	}
}

func TestService_Login_UnknownEmail(t *testing.T) {
	svc := NewService(&mockAdminRepo{admin: nil}, &mockTokenRepo{}, testCfg)

	_, _, _, err := svc.Login(context.Background(), "ghost@b.com", "pass")
	if !errors.Is(err, ErrInvalidCredentials) {
		t.Fatalf("expected ErrInvalidCredentials, got %v", err)
	}
}

func TestService_Login_RepoError(t *testing.T) {
	svc := NewService(&mockAdminRepo{err: errors.New("db down")}, &mockTokenRepo{}, testCfg)

	_, _, _, err := svc.Login(context.Background(), "a@b.com", "pass")
	if err == nil || errors.Is(err, ErrInvalidCredentials) {
		t.Fatalf("expected wrapped repo error, got %v", err)
	}
}

// --- Refresh tests ---

func TestService_Refresh_Success(t *testing.T) {
	adminID := uuid.New()
	admin := &Admin{ID: adminID, Email: "a@b.com", PasswordHash: hashPassword(t, "s")}

	rawRefresh, _, _ := IssueRefreshToken(adminID, testCfg)
	hash := HashToken(rawRefresh)
	record := &RefreshToken{
		ID:        uuid.New(),
		AdminID:   adminID,
		TokenHash: hash,
		ExpiresAt: time.Now().Add(7 * 24 * time.Hour),
	}

	svc := NewService(&mockAdminRepo{admin: admin}, &mockTokenRepo{record: record}, testCfg)

	at, exp, err := svc.Refresh(context.Background(), rawRefresh)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if at == "" {
		t.Fatal("expected access token")
	}
	if exp.Before(time.Now()) {
		t.Fatal("expected future expiry")
	}
}

func TestService_Refresh_ExpiredJWT(t *testing.T) {
	svc := NewService(&mockAdminRepo{}, &mockTokenRepo{}, testCfg)

	_, _, err := svc.Refresh(context.Background(), "not.a.valid.jwt")
	if !errors.Is(err, ErrInvalidRefreshToken) {
		t.Fatalf("expected ErrInvalidRefreshToken, got %v", err)
	}
}

func TestService_Refresh_RevokedRecord(t *testing.T) {
	adminID := uuid.New()
	rawRefresh, _, _ := IssueRefreshToken(adminID, testCfg)

	// Token record not found (revoked/expired in DB)
	svc := NewService(&mockAdminRepo{}, &mockTokenRepo{record: nil}, testCfg)

	_, _, err := svc.Refresh(context.Background(), rawRefresh)
	if !errors.Is(err, ErrInvalidRefreshToken) {
		t.Fatalf("expected ErrInvalidRefreshToken, got %v", err)
	}
}

func TestService_Refresh_WrongRole(t *testing.T) {
	adminID := uuid.New()
	// Issue an access token (role=admin) — not a refresh token
	accessToken, _, _ := IssueAccessToken(adminID, testCfg)

	svc := NewService(&mockAdminRepo{}, &mockTokenRepo{}, testCfg)
	_, _, err := svc.Refresh(context.Background(), accessToken)
	if !errors.Is(err, ErrInvalidRefreshToken) {
		t.Fatalf("expected ErrInvalidRefreshToken for access token, got %v", err)
	}
}

func TestService_Refresh_SoftDeletedAdmin(t *testing.T) {
	adminID := uuid.New()
	rawRefresh, _, _ := IssueRefreshToken(adminID, testCfg)
	hash := HashToken(rawRefresh)
	record := &RefreshToken{
		ID:        uuid.New(),
		AdminID:   adminID,
		TokenHash: hash,
		ExpiresAt: time.Now().Add(7 * 24 * time.Hour),
	}

	// Admin deleted (FindByID returns nil)
	svc := NewService(&mockAdminRepo{admin: nil}, &mockTokenRepo{record: record}, testCfg)

	_, _, err := svc.Refresh(context.Background(), rawRefresh)
	if !errors.Is(err, ErrInvalidRefreshToken) {
		t.Fatalf("expected ErrInvalidRefreshToken for deleted admin, got %v", err)
	}
}

// --- Logout tests ---

func TestService_Logout_Success(t *testing.T) {
	adminID := uuid.New()
	rawRefresh, _, _ := IssueRefreshToken(adminID, testCfg)

	svc := NewService(&mockAdminRepo{}, &mockTokenRepo{}, testCfg)
	if err := svc.Logout(context.Background(), rawRefresh); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestService_Logout_Idempotent(t *testing.T) {
	adminID := uuid.New()
	rawRefresh, _, _ := IssueRefreshToken(adminID, testCfg)

	// Revoke returns nil even if already revoked (Revoke is idempotent by SQL design)
	svc := NewService(&mockAdminRepo{}, &mockTokenRepo{revokeErr: nil}, testCfg)
	err1 := svc.Logout(context.Background(), rawRefresh)
	err2 := svc.Logout(context.Background(), rawRefresh)
	if err1 != nil || err2 != nil {
		t.Fatalf("expected no error on repeated logout, got %v / %v", err1, err2)
	}
}

func TestService_Logout_RepoError(t *testing.T) {
	adminID := uuid.New()
	rawRefresh, _, _ := IssueRefreshToken(adminID, testCfg)

	svc := NewService(&mockAdminRepo{}, &mockTokenRepo{revokeErr: errors.New("db down")}, testCfg)
	if err := svc.Logout(context.Background(), rawRefresh); err == nil {
		t.Fatal("expected error when repo fails")
	}
}
