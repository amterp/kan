package service

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/amterp/kan/internal/config"
	"github.com/amterp/kan/internal/store"
	"github.com/amterp/kan/internal/version"
)

func readGlobalFixture(t *testing.T, name string) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("testdata", "global", name))
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}
	return string(data)
}

func TestMigrateService_GlobalV2ToV3_DropsProjectsAndHidesTempEntries(t *testing.T) {
	path := writeGlobalConfig(t, readGlobalFixture(t, "global-v2.toml"))

	svc := NewQuietMigrateService(config.NewPaths("", ""))
	plan, err := svc.PlanGlobalMigration()
	if err != nil {
		t.Fatalf("PlanGlobalMigration: %v", err)
	}
	if err := svc.Execute(&MigrationPlan{GlobalConfig: plan}, false); err != nil {
		t.Fatalf("Execute: %v", err)
	}

	cfg, err := store.NewGlobalStore().Load()
	if err != nil {
		t.Fatalf("Load migrated config: %v", err)
	}
	if cfg.KanSchema != version.CurrentGlobalSchema() {
		t.Errorf("KanSchema = %q", cfg.KanSchema)
	}
	if data, _ := os.ReadFile(path); strings.Contains(string(data), "projects") {
		t.Errorf("[projects] should be gone:\n%s", data)
	}
	if !cfg.Repos["/tmp/scratch-project"].Hidden {
		t.Error("entry under /tmp should be hidden")
	}
	if api := cfg.Repos["/home/me/src/api"]; api.Hidden || api.DefaultBoard != "main" {
		t.Errorf("api entry changed: %+v", api)
	}
	if notes := cfg.Repos["/home/me/notes"]; notes.Hidden || notes.DataLocation != "kan-data" {
		t.Errorf("notes entry changed: %+v", notes)
	}
	if cfg.Editor != "vim" || cfg.GlobalBoard == nil || cfg.GlobalBoard.Board != "inbox" {
		t.Errorf("other settings not preserved: editor %q, global board %+v", cfg.Editor, cfg.GlobalBoard)
	}
}

func TestMigrateService_GlobalV3_NoOp(t *testing.T) {
	writeGlobalConfig(t, readGlobalFixture(t, "global-v3.toml"))

	plan, err := NewQuietMigrateService(config.NewPaths("", "")).PlanGlobalMigration()
	if err != nil {
		t.Fatalf("PlanGlobalMigration: %v", err)
	}
	if plan != nil && plan.NeedsMigration {
		t.Errorf("global/3 should not need migration: %+v", plan)
	}
	if _, err := store.NewGlobalStore().Load(); err != nil {
		t.Errorf("global/3 fixture should load: %v", err)
	}
}

func TestReadGlobalConfigInMemory_AppliesMigrationWithoutWriting(t *testing.T) {
	path := writeGlobalConfig(t, readGlobalFixture(t, "global-v2.toml"))
	before, _ := os.ReadFile(path)

	cfg, err := ReadGlobalConfigInMemory(path)
	if err != nil {
		t.Fatalf("ReadGlobalConfigInMemory: %v", err)
	}
	if !cfg.Repos["/tmp/scratch-project"].Hidden {
		t.Error("temp entry should read as hidden")
	}
	if after, _ := os.ReadFile(path); string(after) != string(before) {
		t.Error("ReadGlobalConfigInMemory wrote the global config")
	}
}
