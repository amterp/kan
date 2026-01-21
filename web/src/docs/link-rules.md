# Link Rules

Link rules automatically convert text patterns into clickable links. This is useful for linking issue trackers, pull requests, or any text that follows a predictable pattern.

## Configuration

Link rules are defined in your board's `config.toml` file under the `link_rules` section:

```toml
[[link_rules]]
  name = "Jira"
  pattern = "([A-Z]+-\\d+)"
  url = "https://jira.example.com/browse/{1}"

[[link_rules]]
  name = "GitHub Issue"
  pattern = "#(\\d+)"
  url = "https://github.com/org/repo/issues/{1}"
```

Each rule has three required fields:

| Field | Description |
|-------|-------------|
| `name` | Human-readable name for the rule (shown in warnings) |
| `pattern` | Regular expression to match |
| `url` | URL template with placeholders |

## Pattern Syntax

Patterns use standard regex syntax. Use capture groups `()` to extract portions of the match for use in the URL template.

### Examples

**Jira tickets** like `ABC-123`:
```toml
pattern = "([A-Z]+-\\d+)"
```

**GitHub issues** like `#42`:
```toml
pattern = "#(\\d+)"
```

**GitHub PRs** with org/repo like `org/repo#123`:
```toml
pattern = "([a-zA-Z0-9_-]+)/([a-zA-Z0-9_-]+)#(\\d+)"
```

Note: In TOML, backslashes must be escaped (use `\\d` not `\d`).

## URL Template Placeholders

URL templates support these placeholders:

| Placeholder | Description |
|-------------|-------------|
| `{0}` | The entire matched text (URL encoded) |
| `{1}`, `{2}`, ... | Capture groups 1, 2, etc. (URL encoded) |
| `{0!raw}`, `{1!raw}`, ... | Same as above, but *not* URL encoded |

### When to use `!raw`

Use the `!raw` suffix when the captured text should be inserted as-is, typically for path segments:

```toml
# Good: path segments don't need encoding
url = "https://github.com/{1!raw}/{2!raw}/pull/{3}"

# Good: query parameters should be encoded
url = "https://search.example.com/?q={1}"
```

## How Matches Work

1. **Rules are processed in order** — earlier rules take priority over later ones
2. **First match wins** — if two patterns could match the same text, the first match is used
3. **Existing links are preserved** — text already inside markdown links `[text](url)` is not processed
4. **Raw URLs are detected** — `http://` and `https://` URLs are automatically linked even without rules

## Troubleshooting

### Invalid Regex Warning

If a pattern has invalid regex syntax, you'll see a warning when loading the board:

```
Warning: link_rules: invalid regex in 'Rule Name': error details
```

The rule will be skipped, but other rules will still work. Fix the pattern syntax to resolve the warning.

### Pattern Not Matching

If your pattern isn't matching as expected:

1. Test your regex using a site like [regexr.com](https://regexr.com/) or [regex101.com](https://regex101.com/) (use JavaScript flavor)
2. Remember to escape backslashes in TOML: `\\d` not `\d`
3. Check if an earlier rule is matching the text first

### Special Characters in URLs

If matched text contains special characters that break URLs, use the encoded placeholder (without `!raw`):

```toml
# Search query might contain spaces or special chars
url = "https://search.example.com/?q={1}"  # Encodes to ?q=hello%20world
```
