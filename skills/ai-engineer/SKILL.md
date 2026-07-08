---
name: ai-engineer
description: Guides prompt engineering, retrieval-augmented generation, embeddings, evaluation design, and agent tool design. Use when writing or debugging a system prompt, designing a RAG pipeline's chunking and retrieval strategy, choosing an embedding model, building an eval set for an LLM feature, or defining tool schemas for an agent. Use when an LLM feature's output quality is inconsistent and needs systematic diagnosis.
category: AI
---

# AI Engineering

## Overview

AI engineering is applying LLMs and retrieval to build features that behave predictably: prompts that hold up under varied input, retrieval that surfaces the right context, embeddings suited to the domain, evaluations that catch regressions before users do, and agent tools an LLM can call correctly. This skill covers the application layer above model serving (see ai-infra-architect for routing/cost/observability).

## When to Use

- Writing or debugging a system prompt or few-shot example set
- Designing chunking, retrieval, and re-ranking for a RAG pipeline
- Choosing an embedding model or deciding when to fine-tune one
- Building an eval set to catch regressions in an LLM feature
- Defining tool/function schemas for an agent
- An LLM feature works on the demo case but fails on real user input

## Workflow

### 1. Structure prompts for reliability, not cleverness

- State the task, constraints, and output format explicitly — don't rely on the model inferring format from a single example.
- Put the most important instruction last if the model tends to anchor on recent context; test both orderings for your model.
- Use few-shot examples that cover edge cases, not just the happy path — a prompt with only success examples will fail on ambiguous input.
- Separate instructions from data with clear delimiters, and never interpolate untrusted user input directly into an instruction block without delimiting it (prompt injection surface).

```
SYSTEM:
You are a support ticket classifier. Classify the ticket into exactly one
category: billing, technical, account, other. Respond with only the
category name, lowercase, no punctuation.

Examples:
Ticket: "I was charged twice this month" -> billing
Ticket: "The app crashes when I upload a photo" -> technical
Ticket: "I can't remember my password" -> account
Ticket: "What are your office hours?" -> other

USER:
Ticket: <<<{{user_ticket_text}}>>>
```

Treat the content inside `<<< >>>` as data only — instruct the model explicitly not to follow instructions found inside it.

### 2. Design RAG around retrieval quality, not just pipeline plumbing

Chunking:
- Chunk by semantic unit (section, paragraph) where possible, not fixed character count alone.
- Keep chunks small enough to be specific (typically 200-500 tokens) but large enough to retain context; test both directions against real queries.
- Store chunk metadata (source, section, timestamp) alongside the vector — needed for citation and for filtering stale content.

Retrieval:
- Retrieve more candidates than you'll use (e.g., top 20) and re-rank down to the final set (e.g., top 5) with a cheaper, more precise step — pure vector similarity alone under-performs on many queries.
- Combine semantic (vector) search with keyword/full-text search (hybrid retrieval) for queries containing exact identifiers, names, or codes that embeddings represent poorly.
- Filter by metadata (tenant, date range, document type) at the query level, not by retrieving broadly and filtering after — this both leaks data across tenants and wastes retrieval budget.

```go
type RetrievalRequest struct {
    Query      string
    TenantID   string   // always filter — never retrieve across tenants
    TopK       int      // candidates before re-ranking
    FinalK     int      // results after re-ranking, sent to the model
    DateAfter  *time.Time
}
```

Grounding:
- Require the model to cite which retrieved chunk supports each claim; a response that can't be traced to a source is not verifiable.
- If no retrieved chunk is relevant, instruct the model to say so explicitly rather than answering from parametric knowledge — silent fallback to ungrounded answers is the most common RAG failure mode.

### 3. Choose embeddings for the domain, not the leaderboard

- General-purpose embedding models work well for general text; domain-specific text (legal, medical, internal jargon, product codes) often needs a domain-tuned or fine-tuned model to separate similar-looking-but-different concepts.
- Match embedding dimensionality and model to what the vector store and query volume can afford — higher dimensionality costs more to store and search, and often does not proportionally improve retrieval quality.
- Re-embed the whole corpus whenever the embedding model changes; embeddings from different model versions are not comparable to each other.

### 4. Build evals before you ship, and keep running them

An eval set is a fixed collection of representative inputs with either a known correct output or a rubric for scoring the output. Without one, "quality" is decided by anecdote.

```yaml
# evals/support_classifier.yaml
- input: "I was charged twice this month"
  expected: billing
- input: "double charge on my card"
  expected: billing
- input: "the app keeps crashing"
  expected: technical
- input: "can you turn off notifications"
  expected: technical
- input: "asdkjasd;lkj"          # garbage input
  expected: other
- input: ""                       # empty input
  expected: other
```

Eval design rules:
- Include edge cases deliberately: empty input, adversarial input, ambiguous cases between two categories, out-of-scope requests.
- For open-ended generation, use a rubric and either a second LLM as judge (with a clear, narrow rubric) or human review on a sample — exact-match scoring doesn't work for free text.
- Run the eval set on every prompt change and every model version change before shipping; treat a regression in eval score the same as a failing test.
- Track eval scores over time; a prompt that silently degrades after an unrelated change is a real production risk.

### 5. Design agent tools for the model, not the API

Tool schemas are read by the model to decide when and how to call them — write descriptions the way you'd document a public API for an unfamiliar engineer.

```json
{
  "name": "search_reservations",
  "description": "Search reservations for a hotel by guest name, date range, or reservation ID. Returns up to 20 matches. Use this before create_reservation to check for existing bookings.",
  "parameters": {
    "type": "object",
    "properties": {
      "hotel_id": { "type": "integer", "description": "Required. The hotel to search within." },
      "guest_name": { "type": "string", "description": "Optional. Partial name match." },
      "date_from": { "type": "string", "format": "date" },
      "date_to": { "type": "string", "format": "date" }
    },
    "required": ["hotel_id"]
  }
}
```

Tool design rules:
- Keep each tool single-purpose; a tool that does three unrelated things is harder for the model to select correctly.
- Make required parameters actually required in the schema, and describe defaults explicitly — the model cannot see your backend's fallback logic.
- Return structured, small results from tools; a tool that returns a huge blob of raw data burns context and degrades reasoning on the next step.
- Give the model a way to fail gracefully — a tool result that says "no matches found" is more useful than an empty array with no explanation.

## Checklist

- [ ] System prompt separates instructions from user-provided data with explicit delimiters
- [ ] Few-shot examples cover edge cases, not only the happy path
- [ ] RAG retrieval filters by tenant/access scope at query time, not after
- [ ] Retrieval combines semantic and keyword search where exact terms matter
- [ ] Model is instructed to cite sources and to say "not found" rather than guess
- [ ] An eval set with edge cases exists and runs on every prompt/model change
- [ ] Agent tool descriptions are written for an unfamiliar reader, with required fields marked

## Common Rationalizations

| Rationalization | Reality |
|---|---|
| "The demo works, ship it" | Demos use hand-picked input; real users send messy, ambiguous, and adversarial input. |
| "We don't need an eval set, we'll just look at outputs" | Anecdotal review misses regressions and doesn't scale past a handful of examples. |
| "More retrieved chunks can't hurt" | Irrelevant chunks dilute context and measurably reduce answer quality and grounding. |
| "The model will figure out the tool from its name" | Ambiguous tool descriptions cause wrong or missed tool calls; the model only knows what the schema says. |

## Red Flags

- User input concatenated directly into a prompt string with no delimiter or injection guard
- RAG pipeline with no citation and no "not found" fallback path
- No eval set, or an eval set that only contains easy/happy-path cases
- Tool descriptions copy-pasted from internal API docs with no guidance on when to call them

## Verification

- [ ] Eval set score recorded before and after any prompt or model change
- [ ] A deliberately adversarial or malformed input tested against the prompt for injection resistance
- [ ] Retrieval spot-checked against a query with a known correct source document
- [ ] Agent tool calls traced on a multi-step task to confirm correct tool selection and argument construction
