import { useCallback } from 'react';
import { useParams, useSearchParams, useNavigate } from 'react-router-dom';

export interface UrlState {
  boardName: string | undefined;
  cardId: string | undefined;
  setBoard: (name: string, options?: { replace?: boolean }) => void;
  openCard: (id: string) => void;
  closeCard: (options?: { replace?: boolean }) => void;
}

export function useUrlState(): UrlState {
  const { boardName } = useParams<{ boardName: string }>();
  const [searchParams] = useSearchParams();
  const navigate = useNavigate();

  const cardId = searchParams.get('card') || undefined;

  const setBoard = useCallback(
    (name: string, options?: { replace?: boolean }) => {
      navigate(`/board/${name}`, { replace: options?.replace });
    },
    [navigate]
  );

  const openCard = useCallback(
    (id: string) => {
      if (boardName) {
        navigate(`/board/${boardName}?card=${id}`);
      }
    },
    [navigate, boardName]
  );

  const closeCard = useCallback(
    (options?: { replace?: boolean }) => {
      if (boardName) {
        navigate(`/board/${boardName}`, { replace: options?.replace });
      }
    },
    [navigate, boardName]
  );

  return { boardName, cardId, setBoard, openCard, closeCard };
}
