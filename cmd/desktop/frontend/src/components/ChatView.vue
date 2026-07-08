<script setup>
import { computed, nextTick, onMounted, ref, watch } from 'vue'
import backgroundUrl from '../assets/background-desktop.svg'
import BrandMark from './icons/BrandMark.vue'
import { useCoreStore } from '../stores/core'
import { renderMarkdown } from '../lib/markdown'

const core = useCoreStore()
const input = ref('')

// Chat is the app's home/default screen now, not a separate feature
// bolted next to a Home landing page — its empty-state hero absorbs
// what HomeView.vue used to show (the time-of-day greeting), so
// there's exactly one landing experience, not two. Single-user desktop
// app, no user-identity concept in the backend — name is a constant.
const USER_NAME = 'Shenna'
const hour = new Date().getHours()
const greeting = `${hour < 11 ? 'Selamat Pagi' : hour < 18 ? 'Selamat Siang' : 'Selamat Malam'}, ${USER_NAME}`
const pendingImage = ref(null) // { name, dataUrl }
const fileInput = ref(null)

// core.chatError covers two different failures with one field: core.init()
// sets it once if the auto-launched cmd/core process itself never came up
// (a real "can't reach the backend" case), while sendChatMessage sets it
// per-turn when the backend answered fine but generation failed server-side
// (network hiccup, provider rate limit, etc. — sanitized to a generic
// "internal error" string before it ever reaches this client, so the exact
// cause isn't visible here). Conflating the two under one banner text was
// misleading for the far more common per-turn case; only the startup one
// is an actual unreachable-backend situation.
const isStartupError = computed(() => core.chatError.startsWith('Core failed to start'))
const chatErrorMessage = computed(() =>
  isStartupError.value
    ? `Sarza can't reach the backend right now: ${core.chatError}`
    : `Sarza hit a problem generating that reply (${core.chatError}). This is usually transient — try again.`,
)
const scroller = ref(null)

// Follow the stream: new messages AND growing streamed content/reasoning
// on the last (pending) message both scroll the log — watching length
// alone would stop following once streaming starts mutating in place.
watch(
  () => {
    const last = core.chatMessages[core.chatMessages.length - 1]
    return `${core.chatMessages.length}:${last ? (last.content?.length || 0) + (last.reasoning?.length || 0) : 0}`
  },
  async () => {
    await nextTick()
    if (scroller.value) scroller.value.scrollTop = scroller.value.scrollHeight
    renderMermaidBlocks()
  },
)

// Per-message CoT visibility: auto-open while streaming, collapsed once
// done, user-toggleable any time.
function reasoningOpen(m) {
  return m.showReasoning ?? m.pending
}
function toggleReasoning(m) {
  m.showReasoning = !reasoningOpen(m)
}

// ```mermaid fences render as a plain fallback code block from
// markdown.js; upgrade any pending ones in the DOM to an SVG diagram once
// mounted. Lazily imported — most chats never use mermaid, so the ~1MB+
// dependency shouldn't ship in the initial bundle.
async function renderMermaidBlocks() {
  const pending = scroller.value?.querySelectorAll?.('[data-mermaid-pending]')
  if (!pending || pending.length === 0) return
  let mermaid
  try {
    mermaid = (await import('mermaid')).default
    mermaid.initialize({ startOnLoad: false, theme: 'dark', securityLevel: 'strict' })
  } catch (err) {
    console.warn('[ChatView] mermaid failed to load, leaving fallback code blocks:', err)
    return
  }
  for (const el of pending) {
    const source = el.querySelector('code')?.textContent ?? ''
    el.removeAttribute('data-mermaid-pending')
    try {
      const id = `mermaid-${Date.now()}-${Math.random().toString(36).slice(2, 8)}`
      const { svg } = await mermaid.render(id, source)
      el.innerHTML = svg
    } catch (err) {
      console.warn('[ChatView] mermaid render failed, keeping fallback code block:', err)
      // Leave the fallback <pre><code> already rendered by markdown.js.
    }
  }
}

// Copy-to-clipboard for code blocks: markdown.js renders a plain "Copy"
// button per fenced code block (data-copy-code) instead of an inline
// onclick handler (DOMPurify strips on* attributes, and a global window
// function is unnecessary here) — a single delegated listener on the
// scroller handles every code block, current and future.
async function onScrollerClick(e) {
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
    console.warn('[ChatView] clipboard copy failed:', err)
  }
}

function pickFile() {
  fileInput.value?.click()
}

function onFileSelected(e) {
  const file = e.target.files?.[0]
  e.target.value = ''
  if (!file) return
  const reader = new FileReader()
  reader.onload = () => {
    pendingImage.value = { name: file.name, dataUrl: reader.result }
  }
  reader.readAsDataURL(file)
}

function clearPendingImage() {
  pendingImage.value = null
}

// Input history: ArrowUp/ArrowDown cycles back/forward through this
// conversation's previously sent user messages, shell-style — only while
// the input is empty (start of a cycle) or already mid-cycle (so normal
// text editing with the arrow keys elsewhere in a line is untouched).
const historyIndex = ref(-1) // -1 = not cycling
const historyDraft = ref('') // whatever the user was typing before cycling started

function sentMessages() {
  return core.chatMessages.filter((m) => m.role === 'user').map((m) => m.content)
}

function historyUp() {
  const sent = sentMessages()
  if (sent.length === 0) return
  if (historyIndex.value === -1) {
    if (input.value.trim().length > 0) return // mid-line edit, not a fresh cycle
    historyDraft.value = input.value
    historyIndex.value = sent.length - 1
  } else if (historyIndex.value > 0) {
    historyIndex.value -= 1
  } else {
    return // already at the oldest message
  }
  input.value = sent[historyIndex.value]
}

function historyDown() {
  if (historyIndex.value === -1) return
  const sent = sentMessages()
  if (historyIndex.value < sent.length - 1) {
    historyIndex.value += 1
    input.value = sent[historyIndex.value]
  } else {
    historyIndex.value = -1
    input.value = historyDraft.value
  }
}

function resetHistoryCycle() {
  historyIndex.value = -1
}

async function submit() {
  const content = input.value.trim()
  if (!content || core.sendingChat) return
  const imageDataUrl = pendingImage.value?.dataUrl
  input.value = ''
  pendingImage.value = null
  resetHistoryCycle()
  await core.sendChatMessage(content, imageDataUrl)
}

// Header actions + History drawer
const showHistory = ref(false)
const historyQuery = ref('')
const openMenuId = ref(null)
const renamingId = ref(null)
const renameDraft = ref('')

const filteredSessions = computed(() => {
  const q = historyQuery.value.trim().toLowerCase()
  const list = core.chatSessions
  if (!q) return list
  return list.filter((s) => (s.title || 'Untitled Chat').toLowerCase().includes(q))
})

function toggleHistory() {
  showHistory.value = !showHistory.value
  if (showHistory.value) core.loadChatSessions()
  openMenuId.value = null
}

function startNewChat() {
  core.newChat()
  resetHistoryCycle()
  input.value = ''
  pendingImage.value = null
}

async function selectSession(id) {
  if (id === core.currentChatSessionId) return
  await core.loadChatSession(id)
  resetHistoryCycle()
  openMenuId.value = null
}

function toggleMenu(id) {
  openMenuId.value = openMenuId.value === id ? null : id
  renamingId.value = null
}

function startRename(session) {
  renamingId.value = session.id
  renameDraft.value = session.title || ''
  openMenuId.value = null
}

async function confirmRename(session) {
  const title = renameDraft.value.trim()
  renamingId.value = null
  if (!title || title === session.title) return
  await core.renameChatSession(session.id, title)
}

async function confirmDelete(session) {
  openMenuId.value = null
  if (!window.confirm(`Delete "${session.title || 'Untitled Chat'}"? This can't be undone.`)) return
  await core.deleteChatSession(session.id)
}

function formatSessionDate(iso) {
  if (!iso) return ''
  const d = new Date(iso)
  if (Number.isNaN(d.getTime())) return ''
  return d.toLocaleDateString(undefined, { month: 'short', day: 'numeric' })
}

onMounted(() => {
  core.loadChatSessions()
})
</script>

<template>
  <section class="chat" :style="{ '--bg-pattern': `url(${backgroundUrl})` }">
    <header class="chat-topbar">
      <div class="topbar-actions">
        <button type="button" class="topbar-btn" title="New Chat" aria-label="New Chat" @click="startNewChat">
          <svg viewBox="0 0 24 24" width="18" height="18" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round">
            <path d="M17 3a2.83 2.83 0 0 1 4 4L7.5 20.5 2 22l1.5-5.5Z" />
            <path d="M15 5l4 4" />
          </svg>
        </button>
        <button
          type="button"
          class="topbar-btn"
          :class="{ 'is-active': showHistory }"
          title="Chat History"
          aria-label="Chat History"
          @click="toggleHistory"
        >
          <svg viewBox="0 0 24 24" width="18" height="18" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round">
            <path d="M3 12a9 9 0 1 0 3-6.7" />
            <path d="M3 4v5h5" />
            <path d="M12 7v5l3 2" />
          </svg>
        </button>
      </div>
    </header>

    <div v-if="core.chatMessages.length === 0" class="chat-hero">
      <BrandMark class="hero-mark" />
      <p class="greeting">{{ greeting }}</p>
      <p class="tagline">Sarza — your Second Brain's conversational agent for profile rules, project context, and session memory.</p>
    </div>

    <div v-else ref="scroller" class="chat-scroll" @click="onScrollerClick">
      <div
        v-for="(m, i) in core.chatMessages"
        :key="i"
        class="chat-bubble"
        :class="m.role === 'user' ? 'is-user' : 'is-agent'"
      >
        <img v-if="m.imageDataUrl && m.imageDataUrl.startsWith('data:image/')" :src="m.imageDataUrl" alt="" class="bubble-image" />

        <!-- Chain of thought: streamed live while the model thinks, kept
             behind a collapsed toggle once the turn is done. Plain text,
             no card/badge/chevron — just a muted "Thinking..." label the
             brand mark spins next to, and the reasoning body below it. -->
        <div v-if="m.role === 'agent' && (m.reasoning || m.pending)" class="thinking-block">
          <button type="button" class="thinking-toggle" @click="toggleReasoning(m)">
            <BrandMark bold class="thinking-mark" :class="{ 'is-spinning': m.pending }" />
            <span>Thinking...</span>
          </button>
          <p v-if="reasoningOpen(m) && m.reasoning" class="thinking-text">{{ m.reasoning }}</p>
        </div>

        <p v-if="m.content && m.role === 'user'" class="bubble-text">{{ m.content }}</p>
        <div v-else-if="m.content" class="bubble-markdown" v-html="renderMarkdown(m.content)" />
        <p v-if="m.stopped" class="stopped-note">Stopped by you</p>
      </div>
    </div>

    <div class="chat-input-wrap">
      <div v-if="core.chatError" class="chat-error" role="alert">
        <span>{{ chatErrorMessage }}</span>
        <button v-if="!isStartupError" type="button" class="chat-error-retry" @click="core.retryLastChatMessage()">
          Retry
        </button>
      </div>
      <div v-if="pendingImage" class="pending-attachment">
        <img :src="pendingImage.dataUrl" alt="" />
        <span>{{ pendingImage.name }}</span>
        <button type="button" class="remove-attachment" title="Remove attachment" @click="clearPendingImage">
          <svg viewBox="0 0 24 24" width="12" height="12" fill="none" stroke="currentColor" stroke-width="2.2">
            <path d="M6 6l12 12M18 6L6 18" stroke-linecap="round" />
          </svg>
        </button>
      </div>
      <form class="search-pill" @submit.prevent="submit">
        <input ref="fileInput" type="file" accept="image/*" class="file-input" @change="onFileSelected" />
        <button type="button" class="attach-btn" title="Attach image" @click="pickFile">
          <svg viewBox="0 0 24 24" width="18" height="18" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round">
            <path d="M20.5 12.5l-7.6 7.6a4.5 4.5 0 0 1-6.4-6.4l8.1-8.1a3 3 0 0 1 4.2 4.2l-7.7 7.7a1.5 1.5 0 0 1-2.1-2.1l6.4-6.4" />
          </svg>
        </button>
        <input
          v-model="input"
          type="text"
          placeholder="Message Sarza…"
          autofocus
          @keydown.enter.exact.prevent="submit"
          @keydown.up="historyUp"
          @keydown.down="historyDown"
          @input="resetHistoryCycle"
        />
        <!-- Send morphs into stop while a reply is streaming: aborting the
             fetch cancels the server-side generation (request context). -->
        <button v-if="core.sendingChat" type="button" class="submit-btn is-stop" aria-label="Stop generating" title="Stop generating" @click="core.stopChat()">
          <svg viewBox="0 0 24 24" width="18" height="18" fill="currentColor">
            <rect x="7" y="7" width="10" height="10" rx="1.5" />
          </svg>
        </button>
        <button v-else type="submit" class="submit-btn" :disabled="!input.trim()" aria-label="Send">
          <svg viewBox="0 0 24 24" width="18" height="18" fill="none" stroke="currentColor" stroke-width="2.5">
            <path d="M12 19V5M5 12l7-7 7 7" stroke-linecap="round" stroke-linejoin="round" />
          </svg>
        </button>
      </form>
    </div>

    <Transition name="history-fade">
      <div v-if="showHistory" class="history-overlay" @click="showHistory = false" />
    </Transition>
    <Transition name="history-slide">
      <aside v-if="showHistory" class="history-drawer">
        <div class="history-drawer-header">
          <h3>Chat History</h3>
          <button type="button" class="drawer-close" aria-label="Close" @click="showHistory = false">
            <svg viewBox="0 0 24 24" width="16" height="16" fill="none" stroke="currentColor" stroke-width="2">
              <path d="M6 6l12 12M18 6L6 18" stroke-linecap="round" />
            </svg>
          </button>
        </div>
        <div class="history-search">
          <svg viewBox="0 0 24 24" width="14" height="14" fill="none" stroke="currentColor" stroke-width="2">
            <circle cx="11" cy="11" r="7" />
            <path d="M21 21l-4.3-4.3" stroke-linecap="round" />
          </svg>
          <input v-model="historyQuery" type="text" placeholder="Search conversations…" />
        </div>
        <div class="history-list">
          <p v-if="core.loadingChatSessions" class="history-empty">Loading…</p>
          <p v-else-if="filteredSessions.length === 0" class="history-empty">
            {{ core.chatSessions.length === 0 ? 'No chats yet' : 'No matches' }}
          </p>
          <div
            v-for="s in filteredSessions"
            :key="s.id"
            class="history-item"
            :class="{ 'is-active': s.id === core.currentChatSessionId }"
          >
            <button
              v-if="renamingId !== s.id"
              type="button"
              class="history-item-main"
              @click="selectSession(s.id)"
            >
              <svg class="history-item-icon" viewBox="0 0 24 24" width="16" height="16" fill="none" stroke="currentColor" stroke-width="1.8">
                <path d="M21 15a2 2 0 0 1-2 2H8l-5 4V5a2 2 0 0 1 2-2h14a2 2 0 0 1 2 2z" />
              </svg>
              <span class="history-item-text">
                <span class="history-item-title">{{ s.title || 'Untitled Chat' }}</span>
                <span class="history-item-date">{{ formatSessionDate(s.updatedAt || s.createdAt) }}</span>
              </span>
            </button>
            <form v-else class="history-item-rename" @submit.prevent="confirmRename(s)">
              <input v-model="renameDraft" type="text" autofocus @blur="confirmRename(s)" @keydown.escape="renamingId = null" />
            </form>
            <div class="history-item-menu">
              <button type="button" class="kebab-btn" aria-label="Chat options" @click.stop="toggleMenu(s.id)">
                <svg viewBox="0 0 24 24" width="16" height="16" fill="currentColor">
                  <circle cx="12" cy="5" r="1.6" />
                  <circle cx="12" cy="12" r="1.6" />
                  <circle cx="12" cy="19" r="1.6" />
                </svg>
              </button>
              <div v-if="openMenuId === s.id" class="kebab-menu" @click.stop>
                <button type="button" @click="startRename(s)">Rename</button>
                <button type="button" class="is-danger" @click="confirmDelete(s)">Delete</button>
              </div>
            </div>
          </div>
        </div>
      </aside>
    </Transition>
  </section>
</template>

<style scoped>
/* Chat is the app's home/default screen — HomeView.vue no longer exists
   as a separate component, its hero (mark + greeting) and search-pill
   composer styling live here now. */
.chat {
  position: relative;
  display: flex;
  flex-direction: column;
  height: 100%;
  overflow: hidden;
}

/* Slim top bar, right-aligned icon actions. Kept clear of App.vue's fixed
   theme-toggle pill (top:16px right:16px, ~60px wide) with right padding
   rather than fighting it for the same corner. */
.chat-topbar {
  display: flex;
  justify-content: flex-end;
  align-items: center;
  flex-shrink: 0;
  padding: 12px 100px 0 20px;
}

.topbar-actions {
  display: flex;
  gap: 6px;
}

.topbar-btn {
  display: flex;
  align-items: center;
  justify-content: center;
  width: 34px;
  height: 34px;
  border: none;
  border-radius: var(--radius-sm);
  background: transparent;
  color: var(--ink-muted);
  cursor: pointer;
  transition: background-color 150ms var(--ease-out-expo), color 150ms var(--ease-out-expo);
}

.topbar-btn:hover {
  background: var(--surface-hover);
  color: var(--ink);
}

.topbar-btn.is-active {
  background: var(--accent-soft);
  color: var(--accent);
}

.chat-hero {
  position: relative;
  flex: 1;
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  gap: 12px;
  text-align: center;
}

.chat-hero::before {
  content: '';
  position: absolute;
  inset: 0;
  background-image: var(--bg-pattern);
  background-repeat: no-repeat;
  background-position: center;
  background-size: min(700px, 90%);
  opacity: 0.05;
  pointer-events: none;
}

.hero-mark {
  position: relative;
  width: 48px;
  height: 48px;
  margin-bottom: 4px;
}

.greeting {
  position: relative;
  margin: 0;
  font-size: 12px;
  font-weight: 600;
  color: var(--ink-faint);
  text-transform: uppercase;
  letter-spacing: 0.04em;
}

.tagline {
  position: relative;
  margin: 0;
  max-width: 420px;
  font-size: 13px;
  color: var(--ink-muted);
}

.chat-scroll {
  flex: 1;
  min-height: 0;
  overflow-y: auto;
  display: flex;
  flex-direction: column;
  gap: 10px;
  padding: 20px 0 12px;
  /* Full-width, fluid message list: fluid up to a comfortable reading cap
     rather than a fixed narrow column with big side gutters. */
  max-width: min(1100px, 94%);
  width: 100%;
  margin: 0 auto;
  /* Scroll still works, the scrollbar itself is just not drawn. */
  scrollbar-width: none;
}

.chat-scroll::-webkit-scrollbar {
  display: none;
}

.chat-bubble {
  max-width: 100%;
  padding: 10px 14px;
  border-radius: var(--radius-lg);
  font-size: 14px;
  line-height: 1.5;
}

.chat-bubble.is-user {
  align-self: flex-end;
  max-width: 75%;
  background: var(--accent);
  color: var(--accent-ink);
}

.chat-bubble.is-agent {
  align-self: stretch;
  width: 100%;
  background: var(--surface);
  border: 1px solid var(--border);
  color: var(--ink);
}

/* Chain-of-thought block inside an agent bubble: brand mark spins while
   the model is thinking, "Thinking..." label — a quiet pill (subtle fill
   + hairline border), not a heavy card/badge/chevron, but enough presence
   to read as a real state on both themes instead of bare text floating
   with nothing to anchor it. Collapsed by default once the turn is done. */
.thinking-block {
  display: flex;
  flex-direction: column;
  gap: 6px;
  margin-bottom: 6px;
}

.thinking-toggle {
  display: flex;
  align-items: center;
  gap: 7px;
  width: fit-content;
  padding: 5px 10px 5px 8px;
  border: 1px solid var(--border);
  border-radius: 999px;
  background: var(--surface-hover);
  color: var(--ink-muted);
  font-size: 12px;
  font-weight: 600;
  cursor: pointer;
}

.thinking-toggle:hover {
  color: var(--ink);
  border-color: var(--accent);
}

.thinking-mark {
  width: 16px;
  height: 16px;
}

.thinking-mark.is-spinning {
  animation: thinking-spin 1.6s linear infinite;
}

@keyframes thinking-spin {
  from {
    transform: rotate(0deg);
  }
  to {
    transform: rotate(360deg);
  }
}

@media (prefers-reduced-motion: reduce) {
  .thinking-mark.is-spinning {
    animation: none;
    opacity: 0.7;
  }
}

.thinking-text {
  margin: 0;
  padding: 8px 10px;
  border-left: 2px solid var(--border);
  color: var(--ink-faint);
  font-size: 12.5px;
  line-height: 1.55;
  white-space: pre-wrap;
}

.stopped-note {
  margin: 6px 0 0;
  color: var(--ink-faint);
  font-size: 11px;
  font-style: italic;
}

.bubble-text {
  margin: 0;
  white-space: pre-wrap;
}

.bubble-image {
  max-width: 100%;
  border-radius: var(--radius-sm);
  margin-bottom: 6px;
}

/* Markdown rendering for agent replies (see src/lib/markdown.js). */
.bubble-markdown {
  font-size: 14px;
  line-height: 1.6;
}

.bubble-markdown :deep(p) {
  margin: 0 0 10px;
}

.bubble-markdown :deep(p:last-child) {
  margin-bottom: 0;
}

.bubble-markdown :deep(h1),
.bubble-markdown :deep(h2),
.bubble-markdown :deep(h3),
.bubble-markdown :deep(h4) {
  margin: 14px 0 8px;
  font-weight: 600;
  color: var(--ink);
}

.bubble-markdown :deep(h1:first-child),
.bubble-markdown :deep(h2:first-child),
.bubble-markdown :deep(h3:first-child) {
  margin-top: 0;
}

.bubble-markdown :deep(ul),
.bubble-markdown :deep(ol) {
  margin: 0 0 10px;
  padding-left: 22px;
}

.bubble-markdown :deep(li) {
  margin: 3px 0;
}

.bubble-markdown :deep(a) {
  color: var(--accent);
  text-decoration: underline;
}

.bubble-markdown :deep(blockquote) {
  margin: 0 0 10px;
  padding: 2px 12px;
  border-left: 2px solid var(--border);
  color: var(--ink-muted);
}

.bubble-markdown :deep(hr) {
  border: none;
  border-top: 1px solid var(--border);
  margin: 12px 0;
}

.bubble-markdown :deep(code) {
  padding: 2px 5px;
  border-radius: 4px;
  background: var(--surface-hover);
  font-size: 12.5px;
  font-family: ui-monospace, 'SF Mono', Menlo, Consolas, monospace;
}

.bubble-markdown :deep(img) {
  max-width: 100%;
  border-radius: var(--radius-sm);
  margin: 6px 0;
}

.bubble-markdown :deep(table) {
  width: 100%;
  border-collapse: collapse;
  margin: 4px 0 12px;
  font-size: 13px;
}

.bubble-markdown :deep(th),
.bubble-markdown :deep(td) {
  border: 1px solid var(--border);
  padding: 6px 10px;
  text-align: left;
}

.bubble-markdown :deep(th) {
  background: var(--surface-hover);
  font-weight: 600;
}

.bubble-markdown :deep(.code-block-wrapper) {
  margin: 4px 0 12px;
  border-radius: var(--radius-sm);
  overflow: hidden;
  border: 1px solid var(--border);
  background: #0d1117;
}

.bubble-markdown :deep(.code-block-wrapper pre) {
  margin: 0;
  padding: 12px 14px;
  overflow-x: auto;
}

.bubble-markdown :deep(.code-block-wrapper code) {
  background: transparent;
  padding: 0;
  font-size: 12.5px;
}

.bubble-markdown :deep(.code-block-header) {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 6px 12px;
  background: oklch(0 0 0 / 0.25);
  border-bottom: 1px solid oklch(1 0 0 / 0.08);
}

.bubble-markdown :deep(.code-lang) {
  font-size: 11px;
  color: oklch(0.8 0.01 316);
  text-transform: uppercase;
  letter-spacing: 0.03em;
}

.bubble-markdown :deep(.copy-code-btn) {
  border: none;
  background: transparent;
  color: oklch(0.8 0.01 316);
  font-size: 11px;
  cursor: pointer;
  padding: 2px 6px;
  border-radius: 4px;
}

.bubble-markdown :deep(.copy-code-btn:hover) {
  background: oklch(1 0 0 / 0.1);
  color: #fff;
}

.bubble-markdown :deep(.copy-code-btn.is-copied) {
  color: #6ee7b7;
}

.bubble-markdown :deep(.mermaid-block) {
  margin: 4px 0 12px;
  text-align: center;
}

.bubble-markdown :deep(.mermaid-block svg) {
  max-width: 100%;
}

.bubble-markdown :deep(.mermaid-fallback) {
  margin: 0;
  padding: 12px 14px;
  border-radius: var(--radius-sm);
  border: 1px solid var(--border);
  background: var(--surface-hover);
  overflow-x: auto;
  text-align: left;
  font-size: 12.5px;
}

.chat-input-wrap {
  position: relative;
  flex-shrink: 0;
  width: 100%;
  max-width: 560px;
  margin: 0 auto;
  padding: 0 24px 32px;
}

.chat-error {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 10px;
  margin: 0 0 8px;
  padding: 8px 12px;
  border-radius: var(--radius-sm);
  background: var(--danger-soft);
  color: var(--danger);
  font-size: 12px;
}

.chat-error-retry {
  flex-shrink: 0;
  padding: 3px 10px;
  border: 1px solid currentColor;
  border-radius: var(--radius-sm);
  background: transparent;
  color: inherit;
  font-size: 11px;
  font-weight: 700;
  cursor: pointer;
}

.chat-error-retry:hover {
  background: var(--danger);
  color: var(--surface);
}

.pending-attachment {
  display: flex;
  align-items: center;
  gap: 8px;
  margin-bottom: 8px;
  padding: 6px 10px;
  border-radius: var(--radius-sm);
  background: var(--surface);
  border: 1px solid var(--border);
  width: fit-content;
  font-size: 12px;
  color: var(--ink-muted);
}

.pending-attachment img {
  width: 28px;
  height: 28px;
  border-radius: 4px;
  object-fit: cover;
}

.remove-attachment {
  display: flex;
  align-items: center;
  justify-content: center;
  width: 18px;
  height: 18px;
  border: none;
  border-radius: 50%;
  background: transparent;
  color: var(--ink-faint);
  cursor: pointer;
}

.remove-attachment:hover {
  background: var(--surface-hover);
  color: var(--ink);
}

/* Identical to HomeView.vue's .search-pill/.submit-btn, plus one extra
   attach-btn slotted in before the text input. */
.search-pill {
  display: flex;
  align-items: center;
  gap: 8px;
  width: 100%;
  padding: 14px 8px 14px 12px;
  border-radius: 24px;
  background: var(--surface);
  border: 1px solid var(--border);
  box-shadow: 0 1px 2px oklch(0 0 0 / 0.03), 0 4px 12px oklch(0 0 0 / 0.04);
  transition: border-color 150ms var(--ease-out-expo);
}

.search-pill:focus-within {
  border-color: var(--accent);
}

.file-input {
  display: none;
}

.attach-btn {
  display: flex;
  align-items: center;
  justify-content: center;
  width: 36px;
  height: 36px;
  flex-shrink: 0;
  border: none;
  border-radius: 50%;
  background: transparent;
  color: var(--ink-muted);
  cursor: pointer;
  transition: background-color 150ms var(--ease-out-expo), color 150ms var(--ease-out-expo);
}

.attach-btn:hover {
  background: var(--surface-hover);
  color: var(--ink);
}

.search-pill input[type='text'] {
  flex: 1;
  border: none;
  background: transparent;
  outline: none;
  font: inherit;
  font-size: 15px;
  color: var(--ink);
}

.search-pill input[type='text']::placeholder {
  color: var(--ink-faint);
}

.submit-btn {
  display: flex;
  align-items: center;
  justify-content: center;
  width: 36px;
  height: 36px;
  flex-shrink: 0;
  border: none;
  border-radius: 50%;
  background: var(--ink);
  color: var(--bg);
  cursor: pointer;
  transition: background-color 150ms var(--ease-out-expo), opacity 150ms var(--ease-out-expo);
}

.submit-btn:hover:not(:disabled) {
  background: var(--accent);
}

.submit-btn:disabled {
  opacity: 0.35;
  cursor: default;
}

.submit-btn:focus-visible {
  outline: 2px solid var(--accent);
  outline-offset: 2px;
}

.submit-btn.is-stop {
  background: var(--danger, #b3261e);
  color: #fff;
}

/* History drawer */
.history-overlay {
  position: fixed;
  inset: 0;
  background: oklch(0 0 0 / 0.25);
  z-index: 40;
}

.history-fade-enter-active,
.history-fade-leave-active {
  transition: opacity 150ms var(--ease-out-expo);
}

.history-fade-enter-from,
.history-fade-leave-to {
  opacity: 0;
}

.history-drawer {
  position: fixed;
  top: 0;
  right: 0;
  bottom: 0;
  width: min(360px, 92vw);
  display: flex;
  flex-direction: column;
  background: var(--surface);
  border-left: 1px solid var(--border);
  box-shadow: -8px 0 24px oklch(0 0 0 / 0.08);
  z-index: 41;
}

.history-slide-enter-active,
.history-slide-leave-active {
  transition: transform 200ms var(--ease-out-expo);
}

.history-slide-enter-from,
.history-slide-leave-to {
  transform: translateX(100%);
}

.history-drawer-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 16px 16px 12px;
  border-bottom: 1px solid var(--border);
}

.history-drawer-header h3 {
  margin: 0;
  font-size: 14px;
  font-weight: 600;
  color: var(--ink);
}

.drawer-close {
  display: flex;
  align-items: center;
  justify-content: center;
  width: 28px;
  height: 28px;
  border: none;
  border-radius: 50%;
  background: transparent;
  color: var(--ink-faint);
  cursor: pointer;
}

.drawer-close:hover {
  background: var(--surface-hover);
  color: var(--ink);
}

.history-search {
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 10px 16px;
  border-bottom: 1px solid var(--border);
  color: var(--ink-faint);
}

.history-search input {
  flex: 1;
  border: none;
  background: transparent;
  outline: none;
  font: inherit;
  font-size: 13px;
  color: var(--ink);
}

.history-search input::placeholder {
  color: var(--ink-faint);
}

.history-list {
  flex: 1;
  overflow-y: auto;
  padding: 6px 8px;
}

.history-empty {
  padding: 24px 12px;
  text-align: center;
  color: var(--ink-faint);
  font-size: 13px;
}

.history-item {
  position: relative;
  display: flex;
  align-items: center;
  gap: 4px;
  border-radius: var(--radius-sm);
}

.history-item:hover {
  background: var(--surface-hover);
}

.history-item.is-active {
  background: var(--accent-soft);
}

.history-item-main {
  flex: 1;
  display: flex;
  align-items: center;
  gap: 10px;
  min-width: 0;
  padding: 9px 8px;
  border: none;
  background: transparent;
  cursor: pointer;
  text-align: left;
}

.history-item-icon {
  flex-shrink: 0;
  color: var(--ink-faint);
}

.history-item-text {
  display: flex;
  flex-direction: column;
  min-width: 0;
  gap: 1px;
}

.history-item-title {
  font-size: 13px;
  color: var(--ink);
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}

.history-item-date {
  font-size: 11px;
  color: var(--ink-faint);
}

.history-item-rename {
  flex: 1;
  padding: 6px 8px;
}

.history-item-rename input {
  width: 100%;
  border: 1px solid var(--accent);
  border-radius: 4px;
  background: var(--bg);
  color: var(--ink);
  font: inherit;
  font-size: 13px;
  padding: 4px 6px;
}

.history-item-menu {
  position: relative;
  flex-shrink: 0;
}

.kebab-btn {
  display: flex;
  align-items: center;
  justify-content: center;
  width: 26px;
  height: 26px;
  margin-right: 6px;
  border: none;
  border-radius: 50%;
  background: transparent;
  color: var(--ink-faint);
  cursor: pointer;
}

.kebab-btn:hover {
  background: var(--surface-hover);
  color: var(--ink);
}

.kebab-menu {
  position: absolute;
  top: 28px;
  right: 6px;
  display: flex;
  flex-direction: column;
  min-width: 110px;
  padding: 4px;
  border-radius: var(--radius-sm);
  background: var(--surface);
  border: 1px solid var(--border);
  box-shadow: 0 4px 16px oklch(0 0 0 / 0.12);
  z-index: 42;
}

.kebab-menu button {
  padding: 7px 10px;
  border: none;
  border-radius: 4px;
  background: transparent;
  color: var(--ink);
  font-size: 13px;
  text-align: left;
  cursor: pointer;
}

.kebab-menu button:hover {
  background: var(--surface-hover);
}

.kebab-menu button.is-danger {
  color: var(--danger);
}
</style>
