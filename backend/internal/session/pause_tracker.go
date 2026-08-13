package session

import (
	"sync"
	"time"

	"github.com/google/uuid"
)

// PauseTracker manages in-memory pause timestamps for active sessions.
type PauseTracker struct {
	mu     sync.RWMutex
	pauses map[uuid.UUID]time.Time
}

// NewPauseTracker creates a new PauseTracker.
func NewPauseTracker() *PauseTracker {
	return &PauseTracker{
		pauses: make(map[uuid.UUID]time.Time),
	}
}

// Pause records that a session's question timer has been paused.
// Returns ErrTimerAlreadyPaused if the session is already in a paused state.
func (pt *PauseTracker) Pause(sessionID uuid.UUID, now time.Time) error {
	pt.mu.Lock()
	defer pt.mu.Unlock()

	if _, exists := pt.pauses[sessionID]; exists {
		return ErrTimerAlreadyPaused
	}

	pt.pauses[sessionID] = now
	return nil
}

// Resume removes the pause state and returns the elapsed pause duration.
// Returns ErrTimerNotPaused if the session was not paused.
func (pt *PauseTracker) Resume(sessionID uuid.UUID, now time.Time) (time.Duration, error) {
	pt.mu.Lock()
	defer pt.mu.Unlock()

	pausedAt, exists := pt.pauses[sessionID]
	if !exists {
		return 0, ErrTimerNotPaused
	}

	delete(pt.pauses, sessionID)
	duration := now.Sub(pausedAt)
	if duration < 0 {
		duration = 0
	}
	return duration, nil
}

// Clear removes any pause state for the given session (e.g. on reveal or end).
func (pt *PauseTracker) Clear(sessionID uuid.UUID) {
	pt.mu.Lock()
	defer pt.mu.Unlock()
	delete(pt.pauses, sessionID)
}

// IsPaused returns true if the session timer is currently paused.
func (pt *PauseTracker) IsPaused(sessionID uuid.UUID) bool {
	pt.mu.RLock()
	defer pt.mu.RUnlock()
	_, exists := pt.pauses[sessionID]
	return exists
}
