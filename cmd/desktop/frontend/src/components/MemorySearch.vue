<script setup>
import { ref } from 'vue'
import { useCoreStore } from '../stores/core'

const core = useCoreStore()
const query = ref('')

function submit() {
  if (query.value.trim() !== '') core.searchMemory(query.value.trim())
}
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
          <h1>Memory Search</h1>
          <p class="panel-subtitle">Semantic search over past session memory.</p>
        </div>
      </div>
    </header>

    <form class="search-pill" @submit.prevent="submit">
      <input v-model="query" type="text" placeholder="Search session memory…" />
      <button type="submit" class="submit-btn" :disabled="core.searchingMemory || !query.trim()" aria-label="Search">
        <svg viewBox="0 0 24 24" width="16" height="16" fill="none" stroke="currentColor" stroke-width="2.5">
          <path d="M12 19V5M5 12l7-7 7 7" stroke-linecap="round" stroke-linejoin="round" />
        </svg>
      </button>
    </form>

    <p v-if="core.memoryError" class="error" role="alert">{{ core.memoryError }}</p>

    <div v-else-if="core.searchingMemory" class="skeleton-list" aria-hidden="true">
      <div class="skeleton-row" v-for="n in 3" :key="n" />
    </div>

    <p v-else-if="core.memorySearched && core.memoryResults.length === 0" class="empty-state">
      No memory entries matched "{{ query }}".
    </p>

    <ul v-else-if="core.memoryResults.length > 0" class="result-list">
      <li v-for="entry in core.memoryResults" :key="entry.id" class="result-row">
        <div class="result-meta">
          <span class="badge" :class="`badge--${entry.role}`">{{ entry.role }}</span>
          <span class="result-session">{{ entry.sessionId }}</span>
        </div>
        <p class="result-content">{{ entry.content }}</p>
      </li>
    </ul>

    <p v-else class="empty-state">Search past session memory by meaning, not just keywords.</p>
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
  background: color-mix(in srgb, #059669 6%, var(--surface));
  border: 1px solid color-mix(in srgb, #059669 15%, var(--border));
  overflow: hidden;
  padding: 22px 24px 16px;
  min-height: 104px;
}
.board-header__bg-mark {
  position: absolute;
  top: -18px; right: -18px;
  width: 130px; height: 130px;
  color: #059669; opacity: 0.10;
  pointer-events: none;
}
.board-header__bg-mark-sm {
  position: absolute; z-index: 0;
  top: 6px; right: 132px;
  width: 36px; height: 36px;
  color: #059669; opacity: 0.3;
  pointer-events: none;
  transform: none;
}
.board-header__bg-mark-md {
  position: absolute; z-index: 0;
  top: 38px; right: 210px;
  width: 60px; height: 60px;
  color: #059669; opacity: 0.18;
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

.search-pill {
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 6px 6px 6px 16px;
  border-radius: 999px;
  background: var(--surface);
  border: 1px solid var(--border);
  transition: border-color 150ms var(--ease-out-expo);
}

.search-pill:focus-within {
  border-color: var(--accent);
}

.search-pill input {
  flex: 1;
  border: none;
  background: transparent;
  outline: none;
  font: inherit;
  font-size: 14px;
  color: var(--ink);
}

.search-pill input::placeholder {
  color: var(--ink-faint);
}

.submit-btn {
  display: flex;
  align-items: center;
  justify-content: center;
  width: 32px;
  height: 32px;
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

.error {
  margin: 0;
  padding: 10px 12px;
  border-radius: var(--radius-sm);
  background: var(--danger-soft);
  color: var(--danger);
  font-size: 13px;
}

.empty-state {
  margin: 0;
  padding: 24px;
  border: 1px dashed var(--border);
  border-radius: var(--radius-md);
  color: var(--ink-muted);
  font-size: 13px;
  text-align: center;
}

.result-list {
  display: flex;
  flex-direction: column;
  gap: 8px;
  margin: 0;
  padding: 0;
  list-style: none;
}

.result-row {
  padding: 12px 14px;
  border: 1px solid var(--border);
  border-radius: var(--radius-md);
  background: var(--surface);
}

.result-meta {
  display: flex;
  align-items: center;
  gap: 8px;
  margin-bottom: 6px;
}

.badge {
  padding: 2px 8px;
  border-radius: 999px;
  font-size: 11px;
  font-weight: 600;
  text-transform: capitalize;
}

.badge--user {
  background: var(--accent-soft);
  color: var(--accent);
}

.badge--agent {
  background: var(--accent-ai-soft);
  color: var(--accent-ai);
}

.result-session {
  font-size: 12px;
  color: var(--ink-faint);
}

.result-content {
  margin: 0;
  color: var(--ink-muted);
  font-size: 13px;
  line-height: 1.55;
}

.skeleton-list {
  display: flex;
  flex-direction: column;
  gap: 8px;
}

.skeleton-row {
  height: 56px;
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
