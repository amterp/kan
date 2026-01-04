package api

import (
	"encoding/json"
	"net/http"

	"github.com/amterp/kan/internal/model"
	"github.com/amterp/kan/internal/service"
	"github.com/amterp/kan/internal/store"
)

// CardResponse wraps a Card for JSON API responses, including the Column field
// which is computed (from board config) and not persisted to card files.
type CardResponse struct {
	ID              string          `json:"id"`
	Alias           string          `json:"alias"`
	AliasExplicit   bool            `json:"alias_explicit"`
	Title           string          `json:"title"`
	Description     string          `json:"description,omitempty"`
	Column          string          `json:"column"`
	Labels          []string        `json:"labels,omitempty"`
	Parent          string          `json:"parent,omitempty"`
	Creator         string          `json:"creator"`
	CreatedAtMillis int64           `json:"created_at_millis"`
	UpdatedAtMillis int64           `json:"updated_at_millis"`
	Comments        []model.Comment `json:"comments,omitempty"`
	CustomFields    map[string]any  `json:"custom_fields,omitempty"`
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
		Labels:          card.Labels,
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
	creator      string
}

// NewHandler creates a new handler with the given dependencies.
func NewHandler(
	cardService *service.CardService,
	boardService *service.BoardService,
	cardStore store.CardStore,
	boardStore store.BoardStore,
	creator string,
) *Handler {
	return &Handler{
		cardService:  cardService,
		boardService: boardService,
		cardStore:    cardStore,
		boardStore:   boardStore,
		creator:      creator,
	}
}

// RegisterRoutes sets up all API routes on the given mux.
func (h *Handler) RegisterRoutes(mux *http.ServeMux) {
	// Board routes
	mux.HandleFunc("GET /api/v1/boards", h.ListBoards)
	mux.HandleFunc("GET /api/v1/boards/{name}", h.GetBoard)

	// Card routes
	mux.HandleFunc("GET /api/v1/boards/{board}/cards", h.ListCards)
	mux.HandleFunc("POST /api/v1/boards/{board}/cards", h.CreateCard)
	mux.HandleFunc("GET /api/v1/boards/{board}/cards/{id}", h.GetCard)
	mux.HandleFunc("PUT /api/v1/boards/{board}/cards/{id}", h.UpdateCard)
	mux.HandleFunc("DELETE /api/v1/boards/{board}/cards/{id}", h.DeleteCard)
	mux.HandleFunc("PATCH /api/v1/boards/{board}/cards/{id}/move", h.MoveCard)

	// Static files (frontend)
	mux.Handle("/", h.StaticHandler())
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
	Title       string   `json:"title"`
	Description string   `json:"description,omitempty"`
	Column      string   `json:"column,omitempty"`
	Labels      []string `json:"labels,omitempty"`
	Parent      string   `json:"parent,omitempty"`
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
		BoardName:   boardName,
		Title:       req.Title,
		Description: req.Description,
		Column:      req.Column,
		Labels:      req.Labels,
		Parent:      req.Parent,
		Creator:     h.creator,
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
	Title       *string  `json:"title,omitempty"`
	Description *string  `json:"description,omitempty"`
	Column      *string  `json:"column,omitempty"`
	Labels      []string `json:"labels,omitempty"`
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

	// Apply other updates
	needsUpdate := false
	if req.Title != nil {
		if err := h.cardService.UpdateTitle(boardName, card, *req.Title); err != nil {
			Error(w, err)
			return
		}
		// Re-fetch after title update (which regenerates alias)
		card, err = h.cardService.Get(boardName, card.ID)
		if err != nil {
			Error(w, err)
			return
		}
		h.populateCardColumn(boardName, card)
	}

	if req.Description != nil && *req.Description != card.Description {
		card.Description = *req.Description
		needsUpdate = true
	}
	if req.Labels != nil {
		card.Labels = req.Labels
		needsUpdate = true
	}

	if needsUpdate {
		if err := h.cardService.Update(boardName, card); err != nil {
			Error(w, err)
			return
		}
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
