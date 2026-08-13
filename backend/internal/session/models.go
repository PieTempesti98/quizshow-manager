package session

import (
	"errors"
	"time"

	"github.com/google/uuid"
)

type Session struct {
	ID                uuid.UUID
	Name              string
	PIN               string
	Status            string
	QuestionCount     int16
	TimePerQuestionS  int16
	PointsPerAnswer   int
	SpeedBonusEnabled bool
	StartedAt         *time.Time
	EndedAt           *time.Time
	CreatedBy         *uuid.UUID
	CreatedAt         time.Time
	UpdatedAt         time.Time
	DeletedAt         *time.Time
}

type CategoryRef struct {
	ID   uuid.UUID
	Name string
}

type SessionFilter struct {
	Statuses []string
	Page     int
	PerPage  int
}

type SessionListItem struct {
	Session
	PlayerCount int
}

type SessionListResult struct {
	Sessions   []SessionListItem
	Total      int
	Page       int
	PerPage    int
	TotalPages int
}

type SessionDetail struct {
	Session
	Categories  []CategoryRef
	PlayerCount int
}

type SessionCreate struct {
	Name              string
	CategoryIDs       []uuid.UUID
	QuestionCount     int16
	TimePerQuestionS  int16
	PointsPerAnswer   *int
	SpeedBonusEnabled *bool
}

type SessionUpdate struct {
	Name              *string
	CategoryIDs       []uuid.UUID
	QuestionCount     *int16
	TimePerQuestionS  *int16
	PointsPerAnswer   *int
	SpeedBonusEnabled *bool
}

var (
	ErrSessionNotFound         = errors.New("session not found")
	ErrSessionNotDraft         = errors.New("session is not in draft status")
	ErrSessionNotInLobby       = errors.New("session is not in lobby status")
	ErrInsufficientQuestions   = errors.New("no questions available in the configured categories")
	ErrSessionNotActive        = errors.New("session is not in active status")
	ErrQuestionNotRevealed     = errors.New("current question must be revealed before advancing")
	ErrNoMoreQuestions         = errors.New("all questions have been asked")
	ErrNoActiveQuestion        = errors.New("no question currently in progress")
	ErrQuestionAlreadyRevealed = errors.New("question has already been revealed")
	ErrTimerAlreadyPaused      = errors.New("timer is already paused")
	ErrTimerNotPaused          = errors.New("timer is not paused")
	ErrSessionAlreadyEnded     = errors.New("session has already ended")
)

// OpenLobbyResult is returned by Service.OpenLobby.
type OpenLobbyResult struct {
	SessionID string
	PIN       string
	QRCodeURL string
	Status    string
}

// LaunchResult is returned by Service.Launch.
type LaunchResult struct {
	SessionID       string
	Status          string
	QuestionCount   int
	ProjectionToken string
	ProjectionURL   string
	StartedAt       time.Time
}

// QuestionSummary carries question options without revealing correct answer.
type QuestionSummary struct {
	Text    string `json:"text"`
	OptionA string `json:"option_a"`
	OptionB string `json:"option_b"`
	OptionC string `json:"option_c"`
	OptionD string `json:"option_d"`
}

// NextQuestionResult is returned by Service.NextQuestion.
type NextQuestionResult struct {
	SessionQuestionID string
	Position          int
	Total             int
	Question          QuestionSummary
	TimeLimitS        int
	AskedAt           time.Time
}

// PauseTimerResult is returned by Service.PauseTimer.
type PauseTimerResult struct {
	Ok       bool
	PausedAt time.Time
}

// ResumeTimerResult is returned by Service.ResumeTimer.
type ResumeTimerResult struct {
	Ok        bool
	ResumedAt time.Time
}

// AnswerDistributionItem represents summary statistics for an answer option.
type AnswerDistributionItem struct {
	Index   int `json:"index"`
	Count   int `json:"count"`
	Percent int `json:"percent"`
}

// LeaderboardEntry represents a player's rank and score on leaderboard displays.
type LeaderboardEntry struct {
	Rank        int    `json:"rank"`
	Nickname    string `json:"nickname"`
	TotalScore  int    `json:"total_score"`
	AvatarColor string `json:"avatar_color"`
}

// RevealResult is returned by Service.Reveal.
type RevealResult struct {
	SessionQuestionID  string
	CorrectIndex       int
	AnswerDistribution []AnswerDistributionItem
	Top5               []LeaderboardEntry
	RevealedAt         time.Time
}

// EndSessionResult is returned by Service.End.
type EndSessionResult struct {
	SessionID string
	Status    string
	EndedAt   time.Time
}

// SessionEventBroadcaster notifies connected clients of session state changes.
// NoopBroadcaster is injected until the WebSocket hub is implemented in feature #10.
type SessionEventBroadcaster interface {
	BroadcastSessionStarted(sessionID string, totalQuestions int)
	BroadcastQuestionStarted(sessionID string, sessionQuestionID string, position int, total int, question QuestionSummary, timeLimitS int, askedAt time.Time)
	BroadcastTimerPaused(sessionID string, pausedAt time.Time, timeRemainingMs int64)
	BroadcastTimerResumed(sessionID string, resumedAt time.Time, timeRemainingMs int64)
	BroadcastQuestionRevealed(sessionID string, sessionQuestionID string, correctIndex int, distribution []AnswerDistributionItem, top5 []LeaderboardEntry)
	BroadcastSessionEnded(sessionID string, reason string)
}

type noopBroadcaster struct{}

func (noopBroadcaster) BroadcastSessionStarted(string, int) {}
func (noopBroadcaster) BroadcastQuestionStarted(string, string, int, int, QuestionSummary, int, time.Time) {
}
func (noopBroadcaster) BroadcastTimerPaused(string, time.Time, int64)  {}
func (noopBroadcaster) BroadcastTimerResumed(string, time.Time, int64) {}
func (noopBroadcaster) BroadcastQuestionRevealed(string, string, int, []AnswerDistributionItem, []LeaderboardEntry) {
}
func (noopBroadcaster) BroadcastSessionEnded(string, string) {}

// NewNoopBroadcaster returns a SessionEventBroadcaster that does nothing.
func NewNoopBroadcaster() SessionEventBroadcaster { return noopBroadcaster{} }
