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
	ID              string               `json:"id"`
	Alias           string               `json:"alias"`
	AliasExplicit   bool                 `json:"alias_explicit"`
	Title           string               `json:"title"`
	Description     string               `json:"description,omitempty"`
	Parent          string               `json:"parent,omitempty"`
	BlockedBy       []string             `json:"blocked_by,omitempty"`
	Related         []string             `json:"related,omitempty"`
	Creator         string               `json:"creator"`
	CreatedAtMillis int64                `json:"created_at_millis"`
	UpdatedAtMillis int64                `json:"updated_at_millis"`
	Comments        []model.Comment      `json:"comments,omitempty"`
	History         []model.HistoryEntry `json:"history,omitempty"`
	Column          string               `json:"column"`
	Position        string               `json:"position"`
	Board           string               `json:"board,omitempty"`
	CustomFields    map[string]any       `json:"-"` // Merged at top level like model.Card
}

func cardToJson(c *model.Card) cardJson {
	return cardJson{
		ID:              c.ID,
		Alias:           c.Alias,
		AliasExplicit:   c.AliasExplicit,
		Title:           c.Title,
		Description:     c.Description,
		Parent:          c.Parent,
		BlockedBy:       c.BlockedBy,
		Related:         c.Related,
		Creator:         c.Creator,
		CreatedAtMillis: c.CreatedAtMillis,
		UpdatedAtMillis: c.UpdatedAtMillis,
		Comments:        c.Comments,
		History:         c.History,
		Column:          c.Column,
		Position:        c.Position,
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
	Command    string `json:"command,omitempty"`
	Stdout     string `json:"stdout,omitempty"`
	Stderr     string `json:"stderr,omitempty"`
	ExitCode   int    `json:"exit_code,omitempty"`
	DurationMs int64  `json:"duration_ms,omitempty"`
	Error      string `json:"error,omitempty"`
	Hint       string `json:"hint,omitempty"`
}

func hookResultToJson(r *service.HookResult) hookResultJson {
	result := hookResultJson{
		Name:       r.HookName,
		Success:    r.Success,
		Command:    r.Command,
		Stdout:     r.Stdout,
		Stderr:     r.Stderr,
		ExitCode:   r.ExitCode,
		DurationMs: r.Duration.Milliseconds(),
	}
	if r.Error != nil {
		result.Error = r.Error.Error()
	}
	if r.CommandNotFound() {
		result.Hint = service.HookPathHint
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

// ReadyOutput is kan ready's JSON. Each card goes unchanged inside an entry,
// beside the query's context, so a custom field may share a name with a
// context field such as "ref".
type ReadyOutput struct {
	Ready     []readyEntryJson   `json:"ready"`
	Attention []readyEntryJson   `json:"attention"`
	Skipped   []skippedBoardJson `json:"skipped"`
}

type readyEntryJson struct {
	Ref         string        `json:"ref"`
	BoardID     string        `json:"board_id"`
	Board       string        `json:"board"`
	ProjectPath string        `json:"project_path"`
	Category    string        `json:"category"`
	Blockers    []blockerJson `json:"blockers"`
	Card        cardJson      `json:"card"`
}

type blockerJson struct {
	Ref      string `json:"ref"`
	Result   string `json:"result"`
	Category string `json:"category,omitempty"`
}

// skippedBoardJson names a board a query couldn't read. Only queries across
// boards skip any, so kan ready on one board always reports none.
type skippedBoardJson struct {
	Path    string `json:"path"`
	BoardID string `json:"board_id"`
	Reason  string `json:"reason"`
}

// NewReadyOutput creates a ReadyOutput for one board's ready query.
func NewReadyOutput(result *service.ReadyResult, projectPath string) ReadyOutput {
	toJson := func(entries []service.ReadyEntry) []readyEntryJson {
		out := make([]readyEntryJson, 0, len(entries))
		for _, e := range entries {
			blockers := make([]blockerJson, 0, len(e.Blockers))
			for _, b := range e.Blockers {
				bj := blockerJson{Ref: b.Ref, Result: string(b.State)}
				if b.State == service.LinkFound && b.Column != nil {
					bj.Category = b.Column.Category
				}
				blockers = append(blockers, bj)
			}
			out = append(out, readyEntryJson{
				Ref:         result.Board.ID + ":" + e.Card.ID,
				BoardID:     result.Board.ID,
				Board:       result.Board.Name,
				ProjectPath: projectPath,
				Category:    result.Board.ColumnCategory(e.Card.Column),
				Blockers:    blockers,
				Card:        cardToJson(e.Card),
			})
		}
		return out
	}
	return ReadyOutput{
		Ready:     toJson(result.Ready),
		Attention: toJson(result.Attention),
		Skipped:   []skippedBoardJson{},
	}
}

// ColumnsOutput wraps a list of columns for JSON output.
type ColumnsOutput struct {
	Columns []ColumnInfo       `json:"columns"`
	Ready   *model.ReadyConfig `json:"ready,omitempty"`
}

// NewColumnsOutput creates a ColumnsOutput from column info and the board's
// [ready] settings, which are left out when none are set.
// Always returns an empty array (not null) when there are no columns.
func NewColumnsOutput(columns []ColumnInfo, ready model.ReadyConfig) ColumnsOutput {
	if columns == nil {
		columns = []ColumnInfo{}
	}
	out := ColumnsOutput{Columns: columns}
	if !ready.IsEmpty() {
		out.Ready = &ready
	}
	return out
}

// ColumnInfo represents column data for JSON output.
type ColumnInfo struct {
	Name        string `json:"name"`
	Color       string `json:"color"`
	Description string `json:"description,omitempty"`
	Limit       int    `json:"limit,omitempty"`
	Category    string `json:"category,omitempty"`
	CardCount   int    `json:"card_count"`
}

// CategorizeOutput is the JSON output of kan column categorize.
type CategorizeOutput struct {
	Board       string                   `json:"board"`
	Applied     bool                     `json:"applied"`
	Suggestions []CategorySuggestionInfo `json:"suggestions"`
}

// CategorySuggestionInfo is one uncategorized column and the category
// suggested for it; Category is empty when its name matches nothing.
type CategorySuggestionInfo struct {
	Column   string `json:"column"`
	Category string `json:"category"`
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
	Ready         model.ReadyConfig                  `json:"ready"`
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
	Limit       int    `json:"limit,omitempty"`
	Category    string `json:"category,omitempty"`
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
