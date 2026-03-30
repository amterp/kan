// Generates llms.txt, llms-full.txt, and raw markdown copies for the Kan docs site.
// Run as a post-build step: node scripts/generate-llms-txt.mjs
//
// Auto-discovers all .md files in web/src/docs/ and extracts their H1 as the title.
// Adding a new doc page requires no changes here.

import { readFileSync, readdirSync, writeFileSync, mkdirSync, copyFileSync } from 'fs';
import { join, dirname } from 'path';
import { fileURLToPath } from 'url';

const __dirname = dirname(fileURLToPath(import.meta.url));
const docsDir = join(__dirname, '..', 'src', 'docs');
const distDir = join(__dirname, '..', '..', 'internal', 'api', 'dist');

// SITE_URL is the public site root (e.g. "https://amterp.dev/kan").
// Set in CI for absolute URLs. When absent, use relative paths.
const siteUrl = process.env.SITE_URL; // no trailing slash
const baseUrl = siteUrl ? `${siteUrl}/` : '/';

// Auto-discover doc files: index first, then alphabetical.
const docFiles = readdirSync(docsDir)
  .filter(f => f.endsWith('.md'))
  .sort((a, b) => {
    if (a === 'index.md') return -1;
    if (b === 'index.md') return 1;
    return a.localeCompare(b);
  });

function extractH1(content) {
  const match = content.match(/^# (.+)$/m);
  return match ? match[1] : null;
}

function extractH2s(content) {
  return [...content.matchAll(/^## (.+)$/gm)].map(m => m[1]);
}

function stripH1(content) {
  return content.replace(/^# .+\n+/, '');
}

const docs = docFiles.map(file => {
  const slug = file.replace('.md', '');
  const content = readFileSync(join(docsDir, file), 'utf-8');
  const title = extractH1(content) || slug;
  const sections = extractH2s(content);
  return { slug, title, sections, content };
});

const header = `# Kan

> A file-based kanban board CLI tool. All data lives as plain files - no database, no server. Works with any VCS.

Kan manages kanban boards using plain files in a \`.kan/\` directory. It includes both a CLI and a web UI (served via \`kan serve\`). Boards, columns, and cards are stored as TOML and JSON files, making them easy to version control and merge.`;

// -- llms.txt --

const llmsTxtLinks = docs.map(d => {
  const url = `${baseUrl}docs/${d.slug}.md`;
  const suffix = d.sections.length > 0 ? `: ${d.sections.join(', ')}` : '';
  return `- [${d.title}](${url})${suffix}`;
});

const llmsTxt = `${header}

## Docs

${llmsTxtLinks.join('\n')}
`;

// -- llms-full.txt --

const llmsFullSections = docs.map(d => {
  const body = stripH1(d.content).trim();
  return `## ${d.title}\n\n${body}`;
});

const llmsFullTxt = `${header}

${llmsFullSections.join('\n\n')}
`;

// -- Write outputs --

writeFileSync(join(distDir, 'llms.txt'), llmsTxt);
writeFileSync(join(distDir, 'llms-full.txt'), llmsFullTxt);

// Copy raw markdown files so llms.txt links resolve
const distDocsDir = join(distDir, 'docs');
mkdirSync(distDocsDir, { recursive: true });
for (const d of docs) {
  copyFileSync(join(docsDir, `${d.slug}.md`), join(distDocsDir, `${d.slug}.md`));
}

// -- sitemap.xml and robots.txt (public builds only) --

if (siteUrl) {
  const urls = [
    `${siteUrl}/llms.txt`,
    `${siteUrl}/llms-full.txt`,
    ...docs.map(d => `${siteUrl}/docs/${d.slug}.md`),
  ];
  const sitemap = [
    '<?xml version="1.0" encoding="UTF-8"?>',
    '<urlset xmlns="http://www.sitemaps.org/schemas/sitemap/0.9">',
    ...urls.map(u => `  <url><loc>${u}</loc></url>`),
    '</urlset>',
    '',
  ].join('\n');
  writeFileSync(join(distDir, 'sitemap.xml'), sitemap);

  const robots = `User-agent: *\nAllow: /\n\nSitemap: ${siteUrl}/sitemap.xml\n`;
  writeFileSync(join(distDir, 'robots.txt'), robots);

  console.log(`Generated sitemap.xml and robots.txt`);
}

console.log(`Generated llms.txt, llms-full.txt, and ${docs.length} doc files in ${distDir}`);
