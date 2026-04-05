package util

// Fractional indexing for card ordering within columns.
//
// Positions are strings that sort lexicographically. Given any two positions
// a < b, we can always generate a position c such that a < c < b, without
// modifying any other positions. This makes reordering a single-card operation
// (only the moved card's file changes), which is ideal for git merge friendliness.
//
// Character set: '!' then digits 0-9, uppercase A-Z, lowercase a-z (63 chars).
// The '!' (ASCII 33) provides room below '0' for prepend operations.
// Normal positions are generated in the 0-z range; '!' is only used when
// prepending past the lower boundary.

const positionChars = "!0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz"
const positionBase = len(positionChars)

// charIndex returns the index of c in positionChars, or -1 if not found.
func charIndex(c byte) int {
	for i := 0; i < len(positionChars); i++ {
		if positionChars[i] == c {
			return i
		}
	}
	return -1
}

// PositionBetween returns a position that sorts between a and b.
//
// Pass "" for a to get a position before b (prepend to start of list).
// Pass "" for b to get a position after a (append to end of list).
// Both a and b empty returns a default starting position.
//
// When both are provided, a must sort before b (a < b).
func PositionBetween(a, b string) string {
	if a == "" && b == "" {
		return string(positionChars[positionBase/2])
	}
	if a == "" {
		return before(b)
	}
	if b == "" {
		return after(a)
	}
	if a >= b {
		panic("PositionBetween: a must be less than b")
	}
	return midpoint(a, b)
}

// PositionAfter returns a position that sorts after a. Shorthand for PositionBetween(a, "").
func PositionAfter(a string) string {
	return PositionBetween(a, "")
}

// PositionBefore returns a position that sorts before a. Shorthand for PositionBetween("", a).
func PositionBefore(a string) string {
	return PositionBetween("", a)
}

// PositionInitial generates n evenly-spaced position strings.
// Useful for assigning initial positions when migrating existing card orderings.
func PositionInitial(n int) []string {
	if n <= 0 {
		return nil
	}
	if n == 1 {
		return []string{string(positionChars[positionBase/2])}
	}

	// Determine how many characters we need for n unique positions.
	// With k characters, we have positionBase^k slots.
	digits := 2
	totalSlots := positionBase * positionBase
	for totalSlots < n+1 {
		digits++
		totalSlots *= positionBase
	}

	step := totalSlots / (n + 1)
	positions := make([]string, n)
	for i := 0; i < n; i++ {
		slot := step * (i + 1)
		pos := make([]byte, digits)
		remaining := slot
		for d := digits - 1; d >= 0; d-- {
			pos[d] = positionChars[remaining%positionBase]
			remaining /= positionBase
		}
		positions[i] = string(pos)
	}

	return positions
}

// after returns a position that sorts after a.
func after(a string) string {
	// Append the midpoint character. Always produces a longer string that
	// sorts after a (since a is a prefix of the result).
	return a + string(positionChars[positionBase/2])
}

// before returns a position that sorts before b.
func before(b string) string {
	// Walk through b looking for a character we can lower.
	// Take the midpoint between 0 (charset minimum) and that character.
	for i := 0; i < len(b); i++ {
		idx := charIndex(b[i])
		if idx > 1 {
			// Room to place something between charset[0] and b[i]
			mid := idx / 2
			return b[:i] + string(positionChars[mid])
		}
		if idx == 1 {
			// b[i] is charset[1] ('0'). charset[0] is '!'.
			// If we use '!' here, we get b[:i]+"!" which is < b[:i]+"0"...
			// but we need to check it's actually < b overall.
			// Take '!' and then find midpoint of remaining space.
			result := b[:i] + string(positionChars[0]) + string(positionChars[positionBase/2])
			if result < b {
				return result
			}
			// If not less (shouldn't happen), continue deeper
		}
		// idx == 0: this character is already at minimum, continue to next position
	}
	// All characters in b are at the minimum. Append a midpoint character.
	// Since b is a prefix of b+"V", and b+"!" < b+midpoint, but we need < b.
	// Actually b < b+"anything", so we can't go below by appending.
	// This means b is like "!!!" - the absolute minimum for its length.
	// Prepend '!' to get a shorter effective value.
	// Wait - "!" < "!!" in lexicographic ordering (prefix sorts first).
	// So return a string shorter than b that starts with minimum chars.
	// The midpoint of ("", b) when b is all-minimum: just use midpoint of first char.
	// With charset[0] = '!', "!" is the absolute minimum 1-char position.
	// "!!" is the minimum 2-char position. But "!" < "!!" (prefix rule).
	// So we can always return a shorter all-minimum-char string.
	if len(b) > 1 {
		return b[:len(b)-1]
	}
	// b is a single minimum character - can't go lower with this charset.
	// This should be virtually unreachable in practice.
	panic("PositionBefore: cannot generate position before absolute minimum")
}

// midpoint generates a string that sorts between a and b.
func midpoint(a, b string) string {
	maxLen := len(a)
	if len(b) > maxLen {
		maxLen = len(b)
	}

	// Build prefix incrementally to avoid out-of-bounds slicing
	// when i >= len(a) or len(b).
	var prefix string

	for i := 0; i < maxLen; i++ {
		aChar := 0
		if i < len(a) {
			aChar = charIndex(a[i])
		}

		bChar := positionBase // treat missing b chars as one-past-end
		if i < len(b) {
			bChar = charIndex(b[i])
		}

		if aChar == bChar {
			prefix += string(positionChars[aChar])
			continue
		}

		if bChar-aChar > 1 {
			// There's room between these characters at this position
			mid := (aChar + bChar) / 2
			return prefix + string(positionChars[mid])
		}

		// Adjacent characters (bChar - aChar == 1). We need to go deeper.
		// Take aChar at this position, then find a midpoint after
		// the remaining suffix of a.
		prefix += string(positionChars[aChar])
		suffix := ""
		if i+1 < len(a) {
			suffix = a[i+1:]
		}
		return prefix + midpointAfter(suffix)
	}

	// Shouldn't reach here if a < b, but defensively:
	return a + string(positionChars[positionBase/2])
}

// midpointAfter returns a string that sorts after s, staying as short as possible.
// Used internally by midpoint to generate positions in the "remaining" space.
func midpointAfter(s string) string {
	if s == "" {
		return string(positionChars[positionBase/2])
	}

	// Try to increment the last character that has room
	for i := len(s) - 1; i >= 0; i-- {
		idx := charIndex(s[i])
		if idx < positionBase-1 {
			mid := (idx + positionBase) / 2
			return s[:i] + string(positionChars[mid])
		}
	}

	// All max chars - append midpoint
	return s + string(positionChars[positionBase/2])
}
