import { stringToColor } from './badgeColors';
import type { BoardEntry, FaviconConfig } from '../api/types';

export interface ResolvedFavicon {
  background: string;
  glyph: string;
  isEmoji: boolean;
}

// Resolve the visual identity for a board tile. Prefers the project's
// configured favicon (the same identity shown in the browser tab). When a
// background is set we honor it even if the glyph is missing - falling back
// only the glyph to the project's initial - so a partially-configured favicon
// still renders in the right color. With no usable favicon at all (e.g. a
// project whose config couldn't be read), derive a deterministic color and
// initial from the project so tiles stay distinct.
export function resolveFavicon(entry: BoardEntry): ResolvedFavicon {
  const fav: FaviconConfig | undefined = entry.favicon;
  const name = entry.project_name || entry.board_name || 'K';
  const initial = (name.charAt(0) || 'K').toUpperCase();

  if (fav?.background) {
    if (fav.icon_type === 'emoji' && fav.emoji) {
      return { background: fav.background, glyph: fav.emoji, isEmoji: true };
    }
    return { background: fav.background, glyph: fav.letter || initial, isEmoji: false };
  }

  return {
    background: stringToColor(entry.project_path || name),
    glyph: initial,
    isEmoji: false,
  };
}
