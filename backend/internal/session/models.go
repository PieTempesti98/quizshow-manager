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
	ErrSessionNotFound = errors.New("session not found")
	ErrSessionNotDraft = errors.New("session is not in draft status")
)
