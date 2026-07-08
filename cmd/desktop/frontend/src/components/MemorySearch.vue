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
    <header class="panel-header">
      <h2>Memory Search</h2>
      <p class="panel-subtitle">Semantic search over past session memory.</p>
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
.panel {
  display: flex;
  flex-direction: column;
  gap: 16px;
  max-width: 720px;
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
