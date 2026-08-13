package session

import (
	"bytes"
	"encoding/csv"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"
)

var (
	nonAlphanumericRegex = regexp.MustCompile(`[^a-z0-9]+`)
	multipleHyphensRegex = regexp.MustCompile(`-+`)
)

// GenerateLeaderboardCSV serializes leaderboard standings into RFC 4180 compliant CSV bytes.
func GenerateLeaderboardCSV(entries []SessionLeaderboardEntry) ([]byte, error) {
	var buf bytes.Buffer
	w := csv.NewWriter(&buf)

	// Write header line
	if err := w.Write([]string{"rank", "nickname", "total_score"}); err != nil {
		return nil, fmt.Errorf("csv write header: %w", err)
	}

	// Write participant rows
	for _, entry := range entries {
		row := []string{
			strconv.Itoa(entry.Rank),
			entry.Nickname,
			strconv.Itoa(entry.TotalScore),
		}
		if err := w.Write(row); err != nil {
			return nil, fmt.Errorf("csv write row for player %s: %w", entry.Nickname, err)
		}
	}

	w.Flush()
	if err := w.Error(); err != nil {
		return nil, fmt.Errorf("csv flush: %w", err)
	}

	return buf.Bytes(), nil
}

// FormatLeaderboardCSVFilename generates a sanitized, safe filename for leaderboard CSV attachment.
// Output format: {session-slug}-{YYYY-MM-DD}-leaderboard.csv
func FormatLeaderboardCSVFilename(name string, endedAt *time.Time, fallbackCreatedAt time.Time) string {
	dateStr := ""
	if endedAt != nil && !endedAt.IsZero() {
		dateStr = endedAt.UTC().Format("2006-01-02")
	} else if !fallbackCreatedAt.IsZero() {
		dateStr = fallbackCreatedAt.UTC().Format("2006-01-02")
	} else {
		dateStr = time.Now().UTC().Format("2006-01-02")
	}

	slug := strings.ToLower(strings.TrimSpace(name))
	slug = nonAlphanumericRegex.ReplaceAllString(slug, "-")
	slug = multipleHyphensRegex.ReplaceAllString(slug, "-")
	slug = strings.Trim(slug, "-")

	if len(slug) > 50 {
		slug = strings.TrimRight(slug[:50], "-")
	}

	if slug == "" {
		slug = "session"
	}

	return fmt.Sprintf("%s-%s-leaderboard.csv", slug, dateStr)
}
