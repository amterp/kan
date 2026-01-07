import { api } from './client';
import type { BoardConfig, Column, CreateColumnInput, UpdateColumnInput } from './types';

export async function listBoards(): Promise<string[]> {
  const result = await api.get<{ boards: string[] }>('/boards');
  return result.boards;
}

export async function getBoard(name: string): Promise<BoardConfig> {
  return api.get<BoardConfig>(`/boards/${encodeURIComponent(name)}`);
}

// Column API functions

export async function createColumn(
  board: string,
  input: CreateColumnInput
): Promise<Column> {
  return api.post<Column>(`/boards/${encodeURIComponent(board)}/columns`, input);
}

export async function deleteColumn(
  board: string,
  columnName: string
): Promise<{ deleted_cards: number }> {
  return api.delete<{ deleted_cards: number }>(
    `/boards/${encodeURIComponent(board)}/columns/${encodeURIComponent(columnName)}`
  );
}

export async function updateColumn(
  board: string,
  columnName: string,
  input: UpdateColumnInput
): Promise<Column> {
  return api.patch<Column>(
    `/boards/${encodeURIComponent(board)}/columns/${encodeURIComponent(columnName)}`,
    input
  );
}

export async function reorderColumns(
  board: string,
  columns: string[]
): Promise<BoardConfig> {
  return api.put<BoardConfig>(
    `/boards/${encodeURIComponent(board)}/columns/order`,
    { columns }
  );
}
