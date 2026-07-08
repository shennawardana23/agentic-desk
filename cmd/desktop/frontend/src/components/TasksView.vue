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
    <!-- Pattern blur background -->
    <div class="pattern-bg" aria-hidden="true">
      <svg class="pattern-bg__svg" viewBox="0 0 525 647" preserveAspectRatio="xMidYMid slice" xmlns="http://www.w3.org/2000/svg">
        <g opacity="0.15">
          <path d="M50.1406 64.7157V91.9056C50.1406 126.944 78.2762 155.108 113.217 155.108H140.38V127.918C140.38 92.9075 112.272 64.7157 77.3041 64.7157H50.1406Z" stroke="currentColor" stroke-miterlimit="10"/>
          <path d="M213.982 64.7157C179.042 64.7157 150.906 92.9075 150.906 127.918V155.108H178.07C213.01 155.108 241.146 126.916 241.146 91.9056V64.7157H213.982Z" stroke="currentColor" stroke-miterlimit="10"/>
          <path d="M113.217 165.683C78.2762 165.683 50.1406 193.875 50.1406 228.913V256.103H77.3041C112.272 256.103 140.38 227.911 140.38 192.901V165.711H113.217V165.683Z" stroke="currentColor" stroke-miterlimit="10"/>
          <path d="M150.906 165.683V192.873C150.906 227.911 179.042 256.075 213.982 256.075H241.146V228.885C241.146 193.847 213.01 165.683 178.07 165.683H150.906V165.683Z" stroke="currentColor" stroke-miterlimit="10"/>
          <path d="M77.1653 83.8631C75.0267 83.8631 72.9714 84.7259 71.4993 86.2565C70.0828 87.7315 69.2773 89.7074 69.2773 91.7668C69.2773 96.1362 72.8047 99.6427 77.1653 99.6427C81.5259 99.6427 85.0255 96.1083 85.0255 91.7668C85.0255 87.3975 81.4981 83.8631 77.1653 83.8631Z" stroke="currentColor" stroke-miterlimit="10"/>
          <path d="M214.177 83.8629C209.816 83.8629 206.289 87.3973 206.289 91.7666C206.289 96.1359 209.816 99.6425 214.177 99.6425C218.538 99.6425 222.037 96.1081 222.037 91.7666C222.037 89.7072 221.26 87.7313 219.815 86.2563C218.315 84.6978 216.288 83.8629 214.177 83.8629Z" stroke="currentColor" stroke-miterlimit="10"/>
          <path d="M113.38 120.153C111.242 120.153 109.186 121.016 107.714 122.546C106.298 124.021 105.492 125.997 105.492 128.057C105.492 132.426 109.02 135.932 113.38 135.932C117.741 135.932 121.24 132.398 121.268 128.057C121.268 123.715 117.741 120.153 113.38 120.153Z" stroke="currentColor" stroke-miterlimit="10"/>
          <path d="M177.931 120.153C173.57 120.153 170.043 123.688 170.043 128.057C170.043 132.426 173.57 135.933 177.931 135.933C182.292 135.933 185.791 132.398 185.791 128.057C185.791 125.997 185.013 124.021 183.569 122.547C182.097 121.016 180.042 120.153 177.931 120.153Z" stroke="currentColor" stroke-miterlimit="10"/>
          <path d="M113.38 184.83C109.02 184.83 105.492 188.365 105.492 192.734C105.492 197.103 109.02 200.61 113.38 200.61C117.741 200.61 121.24 197.075 121.24 192.734C121.24 190.675 120.463 188.699 119.018 187.224C117.546 185.693 115.519 184.83 113.38 184.83Z" stroke="currentColor" stroke-miterlimit="10"/>
          <path d="M177.931 184.83C175.792 184.83 173.737 185.693 172.265 187.224C170.848 188.699 170.043 190.675 170.043 192.734C170.071 197.103 173.57 200.61 177.931 200.61C182.292 200.61 185.791 197.075 185.791 192.734C185.791 188.365 182.292 184.83 177.931 184.83Z" stroke="currentColor" stroke-miterlimit="10"/>
          <path d="M77.1375 221.149C72.7769 221.149 69.2773 224.683 69.2773 229.052C69.2773 233.422 72.8047 236.928 77.1375 236.928C81.5259 236.928 85.0255 233.394 85.0255 229.052C85.0255 226.993 84.2478 225.017 82.7758 223.542C81.3037 222.011 79.2762 221.149 77.1375 221.149Z" stroke="currentColor" stroke-miterlimit="10"/>
          <path d="M214.177 221.148C212.038 221.148 209.983 222.011 208.511 223.57C207.095 225.017 206.289 227.02 206.289 229.08C206.289 233.449 209.816 236.956 214.177 236.956C218.538 236.956 222.037 233.421 222.065 229.08C222.065 224.683 218.538 221.148 214.177 221.148Z" stroke="currentColor" stroke-miterlimit="10"/>
        </g>
        <g opacity="0.15">
          <path d="M300.082 4.08276V20.255C300.082 41.0951 316.848 57.8467 337.668 57.8467H353.855V41.6745C353.855 20.8509 337.106 4.08276 316.268 4.08276H300.082Z" stroke="currentColor" stroke-miterlimit="10"/>
          <path d="M397.715 4.08276C376.895 4.08276 360.129 20.8509 360.129 41.6745V57.8467H376.315C397.136 57.8467 413.902 41.0786 413.902 20.255V4.08276H397.715Z" stroke="currentColor" stroke-miterlimit="10"/>
          <path d="M337.668 64.1367C316.848 64.1367 300.082 80.9048 300.082 101.745V117.917H316.268C337.106 117.917 353.855 101.149 353.855 80.3255V64.1533H337.668V64.1367Z" stroke="currentColor" stroke-miterlimit="10"/>
          <path d="M360.129 64.137V80.3092C360.129 101.149 376.895 117.901 397.715 117.901H413.902V101.729C413.902 80.8885 397.136 64.137 376.315 64.137H360.129V64.137Z" stroke="currentColor" stroke-miterlimit="10"/>
          <path d="M316.189 15.4712C314.914 15.4712 313.689 15.9843 312.812 16.8947C311.968 17.772 311.488 18.9473 311.488 20.1722C311.488 22.771 313.59 24.8567 316.189 24.8567C318.787 24.8567 320.872 22.7545 320.872 20.1722C320.872 17.5734 318.771 15.4712 316.189 15.4712Z" stroke="currentColor" stroke-miterlimit="10"/>
          <path d="M397.833 15.4712C395.235 15.4712 393.133 17.5734 393.133 20.1722C393.133 22.771 395.235 24.8567 397.833 24.8567C400.432 24.8567 402.517 22.7545 402.517 20.1722C402.517 18.9473 402.054 17.772 401.193 16.8947C400.299 15.9678 399.091 15.4712 397.833 15.4712Z" stroke="currentColor" stroke-miterlimit="10"/>
          <path d="M337.767 37.0562C336.492 37.0562 335.268 37.5693 334.39 38.4797C333.546 39.357 333.066 40.5323 333.066 41.7572C333.066 44.356 335.168 46.4417 337.767 46.4417C340.365 46.4417 342.451 44.3394 342.467 41.7572C342.467 39.1749 340.365 37.0562 337.767 37.0562Z" stroke="currentColor" stroke-miterlimit="10"/>
          <path d="M376.232 37.0562C373.633 37.0562 371.531 39.1584 371.531 41.7572C371.531 44.356 373.633 46.4417 376.232 46.4417C378.83 46.4417 380.915 44.3394 380.915 41.7572C380.915 40.5323 380.452 39.357 379.591 38.4797C378.714 37.5693 377.489 37.0562 376.232 37.0562Z" stroke="currentColor" stroke-miterlimit="10"/>
          <path d="M337.767 75.5254C335.168 75.5254 333.066 77.6276 333.066 80.2264C333.066 82.8252 335.168 84.9109 337.767 84.9109C340.365 84.9109 342.451 82.8087 342.451 80.2264C342.451 79.0015 341.987 77.8262 341.127 76.9489C340.249 76.0385 339.041 75.5254 337.767 75.5254Z" stroke="currentColor" stroke-miterlimit="10"/>
          <path d="M376.232 75.5254C374.957 75.5254 373.732 76.0385 372.855 76.9489C372.011 77.8262 371.531 79.0015 371.531 80.2264C371.548 82.8252 373.633 84.9109 376.232 84.9109C378.83 84.9109 380.915 82.8087 380.915 80.2264C380.915 77.6276 378.83 75.5254 376.232 75.5254Z" stroke="currentColor" stroke-miterlimit="10"/>
          <path d="M316.172 97.127C313.574 97.127 311.488 99.2292 311.488 101.828C311.488 104.427 313.59 106.512 316.172 106.512C318.787 106.512 320.872 104.41 320.872 101.828C320.872 100.603 320.409 99.4278 319.532 98.5505C318.655 97.6401 317.446 97.127 316.172 97.127Z" stroke="currentColor" stroke-miterlimit="10"/>
          <path d="M397.833 97.127C396.559 97.127 395.334 97.6401 394.457 98.5671C393.613 99.4278 393.133 100.62 393.133 101.845C393.133 104.443 395.235 106.529 397.833 106.529C400.432 106.529 402.517 104.427 402.534 101.845C402.534 99.2292 400.432 97.127 397.833 97.127Z" stroke="currentColor" stroke-miterlimit="10"/>
        </g>
      </svg>
      <div class="pattern-bg__frost" />
      <div class="pattern-bg__grain" />
    </div>

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
      <div class="board-header__content">
        <div>
          <h1>Task Management</h1>
          <p class="board-sub">Local kanban board — drag cards between columns.</p>
        </div>
        <div class="board-header__right">
          <input v-model="query" type="search" class="board-search" placeholder="Search tasks…" />
        </div>
      </div>
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
  position: relative;
  display: flex;
  flex-direction: column;
  height: 100%;
}

/* ── Pattern blur background ── */
.pattern-bg {
  position: absolute;
  inset: 0;
  pointer-events: none;
  overflow: hidden;
  z-index: 0;
}

.pattern-bg__svg {
  position: absolute;
  top: 0;
  right: 0;
  width: 50%;
  height: 80%;
  opacity: 0.5;
  color: var(--accent);
  filter: blur(80px);
  transform: translateX(15%);
}

.pattern-bg__frost {
  position: absolute;
  inset: 0;
  background: var(--bg);
  opacity: 0.7;
  backdrop-filter: blur(40px);
  -webkit-backdrop-filter: blur(40px);
}

.pattern-bg__grain {
  position: absolute;
  inset: 0;
  opacity: 0.015;
  background-image: url("data:image/svg+xml,%3Csvg viewBox='0 0 400 400' xmlns='http://www.w3.org/2000/svg'%3E%3Cfilter id='noiseFilter'%3E%3CfeTurbulence type='fractalNoise' baseFrequency='0.9' numOctaves='4' stitchTiles='stitch'/%3E%3C/filter%3E%3Crect width='100%25' height='100%25' filter='url(%23noiseFilter)'/%3E%3C/svg%3E");
}

.board-header {
  position: relative;
  z-index: 1;
  display: flex;
  flex-direction: column;
  margin-bottom: 16px;
  border-radius: var(--radius-lg);
  background: color-mix(in srgb, var(--accent) 5%, var(--surface));
  border: 1px solid color-mix(in srgb, var(--accent) 12%, var(--border));
  overflow: hidden;
  padding: 22px 24px 16px;
  min-height: 104px;
}

.board-header__bg-mark {
  position: absolute;
  top: -18px;
  right: -18px;
  width: 130px;
  height: 130px;
  color: var(--accent);
  opacity: 0.10;
  pointer-events: none;
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

.board-header__right {
  display: flex;
  align-items: flex-end;
  gap: 8px;
  flex-shrink: 0;
  padding-bottom: 2px;
}

.board-header h1 {
  margin: 0 0 3px;
  font-size: 22px;
}


.board-sub {
  margin: 0;
  font-size: 13px;
  color: var(--ink-muted);
  line-height: 1.4;
  position: relative;
  z-index: 1;
}

.board-search {
  width: 220px;
  padding: 8px 12px;
  border: 1px solid var(--border);
  border-radius: var(--radius-md);
  background: color-mix(in srgb, var(--surface) 80%, transparent);
  backdrop-filter: blur(6px);
  -webkit-backdrop-filter: blur(6px);
  color: var(--ink);
  font: inherit;
  font-size: 13px;
}

.board-search:focus {
  outline: 2px solid var(--accent);
  outline-offset: -1px;
}

.board-error {
  position: relative;
  z-index: 1;
  margin: 0 0 12px;
  padding: 8px 12px;
  border-radius: var(--radius-sm);
  background: var(--danger-soft);
  color: var(--danger);
  font-size: 12px;
}

.add-row {
  position: relative;
  z-index: 1;
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
  position: relative;
  z-index: 1;
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
  background: color-mix(in srgb, var(--surface) 85%, transparent);
  backdrop-filter: blur(8px);
  -webkit-backdrop-filter: blur(8px);
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
  background: color-mix(in srgb, var(--bg) 85%, transparent);
  backdrop-filter: blur(4px);
  -webkit-backdrop-filter: blur(4px);
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
