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

// SlugWords converts a string to normalized slug words.
//   - Converts to lowercase
//   - Normalizes unicode (removes accents)
//   - Replaces spaces and special characters with hyphens
//   - Splits on hyphens into individual words
//
// The caller is responsible for joining/truncating as needed.
func SlugWords(s string) []string {
	s = strings.ToLower(s)
	s = removeAccents(s)
	s = nonAlphanumeric.ReplaceAllString(s, "-")
	s = trimHyphens.ReplaceAllString(s, "")

	if s == "" {
		return nil
	}

	return strings.Split(s, "-")
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
