package service

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/BurntSushi/toml"
	"github.com/amterp/kan/internal/config"
	"github.com/amterp/kan/internal/version"
)

// MigrateService handles schema migrations for Kan data files.
// Uses raw file I/O to bypass store validation.
type MigrateService struct {
	paths *config.Paths
}

// NewMigrateService creates a new migration service.
func NewMigrateService(paths *config.Paths) *MigrateService {
	return &MigrateService{paths: paths}
}

// MigrationPlan describes what changes would be made during migration.
type MigrationPlan struct {
	GlobalConfig *GlobalMigration
	Boards       []BoardMigration
}

// GlobalMigration describes changes to the global config.
type GlobalMigration struct {
	Path           string
	NeedsMigration bool
	FromSchema     string // empty if missing
	ToSchema       string
}

// BoardMigration describes changes to a board.
type BoardMigration struct {
	BoardName      string
	ConfigPath     string
	NeedsMigration bool
	FromSchema     string // empty if missing
	ToSchema       string
	Cards          []CardMigration
}

// CardMigration describes changes to a card.
type CardMigration struct {
	CardID       string
	Path         string
	FromVersion  int // 0 if missing
	ToVersion    int
	RemoveColumn bool
}

// Plan analyzes the current state and returns a migration plan.
func (s *MigrateService) Plan() (*MigrationPlan, error) {
	plan := &MigrationPlan{}

	// Plan global config migration
	globalPlan, err := s.planGlobalMigration()
	if err != nil {
		return nil, fmt.Errorf("failed to plan global config migration: %w", err)
	}
	plan.GlobalConfig = globalPlan

	// Plan board migrations
	boards, err := s.listBoards()
	if err != nil {
		return nil, fmt.Errorf("failed to list boards: %w", err)
	}

	for _, boardName := range boards {
		boardPlan, err := s.planBoardMigration(boardName)
		if err != nil {
			return nil, fmt.Errorf("failed to plan migration for board %q: %w", boardName, err)
		}
		plan.Boards = append(plan.Boards, *boardPlan)
	}

	return plan, nil
}

// Execute performs the migration.
func (s *MigrateService) Execute(plan *MigrationPlan, dryRun bool) error {
	// Migrate global config
	if plan.GlobalConfig != nil && plan.GlobalConfig.NeedsMigration {
		if dryRun {
			fmt.Printf("Would migrate global config: add kan_schema = %q\n", plan.GlobalConfig.ToSchema)
		} else {
			if err := s.migrateGlobalConfig(plan.GlobalConfig); err != nil {
				return fmt.Errorf("failed to migrate global config: %w", err)
			}
			fmt.Printf("Migrated global config\n")
		}
	}

	// Migrate boards
	for _, board := range plan.Boards {
		cardsToMigrate := 0
		for _, card := range board.Cards {
			if card.FromVersion != card.ToVersion || card.RemoveColumn {
				cardsToMigrate++
			}
		}

		if dryRun {
			if board.NeedsMigration {
				fmt.Printf("Would migrate board %q config: add kan_schema = %q\n", board.BoardName, board.ToSchema)
			}
			if cardsToMigrate > 0 {
				fmt.Printf("Would migrate %d cards in board %q: add _v=%d, remove column\n",
					cardsToMigrate, board.BoardName, version.CurrentCardVersion)
			}
		} else {
			if board.NeedsMigration {
				if err := s.migrateBoardConfig(&board); err != nil {
					return fmt.Errorf("failed to migrate board %q config: %w", board.BoardName, err)
				}
			}

			for _, card := range board.Cards {
				if card.FromVersion == card.ToVersion && !card.RemoveColumn {
					continue
				}
				if err := s.migrateCard(&card); err != nil {
					return fmt.Errorf("failed to migrate card %q: %w", card.CardID, err)
				}
			}

			if board.NeedsMigration || cardsToMigrate > 0 {
				fmt.Printf("Migrated board %q", board.BoardName)
				if board.NeedsMigration {
					fmt.Printf(" (config")
					if cardsToMigrate > 0 {
						fmt.Printf(" + %d cards", cardsToMigrate)
					}
					fmt.Printf(")")
				} else if cardsToMigrate > 0 {
					fmt.Printf(" (%d cards)", cardsToMigrate)
				}
				fmt.Println()
			}
		}
	}

	return nil
}

// HasChanges returns true if the plan has any migrations to perform.
func (p *MigrationPlan) HasChanges() bool {
	if p.GlobalConfig != nil && p.GlobalConfig.NeedsMigration {
		return true
	}
	for _, board := range p.Boards {
		if board.NeedsMigration {
			return true
		}
		for _, card := range board.Cards {
			if card.FromVersion != card.ToVersion || card.RemoveColumn {
				return true
			}
		}
	}
	return false
}

func (s *MigrateService) planGlobalMigration() (*GlobalMigration, error) {
	path := config.GlobalConfigPath()
	if path == "" {
		return nil, nil
	}

	plan := &GlobalMigration{
		Path:     path,
		ToSchema: version.CurrentGlobalSchema(),
	}

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil // No global config to migrate
		}
		return nil, err
	}

	var raw map[string]any
	if _, err := toml.Decode(string(data), &raw); err != nil {
		return nil, fmt.Errorf("invalid TOML: %w", err)
	}

	if schema, ok := raw["kan_schema"].(string); ok {
		plan.FromSchema = schema
		plan.NeedsMigration = schema != version.CurrentGlobalSchema()
	} else {
		plan.FromSchema = ""
		plan.NeedsMigration = true
	}

	return plan, nil
}

func (s *MigrateService) planBoardMigration(boardName string) (*BoardMigration, error) {
	plan := &BoardMigration{
		BoardName:  boardName,
		ConfigPath: s.paths.BoardConfigPath(boardName),
		ToSchema:   version.CurrentBoardSchema(),
	}

	// Check board config
	data, err := os.ReadFile(plan.ConfigPath)
	if err != nil {
		return nil, err
	}

	var raw map[string]any
	if _, err := toml.Decode(string(data), &raw); err != nil {
		return nil, fmt.Errorf("invalid TOML: %w", err)
	}

	if schema, ok := raw["kan_schema"].(string); ok {
		plan.FromSchema = schema
		plan.NeedsMigration = schema != version.CurrentBoardSchema()
	} else {
		plan.FromSchema = ""
		plan.NeedsMigration = true
	}

	// Check cards
	cardsDir := s.paths.CardsDir(boardName)
	entries, err := os.ReadDir(cardsDir)
	if err != nil {
		if os.IsNotExist(err) {
			return plan, nil // No cards directory
		}
		return nil, err
	}

	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}

		cardPath := filepath.Join(cardsDir, entry.Name())
		cardPlan, err := s.planCardMigration(cardPath)
		if err != nil {
			return nil, fmt.Errorf("failed to plan migration for card %s: %w", entry.Name(), err)
		}
		plan.Cards = append(plan.Cards, *cardPlan)
	}

	return plan, nil
}

func (s *MigrateService) planCardMigration(path string) (*CardMigration, error) {
	plan := &CardMigration{
		Path:      path,
		ToVersion: version.CurrentCardVersion,
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var raw map[string]any
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, fmt.Errorf("invalid JSON: %w", err)
	}

	// Extract card ID
	if id, ok := raw["id"].(string); ok {
		plan.CardID = id
	} else {
		plan.CardID = filepath.Base(path)
	}

	// Check version
	if v, ok := raw["_v"].(float64); ok {
		plan.FromVersion = int(v)
	} else {
		plan.FromVersion = 0
	}

	// Check for column field
	if _, hasColumn := raw["column"]; hasColumn {
		plan.RemoveColumn = true
	}

	return plan, nil
}

func (s *MigrateService) migrateGlobalConfig(plan *GlobalMigration) error {
	// Prepend kan_schema to preserve existing file formatting
	return s.prependTOMLField(plan.Path, "kan_schema", plan.ToSchema)
}

func (s *MigrateService) migrateBoardConfig(plan *BoardMigration) error {
	// Prepend kan_schema to preserve existing file formatting
	return s.prependTOMLField(plan.ConfigPath, "kan_schema", plan.ToSchema)
}

// prependTOMLField adds a field at the top of a TOML file, preserving existing formatting.
// This avoids scrambling field order that would happen with decode/encode round-trip.
func (s *MigrateService) prependTOMLField(path, key, value string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}

	// Prepend the new field with a blank line separator
	newContent := fmt.Sprintf("%s = %q\n\n%s", key, value, string(data))

	return os.WriteFile(path, []byte(newContent), 0644)
}

func (s *MigrateService) migrateCard(plan *CardMigration) error {
	data, err := os.ReadFile(plan.Path)
	if err != nil {
		return err
	}

	var raw map[string]any
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}

	// Add/update version
	raw["_v"] = plan.ToVersion

	// Remove column field
	delete(raw, "column")

	return s.writeJSON(plan.Path, raw)
}

func (s *MigrateService) listBoards() ([]string, error) {
	boardsRoot := s.paths.BoardsRoot()

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
			configPath := s.paths.BoardConfigPath(entry.Name())
			if _, err := os.Stat(configPath); err == nil {
				boards = append(boards, entry.Name())
			}
		}
	}

	return boards, nil
}

func (s *MigrateService) writeJSON(path string, data map[string]any) error {
	output, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, output, 0644)
}
