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
	ErrSessionNotFound       = errors.New("session not found")
	ErrSessionNotDraft       = errors.New("session is not in draft status")
	ErrSessionNotInLobby     = errors.New("session is not in lobby status")
	ErrInsufficientQuestions = errors.New("no questions available in the configured categories")
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

// SessionEventBroadcaster notifies connected clients of session state changes.
// NoopBroadcaster is injected until the WebSocket hub is implemented in feature #10.
type SessionEventBroadcaster interface {
	BroadcastSessionStarted(sessionID string, totalQuestions int)
}

type noopBroadcaster struct{}

func (noopBroadcaster) BroadcastSessionStarted(string, int) {}

// NewNoopBroadcaster returns a SessionEventBroadcaster that does nothing.
func NewNoopBroadcaster() SessionEventBroadcaster { return noopBroadcaster{} }
