<!--
  Shared detail modal for both the Skill Catalog and the Prompt Catalog
  (SkillCatalogView.vue / PromptCatalogView.vue) — same component and
  styling for both, differentiated only by the `kind` prop (header icon +
  accent chip) so the two catalogs don't read as identical screens.
-->
<script setup>
import { computed, ref, watch } from 'vue'
import { renderMarkdown, stripFrontmatter } from '../lib/markdown'

const props = defineProps({
  kind: { type: String, required: true }, // 'skill' | 'prompt'
  name: { type: String, required: true },
  content: { type: String, default: '' },
  loading: { type: Boolean, default: false },
  error: { type: String, default: '' },
})
const emit = defineEmits(['close'])

const tab = ref('preview')
const copied = ref(false)
// Raw keeps props.content verbatim (a faithful dump of the real file,
// frontmatter included); Preview strips the leading name:/description:/
// category: block since those are already shown in the modal header/chips.
const previewHtml = computed(() => renderMarkdown(stripFrontmatter(props.content)))

// Reset to the Preview tab and clear any stale "Copied" state each time a
// different item is opened in the (single, reused) modal instance.
watch(
  () => props.name,
  () => {
    tab.value = 'preview'
    copied.value = false
  },
)

async function copyContent() {
  const text = tab.value === 'preview' ? stripFrontmatter(props.content) : props.content
  try {
    await navigator.clipboard.writeText(text)
    copied.value = true
    setTimeout(() => (copied.value = false), 1500)
  } catch (err) {
    console.warn('[CatalogItemModal] clipboard copy failed:', err)
  }
}

// Copy-to-clipboard for fenced code blocks in the Preview tab — same
// delegated-listener pattern as ChatView.vue's onScrollerClick (markdown.js
// renders a plain data-copy-code button, no inline onclick since DOMPurify
// strips on* attributes).
async function onCodeBlockClick(e) {
  const btn = e.target.closest?.('[data-copy-code]')
  if (!btn) return
  const code = btn.closest('.code-block-wrapper')?.querySelector('code')
  if (!code) return
  try {
    await navigator.clipboard.writeText(code.textContent ?? '')
    const original = btn.textContent
    btn.textContent = 'Copied'
    btn.classList.add('is-copied')
    setTimeout(() => {
      btn.textContent = original
      btn.classList.remove('is-copied')
    }, 1500)
  } catch (err) {
    console.warn('[CatalogItemModal] code block copy failed:', err)
  }
}
</script>

<template>
  <div class="modal-overlay" @click.self="emit('close')">
    <div class="modal-panel" :class="kind" role="dialog" aria-modal="true" :aria-label="name">
      <header class="modal-head">
        <span class="kind-icon" :class="kind">
          <svg v-if="kind === 'skill'" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round">
            <rect x="4" y="4" width="7" height="7" rx="1.5" />
            <rect x="13" y="4" width="7" height="7" rx="1.5" />
            <rect x="4" y="13" width="7" height="7" rx="1.5" />
            <rect x="13" y="13" width="7" height="7" rx="1.5" />
          </svg>
          <svg v-else viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round">
            <path d="M7 3.5h7l3 3v13a1 1 0 0 1-1 1H7a1 1 0 0 1-1-1v-15a1 1 0 0 1 1-1z" />
            <line x1="8.5" y1="10" x2="15.5" y2="10" />
            <line x1="8.5" y1="13.5" x2="15.5" y2="13.5" />
            <line x1="8.5" y1="17" x2="13" y2="17" />
          </svg>
        </span>
        <div class="modal-title">
          <h2>{{ name }}</h2>
          <span class="kind-chip" :class="kind">{{ kind === 'skill' ? 'Skill' : 'Prompt' }}</span>
        </div>
        <button type="button" class="modal-close" aria-label="Close" @click="emit('close')">
          <svg viewBox="0 0 24 24" width="16" height="16" fill="none" stroke="currentColor" stroke-width="2">
            <path d="M6 6l12 12M18 6L6 18" stroke-linecap="round" />
          </svg>
        </button>
      </header>

      <div class="modal-toolbar">
        <div class="modal-tabs" role="tablist">
          <button type="button" role="tab" :aria-selected="tab === 'preview'" :class="{ 'is-active': tab === 'preview' }" @click="tab = 'preview'">
            Preview
          </button>
          <button type="button" role="tab" :aria-selected="tab === 'raw'" :class="{ 'is-active': tab === 'raw' }" @click="tab = 'raw'">
            Raw
          </button>
        </div>
        <button type="button" class="copy-btn" :class="{ 'is-copied': copied }" :disabled="!content" @click="copyContent">
          {{ copied ? 'Copied' : 'Copy' }}
        </button>
      </div>

      <div class="modal-body">
        <p v-if="loading" class="modal-note">Loading…</p>
        <p v-else-if="error" class="modal-error" role="alert">{{ error }}</p>
        <div v-else-if="tab === 'preview'" class="doc-preview" v-html="previewHtml" @click="onCodeBlockClick" />
        <pre v-else class="doc-raw"><code>{{ content }}</code></pre>
      </div>
    </div>
  </div>
</template>

<style scoped>
.modal-overlay {
  position: fixed;
  inset: 0;
  display: flex;
  align-items: center;
  justify-content: center;
  padding: 24px;
  background: oklch(0 0 0 / 0.32);
  z-index: 50;
}

.modal-panel {
  display: flex;
  flex-direction: column;
  width: min(720px, 100%);
  max-height: min(80vh, 720px);
  border-radius: var(--radius-lg);
  background: var(--surface);
  border: 1px solid var(--border);
  box-shadow: 0 12px 40px oklch(0 0 0 / 0.18);
  overflow: hidden;
}

.modal-head {
  display: flex;
  align-items: center;
  gap: 10px;
  flex-shrink: 0;
  padding: 16px 14px 14px 18px;
  border-bottom: 1px solid var(--border);
}

.kind-icon {
  display: flex;
  align-items: center;
  justify-content: center;
  width: 30px;
  height: 30px;
  flex-shrink: 0;
  border-radius: var(--radius-sm);
}

.kind-icon svg {
  width: 17px;
  height: 17px;
}

.kind-icon.skill {
  background: color-mix(in srgb, #0d9488 12%, transparent);
  color: #0d9488;
}

.kind-icon.prompt {
  background: color-mix(in srgb, #7c3aed 12%, transparent);
  color: #7c3aed;
}

.modal-title {
  flex: 1;
  min-width: 0;
  display: flex;
  align-items: center;
  gap: 8px;
}

.modal-title h2 {
  margin: 0;
  font-size: 15px;
  color: var(--ink);
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}

.kind-chip {
  flex-shrink: 0;
  padding: 2px 8px;
  border-radius: 999px;
  font-size: 10px;
  font-weight: 700;
  text-transform: uppercase;
  letter-spacing: 0.03em;
}

.kind-chip.skill {
  background: color-mix(in srgb, #0d9488 12%, transparent);
  color: #0d9488;
}

.kind-chip.prompt {
  background: color-mix(in srgb, #7c3aed 12%, transparent);
  color: #7c3aed;
}

.modal-close {
  display: flex;
  align-items: center;
  justify-content: center;
  width: 28px;
  height: 28px;
  flex-shrink: 0;
  border: none;
  border-radius: 50%;
  background: transparent;
  color: var(--ink-faint);
  cursor: pointer;
  transition: background-color 150ms var(--ease-out-expo), color 150ms var(--ease-out-expo);
}

.modal-close:hover {
  background: var(--surface-hover);
  color: var(--ink);
}

.modal-toolbar {
  display: flex;
  align-items: center;
  justify-content: space-between;
  flex-shrink: 0;
  padding: 10px 14px;
  border-bottom: 1px solid var(--border);
}

.modal-tabs {
  display: flex;
  gap: 2px;
  padding: 2px;
  border-radius: var(--radius-sm);
  background: var(--surface-hover);
}

.modal-tabs button {
  padding: 5px 12px;
  border: none;
  border-radius: calc(var(--radius-sm) - 2px);
  background: transparent;
  color: var(--ink-muted);
  font-size: 12.5px;
  font-weight: 600;
  cursor: pointer;
  transition: background-color 150ms var(--ease-out-expo), color 150ms var(--ease-out-expo);
}

.modal-tabs button.is-active {
  background: var(--surface);
  color: var(--ink);
}

.copy-btn {
  padding: 5px 12px;
  border: 1px solid var(--border);
  border-radius: var(--radius-sm);
  background: transparent;
  color: var(--ink-muted);
  font-size: 12.5px;
  font-weight: 600;
  cursor: pointer;
  transition: background-color 150ms var(--ease-out-expo), color 150ms var(--ease-out-expo), border-color 150ms var(--ease-out-expo);
}

.copy-btn:hover:not(:disabled) {
  background: var(--surface-hover);
  color: var(--ink);
}

.copy-btn:disabled {
  opacity: 0.5;
  cursor: default;
}

.copy-btn.is-copied {
  border-color: var(--accent);
  color: var(--accent);
}

.modal-panel.skill .copy-btn.is-copied {
  border-color: #0d9488;
  color: #0d9488;
}

.modal-panel.prompt .copy-btn.is-copied {
  border-color: #7c3aed;
  color: #7c3aed;
}

.modal-body {
  flex: 1;
  min-height: 0;
  overflow-y: auto;
  padding: 18px 20px;
}

.modal-note {
  margin: 0;
  font-size: 13px;
  color: var(--ink-faint);
}

.modal-error {
  margin: 0;
  padding: 8px 12px;
  border-radius: var(--radius-sm);
  background: var(--danger-soft);
  color: var(--danger);
  font-size: 12px;
}

.doc-raw {
  margin: 0;
  font-size: 12.5px;
  line-height: 1.6;
  white-space: pre-wrap;
  word-break: break-word;
  color: var(--ink);
  font-family: ui-monospace, SFMono-Regular, Menlo, monospace;
}

.doc-preview {
  font-size: 13.5px;
  line-height: 1.6;
  color: var(--ink);
}

.doc-preview :deep(p) {
  margin: 0 0 10px;
}

.doc-preview :deep(p:last-child) {
  margin-bottom: 0;
}

.doc-preview :deep(h1),
.doc-preview :deep(h2),
.doc-preview :deep(h3),
.doc-preview :deep(h4) {
  margin: 14px 0 8px;
  font-weight: 600;
  color: var(--ink);
}

.doc-preview :deep(h1:first-child),
.doc-preview :deep(h2:first-child),
.doc-preview :deep(h3:first-child) {
  margin-top: 0;
}

.doc-preview :deep(ul),
.doc-preview :deep(ol) {
  margin: 0 0 10px;
  padding-left: 22px;
}

.doc-preview :deep(li) {
  margin: 3px 0;
}

.doc-preview :deep(a) {
  color: var(--accent);
  text-decoration: underline;
}

.doc-preview :deep(code) {
  padding: 2px 5px;
  border-radius: 4px;
  background: var(--surface-hover);
  font-size: 12.5px;
  font-family: ui-monospace, 'SF Mono', Menlo, Consolas, monospace;
}

.doc-preview :deep(pre) {
  padding: 12px 14px;
  border-radius: var(--radius-sm);
  border: 1px solid var(--border);
  background: var(--surface-hover);
  overflow-x: auto;
  margin: 0 0 10px;
}

.doc-preview :deep(pre code) {
  background: transparent;
  padding: 0;
}

.doc-preview :deep(blockquote) {
  margin: 0 0 10px;
  padding: 2px 12px;
  border-left: 2px solid var(--border);
  color: var(--ink-muted);
}

.doc-preview :deep(hr) {
  border: none;
  border-top: 1px solid var(--border);
  margin: 12px 0;
}

.doc-preview :deep(table) {
  width: 100%;
  border-collapse: collapse;
  margin: 4px 0 12px;
  font-size: 13px;
}

.doc-preview :deep(th),
.doc-preview :deep(td) {
  border: 1px solid var(--border);
  padding: 6px 10px;
  text-align: left;
}

.doc-preview :deep(th) {
  background: var(--surface-hover);
  font-weight: 600;
}

.doc-preview :deep(.code-block-wrapper) {
  margin: 4px 0 12px;
  border-radius: var(--radius-sm);
  overflow: hidden;
  border: 1px solid var(--border);
  background: #0d1117;
}

.doc-preview :deep(.code-block-wrapper pre) {
  margin: 0;
  padding: 12px 14px;
  overflow-x: auto;
  border: none;
  background: transparent;
}

.doc-preview :deep(.code-block-wrapper code) {
  background: transparent;
  padding: 0;
  font-size: 12.5px;
}

.doc-preview :deep(.code-block-header) {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 6px 12px;
  background: oklch(0 0 0 / 0.25);
  border-bottom: 1px solid oklch(1 0 0 / 0.08);
}

.doc-preview :deep(.code-lang) {
  font-size: 11px;
  color: oklch(0.8 0.01 316);
  text-transform: uppercase;
  letter-spacing: 0.03em;
}

.doc-preview :deep(.copy-code-btn) {
  border: none;
  background: transparent;
  color: oklch(0.8 0.01 316);
  font-size: 11px;
  cursor: pointer;
  padding: 2px 6px;
  border-radius: 4px;
}

.doc-preview :deep(.copy-code-btn:hover) {
  background: oklch(1 0 0 / 0.1);
  color: #fff;
}

.doc-preview :deep(.copy-code-btn.is-copied) {
  color: #6ee7b7;
}
</style>
