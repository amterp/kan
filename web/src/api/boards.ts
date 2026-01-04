import { api } from './client';
import type { BoardConfig } from './types';

export async function listBoards(): Promise<string[]> {
  const result = await api.get<{ boards: string[] }>('/boards');
  return result.boards;
}

export async function getBoard(name: string): Promise<BoardConfig> {
  return api.get<BoardConfig>(`/boards/${encodeURIComponent(name)}`);
}
