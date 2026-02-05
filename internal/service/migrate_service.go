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
	globalPlan, err := s.PlanGlobalMigration()
	if err != nil {
		return nil, fmt.Errorf("failed to plan global config migration: %w", err)
	}
	plan.GlobalConfig = globalPlan

	// Plan board migrations
	boardsPlan, err := s.PlanBoardsOnly()
	if err != nil {
		return nil, err
	}
	plan.Boards = boardsPlan.Boards

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

// PlanGlobalMigration analyzes the global config and returns a migration plan.
// Exported for use by --all, which handles global config separately from boards.
func (s *MigrateService) PlanGlobalMigration() (*GlobalMigration, error) {
	return s.planGlobalMigration()
}

// PlanBoardsOnly returns a migration plan with only board migrations (no global config).
// Used by --all to plan each project's boards independently.
func (s *MigrateService) PlanBoardsOnly() (*MigrationPlan, error) {
	plan := &MigrationPlan{}

	boards, err := listBoards(s.paths)
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
	// Determine which migration path to use based on source version
	fromVersion := 0
	if plan.FromSchema != "" {
		v, err := version.ParseBoardVersion(plan.FromSchema)
		if err == nil {
			fromVersion = v
		}
	}

	// v0 or missing → v1: just prepend schema
	// v1 → v2: convert labels to custom fields
	// v2 → v3: just update schema (pattern_hooks is optional, no structural changes)
	if fromVersion == 0 && version.CurrentBoardVersion == 1 {
		return s.prependTOMLField(plan.ConfigPath, "kan_schema", plan.ToSchema)
	}

	if fromVersion <= 1 && version.CurrentBoardVersion >= 2 {
		if err := s.migrateBoardV1ToV2(plan.ConfigPath); err != nil {
			return err
		}
		// If target is v3+, continue to update schema
		if version.CurrentBoardVersion >= 3 {
			return s.updateBoardSchema(plan.ConfigPath, plan.ToSchema)
		}
		return nil
	}

	// v2 → v3: just update schema version (pattern_hooks is optional)
	if fromVersion == 2 && version.CurrentBoardVersion == 3 {
		return s.updateBoardSchema(plan.ConfigPath, plan.ToSchema)
	}

	// Fallback for unknown versions: just update the schema
	return s.updateBoardSchema(plan.ConfigPath, plan.ToSchema)
}

// updateBoardSchema updates only the kan_schema field in a board config file.
func (s *MigrateService) updateBoardSchema(path, newSchema string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}

	var raw map[string]any
	if _, err := toml.Decode(string(data), &raw); err != nil {
		return fmt.Errorf("invalid TOML: %w", err)
	}

	raw["kan_schema"] = newSchema
	return writeTOMLMap(path, raw)
}

// migrateBoardV1ToV2 converts a v1 board config to v2 format.
// This involves converting [[labels]] to [custom_fields.labels] with type="tags"
// and adding a [card_display] section.
func (s *MigrateService) migrateBoardV1ToV2(path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}

	var raw map[string]any
	if _, err := toml.Decode(string(data), &raw); err != nil {
		return fmt.Errorf("invalid TOML: %w", err)
	}

	// Update schema version
	raw["kan_schema"] = version.CurrentBoardSchema()

	// Convert labels to custom field
	hasLabels := false
	if labels, ok := raw["labels"].([]map[string]any); ok && len(labels) > 0 {
		hasLabels = true
		options := make([]map[string]any, 0, len(labels))
		for _, lbl := range labels {
			opt := map[string]any{
				"value": lbl["name"],
			}
			if color, ok := lbl["color"].(string); ok && color != "" {
				opt["color"] = color
			}
			options = append(options, opt)
		}

		// Create or update custom_fields
		customFields, _ := raw["custom_fields"].(map[string]any)
		if customFields == nil {
			customFields = make(map[string]any)
		}
		customFields["labels"] = map[string]any{
			"type":    "tags",
			"options": options,
		}
		raw["custom_fields"] = customFields

		// Remove old labels section
		delete(raw, "labels")
	}

	// Add card_display if we had labels
	if hasLabels {
		if _, ok := raw["card_display"]; !ok {
			raw["card_display"] = map[string]any{
				"badges": []string{"labels"},
			}
		}
	}

	return writeTOMLMap(path, raw)
}

// writeTOML writes a map to a TOML file.

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

	return writeJSONMap(plan.Path, raw)
}
