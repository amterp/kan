package api

import (
	"path/filepath"
	"testing"

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

// mockSubscriber implements FileWatcherSubscriber for testing
type mockSubscriber struct {
	changes []FileChange
}

func (m *mockSubscriber) OnFileChange(change FileChange) {
	m.changes = append(m.changes, change)
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
