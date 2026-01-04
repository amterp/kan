package util

import "time"

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
