package service

import (
	"encoding/json"
	"os"

	"github.com/BurntSushi/toml"
	"github.com/amterp/kan/internal/config"
	"github.com/amterp/kan/internal/model"
)

// writeTOMLMap writes a map to a TOML file.
func writeTOMLMap(path string, data map[string]any) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()

	encoder := toml.NewEncoder(f)
	return encoder.Encode(data)
}

// writeBoardConfig writes a BoardConfig to a TOML file.
func writeBoardConfig(path string, cfg *model.BoardConfig) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()

	encoder := toml.NewEncoder(f)
	return encoder.Encode(cfg)
}

// writeJSONMap writes a map to an indented JSON file.
func writeJSONMap(path string, data map[string]any) error {
	output, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, output, 0644)
}

// writeCardMap writes a migrated card map to disk in the same on-disk format
// the store uses (model.Card.MarshalFile: pretty-printed, one compact line per
// history entry). This keeps migrated cards byte-identical to store-written
// ones, so a migration doesn't produce verbose multi-line history that then
// reformats on the card's next edit. Falls back to writeJSONMap if the map
// can't be round-tripped through model.Card (e.g. malformed data mid-migration).
func writeCardMap(path string, data map[string]any) error {
	raw, err := json.Marshal(data)
	if err != nil {
		return writeJSONMap(path, data)
	}
	var card model.Card
	if err := json.Unmarshal(raw, &card); err != nil {
		return writeJSONMap(path, data)
	}
	output, err := card.MarshalFile()
	if err != nil {
		return writeJSONMap(path, data)
	}
	return os.WriteFile(path, output, 0644)
}

// listBoards returns the names of all boards in the given paths.
func listBoards(paths *config.Paths) ([]string, error) {
	boardsRoot := paths.BoardsRoot()

	entries, err := os.ReadDir(boardsRoot)
	if err != nil {
		if os.IsNotExist(err) {
			return []string{}, nil
		}
		return nil, err
	}

	var boards []string
	for _, entry := range entries {
		if entry.IsDir() {
			configPath := paths.BoardConfigPath(entry.Name())
			if _, err := os.Stat(configPath); err == nil {
				boards = append(boards, entry.Name())
			}
		}
	}

	return boards, nil
}
