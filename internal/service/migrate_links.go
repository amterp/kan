package service

import (
	"fmt"
	"strings"
)

func rawCardVersion(raw map[string]any) int {
	switch value := raw["_v"].(type) {
	case float64:
		return int(value)
	case int:
		return value
	default:
		return 0
	}
}

func migrateLinksRaw(b *rawBoard) ([]string, bool, error) {
	definitions, _ := b.config["custom_fields"].(map[string]any)
	var renames []string
	for _, name := range []string{"blocked_by", "related"} {
		_, collision := definitions[name]
		for _, card := range b.cards {
			if rawCardVersion(card.data) < 4 {
				if _, exists := card.data[name]; exists {
					collision = true
				}
			}
		}
		if !collision {
			continue
		}
		destination := "x_" + name
		if _, exists := definitions[destination]; exists {
			return nil, false, fmt.Errorf("board %q: cannot rename custom field %s to %s: destination already exists", b.name, name, destination)
		}
		for _, card := range b.cards {
			if _, exists := card.data[destination]; exists {
				return nil, false, fmt.Errorf("board %q: cannot rename custom field %s to %s: destination already exists in a card", b.name, name, destination)
			}
		}
		renames = append(renames, name)
	}

	configChanged := false
	for _, name := range renames {
		destination := "x_" + name
		if value, exists := definitions[name]; exists {
			definitions[destination] = value
			delete(definitions, name)
			configChanged = true
		}
		if display, ok := b.config["card_display"].(map[string]any); ok {
			for key, value := range display {
				if replacement, changed := renameDisplayReference(value, name, destination); changed {
					display[key] = replacement
					configChanged = true
				}
			}
		}
		if ready, ok := b.config["ready"].(map[string]any); ok {
			if values, ok := ready["exclude"].([]any); ok {
				for i, value := range values {
					if expression, ok := value.(string); ok {
						if key, _, found := strings.Cut(expression, "="); found && strings.TrimSpace(key) == name {
							values[i] = strings.Replace(key, name, destination, 1) + expression[len(key):]
							configChanged = true
						}
					}
				}
			}
		}
		for _, card := range b.cards {
			if rawCardVersion(card.data) >= 4 {
				continue
			}
			if value, exists := card.data[name]; exists {
				card.data[destination] = value
				delete(card.data, name)
				card.changed = true
			}
		}
	}

	ids := map[string]bool{}
	aliases := map[string]string{}
	for _, card := range b.cards {
		id, _ := card.data["id"].(string)
		if id == "" {
			continue
		}
		ids[id] = true
		if alias, ok := card.data["alias"].(string); ok && alias != "" {
			if _, duplicate := aliases[alias]; duplicate {
				aliases[alias] = ""
			} else {
				aliases[alias] = id
			}
		}
	}
	for _, card := range b.cards {
		if rawCardVersion(card.data) >= 4 {
			continue
		}
		parent, _ := card.data["parent"].(string)
		if parent != "" && !ids[parent] && aliases[parent] != "" {
			card.data["parent"] = aliases[parent]
			card.changed = true
		}
	}
	return renames, configChanged, nil
}

func renameDisplayReference(value any, oldName, newName string) (any, bool) {
	switch value := value.(type) {
	case string:
		if value == oldName {
			return newName, true
		}
	case []any:
		changed := false
		for i, item := range value {
			if name, ok := item.(string); ok && name == oldName {
				value[i] = newName
				changed = true
			}
		}
		return value, changed
	case []string:
		changed := false
		for i, item := range value {
			if item == oldName {
				value[i] = newName
				changed = true
			}
		}
		return value, changed
	}
	return value, false
}
