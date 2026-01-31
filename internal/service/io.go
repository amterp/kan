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
