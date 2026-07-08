<script setup>
import BrandMark from './icons/BrandMark.vue'
import SarzaMark from './icons/SarzaMark.vue'

defineProps({
  active: { type: String, required: true },
  expanded: { type: Boolean, required: true },
})
const emit = defineEmits(['navigate', 'toggle'])

// Every nav item routes to a real, wired feature — the old disabled
// "Soon" group is gone as of 2026-07-07: Voice Assistant, Knowledge
// Graph, Task Management, Skill Catalog and Prompt Catalog all have
// real backends (/chat multimodal audio, /graph, /tasks, /skills,
// /prompts) and views now. Chat (Sarza) is the app's home/default
// screen, so the brand button (below) opens Chat.
const available = [
  { id: 'chat', label: 'Chat', icon: 'sarza' },
  { id: 'voice', label: 'Voice Assistant', icon: 'voice' },
  { id: 'profile', label: 'Profile Rules' },
  { id: 'memory', label: 'Memory Search' },
  { id: 'project', label: 'Project Context' },
  { id: 'graph', label: 'Knowledge Graph', icon: 'graph' },
  { id: 'tasks', label: 'Task Management', icon: 'tasks' },
  { id: 'skills', label: 'Skill Catalog', icon: 'skills' },
  { id: 'prompts', label: 'Prompt Catalog', icon: 'prompts' },
]

const ICONS = {
  profile:
    '<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round"><circle cx="12" cy="8" r="3.2"/><path d="M5.5 19c1-3 3.5-4.5 6.5-4.5s5.5 1.5 6.5 4.5"/></svg>',
  memory:
    '<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round"><circle cx="10.5" cy="10.5" r="6"/><line x1="20" y1="20" x2="15.2" y2="15.2"/></svg>',
  project:
    '<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round"><path d="M4 6.5A1.5 1.5 0 0 1 5.5 5h4l2 2.5h7A1.5 1.5 0 0 1 20 9v8.5A1.5 1.5 0 0 1 18.5 19h-13A1.5 1.5 0 0 1 4 17.5z"/></svg>',
  voice:
    '<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round"><rect x="9" y="3" width="6" height="11" rx="3"/><path d="M5.5 11a6.5 6.5 0 0 0 13 0"/><line x1="12" y1="17.5" x2="12" y2="21"/><line x1="8.5" y1="21" x2="15.5" y2="21"/></svg>',
  graph:
    '<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round"><circle cx="6" cy="6" r="2.2"/><circle cx="18" cy="6" r="2.2"/><circle cx="12" cy="13" r="2.2"/><circle cx="6" cy="19" r="2.2"/><circle cx="18" cy="19" r="2.2"/><line x1="7.6" y1="7.6" x2="10.6" y2="11.4"/><line x1="16.4" y1="7.6" x2="13.4" y2="11.4"/><line x1="10.6" y1="14.6" x2="7.6" y2="17.4"/><line x1="13.4" y1="14.6" x2="16.4" y2="17.4"/></svg>',
  tasks:
    '<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round"><rect x="3.5" y="4" width="17" height="16" rx="2"/><path d="M7 9.2l1.4 1.4L11 8"/><line x1="13.5" y1="9" x2="17" y2="9"/><path d="M7 15.2l1.4 1.4L11 14"/><line x1="13.5" y1="15" x2="17" y2="15"/></svg>',
  skills:
    '<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round"><rect x="4" y="4" width="7" height="7" rx="1.5"/><rect x="13" y="4" width="7" height="7" rx="1.5"/><rect x="4" y="13" width="7" height="7" rx="1.5"/><rect x="13" y="13" width="7" height="7" rx="1.5"/></svg>',
  prompts:
    '<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round"><path d="M7 3.5h7l3 3v13a1 1 0 0 1-1 1H7a1 1 0 0 1-1-1v-15a1 1 0 0 1 1-1z"/><line x1="8.5" y1="10" x2="15.5" y2="10"/><line x1="8.5" y1="13.5" x2="15.5" y2="13.5"/><line x1="8.5" y1="17" x2="13" y2="17"/></svg>',
}
</script>

<template>
  <nav class="sidebar" :class="{ 'is-expanded': expanded }">
    <button
      type="button"
      class="collapse-toggle"
      :title="expanded ? 'Collapse sidebar' : 'Expand sidebar'"
      @click="emit('toggle')"
    >
      <svg viewBox="0 0 24 24" width="13" height="13" fill="none" stroke="currentColor" stroke-width="2">
        <rect x="3" y="4" width="18" height="16" rx="3" />
        <line x1="9" y1="4" x2="9" y2="20" />
      </svg>
    </button>

    <div class="rail-top">
      <button type="button" class="brand-btn" title="Agentic Desk" @click="emit('navigate', 'chat')">
        <BrandMark class="brand-mark" />
      </button>
    </div>

    <div class="nav-scroll">
      <div class="nav-group">
        <button
          v-for="item in available"
          :key="item.id"
          type="button"
          class="nav-item"
          :class="{ 'is-active': active === item.id }"
          :title="item.label"
          @click="emit('navigate', item.id)"
        >
          <span class="nav-icon">
            <SarzaMark v-if="item.icon === 'sarza'" />
            <span v-else v-html="ICONS[item.icon ?? item.id]"></span>
          </span>
          <span v-if="expanded" class="nav-label">{{ item.label }}</span>
        </button>
      </div>
    </div>
  </nav>
</template>

<style scoped>
.sidebar {
  position: relative;
  display: flex;
  flex-direction: column;
  gap: 12px;
  width: 56px;
  flex-shrink: 0;
  height: 100%;
  padding: 12px 8px;
  background: var(--surface);
  border-right: 1px solid var(--border);
  transition: width 200ms var(--ease-out-expo);
}

.sidebar.is-expanded {
  width: 236px;
}

.collapse-toggle {
  position: absolute;
  top: 52px;
  right: -12px;
  display: flex;
  align-items: center;
  justify-content: center;
  width: 26px;
  height: 26px;
  border: 1px solid var(--border);
  border-radius: 50%;
  background: var(--surface);
  color: var(--ink-muted);
  cursor: pointer;
  z-index: 5;
  transition: background-color 150ms var(--ease-out-expo), color 150ms var(--ease-out-expo);
}

.collapse-toggle:hover {
  background: var(--surface-hover);
  color: var(--ink);
}

.collapse-toggle:focus-visible {
  outline: 2px solid var(--accent);
  outline-offset: 1px;
}

.rail-top {
  display: flex;
  align-items: center;
  justify-content: center;
  gap: 4px;
}

/* brand-btn is static by design — no hover background/scale, it should
   read as a fixed mark, not an interactive control (click-to-navigate to
   Chat still works via cursor: pointer + the click handler). */
.brand-btn {
  display: flex;
  align-items: center;
  justify-content: center;
  width: 36px;
  height: 36px;
  flex-shrink: 0;
  border: none;
  border-radius: var(--radius-sm);
  background: transparent;
  cursor: pointer;
}

.brand-btn:focus-visible {
  outline: 2px solid var(--accent);
  outline-offset: -2px;
}

.brand-mark {
  width: 28px;
  height: 28px;
}

.nav-scroll {
  flex: 1;
  min-height: 0;
  overflow-y: auto;
  display: flex;
  flex-direction: column;
  gap: 16px;
}

.nav-group {
  display: flex;
  flex-direction: column;
  gap: 2px;
  white-space: nowrap;
}

.nav-group-divider {
  padding-top: 12px;
  border-top: 1px solid var(--border);
}

.nav-item {
  display: flex;
  align-items: center;
  gap: 10px;
  width: 100%;
  padding: 8px 10px;
  border: none;
  border-radius: var(--radius-md);
  background: transparent;
  color: var(--ink-muted);
  font-size: 13px;
  text-align: left;
  cursor: pointer;
  transition: background-color 150ms var(--ease-out-expo), color 150ms var(--ease-out-expo);
}

.sidebar:not(.is-expanded) .nav-item {
  justify-content: center;
  padding: 8px;
}

.nav-item:hover:not(.is-disabled) {
  background: var(--surface-hover);
  color: var(--ink);
}

.nav-item:focus-visible {
  outline: 2px solid var(--accent);
  outline-offset: -2px;
}

.nav-item.is-active {
  background: var(--accent-soft);
  color: var(--accent);
  font-weight: 600;
}

.nav-item.is-disabled {
  color: var(--ink-faint);
  opacity: 0.6;
  cursor: default;
}

.nav-icon {
  display: flex;
  align-items: center;
  justify-content: center;
  width: 18px;
  height: 18px;
  flex-shrink: 0;
}

.nav-icon :deep(svg),
.nav-icon :deep(img) {
  width: 100%;
  height: 100%;
}

.nav-label {
  flex: 1;
  min-width: 0;
  overflow: hidden;
  text-overflow: ellipsis;
}

.badge-soon {
  display: flex;
  align-items: center;
  gap: 3px;
  flex-shrink: 0;
  padding: 2px 6px;
  border-radius: 999px;
  background: var(--accent-ai-soft);
  color: var(--accent-ai);
  font-size: 10px;
  font-weight: 700;
  text-transform: uppercase;
  letter-spacing: 0.03em;
}

.badge-icon {
  width: 9px;
  height: 9px;
}

</style>
