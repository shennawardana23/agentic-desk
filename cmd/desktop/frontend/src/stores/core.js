import { defineStore } from 'pinia'
import { CoreAPIURL, CoreStartupError } from '../../wailsjs/go/main/App'

// useCoreStore centralizes every fetch to cmd/core's Phase 9 HTTP
// API. Components never fetch() directly — this store is the single
// place that knows the API's base URL and response shapes, so a
// backend route change only touches one file.
export const useCoreStore = defineStore('core', {
  state: () => ({
    baseUrl: '',
    profileRules: [],
    loadingProfile: false,
    profileError: '',
    memoryResults: [],
    searchingMemory: false,
    memoryError: '',
    memorySearched: false,
    projectContext: null,
    loadingProjectContext: false,
    projectContextError: '',
    // chatMessages holds {role: 'user'|'agent', content, imageDataUrl?,
    // reasoning?, pending?, stopped?} turns, in order. role/content/
    // imageDataUrl match /chat's wire shape; reasoning (the model's
    // streamed chain-of-thought), pending (still streaming) and stopped
    // (user hit stop) are UI-only and stripped before being sent back
    // as history.
    chatMessages: [],
    sendingChat: false,
    chatError: '',
    // chatAbort is the in-flight stream's AbortController — the stop
    // button calls stopChat(), which aborts the fetch; the dropped
    // connection cancels the server-side generation via request context.
    chatAbort: null,
    // Chat Sessions (/chat/sessions CRUD) — persistence for chatMessages is
    // best-effort: a session is created lazily on the first message of a
    // fresh chat (ensureChatSession), then every completed turn is appended
    // to it (persistChatMessage). Failures here must never break chatting —
    // see the catch blocks below, which console.warn and move on.
    chatSessions: [],
    loadingChatSessions: false,
    chatSessionsError: '',
    currentChatSessionId: null,
    // Task Management (/tasks CRUD).
    tasks: [],
    loadingTasks: false,
    tasksError: '',
    // Knowledge Graph (GET /graph).
    graphData: null,
    loadingGraph: false,
    graphError: '',
    // Skill Catalog + Prompt Catalog (browse-only filesystem reads).
    skills: [],
    loadingSkills: false,
    skillsError: '',
    prompts: [],
    loadingPrompts: false,
    promptsError: '',
    // Voice Assistant: {role:'user'|'agent', content} turns. Audio goes
    // up as a data URL through the same /chat multimodal path images use.
    voiceMessages: [],
    sendingVoice: false,
    voiceError: '',
    // Legacy alias some older callers/tests may read.
    error: '',
  }),
  actions: {
    async init() {
      // CoreAPIURL blocks (on the Go side) until the auto-launched
      // cmd/core either answers or gives up — see cmd/desktop/app.go.
      // If it gave up, surface why on Chat (the default screen, whose
      // error banner is already rendered unconditionally) instead of
      // every view failing later with a generic, causeless fetch error.
      this.baseUrl = await CoreAPIURL()
      const startupError = await CoreStartupError()
      if (startupError) this.chatError = this.error = `Core failed to start: ${startupError}`
    },
    // sendChatMessage streams from cmd/core's /chat/stream SSE route
    // (real Gemini generation via internal/orchestrator's streaming chat
    // flow). Reasoning ("thinking") chunks and reply text chunks arrive
    // incrementally and are appended live onto a pending agent message;
    // the terminal "done" event carries the authoritative full reply.
    // imageDataUrl, if given, is a full "data:<mime>;base64,..." URL from
    // a FileReader read — sent as real multimodal input (image or audio),
    // not just acknowledged.
    async sendChatMessage(content, imageDataUrl) {
      const history = this.chatMessages
        .filter((m) => !m.pending)
        .map((m) => ({
          role: m.role,
          content: m.content,
          imageDataUrl: m.imageDataUrl || undefined,
        }))
      this.chatMessages.push({ role: 'user', content, imageDataUrl })
      // Best-effort persistence — fire-and-forget, never blocks sending.
      this.persistChatMessage('user', content, '')
      const agentMsg = { role: 'agent', content: '', reasoning: '', pending: true, stopped: false }
      this.chatMessages.push(agentMsg)
      this.sendingChat = true
      this.chatError = ''
      const controller = new AbortController()
      this.chatAbort = controller
      try {
        const res = await fetch(`${this.baseUrl}/chat/stream`, {
          method: 'POST',
          headers: { 'Content-Type': 'application/json' },
          body: JSON.stringify({ history, message: content, imageDataUrl }),
          signal: controller.signal,
        })
        if (!res.ok || !res.body) throw new Error(`chat: ${res.status}`)
        const reader = res.body.getReader()
        const decoder = new TextDecoder()
        let buffer = ''
        for (;;) {
          const { done, value } = await reader.read()
          if (done) break
          buffer += decoder.decode(value, { stream: true })
          // SSE frames are separated by a blank line; each frame here is
          // a single "data: <json>" line (see router.go's /chat/stream).
          const frames = buffer.split('\n\n')
          buffer = frames.pop()
          for (const frame of frames) {
            const line = frame.trim()
            if (!line.startsWith('data:')) continue
            const event = JSON.parse(line.slice(5))
            if (event.type === 'reasoning') agentMsg.reasoning += event.content
            else if (event.type === 'text') agentMsg.content += event.content
            else if (event.type === 'done') agentMsg.content = event.reply
            else if (event.type === 'error') throw new Error(event.error)
          }
        }
      } catch (err) {
        if (err?.name === 'AbortError') {
          // User pressed stop: keep whatever partial reply/reasoning
          // arrived, honestly marked — not an error.
          agentMsg.stopped = true
        } else {
          this.chatError = this.error = String(err)
        }
      } finally {
        agentMsg.pending = false
        // A stop/error before anything arrived leaves an empty shell
        // bubble — drop it rather than render a blank agent turn.
        if (!agentMsg.content && !agentMsg.reasoning) {
          const i = this.chatMessages.indexOf(agentMsg)
          if (i >= 0) this.chatMessages.splice(i, 1)
        } else {
          // Best-effort persistence — fire-and-forget, never blocks the UI.
          this.persistChatMessage('agent', agentMsg.content, agentMsg.reasoning)
        }
        this.sendingChat = false
        this.chatAbort = null
      }
    },
    // stopChat aborts the in-flight streamed generation — the GUI's stop
    // button. Server-side generation is cancelled through the dropped
    // request's context (verified: /chat/stream passes c.Request.Context()
    // into the flow).
    stopChat() {
      this.chatAbort?.abort()
    },
    // retryLastChatMessage re-sends the most recent user turn after a
    // generation error (see ChatView.vue's error banner) — removes the
    // old copy first so sendChatMessage's fresh push doesn't duplicate it.
    retryLastChatMessage() {
      for (let i = this.chatMessages.length - 1; i >= 0; i--) {
        if (this.chatMessages[i].role === 'user') {
          const { content, imageDataUrl } = this.chatMessages[i]
          this.chatMessages.splice(i, 1)
          this.sendChatMessage(content, imageDataUrl)
          return
        }
      }
    },
    // loadChatSessions fetches the session list for the History drawer.
    async loadChatSessions() {
      this.loadingChatSessions = true
      this.chatSessionsError = ''
      try {
        const res = await fetch(`${this.baseUrl}/chat/sessions`)
        if (!res.ok) throw new Error(`chat sessions: ${res.status}`)
        // Router wraps the list: {"sessions": [...]} (see internal/api/router.go).
        const data = await res.json()
        this.chatSessions = data.sessions ?? []
      } catch (err) {
        this.chatSessionsError = String(err)
        console.warn('[core] loadChatSessions failed:', err)
      } finally {
        this.loadingChatSessions = false
      }
    },
    // ensureChatSession lazily creates a session on the first message of a
    // fresh chat. Best-effort: a failure here still lets the turn stream —
    // it just won't be saved to history.
    async ensureChatSession() {
      if (this.currentChatSessionId) return this.currentChatSessionId
      try {
        const res = await fetch(`${this.baseUrl}/chat/sessions`, { method: 'POST' })
        if (!res.ok) throw new Error(`create session: ${res.status}`)
        const session = await res.json()
        this.currentChatSessionId = session.id
        this.chatSessions.unshift(session)
        return session.id
      } catch (err) {
        console.warn('[core] ensureChatSession failed:', err)
        return null
      }
    },
    // persistChatMessage best-effort appends a completed turn to the
    // current session (creating one first if needed). Never throws.
    async persistChatMessage(role, content, reasoning) {
      try {
        const sessionId = await this.ensureChatSession()
        if (!sessionId) return
        const res = await fetch(`${this.baseUrl}/chat/sessions/${sessionId}/messages`, {
          method: 'POST',
          headers: { 'Content-Type': 'application/json' },
          body: JSON.stringify({ role, content, reasoning: reasoning || '' }),
        })
        if (!res.ok) throw new Error(`append message: ${res.status}`)
      } catch (err) {
        console.warn('[core] persistChatMessage failed:', err)
      }
    },
    // loadChatSession replaces the live conversation with a saved
    // session's messages (History drawer → click a session).
    async loadChatSession(sessionId) {
      this.chatSessionsError = ''
      try {
        const res = await fetch(`${this.baseUrl}/chat/sessions/${sessionId}/messages`)
        if (!res.ok) throw new Error(`load session: ${res.status}`)
        // Router wraps the list: {"messages": [...]} (see internal/api/router.go).
        const msgs = (await res.json()).messages ?? []
        this.chatMessages = msgs.map((m) => ({
          role: m.role === 'user' ? 'user' : 'agent',
          content: m.content,
          reasoning: m.reasoning || '',
          pending: false,
          stopped: false,
        }))
        this.currentChatSessionId = sessionId
      } catch (err) {
        this.chatSessionsError = String(err)
        console.warn('[core] loadChatSession failed:', err)
      }
    },
    // newChat clears the live conversation (History drawer's "New Chat"
    // button) — the next message lazily creates a fresh session.
    newChat() {
      this.chatAbort?.abort()
      this.chatMessages = []
      this.currentChatSessionId = null
    },
    // renameChatSession renames a session's title (History drawer's kebab
    // menu → Rename).
    async renameChatSession(sessionId, title) {
      try {
        const res = await fetch(`${this.baseUrl}/chat/sessions/${sessionId}`, {
          method: 'PATCH',
          headers: { 'Content-Type': 'application/json' },
          body: JSON.stringify({ title }),
        })
        if (!res.ok) throw new Error(`rename session: ${res.status}`)
        const updated = await res.json()
        const i = this.chatSessions.findIndex((s) => s.id === sessionId)
        if (i >= 0) this.chatSessions[i] = updated
      } catch (err) {
        this.chatSessionsError = String(err)
        console.warn('[core] renameChatSession failed:', err)
      }
    },
    // deleteChatSession removes a session (History drawer's kebab menu →
    // Delete). Clears the live conversation too if it was the one deleted.
    async deleteChatSession(sessionId) {
      try {
        const res = await fetch(`${this.baseUrl}/chat/sessions/${sessionId}`, { method: 'DELETE' })
        if (!res.ok && res.status !== 204) throw new Error(`delete session: ${res.status}`)
        this.chatSessions = this.chatSessions.filter((s) => s.id !== sessionId)
        if (this.currentChatSessionId === sessionId) this.newChat()
      } catch (err) {
        this.chatSessionsError = String(err)
        console.warn('[core] deleteChatSession failed:', err)
      }
    },
    async loadProfile() {
      this.loadingProfile = true
      this.profileError = ''
      try {
        const res = await fetch(`${this.baseUrl}/profile`)
        if (!res.ok) throw new Error(`profile: ${res.status}`)
        const data = await res.json()
        this.profileRules = data.rules ?? []
      } catch (err) {
        this.profileError = this.error = String(err)
      } finally {
        this.loadingProfile = false
      }
    },
    async searchMemory(query) {
      this.searchingMemory = true
      this.memoryError = ''
      try {
        const res = await fetch(`${this.baseUrl}/memory/search?query=${encodeURIComponent(query)}`)
        if (!res.ok) throw new Error(`memory search: ${res.status}`)
        const data = await res.json()
        this.memoryResults = data.entries ?? []
        this.memorySearched = true
      } catch (err) {
        this.memoryError = this.error = String(err)
      } finally {
        this.searchingMemory = false
      }
    },
    async loadTasks() {
      this.loadingTasks = true
      this.tasksError = ''
      try {
        const res = await fetch(`${this.baseUrl}/tasks`)
        if (!res.ok) throw new Error(`tasks: ${res.status}`)
        const data = await res.json()
        this.tasks = data.tasks ?? []
      } catch (err) {
        this.tasksError = this.error = String(err)
      } finally {
        this.loadingTasks = false
      }
    },
    async createTask(title, notes = '', priority = 0) {
      this.tasksError = ''
      try {
        const res = await fetch(`${this.baseUrl}/tasks`, {
          method: 'POST',
          headers: { 'Content-Type': 'application/json' },
          body: JSON.stringify({ title, notes, priority }),
        })
        if (!res.ok) throw new Error(`create task: ${res.status}`)
        this.tasks.push(await res.json())
      } catch (err) {
        this.tasksError = this.error = String(err)
      }
    },
    async updateTask(task) {
      this.tasksError = ''
      try {
        const res = await fetch(`${this.baseUrl}/tasks/${task.id}`, {
          method: 'PUT',
          headers: { 'Content-Type': 'application/json' },
          body: JSON.stringify({
            title: task.title,
            notes: task.notes,
            description: task.description ?? '',
            status: task.status,
            priority: task.priority,
          }),
        })
        if (!res.ok) throw new Error(`update task: ${res.status}`)
        const updated = await res.json()
        const i = this.tasks.findIndex((t) => t.id === updated.id)
        if (i >= 0) this.tasks[i] = updated
      } catch (err) {
        this.tasksError = this.error = String(err)
      }
    },
    async deleteTask(id) {
      this.tasksError = ''
      try {
        const res = await fetch(`${this.baseUrl}/tasks/${id}`, { method: 'DELETE' })
        if (!res.ok && res.status !== 204) throw new Error(`delete task: ${res.status}`)
        this.tasks = this.tasks.filter((t) => t.id !== id)
      } catch (err) {
        this.tasksError = this.error = String(err)
      }
    },
    async loadGraph() {
      this.loadingGraph = true
      this.graphError = ''
      try {
        const res = await fetch(`${this.baseUrl}/graph`)
        if (!res.ok) throw new Error(`graph: ${res.status}`)
        this.graphData = await res.json()
      } catch (err) {
        this.graphError = this.error = String(err)
      } finally {
        this.loadingGraph = false
      }
    },
    async loadSkills() {
      this.loadingSkills = true
      this.skillsError = ''
      try {
        const res = await fetch(`${this.baseUrl}/skills`)
        if (!res.ok) throw new Error(`skills: ${res.status}`)
        const data = await res.json()
        this.skills = data.skills ?? []
      } catch (err) {
        this.skillsError = this.error = String(err)
      } finally {
        this.loadingSkills = false
      }
    },
    // loadSkillContent returns the markdown body (detail views keep their
    // own "open item" state; the list stays in the store).
    async loadSkillContent(name) {
      const res = await fetch(`${this.baseUrl}/skills/${encodeURIComponent(name)}`)
      if (!res.ok) throw new Error(`skill: ${res.status}`)
      return (await res.json()).content
    },
    async loadPrompts() {
      this.loadingPrompts = true
      this.promptsError = ''
      try {
        const res = await fetch(`${this.baseUrl}/prompts`)
        if (!res.ok) throw new Error(`prompts: ${res.status}`)
        const data = await res.json()
        this.prompts = data.prompts ?? []
      } catch (err) {
        this.promptsError = this.error = String(err)
      } finally {
        this.loadingPrompts = false
      }
    },
    async loadPromptContent(name) {
      const res = await fetch(`${this.baseUrl}/prompts/${encodeURIComponent(name)}`)
      if (!res.ok) throw new Error(`prompt: ${res.status}`)
      return (await res.json()).content
    },
    // sendVoiceMessage ships recorded audio (a data:<mime>;base64,... URL
    // from MediaRecorder) through the same non-streaming /chat multimodal
    // path images use — the backend's media-part handling is mime-agnostic,
    // so Gemini receives real audio and transcribes+answers in one call.
    //
    // The model is instructed to reply with strict JSON containing both the
    // verbatim transcript and the reply, so the transcript panel can show
    // what the user actually said instead of a placeholder. The response is
    // parsed defensively (markdown code fences stripped, JSON.parse
    // wrapped in try/catch) because a multimodal model can still ignore the
    // format instruction; on parse failure we fall back to treating the
    // whole response as the reply and leave the user bubble as a generic
    // placeholder.
    //
    // customInstructions (optional) is VoiceView's persona/system-instructions
    // textarea content — the caller passes it in rather than this store
    // reading localStorage directly, so the UI owns its own persisted state.
    async sendVoiceMessage(audioDataUrl, customInstructions) {
      const history = this.voiceMessages.map((m) => ({ role: m.role, content: m.content }))
      const userMsgIndex = this.voiceMessages.push({ role: 'user', content: 'Voice message' }) - 1
      this.sendingVoice = true
      this.voiceError = ''
      try {
        const preamble = customInstructions?.trim() ? `${customInstructions.trim()}\n\n` : ''
        const res = await fetch(`${this.baseUrl}/chat`, {
          method: 'POST',
          headers: { 'Content-Type': 'application/json' },
          body: JSON.stringify({
            history,
            message:
              preamble +
              'The attached audio is a spoken voice message from the user. ' +
              'Reply with STRICT JSON and nothing else — no markdown, no code fences, no commentary — ' +
              'in exactly this shape: {"transcript":"<verbatim transcription of the user\'s audio, in its original language>","reply":"<the assistant\'s brief answer in that language>"}',
            imageDataUrl: audioDataUrl,
          }),
        })
        if (!res.ok) throw new Error(`voice: ${res.status}`)
        const data = await res.json()
        const raw = String(data.reply ?? '')
        const cleaned = raw
          .trim()
          .replace(/^```(?:json)?\s*/i, '')
          .replace(/```\s*$/, '')
        let transcript = 'Voice message'
        let reply = raw
        try {
          const parsed = JSON.parse(cleaned)
          if (parsed && typeof parsed.transcript === 'string' && typeof parsed.reply === 'string') {
            transcript = parsed.transcript || 'Voice message'
            reply = parsed.reply
          }
        } catch {
          // Fall back to current behavior: whole text as reply, placeholder stays.
        }
        this.voiceMessages[userMsgIndex].content = transcript
        this.voiceMessages.push({ role: 'agent', content: reply })
        return reply
      } catch (err) {
        this.voiceError = this.error = String(err)
        return ''
      } finally {
        this.sendingVoice = false
      }
    },
    async loadProjectContext(projectPath) {
      this.loadingProjectContext = true
      this.projectContextError = ''
      try {
        const res = await fetch(`${this.baseUrl}/project-context?projectPath=${encodeURIComponent(projectPath)}`)
        if (!res.ok) throw new Error(`project context: ${res.status}`)
        const data = await res.json()
        this.projectContext = data.found ? { projectPath, summary: data.summary } : null
      } catch (err) {
        this.projectContextError = this.error = String(err)
      } finally {
        this.loadingProjectContext = false
      }
    },
    async saveProjectContext(projectPath, summary) {
      this.projectContextError = ''
      try {
        const res = await fetch(`${this.baseUrl}/project-context`, {
          method: 'PUT',
          headers: { 'Content-Type': 'application/json' },
          body: JSON.stringify({ projectPath, summary }),
        })
        if (!res.ok) throw new Error(`save project context: ${res.status}`)
        this.projectContext = { projectPath, summary }
      } catch (err) {
        this.projectContextError = this.error = String(err)
      }
    },
  },
})
