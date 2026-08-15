package api

import (
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/fsnotify/fsnotify"
)

func TestClassifyChange_Card(t *testing.T) {
	fw := &FileWatcher{kanDir: "/project/.kan"}

	tests := []struct {
		name      string
		path      string
		op        fsnotify.Op
		wantKind  FileChangeKind
		wantType  FileChangeType
		wantBoard string
		wantCard  string
	}{
		{
			name:      "card created",
			path:      "/project/.kan/boards/main/cards/abc123.json",
			op:        fsnotify.Create,
			wantKind:  FileChangeKindCard,
			wantType:  FileChangeCreated,
			wantBoard: "main",
			wantCard:  "abc123",
		},
		{
			name:      "card modified",
			path:      "/project/.kan/boards/features/cards/xyz789.json",
			op:        fsnotify.Write,
			wantKind:  FileChangeKindCard,
			wantType:  FileChangeModified,
			wantBoard: "features",
			wantCard:  "xyz789",
		},
		{
			name:      "card deleted",
			path:      "/project/.kan/boards/main/cards/def456.json",
			op:        fsnotify.Remove,
			wantKind:  FileChangeKindCard,
			wantType:  FileChangeDeleted,
			wantBoard: "main",
			wantCard:  "def456",
		},
		{
			name:      "card renamed (treated as deleted)",
			path:      "/project/.kan/boards/main/cards/old.json",
			op:        fsnotify.Rename,
			wantKind:  FileChangeKindCard,
			wantType:  FileChangeDeleted,
			wantBoard: "main",
			wantCard:  "old",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			event := fsnotify.Event{Name: tt.path, Op: tt.op}
			change := fw.classifyChange(event)

			if change.Kind != tt.wantKind {
				t.Errorf("Kind = %q, want %q", change.Kind, tt.wantKind)
			}
			if change.Type != tt.wantType {
				t.Errorf("Type = %q, want %q", change.Type, tt.wantType)
			}
			if change.BoardName != tt.wantBoard {
				t.Errorf("BoardName = %q, want %q", change.BoardName, tt.wantBoard)
			}
			if change.CardID != tt.wantCard {
				t.Errorf("CardID = %q, want %q", change.CardID, tt.wantCard)
			}
		})
	}
}

func TestClassifyChange_Board(t *testing.T) {
	fw := &FileWatcher{kanDir: "/project/.kan"}

	event := fsnotify.Event{
		Name: "/project/.kan/boards/main/config.toml",
		Op:   fsnotify.Write,
	}
	change := fw.classifyChange(event)

	if change.Kind != FileChangeKindBoard {
		t.Errorf("Kind = %q, want %q", change.Kind, FileChangeKindBoard)
	}
	if change.BoardName != "main" {
		t.Errorf("BoardName = %q, want %q", change.BoardName, "main")
	}
	if change.CardID != "" {
		t.Errorf("CardID = %q, want empty", change.CardID)
	}
}

func TestClassifyChange_BoardDirectory(t *testing.T) {
	fw := &FileWatcher{kanDir: "/project/.kan"}

	tests := []struct {
		name     string
		op       fsnotify.Op
		wantType FileChangeType
	}{
		{"board created", fsnotify.Create, FileChangeCreated},
		{"board deleted", fsnotify.Remove, FileChangeDeleted},
		{"board renamed away", fsnotify.Rename, FileChangeDeleted},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			event := fsnotify.Event{Name: "/project/.kan/boards/second", Op: tt.op}
			change := fw.classifyChange(event)

			if change.Kind != FileChangeKindBoard {
				t.Errorf("Kind = %q, want %q", change.Kind, FileChangeKindBoard)
			}
			if change.Type != tt.wantType {
				t.Errorf("Type = %q, want %q", change.Type, tt.wantType)
			}
			if change.BoardName != "second" {
				t.Errorf("BoardName = %q, want %q", change.BoardName, "second")
			}
			if change.CardID != "" {
				t.Errorf("CardID = %q, want empty", change.CardID)
			}
		})
	}
}

func TestClassifyChange_Project(t *testing.T) {
	fw := &FileWatcher{kanDir: "/project/.kan"}

	event := fsnotify.Event{
		Name: "/project/.kan/config.toml",
		Op:   fsnotify.Write,
	}
	change := fw.classifyChange(event)

	if change.Kind != FileChangeKindProject {
		t.Errorf("Kind = %q, want %q", change.Kind, FileChangeKindProject)
	}
}

func TestClassifyChange_Unknown(t *testing.T) {
	fw := &FileWatcher{kanDir: "/project/.kan"}

	tests := []struct {
		name string
		path string
	}{
		{"random file", "/project/.kan/random.txt"},
		{"nested too deep", "/project/.kan/boards/main/cards/sub/file.json"},
		{"not json", "/project/.kan/boards/main/cards/file.txt"},
		{"boards directory", "/project/.kan/boards"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			event := fsnotify.Event{Name: tt.path, Op: fsnotify.Write}
			change := fw.classifyChange(event)

			if change.Kind != FileChangeKindUnknown {
				t.Errorf("Kind = %q, want %q", change.Kind, FileChangeKindUnknown)
			}
		})
	}
}

func TestClassifyChange_CrossPlatform(t *testing.T) {
	// Test with platform-specific path separators
	kanDir := filepath.Join("/project", ".kan")
	fw := &FileWatcher{kanDir: kanDir}

	cardPath := filepath.Join(kanDir, "boards", "main", "cards", "test.json")
	event := fsnotify.Event{Name: cardPath, Op: fsnotify.Create}
	change := fw.classifyChange(event)

	if change.Kind != FileChangeKindCard {
		t.Errorf("Kind = %q, want %q", change.Kind, FileChangeKindCard)
	}
	if change.BoardName != "main" {
		t.Errorf("BoardName = %q, want %q", change.BoardName, "main")
	}
}

// mockSubscriber implements FileWatcherSubscriber for testing. Changes arrive on
// the watcher's own goroutine, so access is guarded.
type mockSubscriber struct {
	mu      sync.Mutex
	changes []FileChange
}

func (m *mockSubscriber) OnFileChange(change FileChange) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.changes = append(m.changes, change)
}

// await polls until match sees a change it likes, or the deadline passes. It
// returns the matching change so callers can assert on its contents.
func (m *mockSubscriber) await(t *testing.T, what string, match func(FileChange) bool) FileChange {
	t.Helper()

	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		m.mu.Lock()
		for _, c := range m.changes {
			if match(c) {
				m.mu.Unlock()
				return c
			}
		}
		m.mu.Unlock()
		time.Sleep(20 * time.Millisecond)
	}

	m.mu.Lock()
	defer m.mu.Unlock()
	t.Fatalf("timed out waiting for %s; saw %d change(s): %+v", what, len(m.changes), m.changes)
	return FileChange{}
}

func TestFileWatcher_Subscribe(t *testing.T) {
	fw := &FileWatcher{
		subscribers: []FileWatcherSubscriber{},
	}

	sub1 := &mockSubscriber{}
	sub2 := &mockSubscriber{}

	fw.Subscribe(sub1)
	fw.Subscribe(sub2)

	if len(fw.subscribers) != 2 {
		t.Errorf("Expected 2 subscribers, got %d", len(fw.subscribers))
	}
}

func TestFileWatcher_Unsubscribe(t *testing.T) {
	sub1 := &mockSubscriber{}
	sub2 := &mockSubscriber{}

	fw := &FileWatcher{
		subscribers: []FileWatcherSubscriber{sub1, sub2},
	}

	fw.Unsubscribe(sub1)

	if len(fw.subscribers) != 1 {
		t.Errorf("Expected 1 subscriber, got %d", len(fw.subscribers))
	}
	if fw.subscribers[0] != sub2 {
		t.Error("Wrong subscriber remained")
	}
}

func TestFileWatcher_StoppedPreventsRestart(t *testing.T) {
	fw := &FileWatcher{
		stopped: true,
	}

	err := fw.Start()
	if err == nil {
		t.Error("Expected error when starting stopped watcher")
	}
}

// startWatcher builds a .kan tree containing one board, starts a watcher on it,
// and returns the root plus a subscriber collecting everything it emits.
func startWatcher(t *testing.T) (string, *mockSubscriber) {
	t.Helper()

	kanRoot := filepath.Join(t.TempDir(), ".kan")
	if err := os.MkdirAll(filepath.Join(kanRoot, "boards", "main", "cards"), 0755); err != nil {
		t.Fatalf("failed to build .kan tree: %v", err)
	}

	fw, err := NewFileWatcher(kanRoot)
	if err != nil {
		t.Fatalf("failed to create watcher: %v", err)
	}
	sub := &mockSubscriber{}
	fw.Subscribe(sub)
	if err := fw.Start(); err != nil {
		t.Fatalf("failed to start watcher: %v", err)
	}
	t.Cleanup(func() { fw.Stop() })

	return kanRoot, sub
}

func writeCard(t *testing.T, kanRoot, board, cardID string) {
	t.Helper()

	path := filepath.Join(kanRoot, "boards", board, "cards", cardID+".json")
	if err := os.WriteFile(path, []byte(`{"id":"`+cardID+`"}`), 0644); err != nil {
		t.Fatalf("failed to write card: %v", err)
	}
}

func TestFileWatcher_EmitsCardChangeForExistingBoard(t *testing.T) {
	kanRoot, sub := startWatcher(t)

	writeCard(t, kanRoot, "main", "abc123")

	change := sub.await(t, "card change in an existing board", func(c FileChange) bool {
		return c.Kind == FileChangeKindCard && c.CardID == "abc123"
	})
	if change.BoardName != "main" {
		t.Errorf("BoardName = %q, want %q", change.BoardName, "main")
	}
}

// A board created while the watcher is already running must be watched as deeply
// as one that existed at startup. This is the regression test for boards being
// write-blind on macOS: fsnotify's kqueue backend auto-registers a new watch's
// subdirectories with delete/rename flags only and emits no Create event for
// them, so watching just the board directory left boards/<new>/cards/ silent and
// cards added there never reached the browser.
func TestFileWatcher_EmitsCardChangeForBoardCreatedWhileRunning(t *testing.T) {
	kanRoot, sub := startWatcher(t)

	if err := os.MkdirAll(filepath.Join(kanRoot, "boards", "second", "cards"), 0755); err != nil {
		t.Fatalf("failed to create board: %v", err)
	}
	// Let the create event land and the recursive watch attach before writing.
	sub.await(t, "board change for the new board directory", func(c FileChange) bool {
		return c.Kind == FileChangeKindBoard && c.BoardName == "second"
	})

	writeCard(t, kanRoot, "second", "xyz789")

	sub.await(t, "card change in a board created while running", func(c FileChange) bool {
		return c.Kind == FileChangeKindCard && c.BoardName == "second" && c.CardID == "xyz789"
	})
}
