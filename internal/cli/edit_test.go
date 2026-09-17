package cli

import (
	"reflect"
	"strings"
	"testing"

	"github.com/amterp/kan/internal/model"
)

func TestParseCustomFields(t *testing.T) {
	tests := []struct {
		name    string
		input   []string
		want    map[string]string
		wantErr bool
	}{
		{
			name:  "empty input",
			input: nil,
			want:  map[string]string{},
		},
		{
			name:  "single key value",
			input: []string{"priority=high"},
			want:  map[string]string{"priority": "high"},
		},
		{
			name:  "multiple distinct keys",
			input: []string{"priority=high", "size=m"},
			want:  map[string]string{"priority": "high", "size": "m"},
		},
		{
			name:  "duplicate key concatenated",
			input: []string{"component=core", "component=cli"},
			want:  map[string]string{"component": "core,cli"},
		},
		{
			name:  "three duplicates preserve order",
			input: []string{"tag=a", "tag=b", "tag=c"},
			want:  map[string]string{"tag": "a,b,c"},
		},
		{
			name:  "duplicate where one value already has comma",
			input: []string{"tags=a,b", "tags=c"},
			want:  map[string]string{"tags": "a,b,c"},
		},
		{
			name:  "whitespace trimmed around key and value",
			input: []string{"  key  =  value  "},
			want:  map[string]string{"key": "value"},
		},
		{
			name:  "value containing equals preserved",
			input: []string{"expr=a=b"},
			want:  map[string]string{"expr": "a=b"},
		},
		{
			name:    "malformed entry without equals",
			input:   []string{"noequals"},
			wantErr: true,
		},
		{
			name:    "empty key",
			input:   []string{"=value"},
			wantErr: true,
		},
		{
			name:    "whitespace-only key",
			input:   []string{"   =value"},
			wantErr: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := parseCustomFields(tc.input)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("expected error, got nil (result=%v)", got)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("parseCustomFields() = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestLongPositionHint(t *testing.T) {
	short := &model.Card{Column: "done", Position: "Zz"}
	if hint := longPositionHint(short); hint != "" {
		t.Errorf("longPositionHint(%q) = %q, want no hint", short.Position, hint)
	}

	long := &model.Card{Column: "done", Position: strings.Repeat("!", 37) + "T"}
	hint := longPositionHint(long)
	for _, want := range []string{`"done"`, "38 characters", "kan doctor --fix"} {
		if !strings.Contains(hint, want) {
			t.Errorf("longPositionHint = %q, want it to contain %q", hint, want)
		}
	}
}
