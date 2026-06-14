import { useCallback } from 'react';
import { useParams, useSearchParams, useNavigate } from 'react-router-dom';

export interface UrlState {
  boardName: string | null;
  cardId: string | undefined;
  setBoard: (name: string, options?: { replace?: boolean }) => void;
  openCard: (id: string) => void;
  closeCard: (options?: { replace?: boolean }) => void;
}

export function useUrlState(): UrlState {
  const { boardName: rawBoardName } = useParams<{ boardName: string }>();
  const [searchParams] = useSearchParams();
  const navigate = useNavigate();

  const boardName = rawBoardName ?? null;
  const cardId = searchParams.get('card') || undefined;

  const setBoard = useCallback(
    (name: string, options?: { replace?: boolean }) => {
      navigate(`/board/${encodeURIComponent(name)}`, { replace: options?.replace });
    },
    [navigate]
  );

  // Open/close the card modal by toggling only the `card` query param, leaving
  // any other params (e.g. sort, sortDir, slim) intact — rebuilding the URL from
  // scratch here would silently reset the active board sort.
  const openCard = useCallback(
    (id: string) => {
      if (!boardName) return;
      const params = new URLSearchParams(searchParams);
      params.set('card', id);
      navigate(`/board/${encodeURIComponent(boardName)}?${params.toString()}`);
    },
    [navigate, boardName, searchParams]
  );

  const closeCard = useCallback(
    (options?: { replace?: boolean }) => {
      if (!boardName) return;
      const params = new URLSearchParams(searchParams);
      params.delete('card');
      const qs = params.toString();
      navigate(`/board/${encodeURIComponent(boardName)}${qs ? `?${qs}` : ''}`, { replace: options?.replace });
    },
    [navigate, boardName, searchParams]
  );

  return { boardName, cardId, setBoard, openCard, closeCard };
}
