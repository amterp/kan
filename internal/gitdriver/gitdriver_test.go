package gitdriver

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestAttributesPattern(t *testing.T) {
	tests := []struct {
		kanRel string
		want   string
	}{
		{".kan", ".kan/boards/*/cards/*.json"},
		{"data", "data/boards/*/cards/*.json"},
		{"nested/kan", "nested/kan/boards/*/cards/*.json"},
		{".", "boards/*/cards/*.json"},
	}
	for _, tt := range tests {
		if got := attributesPattern(tt.kanRel); got != tt.want {
			t.Errorf("attributesPattern(%q) = %q, want %q", tt.kanRel, got, tt.want)
		}
	}
}

func TestEnsureAttributesIdempotent(t *testing.T) {
	dir := t.TempDir()

	wrote, err := ensureAttributes(dir, ".kan")
	if err != nil {
		t.Fatalf("ensureAttributes: %v", err)
	}
	if !wrote {
		t.Fatal("expected first call to write the attributes line")
	}

	// Second call must be a no-op.
	wrote, err = ensureAttributes(dir, ".kan")
	if err != nil {
		t.Fatalf("ensureAttributes (2nd): %v", err)
	}
	if wrote {
		t.Error("expected second call to be a no-op")
	}

	data, err := os.ReadFile(filepath.Join(dir, ".gitattributes"))
	if err != nil {
		t.Fatalf("read .gitattributes: %v", err)
	}
	if count := strings.Count(string(data), "merge=kan"); count != 1 {
		t.Errorf("expected exactly one merge=kan line, got %d:\n%s", count, data)
	}
	if !OptedIn(dir, ".kan") {
		t.Error("OptedIn should be true after ensureAttributes")
	}
}

func TestEnsureAttributesPreservesExisting(t *testing.T) {
	dir := t.TempDir()
	existing := "*.png binary\n"
	if err := os.WriteFile(filepath.Join(dir, ".gitattributes"), []byte(existing), 0o644); err != nil {
		t.Fatal(err)
	}

	if _, err := ensureAttributes(dir, ".kan"); err != nil {
		t.Fatalf("ensureAttributes: %v", err)
	}

	data, _ := os.ReadFile(filepath.Join(dir, ".gitattributes"))
	if !strings.Contains(string(data), "*.png binary") {
		t.Errorf("existing entry was lost:\n%s", data)
	}
	if !strings.Contains(string(data), "merge=kan") {
		t.Errorf("kan entry was not added:\n%s", data)
	}
}

func TestOptedInFalseWhenAbsent(t *testing.T) {
	dir := t.TempDir()
	if OptedIn(dir, ".kan") {
		t.Error("OptedIn should be false with no .gitattributes")
	}
}

func TestDriverCommandQuotesPath(t *testing.T) {
	got := driverCommand("/path with space/kan")
	if !strings.HasPrefix(got, `"/path with space/kan" merge-driver `) {
		t.Errorf("driverCommand did not quote the binary path: %q", got)
	}
	for _, ph := range []string{"%O", "%A", "%B", "%P", "%L"} {
		if !strings.Contains(got, ph) {
			t.Errorf("driverCommand missing placeholder %s: %q", ph, got)
		}
	}
}
