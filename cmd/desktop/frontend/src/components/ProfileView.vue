<script setup>
import { computed, onMounted } from 'vue'
import { useCoreStore } from '../stores/core'

const core = useCoreStore()
onMounted(() => core.loadProfile())

// Group by source file (e.g. CLAUDE.md, RULES.md) so rules read as an
// outline of each imported file rather than a flat undifferentiated list.
const groupedRules = computed(() => {
  const groups = new Map()
  for (const rule of core.profileRules) {
    if (!groups.has(rule.sourceFile)) groups.set(rule.sourceFile, [])
    groups.get(rule.sourceFile).push(rule)
  }
  return [...groups.entries()].map(([sourceFile, rules]) => ({ sourceFile, rules }))
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
          <h1>Profile Rules</h1>
          <p class="panel-subtitle">Coding-principles rules imported from your CLAUDE.md-style files.</p>
        </div>
      </div>
    </header>

    <p v-if="core.profileError" class="error" role="alert">{{ core.profileError }}</p>

    <div v-else-if="core.loadingProfile" class="skeleton-list" aria-hidden="true">
      <div class="skeleton-row" v-for="n in 4" :key="n" />
    </div>

    <p v-else-if="core.profileRules.length === 0" class="empty-state">
      No profile rules imported yet. Run the importer against your CLAUDE.md/RULES.md files to seed this list.
    </p>

    <div v-else class="source-groups">
      <div v-for="group in groupedRules" :key="group.sourceFile" class="source-group">
        <h3 class="source-heading">{{ group.sourceFile }}</h3>
        <ul class="rule-list">
          <li v-for="rule in group.rules" :key="`${rule.sourceFile}/${rule.heading}`" class="rule-row">
            <div class="rule-meta">
              <span class="rule-heading">{{ rule.heading }}</span>
              <span v-if="rule.overridden" class="badge">Overridden</span>
            </div>
            <p class="rule-content">{{ rule.content }}</p>
          </li>
        </ul>
      </div>
    </div>
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
  background: color-mix(in srgb, #d97706 6%, var(--surface));
  border: 1px solid color-mix(in srgb, #d97706 15%, var(--border));
  overflow: hidden;
  padding: 22px 24px 16px;
  min-height: 104px;
}
.board-header__bg-mark {
  position: absolute;
  top: 8px; right: -8px;
  width: 130px; height: 130px;
  color: #d97706; opacity: 0.10;
  pointer-events: none;
}
.board-header__bg-mark-sm {
  position: absolute; z-index: 0;
  top: 4.5px; right: 122px;
  width: 36px; height: 36px;
  color: #d97706; opacity: 0.3;
  pointer-events: none;
  transform: none;
}
.board-header__bg-mark-md {
  position: absolute; z-index: 0;
  top: 32px; right: 150px;
  width: 60px; height: 60px;
  color: #d97706; opacity: 0.18;
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

.source-groups {
  display: flex;
  flex-direction: column;
  gap: 22px;
}

.source-group {
  display: flex;
  flex-direction: column;
  gap: 8px;
}

.source-heading {
  margin: 0;
  padding-bottom: 6px;
  border-bottom: 1px solid var(--border);
  font-size: 11px;
  font-weight: 700;
  letter-spacing: 0.04em;
  text-transform: uppercase;
  color: var(--ink-faint);
}

.rule-list {
  display: flex;
  flex-direction: column;
  gap: 8px;
  margin: 0;
  padding: 0;
  list-style: none;
}

.rule-row {
  padding: 14px 16px;
  border: 1px solid var(--border);
  border-radius: var(--radius-md);
  background: var(--surface);
}

.rule-meta {
  display: flex;
  align-items: center;
  gap: 8px;
  margin-bottom: 8px;
}

.rule-heading {
  font-weight: 600;
  font-size: 14px;
  color: var(--ink);
}

.badge {
  padding: 2px 8px;
  border-radius: 999px;
  background: var(--accent-soft);
  color: var(--accent);
  font-size: 11px;
  font-weight: 600;
}

.rule-content {
  margin: 0;
  padding: 10px 12px;
  border-radius: var(--radius-sm);
  background: var(--surface-hover);
  color: var(--ink-muted);
  font-size: 13px;
  line-height: 1.55;
  white-space: pre-wrap;
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
