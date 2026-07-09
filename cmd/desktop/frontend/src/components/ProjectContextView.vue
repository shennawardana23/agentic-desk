<script setup>
import { computed, onMounted, ref } from 'vue'
import { useCoreStore } from '../stores/core'

const core = useCoreStore()
const projectPath = ref('')
const summaryDraft = ref('')
const savedSummary = ref('')
const saving = ref(false)
const justSaved = ref(false)

// Save is disabled with no pending change and briefly confirms success
// after a real save — mirrors the "Copied" pattern already used for the
// catalog/chat copy buttons, so a save that silently no-ops or silently
// succeeds isn't left unconfirmed (the same completeness gap those
// buttons had before they got their own state).
const dirty = computed(() => summaryDraft.value !== savedSummary.value)

async function load() {
  if (!projectPath.value.trim()) return
  await core.loadProjectContext(projectPath.value.trim())
  summaryDraft.value = core.projectContext?.summary ?? ''
  savedSummary.value = summaryDraft.value
  justSaved.value = false
}

async function save() {
  if (!projectPath.value.trim() || !dirty.value) return
  saving.value = true
  try {
    await core.saveProjectContext(projectPath.value.trim(), summaryDraft.value)
    if (!core.projectContextError) {
      savedSummary.value = summaryDraft.value
      justSaved.value = true
      setTimeout(() => (justSaved.value = false), 1500)
    }
  } finally {
    saving.value = false
  }
}

onMounted(() => {
  projectPath.value = '/Users/msw/Desktop/Development/Startup_Companies/Arcipelago_International/repo-personal/agentic-desk'
  load()
})
</script>

<template>
  <section class="panel">
    <header class="board-header">
      <svg class="board-header__bg-mark" viewBox="0 0 131 131" fill="none" xmlns="http://www.w3.org/2000/svg">
        <path d="M53.7421 59.6211L29.8369 60.0105L2 32.1736L25.9052 31.7842L53.7421 59.6211Z" stroke="currentColor" stroke-miterlimit="10"/>
        <path d="M59.6211 53.7421L60.0105 29.8183L32.1736 2L31.7842 25.9052L59.6211 53.7421Z" stroke="currentColor" stroke-miterlimit="10"/>
        <path d="M77.2579 71.3789L101.163 70.9895L129 98.8264L105.095 99.2158L77.2579 71.3789Z" stroke="currentColor" stroke-miterlimit="10"/>
        <path d="M71.3789 77.2579L70.9895 101.182L98.8264 129L99.2158 105.095L71.3789 77.2579Z" stroke="currentColor" stroke-miterlimit="10"/>
        <path d="M59.6211 77.2579L60.0105 101.163L32.1736 129L31.7842 105.095L59.6211 77.2579Z" stroke="currentColor" stroke-miterlimit="10"/>
        <path d="M53.7421 71.3789L29.8369 70.9895L2 98.8264L25.9052 99.2158L53.7421 71.3789Z" stroke="currentColor" stroke-miterlimit="10"/>
        <path d="M71.3789 53.7421L70.9895 29.8369L98.8264 2L99.2158 25.9052L71.3789 53.7421Z" stroke="currentColor" stroke-miterlimit="10"/>
        <path d="M77.2579 59.6211L101.182 60.0105L129 32.1736L105.095 31.7842L77.2579 59.6211Z" stroke="currentColor" stroke-miterlimit="10"/>
        <path d="M69.9324 69.9324H61.0676V61.0676H69.9324V69.9324Z" stroke="currentColor" stroke-miterlimit="10"/>
      </svg>
      <svg class="board-header__bg-mark-sm" viewBox="0 0 131 131" fill="none" xmlns="http://www.w3.org/2000/svg">
        <path d="M53.7421 59.6211L29.8369 60.0105L2 32.1736L25.9052 31.7842L53.7421 59.6211Z" stroke="currentColor" stroke-miterlimit="10"/>
        <path d="M59.6211 53.7421L60.0105 29.8183L32.1736 2L31.7842 25.9052L59.6211 53.7421Z" stroke="currentColor" stroke-miterlimit="10"/>
        <path d="M77.2579 71.3789L101.163 70.9895L129 98.8264L105.095 99.2158L77.2579 71.3789Z" stroke="currentColor" stroke-miterlimit="10"/>
        <path d="M71.3789 77.2579L70.9895 101.182L98.8264 129L99.2158 105.095L71.3789 77.2579Z" stroke="currentColor" stroke-miterlimit="10"/>
        <path d="M59.6211 77.2579L60.0105 101.163L32.1736 129L31.7842 105.095L59.6211 77.2579Z" stroke="currentColor" stroke-miterlimit="10"/>
        <path d="M53.7421 71.3789L29.8369 70.9895L2 98.8264L25.9052 99.2158L53.7421 71.3789Z" stroke="currentColor" stroke-miterlimit="10"/>
        <path d="M71.3789 53.7421L70.9895 29.8369L98.8264 2L99.2158 25.9052L71.3789 53.7421Z" stroke="currentColor" stroke-miterlimit="10"/>
        <path d="M77.2579 59.6211L101.182 60.0105L129 32.1736L105.095 31.7842L77.2579 59.6211Z" stroke="currentColor" stroke-miterlimit="10"/>
        <path d="M69.9324 69.9324H61.0676V61.0676H69.9324V69.9324Z" stroke="currentColor" stroke-miterlimit="10"/>
      </svg>
      <svg class="board-header__bg-mark-md" viewBox="0 0 131 131" fill="none" xmlns="http://www.w3.org/2000/svg">
        <path d="M53.7421 59.6211L29.8369 60.0105L2 32.1736L25.9052 31.7842L53.7421 59.6211Z" stroke="currentColor" stroke-miterlimit="10"/>
        <path d="M59.6211 53.7421L60.0105 29.8183L32.1736 2L31.7842 25.9052L59.6211 53.7421Z" stroke="currentColor" stroke-miterlimit="10"/>
        <path d="M77.2579 71.3789L101.163 70.9895L129 98.8264L105.095 99.2158L77.2579 71.3789Z" stroke="currentColor" stroke-miterlimit="10"/>
        <path d="M71.3789 77.2579L70.9895 101.182L98.8264 129L99.2158 105.095L71.3789 77.2579Z" stroke="currentColor" stroke-miterlimit="10"/>
        <path d="M59.6211 77.2579L60.0105 101.163L32.1736 129L31.7842 105.095L59.6211 77.2579Z" stroke="currentColor" stroke-miterlimit="10"/>
        <path d="M53.7421 71.3789L29.8369 70.9895L2 98.8264L25.9052 99.2158L53.7421 71.3789Z" stroke="currentColor" stroke-miterlimit="10"/>
        <path d="M71.3789 53.7421L70.9895 29.8369L98.8264 2L99.2158 25.9052L71.3789 53.7421Z" stroke="currentColor" stroke-miterlimit="10"/>
        <path d="M77.2579 59.6211L101.182 60.0105L129 32.1736L105.095 31.7842L77.2579 59.6211Z" stroke="currentColor" stroke-miterlimit="10"/>
        <path d="M69.9324 69.9324H61.0676V61.0676H69.9324V69.9324Z" stroke="currentColor" stroke-miterlimit="10"/>
      </svg>
      <div class="board-header__content">
        <div>
          <h1>Project Context</h1>
          <p class="panel-subtitle">The running summary Second Brain keeps for one project.</p>
        </div>
      </div>
    </header>

    <form class="path-row" @submit.prevent="load">
      <input v-model="projectPath" type="text" placeholder="/path/to/project" />
      <button type="submit" class="btn-secondary">Load</button>
    </form>

    <p v-if="core.projectContextError" class="error" role="alert">{{ core.projectContextError }}</p>

    <div v-if="core.loadingProjectContext" class="skeleton-block" aria-hidden="true" />
    <template v-else>
      <textarea
        v-model="summaryDraft"
        rows="10"
        placeholder="No context saved for this project yet — write a summary and save it."
      />
      <div class="actions">
        <span v-if="dirty" class="dirty-note">Unsaved changes</span>
        <button type="button" class="btn-primary" :disabled="saving || !dirty" @click="save">
          {{ saving ? 'Saving…' : justSaved ? 'Saved' : 'Save' }}
        </button>
      </div>
    </template>
  </section>
</template>

<style scoped>
.board-header {
  position: relative;
  z-index: 1;
  display: flex;
  flex-direction: column;
  margin-bottom: 16px;
  border-radius: var(--radius-lg);
  background: color-mix(in srgb, #0284c7 6%, var(--surface));
  border: 1px solid color-mix(in srgb, #0284c7 15%, var(--border));
  overflow: hidden;
  padding: 22px 24px 16px;
  min-height: 104px;
}
.board-header__bg-mark {
  position: absolute;
  top: -18px; right: -18px;
  width: 130px; height: 130px;
  color: #0284c7; opacity: 0.10;
  pointer-events: none;
}
.board-header__bg-mark-sm {
  position: absolute; z-index: 0;
  top: 6px; right: 132px;
  width: 36px; height: 36px;
  color: #0284c7; opacity: 0.3;
  pointer-events: none;
  transform: none;
}
.board-header__bg-mark-md {
  position: absolute; z-index: 0;
  top: 38px; right: 210px;
  width: 60px; height: 60px;
  color: #0284c7; opacity: 0.18;
  pointer-events: none;
  transform: none;
}
.board-header__content {
  position: relative;
  z-index: 1;
  display: flex;
  align-items: flex-end;
  justify-content: space-between;
  gap: 16px;
}
.board-header__content > div:first-child {
  min-width: 0;
  flex: 1;
}
.board-header h1 {
  margin: 0 0 3px;
  font-size: 22px;
}
.panel {
  display: flex;
  flex-direction: column;
  gap: 16px;
}

.panel-header h2 {
  margin: 0 0 4px;
  font-size: 18px;
  font-weight: 600;
  letter-spacing: -0.01em;
}

.panel-subtitle {
  margin: 0;
  font-size: 13px;
  color: var(--ink-muted);
}

.path-row {
  display: flex;
  gap: 8px;
}

.path-row input {
  flex: 1;
  padding: 8px 10px;
  border: 1px solid var(--border);
  border-radius: var(--radius-sm);
  font-size: 13px;
  color: var(--ink);
  background: var(--bg);
}

.path-row input:focus-visible {
  outline: 2px solid var(--accent);
  outline-offset: 1px;
}

textarea {
  width: 100%;
  padding: 12px;
  border: 1px solid var(--border);
  border-radius: var(--radius-md);
  font: inherit;
  font-size: 13px;
  color: var(--ink);
  background: var(--bg);
  resize: vertical;
}

textarea:focus-visible {
  outline: 2px solid var(--accent);
  outline-offset: 1px;
}

.actions {
  display: flex;
  align-items: center;
  justify-content: flex-end;
  gap: 10px;
}

.dirty-note {
  font-size: 12px;
  color: var(--ink-faint);
}

.btn-primary,
.btn-secondary {
  padding: 8px 14px;
  border: none;
  border-radius: var(--radius-sm);
  font-size: 13px;
  font-weight: 600;
  cursor: pointer;
  transition: background-color 150ms var(--ease-out-expo);
}

.btn-primary {
  background: var(--accent);
  color: var(--accent-ink);
}

.btn-primary:hover:not(:disabled) {
  background: var(--accent-hover);
}

.btn-primary:disabled {
  opacity: 0.6;
  cursor: default;
}

.btn-secondary {
  background: var(--surface);
  border: 1px solid var(--border);
  color: var(--ink);
}

.btn-secondary:hover {
  background: var(--surface-hover);
}

.btn-primary:focus-visible,
.btn-secondary:focus-visible {
  outline: 2px solid var(--accent);
  outline-offset: 1px;
}

.error {
  margin: 0;
  padding: 10px 12px;
  border-radius: var(--radius-sm);
  background: var(--danger-soft);
  color: var(--danger);
  font-size: 13px;
}

.skeleton-block {
  height: 200px;
  border-radius: var(--radius-md);
  background: linear-gradient(90deg, var(--surface) 25%, var(--surface-hover) 50%, var(--surface) 75%);
  background-size: 200% 100%;
  animation: shimmer 1.4s ease-in-out infinite;
}

@keyframes shimmer {
  0% { background-position: 200% 0; }
  100% { background-position: -200% 0; }
}
</style>
