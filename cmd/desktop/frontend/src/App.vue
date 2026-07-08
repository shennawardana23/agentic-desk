<script setup>
import { ref, watch } from 'vue'
import AppSidebar from './components/AppSidebar.vue'
import ChatView from './components/ChatView.vue'
import VoiceView from './components/VoiceView.vue'
import ProfileView from './components/ProfileView.vue'
import MemorySearch from './components/MemorySearch.vue'
import ProjectContextView from './components/ProjectContextView.vue'
import KnowledgeGraphView from './components/KnowledgeGraphView.vue'
import TasksView from './components/TasksView.vue'
import SkillCatalogView from './components/SkillCatalogView.vue'
import PromptCatalogView from './components/PromptCatalogView.vue'
import backgroundUrl from './assets/background-desktop.svg'

// Chat (Sarza) is the app's home/default screen — there is no separate
// Home view/route anymore, it merged into Chat's empty-state hero (see
// ChatView.vue). ChatView stays a direct child of .content (not wrapped
// in .content-inner) because it manages its own full-height layout;
// wrapping it in an auto-height div breaks that (see the vertical-
// centering bug this exact pattern caused when Home was separate).
const activeView = ref('chat')
const sidebarExpanded = ref(false)

function navigate(view) {
  activeView.value = view
}

// Theme toggle — a real, working feature (CSS vars already token-based), not a
// stub: persists to localStorage, otherwise falls back to OS preference. Lives
// at the window level (not the sidebar) — it's a screen-wide setting, not a nav item.
const stored = localStorage.getItem('agentic-desk-theme')
const isDark = ref(stored ? stored === 'dark' : window.matchMedia('(prefers-color-scheme: dark)').matches)
watch(
  isDark,
  (dark) => {
    document.documentElement.setAttribute('data-theme', dark ? 'dark' : 'light')
    localStorage.setItem('agentic-desk-theme', dark ? 'dark' : 'light')
  },
  { immediate: true },
)

</script>

<template>
  <div class="shell">
    <AppSidebar
      :active="activeView"
      :expanded="sidebarExpanded"
      @navigate="navigate"
      @toggle="sidebarExpanded = !sidebarExpanded"
    />
    <main class="content" :style="{ '--bg-pattern': `url(${backgroundUrl})` }">
      <ChatView v-if="activeView === 'chat'" />
      <!-- Knowledge Graph is full-bleed (reference parity: one WebGL
           surface, glass overlays) — a direct child of .content like
           ChatView, not wrapped in the padded .content-inner. -->
      <KnowledgeGraphView v-else-if="activeView === 'graph'" />
      <div v-else class="content-inner">
        <div class="content-bg" aria-hidden="true"></div>
        <VoiceView v-if="activeView === 'voice'" />
        <ProfileView v-else-if="activeView === 'profile'" />
        <MemorySearch v-else-if="activeView === 'memory'" />
        <ProjectContextView v-else-if="activeView === 'project'" />
        <TasksView v-else-if="activeView === 'tasks'" />
        <SkillCatalogView v-else-if="activeView === 'skills'" />
        <PromptCatalogView v-else-if="activeView === 'prompts'" />
      </div>
    </main>

    <div class="theme-toggle" role="group" aria-label="Theme">
      <button type="button" data-mode="light" aria-label="Light mode" :aria-pressed="!isDark" @click="isDark = false">
        <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true">
          <circle cx="12" cy="12" r="4"></circle>
          <path d="M12 2v2M12 20v2M4.93 4.93l1.41 1.41M17.66 17.66l1.41 1.41M2 12h2M20 12h2M4.93 19.07l1.41-1.41M17.66 6.34l1.41-1.41"></path>
        </svg>
      </button>
      <button type="button" data-mode="dark" aria-label="Dark mode" :aria-pressed="isDark" @click="isDark = true">
        <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true">
          <path d="M21 12.79A9 9 0 1 1 11.21 3 7 7 0 0 0 21 12.79z"></path>
        </svg>
      </button>
    </div>
  </div>
</template>

<style scoped>
.shell {
  display: flex;
  height: 100vh;
}

.content {
  position: relative;
  flex: 1;
  min-width: 0;
  overflow-y: auto;
}

.content-inner {
  position: relative;
  min-height: 100%;
  /* top padding clears the fixed .theme-toggle pill (top:16px, ~30px tall)
     so per-view top-right elements (e.g. VoiceView's status badge) don't
     stack under it. */
  padding: 64px 40px 32px;
}

.content-bg {
  position: absolute;
  inset: 0;
  background-image: var(--bg-pattern);
  background-repeat: no-repeat;
  background-position: bottom right;
  background-size: min(480px, 60%);
  opacity: 0.04;
  pointer-events: none;
}

.theme-toggle {
  position: fixed;
  top: 16px;
  right: 16px;
  display: flex;
  align-items: center;
  gap: 2px;
  padding: 2px;
  border: 1px solid var(--border);
  border-radius: 999px;
  background: var(--surface);
  z-index: 10;
}

.theme-toggle button {
  display: flex;
  align-items: center;
  justify-content: center;
  width: 26px;
  height: 26px;
  border: none;
  border-radius: 50%;
  background: transparent;
  color: var(--ink-faint);
  cursor: pointer;
  transition: background-color 150ms var(--ease-out-expo), color 150ms var(--ease-out-expo);
}

.theme-toggle button svg {
  width: 14px;
  height: 14px;
}

.theme-toggle button:hover {
  color: var(--ink);
}

.theme-toggle button[aria-pressed='true'] {
  background: var(--accent-soft);
  color: var(--accent);
}

.theme-toggle button:focus-visible {
  outline: 2px solid var(--accent);
  outline-offset: 1px;
}
</style>
