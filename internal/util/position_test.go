package util

import (
	"testing"
)

func TestPositionBetween_BasicCases(t *testing.T) {
	tests := []struct {
		name string
		a, b string
	}{
		{"single chars with gap", "A", "Z"},
		{"single chars adjacent", "A", "B"},
		{"multi char", "Am", "An"},
		{"different lengths", "A", "B0"},
		{"same prefix", "abc", "abd"},
		{"wide gap", "0", "z"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := PositionBetween(tt.a, tt.b)
			if result <= tt.a {
				t.Errorf("PositionBetween(%q, %q) = %q, expected > %q", tt.a, tt.b, result, tt.a)
			}
			if result >= tt.b {
				t.Errorf("PositionBetween(%q, %q) = %q, expected < %q", tt.a, tt.b, result, tt.b)
			}
		})
	}
}

func TestPositionBetween_Panics(t *testing.T) {
	tests := []struct {
		name string
		a, b string
	}{
		{"equal", "A", "A"},
		{"a > b", "B", "A"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			defer func() {
				if r := recover(); r == nil {
					t.Errorf("PositionBetween(%q, %q) should have panicked", tt.a, tt.b)
				}
			}()
			PositionBetween(tt.a, tt.b)
		})
	}
}

func TestPositionBetween_ManyInsertions(t *testing.T) {
	// Simulate many insertions at the same spot (always inserting between
	// the first two elements). This is the worst case for position growth.
	a := "A"
	b := "z"
	for i := 0; i < 100; i++ {
		mid := PositionBetween(a, b)
		if mid <= a {
			t.Fatalf("iteration %d: PositionBetween(%q, %q) = %q, not > a", i, a, b, mid)
		}
		if mid >= b {
			t.Fatalf("iteration %d: PositionBetween(%q, %q) = %q, not < b", i, a, b, mid)
		}
		// Insert between a and mid (worst case: always going left)
		b = mid
	}
	t.Logf("After 100 left-insertions: last position = %q (len=%d)", b, len(b))
}

func TestPositionBetween_ManyInsertionsRight(t *testing.T) {
	// Simulate appending right (always inserting between last and a far-right bound)
	a := "A"
	b := "z"
	for i := 0; i < 100; i++ {
		mid := PositionBetween(a, b)
		if mid <= a {
			t.Fatalf("iteration %d: PositionBetween(%q, %q) = %q, not > a", i, a, b, mid)
		}
		if mid >= b {
			t.Fatalf("iteration %d: PositionBetween(%q, %q) = %q, not < b", i, a, b, mid)
		}
		a = mid
	}
	t.Logf("After 100 right-insertions: last position = %q (len=%d)", a, len(a))
}

func TestPositionAfter(t *testing.T) {
	tests := []string{"0", "A", "Z", "a", "z", "ZZ", "zz", "Am"}
	for _, pos := range tests {
		result := PositionAfter(pos)
		if result <= pos {
			t.Errorf("PositionAfter(%q) = %q, expected > %q", pos, result, pos)
		}
	}
}

func TestPositionBefore(t *testing.T) {
	tests := []string{"1", "A", "Z", "a", "z", "ZZ", "Am"}
	for _, pos := range tests {
		result := PositionBefore(pos)
		if result >= pos {
			t.Errorf("PositionBefore(%q) = %q, expected < %q", pos, result, pos)
		}
	}
}

func TestPositionInitial(t *testing.T) {
	t.Run("zero", func(t *testing.T) {
		result := PositionInitial(0)
		if result != nil {
			t.Errorf("PositionInitial(0) = %v, expected nil", result)
		}
	})

	t.Run("one", func(t *testing.T) {
		result := PositionInitial(1)
		if len(result) != 1 {
			t.Fatalf("PositionInitial(1) returned %d positions, expected 1", len(result))
		}
	})

	for _, n := range []int{2, 3, 5, 10, 50, 100} {
		t.Run("n="+string(rune('0'+n/100))+string(rune('0'+(n%100)/10))+string(rune('0'+n%10)), func(t *testing.T) {
			result := PositionInitial(n)
			if len(result) != n {
				t.Fatalf("PositionInitial(%d) returned %d positions", n, len(result))
			}
			for i := 1; i < len(result); i++ {
				if result[i] <= result[i-1] {
					t.Errorf("PositionInitial(%d): position %d (%q) <= position %d (%q)",
						n, i, result[i], i-1, result[i-1])
				}
			}
		})
	}
}

func TestPositionInitial_LargeN(t *testing.T) {
	// Test with a large number to ensure we handle it gracefully
	n := 2000
	result := PositionInitial(n)
	if len(result) != n {
		t.Fatalf("PositionInitial(%d) returned %d positions", n, len(result))
	}
	for i := 1; i < len(result); i++ {
		if result[i] <= result[i-1] {
			t.Fatalf("PositionInitial(%d): position %d (%q) <= position %d (%q)",
				n, i, result[i], i-1, result[i-1])
		}
	}
}

func TestPositionBetween_Stability(t *testing.T) {
	// Generate initial positions, then insert between each pair.
	// All results should maintain strict ordering.
	initial := PositionInitial(10)
	var all []string

	for i := 0; i < len(initial)-1; i++ {
		all = append(all, initial[i])
		between := PositionBetween(initial[i], initial[i+1])
		all = append(all, between)
	}
	all = append(all, initial[len(initial)-1])

	for i := 1; i < len(all); i++ {
		if all[i] <= all[i-1] {
			t.Errorf("ordering broken at index %d: %q <= %q", i, all[i], all[i-1])
		}
	}
}

func TestPositionAfter_Chain(t *testing.T) {
	// Chain of PositionAfter calls should always produce increasing values
	pos := "V"
	for i := 0; i < 50; i++ {
		next := PositionAfter(pos)
		if next <= pos {
			t.Fatalf("iteration %d: PositionAfter(%q) = %q, not increasing", i, pos, next)
		}
		pos = next
	}
	t.Logf("After 50 appends: position = %q (len=%d)", pos, len(pos))
}

func TestPositionBefore_Chain(t *testing.T) {
	// Chain of PositionBefore calls should always produce decreasing values
	pos := "V"
	for i := 0; i < 50; i++ {
		prev := PositionBefore(pos)
		if prev >= pos {
			t.Fatalf("iteration %d: PositionBefore(%q) = %q, not decreasing", i, pos, prev)
		}
		pos = prev
	}
	t.Logf("After 50 prepends: position = %q (len=%d)", pos, len(pos))
}
