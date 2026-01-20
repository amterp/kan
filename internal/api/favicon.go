package api

import (
	"fmt"
	"html"
	"net/http"
	"os"

	"github.com/amterp/kan/internal/model"
)

// GenerateFaviconSVG creates an SVG favicon from the favicon config.
func GenerateFaviconSVG(cfg *model.FaviconConfig) string {
	bg := cfg.Background
	if bg == "" {
		bg = "#3b82f6" // Default blue
	}

	var content string
	if cfg.IconType == model.IconTypeEmoji && cfg.Emoji != "" {
		// Emoji variant - larger font, centered
		content = fmt.Sprintf(
			`<text x="50%%" y="50%%" dominant-baseline="central" text-anchor="middle" font-size="20">%s</text>`,
			html.EscapeString(cfg.Emoji),
		)
	} else {
		// Letter variant (default)
		letter := cfg.Letter
		if letter == "" {
			letter = "K"
		}
		content = fmt.Sprintf(
			`<text x="50%%" y="50%%" dominant-baseline="central" text-anchor="middle" fill="white" font-family="system-ui, -apple-system, sans-serif" font-weight="600" font-size="20">%s</text>`,
			html.EscapeString(letter),
		)
	}

	return fmt.Sprintf(
		`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 32 32"><rect width="32" height="32" rx="6" fill="%s"/>%s</svg>`,
		bg, content,
	)
}

// GetFavicon serves the favicon, checking for a custom file first.
func (h *Handler) GetFavicon(w http.ResponseWriter, r *http.Request) {
	// Check for custom favicon first
	customPath := h.paths.CustomFaviconPath()
	if data, err := os.ReadFile(customPath); err == nil {
		w.Header().Set("Content-Type", "image/svg+xml")
		w.Header().Set("Cache-Control", "public, max-age=3600")
		w.Write(data)
		return
	}

	// Generate dynamic favicon from project config
	cfg, err := h.projectStore.Load()
	if err != nil {
		// Fallback to default favicon on error
		cfg = &model.ProjectConfig{
			Favicon: model.DefaultFaviconConfig("", "Kan"),
		}
	}

	// If no favicon config, use defaults
	if cfg.Favicon.Background == "" {
		cfg.Favicon = model.DefaultFaviconConfig(cfg.ID, cfg.Name)
	}

	svg := GenerateFaviconSVG(&cfg.Favicon)

	w.Header().Set("Content-Type", "image/svg+xml")
	w.Header().Set("Cache-Control", "public, max-age=3600")
	w.Write([]byte(svg))
}
