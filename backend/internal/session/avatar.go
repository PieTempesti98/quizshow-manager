package session

import (
	"hash/fnv"
	"strings"
)

// AvatarPalette contains 12 vibrant, high-contrast, accessible hex color strings.
var AvatarPalette = []string{
	"#E85D24", // Vibrant Orange
	"#3B8BD4", // Vivid Blue
	"#7B2CBF", // Purple
	"#2A9D8F", // Teal
	"#E76F51", // Coral
	"#264653", // Deep Cyan / Slate
	"#F4A261", // Warm Sand
	"#E63946", // Imperial Red
	"#457B9D", // Steel Blue
	"#1D3557", // Prussian Navy
	"#06D6A0", // Mint Green
	"#118AB2", // Ocean Blue
}

// AssignAvatarColor deterministically assigns a color from the palette based on the player's nickname.
func AssignAvatarColor(nickname string) string {
	cleaned := strings.TrimSpace(strings.ToLower(nickname))
	if cleaned == "" {
		return AvatarPalette[0]
	}

	h := fnv.New32a()
	_, _ = h.Write([]byte(cleaned))
	idx := int(h.Sum32()) % len(AvatarPalette)
	if idx < 0 {
		idx = -idx
	}
	return AvatarPalette[idx]
}
