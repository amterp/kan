package service

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/BurntSushi/toml"
	"github.com/amterp/kan/internal/version"
)

// readRawTOML decodes a TOML file without schema validation.
func readRawTOML(t *testing.T, path string) map[string]any {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	var raw map[string]any
	if _, err := toml.Decode(string(data), &raw); err != nil {
		t.Fatalf("decode %s: %v", path, err)
	}
	return raw
}

func TestMigrateService_ProjectV1ToCurrent(t *testing.T) {
	svc, tempDir, cleanup := setupMigrationTest(t, "v2")
	defer cleanup()

	// A key this kan doesn't know must survive, as one from a later kan would.
	configPath := filepath.Join(tempDir, ".kan", "config.toml")
	f, err := os.OpenFile(configPath, os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		t.Fatalf("open project config: %v", err)
	}
	if _, err := f.WriteString("\n[unknown_table]\nkey = \"kept\"\n"); err != nil {
		t.Fatalf("append: %v", err)
	}
	f.Close()

	plan, err := svc.PlanProject()
	if err != nil {
		t.Fatalf("PlanProject: %v", err)
	}
	if plan.Project == nil || !plan.Project.NeedsMigration {
		t.Fatalf("project/1 config should need migration, got %+v", plan.Project)
	}
	if plan.Project.FromSchema != "project/1" {
		t.Errorf("FromSchema = %q, want project/1", plan.Project.FromSchema)
	}

	if err := svc.Execute(plan, false); err != nil {
		t.Fatalf("Execute: %v", err)
	}

	raw := readRawTOML(t, configPath)
	if raw["kan_schema"] != version.CurrentProjectSchema() {
		t.Errorf("kan_schema = %v, want %s", raw["kan_schema"], version.CurrentProjectSchema())
	}
	if raw["id"] != "p_test123xyz" || raw["name"] != "test-project" {
		t.Errorf("id/name not preserved: %v / %v", raw["id"], raw["name"])
	}
	unknown, _ := raw["unknown_table"].(map[string]any)
	if unknown["key"] != "kept" {
		t.Errorf("unknown table dropped: %v", raw["unknown_table"])
	}
}

func TestMigrateService_ProjectCurrent_NoOp(t *testing.T) {
	svc, _, cleanup := setupMigrationTest(t, "v13")
	defer cleanup()

	plan, err := svc.PlanProject()
	if err != nil {
		t.Fatalf("PlanProject: %v", err)
	}
	if plan.Project == nil {
		t.Fatal("v13 fixture has a project config; plan.Project should be set")
	}
	if plan.HasChanges() {
		t.Errorf("current project config should not need migration: %+v", plan.Project)
	}
}

func TestMigrateService_ProjectMissing_NothingToMigrate(t *testing.T) {
	svc, tempDir, cleanup := setupMigrationTest(t, "project_missing")
	defer cleanup()

	plan, err := svc.PlanProject()
	if err != nil {
		t.Fatalf("PlanProject: %v", err)
	}
	if plan.Project != nil {
		t.Errorf("missing project config should plan nothing, got %+v", plan.Project)
	}
	if plan.HasChanges() {
		t.Error("project_missing fixture should not need migration")
	}

	if _, err := svc.MigrateProject(); err != nil {
		t.Fatalf("MigrateProject: %v", err)
	}
	if _, err := os.Stat(filepath.Join(tempDir, ".kan", "config.toml")); !os.IsNotExist(err) {
		t.Errorf("migration must not create a project config (stat err: %v)", err)
	}
}

func TestMigrateService_ProjectFuture_Refused(t *testing.T) {
	svc, tempDir, cleanup := setupMigrationTest(t, "project_future")
	defer cleanup()

	configPath := filepath.Join(tempDir, ".kan", "config.toml")
	before, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("read: %v", err)
	}

	plan, err := svc.PlanProject()
	if err != nil {
		t.Fatalf("PlanProject: %v", err)
	}
	var schemaErr *version.SchemaVersionError
	if err := plan.FutureVersionError(); !errors.As(err, &schemaErr) {
		t.Fatalf("FutureVersionError = %v, want SchemaVersionError", err)
	}

	if _, err := svc.MigrateProject(); !errors.As(err, &schemaErr) {
		t.Fatalf("MigrateProject = %v, want SchemaVersionError", err)
	}

	after, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if !bytes.Equal(before, after) {
		t.Errorf("future project config was rewritten:\n%s", after)
	}
}
