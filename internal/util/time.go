package util

import (
	"fmt"
	"time"
)

// NowMillis returns the current time in milliseconds since Unix epoch.
func NowMillis() int64 {
	return time.Now().UnixMilli()
}

// MillisToTime converts milliseconds since Unix epoch to time.Time.
func MillisToTime(millis int64) time.Time {
	return time.UnixMilli(millis)
}

// FormatTime formats a time in a human-readable way.
func FormatTime(t time.Time) string {
	return t.Format("2006-01-02 15:04")
}

// FormatMillis formats milliseconds since epoch in a human-readable way.
func FormatMillis(millis int64) string {
	return FormatTime(MillisToTime(millis))
}

// FormatDuration renders a millisecond duration as a coarse, single-unit,
// at-a-glance string (e.g. "just now", "5m", "3h", "2 days", "4 months").
// Used for "how long in this column" displays where precision isn't the point.
func FormatDuration(millis int64) string {
	const (
		minute = int64(60_000)
		hour   = 60 * minute
		day    = 24 * hour
		month  = 30 * day
		year   = 365 * day
	)

	switch {
	case millis < minute:
		return "just now"
	case millis < hour:
		return fmt.Sprintf("%dm", millis/minute)
	case millis < day:
		return fmt.Sprintf("%dh", millis/hour)
	case millis < month:
		return pluralizeUnit(millis/day, "day")
	case millis < year:
		return pluralizeUnit(millis/month, "month")
	default:
		return pluralizeUnit(millis/year, "year")
	}
}

func pluralizeUnit(n int64, unit string) string {
	if n == 1 {
		return fmt.Sprintf("1 %s", unit)
	}
	return fmt.Sprintf("%d %ss", n, unit)
}
