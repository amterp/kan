import { useState, useCallback, useEffect, useMemo } from 'react';
import { BrowserRouter, Routes, Route, Navigate, useSearchParams } from 'react-router-dom';
import { useBoards, useBoard } from './hooks/useBoards';
import { useOmnibar } from './hooks/useOmnibar';
import { useBoardSwitcher } from './hooks/useBoardSwitcher';
import { useSlashCommandAutocomplete } from './hooks/useSlashCommandAutocomplete';
import { useThemeSwitcher } from './hooks/useThemeSwitcher';
import { COMPACT_COMMAND, SLIM_COMMAND } from './hooks/omnibarConstants';
import type { SlashCommand } from './hooks/omnibarConstants';
import { useProject, usePageTitle, useFavicon } from './hooks/useProject';
import { useUrlState } from './hooks/useUrlState';
import { cardMatchesQuery } from './utils/fuzzyMatch';
import Header from './components/Header';
import Board from './components/Board';
import CardEditModal from './components/CardEditModal';
import LandingPage from './pages/LandingPage';
import Omnibar, { type NavigationDirection } from './components/Omnibar';
import DocsPage from './pages/DocsPage';
import { switchProject } from './api/projects';
import type { UpdateCardInput } from './api/types';
import { restoreCard as apiRestoreCard } from './api/cards';
import { useUndo } from './hooks/useUndo';
import { useCompactMode } from './contexts/CompactModeContext';
import { useSlimMode } from './contexts/SlimModeContext';
import { useToast } from './contexts/ToastContext';


function BoardApp() {
  const [refreshKey, setRefreshKey] = useState(0);
  const { boards, loading: boardsLoading, error: boardsError } = useBoards(refreshKey);
  const { boardName, cardId, setBoard, openCard, closeCard } = useUrlState();
  const {
    board,
    cards,
    loading,
    error,
    moveCard,
    createCard,
    updateCard,
    deleteCard,
    addCardToState,
    createColumn,
    deleteColumn,
    updateColumn,
    reorderColumns,
    refresh,
    fileSyncConnected,
    fileSyncReconnecting,
    fileSyncFailed,
  } = useBoard(boardName, refreshKey);
  const [newlyCreatedCardId, setNewlyCreatedCardId] = useState<string | null>(null);
  const omnibar = useOmnibar();
  const { showToast } = useToast();

  const { pushUndo } = useUndo({
    boardName,
    cards,
    board,
    moveCard,
    updateCard,
    deleteCard,
    restoreCard: apiRestoreCard,
    addCardToState,
    showToast,
  });
  const { toggleCompact, setProjectPath } = useCompactMode();
  const { isSlim, toggleSlim } = useSlimMode();
  const [searchParams, setSearchParams] = useSearchParams();
  const { project } = useProject(refreshKey);

  // Keep URL ?slim param in sync with slim mode state.
  // Initial state is read from URL in SlimModeContext (URL wins over localStorage).
  useEffect(() => {
    const hasSlimParam = searchParams.get('slim') === 'true';
    if (isSlim && !hasSlimParam) {
      setSearchParams((prev) => {
        const next = new URLSearchParams(prev);
        next.set('slim', 'true');
        return next;
      }, { replace: true });
    } else if (!isSlim && hasSlimParam) {
      setSearchParams((prev) => {
        const next = new URLSearchParams(prev);
        next.delete('slim');
        return next;
      }, { replace: true });
    }
  }, [isSlim, searchParams, setSearchParams]);

  // Keep compact mode context aware of the current project path so
  // per-board compact overrides are scoped to each project.
  useEffect(() => {
    if (project?.project_path) {
      setProjectPath(project.project_path);
    }
  }, [project?.project_path, setProjectPath]);

  // Board switcher
  const boardSwitcher = useBoardSwitcher(omnibar.query, omnibar.isOpen && omnibar.mode === 'boards');

  // Slash command autocomplete
  const slashAutocomplete = useSlashCommandAutocomplete(omnibar.query);

  // Theme switcher
  const themeSwitcher = useThemeSwitcher(omnibar.query, omnibar.isOpen && omnibar.mode === 'themes');

  // Execute slash commands that run immediately (insertsIntoInput: false).
  // Prefix commands like /board are handled separately by inserting into the input.
  const executeSlashCommand = useCallback((cmd: SlashCommand) => {
    if (cmd.insertsIntoInput) {
      omnibar.setQuery(cmd.command + ' ');
    } else if (cmd.command === COMPACT_COMMAND) {
      toggleCompact();
      omnibar.close();
    } else if (cmd.command === SLIM_COMMAND) {
      toggleSlim();
      omnibar.close();
    }
  }, [omnibar, toggleCompact, toggleSlim]);

  // Set page title and favicon
  usePageTitle(project?.name, boardName);
  useFavicon();

  // Compute filtered cards for navigation
  const filteredCards = useMemo(() => {
    if (!board || !omnibar.query.trim() || omnibar.mode === 'boards' || omnibar.mode === 'themes' || slashAutocomplete.isActive) return cards;
    return cards.filter((card) => cardMatchesQuery(card, omnibar.query.trim(), board));
  }, [cards, omnibar.query, omnibar.mode, board, slashAutocomplete.isActive]);

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
    if (omnibar.isOpen && omnibar.mode === 'cards' && filteredCards.length > 0) {
      const currentHighlight = omnibar.highlightedCardId;
      const highlightStillValid = currentHighlight && filteredCards.some((c) => c.id === currentHighlight);
      if (!highlightStillValid) {
        omnibar.setHighlightedCardId(filteredCards[0].id);
      }
    } else if (omnibar.isOpen && omnibar.mode === 'cards' && filteredCards.length === 0) {
      omnibar.setHighlightedCardId(null);
    }
  }, [omnibar, filteredCards]);

  // Handle arrow key navigation
  const handleNavigate = useCallback(
    (direction: NavigationDirection) => {
      // Slash command autocomplete takes priority
      if (slashAutocomplete.isActive) {
        if (direction === 'up') {
          slashAutocomplete.moveHighlight(-1);
        } else if (direction === 'down') {
          slashAutocomplete.moveHighlight(1);
        }
        return;
      }

      if (omnibar.mode === 'themes') {
        if (direction === 'up') {
          themeSwitcher.moveHighlight(-1);
        } else if (direction === 'down') {
          themeSwitcher.moveHighlight(1);
        }
        return;
      }

      if (omnibar.mode === 'boards') {
        // In boards mode, up/down moves the board highlight
        if (direction === 'up') {
          boardSwitcher.moveHighlight(-1);
        } else if (direction === 'down') {
          boardSwitcher.moveHighlight(1);
        }
        return;
      }

      if (filteredCardsByColumn.length === 0) return;

      const currentId = omnibar.highlightedCardId;
      if (!currentId) {
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
          } else if (isSlim && currentColIdx > 0) {
            const prevColumn = filteredCardsByColumn[currentColIdx - 1];
            nextCardId = prevColumn.cards[prevColumn.cards.length - 1].id;
          }
          break;
        }
        case 'down': {
          if (currentCardIdx < currentColumn.cards.length - 1) {
            nextCardId = currentColumn.cards[currentCardIdx + 1].id;
          } else if (isSlim && currentColIdx < filteredCardsByColumn.length - 1) {
            nextCardId = filteredCardsByColumn[currentColIdx + 1].cards[0].id;
          }
          break;
        }
        case 'left': {
          // In slim mode, columns are vertical - left/right column jumping disabled
          if (isSlim) break;
          if (currentColIdx > 0) {
            const prevColumn = filteredCardsByColumn[currentColIdx - 1];
            const targetIdx = Math.min(currentCardIdx, prevColumn.cards.length - 1);
            nextCardId = prevColumn.cards[targetIdx].id;
          }
          break;
        }
        case 'right': {
          if (isSlim) break;
          if (currentColIdx < filteredCardsByColumn.length - 1) {
            const nextColumn = filteredCardsByColumn[currentColIdx + 1];
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
    [filteredCardsByColumn, omnibar, boardSwitcher, themeSwitcher, slashAutocomplete, isSlim]
  );

  // Handle Enter to select
  const handleSelect = useCallback(async () => {
    // Slash command autocomplete selection
    if (slashAutocomplete.isActive && slashAutocomplete.filteredCommands.length > 0) {
      const selected = slashAutocomplete.filteredCommands[slashAutocomplete.highlightedIndex];
      if (selected) {
        executeSlashCommand(selected);
      }
      return;
    }

    if (omnibar.mode === 'themes') {
      if (themeSwitcher.selectHighlighted()) {
        omnibar.close();
      }
      return;
    }

    if (omnibar.mode === 'boards') {
      const result = await boardSwitcher.selectHighlighted();
      if (result) {
        setProjectPath(result.projectPath);
        omnibar.close();
        setBoard(result.boardName);
        setRefreshKey((k) => k + 1);
        // Force favicon refresh by changing the URL (triggers a new fetch)
        const link = document.querySelector<HTMLLinkElement>('link[rel="icon"]');
        if (link) {
          link.href = '/favicon.svg?' + Date.now();
        }
      }
      return;
    }

    // Cards mode - highlighted card takes priority over commands
    if (omnibar.highlightedCardId) {
      const card = cards.find((c) => c.id === omnibar.highlightedCardId);
      if (card) {
        omnibar.close();
        openCard(card.id);
      }
      return;
    }

    // Slash commands (only when no card is highlighted)
    const trimmedQuery = omnibar.query.trim().toLowerCase();
    if (trimmedQuery === COMPACT_COMMAND) {
      toggleCompact();
      omnibar.close();
    } else if (trimmedQuery === SLIM_COMMAND) {
      toggleSlim();
      omnibar.close();
    }
  }, [omnibar, boardSwitcher, cards, setBoard, openCard, toggleCompact, toggleSlim, executeSlashCommand, slashAutocomplete, setProjectPath]);

  // Handle clicking a board entry in the list
  const handleBoardSelect = useCallback(async (index: number) => {
    boardSwitcher.setHighlightedIndex(index);
    const entry = boardSwitcher.filteredBoards[index];
    if (!entry) return;

    try {
      await switchProject(entry.project_path);
      setProjectPath(entry.project_path);
      omnibar.close();
      setBoard(entry.board_name);
      setRefreshKey((k) => k + 1);
      // Bust favicon cache
      const link = document.querySelector<HTMLLinkElement>('link[rel="icon"]');
      if (link) {
        link.href = '/favicon.svg?t=' + Date.now();
      }
    } catch {
      // Error is handled by the switcher hook
    }
  }, [boardSwitcher, omnibar, setBoard, setProjectPath]);

  // Handle clicking a theme option in the list
  const handleThemeSelect = useCallback((index: number) => {
    if (themeSwitcher.selectByIndex(index)) {
      omnibar.close();
    }
  }, [themeSwitcher, omnibar]);

  // Handle clicking a slash command suggestion
  const handleSlashCommandSelect = useCallback((index: number) => {
    const selected = slashAutocomplete.filteredCommands[index];
    if (!selected) return;
    executeSlashCommand(selected);
  }, [slashAutocomplete, executeSlashCommand]);

  // Cmd+K keyboard shortcut for omnibar (cards mode)
  // Cmd+P keyboard shortcut for board switcher
  // Cmd+C keyboard shortcut for compact mode toggle
  useEffect(() => {
    const handleKeyDown = (e: KeyboardEvent) => {
      if ((e.metaKey || e.ctrlKey) && e.key === 'k') {
        e.preventDefault();
        if (cardId) return;
        if (omnibar.isOpen) {
          omnibar.close();
        } else {
          omnibar.open('cards');
        }
      }
      if ((e.metaKey || e.ctrlKey) && e.key === 'p') {
        e.preventDefault();
        if (cardId) return;
        if (omnibar.isOpen && omnibar.mode === 'boards') {
          omnibar.close();
        } else {
          omnibar.open('boards');
        }
      }
      if ((e.metaKey || e.ctrlKey) && e.key === 'c') {
        // Only toggle compact when nothing else would use Cmd+C
        const selection = window.getSelection();
        if (selection && selection.toString().length > 0) return;
        const tag = document.activeElement?.tagName;
        if (tag === 'INPUT' || tag === 'TEXTAREA' || tag === 'SELECT') return;
        if (omnibar.isOpen || cardId) return;
        e.preventDefault();
        toggleCompact();
      }
      if ((e.metaKey || e.ctrlKey) && e.key === 'j') {
        const selection = window.getSelection();
        if (selection && selection.toString().length > 0) return;
        const tag = document.activeElement?.tagName;
        if (tag === 'INPUT' || tag === 'TEXTAREA' || tag === 'SELECT') return;
        if (omnibar.isOpen || cardId) return;
        e.preventDefault();
        toggleSlim();
      }
    };

    document.addEventListener('keydown', handleKeyDown);
    return () => document.removeEventListener('keydown', handleKeyDown);
  }, [omnibar, cardId, toggleCompact, toggleSlim]);

  const handleNewCard = useCallback(async () => {
    if (!board) return;
    const response = await createCard({ title: 'New Card', column: board.default_column });
    if (response?.card) {
      setNewlyCreatedCardId(response.card.id);
      openCard(response.card.id);
    }
  }, [board, createCard, openCard]);

  const handleOpenCard = useCallback((id: string, focusDescription?: boolean) => {
    if (focusDescription) {
      setNewlyCreatedCardId(id);
    }
    openCard(id);
  }, [openCard]);

  // Auto-select first board if only one exists (replace so no extra history entry)
  useEffect(() => {
    if (!boardName && !boardsLoading && boards.length === 1) {
      setBoard(boards[0], { replace: true });
    }
  }, [boardName, boardsLoading, boards, setBoard]);

  // Find the card for the modal (from URL ?card= param)
  const modalCard = useMemo(
    () => (cardId ? cards.find((c) => c.id === cardId) : undefined),
    [cardId, cards]
  );

  // If cardId is set but card not found (deleted/invalid), silently clear it.
  // Only act once the board has loaded so we don't race the initial fetch.
  useEffect(() => {
    if (cardId && !loading && board && !modalCard) {
      closeCard({ replace: true });
    }
  }, [cardId, loading, board, modalCard, closeCard]);

  // onSave/onDelete don't navigate - CardEditModal calls onClose() after these
  // resolve, which handles the URL update via closeCard.
  const handleSaveModalCard = useCallback(async (updates: UpdateCardInput) => {
    if (modalCard) {
      // Compute per-field diffs for undo (capture before the async call)
      const fieldChanges: Record<string, { from: unknown; to: unknown }> = {};
      if (updates.title !== undefined && updates.title !== modalCard.title) {
        fieldChanges.title = { from: modalCard.title, to: updates.title };
      }
      if (updates.description !== undefined && (updates.description || '') !== (modalCard.description || '')) {
        fieldChanges.description = { from: modalCard.description || '', to: updates.description };
      }
      if (updates.column !== undefined && updates.column !== modalCard.column) {
        fieldChanges.column = { from: modalCard.column, to: updates.column };
      }
      if (updates.custom_fields) {
        for (const [key, apiValue] of Object.entries(updates.custom_fields)) {
          const current = modalCard[key];
          // Normalize API-format values back to card format for undo
          // comparison. The modal converts booleans to strings ("true")
          // and sets to comma-separated strings for the API, but cards
          // store native types. Without this, staleness checks would
          // always see a type mismatch and skip the undo.
          const fieldType = board?.custom_fields?.[key]?.type;
          let cardValue: unknown = apiValue;
          if (fieldType === 'boolean' && typeof apiValue === 'string') {
            cardValue = apiValue === 'true';
          } else if ((fieldType === 'enum-set' || fieldType === 'free-set') && typeof apiValue === 'string') {
            cardValue = apiValue === '' ? [] : apiValue.split(',');
          }
          if (JSON.stringify(current) !== JSON.stringify(cardValue)) {
            fieldChanges[key] = { from: current, to: cardValue };
          }
        }
      }

      await updateCard(modalCard.id, updates);
      setNewlyCreatedCardId(null);

      // Push undo only after success
      if (Object.keys(fieldChanges).length > 0) {
        pushUndo({ type: 'edit', cardId: modalCard.id, fieldChanges });
      }
    }
  }, [modalCard, board, updateCard, pushUndo]);

  const handleDeleteModalCard = useCallback(async () => {
    if (modalCard) {
      // Capture undo data before deletion (need position info while card exists)
      const columnCards = cards.filter((c) => c.column === modalCard.column);
      const position = columnCards.findIndex((c) => c.id === modalCard.id);
      const undoAction = {
        type: 'delete' as const,
        card: { ...modalCard },
        column: modalCard.column,
        position: position >= 0 ? position : 0,
      };

      setNewlyCreatedCardId(null);
      await deleteCard(modalCard.id);

      // Push undo only after successful delete
      pushUndo(undoAction);
      showToast('info', 'Card deleted \u2013 Cmd+Z to undo');
    }
  }, [modalCard, cards, deleteCard, pushUndo, showToast]);

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
    <div className="h-screen flex flex-col bg-gray-100 dark:bg-gray-900 board-bg">
      <Header
        boards={boards}
        selectedBoard={boardName}
        onSelectBoard={setBoard}
        onRefresh={refresh}
        onNewCard={board ? handleNewCard : undefined}
        syncStatus={{
          connected: fileSyncConnected,
          reconnecting: fileSyncReconnecting,
          failed: fileSyncFailed,
        }}
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
            filterQuery={omnibar.mode === 'cards' && !slashAutocomplete.isActive ? omnibar.query : ''}
            highlightedCardId={omnibar.isOpen && omnibar.mode === 'cards' ? omnibar.highlightedCardId : null}
            onMoveCard={moveCard}
            onCreateCard={createCard}
            onUpdateCard={updateCard}
            onDeleteCard={deleteCard}
            onPushUndo={pushUndo}
            onCreateColumn={createColumn}
            onDeleteColumn={deleteColumn}
            onUpdateColumn={updateColumn}
            onReorderColumns={reorderColumns}
            onOpenCard={handleOpenCard}
            isOmnibarOpen={omnibar.isOpen}
            isCardModalOpen={!!cardId}
          />
        ) : (
          <div className="h-full flex items-center justify-center">
            <p className="text-gray-500 dark:text-gray-400">Select a board to get started</p>
          </div>
        )}
      </main>
      {modalCard && board && (
        <CardEditModal
          card={modalCard}
          board={board}
          onSave={handleSaveModalCard}
          onDelete={handleDeleteModalCard}
          onClose={closeCard}
          focusDescription={modalCard?.id === newlyCreatedCardId}
        />
      )}
      {omnibar.isOpen && (
        <Omnibar
          mode={omnibar.mode}
          query={omnibar.query}
          matchCount={filteredCards.length}
          totalCount={cards.length}
          hasHighlight={omnibar.highlightedCardId !== null}
          isModalOpen={!!cardId}
          boardEntries={boardSwitcher.filteredBoards}
          boardHighlightedIndex={boardSwitcher.highlightedIndex}
          boardCurrentProjectPath={boardSwitcher.currentProjectPath}
          boardCurrentBoardName={boardName}
          boardSkipped={boardSwitcher.skipped}
          boardLoading={boardSwitcher.loading}
          boardError={boardSwitcher.fetchError || boardSwitcher.switchError}
          boardDisplayLabel={boardSwitcher.displayLabel}
          themeOptions={themeSwitcher.filteredOptions}
          themeHighlightedIndex={themeSwitcher.highlightedIndex}
          themeCurrentTheme={themeSwitcher.currentTheme}
          slashCommands={slashAutocomplete.filteredCommands}
          slashHighlightedIndex={slashAutocomplete.highlightedIndex}
          slashAutocompleteActive={slashAutocomplete.isActive}
          onQueryChange={omnibar.setQuery}
          onNavigate={handleNavigate}
          onSelect={handleSelect}
          onClose={omnibar.close}
          onBoardSelect={handleBoardSelect}
          onThemeSelect={handleThemeSelect}
          onSlashCommandSelect={handleSlashCommandSelect}
        />
      )}
    </div>
  );
}

function App() {
  // Remove trailing slash from base URL for BrowserRouter basename
  const basename = import.meta.env.BASE_URL.replace(/\/$/, '') || undefined;

  // Docs-only mode: when deployed to a subpath (e.g., GitHub Pages at /kan/),
  // there's no backend, so redirect all non-docs routes to /docs
  const isDocsOnly = import.meta.env.BASE_URL !== '/';

  return (
    <BrowserRouter basename={basename}>
      <Routes>
        <Route path="/docs/*" element={<DocsPage />} />
        {isDocsOnly ? (
          <>
            <Route path="/" element={<LandingPage />} />
            <Route path="/*" element={<Navigate to="/" replace />} />
          </>
        ) : (
          <>
            <Route path="/" element={<BoardApp />} />
            <Route path="/board/:boardName" element={<BoardApp />} />
            <Route path="*" element={<Navigate to="/" replace />} />
          </>
        )}
      </Routes>
    </BrowserRouter>
  );
}

export default App;
