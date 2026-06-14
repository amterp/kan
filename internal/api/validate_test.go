package api

import "testing"

func TestIsSafePathSegment(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want bool
	}{
		{"normal name", "main", true},
		{"name with hyphens", "my-board", true},
		{"generated id", "a_01jdef-x9z", true},
		{"empty", "", false},
		{"dot", ".", false},
		{"dot dot", "..", false},
		{"forward slash", "../secret", false},
		{"embedded forward slash", "foo/bar", false},
		{"backslash", "..\\secret", false},
		{"embedded backslash", "foo\\bar", false},
		{"trailing slash", "main/", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isSafePathSegment(tt.in); got != tt.want {
				t.Errorf("isSafePathSegment(%q) = %v, want %v", tt.in, got, tt.want)
			}
		})
	}
}
