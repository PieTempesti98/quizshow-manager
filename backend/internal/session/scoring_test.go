package session_test

import (
	"testing"

	"github.com/PieTempesti98/quizshow/internal/session"
)

func TestScoreAnswer(t *testing.T) {
	tests := []struct {
		name              string
		pointsPerAnswer   int
		isCorrect         bool
		speedBonusEnabled bool
		timeRemainingMs   int64
		totalTimeMs       int64
		expectedPoints    int
	}{
		{
			name:              "Incorrect answer always awards 0 points (speed bonus off)",
			pointsPerAnswer:   100,
			isCorrect:         false,
			speedBonusEnabled: false,
			timeRemainingMs:   30000,
			totalTimeMs:       30000,
			expectedPoints:    0,
		},
		{
			name:              "Incorrect answer always awards 0 points (speed bonus on)",
			pointsPerAnswer:   100,
			isCorrect:         false,
			speedBonusEnabled: true,
			timeRemainingMs:   30000,
			totalTimeMs:       30000,
			expectedPoints:    0,
		},
		{
			name:              "Correct answer with speed bonus off awards flat base points",
			pointsPerAnswer:   100,
			isCorrect:         true,
			speedBonusEnabled: false,
			timeRemainingMs:   15000,
			totalTimeMs:       30000,
			expectedPoints:    100,
		},
		{
			name:              "Correct answer immediately (100% time left, speed bonus on) awards 1.5x",
			pointsPerAnswer:   100,
			isCorrect:         true,
			speedBonusEnabled: true,
			timeRemainingMs:   30000,
			totalTimeMs:       30000,
			expectedPoints:    150,
		},
		{
			name:              "Correct answer at exactly half timer (50% time left, speed bonus on) awards 1.25x",
			pointsPerAnswer:   100,
			isCorrect:         true,
			speedBonusEnabled: true,
			timeRemainingMs:   15000,
			totalTimeMs:       30000,
			expectedPoints:    125,
		},
		{
			name:              "Correct answer at last millisecond (0% time left, speed bonus on) awards 1.0x",
			pointsPerAnswer:   100,
			isCorrect:         true,
			speedBonusEnabled: true,
			timeRemainingMs:   0,
			totalTimeMs:       30000,
			expectedPoints:    100,
		},
		{
			name:              "Negative remaining time is clamped to 0",
			pointsPerAnswer:   100,
			isCorrect:         true,
			speedBonusEnabled: true,
			timeRemainingMs:   -500,
			totalTimeMs:       30000,
			expectedPoints:    100,
		},
		{
			name:              "Remaining time exceeding total is clamped to total",
			pointsPerAnswer:   100,
			isCorrect:         true,
			speedBonusEnabled: true,
			timeRemainingMs:   35000,
			totalTimeMs:       30000,
			expectedPoints:    150,
		},
		{
			name:              "Custom pointsPerAnswer (200 base) scales linearly",
			pointsPerAnswer:   200,
			isCorrect:         true,
			speedBonusEnabled: true,
			timeRemainingMs:   15000,
			totalTimeMs:       30000,
			expectedPoints:    250,
		},
		{
			name:              "Zero total time returns base points",
			pointsPerAnswer:   100,
			isCorrect:         true,
			speedBonusEnabled: true,
			timeRemainingMs:   0,
			totalTimeMs:       0,
			expectedPoints:    100,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := session.ScoreAnswer(
				tt.pointsPerAnswer,
				tt.isCorrect,
				tt.speedBonusEnabled,
				tt.timeRemainingMs,
				tt.totalTimeMs,
			)
			if got != tt.expectedPoints {
				t.Errorf("ScoreAnswer() = %d, want %d", got, tt.expectedPoints)
			}
		})
	}
}
