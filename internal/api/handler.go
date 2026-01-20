package api

import (
	"encoding/json"
	"net/http"

	"github.com/amterp/kan/internal/config"
	"github.com/amterp/kan/internal/model"
	"github.com/amterp/kan/internal/service"
	"github.com/amterp/kan/internal/store"
)

// CardResponse wraps a Card for JSON API responses, including the Column field
// which is computed (from board config) and not persisted to card files.
// Custom fields are flattened into the top level to match the card JSON storage format.
type CardResponse struct {
	ID              string          `json:"id"`
	Alias           string          `json:"alias"`
	AliasExplicit   bool            `json:"alias_explicit"`
	Title           string          `json:"title"`
	Description     string          `json:"description,omitempty"`
	Column          string          `json:"column"`
	Parent          string          `json:"parent,omitempty"`
	Creator         string          `json:"creator"`
	CreatedAtMillis int64           `json:"created_at_millis"`
	UpdatedAtMillis int64           `json:"updated_at_millis"`
	Comments        []model.Comment `json:"comments,omitempty"`
	CustomFields    map[string]any  `json:"-"` // Flattened into top level by MarshalJSON
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

// toCardResponses converts a slice of model.Card to CardResponses.
func toCardResponses(cards []*model.Card) []CardResponse {
	responses := make([]CardResponse, len(cards))
	for i, card := range cards {
		responses[i] = toCardResponse(card)
	}
	return responses
}

// populateCardColumn sets the Column field on a card by looking up the board config.
func (h *Handler) populateCardColumn(boardName string, card *model.Card) {
	boardCfg, err := h.boardStore.Get(boardName)
	if err != nil {
		return // Leave column empty if board config can't be read
	}
	card.Column = boardCfg.GetCardColumn(card.ID)
}

// Handler contains all HTTP handlers for the API.
type Handler struct {
	cardService  *service.CardService
	boardService *service.BoardService
	cardStore    store.CardStore
	boardStore   store.BoardStore
	projectStore store.ProjectStore
	paths        *config.Paths
	creator      string
}

// NewHandler creates a new handler with the given dependencies.
func NewHandler(
	cardService *service.CardService,
	boardService *service.BoardService,
	cardStore store.CardStore,
	boardStore store.BoardStore,
	projectStore store.ProjectStore,
	paths *config.Paths,
	creator string,
) *Handler {
	return &Handler{
		cardService:  cardService,
		boardService: boardService,
		cardStore:    cardStore,
		boardStore:   boardStore,
		projectStore: projectStore,
		paths:        paths,
		creator:      creator,
	}
}

// RegisterRoutes sets up all API routes on the given mux.
func (h *Handler) RegisterRoutes(mux *http.ServeMux) {
	// Project routes
	mux.HandleFunc("GET /api/v1/project", h.GetProject)
	mux.HandleFunc("GET /favicon.svg", h.GetFavicon)

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
	cfg, err := h.projectStore.Load()
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
	boards, err := h.boardStore.List()
	if err != nil {
		Error(w, err)
		return
	}
	JSON(w, http.StatusOK, map[string][]string{"boards": boards})
}

// GetBoard returns a board's configuration.
func (h *Handler) GetBoard(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	board, err := h.boardStore.Get(name)
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
	if !h.boardStore.Exists(boardName) {
		NotFound(w, "board", boardName)
		return
	}

	cards, err := h.cardService.List(boardName, columnFilter)
	if err != nil {
		Error(w, err)
		return
	}
	JSON(w, http.StatusOK, map[string]any{"cards": toCardResponses(cards)})
}

// CreateCardRequest is the JSON body for creating a card.
type CreateCardRequest struct {
	Title        string            `json:"title"`
	Description  string            `json:"description,omitempty"`
	Column       string            `json:"column,omitempty"`
	Parent       string            `json:"parent,omitempty"`
	CustomFields map[string]string `json:"custom_fields,omitempty"`
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
		Creator:      h.creator,
		CustomFields: req.CustomFields,
	}

	card, err := h.cardService.Add(input)
	if err != nil {
		Error(w, err)
		return
	}

	// Populate Column from board config for API response
	h.populateCardColumn(boardName, card)
	JSON(w, http.StatusCreated, toCardResponse(card))
}

// GetCard returns a single card by ID.
func (h *Handler) GetCard(w http.ResponseWriter, r *http.Request) {
	boardName := r.PathValue("board")
	cardID := r.PathValue("id")

	card, err := h.cardService.FindByIDOrAlias(boardName, cardID)
	if err != nil {
		Error(w, err)
		return
	}

	// Populate Column from board config for API response
	h.populateCardColumn(boardName, card)
	JSON(w, http.StatusOK, toCardResponse(card))
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

	card, err := h.cardService.FindByIDOrAlias(boardName, cardID)
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
		if err := h.cardService.MoveCard(boardName, card.ID, *req.Column); err != nil {
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
		updated, err := h.cardService.Edit(input)
		if err != nil {
			Error(w, err)
			return
		}
		card = updated
		h.populateCardColumn(boardName, card)
	}

	JSON(w, http.StatusOK, toCardResponse(card))
}

// DeleteCard deletes a card.
func (h *Handler) DeleteCard(w http.ResponseWriter, r *http.Request) {
	boardName := r.PathValue("board")
	cardID := r.PathValue("id")

	// First resolve the card ID (might be an alias)
	card, err := h.cardService.FindByIDOrAlias(boardName, cardID)
	if err != nil {
		Error(w, err)
		return
	}

	if err := h.cardService.Delete(boardName, card.ID); err != nil {
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
	card, err := h.cardService.FindByIDOrAlias(boardName, cardID)
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
	if err := h.cardService.MoveCardAt(boardName, card.ID, req.Column, position); err != nil {
		Error(w, err)
		return
	}

	// Set Column to the target column for response
	card.Column = req.Column
	JSON(w, http.StatusOK, toCardResponse(card))
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

	if err := h.boardService.AddColumn(boardName, req.Name, req.Color, position); err != nil {
		Error(w, err)
		return
	}

	// Get the updated board to return the new column
	board, err := h.boardStore.Get(boardName)
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

	deletedCards, err := h.boardService.DeleteColumn(boardName, columnName)
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
		if err := h.boardService.RenameColumn(boardName, columnName, *req.Name); err != nil {
			Error(w, err)
			return
		}
		columnName = *req.Name // Update for subsequent operations
	}

	// Handle color change
	if req.Color != nil {
		if err := h.boardService.UpdateColumnColor(boardName, columnName, *req.Color); err != nil {
			Error(w, err)
			return
		}
	}

	// Return the updated column
	board, err := h.boardStore.Get(boardName)
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

	if err := h.boardService.ReorderColumns(boardName, req.Columns); err != nil {
		Error(w, err)
		return
	}

	// Return the updated board config
	board, err := h.boardStore.Get(boardName)
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

	comment, err := h.cardService.AddComment(boardName, cardID, req.Body, h.creator)
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

	comment, err := h.cardService.EditComment(boardName, commentID, req.Body)
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

	if err := h.cardService.DeleteComment(boardName, commentID); err != nil {
		Error(w, err)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}
