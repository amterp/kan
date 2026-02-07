package cli

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"

	"github.com/amterp/kan/internal/model"
)

// TestCardJsonFieldSync ensures cardJson stays in sync with model.Card.
// If this test fails, you probably added a field to model.Card but forgot
// to add it to cardJson in json_output.go.
func TestCardJsonFieldSync(t *testing.T) {
	cardType := reflect.TypeOf(model.Card{})
	cardJsonType := reflect.TypeOf(cardJson{})

	// Fields intentionally excluded from cardJson (with reasons)
	excluded := map[string]string{
		"Version": "Internal schema version (_v), not useful for CLI users",
	}

	// Fields that exist in cardJson but not in model.Card
	cardJsonOnly := map[string]bool{
		// Currently none - all cardJson fields should map to model.Card fields
	}

	// Check that all model.Card fields are in cardJson (or explicitly excluded)
	for i := 0; i < cardType.NumField(); i++ {
		field := cardType.Field(i)
		fieldName := field.Name

		if reason, isExcluded := excluded[fieldName]; isExcluded {
			// Verify the field is NOT in cardJson
			if _, found := cardJsonType.FieldByName(fieldName); found {
				t.Errorf("Field %q is marked as excluded (reason: %s) but exists in cardJson", fieldName, reason)
			}
			continue
		}

		// Field should exist in cardJson
		jsonField, found := cardJsonType.FieldByName(fieldName)
		if !found {
			t.Errorf("model.Card has field %q but cardJson does not. "+
				"Either add it to cardJson and cardToJson(), or add it to the excluded map with a reason.",
				fieldName)
			continue
		}

		// Verify types match (allowing for some flexibility)
		if !typesCompatible(field.Type, jsonField.Type) {
			t.Errorf("Field %q has type %v in model.Card but %v in cardJson",
				fieldName, field.Type, jsonField.Type)
		}
	}

	// Check for extra fields in cardJson that don't exist in model.Card
	for i := 0; i < cardJsonType.NumField(); i++ {
		field := cardJsonType.Field(i)
		fieldName := field.Name

		if cardJsonOnly[fieldName] {
			continue
		}

		if _, found := cardType.FieldByName(fieldName); !found {
			if _, isExcluded := excluded[fieldName]; !isExcluded {
				t.Errorf("cardJson has field %q that doesn't exist in model.Card. "+
					"If this is intentional, add it to cardJsonOnly map.",
					fieldName)
			}
		}
	}
}

// typesCompatible checks if two types are compatible for our purposes.
// This is stricter than just checking Kind - it verifies element types for
// slices and maps to catch real type drift.
func typesCompatible(a, b reflect.Type) bool {
	if a == b {
		return true
	}

	// Different kinds are never compatible
	if a.Kind() != b.Kind() {
		return false
	}

	switch a.Kind() {
	case reflect.Slice, reflect.Array:
		// Check element types match
		return typesCompatible(a.Elem(), b.Elem())

	case reflect.Map:
		// Check key and value types match
		return typesCompatible(a.Key(), b.Key()) && typesCompatible(a.Elem(), b.Elem())

	case reflect.Ptr:
		// Check pointed-to types match
		return typesCompatible(a.Elem(), b.Elem())

	default:
		// For basic types (string, int, bool, etc.), same kind means compatible
		// This handles cases like int vs int64 being flagged as incompatible (different types)
		// but allows the test to pass when types are genuinely the same
		return true
	}
}

// TestCardToJsonCopiesAllFields ensures cardToJson copies all fields.
func TestCardToJsonCopiesAllFields(t *testing.T) {
	card := &model.Card{
		ID:              "test-id",
		Alias:           "test-alias",
		AliasExplicit:   true,
		Title:           "Test Title",
		Description:     "Test Description",
		Parent:          "parent-id",
		Creator:         "Test Creator",
		CreatedAtMillis: 1234567890,
		UpdatedAtMillis: 1234567891,
		Column:          "test-column",
		CustomFields:    map[string]any{"priority": "high"},
	}

	cj := cardToJson(card)

	if cj.ID != card.ID {
		t.Errorf("ID mismatch: got %q, want %q", cj.ID, card.ID)
	}
	if cj.Alias != card.Alias {
		t.Errorf("Alias mismatch: got %q, want %q", cj.Alias, card.Alias)
	}
	if cj.AliasExplicit != card.AliasExplicit {
		t.Errorf("AliasExplicit mismatch: got %v, want %v", cj.AliasExplicit, card.AliasExplicit)
	}
	if cj.Title != card.Title {
		t.Errorf("Title mismatch: got %q, want %q", cj.Title, card.Title)
	}
	if cj.Description != card.Description {
		t.Errorf("Description mismatch: got %q, want %q", cj.Description, card.Description)
	}
	if cj.Parent != card.Parent {
		t.Errorf("Parent mismatch: got %q, want %q", cj.Parent, card.Parent)
	}
	if cj.Creator != card.Creator {
		t.Errorf("Creator mismatch: got %q, want %q", cj.Creator, card.Creator)
	}
	if cj.CreatedAtMillis != card.CreatedAtMillis {
		t.Errorf("CreatedAtMillis mismatch: got %d, want %d", cj.CreatedAtMillis, card.CreatedAtMillis)
	}
	if cj.UpdatedAtMillis != card.UpdatedAtMillis {
		t.Errorf("UpdatedAtMillis mismatch: got %d, want %d", cj.UpdatedAtMillis, card.UpdatedAtMillis)
	}
	if cj.Column != card.Column {
		t.Errorf("Column mismatch: got %q, want %q", cj.Column, card.Column)
	}
	if len(cj.CustomFields) != len(card.CustomFields) {
		t.Errorf("CustomFields length mismatch: got %d, want %d", len(cj.CustomFields), len(card.CustomFields))
	}
}

// TestBoardDescribeFieldSync ensures BoardDescribeInfo stays in sync with model.BoardConfig.
// If this test fails, you probably added a field to model.BoardConfig but forgot
// to add it to BoardDescribeInfo in json_output.go.
func TestBoardDescribeFieldSync(t *testing.T) {
	boardType := reflect.TypeOf(model.BoardConfig{})
	describeType := reflect.TypeOf(BoardDescribeInfo{})

	// BoardConfig fields that are intentionally excluded or transformed in BoardDescribeInfo.
	// Every BoardConfig field must appear here or have a matching field in BoardDescribeInfo.
	excluded := map[string]string{
		"ID":            "Internal board ID, not useful in describe output",
		"KanSchema":     "Exposed as 'Schema' (renamed for cleaner output)",
		"Columns":       "Transformed to BoardDescribeColumnInfo (adds CardCount, IsDefault; omits CardIDs)",
		"DefaultColumn": "Surfaced as IsDefault flag on individual columns instead",
	}

	// Fields that exist in BoardDescribeInfo but not in BoardConfig
	describeOnly := map[string]bool{
		"Name":          true, // Same name but verified separately since it's on BoardConfig too
		"Schema":        true, // Renamed from KanSchema
		"DefaultColumn": true, // Also kept at top level in describe output
		"Columns":       true, // Different type (BoardDescribeColumnInfo vs model.Column)
	}

	// Check that all BoardConfig fields are either in BoardDescribeInfo or explicitly excluded
	for i := 0; i < boardType.NumField(); i++ {
		field := boardType.Field(i)
		fieldName := field.Name

		if _, isExcluded := excluded[fieldName]; isExcluded {
			continue
		}

		descField, found := describeType.FieldByName(fieldName)
		if !found {
			t.Errorf("model.BoardConfig has field %q but BoardDescribeInfo does not. "+
				"Either add it to BoardDescribeInfo and printBoardDescribeJson(), "+
				"or add it to the excluded map with a reason.",
				fieldName)
			continue
		}

		if !typesCompatible(field.Type, descField.Type) {
			t.Errorf("Field %q has type %v in model.BoardConfig but %v in BoardDescribeInfo",
				fieldName, field.Type, descField.Type)
		}
	}

	// Check for unexpected extra fields in BoardDescribeInfo
	for i := 0; i < describeType.NumField(); i++ {
		field := describeType.Field(i)
		fieldName := field.Name

		if describeOnly[fieldName] {
			continue
		}

		if _, found := boardType.FieldByName(fieldName); !found {
			t.Errorf("BoardDescribeInfo has field %q that doesn't exist in model.BoardConfig. "+
				"If this is intentional, add it to describeOnly map.",
				fieldName)
		}
	}
}

// TestEmptySlicesNotNull ensures empty slices serialize as [] not null.
func TestEmptySlicesNotNull(t *testing.T) {
	tests := []struct {
		name   string
		output any
		check  string // JSON substring that should be present
	}{
		{
			name:   "empty boards",
			output: NewBoardsOutput(nil),
			check:  `"boards":[]`,
		},
		{
			name:   "empty boards from empty slice",
			output: NewBoardsOutput([]string{}),
			check:  `"boards":[]`,
		},
		{
			name:   "empty columns",
			output: NewColumnsOutput(nil),
			check:  `"columns":[]`,
		},
		{
			name:   "empty cards",
			output: NewListOutput(nil),
			check:  `"cards":[]`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data, err := json.Marshal(tt.output)
			if err != nil {
				t.Fatalf("Marshal failed: %v", err)
			}
			// Check for [] instead of null (json.Marshal omits spaces)
			if !strings.Contains(string(data), tt.check) {
				t.Errorf("Expected JSON to contain %q, got: %s", tt.check, string(data))
			}
			// Also verify it doesn't contain null
			if strings.Contains(string(data), "null") {
				t.Errorf("Expected no null in JSON, got: %s", string(data))
			}
		})
	}
}
