package model

// ColumnColors is a palette of colors for auto-assigning to new columns.
// Colors cycle through this list based on the current column count.
var ColumnColors = []string{
	"#6b7280", // gray
	"#3b82f6", // blue
	"#f59e0b", // amber
	"#10b981", // green
	"#9333ea", // purple
	"#ec4899", // pink
	"#ef4444", // red
	"#06b6d4", // cyan
}

// NextColumnColor returns the next color to use for a new column,
// cycling through the palette based on the current column count.
func NextColumnColor(columnCount int) string {
	return ColumnColors[columnCount%len(ColumnColors)]
}
