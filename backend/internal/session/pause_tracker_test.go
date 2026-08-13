package session_test

import (
	"testing"
	"time"

	"github.com/PieTempesti98/quizshow/internal/session"
	"github.com/google/uuid"
)

func TestPauseTracker(t *testing.T) {
	pt := session.NewPauseTracker()
	sessionID := uuid.New()
	t0 := time.Date(2026, 8, 13, 20, 0, 0, 0, time.UTC)

	// Initially not paused
	if pt.IsPaused(sessionID) {
		t.Fatal("expected session not to be paused")
	}

	// Resume when not paused should return error
	_, err := pt.Resume(sessionID, t0)
	if err != session.ErrTimerNotPaused {
		t.Fatalf("expected ErrTimerNotPaused, got %v", err)
	}

	// Pause
	if err := pt.Pause(sessionID, t0); err != nil {
		t.Fatalf("unexpected error pausing: %v", err)
	}

	if !pt.IsPaused(sessionID) {
		t.Fatal("expected session to be paused")
	}

	// Pause again while already paused should return error
	if err := pt.Pause(sessionID, t0); err != session.ErrTimerAlreadyPaused {
		t.Fatalf("expected ErrTimerAlreadyPaused, got %v", err)
	}

	// Resume 15 seconds later
	t1 := t0.Add(15 * time.Second)
	duration, err := pt.Resume(sessionID, t1)
	if err != nil {
		t.Fatalf("unexpected error resuming: %v", err)
	}
	if duration != 15*time.Second {
		t.Fatalf("expected duration 15s, got %v", duration)
	}

	// After resume, no longer paused
	if pt.IsPaused(sessionID) {
		t.Fatal("expected session not to be paused after resume")
	}

	// Test Clear
	_ = pt.Pause(sessionID, t0)
	pt.Clear(sessionID)
	if pt.IsPaused(sessionID) {
		t.Fatal("expected session not to be paused after clear")
	}
}
