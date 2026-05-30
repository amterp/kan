import { describe, expect, it } from 'vitest';
import { formatDuration } from './duration';

const MINUTE = 60_000;
const HOUR = 60 * MINUTE;
const DAY = 24 * HOUR;
const MONTH = 30 * DAY;
const YEAR = 365 * DAY;

describe('formatDuration', () => {
  it('renders sub-minute durations as "just now"', () => {
    expect(formatDuration(0)).toBe('just now');
    expect(formatDuration(MINUTE - 1)).toBe('just now');
  });

  it('renders minutes and hours compactly', () => {
    expect(formatDuration(MINUTE)).toBe('1m');
    expect(formatDuration(59 * MINUTE)).toBe('59m');
    expect(formatDuration(HOUR)).toBe('1h');
    expect(formatDuration(23 * HOUR)).toBe('23h');
  });

  it('pluralizes days, months, and years', () => {
    expect(formatDuration(DAY)).toBe('1 day');
    expect(formatDuration(3 * DAY)).toBe('3 days');
    expect(formatDuration(MONTH)).toBe('1 month');
    expect(formatDuration(4 * MONTH)).toBe('4 months');
    expect(formatDuration(YEAR)).toBe('1 year');
    expect(formatDuration(2 * YEAR)).toBe('2 years');
  });
});
