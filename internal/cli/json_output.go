package cli

import (
	"encoding/json"
	"fmt"

	"github.com/amterp/kan/internal/model"
	"github.com/amterp/kan/internal/service"
)

// cardJson represents a card with all fields for JSON output.
// This exists because model.Card has Column tagged as json:"-" (not persisted to files).
//
// SYNC WARNING: This struct must stay in sync with model.Card fields.
// If you add fields to model.Card, add them here too. See TestCardJsonFieldSync.
type cardJson struct {
	// Note: Version (_v) is intentionally omitted - it's an internal schema version
	ID              string          `json:"id"`
	Alias           string          `json:"alias"`
	AliasExplicit   bool            `json:"alias_explicit"`
	Title           string          `json:"title"`
	Description     string          `json:"description,omitempty"`
	Parent          string          `json:"parent,omitempty"`
	Creator         string          `json:"creator"`
	CreatedAtMillis int64           `json:"created_at_millis"`
	UpdatedAtMillis int64           `json:"updated_at_millis"`
	Comments        []model.Comment `json:"comments,omitempty"`
	Column          string          `json:"column,omitempty"`
	CustomFields    map[string]any  `json:"-"` // Merged at top level like model.Card
}

func cardToJson(c *model.Card) cardJson {
	return cardJson{
		ID:              c.ID,
		Alias:           c.Alias,
		AliasExplicit:   c.AliasExplicit,
		Title:           c.Title,
		Description:     c.Description,
		Parent:          c.Parent,
		Creator:         c.Creator,
		CreatedAtMillis: c.CreatedAtMillis,
		UpdatedAtMillis: c.UpdatedAtMillis,
		Comments:        c.Comments,
		Column:          c.Column,
		CustomFields:    c.CustomFields,
	}
}

// MarshalJSON implements custom marshaling to merge CustomFields at top level.
func (c cardJson) MarshalJSON() ([]byte, error) {
	type Alias cardJson
	base, err := json.Marshal(Alias(c))
	if err != nil {
		return nil, err
	}

	if len(c.CustomFields) == 0 {
		return base, nil
	}

	var merged map[string]any
	if err := json.Unmarshal(base, &merged); err != nil {
		return nil, err
	}

	for k, v := range c.CustomFields {
		merged[k] = v
	}

	return json.Marshal(merged)
}

// CardOutput wraps a single card for JSON output.
type CardOutput struct {
	Card cardJson `json:"card"`
}

// NewCardOutput creates a CardOutput from a model.Card.
func NewCardOutput(card *model.Card) CardOutput {
	return CardOutput{Card: cardToJson(card)}
}

// hookResultJson represents a hook execution result for JSON output.
type hookResultJson struct {
	Name       string `json:"name"`
	Success    bool   `json:"success"`
	Stdout     string `json:"stdout,omitempty"`
	Stderr     string `json:"stderr,omitempty"`
	ExitCode   int    `json:"exit_code,omitempty"`
	DurationMs int64  `json:"duration_ms,omitempty"`
	Error      string `json:"error,omitempty"`
}

func hookResultToJson(r *service.HookResult) hookResultJson {
	result := hookResultJson{
		Name:       r.HookName,
		Success:    r.Success,
		Stdout:     r.Stdout,
		Stderr:     r.Stderr,
		ExitCode:   r.ExitCode,
		DurationMs: r.Duration.Milliseconds(),
	}
	if r.Error != nil {
		result.Error = r.Error.Error()
	}
	return result
}

// AddOutput wraps a created card and any hook results for JSON output.
type AddOutput struct {
	Card  cardJson         `json:"card"`
	Hooks []hookResultJson `json:"hooks,omitempty"`
}

// NewAddOutput creates an AddOutput from a card and hook results.
func NewAddOutput(card *model.Card, hookResults []*service.HookResult) AddOutput {
	output := AddOutput{Card: cardToJson(card)}
	if len(hookResults) > 0 {
		output.Hooks = make([]hookResultJson, len(hookResults))
		for i, r := range hookResults {
			output.Hooks[i] = hookResultToJson(r)
		}
	}
	return output
}

// ListOutput wraps a list of cards for JSON output.
type ListOutput struct {
	Cards []cardJson `json:"cards"`
}

// NewListOutput creates a ListOutput from a slice of model.Card.
// Always returns an empty array (not null) when there are no cards.
func NewListOutput(cards []*model.Card) ListOutput {
	result := make([]cardJson, 0, len(cards))
	for _, c := range cards {
		result = append(result, cardToJson(c))
	}
	return ListOutput{Cards: result}
}

// ColumnsOutput wraps a list of columns for JSON output.
type ColumnsOutput struct {
	Columns []ColumnInfo `json:"columns"`
}

// NewColumnsOutput creates a ColumnsOutput from column info.
// Always returns an empty array (not null) when there are no columns.
func NewColumnsOutput(columns []ColumnInfo) ColumnsOutput {
	if columns == nil {
		columns = []ColumnInfo{}
	}
	return ColumnsOutput{Columns: columns}
}

// ColumnInfo represents column data for JSON output.
type ColumnInfo struct {
	Name        string `json:"name"`
	Color       string `json:"color"`
	Description string `json:"description,omitempty"`
	CardCount   int    `json:"card_count"`
}

// BoardsOutput wraps a list of board names for JSON output.
type BoardsOutput struct {
	Boards []string `json:"boards"`
}

// NewBoardsOutput creates a BoardsOutput from board names.
// Always returns an empty array (not null) when there are no boards.
func NewBoardsOutput(boards []string) BoardsOutput {
	if boards == nil {
		boards = []string{}
	}
	return BoardsOutput{Boards: boards}
}

// CommentOutput wraps a single comment for JSON output.
type CommentOutput struct {
	Comment *model.Comment `json:"comment"`
}

// BoardDescribeOutput wraps board describe data for JSON output.
type BoardDescribeOutput struct {
	Board BoardDescribeInfo `json:"board"`
}

// BoardDescribeInfo contains full board documentation for JSON output.
// Kept in sync with model.BoardConfig by TestBoardDescribeFieldSync.
type BoardDescribeInfo struct {
	Name          string                             `json:"name"`
	Schema        string                             `json:"schema"`
	DefaultColumn string                             `json:"default_column"`
	Columns       []BoardDescribeColumnInfo          `json:"columns"`
	CustomFields  map[string]model.CustomFieldSchema `json:"custom_fields,omitempty"`
	CardDisplay   model.CardDisplayConfig            `json:"card_display,omitempty"`
	LinkRules     []model.LinkRule                   `json:"link_rules,omitempty"`
	PatternHooks  []model.PatternHook                `json:"pattern_hooks,omitempty"`
}

// BoardDescribeColumnInfo contains column data for board describe JSON output.
type BoardDescribeColumnInfo struct {
	Name        string `json:"name"`
	Color       string `json:"color"`
	Description string `json:"description,omitempty"`
	CardCount   int    `json:"card_count"`
	IsDefault   bool   `json:"is_default"`
}

// printJson marshals the value as indented JSON and prints it to stdout.
func printJson(v any) error {
	output, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	fmt.Println(string(output))
	return nil
}

// warnJsonNotSupported prints a warning to stderr when --json is used on an unsupported command.
func warnJsonNotSupported(command string) {
	PrintWarning("--json is not supported for '%s' (flag ignored)", command)
}
