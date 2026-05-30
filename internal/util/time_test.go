package util

import "testing"

func TestFormatDuration(t *testing.T) {
	const (
		minute = int64(60_000)
		hour   = 60 * minute
		day    = 24 * hour
		month  = 30 * day
		year   = 365 * day
	)

	cases := []struct {
		name   string
		millis int64
		want   string
	}{
		{"zero is just now", 0, "just now"},
		{"sub-minute is just now", minute - 1, "just now"},
		{"one minute", minute, "1m"},
		{"many minutes", 59 * minute, "59m"},
		{"one hour", hour, "1h"},
		{"many hours", 23 * hour, "23h"},
		{"one day singular", day, "1 day"},
		{"many days", 5 * day, "5 days"},
		{"one month singular", month, "1 month"},
		{"many months", 4 * month, "4 months"},
		{"one year singular", year, "1 year"},
		{"many years", 2 * year, "2 years"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := FormatDuration(tc.millis); got != tc.want {
				t.Errorf("FormatDuration(%d) = %q, want %q", tc.millis, got, tc.want)
			}
		})
	}
}
