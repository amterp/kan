import { describe, it, expect } from 'vitest';
import { resolveFavicon } from './favicon';
import type { BoardEntry, FaviconConfig } from '../api/types';

function entry(favicon?: FaviconConfig, overrides: Partial<BoardEntry> = {}): BoardEntry {
  return {
    project_path: '/repo/acme',
    project_name: 'Acme',
    board_name: 'main',
    favicon,
    ...overrides,
  };
}

describe('resolveFavicon', () => {
  it('uses the configured emoji when icon_type is emoji', () => {
    const r = resolveFavicon(entry({ background: '#123456', icon_type: 'emoji', emoji: '🚀', letter: 'A' }));
    expect(r).toEqual({ background: '#123456', glyph: '🚀', isEmoji: true });
  });

  it('uses the configured letter for letter favicons', () => {
    const r = resolveFavicon(entry({ background: '#abcdef', icon_type: 'letter', letter: 'Z', emoji: '' }));
    expect(r).toEqual({ background: '#abcdef', glyph: 'Z', isEmoji: false });
  });

  it('keeps a configured background even when the letter is empty', () => {
    // Regression: a partially-configured favicon (background set, letter blank)
    // must still render in its color, falling back only the glyph to the initial.
    const r = resolveFavicon(entry({ background: '#abcdef', icon_type: 'letter', letter: '', emoji: '' }));
    expect(r.background).toBe('#abcdef');
    expect(r.glyph).toBe('A'); // initial of "Acme"
    expect(r.isEmoji).toBe(false);
  });

  it('falls back to icon_type emoji only when an emoji is present', () => {
    // icon_type says emoji but none set -> treat as letter, keep background.
    const r = resolveFavicon(entry({ background: '#abcdef', icon_type: 'emoji', emoji: '', letter: 'L' }));
    expect(r).toEqual({ background: '#abcdef', glyph: 'L', isEmoji: false });
  });

  it('derives a deterministic color and initial when no favicon is set', () => {
    const r = resolveFavicon(entry(undefined));
    expect(r.isEmoji).toBe(false);
    expect(r.glyph).toBe('A');
    expect(r.background).toMatch(/^#[0-9a-f]{6}$/i);
    // Deterministic for the same project path.
    expect(resolveFavicon(entry(undefined)).background).toBe(r.background);
  });

  it('falls back to the board name initial when project name is empty', () => {
    const r = resolveFavicon(entry(undefined, { project_name: '', board_name: 'roadmap' }));
    expect(r.glyph).toBe('R');
  });
});
