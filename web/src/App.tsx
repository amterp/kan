import { useState, useCallback } from 'react';
import { BrowserRouter, Routes, Route } from 'react-router-dom';
import { useBoards, useBoard } from './hooks/useBoards';
import Header from './components/Header';
import Board from './components/Board';
import CardEditModal from './components/CardEditModal';
import DocsPage from './pages/DocsPage';
import type { Card, UpdateCardInput } from './api/types';

function BoardApp() {
  const { boards, loading: boardsLoading, error: boardsError } = useBoards();
  const [selectedBoard, setSelectedBoard] = useState<string | null>(null);
  const { board, cards, loading, error, moveCard, createCard, updateCard, deleteCard, refresh } = useBoard(selectedBoard);
  const [newCardForEdit, setNewCardForEdit] = useState<Card | null>(null);

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
            onMoveCard={moveCard}
            onCreateCard={createCard}
            onUpdateCard={updateCard}
            onDeleteCard={deleteCard}
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
    </div>
  );
}

function App() {
  return (
    <BrowserRouter>
      <Routes>
        <Route path="/docs/*" element={<DocsPage />} />
        <Route path="/*" element={<BoardApp />} />
      </Routes>
    </BrowserRouter>
  );
}

export default App;
