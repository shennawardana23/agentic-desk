---
name: frontend-engineering
description: Guides component architecture, state management, accessibility, and performance budgets for frontend applications. Use when structuring components, choosing between local and global state, auditing keyboard and screen-reader accessibility, or when a page's bundle size or render time exceeds its budget. Use when reviewing frontend pull requests for prop drilling, unnecessary re-renders, or missing ARIA semantics.
category: Engineering
---

# Frontend Engineering

## Overview

Frontend engineering here is about building UI that stays maintainable as it grows, stays fast as content scales, and works for every user including those on assistive technology. This skill covers component architecture, state management boundaries, accessibility, and performance budgets — the four areas that degrade first in unmanaged frontend codebases.

## When to Use

- Structuring a new feature's component tree
- Deciding whether state belongs local, in context, or in a global store
- Reviewing a component for accessibility gaps
- A page's Lighthouse score, bundle size, or Core Web Vitals regress
- Refactoring a component that has grown too many responsibilities

## Workflow

### 1. Component architecture: separate container from presentation

```tsx
// Container: owns data fetching and state
function ReservationListContainer() {
  const { data, isLoading, error } = useReservations();
  if (isLoading) return <Spinner />;
  if (error) return <ErrorState error={error} />;
  return <ReservationList reservations={data} />;
}

// Presentation: pure, receives props, no data fetching
function ReservationList({ reservations }: { reservations: Reservation[] }) {
  return (
    <ul>
      {reservations.map((r) => (
        <ReservationRow key={r.id} reservation={r} />
      ))}
    </ul>
  );
}
```

Rules of thumb:
- A component that both fetches data and renders complex markup is doing two jobs — split it.
- Presentation components should be testable with plain prop objects, no mocking network calls.
- Compose small components over configuring one large component with many boolean props.

### 2. State management: pick the smallest scope that works

| Scope | Use for | Tool |
|---|---|---|
| Local (`useState`) | UI-only state: open/closed, hover, form input before submit | React built-ins |
| Lifted to parent | State shared by 2-3 sibling components | Prop passing |
| Context | State needed by a subtree, changes infrequently (theme, auth) | `useContext` |
| Global store | State shared across unrelated parts of the app, changes frequently | Store library (Zustand, Redux, etc.) |
| Server cache | Data that originates from an API | Query library (React Query, SWR) — never duplicate this into local/global state |

The most common mistake is skipping straight to a global store for state that only two components need — this creates unnecessary re-render surface and hides the actual data dependency.

```tsx
// Bad: server data duplicated into global store, now two sources of truth
const reservations = useGlobalStore((s) => s.reservations);

// Good: server cache is the source of truth
const { data: reservations } = useQuery(['reservations', hotelId], fetchReservations);
```

### 3. Accessibility: build it in, don't audit it in afterward

Minimum bar for any interactive component:

- Every interactive element is reachable and operable by keyboard alone (Tab, Enter, Space, Arrow keys where applicable)
- Every form input has a programmatically associated label (`<label for>` or `aria-labelledby`)
- Focus is visible (`:focus-visible` styled, never `outline: none` without a replacement)
- Focus moves predictably on route change, modal open/close, and after async actions complete
- Color is never the only signal — pair with icon, text, or pattern
- Images have `alt` text; decorative images have `alt=""`
- Custom components (dropdowns, tabs, modals) follow the corresponding ARIA pattern, not an ad hoc `div` with `onClick`

```tsx
// Bad: div masquerading as a button, invisible to screen readers and keyboard
<div onClick={handleSave}>Save</div>

// Good
<button type="button" onClick={handleSave}>Save</button>
```

Modal focus trap example:

```tsx
function Modal({ onClose, children }: ModalProps) {
  const ref = useRef<HTMLDivElement>(null);
  useEffect(() => {
    ref.current?.focus();
    const onKey = (e: KeyboardEvent) => e.key === 'Escape' && onClose();
    window.addEventListener('keydown', onKey);
    return () => window.removeEventListener('keydown', onKey);
  }, [onClose]);
  return (
    <div role="dialog" aria-modal="true" ref={ref} tabIndex={-1}>
      {children}
    </div>
  );
}
```

### 4. Performance budgets: set numbers, not vibes

Set explicit budgets per route before building, not after shipping:

| Metric | Budget | Why |
|---|---|---|
| JS shipped (gzipped) | < 200 KB per route | Parse/execute cost on mid-tier mobile |
| Largest Contentful Paint | < 2.5s | Core Web Vital, SEO and perceived speed |
| Interaction to Next Paint | < 200ms | Perceived responsiveness |
| Cumulative Layout Shift | < 0.1 | Visual stability |

Techniques to stay under budget:
- Code-split by route; lazy-load anything below the fold or behind an interaction.
- Memoize expensive derived values (`useMemo`) only after profiling shows the cost — memoizing everything adds overhead without benefit.
- Virtualize long lists (>100 rows) instead of rendering all rows.
- Audit third-party scripts; each one is unaccountable bundle weight and a CLS risk.

```tsx
// Route-level code splitting
const ReservationDetail = lazy(() => import('./ReservationDetail'));
```

## Checklist

- [ ] Container/presentation split — presentation components take plain props and have no data-fetching side effects
- [ ] State lives at the smallest scope that satisfies all consumers
- [ ] Server data lives in the query cache, not duplicated into local/global state
- [ ] Every interactive control is keyboard-operable and has a visible focus state
- [ ] Custom widgets (modal, dropdown, tabs) implement the matching ARIA pattern
- [ ] Route has a defined JS/LCP/INP/CLS budget and it is measured, not assumed
- [ ] Long lists are virtualized; below-the-fold content is lazy-loaded

## Common Rationalizations

| Rationalization | Reality |
|---|---|
| "It's just an internal tool, accessibility doesn't matter" | Internal tools get used by people with disabilities too, and requirements change; retrofitting is far more expensive. |
| "We'll optimize performance once it's a problem" | By the time users complain, the architecture is load-bearing and expensive to change. |
| "Global store is simpler than deciding scope" | It's simpler to write, not simpler to reason about — every consumer now re-renders on unrelated changes. |
| "One div with onClick is fine, it looks like a button" | Screen readers and keyboard users don't get "looks like" — they get the DOM semantics you actually wrote. |

## Red Flags

- Components with more than one reason to re-render for unrelated data
- `useEffect` used to sync state that could be derived during render
- Server-fetched data copied into `useState` and manually kept in sync
- `tabIndex` values other than `0` or `-1` (positive values break natural tab order)
- Bundle analyzer never run, or run once and never checked again

## Verification

- [ ] Bundle analyzer report shows the route under its JS budget
- [ ] Lighthouse or equivalent shows LCP/INP/CLS within budget on throttled mobile profile
- [ ] Keyboard-only pass through the feature reaches and operates every control
- [ ] Screen reader pass (VoiceOver/NVDA) announces labels, roles, and state changes correctly
- [ ] No console warnings about missing keys, unhandled promise rejections, or accessibility violations
