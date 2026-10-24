package service

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/amterp/kan/internal/config"
)

// snapshotTree maps every file under root, by path relative to root, to its
// contents.
func snapshotTree(t *testing.T, root string) map[string][]byte {
	t.Helper()
	files := map[string][]byte{}
	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return err
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(root, path)
		files[rel] = data
		return nil
	})
	if err != nil {
		t.Fatalf("snapshot %s: %v", root, err)
	}
	return files
}

// TestMigrateInMemory_MatchesExecute checks that the in-memory migration
// used by read commands produces exactly the files `kan migrate` writes, for
// every fixture, and writes nothing itself.
func TestMigrateInMemory_MatchesExecute(t *testing.T) {
	entries, err := os.ReadDir(filepath.Join("testdata", "migrations"))
	if err != nil {
		t.Fatalf("list fixtures: %v", err)
	}

	for _, entry := range entries {
		if !entry.IsDir() || entry.Name() == "project_future" {
			continue // project_future must fail; TestMigrateService_ProjectFuture_Refused covers it
		}
		fixture := entry.Name()
		t.Run(fixture, func(t *testing.T) {
			_, memDir, cleanupMem := setupMigrationTest(t, fixture)
			defer cleanupMem()
			diskSvc, diskDir, cleanupDisk := setupMigrationTest(t, fixture)
			defer cleanupDisk()

			original := snapshotTree(t, memDir)

			memSvc := NewQuietMigrateService(config.NewPaths(memDir, ""))
			memPlan, err := memSvc.PlanProject()
			if err != nil {
				t.Fatalf("PlanProject: %v", err)
			}
			overlay, err := memSvc.MigrateInMemory(memPlan)
			if err != nil {
				t.Fatalf("MigrateInMemory: %v", err)
			}

			if after := snapshotTree(t, memDir); !equalTrees(original, after) {
				t.Fatal("MigrateInMemory changed files on disk")
			}

			diskPlan, err := diskSvc.PlanProject()
			if err != nil {
				t.Fatalf("PlanProject: %v", err)
			}
			if err := diskSvc.Execute(diskPlan, false); err != nil {
				t.Fatalf("Execute: %v", err)
			}

			for rel, onDisk := range snapshotTree(t, diskDir) {
				inMemory, ok := overlay.Get(filepath.Join(memDir, rel))
				if !ok {
					inMemory = original[rel]
				}
				if !bytes.Equal(inMemory, onDisk) {
					t.Errorf("%s differs\nin memory:\n%s\non disk:\n%s", rel, inMemory, onDisk)
				}
			}
		})
	}
}

func equalTrees(a, b map[string][]byte) bool {
	if len(a) != len(b) {
		return false
	}
	for k, v := range a {
		if !bytes.Equal(v, b[k]) {
			return false
		}
	}
	return true
}
