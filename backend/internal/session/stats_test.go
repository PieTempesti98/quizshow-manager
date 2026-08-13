package session

import (
	"context"
	"encoding/json"
	"io"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/PieTempesti98/quizshow/internal/api"
	"github.com/PieTempesti98/quizshow/internal/auth"
	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
)

type mockStatsRepo struct {
	SessionRepo
	getLeaderboardFn func(ctx context.Context, sessionID uuid.UUID) (Session, []SessionLeaderboardEntry, error)
	getStatsFn       func(ctx context.Context, sessionID uuid.UUID) (Session, []QuestionStatsItem, error)
}

func (m *mockStatsRepo) GetLeaderboard(ctx context.Context, sessionID uuid.UUID) (Session, []SessionLeaderboardEntry, error) {
	if m.getLeaderboardFn != nil {
		return m.getLeaderboardFn(ctx, sessionID)
	}
	return Session{}, nil, nil
}

func (m *mockStatsRepo) GetStats(ctx context.Context, sessionID uuid.UUID) (Session, []QuestionStatsItem, error) {
	if m.getStatsFn != nil {
		return m.getStatsFn(ctx, sessionID)
	}
	return Session{}, nil, nil
}

func setupStatsFiberApp(svc Service, authCfg auth.Config) *fiber.App {
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
	protected := v1.Group("", auth.RequireAdmin(authCfg))

	protected.Get("/sessions/:id/leaderboard", handler.GetLeaderboard)
	protected.Get("/sessions/:id/stats", handler.GetStats)

	return app
}

func TestService_GetLeaderboard(t *testing.T) {
	sessionID := uuid.New()
	ended := time.Date(2026, 8, 14, 18, 0, 0, 0, time.UTC)

	t.Run("success on completed session", func(t *testing.T) {
		mockRepo := &mockStatsRepo{
			getLeaderboardFn: func(ctx context.Context, sID uuid.UUID) (Session, []SessionLeaderboardEntry, error) {
				return Session{
					ID:            sID,
					Name:          "Quiz Aziendale Q2",
					Status:        "completed",
					QuestionCount: 10,
					EndedAt:       &ended,
				}, []SessionLeaderboardEntry{
					{
						Rank:           1,
						PlayerID:       uuid.New(),
						Nickname:       "Alice",
						TotalScore:     1500,
						AvatarColor:    "#E85D24",
						CorrectAnswers: 9,
						TotalQuestions: 10,
					},
					{
						Rank:           2,
						PlayerID:       uuid.New(),
						Nickname:       "Bob",
						TotalScore:     1200,
						AvatarColor:    "#3B8BD4",
						CorrectAnswers: 8,
						TotalQuestions: 10,
					},
				}, nil
			},
		}

		svc := NewService(mockRepo, playerTestCfg, "http://localhost:5173", NewNoopBroadcaster())
		res, err := svc.GetLeaderboard(context.Background(), sessionID)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if res.SessionID != sessionID.String() {
			t.Errorf("expected session_id %s, got %s", sessionID, res.SessionID)
		}
		if res.SessionName != "Quiz Aziendale Q2" {
			t.Errorf("expected session name %s, got %s", "Quiz Aziendale Q2", res.SessionName)
		}
		if len(res.Leaderboard) != 2 {
			t.Fatalf("expected 2 leaderboard entries, got %d", len(res.Leaderboard))
		}
		if res.Leaderboard[0].Rank != 1 || res.Leaderboard[0].Nickname != "Alice" {
			t.Errorf("expected top rank Alice, got %v", res.Leaderboard[0])
		}
	})

	t.Run("returns ErrSessionNotCompleted when session is not completed", func(t *testing.T) {
		mockRepo := &mockStatsRepo{
			getLeaderboardFn: func(ctx context.Context, sID uuid.UUID) (Session, []SessionLeaderboardEntry, error) {
				return Session{}, nil, ErrSessionNotCompleted
			},
		}

		svc := NewService(mockRepo, playerTestCfg, "http://localhost:5173", NewNoopBroadcaster())
		_, err := svc.GetLeaderboard(context.Background(), sessionID)
		if err != ErrSessionNotCompleted {
			t.Errorf("expected ErrSessionNotCompleted, got %v", err)
		}
	})

	t.Run("returns ErrSessionNotFound when session does not exist", func(t *testing.T) {
		mockRepo := &mockStatsRepo{
			getLeaderboardFn: func(ctx context.Context, sID uuid.UUID) (Session, []SessionLeaderboardEntry, error) {
				return Session{}, nil, ErrSessionNotFound
			},
		}

		svc := NewService(mockRepo, playerTestCfg, "http://localhost:5173", NewNoopBroadcaster())
		_, err := svc.GetLeaderboard(context.Background(), sessionID)
		if err != ErrSessionNotFound {
			t.Errorf("expected ErrSessionNotFound, got %v", err)
		}
	})
}

func TestService_GetLeaderboardCSV(t *testing.T) {
	sessionID := uuid.New()
	ended := time.Date(2026, 8, 14, 18, 0, 0, 0, time.UTC)

	mockRepo := &mockStatsRepo{
		getLeaderboardFn: func(ctx context.Context, sID uuid.UUID) (Session, []SessionLeaderboardEntry, error) {
			return Session{
				ID:            sID,
				Name:          "Quiz Aziendale Q2",
				Status:        "completed",
				QuestionCount: 10,
				EndedAt:       &ended,
			}, []SessionLeaderboardEntry{
				{
					Rank:       1,
					PlayerID:   uuid.New(),
					Nickname:   "Alice",
					TotalScore: 1500,
				},
				{
					Rank:       2,
					PlayerID:   uuid.New(),
					Nickname:   "Bob",
					TotalScore: 1200,
				},
			}, nil
		},
	}

	svc := NewService(mockRepo, playerTestCfg, "http://localhost:5173", NewNoopBroadcaster())
	filename, data, err := svc.GetLeaderboardCSV(context.Background(), sessionID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	expectedFilename := "quiz-aziendale-q2-2026-08-14-leaderboard.csv"
	if filename != expectedFilename {
		t.Errorf("expected filename %q, got %q", expectedFilename, filename)
	}

	csvStr := string(data)
	if !strings.Contains(csvStr, "rank,nickname,total_score") {
		t.Errorf("missing header in csv: %s", csvStr)
	}
	if !strings.Contains(csvStr, "1,Alice,1500") || !strings.Contains(csvStr, "2,Bob,1200") {
		t.Errorf("missing rows in csv: %s", csvStr)
	}
}

func TestService_GetStats(t *testing.T) {
	sessionID := uuid.New()

	t.Run("success on completed session with accurate counts", func(t *testing.T) {
		mockRepo := &mockStatsRepo{
			getStatsFn: func(ctx context.Context, sID uuid.UUID) (Session, []QuestionStatsItem, error) {
				return Session{
					ID:     sID,
					Name:   "Quiz Show",
					Status: "completed",
				}, []QuestionStatsItem{
					{
						Position:      1,
						Text:          "Capital of Italy?",
						Difficulty:    "easy",
						CorrectIndex:  0,
						CorrectCount:  8,
						WrongCount:    2,
						NoAnswerCount: 1,
						AnswerDistribution: []AnswerDistributionItem{
							{Index: 0, Count: 8, Percent: 80},
							{Index: 1, Count: 1, Percent: 10},
							{Index: 2, Count: 1, Percent: 10},
							{Index: 3, Count: 0, Percent: 0},
						},
						AvgAnswerTimeMs: 4500,
					},
				}, nil
			},
		}

		svc := NewService(mockRepo, playerTestCfg, "http://localhost:5173", NewNoopBroadcaster())
		res, err := svc.GetStats(context.Background(), sessionID)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if len(res.Questions) != 1 {
			t.Fatalf("expected 1 question stat, got %d", len(res.Questions))
		}

		q := res.Questions[0]
		if q.CorrectCount != 8 || q.WrongCount != 2 || q.NoAnswerCount != 1 {
			t.Errorf("unexpected counts: %+v", q)
		}
		if q.AvgAnswerTimeMs != 4500 {
			t.Errorf("expected avg time 4500, got %d", q.AvgAnswerTimeMs)
		}
	})

	t.Run("returns ErrSessionNotCompleted on active session", func(t *testing.T) {
		mockRepo := &mockStatsRepo{
			getStatsFn: func(ctx context.Context, sID uuid.UUID) (Session, []QuestionStatsItem, error) {
				return Session{}, nil, ErrSessionNotCompleted
			},
		}

		svc := NewService(mockRepo, playerTestCfg, "http://localhost:5173", NewNoopBroadcaster())
		_, err := svc.GetStats(context.Background(), sessionID)
		if err != ErrSessionNotCompleted {
			t.Errorf("expected ErrSessionNotCompleted, got %v", err)
		}
	})
}

func TestHTTP_LeaderboardAndStatsEndpoints(t *testing.T) {
	sessionID := uuid.New()
	adminToken, _, err := auth.IssueAccessToken(uuid.New(), playerTestCfg)
	if err != nil {
		t.Fatalf("failed to issue admin token: %v", err)
	}

	ended := time.Date(2026, 8, 14, 18, 0, 0, 0, time.UTC)
	mockRepo := &mockStatsRepo{
		getLeaderboardFn: func(ctx context.Context, sID uuid.UUID) (Session, []SessionLeaderboardEntry, error) {
			if sID != sessionID {
				return Session{}, nil, ErrSessionNotFound
			}
			return Session{
				ID:            sID,
				Name:          "Quiz Aziendale Q2",
				Status:        "completed",
				QuestionCount: 10,
				EndedAt:       &ended,
			}, []SessionLeaderboardEntry{
				{
					Rank:           1,
					PlayerID:       uuid.New(),
					Nickname:       "Alice",
					TotalScore:     1500,
					AvatarColor:    "#E85D24",
					CorrectAnswers: 9,
					TotalQuestions: 10,
				},
			}, nil
		},
		getStatsFn: func(ctx context.Context, sID uuid.UUID) (Session, []QuestionStatsItem, error) {
			if sID != sessionID {
				return Session{}, nil, ErrSessionNotFound
			}
			return Session{
				ID:     sID,
				Name:   "Quiz Aziendale Q2",
				Status: "completed",
			}, []QuestionStatsItem{
				{
					Position:      1,
					Text:          "Sample Question",
					Difficulty:    "medium",
					CorrectIndex:  2,
					CorrectCount:  5,
					WrongCount:    1,
					NoAnswerCount: 0,
					AnswerDistribution: []AnswerDistributionItem{
						{Index: 0, Count: 0, Percent: 0},
						{Index: 1, Count: 1, Percent: 17},
						{Index: 2, Count: 5, Percent: 83},
						{Index: 3, Count: 0, Percent: 0},
					},
					AvgAnswerTimeMs: 6200,
				},
			}, nil
		},
	}

	svc := NewService(mockRepo, playerTestCfg, "http://localhost:5173", NewNoopBroadcaster())
	app := setupStatsFiberApp(svc, playerTestCfg)

	// 1. GET /api/v1/sessions/:id/leaderboard without auth -> 401
	req := httptest.NewRequest("GET", "/api/v1/sessions/"+sessionID.String()+"/leaderboard", nil)
	resp, _ := app.Test(req)
	if resp.StatusCode != 401 {
		t.Errorf("expected 401 for unauthenticated request, got %d", resp.StatusCode)
	}

	// 2. GET /api/v1/sessions/:id/leaderboard with valid auth -> 200 JSON
	req = httptest.NewRequest("GET", "/api/v1/sessions/"+sessionID.String()+"/leaderboard", nil)
	req.Header.Set("Authorization", "Bearer "+adminToken)
	resp, err = app.Test(req)
	if err != nil {
		t.Fatalf("app.Test: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		b, _ := io.ReadAll(resp.Body)
		t.Fatalf("expected 200 for leaderboard JSON, got %d; body: %s", resp.StatusCode, b)
	}

	var jsonResp struct {
		Data struct {
			SessionID   string                    `json:"session_id"`
			SessionName string                    `json:"session_name"`
			Leaderboard []SessionLeaderboardEntry `json:"leaderboard"`
		} `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&jsonResp); err != nil {
		t.Fatalf("failed to decode json response: %v", err)
	}
	if jsonResp.Data.SessionName != "Quiz Aziendale Q2" || len(jsonResp.Data.Leaderboard) != 1 {
		t.Errorf("unexpected leaderboard json payload: %+v", jsonResp)
	}

	// 3. GET /api/v1/sessions/:id/leaderboard?format=csv -> 200 CSV
	req = httptest.NewRequest("GET", "/api/v1/sessions/"+sessionID.String()+"/leaderboard?format=csv", nil)
	req.Header.Set("Authorization", "Bearer "+adminToken)
	resp, err = app.Test(req)
	if err != nil {
		t.Fatalf("app.Test: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		t.Fatalf("expected 200 for leaderboard CSV, got %d", resp.StatusCode)
	}
	if !strings.Contains(resp.Header.Get("Content-Type"), "text/csv") {
		t.Errorf("expected Content-Type text/csv, got %s", resp.Header.Get("Content-Type"))
	}
	if !strings.Contains(resp.Header.Get("Content-Disposition"), "attachment; filename=") {
		t.Errorf("expected Content-Disposition header, got %s", resp.Header.Get("Content-Disposition"))
	}
	csvBody, _ := io.ReadAll(resp.Body)
	if !strings.Contains(string(csvBody), "1,Alice,1500") {
		t.Errorf("expected Alice in csv body, got %s", string(csvBody))
	}

	// 4. GET /api/v1/sessions/:id/stats with valid auth -> 200 JSON
	req = httptest.NewRequest("GET", "/api/v1/sessions/"+sessionID.String()+"/stats", nil)
	req.Header.Set("Authorization", "Bearer "+adminToken)
	resp, err = app.Test(req)
	if err != nil {
		t.Fatalf("app.Test: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		b, _ := io.ReadAll(resp.Body)
		t.Fatalf("expected 200 for stats JSON, got %d; body: %s", resp.StatusCode, b)
	}

	var statsResp struct {
		Data struct {
			SessionID string              `json:"session_id"`
			Questions []QuestionStatsItem `json:"questions"`
		} `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&statsResp); err != nil {
		t.Fatalf("failed to decode stats json response: %v", err)
	}
	if len(statsResp.Data.Questions) != 1 || statsResp.Data.Questions[0].CorrectCount != 5 {
		t.Errorf("unexpected stats json payload: %+v", statsResp)
	}

	// 5. GET /api/v1/sessions/:id/leaderboard with active session (ErrSessionNotCompleted) -> 409
	mockRepo.getLeaderboardFn = func(ctx context.Context, sID uuid.UUID) (Session, []SessionLeaderboardEntry, error) {
		return Session{}, nil, ErrSessionNotCompleted
	}
	req = httptest.NewRequest("GET", "/api/v1/sessions/"+sessionID.String()+"/leaderboard", nil)
	req.Header.Set("Authorization", "Bearer "+adminToken)
	resp, _ = app.Test(req)
	if resp.StatusCode != 409 {
		t.Errorf("expected 409 for non-completed session, got %d", resp.StatusCode)
	}

	// 6. GET /api/v1/sessions/:id/stats with non-existent session (ErrSessionNotFound) -> 404
	mockRepo.getStatsFn = func(ctx context.Context, sID uuid.UUID) (Session, []QuestionStatsItem, error) {
		return Session{}, nil, ErrSessionNotFound
	}
	req = httptest.NewRequest("GET", "/api/v1/sessions/"+uuid.New().String()+"/stats", nil)
	req.Header.Set("Authorization", "Bearer "+adminToken)
	resp, _ = app.Test(req)
	if resp.StatusCode != 404 {
		t.Errorf("expected 404 for non-existent session, got %d", resp.StatusCode)
	}
}
