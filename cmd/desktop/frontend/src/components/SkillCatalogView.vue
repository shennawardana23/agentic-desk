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

onMounted(() => core.loadSkills())

// Frontmatter category tabs (falls back to 'General' when a skill has
// none), sorted alphabetically with General pinned last since it's the
// catch-all rather than a real category. Built from the full unfiltered
// list so the tab bar doesn't reshuffle while the user types a search.
const categories = computed(() => {
  const set = new Set(core.skills.map((s) => s.category || 'General'))
  return [...set].sort((a, b) => {
    if (a === 'General') return 1
    if (b === 'General') return -1
    return a.localeCompare(b)
  })
})
const activeCategory = ref('all')

const filtered = computed(() => {
  const q = query.value.trim().toLowerCase()
  return core.skills.filter((s) => {
    if (activeCategory.value !== 'all' && (s.category || 'General') !== activeCategory.value) return false
    if (!q) return true
    return s.name.toLowerCase().includes(q) || s.description.toLowerCase().includes(q)
  })
})

async function open(name) {
  openName.value = name
  openContent.value = ''
  openError.value = ''
  openLoading.value = true
  try {
    openContent.value = await core.loadSkillContent(name)
  } catch (err) {
    openError.value = String(err)
  } finally {
    openLoading.value = false
  }
}
</script>

<template>
  <section class="catalog">
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

    <header class="view-head">
      <!-- bg watermark -->
      <svg class="view-head__bg-mark" viewBox="0 0 131 131" fill="none" xmlns="http://www.w3.org/2000/svg">
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
      <div class="view-head__content">
        <div>
          <h1>Skill Catalog</h1>
          <p class="view-sub">Agent skills shipped with this desk — browse &amp; explore.</p>
        </div>
        <div class="view-head__right">
          <input v-model="query" type="search" class="filter" placeholder="Filter skills…" />
        </div>
      </div>
    </header>

    <p v-if="core.skillsError" class="view-error" role="alert">{{ core.skillsError }}</p>

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

    <p v-if="!core.loadingSkills && filtered.length === 0" class="view-note">No skills match.</p>

    <div class="card-grid">
      <button v-for="s in filtered" :key="s.name" type="button" class="item-card" @click="open(s.name)">
        <h3>{{ s.name }}</h3>
        <p>{{ s.description || 'No description.' }}</p>
      </button>
    </div>

    <CatalogItemModal
      v-if="openName"
      kind="skill"
      :name="openName"
      :content="openContent"
      :loading="openLoading"
      :error="openError"
      @close="openName = ''"
    />
  </section>
</template>

<style scoped>
.catalog {
  position: relative;
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
  color: var(--accent-ai);
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

.view-head {
  position: relative;
  z-index: 1;
  display: flex;
  flex-direction: column;
  margin-bottom: 20px;
  border-radius: var(--radius-lg);
  background: color-mix(in srgb, #0d9488 7%, var(--surface));
  border: 1px solid color-mix(in srgb, #0d9488 18%, var(--border));
  overflow: hidden;
  padding: 22px 24px 18px;
  min-height: 104px;
}

.view-head__bg-mark {
  position: absolute;
  top: -20px;
  right: -20px;
  width: 140px;
  height: 140px;
  color: #0d9488;
  opacity: 0.13;
  pointer-events: none;
}

.view-head__content {
  position: relative;
  z-index: 1;
  display: flex;
  align-items: flex-end;
  justify-content: space-between;
  gap: 16px;
}

.view-head__content > div:first-child {
  min-width: 0;
  flex: 1;
}

.view-head__right {
  display: flex;
  align-items: flex-end;
  gap: 8px;
  flex-shrink: 0;
  padding-bottom: 2px;
}

.view-head h1 {
  margin: 0 0 3px;
  font-size: 22px;
}


.view-sub {
  margin: 0;
  font-size: 13px;
  color: var(--ink-muted);
  position: relative;
  z-index: 1;
  line-height: 1.4;
}

.view-error {
  position: relative;
  z-index: 1;
  margin: 0 0 12px;
  padding: 8px 12px;
  border-radius: var(--radius-sm);
  background: var(--danger-soft);
  color: var(--danger);
  font-size: 12px;
}

.view-note {
  position: relative;
  z-index: 1;
  font-size: 13px;
  color: var(--ink-faint);
}

.filter {
  width: 220px;
  padding: 9px 12px;
  border: 1px solid var(--border);
  border-radius: var(--radius-md);
  background: color-mix(in srgb, var(--surface) 80%, transparent);
  backdrop-filter: blur(6px);
  -webkit-backdrop-filter: blur(6px);
  color: var(--ink);
  font: inherit;
  font-size: 13px;
}

.filter:focus {
  outline: 2px solid #0d9488;
  outline-offset: -1px;
}

.category-tabs {
  position: relative;
  z-index: 1;
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
  background: color-mix(in srgb, #0d9488 12%, transparent);
  color: #0d9488;
}

.card-grid {
  position: relative;
  z-index: 1;
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(240px, 1fr));
  gap: 12px;
}

.item-card {
  padding: 14px;
  border: 1px solid var(--border);
  border-radius: var(--radius-md);
  background: color-mix(in srgb, var(--surface) 80%, transparent);
  backdrop-filter: blur(4px);
  -webkit-backdrop-filter: blur(4px);
  text-align: left;
  cursor: pointer;
  transition: background-color 150ms var(--ease-out-expo), border-color 150ms var(--ease-out-expo);
}

.item-card:hover {
  background: color-mix(in srgb, #0d9488 8%, var(--surface));
  border-color: #0d9488;
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
