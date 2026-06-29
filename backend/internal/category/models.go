package category

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
)

// Category represents a row in the categories table.
type Category struct {
	ID        uuid.UUID
	Name      string
	Slug      string
	CreatedAt time.Time
	UpdatedAt time.Time
	DeletedAt *time.Time
}

// CategoryWithCount is a Category plus a derived question count.
type CategoryWithCount struct {
	Category
	QuestionCount int
}

// slugify converts a display name into a URL-safe slug.
// Rules: lowercase, spaces/underscores → hyphens, non-alphanumeric stripped,
// consecutive hyphens collapsed, leading/trailing hyphens trimmed.
func slugify(name string) string {
	var b strings.Builder
	prevHyphen := true // start true so leading hyphens are skipped
	for _, r := range strings.ToLower(name) {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9':
			b.WriteRune(r)
			prevHyphen = false
		case r == ' ' || r == '-' || r == '_':
			if !prevHyphen {
				b.WriteByte('-')
				prevHyphen = true
			}
		}
	}
	s := strings.TrimRight(b.String(), "-")
	return s
}

// ErrCategoryNotFound is returned when a category ID does not exist or is soft-deleted.
var ErrCategoryNotFound = errors.New("category not found")

// ErrCategoryNameConflict is returned when the name (or generated slug) already exists.
var ErrCategoryNameConflict = errors.New("category name already exists")

// ErrCategoryHasQuestions is returned when deletion is blocked by active questions.
// It is a struct (not a plain var) so it can carry the blocking count.
type ErrCategoryHasQuestions struct {
	Count int
}

func (e ErrCategoryHasQuestions) Error() string {
	return fmt.Sprintf("category has %d active questions", e.Count)
}
