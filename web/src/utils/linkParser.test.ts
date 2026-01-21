import { describe, it, expect, vi } from 'vitest';
import {
  parseTextWithLinks,
  applyLinkRulesToMarkdown,
  compileRules,
  type TextSegment,
} from './linkParser';
import type { LinkRule } from '../api/types';

describe('linkParser', () => {
  // Common test rules
  const jiraRule: LinkRule = {
    name: 'Jira',
    pattern: '([A-Z]+-\\d+)',
    url: 'https://jira.example.com/browse/{1}',
  };

  const githubIssueRule: LinkRule = {
    name: 'GitHub Issue',
    pattern: '#(\\d+)',
    url: 'https://github.com/org/repo/issues/{1}',
  };

  describe('compileRules', () => {
    it('compiles valid rules', () => {
      const rules = [jiraRule];
      const compiled = compileRules(rules);

      expect(compiled).toHaveLength(1);
      expect(compiled[0].name).toBe('Jira');
      expect(compiled[0].regex).toBeInstanceOf(RegExp);
      expect(compiled[0].urlTemplate).toBe(jiraRule.url);
    });

    it('caches compiled rules (same array reference returns same result)', () => {
      const rules = [jiraRule];
      const compiled1 = compileRules(rules);
      const compiled2 = compileRules(rules);

      expect(compiled1).toBe(compiled2); // Same reference
    });

    it('skips invalid regex patterns without throwing', () => {
      const consoleSpy = vi.spyOn(console, 'warn').mockImplementation(() => {});

      const badRule: LinkRule = {
        name: 'Bad',
        pattern: '[invalid(regex',
        url: 'https://example.com/{1}',
      };

      const compiled = compileRules([badRule, jiraRule]);

      expect(compiled).toHaveLength(1); // Only valid rule
      expect(compiled[0].name).toBe('Jira');
      expect(consoleSpy).toHaveBeenCalled();

      consoleSpy.mockRestore();
    });

    it('handles empty rules array', () => {
      const compiled = compileRules([]);
      expect(compiled).toHaveLength(0);
    });
  });

  describe('parseTextWithLinks', () => {
    describe('empty/null inputs', () => {
      it('returns empty array for empty string', () => {
        expect(parseTextWithLinks('')).toEqual([]);
      });

      it('returns empty array for null/undefined', () => {
        expect(parseTextWithLinks(null as unknown as string)).toEqual([]);
        expect(parseTextWithLinks(undefined as unknown as string)).toEqual([]);
      });

      it('returns text segment for text with no rules', () => {
        const result = parseTextWithLinks('Hello world');
        expect(result).toEqual([{ type: 'text', content: 'Hello world' }]);
      });
    });

    describe('basic pattern matching', () => {
      it('matches Jira-style ticket IDs', () => {
        const result = parseTextWithLinks('Fix ABC-123 now', [jiraRule]);

        expect(result).toEqual([
          { type: 'text', content: 'Fix ' },
          { type: 'link', content: 'ABC-123', url: 'https://jira.example.com/browse/ABC-123' },
          { type: 'text', content: ' now' },
        ]);
      });

      it('matches GitHub issue numbers', () => {
        const result = parseTextWithLinks('See #42 for details', [githubIssueRule]);

        expect(result).toEqual([
          { type: 'text', content: 'See ' },
          { type: 'link', content: '#42', url: 'https://github.com/org/repo/issues/42' },
          { type: 'text', content: ' for details' },
        ]);
      });

      it('matches multiple occurrences', () => {
        const result = parseTextWithLinks('ABC-1 and ABC-2', [jiraRule]);

        // Implementation doesn't create empty text segments at boundaries
        expect(result).toHaveLength(3);
        expect(result[0]).toEqual({ type: 'link', content: 'ABC-1', url: 'https://jira.example.com/browse/ABC-1' });
        expect(result[1]).toEqual({ type: 'text', content: ' and ' });
        expect(result[2]).toEqual({ type: 'link', content: 'ABC-2', url: 'https://jira.example.com/browse/ABC-2' });
      });

      it('handles text at start and end', () => {
        const result = parseTextWithLinks('Start ABC-123 end', [jiraRule]);

        expect(result).toHaveLength(3);
        expect(result[0].type).toBe('text');
        expect(result[1].type).toBe('link');
        expect(result[2].type).toBe('text');
      });
    });

    describe('capture group substitution', () => {
      it('substitutes {1} with first capture group', () => {
        const result = parseTextWithLinks('ABC-123', [jiraRule]);
        expect(result[0].url).toBe('https://jira.example.com/browse/ABC-123');
      });

      it('substitutes {0} with full match', () => {
        const fullMatchRule: LinkRule = {
          name: 'Full',
          pattern: 'ISSUE-\\d+',
          url: 'https://example.com/{0}',
        };

        const result = parseTextWithLinks('See ISSUE-99', [fullMatchRule]);
        expect(result[1].url).toBe('https://example.com/ISSUE-99');
      });

      it('substitutes multiple capture groups', () => {
        const multiGroupRule: LinkRule = {
          name: 'Multi',
          pattern: '([A-Z]+)-([0-9]+)',
          url: 'https://example.com/project/{1}/issue/{2}',
        };

        const result = parseTextWithLinks('ABC-123', [multiGroupRule]);
        expect(result[0].url).toBe('https://example.com/project/ABC/issue/123');
      });
    });

    describe('URL encoding', () => {
      it('encodes special characters in {1}', () => {
        // Use a pattern that captures text with spaces
        const specialRule: LinkRule = {
          name: 'Special',
          pattern: 'SEARCH-([A-Za-z ]+)-END',
          url: 'https://example.com/search?q={1}',
        };

        const result = parseTextWithLinks('SEARCH-hello world-END', [specialRule]);
        expect(result[0].url).toBe('https://example.com/search?q=hello%20world');
      });

      it('does not encode with {1!raw}', () => {
        const rawRule: LinkRule = {
          name: 'Raw',
          pattern: 'PATH:([a-z/]+)',
          url: 'https://example.com/{1!raw}',
        };

        const result = parseTextWithLinks('PATH:foo/bar/baz', [rawRule]);
        expect(result[0].url).toBe('https://example.com/foo/bar/baz');
      });

      it('encodes with {1} but not {1!raw} in same template', () => {
        const mixedRule: LinkRule = {
          name: 'Mixed',
          pattern: 'REF:([a-z]+):([A-Za-z ]+)',
          url: 'https://example.com/{1!raw}?name={2}',
        };

        const result = parseTextWithLinks('REF:path:hello world', [mixedRule]);
        expect(result[0].url).toBe('https://example.com/path?name=hello%20world');
      });
    });

    describe('raw URL detection', () => {
      it('detects http URLs', () => {
        const result = parseTextWithLinks('Visit http://example.com now', []);

        expect(result).toEqual([
          { type: 'text', content: 'Visit ' },
          { type: 'link', content: 'http://example.com', url: 'http://example.com' },
          { type: 'text', content: ' now' },
        ]);
      });

      it('detects https URLs', () => {
        const result = parseTextWithLinks('Visit https://example.com/path?q=1 now', []);

        expect(result).toEqual([
          { type: 'text', content: 'Visit ' },
          { type: 'link', content: 'https://example.com/path?q=1', url: 'https://example.com/path?q=1' },
          { type: 'text', content: ' now' },
        ]);
      });

      it('does not match non-http URLs', () => {
        const result = parseTextWithLinks('ftp://example.com', []);
        expect(result).toEqual([{ type: 'text', content: 'ftp://example.com' }]);
      });
    });

    describe('overlapping patterns', () => {
      it('first match wins for overlapping patterns', () => {
        // Both rules could match, but first in processing order wins
        const result = parseTextWithLinks('ABC-123', [jiraRule, githubIssueRule]);

        // Jira rule is processed first
        expect(result).toHaveLength(1);
        expect(result[0].type).toBe('link');
        expect(result[0].url).toContain('jira.example.com');
      });

      it('prefers custom rules over raw URLs', () => {
        // If a custom rule matches, raw URL detection shouldn't override it
        const urlRule: LinkRule = {
          name: 'Custom URL',
          pattern: 'https://special.com/([^\\s]+)',
          url: 'https://redirect.com/{1}',
        };

        const result = parseTextWithLinks('See https://special.com/path', [urlRule]);

        expect(result[1].url).toBe('https://redirect.com/path');
      });
    });

    describe('edge cases', () => {
      it('handles match at very start', () => {
        const result = parseTextWithLinks('ABC-1 text', [jiraRule]);
        expect(result[0].type).toBe('link');
        expect(result[0].content).toBe('ABC-1');
      });

      it('handles match at very end', () => {
        const result = parseTextWithLinks('text ABC-1', [jiraRule]);
        expect(result[result.length - 1].type).toBe('link');
      });

      it('handles only match (no surrounding text)', () => {
        const result = parseTextWithLinks('ABC-123', [jiraRule]);
        expect(result).toHaveLength(1);
        expect(result[0]).toEqual({
          type: 'link',
          content: 'ABC-123',
          url: 'https://jira.example.com/browse/ABC-123',
        });
      });

      it('handles consecutive matches', () => {
        const result = parseTextWithLinks('ABC-1ABC-2', [jiraRule]);
        // Both should be matched
        const links = result.filter((s): s is TextSegment & { type: 'link' } => s.type === 'link');
        expect(links).toHaveLength(2);
      });
    });
  });

  describe('applyLinkRulesToMarkdown', () => {
    describe('empty/null inputs', () => {
      it('returns original text for empty string', () => {
        expect(applyLinkRulesToMarkdown('')).toBe('');
      });

      it('returns original text for null/undefined', () => {
        expect(applyLinkRulesToMarkdown(null as unknown as string)).toBe(null);
      });

      it('returns original text when no rules', () => {
        expect(applyLinkRulesToMarkdown('Hello ABC-123', [])).toBe('Hello ABC-123');
      });
    });

    describe('basic conversion', () => {
      it('converts matches to markdown links', () => {
        const result = applyLinkRulesToMarkdown('Fix ABC-123 now', [jiraRule]);
        expect(result).toBe('Fix [ABC-123](https://jira.example.com/browse/ABC-123) now');
      });

      it('converts multiple matches', () => {
        const result = applyLinkRulesToMarkdown('ABC-1 and ABC-2', [jiraRule]);
        expect(result).toBe('[ABC-1](https://jira.example.com/browse/ABC-1) and [ABC-2](https://jira.example.com/browse/ABC-2)');
      });
    });

    describe('markdown link preservation', () => {
      it('preserves existing markdown links', () => {
        const text = 'See [ABC-123](https://custom.com) for details';
        const result = applyLinkRulesToMarkdown(text, [jiraRule]);

        // Should not modify the existing link
        expect(result).toBe(text);
      });

      it('processes text outside existing markdown links', () => {
        const text = 'Fix ABC-1, see [ABC-2](https://custom.com), then ABC-3';
        const result = applyLinkRulesToMarkdown(text, [jiraRule]);

        // ABC-2 should be preserved, ABC-1 and ABC-3 should be converted
        expect(result).toContain('[ABC-1](https://jira.example.com/browse/ABC-1)');
        expect(result).toContain('[ABC-2](https://custom.com)'); // Preserved
        expect(result).toContain('[ABC-3](https://jira.example.com/browse/ABC-3)');
      });

      it('does not create nested markdown links', () => {
        const text = '[Link with ABC-123 inside](https://example.com)';
        const result = applyLinkRulesToMarkdown(text, [jiraRule]);

        // Should not modify text inside existing link
        expect(result).toBe(text);
      });
    });

    describe('complex markdown scenarios', () => {
      it('handles multiple existing links with matches between', () => {
        const text = '[First](url1) ABC-1 [Second](url2) ABC-2 [Third](url3)';
        const result = applyLinkRulesToMarkdown(text, [jiraRule]);

        expect(result).toContain('[First](url1)');
        expect(result).toContain('[ABC-1](https://jira.example.com/browse/ABC-1)');
        expect(result).toContain('[Second](url2)');
        expect(result).toContain('[ABC-2](https://jira.example.com/browse/ABC-2)');
        expect(result).toContain('[Third](url3)');
      });

      it('handles adjacent link and match', () => {
        const text = '[Link](url)ABC-123';
        const result = applyLinkRulesToMarkdown(text, [jiraRule]);

        expect(result).toBe('[Link](url)[ABC-123](https://jira.example.com/browse/ABC-123)');
      });
    });
  });

  describe('integration scenarios', () => {
    it('handles realistic Jira + GitHub config', () => {
      const rules: LinkRule[] = [jiraRule, githubIssueRule];
      const text = 'Fix ABC-123, related to #42';

      const segments = parseTextWithLinks(text, rules);

      expect(segments).toHaveLength(4);
      expect(segments[1]).toEqual({
        type: 'link',
        content: 'ABC-123',
        url: 'https://jira.example.com/browse/ABC-123',
      });
      expect(segments[3]).toEqual({
        type: 'link',
        content: '#42',
        url: 'https://github.com/org/repo/issues/42',
      });
    });

    it('handles PR reference pattern', () => {
      const prRule: LinkRule = {
        name: 'GitHub PR',
        pattern: '([a-zA-Z0-9_-]+)/([a-zA-Z0-9_-]+)#(\\d+)',
        url: 'https://github.com/{1!raw}/{2!raw}/pull/{3}',
      };

      const result = parseTextWithLinks('See org/repo#123', [prRule]);

      expect(result[1].url).toBe('https://github.com/org/repo/pull/123');
    });

    it('handles URL with special characters needing encoding', () => {
      const searchRule: LinkRule = {
        name: 'Search',
        pattern: '@search\\(([^)]+)\\)',
        url: 'https://search.example.com/?q={1}',
      };

      const result = parseTextWithLinks('@search(hello world)', [searchRule]);
      expect(result[0].url).toBe('https://search.example.com/?q=hello%20world');
    });
  });
});
