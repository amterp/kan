// 20-color palette optimized for perceptual distinctness with white text.
// Bright hues (600/700) for core colors, darker shades (800) for contrast variety,
// plus slate as a neutral. Near-duplicate hues (violet~purple, sky~blue, amber~orange)
// were removed in favor of darker variants that read as genuinely different colors.
const BADGE_COLORS = [
  '#2563eb', // blue-600
  '#dc2626', // red-600
  '#047857', // emerald-700
  '#c2410c', // orange-700
  '#9333ea', // purple-600
  '#db2777', // pink-600
  '#0e7490', // cyan-700
  '#a16207', // yellow-700
  '#4f46e5', // indigo-600
  '#4d7c0f', // lime-700
  '#c026d3', // fuchsia-600
  '#0f766e', // teal-700
  '#e11d48', // rose-600
  '#991b1b', // maroon (red-800)
  '#92400e', // brown (amber-800)
  '#166534', // forest (green-800)
  '#1e40af', // navy (blue-800)
  '#6b21a8', // deep purple (purple-800)
  '#475569', // slate (slate-600)
  '#9f1239', // wine (rose-800)
];

/** djb2 hash - simple, well-distributed string hash. */
function hashString(str: string): number {
  let hash = 5381;
  for (let i = 0; i < str.length; i++) {
    hash = ((hash << 5) + hash + str.charCodeAt(i)) | 0; // hash * 33 + char
  }
  return Math.abs(hash);
}

/** Maps a string value to a deterministic badge color. Case-insensitive. */
export function stringToColor(value: string): string {
  if (!value) return BADGE_COLORS[0];
  return BADGE_COLORS[hashString(value.toLowerCase()) % BADGE_COLORS.length];
}
