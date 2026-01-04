import { api } from './client';
import type { Card, CreateCardInput, UpdateCardInput } from './types';

export async function listCards(board: string, column?: string): Promise<Card[]> {
  const params = column ? `?column=${encodeURIComponent(column)}` : '';
  const result = await api.get<{ cards: Card[] }>(`/boards/${encodeURIComponent(board)}/cards${params}`);
  return result.cards;
}

export async function getCard(board: string, id: string): Promise<Card> {
  return api.get<Card>(`/boards/${encodeURIComponent(board)}/cards/${encodeURIComponent(id)}`);
}

export async function createCard(board: string, input: CreateCardInput): Promise<Card> {
  return api.post<Card>(`/boards/${encodeURIComponent(board)}/cards`, input);
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
