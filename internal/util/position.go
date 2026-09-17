package util

// Fractional indexing for card ordering within columns.
//
// A position is a string, and cards in a column sort by comparing positions
// byte by byte. Between any two positions a < b another position fits, so
// moving a card rewrites only that card's file, which keeps concurrent board
// edits mergeable in version control.
//
// New positions use the length-prefixed scheme from rocicorp's fractional
// indexing, ported from https://github.com/rocicorp/fracdex (CC0 1.0). A key
// is an integer part followed by an optional fraction, both in base62 digits
// (0-9A-Za-z). The integer part's first character says how many digits follow
// it: 'a' one, 'b' two, and so on up to 'z'; 'Z' one, 'Y' two, and so on down
// to 'A' for negative integers. Appending increments the integer (a0, a1, ...
// az, b00) and prepending decrements it (Zz, Zy, ... Z0, Yzz), so keys at
// either end of a column stay a few characters long however many cards pass
// through it. Repeated inserts into the same gap still lengthen the fraction.
//
// Kan used to generate unstructured keys over "!0-9A-Za-z", and boards still
// carry them. They sort correctly against new keys, so PositionBetween accepts
// them: it returns a length-prefixed key whenever one fits between the
// neighbors, and otherwise falls back to a plain midpoint over the old
// alphabet. `kan doctor --fix` rewrites columns holding such keys.

import (
	"errors"
	"fmt"
	"strings"
)

const (
	base62Digits = "0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz"
	// legacyDigits is the alphabet of keys written before the length-prefixed
	// scheme. It sorts in byte order, like base62Digits.
	legacyDigits = "!" + base62Digits

	startPosition = "a0"
	// smallestInt is the lowest integer part. The scheme reserves it so that a
	// key below every other key always exists.
	smallestInt = "A00000000000000000000000000"
)

// LongPositionLength is the key length above which kan suggests rewriting a
// column's positions.
const LongPositionLength = 12

var (
	// ErrPositionsOutOfOrder means the lower bound is not strictly below the
	// upper bound, usually because two cards share a key.
	ErrPositionsOutOfOrder = errors.New("positions are equal or out of order")
	// ErrNoPositionBetween means no key sorts strictly between the bounds,
	// e.g. "r" and "r!".
	ErrNoPositionBetween = errors.New("no position fits between these positions")
	// ErrInvalidPositionChars means a key holds characters kan never
	// generates, e.g. from a hand edit.
	ErrInvalidPositionChars = errors.New("position contains characters kan does not use")
)

// PositionBetween returns a position that sorts strictly between a and b.
// An empty a means no lower bound, and an empty b means no upper bound.
func PositionBetween(a, b string) (string, error) {
	if a != "" && b != "" && a >= b {
		return "", fmt.Errorf("%w: %q is not below %q", ErrPositionsOutOfOrder, a, b)
	}

	lo, hi := a, b
	if !IsLengthPrefixedPosition(lo) {
		lo = ""
	}
	if !IsLengthPrefixedPosition(hi) {
		hi = ""
	}
	if key := keyBetween(lo, hi); key != "" && sortsBetween(key, a, b) {
		return key, nil
	}

	for _, key := range []string{a, b} {
		if !usesOnly(key, legacyDigits) {
			return "", fmt.Errorf("%w: %q", ErrInvalidPositionChars, key)
		}
	}
	// midpoint assumes neither bound ends in its zero digit; given "q" and
	// "r!" it would return "r", leaving no room below "r!". Trailing zeros
	// don't change a key's value as a fraction, so drop them.
	key := midpoint(strings.TrimRight(a, legacyDigits[:1]), strings.TrimRight(b, legacyDigits[:1]), legacyDigits)
	if !sortsBetween(key, a, b) {
		return "", fmt.Errorf("%w: %q and %q", ErrNoPositionBetween, a, b)
	}
	return key, nil
}

// PositionsBetween returns n ascending positions that sort strictly between
// a and b, with the same bound rules as PositionBetween.
func PositionsBetween(a, b string, n int) ([]string, error) {
	if n <= 0 {
		return nil, nil
	}
	c, err := PositionBetween(a, b)
	if err != nil {
		return nil, err
	}
	if n == 1 {
		return []string{c}, nil
	}

	result := make([]string, 0, n)
	switch {
	case b == "":
		result = append(result, c)
		for len(result) < n {
			if c, err = PositionBetween(c, ""); err != nil {
				return nil, err
			}
			result = append(result, c)
		}
	case a == "":
		result = append(result, c)
		for len(result) < n {
			if c, err = PositionBetween("", c); err != nil {
				return nil, err
			}
			result = append(result, c)
		}
		for i, j := 0, len(result)-1; i < j; i, j = i+1, j-1 {
			result[i], result[j] = result[j], result[i]
		}
	default:
		mid := n / 2
		left, err := PositionsBetween(a, c, mid)
		if err != nil {
			return nil, err
		}
		right, err := PositionsBetween(c, b, n-mid-1)
		if err != nil {
			return nil, err
		}
		result = append(result, left...)
		result = append(result, c)
		result = append(result, right...)
	}
	return result, nil
}

// PositionInitial returns n ascending positions for a column's cards:
// a0, a1, a2, ...
func PositionInitial(n int) []string {
	// Unbounded on both sides, so this cannot fail.
	positions, _ := PositionsBetween("", "", n)
	return positions
}

// IsLengthPrefixedPosition reports whether key is a valid key in the
// length-prefixed scheme, as opposed to a key written by an older kan.
func IsLengthPrefixedPosition(key string) bool {
	if key == "" || key == smallestInt || !usesOnly(key, base62Digits) {
		return false
	}
	n := intPartLen(key[0])
	return n > 0 && len(key) >= n && !strings.HasSuffix(key[n:], "0")
}

// keyBetween is fracdex's KeyBetween. a and b must each be empty or a
// length-prefixed key, with a < b when both are set. It returns "" when the
// integer range is exhausted.
func keyBetween(a, b string) string {
	switch {
	case a == "" && b == "":
		return startPosition
	case a == "":
		ib := b[:intPartLen(b[0])]
		if ib == smallestInt {
			return ib + midpoint("", b[len(ib):], base62Digits)
		}
		if ib < b {
			return ib
		}
		return decrementInt(ib)
	case b == "":
		ia := a[:intPartLen(a[0])]
		if i := incrementInt(ia); i != "" {
			return i
		}
		return ia + midpoint(a[len(ia):], "", base62Digits)
	}

	ia := a[:intPartLen(a[0])]
	ib := b[:intPartLen(b[0])]
	if ia == ib {
		return ia + midpoint(a[len(ia):], b[len(ib):], base62Digits)
	}
	i := incrementInt(ia)
	if i == "" {
		return ""
	}
	if i < b {
		return i
	}
	return ia + midpoint(a[len(ia):], "", base62Digits)
}

// midpoint returns a string strictly between a and b, reading both as
// fractions over digits, where a missing digit counts as digits[0]. An empty b
// means no upper bound. Both must use only characters from digits. The result
// can fail to sort between a and b when no string does (b is a followed only
// by digits[0]), so callers check it.
func midpoint(a, b, digits string) string {
	zero := digits[0]
	if b != "" {
		// Strip the common prefix, padding a with zeros. b needs no padding:
		// it cannot run out before a while the two still match.
		i := 0
		for ; i < len(b); i++ {
			c := zero
			if i < len(a) {
				c = a[i]
			}
			if c != b[i] {
				break
			}
		}
		if i > 0 {
			if i >= len(a) {
				return b[:i] + midpoint("", b[i:], digits)
			}
			return b[:i] + midpoint(a[i:], b[i:], digits)
		}
	}

	digitA := 0
	if a != "" {
		digitA = strings.IndexByte(digits, a[0])
	}
	digitB := len(digits)
	if b != "" {
		digitB = strings.IndexByte(digits, b[0])
	}
	if digitB-digitA > 1 {
		return string(digits[(digitA+digitB+1)/2])
	}

	// The first digits are consecutive. If b is longer than one digit, its
	// first digit alone sorts between.
	if len(b) > 1 {
		return b[:1]
	}
	// Otherwise keep a's first digit and go past the rest of a, e.g.
	// midpoint("49", "5") is "4" + midpoint("9", "") = "495".
	rest := ""
	if a != "" {
		rest = a[1:]
	}
	return string(digits[digitA]) + midpoint(rest, "", digits)
}

// intPartLen returns the length of the integer part a key with this head
// character has, or 0 if head cannot start a key.
func intPartLen(head byte) int {
	switch {
	case head >= 'a' && head <= 'z':
		return int(head-'a') + 2
	case head >= 'A' && head <= 'Z':
		return int('Z'-head) + 2
	}
	return 0
}

// incrementInt returns the integer part after x, or "" if x is the largest.
func incrementInt(x string) string {
	head, digs := x[0], []byte(x[1:])
	for i := len(digs) - 1; i >= 0; i-- {
		d := strings.IndexByte(base62Digits, digs[i]) + 1
		if d < len(base62Digits) {
			digs[i] = base62Digits[d]
			return string(head) + string(digs)
		}
		digs[i] = base62Digits[0]
	}

	// Every digit carried over, so the integer needs a different head.
	switch head {
	case 'Z':
		return startPosition
	case 'z':
		return ""
	}
	head++
	if head > 'a' {
		digs = append(digs, base62Digits[0])
	} else {
		digs = digs[1:]
	}
	return string(head) + string(digs)
}

// decrementInt returns the integer part before x, or "" if x is the smallest.
func decrementInt(x string) string {
	maxDigit := base62Digits[len(base62Digits)-1]
	head, digs := x[0], []byte(x[1:])
	for i := len(digs) - 1; i >= 0; i-- {
		d := strings.IndexByte(base62Digits, digs[i]) - 1
		if d >= 0 {
			digs[i] = base62Digits[d]
			return string(head) + string(digs)
		}
		digs[i] = maxDigit
	}

	// Every digit borrowed, so the integer needs a different head.
	switch head {
	case 'a':
		return "Z" + string(maxDigit)
	case 'A':
		return ""
	}
	head--
	if head < 'Z' {
		digs = append(digs, maxDigit)
	} else {
		digs = digs[1:]
	}
	return string(head) + string(digs)
}

// sortsBetween reports whether a < key < b, where an empty bound is open.
func sortsBetween(key, a, b string) bool {
	return (a == "" || a < key) && (b == "" || key < b)
}

func usesOnly(s, digits string) bool {
	for i := 0; i < len(s); i++ {
		if strings.IndexByte(digits, s[i]) < 0 {
			return false
		}
	}
	return true
}
