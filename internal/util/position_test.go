package util

import (
	"errors"
	"math/rand"
	"sort"
	"strings"
	"testing"
)

// Vectors from github.com/rocicorp/fracdex fracdex_test.go (valid inputs only;
// its invalid-input cases fall back to the older-key path here instead of
// erroring, and are covered by TestPositionBetween_OlderKeys).
func TestPositionBetween_FracdexVectors(t *testing.T) {
	tests := []struct{ a, b, want string }{
		{"", "", "a0"},
		{"", "a0", "Zz"},
		{"", "Zz", "Zy"},
		{"a0", "", "a1"},
		{"a1", "", "a2"},
		{"a0", "a1", "a0V"},
		{"a1", "a2", "a1V"},
		{"a0V", "a1", "a0l"},
		{"Zz", "a0", "ZzV"},
		{"Zz", "a1", "a0"},
		{"", "Y00", "Xzzz"},
		{"bzz", "", "c000"},
		{"a0", "a0V", "a0G"},
		{"a0", "a0G", "a08"},
		{"b125", "b129", "b127"},
		{"a0", "a1V", "a1"},
		{"Zz", "a01", "a0"},
		{"", "a0V", "a0"},
		{"", "b999", "b99"},
		{"aV", "aV0V", "aV0G"},
		{"", "A000000000000000000000000001", "A000000000000000000000000000V"},
		{"zzzzzzzzzzzzzzzzzzzzzzzzzzy", "", "zzzzzzzzzzzzzzzzzzzzzzzzzzz"},
		{"zzzzzzzzzzzzzzzzzzzzzzzzzzz", "", "zzzzzzzzzzzzzzzzzzzzzzzzzzzV"},
	}
	for _, tt := range tests {
		got, err := PositionBetween(tt.a, tt.b)
		if err != nil {
			t.Errorf("PositionBetween(%q, %q) error: %v", tt.a, tt.b, err)
			continue
		}
		if got != tt.want {
			t.Errorf("PositionBetween(%q, %q) = %q, want %q", tt.a, tt.b, got, tt.want)
		}
	}
}

func TestPositionsBetween_FracdexVectors(t *testing.T) {
	tests := []struct {
		a, b string
		n    int
		want string
	}{
		{"", "", 5, "a0 a1 a2 a3 a4"},
		{"a4", "", 10, "a5 a6 a7 a8 a9 aA aB aC aD aE"},
		{"", "a0", 5, "Zv Zw Zx Zy Zz"},
		{"a0", "a2", 20, "a04 a08 a0G a0K a0O a0V a0Z a0d a0l a0t a1 a14 a18 a1G a1O a1V a1Z a1d a1l a1t"},
	}
	for _, tt := range tests {
		got, err := PositionsBetween(tt.a, tt.b, tt.n)
		if err != nil {
			t.Errorf("PositionsBetween(%q, %q, %d) error: %v", tt.a, tt.b, tt.n, err)
			continue
		}
		if strings.Join(got, " ") != tt.want {
			t.Errorf("PositionsBetween(%q, %q, %d) = %q, want %q", tt.a, tt.b, tt.n, strings.Join(got, " "), tt.want)
		}
	}
}

// Issue #13: appends and top inserts on a fresh column must not grow a
// character per card.
func TestPositionBetween_FreshColumn(t *testing.T) {
	var appended []string
	last := ""
	for i := 0; i < 12; i++ {
		last = mustBetween(t, last, "")
		appended = append(appended, last)
	}
	if got, want := strings.Join(appended, " "), "a0 a1 a2 a3 a4 a5 a6 a7 a8 a9 aA aB"; got != want {
		t.Errorf("12 appends = %q, want %q", got, want)
	}

	var prepended []string
	first := ""
	for i := 0; i < 12; i++ {
		first = mustBetween(t, "", first)
		prepended = append(prepended, first)
	}
	if got, want := strings.Join(prepended, " "), "a0 Zz Zy Zx Zw Zv Zu Zt Zs Zr Zq Zp"; got != want {
		t.Errorf("12 top inserts = %q, want %q", got, want)
	}
}

func TestPositionBetween_EndsStayShort(t *testing.T) {
	last, first := "", ""
	for i := 0; i < 10000; i++ {
		last = mustBetween(t, last, "")
		first = mustBetween(t, "", first)
	}
	if len(last) > 4 {
		t.Errorf("after 10000 appends, key %q has length %d, want <= 4", last, len(last))
	}
	if len(first) > 4 {
		t.Errorf("after 10000 top inserts, key %q has length %d, want <= 4", first, len(first))
	}
}

func TestPositionBetween_OlderKeys(t *testing.T) {
	longUs := strings.Repeat("U", 99)
	bangs := strings.Repeat("!", 37) + "U"

	tests := []struct {
		name string
		a, b string
		want string // empty means only check ordering
	}{
		// 7+ Us parse as a length-prefixed key, so appends increment it.
		{"append after a long run of U", longUs, "", "UUUUUUV"},
		{"append after a short older key", "UUUU", "", "a0"},
		{"prepend before a short older key", "", "vfUUUUU", "a0"},
		{"between two older keys that straddle a0", "U", "k", "a0"},
		{"after a new key, below an older key", "a5", "k", "a6"},
		{"before a new key, above an older key", "U", "a0", "Zz"},
		{"prepend below a key starting with !", "", bangs, ""},
		{"prepend below a key ending in !", "", "a!", ""},
		{"between keys ending in !", "K!", "r!", ""},
		{"below a key ending in ! that shares a first digit", "q", "r!", "qV"},
		{"between an older key and its extension", "r", "r0", ""},
		{"append after a key ending in !", "t!", "", ""},
		{"append after the largest single character", "z", "", ""},
		{"between keys differing past a common prefix", "!!0", "!!2", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := PositionBetween(tt.a, tt.b)
			if err != nil {
				t.Fatalf("PositionBetween(%q, %q) error: %v", tt.a, tt.b, err)
			}
			if !sortsBetween(got, tt.a, tt.b) {
				t.Fatalf("PositionBetween(%q, %q) = %q, not strictly between", tt.a, tt.b, got)
			}
			if strings.HasSuffix(got, "!") {
				t.Errorf("PositionBetween(%q, %q) = %q, ends in the lowest character", tt.a, tt.b, got)
			}
			if tt.want != "" && got != tt.want {
				t.Errorf("PositionBetween(%q, %q) = %q, want %q", tt.a, tt.b, got, tt.want)
			}
		})
	}
}

func TestPositionBetween_Errors(t *testing.T) {
	tests := []struct {
		name string
		a, b string
		want error
	}{
		{"equal", "a0", "a0", ErrPositionsOutOfOrder},
		{"reversed", "a1", "a0", ErrPositionsOutOfOrder},
		{"no key between a key and itself plus !", "r", "r!", ErrNoPositionBetween},
		// A new key fits beside most unknown characters; these need the
		// fallback, which can't read them.
		{"unknown character below", "a-b", "a0", ErrInvalidPositionChars},
		{"unknown character above", "~", "", ErrInvalidPositionChars},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := PositionBetween(tt.a, tt.b)
			if !errors.Is(err, tt.want) {
				t.Fatalf("PositionBetween(%q, %q) = %q, %v; want error %v", tt.a, tt.b, got, err, tt.want)
			}
		})
	}
}

func TestIsLengthPrefixedPosition(t *testing.T) {
	valid := []string{"a0", "Zz", "b00", "a0V", "UUUUUUU", "UUUUUUUU", "zzzzzzzzzzzzzzzzzzzzzzzzzzz"}
	invalid := []string{
		"", "U", "UU", "a", "a00", "a0!", "a!", "!a0", "0", "a0 ", "b0",
		"A00000000000000000000000000", // smallest integer, reserved
	}
	for _, k := range valid {
		if !IsLengthPrefixedPosition(k) {
			t.Errorf("IsLengthPrefixedPosition(%q) = false, want true", k)
		}
	}
	for _, k := range invalid {
		if IsLengthPrefixedPosition(k) {
			t.Errorf("IsLengthPrefixedPosition(%q) = true, want false", k)
		}
	}
}

func TestPositionInitial(t *testing.T) {
	if got := PositionInitial(0); got != nil {
		t.Errorf("PositionInitial(0) = %v, want nil", got)
	}
	got := PositionInitial(2000)
	if len(got) != 2000 {
		t.Fatalf("PositionInitial(2000) returned %d positions", len(got))
	}
	if got[0] != "a0" {
		t.Errorf("PositionInitial(2000)[0] = %q, want a0", got[0])
	}
	for i := 1; i < len(got); i++ {
		if got[i] <= got[i-1] {
			t.Fatalf("position %d (%q) <= position %d (%q)", i, got[i], i-1, got[i-1])
		}
		if len(got[i]) > 3 {
			t.Fatalf("position %d (%q) is longer than 3", i, got[i])
		}
	}
}

// A column holding keys from older kan versions alongside new ones must stay
// strictly ordered however cards are inserted.
func TestPositionBetween_RandomInsertsIntoMixedColumn(t *testing.T) {
	column := []string{
		strings.Repeat("!", 37) + "U", "!!0", "!0", "!Q", "0", "2", "6", "E",
		"K!", "U", "UU", "UUUU", strings.Repeat("U", 99), "Zz", "a0", "a0V",
		"a1", "f!", "k", "r!", "t!UUUUUUU", "vfUUUUU", "w2UU", "xRUUUUUUUU",
		"yWUU", "z", "zU",
	}
	sort.Strings(column)

	rng := rand.New(rand.NewSource(13))
	for i := 0; i < 5000; i++ {
		idx := rng.Intn(len(column) + 1)
		lo, hi := "", ""
		if idx > 0 {
			lo = column[idx-1]
		}
		if idx < len(column) {
			hi = column[idx]
		}
		key, err := PositionBetween(lo, hi)
		if err != nil {
			t.Fatalf("insert %d: PositionBetween(%q, %q) error: %v", i, lo, hi, err)
		}
		if !sortsBetween(key, lo, hi) {
			t.Fatalf("insert %d: PositionBetween(%q, %q) = %q, not strictly between", i, lo, hi, key)
		}
		column = append(column[:idx], append([]string{key}, column[idx:]...)...)
	}
}

func FuzzPositionBetween(f *testing.F) {
	for _, seed := range [][2]string{
		{"", ""}, {"a0", ""}, {"", "a0"}, {"a!", ""}, {"", "a!"}, {"r", "r!!"},
		{"UUUU", "UUUUV"}, {"!!!U", "!0"}, {"Zz", "a0"}, {"z", ""}, {"a0", "a00V"},
	} {
		f.Add(seed[0], seed[1])
	}
	f.Fuzz(func(t *testing.T, a, b string) {
		key, err := PositionBetween(a, b)
		if err != nil {
			return
		}
		if !sortsBetween(key, a, b) {
			t.Fatalf("PositionBetween(%q, %q) = %q, not strictly between", a, b, key)
		}
		if !usesOnly(key, legacyDigits) {
			t.Fatalf("PositionBetween(%q, %q) = %q, uses characters outside the alphabet", a, b, key)
		}
	})
}

func mustBetween(t *testing.T, a, b string) string {
	t.Helper()
	key, err := PositionBetween(a, b)
	if err != nil {
		t.Fatalf("PositionBetween(%q, %q) error: %v", a, b, err)
	}
	return key
}
