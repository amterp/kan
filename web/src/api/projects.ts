import { api } from './client';
import type { AllBoardsResponse, SwitchResponse } from './types';

export async function listAllBoards(): Promise<AllBoardsResponse> {
  return api.get<AllBoardsResponse>('/all-boards');
}

export async function switchProject(projectPath: string): Promise<SwitchResponse> {
  return api.post<SwitchResponse>('/switch', { project_path: projectPath });
}
