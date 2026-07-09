import { createPinia } from 'pinia'
import { createApp } from 'vue'
import App from './App.vue'
import './style.css'
import { useCoreStore } from './stores/core'

// Dev-only Wails shim: lets `npm run dev` render the whole app in a plain
// browser (no Wails runtime) against a separately-run cmd/core — needed
// to browser-test/screenshot the UI at all. Point it elsewhere via
// localStorage['agentic-desk-dev-core-url']. import.meta.env.DEV is false
// in production builds, so this is stripped from the packaged app.
if (import.meta.env.DEV && !window.go) {
  window.go = {
    main: {
      App: {
        // Empty string = same-origin. Vite proxy forwards /voice, /ws, /api
        // to WEB_CORE_PORT (9317). No cross-origin, no CORS, no blocked WS.
        CoreAPIURL: async () =>
          localStorage.getItem('agentic-desk-dev-core-url') || '',
        CoreStartupError: async () => '',
      },
    },
  }
}

const pinia = createPinia()
const app = createApp(App).use(pinia)

// Child components' onMounted (ProfileView.loadProfile, MemorySearch)
// fire before App.vue's own onMounted — baseUrl must already be set
// by the time any of them mount, or their fetch() calls resolve
// against the Wails asset-server origin instead of cmd/core.
await useCoreStore(pinia).init()

app.mount('#app')
