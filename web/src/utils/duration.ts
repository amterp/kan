/**
 * Render a millisecond duration as a coarse, single-unit, at-a-glance string
 * (e.g. "just now", "5m", "3h", "2 days", "4 months"). Mirrors the Go
 * util.FormatDuration so CLI and web read identically.
 */
export function formatDuration(millis: number): string {
  const minute = 60_000;
  const hour = 60 * minute;
  const day = 24 * hour;
  const month = 30 * day;
  const year = 365 * day;

  if (millis < minute) return 'just now';
  if (millis < hour) return `${Math.floor(millis / minute)}m`;
  if (millis < day) return `${Math.floor(millis / hour)}h`;
  if (millis < month) return pluralize(Math.floor(millis / day), 'day');
  if (millis < year) return pluralize(Math.floor(millis / month), 'month');
  return pluralize(Math.floor(millis / year), 'year');
}

function pluralize(n: number, unit: string): string {
  return n === 1 ? `1 ${unit}` : `${n} ${unit}s`;
}
