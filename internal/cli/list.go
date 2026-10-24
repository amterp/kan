package cli

import (
	"fmt"
	"sort"
	"strings"

	"github.com/amterp/kan/internal/model"
	"github.com/amterp/ra"
)

// customFieldNames returns the board's custom field names, sorted, joined for
// display in error messages.
func customFieldNames(boardCfg *model.BoardConfig) string {
	names := make([]string, 0, len(boardCfg.CustomFields))
	for name := range boardCfg.CustomFields {
		names = append(names, name)
	}
	sort.Strings(names)
	if len(names) == 0 {
		return "(none defined; add custom fields to the board config to enable sorting)"
	}
	return strings.Join(names, ", ")
}

func registerList(parent *ra.Cmd, ctx *CommandContext) {
	cmd := ra.NewCmd("list")
	cmd.SetDescription("List cards")

	ctx.ListBoard, _ = ra.NewString("board").
		SetShort("b").
		SetOptional(true).
		SetFlagOnly(true).
		SetUsage("Board name or ID").
		SetCompletionFunc(completeBoards).
		Register(cmd)

	ctx.ListColumn, _ = ra.NewString("column").
		SetShort("c").
		SetOptional(true).
		SetFlagOnly(true).
		SetUsage("Filter by column").
		SetCompletionFunc(completeColumns).
		Register(cmd)

	ctx.ListFilter, _ = ra.NewStringSlice("filter").
		SetShort("f").
		SetOptional(true).
		SetFlagOnly(true).
		SetUsage("Keep cards whose custom field matches key=value (repeatable; all must match)").
		SetCompletionFunc(completeFilters).
		Register(cmd)

	ctx.ListExclude, _ = ra.NewStringSlice("exclude").
		SetShort("x").
		SetOptional(true).
		SetFlagOnly(true).
		SetUsage("Drop cards whose custom field matches key=value (repeatable)").
		SetCompletionFunc(completeFilters).
		Register(cmd)

	ctx.ListCategory, _ = ra.NewString("category").
		SetOptional(true).
		SetFlagOnly(true).
		SetEnumConstraint(model.ValidCategories).
		SetUsage("Keep cards in columns of this category").
		Register(cmd)

	ctx.ListSort, _ = ra.NewString("sort").
		SetShort("s").
		SetOptional(true).
		SetFlagOnly(true).
		SetUsage("Sort cards within each column by a custom field (e.g. priority; see 'kan board describe')").
		SetCompletionFunc(completeCustomFields).
		Register(cmd)

	ctx.ListDescending, _ = ra.NewBool("descending").
		SetShort("d").
		SetOptional(true).
		SetFlagOnly(true).
		SetUsage("Sort descending (use with --sort)").
		Register(cmd)

	ctx.ListGlobal = registerGlobalFlag(cmd)

	ctx.ListUsed, _ = parent.RegisterCmd(cmd)
}

func runList(board, column, sortField string, filters, excludes []string, category string, global, descending, jsonOutput bool) {
	app, err := NewAppWithOptions(AppOptions{Interactive: true, UseGlobalBoard: global, ReadOnly: true})
	if err != nil {
		Fatal(err)
	}

	// Resolve board
	project, boardName, err := app.ResolveBoard(board, true, forRead)
	if err != nil {
		Fatal(err)
	}
	project.PrintTarget(boardName)

	// Get board config for column ordering
	boardCfg, err := project.BoardService.Get(boardName)
	if err != nil {
		Fatal(err)
	}

	// Validate the sort field (if any) names a defined custom field.
	if sortField != "" {
		if _, ok := boardCfg.CustomFields[sortField]; !ok {
			Fatal(fmt.Errorf("unknown sort field %q; valid fields: %s",
				sortField, customFieldNames(boardCfg)))
		}
	}

	query, err := buildListQuery(boardCfg, filters, excludes, category)
	if err != nil {
		Fatal(err)
	}

	// --descending only affects a field sort; warn rather than silently ignore it.
	if descending && sortField == "" {
		PrintWarning("--descending has no effect without --sort")
	}

	// Get cards
	cards, err := project.CardService.ListSorted(boardName, column, sortField, descending)
	if err != nil {
		Fatal(err)
	}
	cards = filterCards(cards, boardCfg, query)

	if jsonOutput {
		if err := printJson(NewListOutput(cards)); err != nil {
			Fatal(err)
		}
		return
	}

	if len(cards) == 0 {
		PrintInfo("No cards found")
		return
	}

	// Group by column if not filtering by column
	if column == "" {
		printCardsByColumn(cards, boardCfg)
	} else {
		printCardsList(cards, boardCfg)
	}
}

// buildListQuery parses and checks the -f, -x and --category flags against
// the board before any card is read, so a typo fails loudly instead of
// quietly listing nothing.
func buildListQuery(boardCfg *model.BoardConfig, filters, excludes []string, category string) (model.FilterSet, error) {
	include, err := model.ParseFilters(filters)
	if err != nil {
		return model.FilterSet{}, err
	}
	exclude, err := model.ParseFilters(excludes)
	if err != nil {
		return model.FilterSet{}, err
	}
	if err := model.ValidateFilters(append(append([]model.Filter{}, include...), exclude...), boardCfg); err != nil {
		return model.FilterSet{}, err
	}

	if category != "" && !boardHasCategory(boardCfg, category) {
		return model.FilterSet{}, fmt.Errorf("board %q has no columns with category %q, so no card can match. "+
			"Run \"kan column categorize\" or \"kan column edit <column> --category %s\" to set categories",
			boardCfg.Name, category, category)
	}

	return model.FilterSet{Include: include, Exclude: exclude, Category: category}, nil
}

func boardHasCategory(boardCfg *model.BoardConfig, category string) bool {
	for _, col := range boardCfg.Columns {
		if col.Category == category {
			return true
		}
	}
	return false
}

// filterCards keeps the cards the query matches, in their existing order.
func filterCards(cards []*model.Card, boardCfg *model.BoardConfig, query model.FilterSet) []*model.Card {
	kept := cards[:0]
	for _, card := range cards {
		if query.Matches(card, boardCfg) {
			kept = append(kept, card)
		}
	}
	return kept
}

// cardColumnWidths holds the calculated widths for aligning card output.
type cardColumnWidths struct {
	idWidth   int
	typeWidth int
}

func printCardsByColumn(cards []*model.Card, boardCfg *model.BoardConfig) {
	cardsByColumn := make(map[string][]*model.Card)
	for _, card := range cards {
		cardsByColumn[card.Column] = append(cardsByColumn[card.Column], card)
	}

	for _, col := range boardCfg.Columns {
		colCards := cardsByColumn[col.Name]
		if len(colCards) == 0 {
			continue
		}

		// Column header with color from board config
		colHeader := RenderColumnColor(col.Name, col.Color)
		var countStr string
		if col.Limit > 0 {
			countStr = RenderMuted(fmt.Sprintf("(%d/%d)", len(colCards), col.Limit))
		} else {
			countStr = RenderMuted(fmt.Sprintf("(%d)", len(colCards)))
		}
		fmt.Printf("\n%s %s\n", colHeader, countStr)

		// Calculate column widths for alignment within this column
		widths := calculateColumnWidths(colCards, boardCfg)
		for _, card := range colCards {
			printCardLine(card, boardCfg, widths)
		}
	}
}

func printCardsList(cards []*model.Card, boardCfg *model.BoardConfig) {
	widths := calculateColumnWidths(cards, boardCfg)
	for _, card := range cards {
		printCardLine(card, boardCfg, widths)
	}
}

// getTypeIndicatorValue returns the type indicator field value for a card, or empty string.
func getTypeIndicatorValue(card *model.Card, boardCfg *model.BoardConfig) string {
	typeField := boardCfg.CardDisplay.TypeIndicator
	if typeField == "" {
		return ""
	}
	if val, ok := card.CustomFields[typeField].(string); ok {
		return val
	}
	return ""
}

// getSetValues extracts string values from a set field (enum-set or free-set).
func getSetValues(card *model.Card, fieldName string) []string {
	val, ok := card.CustomFields[fieldName]
	if !ok || val == nil {
		return nil
	}
	switch v := val.(type) {
	case []any:
		result := make([]string, 0, len(v))
		for _, item := range v {
			if s, ok := item.(string); ok {
				result = append(result, s)
			}
		}
		return result
	case []string:
		return v
	default:
		return nil
	}
}

// renderBadges renders all badge values for a card as colored [value] tags.
func renderBadges(card *model.Card, boardCfg *model.BoardConfig) string {
	badgeFields := boardCfg.CardDisplay.Badges
	if len(badgeFields) == 0 {
		return ""
	}
	var parts []string
	for _, fieldName := range badgeFields {
		schema, exists := boardCfg.CustomFields[fieldName]
		if !exists {
			continue
		}

		if schema.Type == model.FieldTypeBoolean {
			if b, ok := card.CustomFields[fieldName].(bool); ok && b {
				color := badgeColor("boolean", fieldName, fieldName)
				if rendered := RenderTypeIndicator(fieldName, color); rendered != "" {
					parts = append(parts, rendered)
				}
			}
		} else {
			values := getSetValues(card, fieldName)
			for _, val := range values {
				color := boardCfg.GetOptionColor(fieldName, val)
				if color == "" {
					color = badgeColor(schema.Type, fieldName, val)
				}
				if rendered := RenderTypeIndicator(val, color); rendered != "" {
					parts = append(parts, rendered)
				}
			}
		}
	}
	if len(parts) == 0 {
		return ""
	}
	return "  " + strings.Join(parts, " ")
}

// calculateColumnWidths calculates the max widths for ID and type indicator columns.
func calculateColumnWidths(cards []*model.Card, boardCfg *model.BoardConfig) cardColumnWidths {
	widths := cardColumnWidths{}
	for _, card := range cards {
		if len(card.ID) > widths.idWidth {
			widths.idWidth = len(card.ID)
		}
		val := getTypeIndicatorValue(card, boardCfg)
		if val != "" {
			// Visual width is len("[" + val + "]")
			width := len(val) + 2
			if width > widths.typeWidth {
				widths.typeWidth = width
			}
		}
	}
	return widths
}

func printCardLine(card *model.Card, boardCfg *model.BoardConfig, widths cardColumnWidths) {
	// Render ID with padding
	idPadding := widths.idWidth - len(card.ID)
	renderedID := RenderID(card.ID) + strings.Repeat(" ", idPadding)

	// Render type indicator with padding
	typeIndicator := ""
	if widths.typeWidth > 0 {
		val := getTypeIndicatorValue(card, boardCfg)
		if val != "" {
			color := boardCfg.GetOptionColor(boardCfg.CardDisplay.TypeIndicator, val)
			rendered := RenderTypeIndicator(val, color)
			// Pad to align: visual width is len(val) + 2 for brackets
			padding := widths.typeWidth - (len(val) + 2)
			typeIndicator = rendered + strings.Repeat(" ", padding) + "  "
		} else {
			// No value but others have type indicators, maintain alignment
			typeIndicator = strings.Repeat(" ", widths.typeWidth) + "  "
		}
	}
	badges := renderBadges(card, boardCfg)
	fmt.Printf("  %s  %s%s%s\n", renderedID, typeIndicator, card.Title, badges)
}
