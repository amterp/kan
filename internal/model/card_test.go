package model

import (
	"encoding/json"
	"reflect"
	"testing"
)

func TestCardJSON_RoundTrip(t *testing.T) {
	original := &Card{
		Version:         1,
		ID:              "abc123",
		Alias:           "test-card",
		AliasExplicit:   false,
		Title:           "Test Card",
		Description:     "A test card",
		Parent:          "parent123",
		Creator:         "tester",
		CreatedAtMillis: 1704307200000,
		UpdatedAtMillis: 1704393600000,
		Comments: []Comment{
			{
				ID:              "c_123",
				Body:            "A comment",
				Author:          "tester",
				CreatedAtMillis: 1704310800000,
			},
		},
	}

	// Marshal
	data, err := json.Marshal(original)
	if err != nil {
		t.Fatalf("Marshal failed: %v", err)
	}

	// Unmarshal
	var restored Card
	if err := json.Unmarshal(data, &restored); err != nil {
		t.Fatalf("Unmarshal failed: %v", err)
	}

	// Compare (CustomFields will be nil/empty in both)
	// Note: Column is not serialized (json:"-") so we don't compare it
	if original.Version != restored.Version {
		t.Errorf("Version mismatch: got %d, want %d", restored.Version, original.Version)
	}
	if original.ID != restored.ID {
		t.Errorf("ID mismatch: got %q, want %q", restored.ID, original.ID)
	}
	if original.Title != restored.Title {
		t.Errorf("Title mismatch: got %q, want %q", restored.Title, original.Title)
	}
	if len(original.Comments) != len(restored.Comments) {
		t.Errorf("Comments length mismatch: got %d, want %d", len(restored.Comments), len(original.Comments))
	}
}

func TestCardJSON_CustomFields(t *testing.T) {
	original := &Card{
		Version:         1,
		ID:              "abc123",
		Alias:           "test-card",
		Title:           "Test Card",
		Creator:         "tester",
		CreatedAtMillis: 1704307200000,
		UpdatedAtMillis: 1704393600000,
		CustomFields: map[string]any{
			"priority": "high",
			"assignee": "john",
		},
	}

	// Marshal
	data, err := json.Marshal(original)
	if err != nil {
		t.Fatalf("Marshal failed: %v", err)
	}

	// Verify custom fields are at top level
	var raw map[string]any
	if err := json.Unmarshal(data, &raw); err != nil {
		t.Fatalf("Unmarshal to map failed: %v", err)
	}

	if raw["priority"] != "high" {
		t.Errorf("priority field not at top level: %v", raw)
	}
	if raw["assignee"] != "john" {
		t.Errorf("assignee field not at top level: %v", raw)
	}

	// Unmarshal back to Card
	var restored Card
	if err := json.Unmarshal(data, &restored); err != nil {
		t.Fatalf("Unmarshal failed: %v", err)
	}

	if restored.CustomFields["priority"] != "high" {
		t.Errorf("priority custom field not restored: %v", restored.CustomFields)
	}
	if restored.CustomFields["assignee"] != "john" {
		t.Errorf("assignee custom field not restored: %v", restored.CustomFields)
	}
}

func TestCardJSON_EmptyCustomFields(t *testing.T) {
	original := &Card{
		Version:         1,
		ID:              "abc123",
		Alias:           "test-card",
		Title:           "Test Card",
		Creator:         "tester",
		CreatedAtMillis: 1704307200000,
		UpdatedAtMillis: 1704393600000,
	}

	data, err := json.Marshal(original)
	if err != nil {
		t.Fatalf("Marshal failed: %v", err)
	}

	var restored Card
	if err := json.Unmarshal(data, &restored); err != nil {
		t.Fatalf("Unmarshal failed: %v", err)
	}

	// CustomFields should be nil when empty
	if restored.CustomFields != nil {
		t.Errorf("Expected nil CustomFields, got %v", restored.CustomFields)
	}
}

func TestBoardConfig_HasColumn(t *testing.T) {
	cfg := &BoardConfig{
		Columns: []Column{
			{Name: "backlog", Color: "#6b7280"},
			{Name: "in-progress", Color: "#f59e0b"},
			{Name: "done", Color: "#10b981"},
		},
	}

	if !cfg.HasColumn("backlog") {
		t.Error("HasColumn should return true for existing column")
	}
	if !cfg.HasColumn("in-progress") {
		t.Error("HasColumn should return true for existing column")
	}
	if cfg.HasColumn("nonexistent") {
		t.Error("HasColumn should return false for nonexistent column")
	}
}

func TestBoardConfig_GetDefaultColumn(t *testing.T) {
	tests := []struct {
		name     string
		cfg      *BoardConfig
		expected string
	}{
		{
			name: "explicit default",
			cfg: &BoardConfig{
				Columns:       []Column{{Name: "backlog"}, {Name: "next"}},
				DefaultColumn: "next",
			},
			expected: "next",
		},
		{
			name: "falls back to first column",
			cfg: &BoardConfig{
				Columns: []Column{{Name: "first"}, {Name: "second"}},
			},
			expected: "first",
		},
		{
			name: "empty columns returns empty string",
			cfg: &BoardConfig{
				Columns: []Column{},
			},
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.cfg.GetDefaultColumn()
			if result != tt.expected {
				t.Errorf("GetDefaultColumn() = %q, want %q", result, tt.expected)
			}
		})
	}
}

func TestDefaultColumns(t *testing.T) {
	cols := DefaultColumns()

	if len(cols) != 4 {
		t.Errorf("Expected 4 default columns, got %d", len(cols))
	}

	expectedNames := []string{"backlog", "next", "in-progress", "done"}
	for i, name := range expectedNames {
		if cols[i].Name != name {
			t.Errorf("Column %d: expected name %q, got %q", i, name, cols[i].Name)
		}
		if cols[i].Color == "" {
			t.Errorf("Column %d: missing color", i)
		}
	}
}

func TestBoardConfig_ValidateCardDisplay(t *testing.T) {
	tests := []struct {
		name         string
		cfg          *BoardConfig
		wantWarnings int
	}{
		{
			name: "valid config",
			cfg: &BoardConfig{
				CustomFields: map[string]CustomFieldSchema{
					"type":   {Type: "enum"},
					"labels": {Type: "enum-set"},
					"notes":  {Type: "string"},
				},
				CardDisplay: CardDisplayConfig{
					TypeIndicator: "type",
					Badges:        []string{"labels"},
					Metadata:      []string{"notes"},
				},
			},
			wantWarnings: 0,
		},
		{
			name: "empty card_display",
			cfg: &BoardConfig{
				CustomFields: map[string]CustomFieldSchema{
					"type": {Type: "enum"},
				},
				CardDisplay: CardDisplayConfig{},
			},
			wantWarnings: 0,
		},
		{
			name: "type_indicator references non-existent field",
			cfg: &BoardConfig{
				CustomFields: map[string]CustomFieldSchema{},
				CardDisplay: CardDisplayConfig{
					TypeIndicator: "missing",
				},
			},
			wantWarnings: 1,
		},
		{
			name: "type_indicator references non-enum field",
			cfg: &BoardConfig{
				CustomFields: map[string]CustomFieldSchema{
					"type": {Type: "string"},
				},
				CardDisplay: CardDisplayConfig{
					TypeIndicator: "type",
				},
			},
			wantWarnings: 1,
		},
		{
			name: "badges references non-existent field",
			cfg: &BoardConfig{
				CustomFields: map[string]CustomFieldSchema{},
				CardDisplay: CardDisplayConfig{
					Badges: []string{"missing"},
				},
			},
			wantWarnings: 1,
		},
		{
			name: "badges references non-set field",
			cfg: &BoardConfig{
				CustomFields: map[string]CustomFieldSchema{
					"labels": {Type: "enum"},
				},
				CardDisplay: CardDisplayConfig{
					Badges: []string{"labels"},
				},
			},
			wantWarnings: 1,
		},
		{
			name: "metadata references non-existent field",
			cfg: &BoardConfig{
				CustomFields: map[string]CustomFieldSchema{},
				CardDisplay: CardDisplayConfig{
					Metadata: []string{"missing"},
				},
			},
			wantWarnings: 1,
		},
		{
			name: "multiple warnings",
			cfg: &BoardConfig{
				CustomFields: map[string]CustomFieldSchema{},
				CardDisplay: CardDisplayConfig{
					TypeIndicator: "missing1",
					Badges:        []string{"missing2"},
					Metadata:      []string{"missing3"},
				},
			},
			wantWarnings: 3,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			warnings := tt.cfg.ValidateCardDisplay()
			if len(warnings) != tt.wantWarnings {
				t.Errorf("ValidateCardDisplay() returned %d warnings, want %d. Warnings: %v", len(warnings), tt.wantWarnings, warnings)
			}
		})
	}
}

func TestGlobalConfig_GetRepoConfig(t *testing.T) {
	cfg := &GlobalConfig{
		Repos: map[string]RepoConfig{
			"/path/to/repo": {DefaultBoard: "main", DataLocation: "tools/kan"},
		},
	}

	// Found
	repoCfg := cfg.GetRepoConfig("/path/to/repo")
	if repoCfg == nil {
		t.Fatal("Expected to find repo config")
	}
	if repoCfg.DefaultBoard != "main" {
		t.Errorf("DefaultBoard = %q, want %q", repoCfg.DefaultBoard, "main")
	}

	// Not found
	if cfg.GetRepoConfig("/other/path") != nil {
		t.Error("Expected nil for unknown repo")
	}

	// Nil repos map
	emptyCfg := &GlobalConfig{}
	if emptyCfg.GetRepoConfig("/any") != nil {
		t.Error("Expected nil for nil repos map")
	}
}

func TestGlobalConfig_SetRepoConfig(t *testing.T) {
	cfg := &GlobalConfig{}

	cfg.SetRepoConfig("/path/to/repo", RepoConfig{DefaultBoard: "features"})

	if cfg.Repos == nil {
		t.Fatal("Repos map should be initialized")
	}

	repoCfg := cfg.GetRepoConfig("/path/to/repo")
	if repoCfg == nil || repoCfg.DefaultBoard != "features" {
		t.Error("SetRepoConfig didn't work correctly")
	}
}

func TestGlobalConfig_RegisterProject(t *testing.T) {
	cfg := &GlobalConfig{}

	cfg.RegisterProject("myproject", "/path/to/myproject")

	if cfg.Projects == nil {
		t.Fatal("Projects map should be initialized")
	}

	if cfg.Projects["myproject"] != "/path/to/myproject" {
		t.Errorf("RegisterProject didn't work correctly: %v", cfg.Projects)
	}
}

func TestComment(t *testing.T) {
	comment := Comment{
		ID:              "c_abc",
		Body:            "Test comment",
		Author:          "tester",
		CreatedAtMillis: 1704307200000,
	}

	data, err := json.Marshal(comment)
	if err != nil {
		t.Fatalf("Marshal failed: %v", err)
	}

	var restored Comment
	if err := json.Unmarshal(data, &restored); err != nil {
		t.Fatalf("Unmarshal failed: %v", err)
	}

	if !reflect.DeepEqual(comment, restored) {
		t.Errorf("Comment round-trip failed: got %+v, want %+v", restored, comment)
	}
}

func TestValidateCustomFieldName(t *testing.T) {
	tests := []struct {
		name      string
		fieldName string
		wantErr   bool
	}{
		{"valid name", "priority", false},
		{"valid with underscore", "my_field", false},
		{"valid x prefix", "x_priority", false},
		{"reserved underscore prefix", "_internal", true},
		{"reserved _v", "_v", true},
		{"reserved kan prefix", "kan_status", true},
		{"reserved kan_schema", "kan_schema", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateCustomFieldName(tt.fieldName)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateCustomFieldName(%q) error = %v, wantErr %v", tt.fieldName, err, tt.wantErr)
			}
		})
	}
}

func TestValidateCustomFields(t *testing.T) {
	// Valid fields
	err := ValidateCustomFields(map[string]any{
		"priority": "high",
		"x_status": "open",
	})
	if err != nil {
		t.Errorf("ValidateCustomFields with valid fields returned error: %v", err)
	}

	// Invalid field with _ prefix
	err = ValidateCustomFields(map[string]any{
		"priority":  "high",
		"_internal": "bad",
	})
	if err == nil {
		t.Error("ValidateCustomFields should reject fields with _ prefix")
	}

	// Invalid field with kan_ prefix
	err = ValidateCustomFields(map[string]any{
		"kan_reserved": "bad",
	})
	if err == nil {
		t.Error("ValidateCustomFields should reject fields with kan_ prefix")
	}

	// Nil map is valid
	err = ValidateCustomFields(nil)
	if err != nil {
		t.Errorf("ValidateCustomFields(nil) returned error: %v", err)
	}
}
