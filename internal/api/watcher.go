package api

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"
)

// FileChangeType indicates what type of change occurred.
type FileChangeType string

const (
	FileChangeCreated  FileChangeType = "created"
	FileChangeModified FileChangeType = "modified"
	FileChangeDeleted  FileChangeType = "deleted"
)

// FileChangeKind indicates what kind of file changed.
type FileChangeKind string

const (
	FileChangeKindCard    FileChangeKind = "card"
	FileChangeKindBoard   FileChangeKind = "board"
	FileChangeKindProject FileChangeKind = "project"
	FileChangeKindUnknown FileChangeKind = "unknown"
)

// FileChange represents a file system change notification.
type FileChange struct {
	Type      FileChangeType `json:"type"`
	Kind      FileChangeKind `json:"kind"`
	BoardName string         `json:"board_name,omitempty"` // For card/board changes
	CardID    string         `json:"card_id,omitempty"`    // For card changes
	Path      string         `json:"path"`                 // Relative path from .kan/
}

// FileWatcherSubscriber receives file change notifications.
type FileWatcherSubscriber interface {
	OnFileChange(change FileChange)
}

// FileWatcher watches the .kan directory for changes and notifies subscribers.
type FileWatcher struct {
	watcher     *fsnotify.Watcher
	kanDir      string
	mu          sync.RWMutex
	subscribers []FileWatcherSubscriber
	debounce    map[string]*time.Timer
	debounceMu  sync.Mutex
	stopCh      chan struct{}
	stopped     bool // Once stopped, cannot restart
	running     bool
}

// NewFileWatcher creates a new file watcher for the .kan directory.
func NewFileWatcher(projectRoot string) (*FileWatcher, error) {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, err
	}

	kanDir := filepath.Join(projectRoot, ".kan")

	fw := &FileWatcher{
		watcher:  watcher,
		kanDir:   kanDir,
		debounce: make(map[string]*time.Timer),
		stopCh:   make(chan struct{}),
	}

	return fw, nil
}

// Subscribe adds a subscriber to receive file change notifications.
func (fw *FileWatcher) Subscribe(sub FileWatcherSubscriber) {
	fw.mu.Lock()
	defer fw.mu.Unlock()
	fw.subscribers = append(fw.subscribers, sub)
}

// Unsubscribe removes a subscriber.
func (fw *FileWatcher) Unsubscribe(sub FileWatcherSubscriber) {
	fw.mu.Lock()
	defer fw.mu.Unlock()
	for i, s := range fw.subscribers {
		if s == sub {
			fw.subscribers = append(fw.subscribers[:i], fw.subscribers[i+1:]...)
			return
		}
	}
}

// Start begins watching the .kan directory for changes.
func (fw *FileWatcher) Start() error {
	fw.mu.Lock()
	if fw.running {
		fw.mu.Unlock()
		return nil
	}
	if fw.stopped {
		fw.mu.Unlock()
		return fmt.Errorf("file watcher cannot be restarted after stop")
	}
	fw.running = true
	fw.mu.Unlock()

	// Add watches recursively
	if err := fw.addWatchesRecursive(fw.kanDir); err != nil {
		return err
	}

	go fw.run()
	return nil
}

// Stop stops watching for changes.
func (fw *FileWatcher) Stop() error {
	fw.mu.Lock()
	if !fw.running || fw.stopped {
		fw.mu.Unlock()
		return nil
	}
	fw.running = false
	fw.stopped = true
	fw.mu.Unlock()

	// Cancel all pending debounce timers to prevent them from firing after stop
	fw.debounceMu.Lock()
	for path, timer := range fw.debounce {
		timer.Stop()
		delete(fw.debounce, path)
	}
	fw.debounceMu.Unlock()

	close(fw.stopCh)
	return fw.watcher.Close()
}

// addWatchesRecursive adds watches to a directory and all subdirectories.
func (fw *FileWatcher) addWatchesRecursive(dir string) error {
	return filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil // Ignore errors, directory might not exist yet
		}
		if info.IsDir() {
			if err := fw.watcher.Add(path); err != nil {
				log.Printf("Warning: failed to watch %s: %v", path, err)
			}
		}
		return nil
	})
}

func (fw *FileWatcher) run() {
	for {
		select {
		case event, ok := <-fw.watcher.Events:
			if !ok {
				return
			}
			fw.handleEvent(event)

		case err, ok := <-fw.watcher.Errors:
			if !ok {
				return
			}
			log.Printf("File watcher error: %v", err)

		case <-fw.stopCh:
			return
		}
	}
}

func (fw *FileWatcher) handleEvent(event fsnotify.Event) {
	// Skip temporary files and hidden files
	base := filepath.Base(event.Name)
	if strings.HasPrefix(base, ".") || strings.HasSuffix(base, "~") {
		return
	}

	// If a directory was created, add a watch for it
	if event.Op&fsnotify.Create != 0 {
		if info, err := os.Stat(event.Name); err == nil && info.IsDir() {
			fw.watcher.Add(event.Name)
		}
	}

	// Debounce: wait 100ms before emitting to coalesce rapid changes
	fw.debounceMu.Lock()
	if timer, exists := fw.debounce[event.Name]; exists {
		timer.Stop()
	}
	fw.debounce[event.Name] = time.AfterFunc(100*time.Millisecond, func() {
		fw.emitChange(event)
		fw.debounceMu.Lock()
		delete(fw.debounce, event.Name)
		fw.debounceMu.Unlock()
	})
	fw.debounceMu.Unlock()
}

func (fw *FileWatcher) emitChange(event fsnotify.Event) {
	// Check if watcher was stopped (debounce timer may fire after Stop)
	fw.mu.RLock()
	if fw.stopped {
		fw.mu.RUnlock()
		return
	}
	subs := make([]FileWatcherSubscriber, len(fw.subscribers))
	copy(subs, fw.subscribers)
	fw.mu.RUnlock()

	change := fw.classifyChange(event)
	if change.Kind == FileChangeKindUnknown {
		return // Don't emit unknown changes
	}

	for _, sub := range subs {
		sub.OnFileChange(change)
	}
}

func (fw *FileWatcher) classifyChange(event fsnotify.Event) FileChange {
	relPath, err := filepath.Rel(fw.kanDir, event.Name)
	if err != nil {
		return FileChange{Kind: FileChangeKindUnknown}
	}

	change := FileChange{
		Path: relPath,
	}

	// Determine change type
	switch {
	case event.Op&fsnotify.Create != 0:
		change.Type = FileChangeCreated
	case event.Op&fsnotify.Write != 0:
		change.Type = FileChangeModified
	case event.Op&fsnotify.Remove != 0:
		change.Type = FileChangeDeleted
	case event.Op&fsnotify.Rename != 0:
		change.Type = FileChangeDeleted // Rename source is effectively deleted
	default:
		return FileChange{Kind: FileChangeKindUnknown}
	}

	// Classify by path pattern
	parts := strings.Split(relPath, string(filepath.Separator))

	// Card: boards/<boardname>/cards/<cardid>.json
	if len(parts) == 4 && parts[0] == "boards" && parts[2] == "cards" && strings.HasSuffix(parts[3], ".json") {
		change.Kind = FileChangeKindCard
		change.BoardName = parts[1]
		change.CardID = strings.TrimSuffix(parts[3], ".json")
		return change
	}

	// Board config: boards/<boardname>/config.toml
	if len(parts) == 3 && parts[0] == "boards" && parts[2] == "config.toml" {
		change.Kind = FileChangeKindBoard
		change.BoardName = parts[1]
		return change
	}

	// Project config: config.toml at root of .kan
	if len(parts) == 1 && parts[0] == "config.toml" {
		change.Kind = FileChangeKindProject
		return change
	}

	return FileChange{Kind: FileChangeKindUnknown}
}
