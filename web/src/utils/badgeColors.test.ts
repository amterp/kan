import { describe, it, expect } from 'vitest';
import { stringToColor } from './badgeColors';

describe('stringToColor', () => {
  it('returns the same color for the same input', () => {
    const color = stringToColor('Bug');
    expect(stringToColor('Bug')).toBe(color);
    expect(stringToColor('Bug')).toBe(color);
  });

  it('is case-insensitive', () => {
    expect(stringToColor('Bug')).toBe(stringToColor('bug'));
    expect(stringToColor('BUG')).toBe(stringToColor('bug'));
    expect(stringToColor('Feature')).toBe(stringToColor('FEATURE'));
  });

  it('returns a valid hex color', () => {
    const samples = ['Bug', 'Feature', 'Urgent', '', 'hello world', '123'];
    for (const s of samples) {
      expect(stringToColor(s)).toMatch(/^#[0-9a-f]{6}$/i);
    }
  });

  it('handles empty string', () => {
    const color = stringToColor('');
    expect(color).toMatch(/^#[0-9a-f]{6}$/i);
  });

  it('produces reasonable distribution across common values', () => {
    const words = ['Bug', 'Feature', 'Enhancement', 'Critical', 'Low', 'Medium', 'High', 'UI', 'Backend', 'API'];
    const colors = new Set(words.map(stringToColor));
    // 10 words should produce at least 5 distinct colors from a 16-color palette
    expect(colors.size).toBeGreaterThanOrEqual(5);
  });
});
