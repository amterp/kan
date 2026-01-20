import { useState, useEffect, useCallback } from 'react';
import { getProject } from '../api/project';
import type { ProjectConfig } from '../api/types';

export function useProject() {
  const [project, setProject] = useState<ProjectConfig | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  const refresh = useCallback(async () => {
    try {
      const data = await getProject();
      setProject(data);
      setError(null);
    } catch (e) {
      setError(e instanceof Error ? e.message : 'Failed to load project');
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    refresh();
  }, [refresh]);

  return { project, loading, error, refresh };
}

// Hook to update the page title based on project and board
export function usePageTitle(projectName: string | undefined, boardName: string | null) {
  useEffect(() => {
    const parts: string[] = ['Kan'];

    if (projectName) {
      parts.push(projectName);
    }

    if (boardName) {
      parts.push(boardName);
    }

    document.title = parts.join(' - ');

    // Reset to "Kan" on unmount
    return () => {
      document.title = 'Kan';
    };
  }, [projectName, boardName]);
}

// Hook to set the favicon dynamically
export function useFavicon() {
  useEffect(() => {
    // Set up the favicon link element if it doesn't exist
    let link = document.querySelector<HTMLLinkElement>('link[rel="icon"]');
    if (!link) {
      link = document.createElement('link');
      link.rel = 'icon';
      link.type = 'image/svg+xml';
      document.head.appendChild(link);
    }

    // Point to the dynamic favicon endpoint
    link.href = '/favicon.svg';
  }, []);
}
