package session

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/PieTempesti98/quizshow/internal/api"
	"github.com/PieTempesti98/quizshow/internal/auth"
	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
)

// setupTestFiberApp sets up a Fiber app mimicking main.go router configuration for player routes.
func setupTestFiberApp(svc Service, authCfg auth.Config) *fiber.App {
	app := fiber.New(fiber.Config{
		ErrorHandler: func(c *fiber.Ctx, err error) error {
			code := fiber.StatusInternalServerError
			if e, ok := err.(*fiber.Error); ok {
				code = e.Code
			}
			return c.Status(code).JSON(api.ErrorResponse{
				Error: api.ErrorDetail{
					Code:    "INTERNAL_ERROR",
					Message: err.Error(),
				},
			})
		},
	})

	handler := NewHandler(svc)
	v1 := app.Group("/api/v1")

	// Public join route
	v1.Post("/sessions/:id/join", handler.Join)

	// Protected player route
	playerSession := v1.Group("/sessions/:session_id", auth.RequirePlayer(authCfg))
	playerSession.Post("/answers", handler.SubmitAnswer)

	return app
}

func TestHTTPSmoke_PlayerJoinAndAnswerFlow(t *testing.T) {
	sessionID := uuid.New()
	playerID := uuid.New()
	sqID := uuid.New()
	ansID := uuid.New()

	mockRepo := &mockSessionRepo{
		joinFn: func(ctx context.Context, sID uuid.UUID, pin string, nickname string, avatarColor string) (Player, Session, int, error) {
			if pin != "482910" {
				return Player{}, Session{}, 0, ErrInvalidPIN
			}
			if nickname == "TakenNick" {
				return Player{}, Session{}, 0, ErrNicknameTaken
			}
			return Player{
				ID:          playerID,
				SessionID:   sID,
				Nickname:    nickname,
				AvatarColor: avatarColor,
				TotalScore:  0,
				JoinedAt:    time.Now(),
			}, Session{
				ID:     sID,
				Name:   "Quiz Show Live",
				PIN:    pin,
				Status: "lobby",
			}, 1, nil
		},
		submitAnswerFn: func(ctx context.Context, sID, pID, qID uuid.UUID, chosenIndex int16, now time.Time) (Answer, bool, int, int, error) {
			if qID != sqID {
				return Answer{}, false, 0, 0, ErrQuestionNotFound
			}
			idx := chosenIndex
			ansTime := 1200
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
	app := setupTestFiberApp(svc, playerTestCfg)

	// Step 1: POST /api/v1/sessions/:id/join (Invalid PIN -> 404)
	joinBody, _ := json.Marshal(map[string]string{"pin": "000000", "nickname": "Mario"})
	req := httptest.NewRequest("POST", "/api/v1/sessions/"+sessionID.String()+"/join", bytes.NewReader(joinBody))
	req.Header.Set("Content-Type", "application/json")
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("app.Test: %v", err)
	}
	if resp.StatusCode != 404 {
		t.Errorf("expected 404 for invalid PIN, got %d", resp.StatusCode)
	}

	// Step 2: POST /api/v1/sessions/:id/join (Duplicate Nickname -> 409)
	joinBody, _ = json.Marshal(map[string]string{"pin": "482910", "nickname": "TakenNick"})
	req = httptest.NewRequest("POST", "/api/v1/sessions/"+sessionID.String()+"/join", bytes.NewReader(joinBody))
	req.Header.Set("Content-Type", "application/json")
	resp, _ = app.Test(req)
	if resp.StatusCode != 409 {
		t.Errorf("expected 409 for duplicate nickname, got %d", resp.StatusCode)
	}

	// Step 3: POST /api/v1/sessions/:id/join (Success -> 201)
	joinBody, _ = json.Marshal(map[string]string{"pin": "482910", "nickname": "Mario"})
	req = httptest.NewRequest("POST", "/api/v1/sessions/"+sessionID.String()+"/join", bytes.NewReader(joinBody))
	req.Header.Set("Content-Type", "application/json")
	resp, err = app.Test(req)
	if err != nil {
		t.Fatalf("app.Test: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 201 {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("expected 201 for join, got %d; body: %s", resp.StatusCode, body)
	}

	var joinRes struct {
		Data struct {
			PlayerID    string `json:"player_id"`
			Nickname    string `json:"nickname"`
			AvatarColor string `json:"avatar_color"`
			PlayerToken string `json:"player_token"`
		} `json:"data"`
	}
	bodyBytes, _ := io.ReadAll(resp.Body)
	if err := json.Unmarshal(bodyBytes, &joinRes); err != nil {
		t.Fatalf("unmarshal join response: %v", err)
	}
	playerToken := joinRes.Data.PlayerToken
	if playerToken == "" {
		t.Fatal("expected non-empty player_token")
	}

	// Step 4: POST /api/v1/sessions/:session_id/answers without auth -> 401
	ansBody, _ := json.Marshal(map[string]any{"session_question_id": sqID.String(), "chosen_index": 2})
	req = httptest.NewRequest("POST", "/api/v1/sessions/"+sessionID.String()+"/answers", bytes.NewReader(ansBody))
	req.Header.Set("Content-Type", "application/json")
	resp, _ = app.Test(req)
	if resp.StatusCode != 401 {
		t.Errorf("expected 401 for unauthenticated answer submission, got %d", resp.StatusCode)
	}

	// Step 5: POST /api/v1/sessions/:session_id/answers with token for different session -> 403
	otherSessionID := uuid.New()
	req = httptest.NewRequest("POST", "/api/v1/sessions/"+otherSessionID.String()+"/answers", bytes.NewReader(ansBody))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+playerToken)
	resp, _ = app.Test(req)
	if resp.StatusCode != 403 {
		t.Errorf("expected 403 for mismatched session ID, got %d", resp.StatusCode)
	}

	// Step 6: POST /api/v1/sessions/:session_id/answers with valid player token -> 201
	req = httptest.NewRequest("POST", "/api/v1/sessions/"+sessionID.String()+"/answers", bytes.NewReader(ansBody))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+playerToken)
	resp, err = app.Test(req)
	if err != nil {
		t.Fatalf("app.Test: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 201 {
		b, _ := io.ReadAll(resp.Body)
		t.Fatalf("expected 201 for answer submission, got %d; body: %s", resp.StatusCode, b)
	}

	// Step 7: POST /api/v1/sessions/:session_id/answers resubmission (idempotent duplicate) -> 200
	mockRepo.submitAnswerFn = func(ctx context.Context, sID, pID, qID uuid.UUID, chosenIndex int16, now time.Time) (Answer, bool, int, int, error) {
		idx := int16(2)
		ansTime := 1200
		return Answer{
			ID:                ansID,
			PlayerID:          pID,
			SessionQuestionID: qID,
			ChosenIndex:       &idx,
			AnswerTimeMs:      &ansTime,
			AnsweredAt:        now.Add(-3 * time.Second),
		}, true, 1, 1, nil
	}

	req = httptest.NewRequest("POST", "/api/v1/sessions/"+sessionID.String()+"/answers", bytes.NewReader(ansBody))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+playerToken)
	resp, err = app.Test(req)
	if err != nil {
		t.Fatalf("app.Test: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		b, _ := io.ReadAll(resp.Body)
		t.Fatalf("expected 200 for idempotent duplicate answer, got %d; body: %s", resp.StatusCode, b)
	}
}
