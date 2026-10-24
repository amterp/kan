package model

import "testing"

func TestParseFilter(t *testing.T) {
	cases := []struct {
		expr    string
		want    Filter
		wantErr bool
	}{
		{expr: "labels=needs-decision", want: Filter{Key: "labels", Value: "needs-decision"}},
		{expr: " type = bug ", want: Filter{Key: "type", Value: "bug"}},
		{expr: "expr=a=b", want: Filter{Key: "expr", Value: "a=b"}},
		{expr: "labels", wantErr: true},
		{expr: "=bug", wantErr: true},
		{expr: "type=", wantErr: true},
		{expr: "", wantErr: true},
	}
	for _, tc := range cases {
		t.Run(tc.expr, func(t *testing.T) {
			got, err := ParseFilter(tc.expr)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("ParseFilter(%q) = %+v, want error", tc.expr, got)
				}
				return
			}
			if err != nil {
				t.Fatalf("ParseFilter(%q) error: %v", tc.expr, err)
			}
			if got != tc.want {
				t.Errorf("ParseFilter(%q) = %+v, want %+v", tc.expr, got, tc.want)
			}
		})
	}
}
