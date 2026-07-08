<script setup>
import { computed, onMounted, ref } from 'vue'
import { useCoreStore } from '../stores/core'
import CatalogItemModal from './CatalogItemModal.vue'

const core = useCoreStore()
const query = ref('')
const openName = ref('')
const openContent = ref('')
const openError = ref('')
const openLoading = ref(false)

onMounted(() => core.loadPrompts())

// Frontmatter category tabs (falls back to 'General' when a prompt has
// none), sorted alphabetically with General pinned last since it's the
// catch-all rather than a real category. Built from the full unfiltered
// list so the tab bar doesn't reshuffle while the user types a search.
const categories = computed(() => {
  const set = new Set(core.prompts.map((p) => p.category || 'General'))
  return [...set].sort((a, b) => {
    if (a === 'General') return 1
    if (b === 'General') return -1
    return a.localeCompare(b)
  })
})
const activeCategory = ref('all')

const filtered = computed(() => {
  const q = query.value.trim().toLowerCase()
  return core.prompts.filter((p) => {
    if (activeCategory.value !== 'all' && (p.category || 'General') !== activeCategory.value) return false
    if (!q) return true
    return p.name.toLowerCase().includes(q) || p.description.toLowerCase().includes(q)
  })
})

async function open(name) {
  openName.value = name
  openContent.value = ''
  openError.value = ''
  openLoading.value = true
  try {
    openContent.value = await core.loadPromptContent(name)
  } catch (err) {
    openError.value = String(err)
  } finally {
    openLoading.value = false
  }
}
</script>

<template>
  <section class="catalog">
    <header class="view-head">
      <h1>Prompt Catalog</h1>
      <p class="view-sub">The .prompt (Dotprompt) files the Genkit core loads (prompts/ in the repo) — browse-only.</p>
    </header>

    <p v-if="core.promptsError" class="view-error" role="alert">{{ core.promptsError }}</p>

    <input v-model="query" type="search" class="filter" placeholder="Filter prompts…" />

    <div class="category-tabs" role="tablist">
      <button
        type="button"
        role="tab"
        :aria-selected="activeCategory === 'all'"
        :class="{ 'is-active': activeCategory === 'all' }"
        @click="activeCategory = 'all'"
      >
        All
      </button>
      <button
        v-for="cat in categories"
        :key="cat"
        type="button"
        role="tab"
        :aria-selected="activeCategory === cat"
        :class="{ 'is-active': activeCategory === cat }"
        @click="activeCategory = cat"
      >
        {{ cat }}
      </button>
    </div>

    <p v-if="!core.loadingPrompts && filtered.length === 0" class="view-note">No prompts match.</p>

    <div class="card-grid">
      <button v-for="p in filtered" :key="p.name" type="button" class="item-card" @click="open(p.name)">
        <h3>{{ p.name }}</h3>
        <p>{{ p.description || 'No description.' }}</p>
      </button>
    </div>

    <CatalogItemModal
      v-if="openName"
      kind="prompt"
      :name="openName"
      :content="openContent"
      :loading="openLoading"
      :error="openError"
      @close="openName = ''"
    />
  </section>
</template>

<style scoped>
.view-head h1 {
  margin: 0 0 4px;
  font-size: 22px;
}

.view-sub {
  margin: 0 0 20px;
  font-size: 13px;
  color: var(--ink-muted);
}

.view-error {
  margin: 0 0 12px;
  padding: 8px 12px;
  border-radius: var(--radius-sm);
  background: var(--danger-soft);
  color: var(--danger);
  font-size: 12px;
}

.view-note {
  font-size: 13px;
  color: var(--ink-faint);
}

.filter {
  width: 100%;
  max-width: 360px;
  margin-bottom: 20px;
  padding: 9px 12px;
  border: 1px solid var(--border);
  border-radius: var(--radius-md);
  background: var(--surface);
  color: var(--ink);
  font: inherit;
  font-size: 13px;
}

.filter:focus {
  outline: 2px solid var(--accent);
  outline-offset: -1px;
}

.category-tabs {
  display: flex;
  flex-wrap: wrap;
  gap: 4px;
  margin-bottom: 20px;
  padding-bottom: 8px;
  border-bottom: 1px solid var(--border);
}

.category-tabs button {
  padding: 7px 14px;
  border: none;
  border-radius: var(--radius-sm);
  background: transparent;
  color: var(--ink-muted);
  font-size: 12.5px;
  font-weight: 600;
  cursor: pointer;
  transition: background-color 150ms var(--ease-out-expo), color 150ms var(--ease-out-expo);
}

.category-tabs button:hover {
  background: var(--surface-hover);
  color: var(--ink);
}

.category-tabs button.is-active {
  background: var(--accent-ai-soft);
  color: var(--accent-ai);
}

.card-grid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(240px, 1fr));
  gap: 12px;
}

.item-card {
  padding: 14px;
  border: 1px solid var(--border);
  border-radius: var(--radius-md);
  background: var(--surface);
  text-align: left;
  cursor: pointer;
  transition: background-color 150ms var(--ease-out-expo), border-color 150ms var(--ease-out-expo);
}

.item-card:hover {
  background: var(--accent-soft);
  border-color: var(--accent);
}

.item-card h3 {
  margin: 0 0 6px;
  font-size: 13.5px;
  color: var(--ink);
  word-break: break-word;
}

.item-card p {
  margin: 0;
  font-size: 12px;
  line-height: 1.5;
  color: var(--ink-muted);
  display: -webkit-box;
  -webkit-line-clamp: 3;
  -webkit-box-orient: vertical;
  overflow: hidden;
}
</style>
