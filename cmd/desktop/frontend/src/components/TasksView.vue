<script setup>
// Task Management — rebuilt 2026-07-07 to match the Arch Connect
// reference (archpublicwebsite-mcp ui/src/components/tasks/TaskBoard.vue):
// Linear/Plane-style kanban with vuedraggable (SortableJS) drag-drop
// between columns, priority pills with the reference's color scale,
// search filter, column counts. Backend stays this app's own /tasks
// CRUD (title/notes/status/priority) — priority int maps onto the
// reference's low/medium/high/urgent scale.
import { onBeforeUnmount, onMounted, reactive, ref, watch } from 'vue'
import draggable from 'vuedraggable'
import { useCoreStore } from '../stores/core'

const core = useCoreStore()
const newTitle = ref('')
const newPriority = ref(0)
const query = ref('')

onMounted(() => core.loadTasks())

const COLUMNS = [
  { id: 'todo', label: 'To Do', dot: '#94a3b8' },
  { id: 'doing', label: 'In Progress', dot: '#f59e0b' },
  { id: 'done', label: 'Done', dot: '#10b981' },
]

// Priority scale per the reference's TaskCard.vue color tokens.
const PRIORITIES = [
  { value: 0, label: 'Low', color: '#64748b', bg: 'rgba(100, 116, 139, 0.12)' },
  { value: 1, label: 'Medium', color: '#8b5cf6', bg: 'rgba(139, 92, 246, 0.12)' },
  { value: 2, label: 'High', color: '#f59e0b', bg: 'rgba(245, 158, 11, 0.14)' },
  { value: 3, label: 'Urgent', color: '#ef4444', bg: 'rgba(239, 68, 68, 0.12)' },
]

function priorityMeta(p) {
  return PRIORITIES.find((x) => x.value === Math.min(Math.max(p, 0), 3)) ?? PRIORITIES[0]
}

// Task detail modal — a local editable copy of the clicked task; nothing
// is persisted until Save. Delete removes the task outright.
const selectedTask = ref(null)

function openTask(t) {
  selectedTask.value = { ...t, description: t.description ?? '' }
}

function closeModal() {
  selectedTask.value = null
}

function onKeydown(e) {
  if (e.key === 'Escape') closeModal()
}

watch(selectedTask, (v) => {
  if (v) window.addEventListener('keydown', onKeydown)
  else window.removeEventListener('keydown', onKeydown)
})

onBeforeUnmount(() => window.removeEventListener('keydown', onKeydown))

function formatMeta(iso) {
  if (!iso) return '—'
  return new Date(iso).toLocaleString()
}

async function saveTask() {
  if (!selectedTask.value) return
  await core.updateTask(selectedTask.value)
  closeModal()
}

async function deleteTask() {
  if (!selectedTask.value) return
  await core.deleteTask(selectedTask.value.id)
  closeModal()
}

// Local per-column arrays for vuedraggable v-model; rebuilt from the
// store whenever tasks change (drag mutates these locally first, then
// the @change handler persists the status move).
const columns = reactive({ todo: [], doing: [], done: [] })

function syncColumns() {
  const q = query.value.trim().toLowerCase()
  for (const col of COLUMNS) {
    columns[col.id] = core.tasks.filter(
      (t) =>
        t.status === col.id &&
        (!q || t.title.toLowerCase().includes(q) || (t.notes ?? '').toLowerCase().includes(q)),
    )
  }
}

watch([() => core.tasks, query], syncColumns, { deep: true, immediate: true })

// vuedraggable @change: evt.added fires on the RECEIVING column when a
// card is dropped cross-column — that's the status change to persist.
// Same-column reorder (evt.moved) is visual-only; the backend has no
// sort_order column. ponytail: add one if ordering must survive reload.
function onColumnChange(status, evt) {
  if (evt.added) core.updateTask({ ...evt.added.element, status })
}

async function add() {
  const title = newTitle.value.trim()
  if (!title) return
  newTitle.value = ''
  await core.createTask(title, '', newPriority.value)
}
</script>

<template>
  <section class="tasks">
    <header class="board-header">
      <div>
        <h1>Task Management</h1>
        <p class="board-sub">Local board — drag cards between columns. Stored in your desk's own Postgres.</p>
      </div>
      <input v-model="query" type="search" class="board-search" placeholder="Search tasks…" />
    </header>

    <p v-if="core.tasksError" class="board-error" role="alert">{{ core.tasksError }}</p>

    <form class="add-row" @submit.prevent="add">
      <input v-model="newTitle" type="text" placeholder="Add a task…" />
      <select v-model.number="newPriority" class="tv-select" aria-label="Priority">
        <option v-for="p in PRIORITIES" :key="p.value" :value="p.value">{{ p.label }}</option>
      </select>
      <button type="submit" :disabled="!newTitle.trim()">Add</button>
    </form>

    <div class="board">
      <div v-for="col in COLUMNS" :key="col.id" class="board-column">
        <header class="column-header">
          <i class="column-dot" :style="{ background: col.dot }"></i>
          <span class="column-title">{{ col.label }}</span>
          <span class="column-count">{{ columns[col.id].length }}</span>
        </header>

        <draggable
          v-model="columns[col.id]"
          group="tasks"
          item-key="id"
          class="column-body"
          ghost-class="card-ghost"
          chosen-class="card-chosen"
          :animation="150"
          @change="onColumnChange(col.id, $event)"
        >
          <template #item="{ element: t }">
            <article class="task-card" @click="openTask(t)">
              <div class="card-pills">
                <span
                  class="pill"
                  :style="{ color: priorityMeta(t.priority).color, background: priorityMeta(t.priority).bg }"
                >
                  {{ priorityMeta(t.priority).label }}
                </span>
              </div>
              <p class="card-title">{{ t.title }}</p>
              <p v-if="t.notes" class="card-notes">{{ t.notes }}</p>
              <footer class="card-footer">
                <span class="card-id">#{{ t.id }}</span>
                <button type="button" class="card-delete" title="Delete task" @click.stop="core.deleteTask(t.id)">✕</button>
              </footer>
            </article>
          </template>
          <template #footer>
            <p v-if="columns[col.id].length === 0" class="column-empty">No tasks</p>
          </template>
        </draggable>
      </div>
    </div>

    <div v-if="selectedTask" class="task-modal-overlay" @click="closeModal">
      <div class="task-modal" role="dialog" aria-modal="true" @click.stop>
        <input v-model="selectedTask.title" type="text" class="modal-title-input" placeholder="Task title" />

        <label class="modal-field">
          <span class="modal-label">Description</span>
          <textarea
            v-model="selectedTask.description"
            class="modal-textarea"
            rows="5"
            placeholder="Add a longer description…"
          ></textarea>
        </label>

        <div class="modal-row">
          <label class="modal-field">
            <span class="modal-label">
              <i class="modal-label-dot" :style="{ background: COLUMNS.find((c) => c.id === selectedTask.status)?.dot }"></i>
              Status
            </span>
            <select v-model="selectedTask.status" class="tv-select">
              <option v-for="col in COLUMNS" :key="col.id" :value="col.id">{{ col.label }}</option>
            </select>
          </label>

          <label class="modal-field">
            <span class="modal-label">
              <i class="modal-label-dot" :style="{ background: priorityMeta(selectedTask.priority).color }"></i>
              Priority
            </span>
            <select v-model.number="selectedTask.priority" class="tv-select">
              <option v-for="p in PRIORITIES" :key="p.value" :value="p.value">{{ p.label }}</option>
            </select>
          </label>
        </div>

        <p class="modal-meta">
          Created {{ formatMeta(selectedTask.createdAt) }} · Updated {{ formatMeta(selectedTask.updatedAt) }}
        </p>

        <footer class="modal-actions">
          <button type="button" class="modal-delete" @click="deleteTask">Delete</button>
          <div class="modal-actions-right">
            <button type="button" class="modal-cancel" @click="closeModal">Cancel</button>
            <button type="button" class="modal-save" @click="saveTask">Save</button>
          </div>
        </footer>
      </div>
    </div>
  </section>
</template>

<style scoped>
.tasks {
  display: flex;
  flex-direction: column;
  height: 100%;
}

.board-header {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 16px;
  margin-bottom: 16px;
}

.board-header h1 {
  margin: 0 0 4px;
  font-size: 22px;
}

.board-sub {
  margin: 0;
  font-size: 13px;
  color: var(--ink-muted);
}

.board-search {
  width: 220px;
  padding: 8px 12px;
  border: 1px solid var(--border);
  border-radius: var(--radius-md);
  background: var(--surface);
  color: var(--ink);
  font: inherit;
  font-size: 13px;
}

.board-search:focus {
  outline: 2px solid var(--accent);
  outline-offset: -1px;
}

.board-error {
  margin: 0 0 12px;
  padding: 8px 12px;
  border-radius: var(--radius-sm);
  background: var(--danger-soft);
  color: var(--danger);
  font-size: 12px;
}

.add-row {
  display: flex;
  gap: 8px;
  margin-bottom: 18px;
  max-width: 560px;
}

.add-row input {
  flex: 1;
  padding: 9px 12px;
  border: 1px solid var(--border);
  border-radius: var(--radius-md);
  background: var(--surface);
  color: var(--ink);
  font: inherit;
  font-size: 13px;
}

.add-row input:focus {
  outline: 2px solid var(--accent);
  outline-offset: -1px;
}

.add-row button {
  padding: 9px 16px;
  border: none;
  border-radius: var(--radius-md);
  background: var(--ink);
  color: var(--bg);
  font-size: 13px;
  font-weight: 600;
  cursor: pointer;
}

.add-row button:disabled {
  opacity: 0.35;
  cursor: default;
}

/* Reference TaskBoard.vue: fixed-width columns in a horizontal row,
   tinted column background, white lifted cards. */
.board {
  flex: 1;
  min-height: 0;
  display: flex;
  gap: 14px;
  align-items: stretch;
  overflow-x: auto;
  padding-bottom: 8px;
}

.board-column {
  display: flex;
  flex-direction: column;
  width: 300px;
  flex-shrink: 0;
  border-radius: var(--radius-lg);
  background: var(--surface);
  border: 1px solid var(--border);
}

.column-header {
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 12px 14px 10px;
}

.column-dot {
  width: 8px;
  height: 8px;
  border-radius: 50%;
}

.column-title {
  font-size: 12px;
  font-weight: 700;
  text-transform: uppercase;
  letter-spacing: 0.04em;
  color: var(--ink-muted);
}

.column-count {
  margin-left: auto;
  font-size: 11px;
  color: var(--ink-faint);
  font-variant-numeric: tabular-nums;
}

.column-body {
  flex: 1;
  min-height: 60px;
  overflow-y: auto;
  display: flex;
  flex-direction: column;
  gap: 8px;
  padding: 0 10px 12px;
}

.column-empty {
  margin: 4px;
  font-size: 12px;
  color: var(--ink-faint);
  font-style: italic;
  text-align: center;
}

.task-card {
  padding: 10px 12px;
  border: 1px solid var(--border);
  border-radius: var(--radius-md);
  background: var(--bg);
  cursor: grab;
  box-shadow: 0 1px 2px oklch(0 0 0 / 0.04);
}

.task-card:active {
  cursor: grabbing;
}

.card-ghost {
  opacity: 0.4;
}

.card-chosen {
  transform: rotate(1deg);
  box-shadow: 0 6px 18px oklch(0 0 0 / 0.14);
}

@media (prefers-reduced-motion: reduce) {
  .card-chosen {
    transform: none;
  }
}

.card-pills {
  display: flex;
  gap: 6px;
  margin-bottom: 6px;
}

.pill {
  padding: 2px 8px;
  border-radius: 999px;
  font-size: 10px;
  font-weight: 700;
  text-transform: uppercase;
  letter-spacing: 0.03em;
}

.card-title {
  margin: 0;
  font-size: 13.5px;
  line-height: 1.4;
  color: var(--ink);
  display: -webkit-box;
  -webkit-line-clamp: 2;
  -webkit-box-orient: vertical;
  overflow: hidden;
}

.card-notes {
  margin: 4px 0 0;
  font-size: 12px;
  color: var(--ink-muted);
  display: -webkit-box;
  -webkit-line-clamp: 2;
  -webkit-box-orient: vertical;
  overflow: hidden;
}

.card-footer {
  display: flex;
  align-items: center;
  justify-content: space-between;
  margin-top: 8px;
}

.card-id {
  font-size: 11px;
  color: var(--ink-faint);
  font-variant-numeric: tabular-nums;
}

.card-delete {
  border: none;
  background: transparent;
  color: var(--ink-faint);
  font-size: 11px;
  cursor: pointer;
  padding: 2px 4px;
  border-radius: var(--radius-sm);
}

.card-delete:hover {
  color: var(--danger);
  background: var(--danger-soft);
}

/* Custom-styled native <select> — kept as a real <select> for a11y
   (keyboard, screen readers, native mobile picker) but restyled to
   match the app's token system instead of the bare OS control. The
   chevron is baked into background-image (see the unscoped block
   below for the dark-mode variant) since background-image can't read
   currentColor. Priority/status color is echoed via .modal-label-dot
   next to the label, not on the control itself (a thick accent border
   on the input reads as an AI-template tell). */
.tv-select {
  appearance: none;
  -webkit-appearance: none;
  -moz-appearance: none;
  padding: 9px 32px 9px 12px;
  border: 1px solid var(--border);
  border-radius: var(--radius-md);
  background-color: var(--surface);
  background-image: url("data:image/svg+xml,%3Csvg xmlns='http://www.w3.org/2000/svg' viewBox='0 0 24 24' fill='none' stroke='%238a8a96' stroke-width='2' stroke-linecap='round' stroke-linejoin='round'%3E%3Cpolyline points='6 9 12 15 18 9'%3E%3C/polyline%3E%3C/svg%3E");
  background-repeat: no-repeat;
  background-position: right 10px center;
  background-size: 14px;
  color: var(--ink);
  font: inherit;
  font-size: 13px;
  cursor: pointer;
  transition: border-color 150ms var(--ease-out-expo), background-color 150ms var(--ease-out-expo);
}

.tv-select:hover {
  background-color: var(--surface-hover);
  border-color: var(--ink-faint);
}

.tv-select:focus {
  outline: 2px solid var(--accent);
  outline-offset: -1px;
}

.task-modal-overlay {
  position: fixed;
  inset: 0;
  z-index: 50;
  display: flex;
  align-items: center;
  justify-content: center;
  background: oklch(0 0 0 / 0.45);
  backdrop-filter: blur(2px);
  padding: 24px;
}

.task-modal {
  width: 100%;
  max-width: 520px;
  max-height: calc(100vh - 48px);
  overflow-y: auto;
  display: flex;
  flex-direction: column;
  gap: 14px;
  padding: 24px;
  border: 1px solid var(--border);
  border-radius: var(--radius-lg);
  background: var(--surface);
  box-shadow: 0 20px 60px oklch(0 0 0 / 0.25);
}

.modal-title-input {
  border: none;
  background: transparent;
  color: var(--ink);
  font: inherit;
  font-size: 19px;
  font-weight: 700;
  padding: 4px 2px;
  border-radius: var(--radius-sm);
}

.modal-title-input:focus {
  outline: 2px solid var(--accent);
  outline-offset: 2px;
  background: var(--surface-hover);
}

.modal-field {
  display: flex;
  flex-direction: column;
  gap: 6px;
  flex: 1;
}

.modal-label {
  display: flex;
  align-items: center;
  gap: 5px;
  font-size: 11px;
  font-weight: 700;
  text-transform: uppercase;
  letter-spacing: 0.04em;
  color: var(--ink-faint);
}

.modal-label-dot {
  width: 6px;
  height: 6px;
  border-radius: 50%;
  flex-shrink: 0;
}

.modal-textarea {
  resize: vertical;
  min-height: 90px;
  padding: 10px 12px;
  border: 1px solid var(--border);
  border-radius: var(--radius-md);
  background: var(--bg);
  color: var(--ink);
  font: inherit;
  font-size: 13px;
  line-height: 1.5;
}

.modal-textarea:focus {
  outline: 2px solid var(--accent);
  outline-offset: -1px;
}

.modal-row {
  display: flex;
  gap: 12px;
}

.modal-meta {
  margin: 0;
  font-size: 11.5px;
  color: var(--ink-faint);
}

.modal-actions {
  display: flex;
  align-items: center;
  justify-content: space-between;
  margin-top: 4px;
}

.modal-actions-right {
  display: flex;
  gap: 8px;
}

.modal-delete {
  padding: 8px 14px;
  border: 1px solid var(--danger-soft);
  border-radius: var(--radius-md);
  background: transparent;
  color: var(--danger);
  font-size: 13px;
  font-weight: 600;
  cursor: pointer;
}

.modal-delete:hover {
  background: var(--danger-soft);
}

.modal-cancel {
  padding: 8px 14px;
  border: 1px solid var(--border);
  border-radius: var(--radius-md);
  background: transparent;
  color: var(--ink-muted);
  font-size: 13px;
  font-weight: 600;
  cursor: pointer;
}

.modal-cancel:hover {
  background: var(--surface-hover);
  color: var(--ink);
}

.modal-save {
  padding: 8px 16px;
  border: none;
  border-radius: var(--radius-md);
  background: var(--ink);
  color: var(--bg);
  font-size: 13px;
  font-weight: 600;
  cursor: pointer;
}

.modal-save:hover {
  background: var(--accent);
  color: var(--accent-ink);
}
</style>

<style>
/* Unscoped: .tv-select's chevron is a background-image, which can't
   read a scoped CSS custom property or currentColor, so the dark-mode
   swap needs a plain (non component-scoped) selector against the
   :root[data-theme] attribute App.vue's theme toggle sets — same
   two-block (explicit + prefers-color-scheme fallback) pattern
   style.css uses for its own tokens. */
:root[data-theme='dark'] .tv-select {
  background-image: url("data:image/svg+xml,%3Csvg xmlns='http://www.w3.org/2000/svg' viewBox='0 0 24 24' fill='none' stroke='%23a3a3b4' stroke-width='2' stroke-linecap='round' stroke-linejoin='round'%3E%3Cpolyline points='6 9 12 15 18 9'%3E%3C/polyline%3E%3C/svg%3E");
}

@media (prefers-color-scheme: dark) {
  :root:not([data-theme='light']) .tv-select {
    background-image: url("data:image/svg+xml,%3Csvg xmlns='http://www.w3.org/2000/svg' viewBox='0 0 24 24' fill='none' stroke='%23a3a3b4' stroke-width='2' stroke-linecap='round' stroke-linejoin='round'%3E%3Cpolyline points='6 9 12 15 18 9'%3E%3C/polyline%3E%3C/svg%3E");
  }
}
</style>
