package cli

import (
	"fmt"
	"os"

	"github.com/charmbracelet/lipgloss"
)

// Adaptive colors that work in both light and dark terminals.
// First value is for dark terminals, second for light terminals.
var (
	ColorSuccess = lipgloss.AdaptiveColor{Dark: "#22c55e", Light: "#16a34a"} // green
	ColorError   = lipgloss.AdaptiveColor{Dark: "#ef4444", Light: "#dc2626"} // red
	ColorWarning = lipgloss.AdaptiveColor{Dark: "#f59e0b", Light: "#d97706"} // amber
	ColorInfo    = lipgloss.AdaptiveColor{Dark: "#3b82f6", Light: "#2563eb"} // blue
	ColorMuted   = lipgloss.AdaptiveColor{Dark: "#6b7280", Light: "#9ca3af"} // gray
	ColorAccent  = lipgloss.AdaptiveColor{Dark: "#a78bfa", Light: "#7c3aed"} // purple for IDs
	ColorURL     = lipgloss.AdaptiveColor{Dark: "#38bdf8", Light: "#0284c7"} // cyan for URLs
)

// Reusable text styles
var (
	StyleSuccess = lipgloss.NewStyle().Foreground(ColorSuccess)
	StyleError   = lipgloss.NewStyle().Foreground(ColorError)
	StyleWarning = lipgloss.NewStyle().Foreground(ColorWarning)
	StyleInfo    = lipgloss.NewStyle().Foreground(ColorInfo)
	StyleMuted   = lipgloss.NewStyle().Foreground(ColorMuted)
	StyleID      = lipgloss.NewStyle().Foreground(ColorAccent)
	StyleURL     = lipgloss.NewStyle().Foreground(ColorURL)
	StyleBold    = lipgloss.NewStyle().Bold(true)
)

// Icons for status messages
const (
	IconSuccess = "✓"
	IconError   = "✗"
	IconWarning = "!"
	IconInfo    = "→"
)

// PrintSuccess prints a success message with a green checkmark.
func PrintSuccess(format string, args ...any) {
	icon := StyleSuccess.Render(IconSuccess)
	msg := fmt.Sprintf(format, args...)
	fmt.Printf("%s %s\n", icon, msg)
}

// PrintError prints an error message with a red X to stderr.
func PrintError(format string, args ...any) {
	icon := StyleError.Render(IconError)
	msg := fmt.Sprintf(format, args...)
	fmt.Fprintf(os.Stderr, "%s %s\n", icon, msg)
}

// PrintWarning prints a warning message with an amber icon to stderr.
func PrintWarning(format string, args ...any) {
	icon := StyleWarning.Render(IconWarning)
	msg := fmt.Sprintf(format, args...)
	fmt.Fprintf(os.Stderr, "%s %s\n", icon, msg)
}

// PrintInfo prints an info message with a blue arrow.
func PrintInfo(format string, args ...any) {
	icon := StyleMuted.Render(IconInfo)
	msg := fmt.Sprintf(format, args...)
	fmt.Printf("%s %s\n", icon, msg)
}

// RenderID renders a card/comment ID in accent color.
func RenderID(id string) string {
	return StyleID.Render(id)
}

// RenderURL renders a URL in the URL color.
func RenderURL(url string) string {
	return StyleURL.Render(url)
}

// RenderMuted renders text in muted color.
func RenderMuted(text string) string {
	return StyleMuted.Render(text)
}

// RenderBold renders text in bold.
func RenderBold(text string) string {
	return StyleBold.Render(text)
}

// RenderColumnColor renders text in the given hex color.
// Falls back to muted if color is empty or invalid.
func RenderColumnColor(text, hexColor string) string {
	if hexColor == "" {
		return StyleMuted.Render(text)
	}
	return lipgloss.NewStyle().Foreground(lipgloss.Color(hexColor)).Render(text)
}

// ColorSwatch renders a small color swatch block in the given hex color.
func ColorSwatch(hexColor string) string {
	if hexColor == "" {
		return StyleMuted.Render("██")
	}
	return lipgloss.NewStyle().Foreground(lipgloss.Color(hexColor)).Render("██")
}

// Box renders content in a bordered box.
func Box(content string) string {
	style := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(ColorMuted).
		Padding(0, 1)
	return style.Render(content)
}

// TitleBox renders a title in a prominent bordered box.
func TitleBox(title string) string {
	style := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(ColorAccent).
		Padding(0, 2).
		Bold(true)
	return style.Render(title)
}

// LabelValue formats a label-value pair with right-aligned label.
func LabelValue(label, value string, labelWidth int) string {
	labelStyle := lipgloss.NewStyle().
		Width(labelWidth).
		Align(lipgloss.Right).
		Foreground(ColorMuted)
	return fmt.Sprintf("%s %s", labelStyle.Render(label+":"), value)
}
