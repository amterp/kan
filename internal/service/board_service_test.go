package service

import (
	"testing"

	kanerr "github.com/amterp/kan/internal/errors"
	"github.com/amterp/kan/internal/store"
)

// We reuse testBoardStore and testCardStore from card_service_test.go
var _ store.BoardStore = (*testBoardStore)(nil)
var _ store.CardStore = (*testCardStore)(nil)

// ============================================================================
// BoardService Tests
// ============================================================================

func TestBoardService_Create_Basic(t *testing.T) {
	boardStore := newTestBoardStore()
	cardStore := newTestCardStore()
	service := NewBoardService(boardStore, cardStore)

	if err := service.Create("main"); err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	// Verify board was created with default columns
	cfg, err := boardStore.Get("main")
	if err != nil {
		t.Fatalf("Failed to get created board: %v", err)
	}
	if cfg.Name != "main" {
		t.Errorf("Expected name 'main', got %q", cfg.Name)
	}
	if len(cfg.Columns) != 4 {
		t.Errorf("Expected 4 default columns, got %d", len(cfg.Columns))
	}
	if cfg.DefaultColumn != "backlog" {
		t.Errorf("Expected default column 'backlog', got %q", cfg.DefaultColumn)
	}
	if cfg.ID == "" {
		t.Error("Expected ID to be generated")
	}
}

func TestBoardService_Create_AlreadyExists(t *testing.T) {
	boardStore := newTestBoardStore()
	cardStore := newTestCardStore()
	service := NewBoardService(boardStore, cardStore)

	// Create first time
	if err := service.Create("main"); err != nil {
		t.Fatalf("First create failed: %v", err)
	}

	// Create second time - should fail
	err := service.Create("main")
	if err == nil {
		t.Fatal("Expected error for duplicate board")
	}
	if !kanerr.IsAlreadyExists(err) {
		t.Errorf("Expected AlreadyExists error, got %v", err)
	}
}

func TestBoardService_List_Empty(t *testing.T) {
	boardStore := newTestBoardStore()
	cardStore := newTestCardStore()
	service := NewBoardService(boardStore, cardStore)

	boards, err := service.List()
	if err != nil {
		t.Fatalf("List failed: %v", err)
	}
	if len(boards) != 0 {
		t.Errorf("Expected 0 boards, got %d", len(boards))
	}
}

func TestBoardService_List_WithBoards(t *testing.T) {
	boardStore := newTestBoardStore()
	cardStore := newTestCardStore()
	service := NewBoardService(boardStore, cardStore)

	service.Create("main")
	service.Create("feature")
	service.Create("bugfix")

	boards, err := service.List()
	if err != nil {
		t.Fatalf("List failed: %v", err)
	}
	if len(boards) != 3 {
		t.Errorf("Expected 3 boards, got %d", len(boards))
	}
}

func TestBoardService_Get_Found(t *testing.T) {
	boardStore := newTestBoardStore()
	cardStore := newTestCardStore()
	service := NewBoardService(boardStore, cardStore)

	service.Create("main")

	cfg, err := service.Get("main")
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}
	if cfg.Name != "main" {
		t.Errorf("Expected name 'main', got %q", cfg.Name)
	}
}

func TestBoardService_Get_NotFound(t *testing.T) {
	boardStore := newTestBoardStore()
	cardStore := newTestCardStore()
	service := NewBoardService(boardStore, cardStore)

	_, err := service.Get("nonexistent")
	if err == nil {
		t.Fatal("Expected error for nonexistent board")
	}
	if !kanerr.IsNotFound(err) {
		t.Errorf("Expected NotFound error, got %v", err)
	}
}

func TestBoardService_Exists_True(t *testing.T) {
	boardStore := newTestBoardStore()
	cardStore := newTestCardStore()
	service := NewBoardService(boardStore, cardStore)

	service.Create("main")

	if !service.Exists("main") {
		t.Error("Expected Exists to return true for existing board")
	}
}

func TestBoardService_Exists_False(t *testing.T) {
	boardStore := newTestBoardStore()
	cardStore := newTestCardStore()
	service := NewBoardService(boardStore, cardStore)

	if service.Exists("nonexistent") {
		t.Error("Expected Exists to return false for nonexistent board")
	}
}

func TestBoardService_Create_DefaultColumns(t *testing.T) {
	boardStore := newTestBoardStore()
	cardStore := newTestCardStore()
	service := NewBoardService(boardStore, cardStore)

	service.Create("main")

	cfg, _ := service.Get("main")

	// Verify default column structure
	expectedColumns := []string{"backlog", "next", "in-progress", "done"}
	if len(cfg.Columns) != len(expectedColumns) {
		t.Fatalf("Expected %d columns, got %d", len(expectedColumns), len(cfg.Columns))
	}
	for i, expected := range expectedColumns {
		if cfg.Columns[i].Name != expected {
			t.Errorf("Expected column %d to be %q, got %q", i, expected, cfg.Columns[i].Name)
		}
	}
}
