import { useState, useCallback, useEffect, useMemo } from 'react';
import { BrowserRouter, Routes, Route } from 'react-router-dom';
import { useBoards, useBoard } from './hooks/useBoards';
import { useOmnibar } from './hooks/useOmnibar';
import { cardMatchesQuery } from './utils/fuzzyMatch';
import Header from './components/Header';
import Board from './components/Board';
import CardEditModal from './components/CardEditModal';
import Omnibar, { type NavigationDirection } from './components/Omnibar';
import DocsPage from './pages/DocsPage';
import type { Card, UpdateCardInput } from './api/types';

function BoardApp() {
  const { boards, loading: boardsLoading, error: boardsError } = useBoards();
  const [selectedBoard, setSelectedBoard] = useState<string | null>(null);
  const {
    board,
    cards,
    loading,
    error,
    moveCard,
    createCard,
    updateCard,
    deleteCard,
    createColumn,
    deleteColumn,
    updateColumn,
    reorderColumns,
    refresh,
  } = useBoard(selectedBoard);
  const [newCardForEdit, setNewCardForEdit] = useState<Card | null>(null);
  const [omnibarSelectedCard, setOmnibarSelectedCard] = useState<Card | null>(null);
  const omnibar = useOmnibar();

  // Compute filtered cards for navigation
  const filteredCards = useMemo(() => {
    if (!board || !omnibar.query.trim()) return cards;
    return cards.filter((card) => cardMatchesQuery(card, omnibar.query.trim(), board));
  }, [cards, omnibar.query, board]);

  // Group filtered cards by column (in column order) for navigation
  const filteredCardsByColumn = useMemo(() => {
    if (!board) return [];
    return board.columns
      .map((col) => ({
        column: col,
        cards: filteredCards.filter((c) => c.column === col.name),
      }))
      .filter((group) => group.cards.length > 0);
  }, [board, filteredCards]);

  // Auto-highlight first card when omnibar opens or filter changes
  useEffect(() => {
    if (omnibar.isOpen && filteredCards.length > 0) {
      // If no highlight or highlighted card no longer in filtered results, highlight first
      const currentHighlight = omnibar.highlightedCardId;
      const highlightStillValid = currentHighlight && filteredCards.some((c) => c.id === currentHighlight);
      if (!highlightStillValid) {
        omnibar.setHighlightedCardId(filteredCards[0].id);
      }
    } else if (omnibar.isOpen && filteredCards.length === 0) {
      omnibar.setHighlightedCardId(null);
    }
  }, [omnibar, filteredCards]);

  // Handle arrow key navigation
  const handleNavigate = useCallback(
    (direction: NavigationDirection) => {
      if (filteredCardsByColumn.length === 0) return;

      const currentId = omnibar.highlightedCardId;
      if (!currentId) {
        // No highlight yet, highlight first card
        omnibar.setHighlightedCardId(filteredCardsByColumn[0].cards[0].id);
        return;
      }

      // Find current position
      let currentColIdx = -1;
      let currentCardIdx = -1;
      for (let ci = 0; ci < filteredCardsByColumn.length; ci++) {
        const cardIdx = filteredCardsByColumn[ci].cards.findIndex((c) => c.id === currentId);
        if (cardIdx !== -1) {
          currentColIdx = ci;
          currentCardIdx = cardIdx;
          break;
        }
      }

      if (currentColIdx === -1) return;

      const currentColumn = filteredCardsByColumn[currentColIdx];
      let nextCardId: string | null = null;

      switch (direction) {
        case 'up': {
          if (currentCardIdx > 0) {
            nextCardId = currentColumn.cards[currentCardIdx - 1].id;
          }
          // At top of column, stay put
          break;
        }
        case 'down': {
          if (currentCardIdx < currentColumn.cards.length - 1) {
            nextCardId = currentColumn.cards[currentCardIdx + 1].id;
          }
          // At bottom of column, stay put
          break;
        }
        case 'left': {
          if (currentColIdx > 0) {
            const prevColumn = filteredCardsByColumn[currentColIdx - 1];
            // Same index clamped to column length
            const targetIdx = Math.min(currentCardIdx, prevColumn.cards.length - 1);
            nextCardId = prevColumn.cards[targetIdx].id;
          }
          break;
        }
        case 'right': {
          if (currentColIdx < filteredCardsByColumn.length - 1) {
            const nextColumn = filteredCardsByColumn[currentColIdx + 1];
            // Same index clamped to column length
            const targetIdx = Math.min(currentCardIdx, nextColumn.cards.length - 1);
            nextCardId = nextColumn.cards[targetIdx].id;
          }
          break;
        }
      }

      if (nextCardId) {
        omnibar.setHighlightedCardId(nextCardId);
      }
    },
    [filteredCardsByColumn, omnibar]
  );

  // Handle Enter to select highlighted card (opens modal but keeps omnibar open)
  const handleSelectCard = useCallback(() => {
    if (!omnibar.highlightedCardId) return;
    const card = cards.find((c) => c.id === omnibar.highlightedCardId);
    if (card) {
      setOmnibarSelectedCard(card);
      // Don't close omnibar - it stays open behind the modal
      // Focus will return to omnibar when modal closes
    }
  }, [omnibar, cards]);

  // Cmd+K keyboard shortcut for omnibar
  useEffect(() => {
    const handleKeyDown = (e: KeyboardEvent) => {
      if ((e.metaKey || e.ctrlKey) && e.key === 'k') {
        e.preventDefault();
        // Don't open omnibar if CardEditModal is open
        if (newCardForEdit || omnibarSelectedCard) return;
        if (omnibar.isOpen) {
          omnibar.close();
        } else {
          omnibar.open();
        }
      }
    };

    document.addEventListener('keydown', handleKeyDown);
    return () => document.removeEventListener('keydown', handleKeyDown);
  }, [omnibar, newCardForEdit, omnibarSelectedCard]);

  const handleNewCard = useCallback(async () => {
    if (!board) return;
    const newCard = await createCard({ title: 'New Card', column: board.default_column });
    if (newCard) {
      setNewCardForEdit(newCard);
    }
  }, [board, createCard]);

  const handleSaveNewCard = useCallback(async (updates: UpdateCardInput) => {
    if (newCardForEdit) {
      await updateCard(newCardForEdit.id, updates);
      setNewCardForEdit(null);
    }
  }, [newCardForEdit, updateCard]);

  // Auto-select first board if only one exists
  if (!selectedBoard && boards.length === 1) {
    setSelectedBoard(boards[0]);
  }

  if (boardsLoading) {
    return (
      <div className="h-screen flex items-center justify-center bg-gray-100 dark:bg-gray-900">
        <p className="text-gray-500 dark:text-gray-400">Loading...</p>
      </div>
    );
  }

  if (boardsError) {
    return (
      <div className="h-screen flex items-center justify-center bg-gray-100 dark:bg-gray-900">
        <div className="text-center">
          <p className="text-red-500 mb-2">Error: {boardsError}</p>
          <p className="text-gray-500 dark:text-gray-400 text-sm">Make sure Kan is initialized in this repository.</p>
        </div>
      </div>
    );
  }

  if (boards.length === 0) {
    return (
      <div className="h-screen flex items-center justify-center bg-gray-100 dark:bg-gray-900">
        <div className="text-center">
          <p className="text-gray-700 dark:text-gray-300 mb-2">No boards found</p>
          <p className="text-gray-500 dark:text-gray-400 text-sm">Run <code className="bg-gray-200 dark:bg-gray-700 px-1 rounded">kan init</code> to get started.</p>
        </div>
      </div>
    );
  }

  return (
    <div className="h-screen flex flex-col bg-gray-100 dark:bg-gray-900">
      <Header
        boards={boards}
        selectedBoard={selectedBoard}
        onSelectBoard={setSelectedBoard}
        onRefresh={refresh}
        onNewCard={board ? handleNewCard : undefined}
      />
      <main className="flex-1 overflow-hidden">
        {loading ? (
          <div className="h-full flex items-center justify-center">
            <p className="text-gray-500 dark:text-gray-400">Loading board...</p>
          </div>
        ) : error ? (
          <div className="h-full flex items-center justify-center">
            <p className="text-red-500">{error}</p>
          </div>
        ) : board ? (
          <Board
            board={board}
            cards={cards}
            filterQuery={omnibar.query}
            highlightedCardId={omnibar.isOpen ? omnibar.highlightedCardId : null}
            onMoveCard={moveCard}
            onCreateCard={createCard}
            onUpdateCard={updateCard}
            onDeleteCard={deleteCard}
            onCreateColumn={createColumn}
            onDeleteColumn={deleteColumn}
            onUpdateColumn={updateColumn}
            onReorderColumns={reorderColumns}
          />
        ) : (
          <div className="h-full flex items-center justify-center">
            <p className="text-gray-500 dark:text-gray-400">Select a board to get started</p>
          </div>
        )}
      </main>
      {newCardForEdit && board && (
        <CardEditModal
          card={newCardForEdit}
          board={board}
          onSave={handleSaveNewCard}
          onDelete={async () => {
            await deleteCard(newCardForEdit.id);
            setNewCardForEdit(null);
          }}
          onClose={() => setNewCardForEdit(null)}
        />
      )}
      {omnibarSelectedCard && board && (
        <CardEditModal
          card={omnibarSelectedCard}
          board={board}
          onSave={async (updates) => {
            await updateCard(omnibarSelectedCard.id, updates);
            setOmnibarSelectedCard(null);
          }}
          onDelete={async () => {
            await deleteCard(omnibarSelectedCard.id);
            setOmnibarSelectedCard(null);
          }}
          onClose={() => setOmnibarSelectedCard(null)}
        />
      )}
      {omnibar.isOpen && (
        <Omnibar
          query={omnibar.query}
          matchCount={filteredCards.length}
          totalCount={cards.length}
          hasHighlight={omnibar.highlightedCardId !== null}
          isModalOpen={omnibarSelectedCard !== null}
          onQueryChange={omnibar.setQuery}
          onNavigate={handleNavigate}
          onSelect={handleSelectCard}
          onClose={omnibar.close}
        />
      )}
    </div>
  );
}

function App() {
  // Remove trailing slash from base URL for BrowserRouter basename
  const basename = import.meta.env.BASE_URL.replace(/\/$/, '') || undefined;

  return (
    <BrowserRouter basename={basename}>
      <Routes>
        <Route path="/docs/*" element={<DocsPage />} />
        <Route path="/*" element={<BoardApp />} />
      </Routes>
    </BrowserRouter>
  );
}

export default App;
