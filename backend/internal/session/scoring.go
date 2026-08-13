package session

import "math"

// ScoreAnswer computes the points awarded for a single answer submission.
//
// Rules (from docs/03-scoring-mechanics.md):
// - If the answer is incorrect, 0 points are awarded.
// - If speed bonus is disabled (or totalTimeMs <= 0), points_per_answer is awarded for correct answers.
// - If speed bonus is enabled:
//   multiplier = 1.0 + (time_remaining_ms / total_time_ms) * 0.5
//   points_awarded = ROUND(points_per_answer * multiplier)
// - time_remaining_ms is clamped to [0, total_time_ms].
func ScoreAnswer(
	pointsPerAnswer int,
	isCorrect bool,
	speedBonusEnabled bool,
	timeRemainingMs int64,
	totalTimeMs int64,
) int {
	if !isCorrect {
		return 0
	}

	if !speedBonusEnabled || totalTimeMs <= 0 {
		return pointsPerAnswer
	}

	ratio := float64(timeRemainingMs) / float64(totalTimeMs)
	if ratio < 0 {
		ratio = 0
	}
	if ratio > 1 {
		ratio = 1
	}

	multiplier := 1.0 + ratio*0.5
	return int(math.Round(float64(pointsPerAnswer) * multiplier))
}
