package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/amterp/kan/internal/model"
)

func createParentTestCard(t *testing.T, api *testAPI, title, parent string) *model.Card {
	t.Helper()
	response := api.request("POST", "/api/v1/boards/main/cards", map[string]string{"title": title, "parent": parent})
	if response.Code != http.StatusCreated {
		t.Fatalf("create returned %d: %s", response.Code, response.Body.String())
	}
	card := createCardFromResponse(t, response)
	return &card
}

func TestHandler_ParentWritesStoreIDs(t *testing.T) {
	api := setupTestAPI(t)
	api.createBoard(t, "main")
	parent := createParentTestCard(t, api, "Parent", "")
	child := createParentTestCard(t, api, "Child", parent.Alias)
	stored, err := api.cardStore.Get("main", child.ID)
	if err != nil || stored.Parent != parent.ID || child.Parent != parent.ID {
		t.Fatalf("create did not store parent ID: %+v, %v", stored, err)
	}
	other := createParentTestCard(t, api, "Other", "")
	response := api.request("PUT", "/api/v1/boards/main/cards/"+child.ID, map[string]string{"parent": other.Alias, "column": "done"})
	if response.Code != http.StatusOK {
		t.Fatalf("update returned %d: %s", response.Code, response.Body.String())
	}
	var updated CardResponse
	if err := json.Unmarshal(response.Body.Bytes(), &updated); err != nil {
		t.Fatal(err)
	}
	stored, err = api.cardStore.Get("main", child.ID)
	if err != nil || stored.Parent != other.ID || updated.Parent != other.ID || stored.Column != "done" {
		t.Fatalf("update did not store parent ID and move: %+v, %v", stored, err)
	}
	response = api.request("PUT", "/api/v1/boards/main/cards/"+child.ID, map[string]string{"parent": ""})
	if response.Code != http.StatusOK {
		t.Fatal(response.Body.String())
	}
	stored, err = api.cardStore.Get("main", child.ID)
	if err != nil || stored.Parent != "" {
		t.Fatalf("clearing parent failed: %+v, %v", stored, err)
	}
}

func TestHandler_ParentWritesRejectInvalidReferences(t *testing.T) {
	for _, ref := range []string{"self-id", "self-alias", "cycle", "missing"} {
		t.Run(ref, func(t *testing.T) {
			api := setupTestAPI(t)
			api.createBoard(t, "main")
			parent := createParentTestCard(t, api, "Parent", "")
			child := createParentTestCard(t, api, "Child", parent.ID)
			refs := map[string]string{"self-id": parent.ID, "self-alias": parent.Alias, "cycle": child.Alias, "missing": "missing"}
			cardPath := filepath.Join(api.tempDir, ".kan", "boards", "main", "cards", parent.ID+".json")
			before, err := os.ReadFile(cardPath)
			if err != nil {
				t.Fatal(err)
			}
			response := api.request("PUT", "/api/v1/boards/main/cards/"+parent.ID,
				map[string]string{"parent": refs[ref], "title": "Changed", "column": "done"})
			if response.Code != http.StatusBadRequest || !strings.Contains(response.Body.String(), "parent") {
				t.Fatalf("expected parent validation failure, got %d: %s", response.Code, response.Body.String())
			}
			after, err := os.ReadFile(cardPath)
			if err != nil {
				t.Fatal(err)
			}
			if !bytes.Equal(before, after) {
				t.Fatal("rejected parent update changed the card file")
			}
		})
	}
}

func TestHandler_CreateCardRejectsMissingParent(t *testing.T) {
	api := setupTestAPI(t)
	api.createBoard(t, "main")
	response := api.request("POST", "/api/v1/boards/main/cards", map[string]string{"title": "Child", "parent": "missing"})
	if response.Code != http.StatusBadRequest {
		t.Fatalf("expected validation error, got %d: %s", response.Code, response.Body.String())
	}
	cards, err := api.cardStore.List("main")
	if err != nil || len(cards) != 0 {
		t.Fatalf("failed create left cards: %+v, %v", cards, err)
	}
}
