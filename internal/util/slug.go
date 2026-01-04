package util

import (
	"regexp"
	"strings"
	"unicode"

	"golang.org/x/text/unicode/norm"
)

var (
	// Match sequences of non-alphanumeric characters
	nonAlphanumeric = regexp.MustCompile(`[^a-z0-9]+`)
	// Match leading/trailing hyphens
	trimHyphens = regexp.MustCompile(`^-+|-+$`)
)

const maxSlugLength = 30

// Slugify converts a string to a URL-friendly slug.
// - Converts to lowercase
// - Normalizes unicode (removes accents)
// - Replaces spaces and special characters with hyphens
// - Removes leading/trailing hyphens
// - Truncates to maxSlugLength chars without cutting mid-word
func Slugify(s string) string {
	// Normalize unicode and convert to lowercase
	s = strings.ToLower(s)
	s = removeAccents(s)

	// Replace non-alphanumeric with hyphens
	s = nonAlphanumeric.ReplaceAllString(s, "-")

	// Trim leading/trailing hyphens
	s = trimHyphens.ReplaceAllString(s, "")

	// Truncate to max length without cutting mid-word
	if len(s) > maxSlugLength {
		s = s[:maxSlugLength]
		if idx := strings.LastIndex(s, "-"); idx > 0 {
			s = s[:idx]
		}
	}

	return s
}

// removeAccents removes diacritical marks from unicode characters.
func removeAccents(s string) string {
	// Decompose unicode characters (NFD normalization)
	result := norm.NFD.String(s)

	// Remove combining characters (accents, diacritics)
	var b strings.Builder
	for _, r := range result {
		if !unicode.Is(unicode.Mn, r) { // Mn = Mark, Nonspacing
			b.WriteRune(r)
		}
	}

	return b.String()
}
