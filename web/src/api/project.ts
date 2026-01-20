import { api } from './client';
import type { ProjectConfig } from './types';

export async function getProject(): Promise<ProjectConfig> {
  return api.get<ProjectConfig>('/project');
}
