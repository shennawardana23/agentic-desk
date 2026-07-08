// markdown.js renders agent chat replies (LLM-generated markdown) into
// sanitized HTML: marked → highlight.js (code fences) → DOMPurify. This is
// the only place chat markdown is parsed — ChatView.vue calls renderMarkdown
// and drops the result straight into v-html, so every allow-listed tag/attr
// here is a deliberate security decision, not an oversight.
//
// Code fences get a small header bar (language label + a plain-text "Copy"
// button) added by the custom code renderer below; ChatView.vue wires up
// the actual clipboard behavior via event delegation (there is no inline
// onclick — DOMPurify strips on* attributes anyway, and delegation avoids
// a global window function).
//
// ```mermaid fences render as a <div class="mermaid-block" data-mermaid-
// pending> wrapping a plain fallback <pre><code>; ChatView.vue lazily
// dynamic-imports 'mermaid' after the HTML is mounted and replaces the
// wrapper's contents with the rendered SVG, leaving the fallback code
// block in place if rendering fails.

import DOMPurify from 'dompurify'
import hljs from 'highlight.js'
// The code-block-wrapper CSS in ChatView.vue/CatalogItemModal.vue hardcodes
// a GitHub-Dark background (#0d1117) for every fence regardless of the app's
// own light/dark theme, but never imported a matching hljs theme — every
// `.hljs-*` token span had no color rule, so syntax text rendered in
// whatever text color the surrounding light/dark theme happened to set
// (dark-on-dark in light mode, the bug reported in a screenshot). This is
// the only missing piece; it needs no runtime theme-switching since the
// code-block background itself never changes with the app theme.
import 'highlight.js/styles/github-dark.css'
import { marked } from 'marked'

marked.setOptions({
  gfm: true,
  breaks: true,
  pedantic: false,
})

const LANG_DISPLAY_NAMES = {
  go: 'Go',
  golang: 'Go',
  javascript: 'JavaScript',
  js: 'JavaScript',
  jsx: 'JSX',
  typescript: 'TypeScript',
  ts: 'TypeScript',
  tsx: 'TSX',
  python: 'Python',
  py: 'Python',
  ruby: 'Ruby',
  rb: 'Ruby',
  java: 'Java',
  kotlin: 'Kotlin',
  swift: 'Swift',
  rust: 'Rust',
  c: 'C',
  cpp: 'C++',
  'c++': 'C++',
  csharp: 'C#',
  'c#': 'C#',
  bash: 'Bash',
  sh: 'Shell',
  shell: 'Shell',
  sql: 'SQL',
  html: 'HTML',
  css: 'CSS',
  scss: 'SCSS',
  yaml: 'YAML',
  yml: 'YAML',
  json: 'JSON',
  xml: 'XML',
  markdown: 'Markdown',
  md: 'Markdown',
  dockerfile: 'Dockerfile',
  makefile: 'Makefile',
  toml: 'TOML',
  ini: 'INI',
  diff: 'Diff',
  plaintext: 'Plain Text',
  text: 'Plain Text',
}

function langDisplayName(rawLang) {
  if (!rawLang) return 'Code'
  const key = rawLang.toLowerCase()
  return LANG_DISPLAY_NAMES[key] || rawLang.charAt(0).toUpperCase() + rawLang.slice(1)
}

function escapeHTML(text) {
  return String(text)
    .replace(/&/g, '&amp;')
    .replace(/</g, '&lt;')
    .replace(/>/g, '&gt;')
    .replace(/"/g, '&quot;')
    .replace(/'/g, '&#39;')
}

const REJECTED_AUTO_LANGS = new Set(['apache', 'ini', 'properties', 'accesslog', 'http'])

const renderer = new marked.Renderer()

renderer.code = ({ text, lang }) => {
  const rawLang = (lang || '').trim().toLowerCase()

  // ```mermaid fences: emit a placeholder the component upgrades to an SVG
  // diagram after mount, with the raw source kept as a plain-text fallback.
  if (rawLang === 'mermaid') {
    return `<div class="mermaid-block" data-mermaid-pending="1"><pre class="mermaid-fallback"><code>${escapeHTML(text)}</code></pre></div>`
  }

  let resolvedLang = rawLang
  let result
  if (resolvedLang && hljs.getLanguage(resolvedLang)) {
    result = hljs.highlight(text, { language: resolvedLang, ignoreIllegals: true })
  } else if (resolvedLang) {
    result = hljs.highlight(text, { language: 'plaintext', ignoreIllegals: true })
  } else {
    const auto = hljs.highlightAuto(text)
    if (!auto.language || REJECTED_AUTO_LANGS.has(auto.language)) {
      resolvedLang = 'plaintext'
      result = hljs.highlight(text, { language: 'plaintext', ignoreIllegals: true })
    } else {
      resolvedLang = auto.language
      result = auto
    }
  }

  const displayName = langDisplayName(resolvedLang || rawLang)
  return (
    `<div class="code-block-wrapper">` +
    `<div class="code-block-header"><span class="code-lang">${escapeHTML(displayName)}</span>` +
    `<button type="button" class="copy-code-btn" data-copy-code="1">Copy</button></div>` +
    `<pre><code class="hljs language-${escapeHTML(resolvedLang || 'plaintext')}">${result.value}</code></pre>` +
    `</div>`
  )
}

// Links always open safely in a new tab regardless of what the model wrote.
renderer.link = ({ href, title, tokens }) => {
  const text = renderer.parser.parseInline(tokens)
  const safeHref = escapeHTML(href || '#')
  const titleAttr = title ? ` title="${escapeHTML(title)}"` : ''
  return `<a href="${safeHref}"${titleAttr} target="_blank" rel="noopener noreferrer">${text}</a>`
}

marked.use({ renderer })

const ALLOWED = Object.freeze({
  ALLOWED_TAGS: [
    'a', 'b', 'blockquote', 'br', 'button', 'code', 'del', 'div', 'em',
    'h1', 'h2', 'h3', 'h4', 'h5', 'h6', 'hr', 'i', 'img', 'li', 'ol', 'p',
    'pre', 'span', 'strong', 'table', 'tbody', 'td', 'th', 'thead', 'tr', 'ul',
  ],
  ALLOWED_ATTR: [
    'href', 'src', 'alt', 'title', 'class', 'id', 'lang', 'target', 'rel',
    'type', 'width', 'height', 'data-mermaid-pending', 'data-copy-code',
  ],
})

/**
 * stripFrontmatter removes a leading "---\n...\n---" YAML block (the
 * name:/description:/category: header every SKILL.md/.prompt file in
 * skills/ and prompts/ starts with — see internal/library.go's
 * parseFrontmatter, which already extracts those fields separately).
 * Catalog content is fetched once and shown in two tabs — Raw wants the
 * frontmatter (it's a faithful dump of the real file), Preview doesn't
 * (those fields are already surfaced as their own UI elements) — so this
 * is applied only where Preview renders, not at the fetch/store layer.
 *
 * @param {string} text
 * @returns {string}
 */
export function stripFrontmatter(text) {
  if (!text) return text
  const match = /^---\r?\n[\s\S]*?\r?\n---\r?\n?/.exec(text)
  return match ? text.slice(match[0].length) : text
}

/**
 * renderMarkdown converts LLM-produced markdown into sanitized HTML, safe
 * to drop directly into v-html. Never call this on user-typed text.
 *
 * @param {string} text
 * @returns {string}
 */
export function renderMarkdown(text) {
  if (!text) return ''
  let html
  try {
    html = marked.parse(String(text))
  } catch (_err) {
    html = escapeHTML(String(text))
  }
  return DOMPurify.sanitize(html, ALLOWED)
}
