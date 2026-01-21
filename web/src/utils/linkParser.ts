import type { LinkRule } from '../api/types';

/**
 * Represents a segment of parsed text - either plain text or a link.
 */
export interface TextSegment {
  type: 'text' | 'link';
  content: string;  // Display text
  url?: string;     // For links only
}

/**
 * Represents a match found in the text.
 */
interface Match {
  start: number;
  end: number;
  content: string;
  url: string;
}

/**
 * A compiled link rule with pre-compiled regex for performance.
 */
interface CompiledLinkRule {
  name: string;
  regex: RegExp;
  urlTemplate: string;
}

// Cache compiled rules keyed by the LinkRule array reference.
// WeakMap allows garbage collection when the rules array is no longer referenced.
const compiledCache = new WeakMap<LinkRule[], CompiledLinkRule[]>();

// URL regex pattern - matches http(s) URLs
const URL_PATTERN = /https?:\/\/[^\s<>)]+/g;

/**
 * Compile link rules into regex objects, caching the result.
 * Invalid patterns are logged and skipped.
 */
export function compileRules(rules: LinkRule[]): CompiledLinkRule[] {
  const cached = compiledCache.get(rules);
  if (cached) return cached;

  const compiled: CompiledLinkRule[] = [];
  for (const rule of rules) {
    try {
      compiled.push({
        name: rule.name,
        regex: new RegExp(rule.pattern, 'g'),
        urlTemplate: rule.url,
      });
    } catch (e) {
      console.warn(`Invalid regex pattern in link rule "${rule.name}": ${rule.pattern}`, e);
    }
  }

  compiledCache.set(rules, compiled);
  return compiled;
}

/**
 * Builds a URL from a template and captured groups.
 *
 * Placeholder syntax:
 * - {0} = full match (URL encoded)
 * - {1}, {2}, etc. = capture groups (URL encoded)
 * - {0!raw}, {1!raw}, etc. = unencoded substitution for path segments
 */
function buildUrl(template: string, fullMatch: string, groups: string[]): string {
  // Handle {0!raw} and {0}
  let url = template
    .replace(/\{0!raw\}/g, fullMatch)
    .replace(/\{0\}/g, encodeURIComponent(fullMatch));

  // Handle {N!raw} and {N} for capture groups
  for (let i = 0; i < groups.length; i++) {
    const value = groups[i] || '';
    url = url
      .replace(new RegExp(`\\{${i + 1}!raw\\}`, 'g'), value)
      .replace(new RegExp(`\\{${i + 1}\\}`, 'g'), encodeURIComponent(value));
  }

  return url;
}

/**
 * Find ranges of existing markdown links to exclude from processing.
 */
function findMarkdownLinkRanges(text: string): Array<{ start: number; end: number }> {
  const ranges: Array<{ start: number; end: number }> = [];
  const markdownLinkRegex = /\[([^\]]+)\]\([^)]+\)/g;

  let match;
  while ((match = markdownLinkRegex.exec(text)) !== null) {
    ranges.push({
      start: match.index,
      end: match.index + match[0].length,
    });
  }

  return ranges;
}

/**
 * Check if a range overlaps with any excluded ranges.
 */
function isInExcludedRange(
  start: number,
  end: number,
  excludedRanges: Array<{ start: number; end: number }>
): boolean {
  return excludedRanges.some(
    range => (start >= range.start && start < range.end) ||
             (end > range.start && end <= range.end) ||
             (start <= range.start && end >= range.end)
  );
}

/**
 * Parse text and find all matches from compiled link rules and raw URLs.
 * Returns matches sorted by position, with overlapping matches resolved (first wins).
 */
function findAllMatches(
  text: string,
  compiledRules: CompiledLinkRule[],
  excludedRanges: Array<{ start: number; end: number }> = []
): Match[] {
  const matches: Match[] = [];

  // Process custom link rules first (they take priority)
  for (const rule of compiledRules) {
    // Reset regex state for each search
    rule.regex.lastIndex = 0;
    let match;

    while ((match = rule.regex.exec(text)) !== null) {
      const start = match.index;
      const end = match.index + match[0].length;

      // Skip if inside excluded range
      if (isInExcludedRange(start, end, excludedRanges)) continue;

      const groups = match.slice(1);
      const url = buildUrl(rule.urlTemplate, match[0], groups);

      matches.push({ start, end, content: match[0], url });
    }
  }

  // Find raw URLs as fallback
  URL_PATTERN.lastIndex = 0;
  let urlMatch;
  while ((urlMatch = URL_PATTERN.exec(text)) !== null) {
    const start = urlMatch.index;
    const end = urlMatch.index + urlMatch[0].length;

    // Skip if inside excluded range
    if (isInExcludedRange(start, end, excludedRanges)) continue;

    matches.push({
      start,
      end,
      content: urlMatch[0],
      url: urlMatch[0],
    });
  }

  // Sort matches by start position
  matches.sort((a, b) => a.start - b.start);

  // Remove overlapping matches (first match wins)
  const nonOverlapping: Match[] = [];
  let lastEnd = 0;

  for (const match of matches) {
    if (match.start >= lastEnd) {
      nonOverlapping.push(match);
      lastEnd = match.end;
    }
  }

  return nonOverlapping;
}

/**
 * Parse text with link rules and raw URL detection.
 *
 * @param text - The text to parse
 * @param linkRules - Array of link rules to apply (optional)
 * @returns Array of text segments
 */
export function parseTextWithLinks(
  text: string,
  linkRules: LinkRule[] = []
): TextSegment[] {
  if (!text) {
    return [];
  }

  const compiledRules = linkRules.length > 0 ? compileRules(linkRules) : [];
  const matches = findAllMatches(text, compiledRules);

  if (matches.length === 0) {
    return [{ type: 'text', content: text }];
  }

  const segments: TextSegment[] = [];
  let lastIndex = 0;

  for (const match of matches) {
    // Add text before the match
    if (match.start > lastIndex) {
      segments.push({
        type: 'text',
        content: text.slice(lastIndex, match.start),
      });
    }

    // Add the link
    segments.push({
      type: 'link',
      content: match.content,
      url: match.url,
    });

    lastIndex = match.end;
  }

  // Add remaining text after last match
  if (lastIndex < text.length) {
    segments.push({
      type: 'text',
      content: text.slice(lastIndex),
    });
  }

  return segments;
}

/**
 * Apply link rules to markdown text by converting matches to markdown links.
 * This pre-processes text before passing to a markdown renderer.
 *
 * Preserves existing markdown links by skipping text inside [...](...).
 *
 * @param text - The markdown text to process
 * @param linkRules - Array of link rules to apply (optional)
 * @returns Processed markdown text with auto-links converted to markdown syntax
 */
export function applyLinkRulesToMarkdown(
  text: string,
  linkRules: LinkRule[] = []
): string {
  if (!text || linkRules.length === 0) {
    return text;
  }

  // Find existing markdown links to exclude from processing
  const excludedRanges = findMarkdownLinkRanges(text);

  // Compile rules and find all matches
  const compiledRules = compileRules(linkRules);
  const matches = findAllMatches(text, compiledRules, excludedRanges);

  if (matches.length === 0) {
    return text;
  }

  // Build result string from segments (no offset tracking needed)
  const parts: string[] = [];
  let lastEnd = 0;

  for (const match of matches) {
    // Add text before the match
    parts.push(text.slice(lastEnd, match.start));
    // Add the markdown link
    parts.push(`[${match.content}](${match.url})`);
    lastEnd = match.end;
  }

  // Add remaining text after last match
  parts.push(text.slice(lastEnd));

  return parts.join('');
}
