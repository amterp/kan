package service

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/BurntSushi/toml"
	"github.com/amterp/kan/internal/config"
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
	svc, _, cleanup := setupMigrationTest(t, "v15")
	defer cleanup()

	plan, err := svc.PlanProject()
	if err != nil {
		t.Fatalf("PlanProject: %v", err)
	}
	if plan.Project == nil {
		t.Fatal("v15 fixture has a project config; plan.Project should be set")
	}
	if plan.Project.NeedsMigration {
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

// project/3 adds [linked_boards], an optional table, so a project/2 config
// only needs its stamp.
func TestMigrateService_ProjectV2ToV3(t *testing.T) {
	svc, tempDir, cleanup := setupMigrationTest(t, "v14")
	defer cleanup()

	plan, err := svc.PlanProject()
	if err != nil {
		t.Fatalf("PlanProject: %v", err)
	}
	if plan.Project == nil || !plan.Project.NeedsMigration || plan.Project.FromSchema != "project/2" {
		t.Fatalf("want a project/2 migration, got %+v", plan.Project)
	}
	if err := svc.Execute(plan, false); err != nil {
		t.Fatalf("Execute: %v", err)
	}

	raw := readRawTOML(t, filepath.Join(tempDir, ".kan", "config.toml"))
	if raw["kan_schema"] != "project/3" {
		t.Errorf("kan_schema = %v, want project/3", raw["kan_schema"])
	}
	if raw["id"] != "p_test123" || raw["name"] != "Test Project" {
		t.Errorf("id/name not preserved: %v / %v", raw["id"], raw["name"])
	}
	if _, ok := raw["favicon"].(map[string]any); !ok {
		t.Errorf("favicon dropped: %v", raw["favicon"])
	}
}

// Most commands don't need the project config, so a broken one is skipped
// with a warning rather than stopping every command.
func TestMigrateService_UnparseableProjectConfigIsSkipped(t *testing.T) {
	_, tempDir, cleanup := setupMigrationTest(t, "v15")
	defer cleanup()
	path := filepath.Join(tempDir, ".kan", "config.toml")
	if err := os.WriteFile(path, []byte("kan_schema = \"project/3\"\nname = [unclosed\n"), 0644); err != nil {
		t.Fatal(err)
	}
	svc := NewQuietMigrateService(config.NewPaths(tempDir, ""))
	var notices bytes.Buffer
	svc.notices = &notices

	plan, err := svc.PlanProject()
	if err != nil {
		t.Fatalf("PlanProject: %v", err)
	}
	if plan.Project != nil {
		t.Errorf("a broken project config should plan nothing: %+v", plan.Project)
	}
	if !strings.Contains(notices.String(), path) || !strings.Contains(notices.String(), "not valid TOML") {
		t.Errorf("warning = %q", notices.String())
	}

	// A command opens a project for reading more than once; warn once.
	if _, err := svc.PlanProject(); err != nil {
		t.Fatal(err)
	}
	if n := strings.Count(notices.String(), "not valid TOML"); n != 1 {
		t.Errorf("warned %d times", n)
	}
}
