package service

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/BurntSushi/toml"
	"github.com/amterp/kan/internal/config"
	"github.com/amterp/kan/internal/util"
	"github.com/amterp/kan/internal/version"
)

// MigrateService handles schema migrations for Kan data files.
// Uses raw file I/O to bypass store validation. Output controls where
// progress messages are written (os.Stdout for interactive use,
// io.Discard for auto-migration).
type MigrateService struct {
	paths  *config.Paths
	output io.Writer
}

// NewMigrateService creates a new migration service that writes progress
// to stdout.
func NewMigrateService(paths *config.Paths) *MigrateService {
	return &MigrateService{paths: paths, output: os.Stdout}
}

// NewQuietMigrateService creates a migration service that suppresses
// progress output. Used by auto-migration, which prints its own summary.
func NewQuietMigrateService(paths *config.Paths) *MigrateService {
	return &MigrateService{paths: paths, output: io.Discard}
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

// FromVersion returns the numeric board version from FromSchema, or 0 if missing/unparseable.
func (b *BoardMigration) FromVersion() int {
	if b.FromSchema == "" {
		return 0
	}
	v, err := version.ParseBoardVersion(b.FromSchema)
	if err != nil {
		return 0
	}
	return v
}

// ToVersion returns the numeric board version from ToSchema, or 0 if unparseable.
func (b *BoardMigration) ToVersion() int {
	v, err := version.ParseBoardVersion(b.ToSchema)
	if err != nil {
		return 0
	}
	return v
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

// Execute performs the migration. Progress output is written to s.output.
func (s *MigrateService) Execute(plan *MigrationPlan, dryRun bool) error {
	w := s.output

	// Migrate global config
	if plan.GlobalConfig != nil && plan.GlobalConfig.NeedsMigration {
		if dryRun {
			// A config with an existing schema is updated in place; only a
			// pre-schema (legacy) config has the field added.
			verb := "update"
			if plan.GlobalConfig.FromSchema == "" {
				verb = "add"
			}
			fmt.Fprintf(w, "Would migrate global config: %s kan_schema = %q\n", verb, plan.GlobalConfig.ToSchema)
		} else {
			if err := s.migrateGlobalConfig(plan.GlobalConfig); err != nil {
				return fmt.Errorf("failed to migrate global config: %w", err)
			}
			fmt.Fprintf(w, "Migrated global config\n")
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
				fmt.Fprintf(w, "Would migrate board %q config: add kan_schema = %q\n", board.BoardName, board.ToSchema)
			}
			if cardsToMigrate > 0 {
				fmt.Fprintf(w, "Would migrate %d cards in board %q to _v=%d\n",
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
				fmt.Fprintf(w, "Migrated board %q", board.BoardName)
				if board.NeedsMigration {
					fmt.Fprintf(w, " (config")
					if cardsToMigrate > 0 {
						fmt.Fprintf(w, " + %d cards", cardsToMigrate)
					}
					fmt.Fprintf(w, ")")
				} else if cardsToMigrate > 0 {
					fmt.Fprintf(w, " (%d cards)", cardsToMigrate)
				}
				fmt.Fprintln(w)
			}
		}
	}

	return nil
}

// FutureVersionError checks if any files in the plan have a schema version
// newer than this binary supports. Returns the first future-version error
// found, or nil if all versions are current or older. Callers should check
// this before Execute() to avoid silently downgrading data.
func (p *MigrationPlan) FutureVersionError() error {
	if p.GlobalConfig != nil && p.GlobalConfig.FromSchema != "" {
		if version.IsFutureGlobalSchema(p.GlobalConfig.FromSchema) {
			return version.InvalidGlobalSchema(p.GlobalConfig.Path, p.GlobalConfig.FromSchema)
		}
	}
	for _, board := range p.Boards {
		if board.FromSchema != "" && version.IsFutureBoardSchema(board.FromSchema) {
			return version.InvalidBoardSchema(board.ConfigPath, board.FromSchema)
		}
		for _, card := range board.Cards {
			if version.IsFutureCardVersion(card.FromVersion) {
				return version.InvalidCardVersion(card.Path, card.FromVersion, version.CurrentCardVersion)
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

	// v0->v1: remove column field (old migration)
	if plan.FromVersion == 0 {
		if _, hasColumn := raw["column"]; hasColumn {
			plan.RemoveColumn = true
		}
	}

	return plan, nil
}

func (s *MigrateService) migrateGlobalConfig(plan *GlobalMigration) error {
	// global/1 -> global/2 and onward: bumping the schema is a no-op transform
	// (the global_board field is purely additive). When the file already declares
	// a schema, update it in place; prepending would create a duplicate
	// kan_schema key and break TOML decoding. Only the pre-schema case (no
	// FromSchema) prepends, to preserve formatting of legacy configs.
	if plan.FromSchema != "" {
		return s.updateTOMLSchema(plan.Path, plan.ToSchema)
	}
	return s.prependTOMLField(plan.Path, "kan_schema", plan.ToSchema)
}

// updateTOMLSchema rewrites a TOML file's kan_schema value in place, preserving
// all other fields.
func (s *MigrateService) updateTOMLSchema(path, newSchema string) error {
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

func (s *MigrateService) migrateBoardConfig(plan *BoardMigration) error {
	// Determine which migration path to use based on source version.
	// Each step MUST update fromVersion after success to allow chaining
	// to the next migration. Using "return nil" instead will silently
	// break migration for any version before the current one.
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
	// v3 → v4: just update schema (wanted fields are optional)
	// v4 → v5: rename tags to enum-set
	if fromVersion == 0 && version.CurrentBoardVersion == 1 {
		return s.prependTOMLField(plan.ConfigPath, "kan_schema", plan.ToSchema)
	}

	if fromVersion <= 1 && version.CurrentBoardVersion >= 2 {
		if err := s.migrateBoardV1ToV2(plan.ConfigPath); err != nil {
			return err
		}
		fromVersion = 2
	}

	// v2/v3 → v4: schema-only update
	if fromVersion >= 2 && fromVersion <= 3 && version.CurrentBoardVersion >= 4 {
		if err := s.updateBoardSchema(plan.ConfigPath, version.FormatBoardSchema(4)); err != nil {
			return err
		}
		fromVersion = 4
	}

	// v4 → v5: rename tags to enum-set
	if fromVersion == 4 && version.CurrentBoardVersion >= 5 {
		if err := s.migrateBoardV4ToV5(plan.ConfigPath); err != nil {
			return err
		}
		fromVersion = 5
	}

	// v5 → v6: schema-only (adds optional description fields to custom fields/options)
	if fromVersion == 5 && version.CurrentBoardVersion >= 6 {
		if err := s.updateBoardSchema(plan.ConfigPath, version.FormatBoardSchema(6)); err != nil {
			return err
		}
		fromVersion = 6
	}

	// v6 → v7: schema-only (adds optional description field to columns)
	if fromVersion == 6 && version.CurrentBoardVersion >= 7 {
		if err := s.updateBoardSchema(plan.ConfigPath, version.FormatBoardSchema(7)); err != nil {
			return err
		}
		fromVersion = 7
	}

	// v7 → v8: schema-only (adds optional limit field to columns)
	if fromVersion == 7 && version.CurrentBoardVersion >= 8 {
		if err := s.updateBoardSchema(plan.ConfigPath, version.FormatBoardSchema(8)); err != nil {
			return err
		}
		fromVersion = 8
	}

	// v8 → v9: schema-only (adds boolean custom field type)
	if fromVersion == 8 && version.CurrentBoardVersion >= 9 {
		if err := s.updateBoardSchema(plan.ConfigPath, version.FormatBoardSchema(9)); err != nil {
			return err
		}
		fromVersion = 9
	}

	if fromVersion == 9 && version.CurrentBoardVersion >= 10 {
		if err := s.migrateBoardV9ToV10(plan); err != nil {
			return err
		}
		fromVersion = 10
	}

	// Fallback for unknown versions: just update the schema
	if fromVersion < version.CurrentBoardVersion {
		return s.updateBoardSchema(plan.ConfigPath, plan.ToSchema)
	}

	return nil
}

// updateBoardSchema updates only the kan_schema field in a board config file.
func (s *MigrateService) updateBoardSchema(path, newSchema string) error {
	return s.updateTOMLSchema(path, newSchema)
}

// migrateBoardV1ToV2 converts a v1 board config to v2 format.
// This involves converting [[labels]] to [custom_fields.labels] with type="tags"
// and adding a [card_display] section.
// Note: "tags" is the historical v2 type name. Later migrations (v4->v5)
// rename it to "enum-set".
func (s *MigrateService) migrateBoardV1ToV2(path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}

	var raw map[string]any
	if _, err := toml.Decode(string(data), &raw); err != nil {
		return fmt.Errorf("invalid TOML: %w", err)
	}

	// Set to v2 - subsequent migrations will bump further
	raw["kan_schema"] = version.FormatBoardSchema(2)

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

// migrateBoardV4ToV5 renames custom field type "tags" to "enum-set".
func (s *MigrateService) migrateBoardV4ToV5(path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}

	var raw map[string]any
	if _, err := toml.Decode(string(data), &raw); err != nil {
		return fmt.Errorf("invalid TOML: %w", err)
	}

	// Update schema version to v5 (not CurrentBoardSchema - each step writes its own target)
	raw["kan_schema"] = version.FormatBoardSchema(5)

	// Rename type = "tags" to type = "enum-set" in custom_fields
	if customFields, ok := raw["custom_fields"].(map[string]any); ok {
		for _, fieldDef := range customFields {
			if fieldMap, ok := fieldDef.(map[string]any); ok {
				if fieldMap["type"] == "tags" {
					fieldMap["type"] = "enum-set"
				}
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

	// Seed column history (card/3) if absent. Done before the already-at-target
	// short-circuit below: the v9->v10 board migration runs first and can stamp
	// _v to the current version (via writeCardColumnPosition) without seeding
	// history, so the version delta alone can't tell us whether a card still
	// needs its history seeded.
	seeded := seedCardHistory(raw)

	// Check if already at target version (e.g., v9->v10 board migration already
	// bumped card files via writeCardColumnPosition). Still persist if we just
	// seeded history.
	if v, ok := raw["_v"].(float64); ok && int(v) == plan.ToVersion {
		if seeded {
			return writeCardMap(plan.Path, raw)
		}
		return nil
	}

	// Add/update version
	raw["_v"] = plan.ToVersion

	// v0->v1: remove column field (moved to board config)
	if plan.RemoveColumn {
		delete(raw, "column")
	}

	return writeCardMap(plan.Path, raw)
}

// seedCardHistory adds an initial column history entry (card/3) to a card map
// if history is absent, returning true if it modified the map. It is
// idempotent, so it is safe to call on cards that already carry history.
//
// This is necessarily an approximation for pre-existing cards: the card file
// records only the current column, not when each past transition happened, so
// we record the current column as if it had been held since the card's
// creation. A card created in "backlog" and later moved to "review" will, after
// migration, show a single {column: review, at: created_at} entry - its
// current-column duration may be overstated and its earlier journey is
// collapsed. History is accurate from migration forward.
func seedCardHistory(raw map[string]any) bool {
	// Treat a present-but-empty history (null or []) as absent so a card that
	// was hand-edited or written by an external tool still gets seeded.
	if h, has := raw["history"]; has && h != nil {
		if arr, ok := h.([]any); !ok || len(arr) > 0 {
			return false
		}
	}
	column, _ := raw["column"].(string)
	if column == "" {
		return false
	}
	var at int64
	if created, ok := raw["created_at_millis"].(float64); ok {
		at = int64(created)
	}
	raw["history"] = []map[string]any{
		{"field": "column", "value": column, "at": at},
	}
	return true
}

// migrateBoardV9ToV10 moves card-column association from board config to card files.
// For each column's card_ids, writes column + position to each card file,
// then strips card_ids from the board config.
func (s *MigrateService) migrateBoardV9ToV10(plan *BoardMigration) error {
	// Read board config with card_ids still present
	data, err := os.ReadFile(plan.ConfigPath)
	if err != nil {
		return err
	}

	var raw map[string]any
	if _, err := toml.Decode(string(data), &raw); err != nil {
		return fmt.Errorf("invalid TOML: %w", err)
	}

	// Extract column -> card_ids mapping before stripping
	columns, _ := raw["columns"].([]map[string]any)
	if columns == nil {
		// Try the TOML decoded format ([]any of map[string]any)
		if rawCols, ok := raw["columns"].([]any); ok {
			for _, rc := range rawCols {
				if m, ok := rc.(map[string]any); ok {
					columns = append(columns, m)
				}
			}
		}
	}

	// Write column + position to each card file
	cardsDir := s.paths.CardsDir(plan.BoardName)
	for _, col := range columns {
		colName, _ := col["name"].(string)
		if colName == "" {
			continue
		}

		var cardIDs []string
		if ids, ok := col["card_ids"].([]any); ok {
			for _, id := range ids {
				if s, ok := id.(string); ok {
					cardIDs = append(cardIDs, s)
				}
			}
		}

		// Generate evenly-spaced positions for all cards in this column
		positions := util.PositionInitial(len(cardIDs))

		for i, cardID := range cardIDs {
			cardPath := filepath.Join(cardsDir, cardID+".json")
			if err := s.writeCardColumnPosition(cardPath, colName, positions[i]); err != nil {
				// Non-fatal: card file might not exist (will be caught by doctor)
				fmt.Fprintf(os.Stderr, "Warning: could not update card %s: %v\n", cardID, err)
			}
		}
	}

	// Strip card_ids from all columns in the board config
	for _, col := range columns {
		delete(col, "card_ids")
	}

	// Update schema version
	raw["kan_schema"] = version.FormatBoardSchema(10)

	return writeTOMLMap(plan.ConfigPath, raw)
}

// writeCardColumnPosition adds/updates column and position fields on a card JSON file.
func (s *MigrateService) writeCardColumnPosition(path, column, position string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}

	var raw map[string]any
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}

	raw["column"] = column
	raw["position"] = position
	raw["_v"] = version.CurrentCardVersion

	return writeJSONMap(path, raw)
}
