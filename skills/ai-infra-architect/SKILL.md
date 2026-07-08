---
name: ai-infra-architect
description: Guides LLM serving infrastructure design including model routing, fallback chains, cost controls, observability, and vector store selection. Use when choosing between self-hosted and managed model serving, designing a multi-model routing or fallback strategy, setting token spend limits, instrumenting LLM call latency and cost, or selecting a vector database for retrieval workloads.
category: AI
---

# AI Infrastructure Architecture

## Overview

AI infrastructure architecture is the plumbing that makes LLM features reliable and affordable at scale: which model serves a given request, what happens when that model is slow or down, how spend is bounded, what gets measured, and where vectors live. This skill covers model routing/fallback design, cost controls, observability, and vector store selection — the layer below prompt and application logic.

## When to Use

- Designing how requests are routed across multiple LLM providers or model tiers
- Building a fallback chain for provider outages or rate limits
- Setting per-user, per-feature, or org-wide token/cost budgets
- Instrumenting latency, token usage, and cost per LLM call
- Choosing or sizing a vector store for RAG or semantic search
- Reviewing an AI feature's production readiness before launch

## Workflow

### 1. Design model routing around task shape, not habit

Route by what the task actually needs, not by defaulting to the largest available model:

| Task shape | Route to | Why |
|---|---|---|
| Classification, extraction, short structured output | Small/fast model | Latency and cost dominate; accuracy ceiling is already high |
| Long-form generation, complex reasoning | Frontier model | Quality difference is material and worth the cost |
| High-volume, latency-sensitive (autocomplete, tagging) | Smallest model that meets a quality bar, cached aggressively | Cost scales with volume |
| Low-volume, high-stakes (contract review, medical) | Frontier model, possibly with a second model as a checker | Cost of an error exceeds cost of the call |

```go
type ModelTier int

const (
    TierFast ModelTier = iota // cheap, low-latency, for classification/extraction
    TierStandard
    TierFrontier // complex reasoning, long-form generation
)

func RouteModel(task TaskType) ModelTier {
    switch task {
    case TaskClassify, TaskExtract:
        return TierFast
    case TaskSummarize, TaskDraft:
        return TierStandard
    case TaskReasonComplex:
        return TierFrontier
    default:
        return TierStandard
    }
}
```

### 2. Build fallback chains, not single points of failure

Every production LLM call needs a defined behavior for: timeout, rate limit, 5xx, and content-policy rejection. Define the chain explicitly rather than letting a single provider outage take down the feature.

```go
type FallbackChain struct {
    Steps []ModelStep
}

type ModelStep struct {
    Provider string
    Model    string
    Timeout  time.Duration
}

func (c FallbackChain) Execute(ctx context.Context, req Request) (Response, error) {
    var lastErr error
    for _, step := range c.Steps {
        stepCtx, cancel := context.WithTimeout(ctx, step.Timeout)
        resp, err := callModel(stepCtx, step, req)
        cancel()
        if err == nil {
            return resp, nil
        }
        lastErr = err
        // log/metric the fallback event — silent fallbacks hide degradation
        recordFallback(step, err)
    }
    return Response{}, fmt.Errorf("all fallback steps exhausted: %w", lastErr)
}
```

Fallback chain design rules:
- Order steps by preference (quality/cost), not by provider alphabetically.
- Each fallback event must be logged and alerted on above a threshold rate — a fallback chain that's silently absorbing failures hides an outage from the team.
- Cap total chain latency; a chain of three 10-second timeouts is worse than failing fast at 10 seconds.
- Test the fallback path deliberately (inject a forced failure), not just the happy path.

### 3. Put cost controls at every layer that can spend money

- **Per-request cap**: reject or truncate requests whose estimated token cost exceeds a ceiling before sending.
- **Per-user/per-tenant budget**: track cumulative spend and throttle or degrade gracefully (smaller model, cached response) when a tenant approaches its budget.
- **Org-wide alerting**: alert at 50/80/100% of a daily/monthly spend target, not just a hard cutoff that silently breaks the feature.
- **Prompt-level cost audit**: recurring review of system prompts and few-shot examples for unnecessary token weight — a bloated system prompt multiplies cost across every single call.

```go
func EnforceBudget(ctx context.Context, tenantID string, estimatedTokens int, pricePerToken float64) error {
    spend, err := budgetStore.GetMonthlySpend(ctx, tenantID)
    if err != nil {
        return fmt.Errorf("check budget: %w", err)
    }
    estimatedCost := float64(estimatedTokens) * pricePerToken
    if spend+estimatedCost > budgetStore.LimitFor(tenantID) {
        return ErrBudgetExceeded
    }
    return nil
}
```

### 4. Instrument every call, not just errors

Minimum fields to log per LLM call: provider, model, input tokens, output tokens, latency, cost, whether a fallback fired, and a correlation ID tying it back to the originating request/feature.

```go
type LLMCallMetric struct {
    Provider     string
    Model        string
    InputTokens  int
    OutputTokens int
    LatencyMS    int64
    CostUSD      float64
    FallbackUsed bool
    FeatureName  string
    RequestID    string
}
```

Dashboard this by feature and model tier — cost and latency regressions are invisible until aggregated. Alert on: p99 latency, error rate, fallback rate, and daily spend delta, not just raw error counts.

### 5. Choose a vector store by workload shape, not popularity

| Workload | Consider | Why |
|---|---|---|
| Small corpus (<1M vectors), already on PostgreSQL | `pgvector` extension | No new infrastructure; transactional consistency with source data |
| Large corpus, high query volume, need hybrid search | Dedicated vector DB (e.g., Qdrant, Weaviate, Pinecone) | Purpose-built indexing (HNSW/IVF), horizontal scale |
| Frequently changing documents | Store that supports fast upsert/delete, not just append | Stale vectors degrade retrieval quality silently |
| Multi-tenant | Store with native namespace/filter support | Avoid leaking one tenant's documents into another's retrieval |

If already on PostgreSQL and the corpus is small to medium, prefer `pgvector` — it avoids a second system to operate, and keeps vector rows in the same partitioned/transactional model as the rest of the data (see backend-engineering for partition-aware querying).

## Checklist

- [ ] Model routing decision documented per task type, not left to "whichever model was easiest to call first"
- [ ] Fallback chain defined with explicit timeouts and ordering, tested with an injected failure
- [ ] Every fallback event and rate-limit hit is logged and alertable
- [ ] Per-tenant or per-feature spend has a tracked budget with staged alerts
- [ ] Every LLM call emits latency, token, and cost metrics tied to a feature name
- [ ] Vector store choice matches corpus size, update frequency, and multi-tenancy needs

## Common Rationalizations

| Rationalization | Reality |
|---|---|
| "We'll just always call the best model" | Cost scales linearly with volume; most tasks don't need frontier-model reasoning. |
| "One provider is enough, they're reliable" | Every provider has outages and rate limits; a single point of failure becomes a feature outage. |
| "We'll add cost tracking once we see the bill" | By then the spend pattern is already baked into product behavior and harder to change. |
| "pgvector won't scale" | It scales further than most workloads ever reach; validate against real corpus size before adding a new system. |

## Red Flags

- LLM calls with no timeout, retried indefinitely on transient errors
- No metric distinguishing "model responded but fallback fired" from "model responded normally"
- System prompts that have grown for months with no token-cost review
- A vector store chosen because a blog post recommended it, without a workload-shape comparison

## Verification

- [ ] Fallback chain triggers correctly when the primary provider is forced to fail in a test
- [ ] Dashboards show cost and latency per feature, refreshed within the alerting window
- [ ] A synthetic spend spike (e.g., load test) triggers the budget alert before the hard limit
- [ ] Vector store query latency measured under realistic corpus size and concurrency
