import { api } from './client';
import type { Card, Comment, CreateCardInput, CreateCardResponse, UpdateCardInput } from './types';

export async function listCards(board: string, column?: string): Promise<Card[]> {
  const params = column ? `?column=${encodeURIComponent(column)}` : '';
  const result = await api.get<{ cards: Card[] }>(`/boards/${encodeURIComponent(board)}/cards${params}`);
  return result.cards;
}

export async function getCard(board: string, id: string): Promise<Card> {
  return api.get<Card>(`/boards/${encodeURIComponent(board)}/cards/${encodeURIComponent(id)}`);
}

export async function createCard(board: string, input: CreateCardInput): Promise<CreateCardResponse> {
  return api.post<CreateCardResponse>(`/boards/${encodeURIComponent(board)}/cards`, input);
}

export async function updateCard(board: string, id: string, input: UpdateCardInput): Promise<Card> {
  return api.put<Card>(`/boards/${encodeURIComponent(board)}/cards/${encodeURIComponent(id)}`, input);
}

export async function moveCard(board: string, id: string, column: string, position?: number): Promise<Card> {
  const body: { column: string; position?: number } = { column };
  if (position !== undefined) {
    body.position = position;
  }
  return api.patch<Card>(`/boards/${encodeURIComponent(board)}/cards/${encodeURIComponent(id)}/move`, body);
}

export async function deleteCard(board: string, id: string): Promise<void> {
  await api.delete<void>(`/boards/${encodeURIComponent(board)}/cards/${encodeURIComponent(id)}`);
}

// Comment operations

export async function createComment(board: string, cardId: string, body: string): Promise<Comment> {
  return api.post<Comment>(
    `/boards/${encodeURIComponent(board)}/cards/${encodeURIComponent(cardId)}/comments`,
    { body }
  );
}

export async function editComment(board: string, cardId: string, commentId: string, body: string): Promise<Comment> {
  return api.patch<Comment>(
    `/boards/${encodeURIComponent(board)}/cards/${encodeURIComponent(cardId)}/comments/${encodeURIComponent(commentId)}`,
    { body }
  );
}

export async function deleteComment(board: string, cardId: string, commentId: string): Promise<void> {
  await api.delete<void>(
    `/boards/${encodeURIComponent(board)}/cards/${encodeURIComponent(cardId)}/comments/${encodeURIComponent(commentId)}`
  );
}
