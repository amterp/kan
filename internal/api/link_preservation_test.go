package api

import (
	"encoding/json"
	"net/http"
	"reflect"
	"testing"

	"github.com/amterp/kan/internal/model"
)

func TestHandler_PreservesAndReturnsLinks(t *testing.T) {
	api := setupTestAPI(t)
	api.createBoard(t, "main")
	card := createParentTestCard(t, api, "Linked card", "")
	card.BlockedBy, card.Related = []string{"blocker"}, []string{"related-card"}
	if err := api.cardStore.Update("main", card); err != nil {
		t.Fatal(err)
	}
	for _, request := range []struct {
		method, suffix string
		body           any
	}{
		{method: "GET"},
		{method: "PUT", body: map[string]string{"title": "Renamed"}},
		{method: "PATCH", suffix: "/move", body: map[string]string{"column": "done"}},
	} {
		response := api.request(request.method, "/api/v1/boards/main/cards/"+card.ID+request.suffix, request.body)
		if response.Code != http.StatusOK {
			t.Fatalf("%s returned %d: %s", request.method, response.Code, response.Body.String())
		}
		var raw map[string]json.RawMessage
		if err := json.Unmarshal(response.Body.Bytes(), &raw); err != nil {
			t.Fatal(err)
		}
		var blocked, related []string
		_ = json.Unmarshal(raw["blocked_by"], &blocked)
		_ = json.Unmarshal(raw["related"], &related)
		if !reflect.DeepEqual(blocked, card.BlockedBy) || !reflect.DeepEqual(related, card.Related) {
			t.Fatalf("%s response dropped links: %s", request.method, response.Body.String())
		}
		stored, err := api.cardStore.Get("main", card.ID)
		if err != nil || !reflect.DeepEqual(stored.BlockedBy, card.BlockedBy) || !reflect.DeepEqual(stored.Related, card.Related) {
			t.Fatalf("%s write dropped links: %+v, %v", request.method, stored, err)
		}
	}
	if err := api.cardStore.Delete("main", card.ID); err != nil {
		t.Fatal(err)
	}
	response := api.request("POST", "/api/v1/boards/main/cards/restore", map[string]any{"card": card, "column": "backlog", "position": -1})
	if response.Code != http.StatusCreated && response.Code != http.StatusOK {
		t.Fatalf("restore returned %d: %s", response.Code, response.Body.String())
	}
	stored, err := api.cardStore.Get("main", card.ID)
	if err != nil || !reflect.DeepEqual(stored.BlockedBy, card.BlockedBy) || !reflect.DeepEqual(stored.Related, card.Related) {
		t.Fatalf("restore dropped links: %+v, %v", stored, err)
	}
	var decoded model.Card
	data, err := json.Marshal(stored)
	if err != nil || json.Unmarshal(data, &decoded) != nil || !reflect.DeepEqual(decoded.BlockedBy, card.BlockedBy) {
		t.Fatal("model link round trip failed")
	}
}
