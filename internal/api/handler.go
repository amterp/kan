package api

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"sync"

	"github.com/amterp/kan/internal/config"
	"github.com/amterp/kan/internal/model"
	"github.com/amterp/kan/internal/service"
	"github.com/amterp/kan/internal/store"
)

// CardResponse wraps a Card for JSON API responses, including the Column field
// which is computed (from board config) and not persisted to card files.
// Custom fields are flattened into the top level to match the card JSON storage format.
type CardResponse struct {
	ID                  string                   `json:"id"`
	Alias               string                   `json:"alias"`
	AliasExplicit       bool                     `json:"alias_explicit"`
	Title               string                   `json:"title"`
	Description         string                   `json:"description,omitempty"`
	Column              string                   `json:"column"`
	Parent              string                   `json:"parent,omitempty"`
	Creator             string                   `json:"creator"`
	CreatedAtMillis     int64                    `json:"created_at_millis"`
	UpdatedAtMillis     int64                    `json:"updated_at_millis"`
	Comments            []model.Comment          `json:"comments,omitempty"`
	CustomFields        map[string]any           `json:"-"` // Flattened into top level by MarshalJSON
	MissingWantedFields []MissingWantedFieldInfo `json:"missing_wanted_fields,omitempty"`
}

// MarshalJSON flattens custom fields into the top level of the JSON output.
func (c CardResponse) MarshalJSON() ([]byte, error) {
	// Build base map with known fields
	m := map[string]any{
		"id":                c.ID,
		"alias":             c.Alias,
		"alias_explicit":    c.AliasExplicit,
		"title":             c.Title,
		"column":            c.Column,
		"creator":           c.Creator,
		"created_at_millis": c.CreatedAtMillis,
		"updated_at_millis": c.UpdatedAtMillis,
	}

	// Add optional fields only if non-empty
	if c.Description != "" {
		m["description"] = c.Description
	}
	if c.Parent != "" {
		m["parent"] = c.Parent
	}
	if len(c.Comments) > 0 {
		m["comments"] = c.Comments
	}
	if len(c.MissingWantedFields) > 0 {
		m["missing_wanted_fields"] = c.MissingWantedFields
	}

	// Flatten custom fields into top level
	for k, v := range c.CustomFields {
		m[k] = v
	}

	return json.Marshal(m)
}

// toCardResponse converts a model.Card to a CardResponse for API output.
func toCardResponse(card *model.Card) CardResponse {
	return CardResponse{
		ID:              card.ID,
		Alias:           card.Alias,
		AliasExplicit:   card.AliasExplicit,
		Title:           card.Title,
		Description:     card.Description,
		Column:          card.Column,
		Parent:          card.Parent,
		Creator:         card.Creator,
		CreatedAtMillis: card.CreatedAtMillis,
		UpdatedAtMillis: card.UpdatedAtMillis,
		Comments:        card.Comments,
		CustomFields:    card.CustomFields,
	}
}

// toCardResponseWithWanted converts a model.Card to a CardResponse including wanted fields check.
func toCardResponseWithWanted(card *model.Card, boardCfg *model.BoardConfig) CardResponse {
	resp := toCardResponse(card)
	if boardCfg != nil {
		for _, mf := range service.CheckWantedFields(card, boardCfg) {
			resp.MissingWantedFields = append(resp.MissingWantedFields, MissingWantedFieldInfo{
				Name:    mf.FieldName,
				Type:    mf.FieldType,
				Options: mf.Options,
			})
		}
	}
	return resp
}

// toCardResponses converts a slice of model.Card to CardResponses.
func toCardResponses(cards []*model.Card, boardCfg *model.BoardConfig) []CardResponse {
	responses := make([]CardResponse, len(cards))
	for i, card := range cards {
		responses[i] = toCardResponseWithWanted(card, boardCfg)
	}
	return responses
}

// populateCardColumn sets the Column field on a card by looking up the board config.
func (h *Handler) populateCardColumn(boardName string, card *model.Card) {
	boardCfg, err := h.ctx().BoardStore.Get(boardName)
	if err != nil {
		return // Leave column empty if board config can't be read
	}
	card.Column = boardCfg.GetCardColumn(card.ID)
}

// Handler contains all HTTP handlers for the API.
//
// Design: single-user, single-session. The Handler holds one active ProjectContext
// that is shared by all requests. SwitchProject swaps it atomically. This is intentional —
// `kan serve` is a local development tool, not a multi-tenant server. All connected
// clients (browser tabs) see the same project.
//
// Lifecycle: globalStore is read fresh on each ListAllBoards/SwitchProject call (never
// cached), so external changes to ~/.config/kan/config.toml are picked up immediately.
type Handler struct {
	globalStore     store.GlobalStore
	mu              sync.RWMutex
	current         *ProjectContext
	onProjectSwitch func(newProjectRoot string) // Called when project is switched
}

// NewHandler creates a new handler with the given dependencies.
func NewHandler(globalStore store.GlobalStore, ctx *ProjectContext) *Handler {
	return &Handler{
		globalStore: globalStore,
		current:     ctx,
	}
}

// SetOnProjectSwitch sets a callback that's called when the active project changes.
// Used by Server to update the file watcher when projects are switched.
func (h *Handler) SetOnProjectSwitch(fn func(newProjectRoot string)) {
	h.onProjectSwitch = fn
}

// ctx returns the current ProjectContext under read lock.
// The returned context is immutable; callers should not modify it.
func (h *Handler) ctx() *ProjectContext {
	h.mu.RLock()
	c := h.current
	h.mu.RUnlock()
	return c
}

// RegisterRoutes sets up all API routes on the given mux.
func (h *Handler) RegisterRoutes(mux *http.ServeMux) {
	// Project routes
	mux.HandleFunc("GET /api/v1/project", h.GetProject)
	mux.HandleFunc("GET /favicon.svg", h.GetFavicon)

	// Cross-project routes
	mux.HandleFunc("GET /api/v1/all-boards", h.ListAllBoards)
	mux.HandleFunc("POST /api/v1/switch", h.SwitchProject)

	// Board routes
	mux.HandleFunc("GET /api/v1/boards", h.ListBoards)
	mux.HandleFunc("GET /api/v1/boards/{name}", h.GetBoard)

	// Column routes
	mux.HandleFunc("POST /api/v1/boards/{board}/columns", h.CreateColumn)
	mux.HandleFunc("DELETE /api/v1/boards/{board}/columns/{name}", h.DeleteColumn)
	mux.HandleFunc("PATCH /api/v1/boards/{board}/columns/{name}", h.UpdateColumn)
	mux.HandleFunc("PUT /api/v1/boards/{board}/columns/order", h.ReorderColumns)

	// Card routes
	mux.HandleFunc("GET /api/v1/boards/{board}/cards", h.ListCards)
	mux.HandleFunc("POST /api/v1/boards/{board}/cards", h.CreateCard)
	mux.HandleFunc("GET /api/v1/boards/{board}/cards/{id}", h.GetCard)
	mux.HandleFunc("PUT /api/v1/boards/{board}/cards/{id}", h.UpdateCard)
	mux.HandleFunc("DELETE /api/v1/boards/{board}/cards/{id}", h.DeleteCard)
	mux.HandleFunc("PATCH /api/v1/boards/{board}/cards/{id}/move", h.MoveCard)

	// Comment routes
	mux.HandleFunc("POST /api/v1/boards/{board}/cards/{id}/comments", h.CreateComment)
	mux.HandleFunc("PATCH /api/v1/boards/{board}/cards/{id}/comments/{cid}", h.EditComment)
	mux.HandleFunc("DELETE /api/v1/boards/{board}/cards/{id}/comments/{cid}", h.DeleteComment)

	// Static files (frontend)
	mux.Handle("/", h.StaticHandler())
}

// --- Project Handlers ---

// ProjectResponse is the JSON response for project metadata.
type ProjectResponse struct {
	Name    string              `json:"name"`
	Favicon model.FaviconConfig `json:"favicon"`
}

// GetProject returns the project metadata.
func (h *Handler) GetProject(w http.ResponseWriter, r *http.Request) {
	cfg, err := h.ctx().ProjectStore.Load()
	if err != nil {
		// Return sensible defaults on error
		JSON(w, http.StatusOK, ProjectResponse{
			Name:    "Kan",
			Favicon: model.DefaultFaviconConfig("", "Kan"),
		})
		return
	}

	// If no name set, use "Kan"
	name := cfg.Name
	if name == "" {
		name = "Kan"
	}

	// If no favicon config, use defaults
	favicon := cfg.Favicon
	if favicon.Background == "" {
		favicon = model.DefaultFaviconConfig(cfg.ID, name)
	}

	JSON(w, http.StatusOK, ProjectResponse{
		Name:    name,
		Favicon: favicon,
	})
}

// --- Board Handlers ---

// ListBoards returns all board names.
func (h *Handler) ListBoards(w http.ResponseWriter, r *http.Request) {
	boards, err := h.ctx().BoardStore.List()
	if err != nil {
		Error(w, err)
		return
	}
	JSON(w, http.StatusOK, map[string][]string{"boards": boards})
}

// GetBoard returns a board's configuration.
func (h *Handler) GetBoard(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	board, err := h.ctx().BoardStore.Get(name)
	if err != nil {
		Error(w, err)
		return
	}
	JSON(w, http.StatusOK, board)
}

// --- Card Handlers ---

// ListCards returns all cards for a board, optionally filtered by column.
func (h *Handler) ListCards(w http.ResponseWriter, r *http.Request) {
	boardName := r.PathValue("board")
	columnFilter := r.URL.Query().Get("column")

	// Verify board exists first
	if !h.ctx().BoardStore.Exists(boardName) {
		NotFound(w, "board", boardName)
		return
	}

	cards, err := h.ctx().CardService.List(boardName, columnFilter)
	if err != nil {
		Error(w, err)
		return
	}

	// Get board config for wanted fields check
	boardCfg, _ := h.ctx().BoardStore.Get(boardName)
	JSON(w, http.StatusOK, map[string]any{"cards": toCardResponses(cards, boardCfg)})
}

// CreateCardRequest is the JSON body for creating a card.
type CreateCardRequest struct {
	Title        string            `json:"title"`
	Description  string            `json:"description,omitempty"`
	Column       string            `json:"column,omitempty"`
	Parent       string            `json:"parent,omitempty"`
	CustomFields map[string]string `json:"custom_fields,omitempty"`
}

// MissingWantedFieldInfo describes a wanted field that is missing from a card.
type MissingWantedFieldInfo struct {
	Name    string   `json:"name"`
	Type    string   `json:"type"`
	Options []string `json:"options,omitempty"`
}

// CreateCardResponse is the JSON response for creating a card.
type CreateCardResponse struct {
	Card                CardResponse             `json:"card"`
	HookResults         []HookInfo               `json:"hook_results,omitempty"`
	MissingWantedFields []MissingWantedFieldInfo `json:"missing_wanted_fields,omitempty"`
}

// HookInfo contains information about a hook execution for API response.
type HookInfo struct {
	Name    string `json:"name"`
	Success bool   `json:"success"`
	Output  string `json:"output,omitempty"`
	Error   string `json:"error,omitempty"`
}

// CreateCard creates a new card.
func (h *Handler) CreateCard(w http.ResponseWriter, r *http.Request) {
	boardName := r.PathValue("board")

	var req CreateCardRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		BadRequest(w, "invalid JSON body")
		return
	}

	if req.Title == "" {
		BadRequest(w, "title is required")
		return
	}

	input := service.AddCardInput{
		BoardName:    boardName,
		Title:        req.Title,
		Description:  req.Description,
		Column:       req.Column,
		Parent:       req.Parent,
		Creator:      h.ctx().Creator,
		CustomFields: req.CustomFields,
	}

	card, hookResults, err := h.ctx().CardService.Add(input)
	if err != nil {
		Error(w, err)
		return
	}

	// Populate Column from board config for API response
	h.populateCardColumn(boardName, card)

	// Get board config for wanted fields check
	boardCfg, _ := h.ctx().BoardStore.Get(boardName)

	// Build hook info response
	var hookInfos []HookInfo
	for _, result := range hookResults {
		info := HookInfo{
			Name:    result.HookName,
			Success: result.Success,
			Output:  result.Stdout,
		}
		if result.Error != nil {
			info.Error = result.Error.Error()
		}
		hookInfos = append(hookInfos, info)
	}

	// Build card response with wanted fields check
	cardResp := toCardResponseWithWanted(card, boardCfg)

	JSON(w, http.StatusCreated, CreateCardResponse{
		Card:                cardResp,
		HookResults:         hookInfos,
		MissingWantedFields: cardResp.MissingWantedFields, // Same data at both levels for compatibility
	})
}

// GetCard returns a single card by ID.
func (h *Handler) GetCard(w http.ResponseWriter, r *http.Request) {
	boardName := r.PathValue("board")
	cardID := r.PathValue("id")

	card, err := h.ctx().CardService.FindByIDOrAlias(boardName, cardID)
	if err != nil {
		Error(w, err)
		return
	}

	// Populate Column from board config for API response
	h.populateCardColumn(boardName, card)

	// Get board config for wanted fields check
	boardCfg, _ := h.ctx().BoardStore.Get(boardName)
	JSON(w, http.StatusOK, toCardResponseWithWanted(card, boardCfg))
}

// UpdateCardRequest is the JSON body for updating a card.
type UpdateCardRequest struct {
	Title        *string           `json:"title,omitempty"`
	Description  *string           `json:"description,omitempty"`
	Column       *string           `json:"column,omitempty"`
	CustomFields map[string]string `json:"custom_fields,omitempty"`
}

// UpdateCard updates an existing card.
func (h *Handler) UpdateCard(w http.ResponseWriter, r *http.Request) {
	boardName := r.PathValue("board")
	cardID := r.PathValue("id")

	card, err := h.ctx().CardService.FindByIDOrAlias(boardName, cardID)
	if err != nil {
		Error(w, err)
		return
	}

	// Populate Column from board config for comparison
	h.populateCardColumn(boardName, card)

	var req UpdateCardRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		BadRequest(w, "invalid JSON body")
		return
	}

	// Handle column change separately (uses MoveCard to update board config)
	if req.Column != nil && *req.Column != card.Column {
		if err := h.ctx().CardService.MoveCard(boardName, card.ID, *req.Column); err != nil {
			Error(w, err)
			return
		}
		card.Column = *req.Column // Update in-memory for response
	}

	// Apply other updates via Edit
	input := service.EditCardInput{
		BoardName:     boardName,
		CardIDOrAlias: card.ID,
		Title:         req.Title,
		Description:   req.Description,
		CustomFields:  req.CustomFields,
	}

	// Only call Edit if there are changes to apply
	if req.Title != nil || req.Description != nil || len(req.CustomFields) > 0 {
		updated, err := h.ctx().CardService.Edit(input)
		if err != nil {
			Error(w, err)
			return
		}
		card = updated
		h.populateCardColumn(boardName, card)
	}

	// Get board config for wanted fields check
	boardCfg, _ := h.ctx().BoardStore.Get(boardName)
	JSON(w, http.StatusOK, toCardResponseWithWanted(card, boardCfg))
}

// DeleteCard deletes a card.
func (h *Handler) DeleteCard(w http.ResponseWriter, r *http.Request) {
	boardName := r.PathValue("board")
	cardID := r.PathValue("id")

	// First resolve the card ID (might be an alias)
	card, err := h.ctx().CardService.FindByIDOrAlias(boardName, cardID)
	if err != nil {
		Error(w, err)
		return
	}

	if err := h.ctx().CardService.Delete(boardName, card.ID); err != nil {
		Error(w, err)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// MoveCardRequest is the JSON body for moving a card.
type MoveCardRequest struct {
	Column   string `json:"column"`
	Position *int   `json:"position,omitempty"` // Optional: position in target column (-1 or omit for end)
}

// MoveCard moves a card to a different column.
func (h *Handler) MoveCard(w http.ResponseWriter, r *http.Request) {
	boardName := r.PathValue("board")
	cardID := r.PathValue("id")

	// First resolve the card ID (might be an alias)
	card, err := h.ctx().CardService.FindByIDOrAlias(boardName, cardID)
	if err != nil {
		Error(w, err)
		return
	}

	var req MoveCardRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		BadRequest(w, "invalid JSON body")
		return
	}

	if req.Column == "" {
		BadRequest(w, "column is required")
		return
	}

	// Determine position (-1 means append to end)
	position := -1
	if req.Position != nil {
		position = *req.Position
	}

	// Use the service's MoveCardAt which updates board config
	if err := h.ctx().CardService.MoveCardAt(boardName, card.ID, req.Column, position); err != nil {
		Error(w, err)
		return
	}

	// Set Column to the target column for response
	card.Column = req.Column

	// Get board config for wanted fields check
	boardCfg, _ := h.ctx().BoardStore.Get(boardName)
	JSON(w, http.StatusOK, toCardResponseWithWanted(card, boardCfg))
}

// --- Column Handlers ---

// CreateColumnRequest is the JSON body for creating a column.
type CreateColumnRequest struct {
	Name     string `json:"name"`
	Color    string `json:"color,omitempty"`
	Position *int   `json:"position,omitempty"` // Optional: insert position (-1 or omit for end)
}

// CreateColumn creates a new column on a board.
func (h *Handler) CreateColumn(w http.ResponseWriter, r *http.Request) {
	boardName := r.PathValue("board")

	var req CreateColumnRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		BadRequest(w, "invalid JSON body")
		return
	}

	if req.Name == "" {
		BadRequest(w, "name is required")
		return
	}

	// Determine position (-1 means append to end)
	position := -1
	if req.Position != nil {
		position = *req.Position
	}

	if err := h.ctx().BoardService.AddColumn(boardName, req.Name, req.Color, position); err != nil {
		Error(w, err)
		return
	}

	// Get the updated board to return the new column
	board, err := h.ctx().BoardStore.Get(boardName)
	if err != nil {
		Error(w, err)
		return
	}

	col := board.GetColumn(req.Name)
	JSON(w, http.StatusCreated, col)
}

// DeleteColumnResponse is returned when a column is deleted.
type DeleteColumnResponse struct {
	DeletedCards int `json:"deleted_cards"`
}

// DeleteColumn deletes a column and all its cards.
func (h *Handler) DeleteColumn(w http.ResponseWriter, r *http.Request) {
	boardName := r.PathValue("board")
	columnName := r.PathValue("name")

	deletedCards, err := h.ctx().BoardService.DeleteColumn(boardName, columnName)
	if err != nil {
		Error(w, err)
		return
	}

	JSON(w, http.StatusOK, DeleteColumnResponse{DeletedCards: deletedCards})
}

// UpdateColumnRequest is the JSON body for updating a column.
type UpdateColumnRequest struct {
	Name  *string `json:"name,omitempty"`  // New name (rename)
	Color *string `json:"color,omitempty"` // New color
}

// UpdateColumn updates a column's properties (rename, color).
func (h *Handler) UpdateColumn(w http.ResponseWriter, r *http.Request) {
	boardName := r.PathValue("board")
	columnName := r.PathValue("name")

	var req UpdateColumnRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		BadRequest(w, "invalid JSON body")
		return
	}

	// Handle rename
	if req.Name != nil && *req.Name != columnName {
		if err := h.ctx().BoardService.RenameColumn(boardName, columnName, *req.Name); err != nil {
			Error(w, err)
			return
		}
		columnName = *req.Name // Update for subsequent operations
	}

	// Handle color change
	if req.Color != nil {
		if err := h.ctx().BoardService.UpdateColumnColor(boardName, columnName, *req.Color); err != nil {
			Error(w, err)
			return
		}
	}

	// Return the updated column
	board, err := h.ctx().BoardStore.Get(boardName)
	if err != nil {
		Error(w, err)
		return
	}

	col := board.GetColumn(columnName)
	if col == nil {
		NotFound(w, "column", columnName)
		return
	}

	JSON(w, http.StatusOK, col)
}

// ReorderColumnsRequest is the JSON body for reordering columns.
type ReorderColumnsRequest struct {
	Columns []string `json:"columns"`
}

// ReorderColumns reorders all columns according to the provided order.
func (h *Handler) ReorderColumns(w http.ResponseWriter, r *http.Request) {
	boardName := r.PathValue("board")

	var req ReorderColumnsRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		BadRequest(w, "invalid JSON body")
		return
	}

	if len(req.Columns) == 0 {
		BadRequest(w, "columns array is required")
		return
	}

	if err := h.ctx().BoardService.ReorderColumns(boardName, req.Columns); err != nil {
		Error(w, err)
		return
	}

	// Return the updated board config
	board, err := h.ctx().BoardStore.Get(boardName)
	if err != nil {
		Error(w, err)
		return
	}

	JSON(w, http.StatusOK, board)
}

// --- Comment Handlers ---

// CreateCommentRequest is the JSON body for creating a comment.
type CreateCommentRequest struct {
	Body string `json:"body"`
}

// CommentResponse is the JSON response for a comment.
type CommentResponse struct {
	ID              string `json:"id"`
	Body            string `json:"body"`
	Author          string `json:"author"`
	CreatedAtMillis int64  `json:"created_at_millis"`
	UpdatedAtMillis int64  `json:"updated_at_millis,omitempty"`
}

// toCommentResponse converts a model.Comment to a CommentResponse.
func toCommentResponse(c *model.Comment) CommentResponse {
	return CommentResponse{
		ID:              c.ID,
		Body:            c.Body,
		Author:          c.Author,
		CreatedAtMillis: c.CreatedAtMillis,
		UpdatedAtMillis: c.UpdatedAtMillis,
	}
}

// CreateComment creates a new comment on a card.
func (h *Handler) CreateComment(w http.ResponseWriter, r *http.Request) {
	boardName := r.PathValue("board")
	cardID := r.PathValue("id")

	var req CreateCommentRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		BadRequest(w, "invalid JSON body")
		return
	}

	if req.Body == "" {
		BadRequest(w, "body is required")
		return
	}

	comment, err := h.ctx().CardService.AddComment(boardName, cardID, req.Body, h.ctx().Creator)
	if err != nil {
		Error(w, err)
		return
	}

	JSON(w, http.StatusCreated, toCommentResponse(comment))
}

// EditCommentRequest is the JSON body for editing a comment.
type EditCommentRequest struct {
	Body string `json:"body"`
}

// EditComment updates an existing comment's body.
func (h *Handler) EditComment(w http.ResponseWriter, r *http.Request) {
	boardName := r.PathValue("board")
	commentID := r.PathValue("cid")

	var req EditCommentRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		BadRequest(w, "invalid JSON body")
		return
	}

	if req.Body == "" {
		BadRequest(w, "body is required")
		return
	}

	comment, err := h.ctx().CardService.EditComment(boardName, commentID, req.Body)
	if err != nil {
		Error(w, err)
		return
	}

	JSON(w, http.StatusOK, toCommentResponse(comment))
}

// DeleteComment removes a comment from a card.
func (h *Handler) DeleteComment(w http.ResponseWriter, r *http.Request) {
	boardName := r.PathValue("board")
	commentID := r.PathValue("cid")

	if err := h.ctx().CardService.DeleteComment(boardName, commentID); err != nil {
		Error(w, err)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// --- Cross-Project Handlers ---

// BoardEntry represents a single board across all registered projects.
type BoardEntry struct {
	ProjectName string `json:"project_name"`
	ProjectPath string `json:"project_path"`
	BoardName   string `json:"board_name"`
}

// SkippedProject describes a registered project that couldn't be listed.
type SkippedProject struct {
	Name   string `json:"name"`
	Path   string `json:"path"`
	Reason string `json:"reason"`
}

// AllBoardsResponse is the JSON response for listing all boards across projects.
type AllBoardsResponse struct {
	Boards             []BoardEntry     `json:"boards"`
	CurrentProjectPath string           `json:"current_project_path"`
	Skipped            []SkippedProject `json:"skipped,omitempty"`
}

// ListAllBoards returns all boards across all registered projects.
func (h *Handler) ListAllBoards(w http.ResponseWriter, r *http.Request) {
	globalCfg, err := h.globalStore.Load()
	if err != nil {
		Error(w, fmt.Errorf("failed to load global config: %w", err))
		return
	}

	var boards []BoardEntry
	var skipped []SkippedProject

	for projectName, projectPath := range globalCfg.Projects {
		dataLocation := ""
		if repoCfg := globalCfg.GetRepoConfig(projectPath); repoCfg != nil {
			dataLocation = repoCfg.DataLocation
		}

		paths := config.NewPaths(projectPath, dataLocation)
		boardStore := store.NewBoardStore(paths)

		boardNames, err := boardStore.List()
		if err != nil {
			log.Printf("Skipping project %q (%s): %v", projectName, projectPath, err)
			skipped = append(skipped, SkippedProject{
				Name:   projectName,
				Path:   projectPath,
				Reason: fmt.Sprintf("failed to list boards: %v", err),
			})
			continue
		}

		if len(boardNames) == 0 {
			log.Printf("Skipping project %q (%s): no boards", projectName, projectPath)
			skipped = append(skipped, SkippedProject{
				Name:   projectName,
				Path:   projectPath,
				Reason: "no boards found",
			})
			continue
		}

		// Try to get display name from project config
		displayName := projectName
		projectStore := store.NewProjectStore(paths)
		if projCfg, err := projectStore.Load(); err == nil && projCfg.Name != "" {
			displayName = projCfg.Name
		}

		for _, bn := range boardNames {
			boards = append(boards, BoardEntry{
				ProjectName: displayName,
				ProjectPath: projectPath,
				BoardName:   bn,
			})
		}
	}

	if boards == nil {
		boards = []BoardEntry{}
	}

	JSON(w, http.StatusOK, AllBoardsResponse{
		Boards:             boards,
		CurrentProjectPath: h.ctx().ProjectRoot,
		Skipped:            skipped,
	})
}

// SwitchProjectRequest is the JSON body for switching projects.
type SwitchProjectRequest struct {
	ProjectPath string `json:"project_path"`
}

// SwitchProjectResponse is the JSON response after switching projects.
type SwitchProjectResponse struct {
	ProjectName string   `json:"project_name"`
	Boards      []string `json:"boards"`
}

// SwitchProject switches the handler's active project context.
func (h *Handler) SwitchProject(w http.ResponseWriter, r *http.Request) {
	var req SwitchProjectRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		BadRequest(w, "invalid JSON body")
		return
	}

	if req.ProjectPath == "" {
		BadRequest(w, "project_path is required")
		return
	}

	// Validate the path is a registered project
	globalCfg, err := h.globalStore.Load()
	if err != nil {
		Error(w, fmt.Errorf("failed to load global config: %w", err))
		return
	}

	dataLocation := ""
	registered := false
	for _, path := range globalCfg.Projects {
		if path == req.ProjectPath {
			registered = true
			break
		}
	}
	if !registered {
		NotFound(w, "project", req.ProjectPath)
		return
	}

	if repoCfg := globalCfg.GetRepoConfig(req.ProjectPath); repoCfg != nil {
		dataLocation = repoCfg.DataLocation
	}

	// Build new context
	newCtx, err := BuildProjectContext(req.ProjectPath, dataLocation, h.ctx().Creator)
	if err != nil {
		Error(w, fmt.Errorf("failed to switch to project: %w", err))
		return
	}

	// Verify the project has at least one board
	boardNames, err := newCtx.BoardStore.List()
	if err != nil || len(boardNames) == 0 {
		BadRequest(w, "project has no boards")
		return
	}

	// Get display name
	projectName := ""
	if projCfg, err := newCtx.ProjectStore.Load(); err == nil && projCfg.Name != "" {
		projectName = projCfg.Name
	}
	if projectName == "" {
		// Fall back to project registry name
		for name, path := range globalCfg.Projects {
			if path == req.ProjectPath {
				projectName = name
				break
			}
		}
	}

	// Swap context
	h.mu.Lock()
	h.current = newCtx
	h.mu.Unlock()

	// Notify server to update file watcher
	if h.onProjectSwitch != nil {
		h.onProjectSwitch(newCtx.ProjectRoot)
	}

	JSON(w, http.StatusOK, SwitchProjectResponse{
		ProjectName: projectName,
		Boards:      boardNames,
	})
}
