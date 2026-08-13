package session

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/PieTempesti98/quizshow/internal/auth"
	"github.com/google/uuid"
	qrcode "github.com/skip2/go-qrcode"
)

// Service defines the session business operations.
type Service interface {
	Create(ctx context.Context, s SessionCreate, adminID uuid.UUID) (Session, int, string, error)
	List(ctx context.Context, f SessionFilter) (SessionListResult, error)
	FindByID(ctx context.Context, id uuid.UUID) (SessionDetail, error)
	Update(ctx context.Context, id uuid.UUID, u SessionUpdate) (Session, int, string, error)
	Delete(ctx context.Context, id uuid.UUID) error
	OpenLobby(ctx context.Context, id uuid.UUID) (OpenLobbyResult, error)
	GetQR(ctx context.Context, id uuid.UUID) ([]byte, error)
	Launch(ctx context.Context, id uuid.UUID) (LaunchResult, error)
	NextQuestion(ctx context.Context, id uuid.UUID) (NextQuestionResult, error)
	PauseTimer(ctx context.Context, id uuid.UUID) (PauseTimerResult, error)
	ResumeTimer(ctx context.Context, id uuid.UUID) (ResumeTimerResult, error)
	Reveal(ctx context.Context, id uuid.UUID) (RevealResult, error)
	End(ctx context.Context, id uuid.UUID, reason string) (EndSessionResult, error)
}

type service struct {
	repo         SessionRepo
	authCfg      auth.Config
	playerURL    string // PLACEHOLDER: update PLAYER_APP_BASE_URL env var when player frontend is deployed
	broadcaster  SessionEventBroadcaster
	pauseTracker *PauseTracker
}

// NewService constructs a session Service.
func NewService(repo SessionRepo, authCfg auth.Config, playerURL string, broadcaster SessionEventBroadcaster) Service {
	return &service{
		repo:         repo,
		authCfg:      authCfg,
		playerURL:    playerURL,
		broadcaster:  broadcaster,
		pauseTracker: NewPauseTracker(),
	}
}

// buildURL joins a base URL (trailing slash stripped) with a path.
func buildURL(base, path string) string {
	return strings.TrimRight(base, "/") + path
}

var validTimePerQuestion = map[int16]bool{10: true, 20: true, 30: true, 60: true}

func validateCreate(s SessionCreate) error {
	if s.Name == "" {
		return fmt.Errorf("VALIDATION_ERROR: name is required")
	}
	if len(s.CategoryIDs) == 0 {
		return fmt.Errorf("VALIDATION_ERROR: category_ids must contain at least one category")
	}
	if s.QuestionCount < 1 || s.QuestionCount > 50 {
		return fmt.Errorf("VALIDATION_ERROR: question_count must be between 1 and 50")
	}
	if !validTimePerQuestion[s.TimePerQuestionS] {
		return fmt.Errorf("VALIDATION_ERROR: time_per_question_s must be one of 10, 20, 30, 60")
	}
	if s.PointsPerAnswer != nil && *s.PointsPerAnswer <= 0 {
		return fmt.Errorf("VALIDATION_ERROR: points_per_answer must be a positive integer")
	}
	return nil
}

func validateUpdate(u SessionUpdate) error {
	if u.Name != nil && *u.Name == "" {
		return fmt.Errorf("VALIDATION_ERROR: name must not be empty")
	}
	if u.CategoryIDs != nil && len(u.CategoryIDs) == 0 {
		return fmt.Errorf("VALIDATION_ERROR: category_ids must contain at least one category")
	}
	if u.QuestionCount != nil && (*u.QuestionCount < 1 || *u.QuestionCount > 50) {
		return fmt.Errorf("VALIDATION_ERROR: question_count must be between 1 and 50")
	}
	if u.TimePerQuestionS != nil && !validTimePerQuestion[*u.TimePerQuestionS] {
		return fmt.Errorf("VALIDATION_ERROR: time_per_question_s must be one of 10, 20, 30, 60")
	}
	if u.PointsPerAnswer != nil && *u.PointsPerAnswer <= 0 {
		return fmt.Errorf("VALIDATION_ERROR: points_per_answer must be a positive integer")
	}
	return nil
}

func warningMessage(available int, requested int16) string {
	if available < int(requested) {
		return fmt.Sprintf("Only %d questions available, session will use all of them", available)
	}
	return ""
}

func (s *service) Create(ctx context.Context, sc SessionCreate, adminID uuid.UUID) (Session, int, string, error) {
	if err := validateCreate(sc); err != nil {
		return Session{}, 0, "", err
	}
	if err := s.repo.ValidateCategoryIDs(ctx, sc.CategoryIDs); err != nil {
		return Session{}, 0, "", err
	}

	sess, available, err := s.repo.Create(ctx, sc, adminID)
	if err != nil {
		return Session{}, 0, "", fmt.Errorf("session service: create: %w", err)
	}

	warning := warningMessage(available, sess.QuestionCount)
	return sess, available, warning, nil
}

func (s *service) List(ctx context.Context, f SessionFilter) (SessionListResult, error) {
	if f.Page < 1 {
		f.Page = 1
	}
	if f.PerPage < 1 {
		f.PerPage = 20
	}
	return s.repo.List(ctx, f)
}

func (s *service) FindByID(ctx context.Context, id uuid.UUID) (SessionDetail, error) {
	return s.repo.FindByID(ctx, id)
}

func (s *service) Update(ctx context.Context, id uuid.UUID, u SessionUpdate) (Session, int, string, error) {
	if err := validateUpdate(u); err != nil {
		return Session{}, 0, "", err
	}

	detail, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return Session{}, 0, "", err
	}
	if detail.Status != "draft" {
		return Session{}, 0, "", ErrSessionNotDraft
	}

	if u.CategoryIDs != nil {
		if err := s.repo.ValidateCategoryIDs(ctx, u.CategoryIDs); err != nil {
			return Session{}, 0, "", err
		}
	}

	sess, err := s.repo.Update(ctx, id, u)
	if err != nil {
		return Session{}, 0, "", fmt.Errorf("session service: update: %w", err)
	}

	available, err := s.repo.CountAvailableQuestions(ctx, sess.ID)
	if err != nil {
		return Session{}, 0, "", err
	}

	warning := warningMessage(available, sess.QuestionCount)
	return sess, available, warning, nil
}

func (s *service) Delete(ctx context.Context, id uuid.UUID) error {
	return s.repo.Delete(ctx, id)
}

func (s *service) OpenLobby(ctx context.Context, id uuid.UUID) (OpenLobbyResult, error) {
	sess, err := s.repo.OpenLobby(ctx, id)
	if err != nil {
		return OpenLobbyResult{}, err
	}
	return OpenLobbyResult{
		SessionID: sess.ID.String(),
		PIN:       sess.PIN,
		QRCodeURL: "/api/v1/sessions/" + sess.ID.String() + "/qr",
		Status:    sess.Status,
	}, nil
}

func (s *service) GetQR(ctx context.Context, id uuid.UUID) ([]byte, error) {
	detail, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}
	url := buildURL(s.playerURL, "/join?pin="+detail.PIN)
	png, err := qrcode.Encode(url, qrcode.Medium, 256)
	if err != nil {
		return nil, fmt.Errorf("session service: generate qr: %w", err)
	}
	return png, nil
}

func (s *service) Launch(ctx context.Context, id uuid.UUID) (LaunchResult, error) {
	sess, drawn, err := s.repo.Launch(ctx, id)
	if err != nil {
		return LaunchResult{}, err
	}
	token, err := auth.IssueProjectionToken(sess.ID, s.authCfg)
	if err != nil {
		return LaunchResult{}, fmt.Errorf("session service: issue projection token: %w", err)
	}
	projURL := buildURL(s.playerURL, "/projection?session="+sess.ID.String()+"&token="+token)
	s.broadcaster.BroadcastSessionStarted(sess.ID.String(), drawn)
	return LaunchResult{
		SessionID:       sess.ID.String(),
		Status:          sess.Status,
		QuestionCount:   drawn,
		ProjectionToken: token,
		ProjectionURL:   projURL,
		StartedAt:       *sess.StartedAt,
	}, nil
}

func (s *service) NextQuestion(ctx context.Context, id uuid.UUID) (NextQuestionResult, error) {
	data, err := s.repo.NextQuestion(ctx, id)
	if err != nil {
		return NextQuestionResult{}, err
	}

	s.broadcaster.BroadcastQuestionStarted(
		id.String(),
		data.SessionQuestionID.String(),
		data.Position,
		data.Total,
		data.Question,
		data.TimeLimitS,
		data.AskedAt,
	)

	return NextQuestionResult{
		SessionQuestionID: data.SessionQuestionID.String(),
		Position:          data.Position,
		Total:             data.Total,
		Question:          data.Question,
		TimeLimitS:        data.TimeLimitS,
		AskedAt:           data.AskedAt,
	}, nil
}

func (s *service) PauseTimer(ctx context.Context, id uuid.UUID) (PauseTimerResult, error) {
	active, err := s.repo.GetActiveQuestion(ctx, id)
	if err != nil {
		return PauseTimerResult{}, err
	}

	now := time.Now().UTC()
	if err := s.pauseTracker.Pause(id, now); err != nil {
		return PauseTimerResult{}, err
	}

	totalMs := int64(active.TimeLimitS) * 1000
	elapsedMs := now.Sub(active.AskedAt).Milliseconds()
	timeRemainingMs := totalMs - elapsedMs
	if timeRemainingMs < 0 {
		timeRemainingMs = 0
	}

	s.broadcaster.BroadcastTimerPaused(id.String(), now, timeRemainingMs)

	return PauseTimerResult{
		Ok:       true,
		PausedAt: now,
	}, nil
}

func (s *service) ResumeTimer(ctx context.Context, id uuid.UUID) (ResumeTimerResult, error) {
	active, err := s.repo.GetActiveQuestion(ctx, id)
	if err != nil {
		return ResumeTimerResult{}, err
	}

	now := time.Now().UTC()
	delta, err := s.pauseTracker.Resume(id, now)
	if err != nil {
		return ResumeTimerResult{}, err
	}

	if err := s.repo.ShiftQuestionAskedAt(ctx, active.SessionQuestionID, delta); err != nil {
		return ResumeTimerResult{}, err
	}

	adjustedAskedAt := active.AskedAt.Add(delta)
	totalMs := int64(active.TimeLimitS) * 1000
	elapsedMs := now.Sub(adjustedAskedAt).Milliseconds()
	timeRemainingMs := totalMs - elapsedMs
	if timeRemainingMs < 0 {
		timeRemainingMs = 0
	}

	s.broadcaster.BroadcastTimerResumed(id.String(), now, timeRemainingMs)

	return ResumeTimerResult{
		Ok:        true,
		ResumedAt: now,
	}, nil
}

func (s *service) Reveal(ctx context.Context, id uuid.UUID) (RevealResult, error) {
	s.pauseTracker.Clear(id)

	data, err := s.repo.RevealQuestion(ctx, id)
	if err != nil {
		return RevealResult{}, err
	}

	s.broadcaster.BroadcastQuestionRevealed(
		id.String(),
		data.SessionQuestionID.String(),
		data.CorrectIndex,
		data.AnswerDistribution,
		data.Top5,
	)

	return RevealResult{
		SessionQuestionID:  data.SessionQuestionID.String(),
		CorrectIndex:       data.CorrectIndex,
		AnswerDistribution: data.AnswerDistribution,
		Top5:               data.Top5,
		RevealedAt:         data.RevealedAt,
	}, nil
}

func (s *service) End(ctx context.Context, id uuid.UUID, reason string) (EndSessionResult, error) {
	if reason == "" {
		reason = "completed"
	}

	s.pauseTracker.Clear(id)

	sess, err := s.repo.EndSession(ctx, id, reason)
	if err != nil {
		return EndSessionResult{}, err
	}

	s.broadcaster.BroadcastSessionEnded(id.String(), reason)

	return EndSessionResult{
		SessionID: sess.ID.String(),
		Status:    sess.Status,
		EndedAt:   *sess.EndedAt,
	}, nil
}
