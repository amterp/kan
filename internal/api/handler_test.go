package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/amterp/kan/internal/config"
	"github.com/amterp/kan/internal/model"
	"github.com/amterp/kan/internal/service"
	"github.com/amterp/kan/internal/store"
)

// testAPI provides a complete test environment for API handler tests.
type testAPI struct {
	handler    *Handler
	mux        *http.ServeMux
	cardStore  store.CardStore
	boardStore store.BoardStore
	tempDir    string
}

// setupTestAPI creates a test environment with real stores backed by a temp directory.
func setupTestAPI(t *testing.T) *testAPI {
	t.Helper()

	tempDir, err := os.MkdirTemp("", "kan-api-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}

	paths := config.NewPaths(tempDir, "")
	cardStore := store.NewCardStore(paths)
	boardStore := store.NewBoardStore(paths)
	aliasService := service.NewAliasService(cardStore)
	cardService := service.NewCardService(cardStore, boardStore, aliasService)
	boardService := service.NewBoardService(boardStore)

	handler := NewHandler(cardService, boardService, cardStore, boardStore, "test-user")
	mux := http.NewServeMux()
	handler.RegisterRoutes(mux)

	t.Cleanup(func() {
		os.RemoveAll(tempDir)
	})

	return &testAPI{
		handler:    handler,
		mux:        mux,
		cardStore:  cardStore,
		boardStore: boardStore,
		tempDir:    tempDir,
	}
}

// createBoard creates a test board directly via store.
func (api *testAPI) createBoard(t *testing.T, name string) {
	t.Helper()
	cfg := &model.BoardConfig{
		ID:            "test-board-id",
		Name:          name,
		DefaultColumn: "Backlog",
		Columns: []model.Column{
			{Name: "Backlog", Color: "#6b7280"},
			{Name: "In Progress", Color: "#f59e0b"},
			{Name: "Done", Color: "#10b981"},
		},
		Labels: []model.Label{
			{Name: "bug", Color: "#ef4444"},
			{Name: "feature", Color: "#3b82f6"},
		},
	}
	if err := api.boardStore.Create(cfg); err != nil {
		t.Fatalf("Failed to create board: %v", err)
	}
}

// request makes an HTTP request and returns the response.
func (api *testAPI) request(method, path string, body any) *httptest.ResponseRecorder {
	var bodyReader *bytes.Reader
	if body != nil {
		data, _ := json.Marshal(body)
		bodyReader = bytes.NewReader(data)
	} else {
		bodyReader = bytes.NewReader(nil)
	}

	req := httptest.NewRequest(method, path, bodyReader)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	w := httptest.NewRecorder()
	api.mux.ServeHTTP(w, req)
	return w
}

// decodeJSON decodes the response body into the given target.
func decodeJSON(t *testing.T, w *httptest.ResponseRecorder, target any) {
	t.Helper()
	if err := json.NewDecoder(w.Body).Decode(target); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}
}

// ============================================================================
// Board Endpoint Tests
// ============================================================================

func TestHandler_ListBoards_Empty(t *testing.T) {
	api := setupTestAPI(t)

	w := api.request("GET", "/api/v1/boards", nil)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	var resp map[string][]string
	decodeJSON(t, w, &resp)
	if len(resp["boards"]) != 0 {
		t.Errorf("Expected empty boards list, got %v", resp["boards"])
	}
}

func TestHandler_ListBoards_WithBoards(t *testing.T) {
	api := setupTestAPI(t)
	api.createBoard(t, "main")
	api.createBoard(t, "feature")

	w := api.request("GET", "/api/v1/boards", nil)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	var resp map[string][]string
	decodeJSON(t, w, &resp)
	if len(resp["boards"]) != 2 {
		t.Errorf("Expected 2 boards, got %d", len(resp["boards"]))
	}
}

func TestHandler_GetBoard_Found(t *testing.T) {
	api := setupTestAPI(t)
	api.createBoard(t, "main")

	w := api.request("GET", "/api/v1/boards/main", nil)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	var board model.BoardConfig
	decodeJSON(t, w, &board)
	if board.Name != "main" {
		t.Errorf("Expected board name 'main', got %q", board.Name)
	}
	if len(board.Columns) != 3 {
		t.Errorf("Expected 3 columns, got %d", len(board.Columns))
	}
}

func TestHandler_GetBoard_NotFound(t *testing.T) {
	api := setupTestAPI(t)

	w := api.request("GET", "/api/v1/boards/nonexistent", nil)

	if w.Code != http.StatusNotFound {
		t.Errorf("Expected status 404, got %d", w.Code)
	}

	var resp map[string]string
	decodeJSON(t, w, &resp)
	if resp["error"] == "" {
		t.Error("Expected error message in response")
	}
}

// ============================================================================
// Card Endpoint Tests
// ============================================================================

func TestHandler_ListCards_Empty(t *testing.T) {
	api := setupTestAPI(t)
	api.createBoard(t, "main")

	w := api.request("GET", "/api/v1/boards/main/cards", nil)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	// Verify Content-Type header
	if ct := w.Header().Get("Content-Type"); ct != "application/json" {
		t.Errorf("Expected Content-Type 'application/json', got %q", ct)
	}

	var resp map[string][]*model.Card
	decodeJSON(t, w, &resp)
	if len(resp["cards"]) != 0 {
		t.Errorf("Expected empty cards list, got %d cards", len(resp["cards"]))
	}
}

func TestHandler_ListCards_NonexistentBoard(t *testing.T) {
	api := setupTestAPI(t)
	// Don't create any board

	w := api.request("GET", "/api/v1/boards/nonexistent/cards", nil)

	if w.Code != http.StatusNotFound {
		t.Errorf("Expected status 404, got %d", w.Code)
	}

	var resp map[string]string
	decodeJSON(t, w, &resp)
	if resp["error"] == "" {
		t.Error("Expected error message in response")
	}
}

func TestHandler_CreateCard_Basic(t *testing.T) {
	api := setupTestAPI(t)
	api.createBoard(t, "main")

	body := map[string]any{
		"title":  "Test card",
		"column": "Backlog",
	}
	w := api.request("POST", "/api/v1/boards/main/cards", body)

	if w.Code != http.StatusCreated {
		t.Errorf("Expected status 201, got %d. Body: %s", w.Code, w.Body.String())
	}

	var card CardResponse
	decodeJSON(t, w, &card)
	if card.Title != "Test card" {
		t.Errorf("Expected title 'Test card', got %q", card.Title)
	}
	if card.Column != "Backlog" {
		t.Errorf("Expected column 'Backlog', got %q", card.Column)
	}
	if card.ID == "" {
		t.Error("Expected card ID to be set")
	}
	if card.Alias == "" {
		t.Error("Expected alias to be generated")
	}
}

func TestHandler_CreateCard_WithLabels(t *testing.T) {
	api := setupTestAPI(t)
	api.createBoard(t, "main")

	body := map[string]any{
		"title":  "Bug fix",
		"column": "Backlog",
		"labels": []string{"bug"},
	}
	w := api.request("POST", "/api/v1/boards/main/cards", body)

	if w.Code != http.StatusCreated {
		t.Errorf("Expected status 201, got %d", w.Code)
	}

	var card model.Card
	decodeJSON(t, w, &card)
	if len(card.Labels) != 1 || card.Labels[0] != "bug" {
		t.Errorf("Expected labels ['bug'], got %v", card.Labels)
	}
}

func TestHandler_CreateCard_DefaultColumn(t *testing.T) {
	api := setupTestAPI(t)
	api.createBoard(t, "main")

	body := map[string]any{
		"title": "No column specified",
	}
	w := api.request("POST", "/api/v1/boards/main/cards", body)

	if w.Code != http.StatusCreated {
		t.Errorf("Expected status 201, got %d. Body: %s", w.Code, w.Body.String())
	}

	var card CardResponse
	decodeJSON(t, w, &card)
	if card.Column != "Backlog" {
		t.Errorf("Expected default column 'Backlog', got %q", card.Column)
	}
}

func TestHandler_CreateCard_MissingTitle(t *testing.T) {
	api := setupTestAPI(t)
	api.createBoard(t, "main")

	body := map[string]any{
		"column": "Backlog",
	}
	w := api.request("POST", "/api/v1/boards/main/cards", body)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected status 400, got %d", w.Code)
	}

	var resp map[string]string
	decodeJSON(t, w, &resp)
	if resp["error"] != "title is required" {
		t.Errorf("Expected 'title is required' error, got %q", resp["error"])
	}
}

func TestHandler_CreateCard_InvalidColumn(t *testing.T) {
	api := setupTestAPI(t)
	api.createBoard(t, "main")

	body := map[string]any{
		"title":  "Bad column",
		"column": "NonExistent",
	}
	w := api.request("POST", "/api/v1/boards/main/cards", body)

	if w.Code != http.StatusNotFound {
		t.Errorf("Expected status 404, got %d", w.Code)
	}
}

func TestHandler_CreateCard_InvalidLabel(t *testing.T) {
	api := setupTestAPI(t)
	api.createBoard(t, "main")

	body := map[string]any{
		"title":  "Bad label",
		"column": "Backlog",
		"labels": []string{"nonexistent"},
	}
	w := api.request("POST", "/api/v1/boards/main/cards", body)

	if w.Code != http.StatusNotFound {
		t.Errorf("Expected status 404, got %d", w.Code)
	}
}

func TestHandler_CreateCard_InvalidJSON(t *testing.T) {
	api := setupTestAPI(t)
	api.createBoard(t, "main")

	req := httptest.NewRequest("POST", "/api/v1/boards/main/cards", bytes.NewReader([]byte("not json")))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	api.mux.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected status 400, got %d", w.Code)
	}
}

func TestHandler_GetCard_ByID(t *testing.T) {
	api := setupTestAPI(t)
	api.createBoard(t, "main")

	// Create a card first
	body := map[string]any{"title": "Test card", "column": "Backlog"}
	createResp := api.request("POST", "/api/v1/boards/main/cards", body)
	var created model.Card
	decodeJSON(t, createResp, &created)

	// Get by ID
	w := api.request("GET", "/api/v1/boards/main/cards/"+created.ID, nil)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	var card model.Card
	decodeJSON(t, w, &card)
	if card.ID != created.ID {
		t.Errorf("Expected card ID %q, got %q", created.ID, card.ID)
	}
}

func TestHandler_GetCard_ByAlias(t *testing.T) {
	api := setupTestAPI(t)
	api.createBoard(t, "main")

	// Create a card
	body := map[string]any{"title": "Fix login bug", "column": "Backlog"}
	createResp := api.request("POST", "/api/v1/boards/main/cards", body)
	var created model.Card
	decodeJSON(t, createResp, &created)

	// Get by alias
	w := api.request("GET", "/api/v1/boards/main/cards/fix-login-bug", nil)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d. Body: %s", w.Code, w.Body.String())
	}

	var card model.Card
	decodeJSON(t, w, &card)
	if card.ID != created.ID {
		t.Errorf("Expected card ID %q, got %q", created.ID, card.ID)
	}
}

func TestHandler_GetCard_NotFound(t *testing.T) {
	api := setupTestAPI(t)
	api.createBoard(t, "main")

	w := api.request("GET", "/api/v1/boards/main/cards/nonexistent", nil)

	if w.Code != http.StatusNotFound {
		t.Errorf("Expected status 404, got %d", w.Code)
	}
}

func TestHandler_UpdateCard_Title(t *testing.T) {
	api := setupTestAPI(t)
	api.createBoard(t, "main")

	// Create a card
	body := map[string]any{"title": "Original", "column": "Backlog"}
	createResp := api.request("POST", "/api/v1/boards/main/cards", body)
	var created model.Card
	decodeJSON(t, createResp, &created)

	// Update title
	updateBody := map[string]any{"title": "Updated title"}
	w := api.request("PUT", "/api/v1/boards/main/cards/"+created.ID, updateBody)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d. Body: %s", w.Code, w.Body.String())
	}

	var updated model.Card
	decodeJSON(t, w, &updated)
	if updated.Title != "Updated title" {
		t.Errorf("Expected title 'Updated title', got %q", updated.Title)
	}
	// Alias should be regenerated
	if updated.Alias != "updated-title" {
		t.Errorf("Expected alias 'updated-title', got %q", updated.Alias)
	}
}

func TestHandler_UpdateCard_Column(t *testing.T) {
	api := setupTestAPI(t)
	api.createBoard(t, "main")

	// Create a card
	body := map[string]any{"title": "Test", "column": "Backlog"}
	createResp := api.request("POST", "/api/v1/boards/main/cards", body)
	var created CardResponse
	decodeJSON(t, createResp, &created)

	// Move to different column via update
	updateBody := map[string]any{"column": "In Progress"}
	w := api.request("PUT", "/api/v1/boards/main/cards/"+created.ID, updateBody)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d. Body: %s", w.Code, w.Body.String())
	}

	var updated CardResponse
	decodeJSON(t, w, &updated)
	if updated.Column != "In Progress" {
		t.Errorf("Expected column 'In Progress', got %q", updated.Column)
	}
}

func TestHandler_UpdateCard_Description(t *testing.T) {
	api := setupTestAPI(t)
	api.createBoard(t, "main")

	// Create a card
	body := map[string]any{"title": "Test", "column": "Backlog"}
	createResp := api.request("POST", "/api/v1/boards/main/cards", body)
	var created model.Card
	decodeJSON(t, createResp, &created)

	// Update description
	updateBody := map[string]any{"description": "New description"}
	w := api.request("PUT", "/api/v1/boards/main/cards/"+created.ID, updateBody)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	var updated model.Card
	decodeJSON(t, w, &updated)
	if updated.Description != "New description" {
		t.Errorf("Expected description 'New description', got %q", updated.Description)
	}
}

func TestHandler_UpdateCard_NotFound(t *testing.T) {
	api := setupTestAPI(t)
	api.createBoard(t, "main")

	updateBody := map[string]any{"title": "Updated"}
	w := api.request("PUT", "/api/v1/boards/main/cards/nonexistent", updateBody)

	if w.Code != http.StatusNotFound {
		t.Errorf("Expected status 404, got %d", w.Code)
	}
}

func TestHandler_DeleteCard(t *testing.T) {
	api := setupTestAPI(t)
	api.createBoard(t, "main")

	// Create a card
	body := map[string]any{"title": "To delete", "column": "Backlog"}
	createResp := api.request("POST", "/api/v1/boards/main/cards", body)
	var created model.Card
	decodeJSON(t, createResp, &created)

	// Delete it
	w := api.request("DELETE", "/api/v1/boards/main/cards/"+created.ID, nil)

	if w.Code != http.StatusNoContent {
		t.Errorf("Expected status 204, got %d", w.Code)
	}

	// Verify it's gone
	getResp := api.request("GET", "/api/v1/boards/main/cards/"+created.ID, nil)
	if getResp.Code != http.StatusNotFound {
		t.Errorf("Expected card to be deleted, got status %d", getResp.Code)
	}
}

func TestHandler_DeleteCard_ByAlias(t *testing.T) {
	api := setupTestAPI(t)
	api.createBoard(t, "main")

	// Create a card
	body := map[string]any{"title": "Delete me", "column": "Backlog"}
	api.request("POST", "/api/v1/boards/main/cards", body)

	// Delete by alias
	w := api.request("DELETE", "/api/v1/boards/main/cards/delete-me", nil)

	if w.Code != http.StatusNoContent {
		t.Errorf("Expected status 204, got %d. Body: %s", w.Code, w.Body.String())
	}
}

func TestHandler_DeleteCard_NotFound(t *testing.T) {
	api := setupTestAPI(t)
	api.createBoard(t, "main")

	w := api.request("DELETE", "/api/v1/boards/main/cards/nonexistent", nil)

	if w.Code != http.StatusNotFound {
		t.Errorf("Expected status 404, got %d", w.Code)
	}
}

func TestHandler_MoveCard_ToColumn(t *testing.T) {
	api := setupTestAPI(t)
	api.createBoard(t, "main")

	// Create a card
	body := map[string]any{"title": "Movable", "column": "Backlog"}
	createResp := api.request("POST", "/api/v1/boards/main/cards", body)
	var created CardResponse
	decodeJSON(t, createResp, &created)

	// Move it
	moveBody := map[string]any{"column": "Done"}
	w := api.request("PATCH", "/api/v1/boards/main/cards/"+created.ID+"/move", moveBody)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d. Body: %s", w.Code, w.Body.String())
	}

	var moved CardResponse
	decodeJSON(t, w, &moved)
	if moved.Column != "Done" {
		t.Errorf("Expected column 'Done', got %q", moved.Column)
	}
}

func TestHandler_MoveCard_WithPosition(t *testing.T) {
	api := setupTestAPI(t)
	api.createBoard(t, "main")

	// Create two cards in Done
	firstResp := api.request("POST", "/api/v1/boards/main/cards", map[string]any{"title": "First", "column": "Done"})
	var first CardResponse
	decodeJSON(t, firstResp, &first)
	secondResp := api.request("POST", "/api/v1/boards/main/cards", map[string]any{"title": "Second", "column": "Done"})
	var second CardResponse
	decodeJSON(t, secondResp, &second)

	// Create a card in Backlog
	createResp := api.request("POST", "/api/v1/boards/main/cards", map[string]any{"title": "Third", "column": "Backlog"})
	var third CardResponse
	decodeJSON(t, createResp, &third)

	// Move to position 0 in Done
	position := 0
	moveBody := map[string]any{"column": "Done", "position": position}
	w := api.request("PATCH", "/api/v1/boards/main/cards/"+third.ID+"/move", moveBody)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d. Body: %s", w.Code, w.Body.String())
	}

	var moved CardResponse
	decodeJSON(t, w, &moved)
	if moved.Column != "Done" {
		t.Errorf("Expected column 'Done', got %q", moved.Column)
	}

	// Verify order in Done column: Third, First, Second
	listResp := api.request("GET", "/api/v1/boards/main/cards?column=Done", nil)
	var listResult map[string][]CardResponse
	decodeJSON(t, listResp, &listResult)
	cards := listResult["cards"]
	if len(cards) != 3 {
		t.Fatalf("Expected 3 cards in Done, got %d", len(cards))
	}
	if cards[0].ID != third.ID {
		t.Errorf("Expected Third at position 0, got %s", cards[0].Title)
	}
	if cards[1].ID != first.ID {
		t.Errorf("Expected First at position 1, got %s", cards[1].Title)
	}
	if cards[2].ID != second.ID {
		t.Errorf("Expected Second at position 2, got %s", cards[2].Title)
	}
}

func TestHandler_MoveCard_InvalidColumn(t *testing.T) {
	api := setupTestAPI(t)
	api.createBoard(t, "main")

	// Create a card
	body := map[string]any{"title": "Test", "column": "Backlog"}
	createResp := api.request("POST", "/api/v1/boards/main/cards", body)
	var created model.Card
	decodeJSON(t, createResp, &created)

	// Try to move to invalid column
	moveBody := map[string]any{"column": "NonExistent"}
	w := api.request("PATCH", "/api/v1/boards/main/cards/"+created.ID+"/move", moveBody)

	if w.Code != http.StatusNotFound {
		t.Errorf("Expected status 404, got %d", w.Code)
	}
}

func TestHandler_MoveCard_MissingColumn(t *testing.T) {
	api := setupTestAPI(t)
	api.createBoard(t, "main")

	// Create a card
	body := map[string]any{"title": "Test", "column": "Backlog"}
	createResp := api.request("POST", "/api/v1/boards/main/cards", body)
	var created model.Card
	decodeJSON(t, createResp, &created)

	// Try to move without specifying column
	moveBody := map[string]any{}
	w := api.request("PATCH", "/api/v1/boards/main/cards/"+created.ID+"/move", moveBody)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected status 400, got %d", w.Code)
	}

	var resp map[string]string
	decodeJSON(t, w, &resp)
	if resp["error"] != "column is required" {
		t.Errorf("Expected 'column is required' error, got %q", resp["error"])
	}
}

func TestHandler_MoveCard_NotFound(t *testing.T) {
	api := setupTestAPI(t)
	api.createBoard(t, "main")

	moveBody := map[string]any{"column": "Done"}
	w := api.request("PATCH", "/api/v1/boards/main/cards/nonexistent/move", moveBody)

	if w.Code != http.StatusNotFound {
		t.Errorf("Expected status 404, got %d", w.Code)
	}
}

func TestHandler_ListCards_WithColumnFilter(t *testing.T) {
	api := setupTestAPI(t)
	api.createBoard(t, "main")

	// Create cards in different columns
	api.request("POST", "/api/v1/boards/main/cards", map[string]any{"title": "Backlog 1", "column": "Backlog"})
	api.request("POST", "/api/v1/boards/main/cards", map[string]any{"title": "Backlog 2", "column": "Backlog"})
	api.request("POST", "/api/v1/boards/main/cards", map[string]any{"title": "Done 1", "column": "Done"})

	// List only Backlog cards
	w := api.request("GET", "/api/v1/boards/main/cards?column=Backlog", nil)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	var resp map[string][]CardResponse
	decodeJSON(t, w, &resp)
	if len(resp["cards"]) != 2 {
		t.Errorf("Expected 2 cards in Backlog, got %d", len(resp["cards"]))
	}
	for _, card := range resp["cards"] {
		if card.Column != "Backlog" {
			t.Errorf("Expected column 'Backlog', got %q", card.Column)
		}
	}
}
