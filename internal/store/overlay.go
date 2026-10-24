package store

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

// ErrReadOnly is returned when a store opened for a read-only command is
// asked to write.
var ErrReadOnly = errors.New("kan bug: a read-only command tried to write project data")

// Overlay holds file contents that stand in for what is on disk, keyed by
// path. A read command on a project with an older schema migrates it in
// memory into an overlay, and its stores read the migrated files from there.
type Overlay struct {
	files map[string][]byte
}

// NewOverlay returns an empty overlay.
func NewOverlay() *Overlay {
	return &Overlay{files: make(map[string][]byte)}
}

// Put records the contents that path should read as.
func (o *Overlay) Put(path string, data []byte) {
	o.files[filepath.Clean(path)] = data
}

// Get returns the contents recorded for path, if any.
func (o *Overlay) Get(path string) ([]byte, bool) {
	data, ok := o.files[filepath.Clean(path)]
	return data, ok
}

// fileAccess is how the file stores reach disk: reads go through an
// optional overlay, and a read-only store refuses every write.
type fileAccess struct {
	overlay  *Overlay
	readOnly bool
}

func (a fileAccess) readFile(path string) ([]byte, error) {
	if a.overlay != nil {
		if data, ok := a.overlay.Get(path); ok {
			return data, nil
		}
	}
	return os.ReadFile(path)
}

func (a fileAccess) checkWritable(path string) error {
	if a.readOnly {
		return fmt.Errorf("%w (%s); nothing was written. Please report this", ErrReadOnly, path)
	}
	return nil
}
