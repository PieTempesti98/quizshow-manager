package session_test

import (
	"encoding/csv"
	"strings"
	"testing"
	"time"

	"github.com/PieTempesti98/quizshow/internal/session"
	"github.com/google/uuid"
)

func TestGenerateLeaderboardCSV(t *testing.T) {
	t.Run("empty leaderboard returns header only", func(t *testing.T) {
		data, err := session.GenerateLeaderboardCSV(nil)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		r := csv.NewReader(strings.NewReader(string(data)))
		records, err := r.ReadAll()
		if err != nil {
			t.Fatalf("failed to read csv: %v", err)
		}

		if len(records) != 1 {
			t.Fatalf("expected 1 row (header), got %d", len(records))
		}
		if records[0][0] != "rank" || records[0][1] != "nickname" || records[0][2] != "total_score" {
			t.Fatalf("unexpected header: %v", records[0])
		}
	})

	t.Run("valid rows with standard formatting", func(t *testing.T) {
		entries := []session.SessionLeaderboardEntry{
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
			{
				Rank:       3,
				PlayerID:   uuid.New(),
				Nickname:   "Charlie",
				TotalScore: 950,
			},
		}

		data, err := session.GenerateLeaderboardCSV(entries)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		r := csv.NewReader(strings.NewReader(string(data)))
		records, err := r.ReadAll()
		if err != nil {
			t.Fatalf("failed to read csv: %v", err)
		}

		if len(records) != 4 {
			t.Fatalf("expected 4 rows (1 header + 3 data), got %d", len(records))
		}

		expected := [][]string{
			{"rank", "nickname", "total_score"},
			{"1", "Alice", "1500"},
			{"2", "Bob", "1200"},
			{"3", "Charlie", "950"},
		}

		for i, expRow := range expected {
			for j, expVal := range expRow {
				if records[i][j] != expVal {
					t.Errorf("row %d col %d: expected %q, got %q", i, j, expVal, records[i][j])
				}
			}
		}
	})

	t.Run("handles special characters, commas, and quotes in nicknames", func(t *testing.T) {
		entries := []session.SessionLeaderboardEntry{
			{
				Rank:       1,
				PlayerID:   uuid.New(),
				Nickname:   `Mario, "The Boss"`,
				TotalScore: 2000,
			},
			{
				Rank:       2,
				PlayerID:   uuid.New(),
				Nickname:   "Luigi\nPlayer",
				TotalScore: 1000,
			},
		}

		data, err := session.GenerateLeaderboardCSV(entries)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		r := csv.NewReader(strings.NewReader(string(data)))
		records, err := r.ReadAll()
		if err != nil {
			t.Fatalf("failed to parse RFC 4180 CSV: %v", err)
		}

		if records[1][1] != `Mario, "The Boss"` {
			t.Errorf("expected escaped quotes/comma nickname to be preserved, got %q", records[1][1])
		}
		if records[2][1] != "Luigi\nPlayer" {
			t.Errorf("expected newline nickname to be preserved, got %q", records[2][1])
		}
	})
}

func TestFormatLeaderboardCSVFilename(t *testing.T) {
	ended := time.Date(2026, 8, 14, 15, 30, 0, 0, time.UTC)
	created := time.Date(2026, 8, 14, 14, 0, 0, 0, time.UTC)

	tests := []struct {
		name      string
		sessName  string
		endedAt   *time.Time
		createdAt time.Time
		expected  string
	}{
		{
			name:      "simple alphanumeric name",
			sessName:  "Quiz Aziendale Q2",
			endedAt:   &ended,
			createdAt: created,
			expected:  "quiz-aziendale-q2-2026-08-14-leaderboard.csv",
		},
		{
			name:      "special characters and accents",
			sessName:  `Quiz "Estate 2026" & Musica!`,
			endedAt:   &ended,
			createdAt: created,
			expected:  "quiz-estate-2026-musica-2026-08-14-leaderboard.csv",
		},
		{
			name:      "empty name defaults to session",
			sessName:  "   ---   ",
			endedAt:   &ended,
			createdAt: created,
			expected:  "session-2026-08-14-leaderboard.csv",
		},
		{
			name:      "nil endedAt uses createdAt date",
			sessName:  "Live Match",
			endedAt:   nil,
			createdAt: created,
			expected:  "live-match-2026-08-14-leaderboard.csv",
		},
		{
			name:      "long name truncated to 50 chars",
			sessName:  "A Very Long Quiz Show Name That Exceeds Fifty Characters Limit And Keeps Going",
			endedAt:   &ended,
			createdAt: created,
			expected:  "a-very-long-quiz-show-name-that-exceeds-fifty-char-2026-08-14-leaderboard.csv",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			actual := session.FormatLeaderboardCSVFilename(tc.sessName, tc.endedAt, tc.createdAt)
			if actual != tc.expected {
				t.Errorf("expected %q, got %q", tc.expected, actual)
			}
		})
	}
}
