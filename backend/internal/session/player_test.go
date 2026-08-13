package session

import (
	"context"
	"testing"
	"time"

	"github.com/PieTempesti98/quizshow/internal/auth"
	"github.com/google/uuid"
)

var playerTestCfg = auth.Config{
	JWTSecret:    []byte("test-secret-that-is-long-enough-32c"),
	JWTIssuer:    "https://test.local",
	AccessTTL:    15 * time.Minute,
	RefreshTTL:   7 * 24 * time.Hour,
	CookieSecure: false,
}

func TestAssignAvatarColor(t *testing.T) {
	if len(AvatarPalette) == 0 {
		t.Fatal("AvatarPalette should not be empty")
	}

	// Empty string fallback
	c1 := AssignAvatarColor("")
	if c1 != AvatarPalette[0] {
		t.Errorf("expected %s for empty string, got %s", AvatarPalette[0], c1)
	}

	// Determinism
	c2 := AssignAvatarColor("Mario")
	c3 := AssignAvatarColor("mario")
	if c2 != c3 {
		t.Errorf("AssignAvatarColor should be case-insensitive: got %s vs %s", c2, c3)
	}

	// Format check
	if len(c2) != 7 || c2[0] != '#' {
		t.Errorf("expected hex color like #RRGGBB, got %s", c2)
	}
}

type mockSessionRepo struct {
	SessionRepo
	joinFn         func(ctx context.Context, sessionID uuid.UUID, pin string, nickname string, avatarColor string) (Player, Session, int, error)
	submitAnswerFn func(ctx context.Context, sessionID uuid.UUID, playerID uuid.UUID, sessionQuestionID uuid.UUID, chosenIndex int16, now time.Time) (Answer, bool, int, int, error)
}

func (m *mockSessionRepo) JoinPlayer(ctx context.Context, sessionID uuid.UUID, pin string, nickname string, avatarColor string) (Player, Session, int, error) {
	if m.joinFn != nil {
		return m.joinFn(ctx, sessionID, pin, nickname, avatarColor)
	}
	return Player{}, Session{}, 0, nil
}

func (m *mockSessionRepo) SubmitAnswer(ctx context.Context, sessionID uuid.UUID, playerID uuid.UUID, sessionQuestionID uuid.UUID, chosenIndex int16, now time.Time) (Answer, bool, int, int, error) {
	if m.submitAnswerFn != nil {
		return m.submitAnswerFn(ctx, sessionID, playerID, sessionQuestionID, chosenIndex, now)
	}
	return Answer{}, false, 0, 0, nil
}

func TestService_Join_Validation(t *testing.T) {
	svc := NewService(&mockSessionRepo{}, playerTestCfg, "http://localhost:5173", NewNoopBroadcaster())
	sessionID := uuid.New()

	tests := []struct {
		name     string
		pin      string
		nickname string
		wantErr  string
	}{
		{"invalid pin length", "12345", "Mario", "pin must be 6 digits"},
		{"invalid pin chars", "12345a", "Mario", "pin must be numeric"},
		{"nickname too short", "123456", "M", "nickname must be between 2 and 20 characters"},
		{"nickname too long", "123456", "SuperMarioBrotherWithVeryLongName", "nickname must be between 2 and 20 characters"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := svc.Join(context.Background(), sessionID, tt.pin, tt.nickname)
			if err == nil {
				t.Fatalf("expected error containing %q, got nil", tt.wantErr)
			}
			if !isValidationError(err) {
				t.Errorf("expected validation error, got %v", err)
			}
		})
	}
}

func TestService_Join_Success(t *testing.T) {
	sessionID := uuid.New()
	playerID := uuid.New()

	mockRepo := &mockSessionRepo{
		joinFn: func(ctx context.Context, sessID uuid.UUID, pin string, nickname string, avatarColor string) (Player, Session, int, error) {
			return Player{
					ID:          playerID,
					SessionID:   sessID,
					Nickname:    nickname,
					AvatarColor: avatarColor,
					TotalScore:  0,
					JoinedAt:    time.Now(),
				}, Session{
					ID:     sessID,
					Name:   "Test Quiz",
					PIN:    pin,
					Status: "lobby",
				}, 1, nil
		},
	}

	svc := NewService(mockRepo, playerTestCfg, "http://localhost:5173", NewNoopBroadcaster())
	res, err := svc.Join(context.Background(), sessionID, "123456", "Mario")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if res.PlayerID != playerID.String() {
		t.Errorf("expected player_id %s, got %s", playerID, res.PlayerID)
	}
	if res.Nickname != "Mario" {
		t.Errorf("expected nickname Mario, got %s", res.Nickname)
	}
	if res.PlayerToken == "" {
		t.Fatal("expected non-empty player token")
	}

	// Verify claims in returned token
	claims, err := auth.ValidatePlayerClaims(res.PlayerToken, playerTestCfg)
	if err != nil {
		t.Fatalf("validate player token failed: %v", err)
	}
	if claims.PlayerID != playerID || claims.SessionID != sessionID {
		t.Errorf("claims mismatch: player %s, session %s", claims.PlayerID, claims.SessionID)
	}
}

func TestService_SubmitAnswer_ValidationAndIdempotency(t *testing.T) {
	sessionID := uuid.New()
	playerID := uuid.New()
	sqID := uuid.New()
	ansID := uuid.New()

	mockRepo := &mockSessionRepo{
		submitAnswerFn: func(ctx context.Context, sID, pID, qID uuid.UUID, chosenIndex int16, now time.Time) (Answer, bool, int, int, error) {
			idx := chosenIndex
			ansTime := 1500
			return Answer{
				ID:                ansID,
				PlayerID:          pID,
				SessionQuestionID: qID,
				ChosenIndex:       &idx,
				AnswerTimeMs:      &ansTime,
				AnsweredAt:        now,
			}, false, 1, 1, nil
		},
	}

	svc := NewService(mockRepo, playerTestCfg, "http://localhost:5173", NewNoopBroadcaster())

	// 1. Invalid index
	_, err := svc.SubmitAnswer(context.Background(), sessionID, playerID, sqID, 4)
	if err == nil {
		t.Fatal("expected validation error for chosen_index 4")
	}

	// 2. Valid first answer
	res, err := svc.SubmitAnswer(context.Background(), sessionID, playerID, sqID, 2)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.AnswerID != ansID.String() {
		t.Errorf("expected answer_id %s, got %s", ansID, res.AnswerID)
	}
	if res.ChosenIndex != 2 {
		t.Errorf("expected chosen_index 2, got %d", res.ChosenIndex)
	}
	if res.IsDuplicate {
		t.Error("expected IsDuplicate=false for first answer")
	}

	// 3. Idempotent repeat
	mockRepo.submitAnswerFn = func(ctx context.Context, sID, pID, qID uuid.UUID, chosenIndex int16, now time.Time) (Answer, bool, int, int, error) {
		idx := int16(2)
		ansTime := 1500
		return Answer{
			ID:                ansID,
			PlayerID:          pID,
			SessionQuestionID: qID,
			ChosenIndex:       &idx,
			AnswerTimeMs:      &ansTime,
			AnsweredAt:        now.Add(-2 * time.Second),
		}, true, 1, 1, nil
	}

	res2, err := svc.SubmitAnswer(context.Background(), sessionID, playerID, sqID, 2)
	if err != nil {
		t.Fatalf("unexpected error on idempotent retry: %v", err)
	}
	if !res2.IsDuplicate {
		t.Error("expected IsDuplicate=true on resubmission")
	}
	if res2.AnswerID != ansID.String() {
		t.Errorf("expected answer_id %s, got %s", ansID, res2.AnswerID)
	}
}
