package model

import (
	"encoding/json"
	"reflect"
	"strings"
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
			name: "badges references non-set/non-boolean field",
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
			name: "badges with boolean field is valid",
			cfg: &BoardConfig{
				CustomFields: map[string]CustomFieldSchema{
					"blocked": {Type: "boolean"},
				},
				CardDisplay: CardDisplayConfig{
					Badges: []string{"blocked"},
				},
			},
			wantWarnings: 0,
		},
		{
			name: "badges with mixed set and boolean fields is valid",
			cfg: &BoardConfig{
				CustomFields: map[string]CustomFieldSchema{
					"labels":  {Type: "enum-set"},
					"blocked": {Type: "boolean"},
				},
				CardDisplay: CardDisplayConfig{
					Badges: []string{"labels", "blocked"},
				},
			},
			wantWarnings: 0,
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
			name: "valid tint config",
			cfg: &BoardConfig{
				CustomFields: map[string]CustomFieldSchema{
					"priority_color": {Type: "enum"},
				},
				CardDisplay: CardDisplayConfig{
					Tint: "priority_color",
				},
			},
			wantWarnings: 0,
		},
		{
			name: "tint references non-existent field",
			cfg: &BoardConfig{
				CustomFields: map[string]CustomFieldSchema{},
				CardDisplay: CardDisplayConfig{
					Tint: "missing",
				},
			},
			wantWarnings: 1,
		},
		{
			name: "tint references non-enum field",
			cfg: &BoardConfig{
				CustomFields: map[string]CustomFieldSchema{
					"priority": {Type: "string"},
				},
				CardDisplay: CardDisplayConfig{
					Tint: "priority",
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
					Tint:          "missing2",
					Badges:        []string{"missing3"},
					Metadata:      []string{"missing4"},
				},
			},
			wantWarnings: 4,
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

func TestCardMarshalFile_HistoryOneLinePerEntry(t *testing.T) {
	card := &Card{
		Version:         3,
		ID:              "abc123",
		Alias:           "c1",
		Title:           "Test",
		Creator:         "tester",
		CreatedAtMillis: 1700000000000,
		UpdatedAtMillis: 1700900000000,
		Column:          "review",
		Position:        "V",
		History: []HistoryEntry{
			{Field: "column", Value: "backlog", At: 1700000000000},
			{Field: "column", Value: "in-progress", At: 1700400000000},
			{Field: "column", Value: "review", At: 1700900000000},
		},
	}

	data, err := card.MarshalFile()
	if err != nil {
		t.Fatalf("MarshalFile failed: %v", err)
	}
	out := string(data)

	// Each history entry must be on its own single line, compact (no newline
	// inside the entry's braces), so appends are one-line diffs.
	wantLines := []string{
		`    {"field":"column","value":"backlog","at":1700000000000},`,
		`    {"field":"column","value":"in-progress","at":1700400000000},`,
		`    {"field":"column","value":"review","at":1700900000000}`,
	}
	for _, want := range wantLines {
		if !strings.Contains(out, want) {
			t.Errorf("MarshalFile output missing expected line:\n%s\n---got---\n%s", want, out)
		}
	}

	// The rest of the card stays pretty-printed.
	if !strings.Contains(out, "\n  \"id\": \"abc123\",") {
		t.Errorf("expected pretty-printed top-level fields, got:\n%s", out)
	}

	// Round-trips: history survives and is not swept into custom fields.
	var restored Card
	if err := json.Unmarshal(data, &restored); err != nil {
		t.Fatalf("Unmarshal failed: %v", err)
	}
	if len(restored.History) != 3 {
		t.Fatalf("expected 3 history entries, got %d", len(restored.History))
	}
	if restored.CustomFields != nil {
		t.Errorf("history leaked into custom fields: %v", restored.CustomFields)
	}
	if restored.History[1].Field != "column" || restored.History[1].Value != "in-progress" {
		t.Errorf("history entry corrupted on round-trip: %+v", restored.History[1])
	}
}

func TestCardMarshalFile_NoHistory(t *testing.T) {
	card := &Card{
		Version:         3,
		ID:              "abc123",
		Title:           "Test",
		Creator:         "tester",
		CreatedAtMillis: 1700000000000,
		UpdatedAtMillis: 1700000000000,
		Column:          "backlog",
		Position:        "V",
	}
	data, err := card.MarshalFile()
	if err != nil {
		t.Fatalf("MarshalFile failed: %v", err)
	}
	if strings.Contains(string(data), "history") {
		t.Errorf("expected no history key when history is empty, got:\n%s", data)
	}
}

func TestCardMarshalFile_PreservesCustomFields(t *testing.T) {
	card := &Card{
		Version:         3,
		ID:              "abc123",
		Title:           "Test",
		Creator:         "tester",
		CreatedAtMillis: 1700000000000,
		UpdatedAtMillis: 1700000000000,
		Column:          "backlog",
		Position:        "V",
		CustomFields:    map[string]any{"priority": "high"},
		History: []HistoryEntry{
			{Field: "column", Value: "backlog", At: 1700000000000},
		},
	}
	data, err := card.MarshalFile()
	if err != nil {
		t.Fatalf("MarshalFile failed: %v", err)
	}
	var restored Card
	if err := json.Unmarshal(data, &restored); err != nil {
		t.Fatalf("Unmarshal failed: %v", err)
	}
	if restored.CustomFields["priority"] != "high" {
		t.Errorf("custom field lost: %v", restored.CustomFields)
	}
	if len(restored.History) != 1 {
		t.Errorf("expected 1 history entry, got %d", len(restored.History))
	}
}

func TestCurrentColumnSinceMillis(t *testing.T) {
	// Empty history falls back to created_at.
	c := Card{CreatedAtMillis: 123}
	if got := c.CurrentColumnSinceMillis(); got != 123 {
		t.Errorf("empty history: got %d, want 123", got)
	}

	// Returns the most recent column entry.
	c = Card{
		CreatedAtMillis: 100,
		History: []HistoryEntry{
			{Field: "column", Value: "backlog", At: 100},
			{Field: "column", Value: "review", At: 500},
		},
	}
	if got := c.CurrentColumnSinceMillis(); got != 500 {
		t.Errorf("got %d, want 500", got)
	}
}
