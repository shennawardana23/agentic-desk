import { defineConfig } from 'vite'
import vue from '@vitejs/plugin-vue'

// WEB_CORE_PORT matches Makefile's WEB_CORE_PORT default (9317).
// When running `make web`, the frontend dev server proxies all API +
// WebSocket requests to the Go backend — no cross-origin issues, no
// CORS preflight, no Chrome autoplay-policy fights from different origins.
const corePort = process.env.WEB_CORE_PORT || '9317'
const coreTarget = `http://127.0.0.1:${corePort}`
const wsTarget   = `ws://127.0.0.1:${corePort}`

export default defineConfig({
  plugins: [vue()],
  server: {
    proxy: {
      // Voice Live WebSocket — binary audio frames + JSON control frames.
      // ws:true enables WebSocket proxying; rewriteWsOrigin ensures the
      // upgrade request carries the backend's own origin so CheckOrigin passes.
      '/voice': {
        target: wsTarget,
        ws: true,
        changeOrigin: true,
      },
      // General event-hub WebSocket
      '/ws': {
        target: wsTarget,
        ws: true,
        changeOrigin: true,
      },
      // All other API calls (REST + agent-live WebSocket stream)
      '/api': {
        target: coreTarget,
        ws: true,           // enables /api/agent-live/sessions/:id/stream WS upgrade
        changeOrigin: true,
      },
      // Skill/prompt catalog static files served by Go
      '/skills': {
        target: coreTarget,
        changeOrigin: true,
      },
      '/prompts': {
        target: coreTarget,
        changeOrigin: true,
      },
      // Health + config endpoints
      '/chat': {
        target: coreTarget,
        changeOrigin: true,
      },
    },
  },
})
