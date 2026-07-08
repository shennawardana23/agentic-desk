<script setup>
// Knowledge Graph — rebuilt 2026-07-07 to match the Arch Connect
// reference (archpublicwebsite-mcp ui/src/views/KnowledgeGraph.vue):
// full-bleed 3d-force-graph WebGL canvas, glassmorphism overlays
// (title+stats, search, legend with kind toggles, toolbar), node
// inspector panel, theme-aware background. Data still comes from this
// app's own GET /graph (Second Brain projection) — only the rendering
// and chrome follow the reference.
import { computed, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import ForceGraph3D from '3d-force-graph'
import { useCoreStore } from '../stores/core'

const core = useCoreStore()
const host = ref(null)
const selected = ref(null)
const query = ref('')
const hiddenKinds = ref(new Set())
let fg = null
let resizeObserver = null
let themeObserver = null

// Kind palette lifted from the reference's useKGColors.js family —
// mapped onto this app's three Second Brain kinds.
const KIND_META = {
  rule: { label: 'Profile rule', dark: '#a78bfa', light: '#7c3aed' },
  memory: { label: 'Memory', dark: '#34d399', light: '#059669' },
  project: { label: 'Project', dark: '#f59e0b', light: '#b45309' },
}

function isDark() {
  return document.documentElement.getAttribute('data-theme') === 'dark'
}

function kindColor(kind) {
  const m = KIND_META[kind] ?? { dark: '#94a3b8', light: '#475569' }
  return isDark() ? m.dark : m.light
}

const stats = computed(() => ({
  nodes: core.graphData?.nodes?.length ?? 0,
  edges: core.graphData?.edges?.length ?? 0,
}))

const kindsInData = computed(() => {
  const present = new Set((core.graphData?.nodes ?? []).map((n) => n.kind))
  return Object.keys(KIND_META).filter((k) => present.has(k))
})

// The graph library mutates links (source/target become node objects
// after layout) — normalize back to ids when filtering/inspecting.
function endpointId(v) {
  return typeof v === 'object' && v !== null ? v.id : v
}

function visibleData() {
  const nodes = (core.graphData?.nodes ?? []).filter((n) => !hiddenKinds.value.has(n.kind))
  const q = query.value.trim().toLowerCase()
  const kept = q ? nodes.filter((n) => n.label.toLowerCase().includes(q) || (n.snippet ?? '').toLowerCase().includes(q)) : nodes
  const ids = new Set(kept.map((n) => n.id))
  const links = (core.graphData?.edges ?? [])
    .filter((e) => ids.has(endpointId(e.source)) && ids.has(endpointId(e.target)))
    .map((e) => ({ source: endpointId(e.source), target: endpointId(e.target), weight: e.weight }))
  return { nodes: kept.map((n) => ({ ...n })), links }
}

function refreshGraph() {
  if (fg) fg.graphData(visibleData())
}

function toggleKind(kind) {
  const s = new Set(hiddenKinds.value)
  s.has(kind) ? s.delete(kind) : s.add(kind)
  hiddenKinds.value = s
  refreshGraph()
}

const neighbors = computed(() => {
  if (!selected.value || !core.graphData) return []
  const id = selected.value.id
  const byId = new Map(core.graphData.nodes.map((n) => [n.id, n]))
  return core.graphData.edges
    .filter((e) => endpointId(e.source) === id || endpointId(e.target) === id)
    .map((e) => {
      const otherId = endpointId(e.source) === id ? endpointId(e.target) : endpointId(e.source)
      return { node: byId.get(otherId), weight: e.weight }
    })
    .filter((x) => x.node)
    .sort((a, b) => b.weight - a.weight)
    .slice(0, 8)
})

function applyTheme() {
  if (!fg) return
  fg.backgroundColor(isDark() ? '#030308' : '#eef2fa')
  refreshGraph() // node colors are theme-dependent
}

function fit() {
  fg?.zoomToFit(600, 40)
}

function initGraph() {
  fg = ForceGraph3D()(host.value)
    .backgroundColor(isDark() ? '#030308' : '#eef2fa')
    .nodeLabel((n) => `${n.label} (${KIND_META[n.kind]?.label ?? n.kind})`)
    .nodeColor((n) => kindColor(n.kind))
    .nodeVal(2.2)
    .nodeOpacity(0.92)
    .linkColor(() => (isDark() ? '#64748b' : '#94a3b8'))
    .linkOpacity(0.28)
    .linkWidth((l) => 0.5 + (l.weight ?? 0.5) * 1.5)
    .onNodeClick((n) => {
      selected.value = core.graphData.nodes.find((x) => x.id === n.id) ?? n
      // Fly the camera toward the node, same feel as the reference.
      const d = 80
      const r = 1 + d / Math.hypot(n.x || 1, n.y || 1, n.z || 1)
      fg.cameraPosition({ x: n.x * r, y: n.y * r, z: n.z * r }, n, 1000)
    })
    .onBackgroundClick(() => {
      selected.value = null
    })
  // Default zoom: the force engine starts every layout tightly clustered
  // at the origin, so the initial camera framing is too close-in until
  // it settles. onEngineStop fires once the layout has relaxed — fit to
  // it there so all nodes are visible by default. Guarded to run only
  // once; without the guard this re-fires (and fights manual zoom/pan)
  // every time the engine re-heats, e.g. on a legend filter toggle.
  let didInitialFit = false
  fg.onEngineStop(() => {
    if (didInitialFit) return
    didInitialFit = true
    fg.zoomToFit(600, 40)
  })
  fg.graphData(visibleData())

  resizeObserver = new ResizeObserver(() => {
    if (fg && host.value) {
      fg.width(host.value.clientWidth)
      fg.height(host.value.clientHeight)
    }
  })
  resizeObserver.observe(host.value)

  themeObserver = new MutationObserver(applyTheme)
  themeObserver.observe(document.documentElement, { attributes: true, attributeFilter: ['data-theme'] })
}

watch(query, refreshGraph)

onMounted(async () => {
  await core.loadGraph()
  if (core.graphData && host.value) initGraph()
})

onBeforeUnmount(() => {
  resizeObserver?.disconnect()
  themeObserver?.disconnect()
  fg?._destructor?.()
  fg = null
})

const hasData = computed(() => (core.graphData?.nodes?.length ?? 0) > 0)
</script>

<template>
  <section class="kg">
    <div ref="host" class="kg-canvas"></div>

    <!-- Glass overlays, same regions as the reference: title+stats
         top-left, search top-right, legend bottom-left, toolbar
         bottom-right. -->
    <div class="kg-panel kg-title">
      <h1>Knowledge Graph</h1>
      <p class="kg-stats">{{ stats.nodes }} nodes · {{ stats.edges }} links</p>
    </div>

    <div class="kg-panel kg-search">
      <input v-model="query" type="search" placeholder="Search nodes…" />
    </div>

    <div v-if="hasData" class="kg-panel kg-legend">
      <button
        v-for="k in kindsInData"
        :key="k"
        type="button"
        class="legend-item"
        :class="{ 'is-hidden': hiddenKinds.has(k) }"
        @click="toggleKind(k)"
      >
        <i :style="{ background: kindColor(k) }"></i>
        {{ KIND_META[k].label }}
      </button>
    </div>

    <div v-if="hasData" class="kg-panel kg-toolbar">
      <button type="button" @click="fit">Fit</button>
      <button type="button" @click="core.loadGraph().then(refreshGraph)">Reload</button>
    </div>

    <aside v-if="selected" class="kg-panel kg-inspector">
      <header>
        <span class="inspector-kind" :style="{ color: kindColor(selected.kind) }">
          {{ KIND_META[selected.kind]?.label ?? selected.kind }}
        </span>
        <button type="button" title="Close" @click="selected = null">✕</button>
      </header>
      <h3>{{ selected.label }}</h3>
      <p v-if="selected.snippet" class="inspector-snippet">{{ selected.snippet }}</p>
      <template v-if="neighbors.length">
        <h4>Connections</h4>
        <ul class="inspector-links">
          <li v-for="n in neighbors" :key="n.node.id">
            <i :style="{ background: kindColor(n.node.kind) }"></i>
            <span class="link-label">{{ n.node.label }}</span>
            <span class="link-weight">{{ n.weight.toFixed(2) }}</span>
          </li>
        </ul>
      </template>
    </aside>

    <div v-if="core.loadingGraph" class="kg-state">
      <span class="kg-spinner" aria-hidden="true"></span>
      Building graph…
    </div>
    <div v-else-if="core.graphError" class="kg-state kg-state-error" role="alert">{{ core.graphError }}</div>
    <div v-else-if="!hasData" class="kg-state">
      No nodes yet — the graph fills up as profile rules, memories and project contexts accumulate.
    </div>
  </section>
</template>

<style scoped>
.kg {
  position: relative;
  height: 100%;
  overflow: hidden;
}

.kg-canvas {
  position: absolute;
  inset: 0;
}

/* Glassmorphism overlay chrome, per the reference's kg-view styling. */
.kg-panel {
  position: absolute;
  z-index: 10;
  padding: 12px 14px;
  border: 1px solid var(--border);
  border-radius: var(--radius-md);
  background: color-mix(in oklab, var(--bg) 82%, transparent);
  backdrop-filter: blur(10px);
  -webkit-backdrop-filter: blur(10px);
  box-shadow: 0 4px 16px oklch(0 0 0 / 0.08);
}

.kg-title {
  top: 14px;
  left: 14px;
}

.kg-title h1 {
  margin: 0;
  font-size: 16px;
}

.kg-stats {
  margin: 2px 0 0;
  font-size: 11.5px;
  color: var(--ink-muted);
}

.kg-search {
  top: 14px;
  /* App.vue's fixed .theme-toggle pill sits at top:16px/right:16px and
     is ~60px wide, so it occupies the window's rightmost 76px — clear
     it with room to spare rather than the previous right:60px, which
     tucked this panel's edge underneath the toggle. */
  right: 110px;
  padding: 6px 8px;
}

.kg-search input {
  width: 200px;
  border: none;
  background: transparent;
  outline: none;
  font: inherit;
  font-size: 13px;
  color: var(--ink);
}

.kg-legend {
  bottom: 14px;
  left: 14px;
  display: flex;
  flex-direction: column;
  gap: 6px;
  padding: 10px 12px;
}

.legend-item {
  display: flex;
  align-items: center;
  gap: 7px;
  border: none;
  background: transparent;
  color: var(--ink-muted);
  font-size: 12px;
  cursor: pointer;
  padding: 2px 0;
}

.legend-item.is-hidden {
  opacity: 0.35;
  text-decoration: line-through;
}

.legend-item i {
  width: 10px;
  height: 10px;
  border-radius: 50%;
}

.kg-toolbar {
  bottom: 14px;
  right: 14px;
  display: flex;
  gap: 6px;
  padding: 8px;
}

.kg-toolbar button {
  padding: 5px 12px;
  border: 1px solid var(--border);
  border-radius: var(--radius-sm);
  background: transparent;
  color: var(--ink-muted);
  font-size: 12px;
  cursor: pointer;
}

.kg-toolbar button:hover {
  background: var(--surface-hover);
  color: var(--ink);
}

.kg-inspector {
  top: 70px;
  right: 14px;
  width: 280px;
  max-height: calc(100% - 100px);
  overflow-y: auto;
}

.kg-inspector header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 6px;
}

.inspector-kind {
  font-size: 10px;
  font-weight: 700;
  text-transform: uppercase;
  letter-spacing: 0.05em;
}

.kg-inspector header button {
  border: none;
  background: transparent;
  color: var(--ink-faint);
  cursor: pointer;
}

.kg-inspector h3 {
  margin: 0 0 6px;
  font-size: 13.5px;
  word-break: break-word;
}

.inspector-snippet {
  margin: 0 0 10px;
  font-size: 12px;
  color: var(--ink-muted);
  white-space: pre-wrap;
  word-break: break-word;
}

.kg-inspector h4 {
  margin: 0 0 6px;
  font-size: 10.5px;
  font-weight: 700;
  text-transform: uppercase;
  letter-spacing: 0.05em;
  color: var(--ink-faint);
}

.inspector-links {
  margin: 0;
  padding: 0;
  list-style: none;
  display: flex;
  flex-direction: column;
  gap: 5px;
}

.inspector-links li {
  display: flex;
  align-items: center;
  gap: 7px;
  font-size: 12px;
  color: var(--ink-muted);
}

.inspector-links i {
  width: 8px;
  height: 8px;
  border-radius: 50%;
  flex-shrink: 0;
}

.link-label {
  flex: 1;
  min-width: 0;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.link-weight {
  font-variant-numeric: tabular-nums;
  color: var(--ink-faint);
}

.kg-state {
  position: absolute;
  inset: 0;
  z-index: 5;
  display: flex;
  align-items: center;
  justify-content: center;
  gap: 10px;
  font-size: 13px;
  color: var(--ink-muted);
  pointer-events: none;
}

.kg-state-error {
  color: var(--danger);
}

.kg-spinner {
  width: 18px;
  height: 18px;
  border: 2px solid var(--border);
  border-top-color: var(--accent);
  border-radius: 50%;
  animation: kg-spin 0.9s linear infinite;
}

@keyframes kg-spin {
  to {
    transform: rotate(360deg);
  }
}

@media (prefers-reduced-motion: reduce) {
  .kg-spinner {
    animation: none;
  }
}
</style>
