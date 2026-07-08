---
name: ui-ux-design
description: Guides usability heuristics, design token systems, typography scales, interaction states, and dark mode implementation. Use when auditing an interface for usability problems, defining a design token system, choosing a typography scale, specifying hover/active/disabled/focus states for a component, or building a dark mode theme. Use when a design review needs a structured checklist rather than subjective opinion.
category: Design
---

# UI/UX Design

## Overview

UI/UX design here means making interfaces predictable, legible, and consistent through systematic tools rather than one-off decisions: usability heuristics for evaluation, design tokens for consistency, a typography scale for hierarchy, defined interaction states for every control, and a dark mode that isn't an afterthought.

## When to Use

- Auditing an existing screen or flow for usability issues
- Defining or extending a design token system (color, spacing, radius, elevation)
- Choosing type sizes and weights for a new component or page
- Specifying what a button/input/card looks like in every state, not just default
- Building or reviewing dark mode support
- Reviewing a design or implementation before it ships

## Workflow

### 1. Audit with Nielsen's usability heuristics

Ten heuristics, applied as a checklist against any screen or flow:

1. **Visibility of system status** — does the user always know what's happening (loading, saved, error)?
2. **Match with the real world** — does language and metaphor match how users actually think about the task?
3. **User control and freedom** — can the user undo, cancel, or back out of an action?
4. **Consistency and standards** — does this screen behave like other screens in the product and like platform conventions?
5. **Error prevention** — does the design prevent the mistake, not just report it after?
6. **Recognition over recall** — are options visible, not something the user has to remember from an earlier screen?
7. **Flexibility and efficiency of use** — are there shortcuts for expert users without penalizing novices?
8. **Aesthetic and minimalist design** — is every element on screen earning its place, or is it noise?
9. **Help users recognize, diagnose, and recover from errors** — is the error message specific and actionable, not "something went wrong"?
10. **Help and documentation** — if help is needed, is it findable in context, not buried?

Run this as a pass/fail table per screen, not a vague impression — vague impressions don't survive stakeholder disagreement.

### 2. Build a design token system, not hardcoded values

Tokens are named values that represent design decisions once, referenced everywhere, so a single change propagates instead of requiring a find-and-replace across the codebase.

```json
{
  "color": {
    "surface": { "default": "#ffffff", "raised": "#f7f7f8", "sunken": "#eeeef0" },
    "text": { "primary": "#111827", "secondary": "#6b7280", "inverse": "#ffffff" },
    "action": { "default": "#2563eb", "hover": "#1d4ed8", "active": "#1e40af", "disabled": "#93c5fd" },
    "feedback": { "success": "#16a34a", "warning": "#d97706", "danger": "#dc2626" }
  },
  "space": { "xs": "4px", "sm": "8px", "md": "16px", "lg": "24px", "xl": "32px", "2xl": "48px" },
  "radius": { "sm": "4px", "md": "8px", "lg": "16px", "full": "9999px" },
  "elevation": { "1": "0 1px 2px rgba(0,0,0,0.08)", "2": "0 4px 8px rgba(0,0,0,0.12)" }
}
```

Rules:
- Name tokens by role (`action.default`), not by literal value (`blue.500`) — role-based names survive a rebrand; value-based names don't.
- Never hardcode a hex value or pixel spacing directly in component code once a token exists for that role.
- Every new color needs a documented use case before being added — an uncontrolled token list becomes as messy as no system at all.

### 3. Use a deliberate typography scale

Pick a ratio and generate sizes from it rather than choosing sizes ad hoc per screen:

| Token | Size (px, 1.25 ratio from 16px base) | Use |
|---|---|---|
| `text.xs` | 12.8 → 13 | Captions, metadata |
| `text.sm` | 16 (base, or 14 if base is smaller) | Body copy, secondary UI |
| `text.md` | 16-18 | Default body text |
| `text.lg` | 20 | Section subheadings |
| `text.xl` | 25 | Page/section headings |
| `text.2xl` | 31 | Primary page title |
| `text.3xl` | 39 | Hero/marketing headline |

Rules:
- Line height decreases as font size increases (tight for large display text, looser for small body text) — roughly 1.2 for headings, 1.5 for body.
- Never use more than 2-3 font weights in one interface; more weights add visual noise without adding hierarchy.
- Line length for body text: 45-75 characters per line for readability — constrain with `max-width`, not by hoping the container is narrow.

### 4. Specify every interaction state, not just default

Every interactive component needs a defined look for each of these states — skipping any of them is a common source of "feels unfinished":

| State | Trigger | Common mistake |
|---|---|---|
| Default | Resting | — |
| Hover | Pointer over (desktop only — don't rely on hover for critical info on touch) | Ignored on touch-first flows where it never fires |
| Focus | Keyboard focus | Removed via `outline: none` with no replacement |
| Active/pressed | Mid-click/tap | Missing, feels unresponsive |
| Disabled | Action unavailable | Looks identical to default, or gives no reason why |
| Loading | Action in progress | Button clickable again mid-submit, causing duplicate submits |
| Error | Validation or system failure | Generic red border with no message |
| Selected | Toggled/chosen state (tabs, chips, list items) | Indistinguishable from hover |

```css
.button {
  background: var(--color-action-default);
}
.button:hover { background: var(--color-action-hover); }
.button:active { background: var(--color-action-active); }
.button:focus-visible { outline: 2px solid var(--color-action-default); outline-offset: 2px; }
.button:disabled { background: var(--color-action-disabled); cursor: not-allowed; }
.button[aria-busy="true"] { cursor: progress; }
```

### 5. Design dark mode as a token remap, not a separate design

If tokens are role-based (step 2), dark mode is a second value set for the same token names — not a parallel design system.

```json
{
  "color": {
    "surface": { "default": "#111213", "raised": "#1c1d1f", "sunken": "#0a0a0b" },
    "text": { "primary": "#f5f5f6", "secondary": "#a1a1aa", "inverse": "#111213" }
  }
}
```

Dark mode rules:
- Don't just invert lightness on every color — desaturate and adjust brand colors slightly in dark mode; fully saturated bright colors vibrate against a dark background.
- Elevation in dark mode is usually shown with a lighter surface color per elevation level, not a stronger shadow — shadows barely read on dark backgrounds.
- Test text contrast ratios independently in both themes; a ratio that passes in light mode does not automatically pass in dark mode.
- Respect `prefers-color-scheme` as the default, with an explicit override stored per user — don't force one mode.

## Checklist

- [ ] Screen or flow evaluated against all 10 usability heuristics, not spot-checked
- [ ] All color, spacing, radius, and elevation values reference tokens, no hardcoded literals
- [ ] Typography sizes come from a defined scale, not ad hoc per-screen choices
- [ ] Every interactive component has default, hover, focus, active, disabled, loading, and error states specified
- [ ] Dark mode is a token remap on the existing system, tested for contrast independently
- [ ] Error messages are specific and actionable, not generic

## Common Rationalizations

| Rationalization | Reality |
|---|---|
| "We'll add loading/error states later" | They're the states users actually hit during real usage — "later" ships a broken-feeling default. |
| "One hex value here is fine, it's just this one spot" | Every ad hoc value is a future inconsistency and a future dark-mode gap. |
| "Dark mode is just inverted colors" | Naive inversion produces poor contrast and vibrating saturated colors; it needs its own token pass. |
| "Users don't use keyboard navigation" | Keyboard and screen-reader users always exist; and `:focus-visible` benefits sighted keyboard users too (power users, RSI). |

## Red Flags

- Design files or code with raw hex values instead of token references
- Buttons/inputs with no visible focus state
- A single font weight and size used for both body text and headings
- Dark mode that was generated by an automatic filter/invert rather than designed

## Verification

- [ ] Contrast ratios meet WCAG AA (4.5:1 body text, 3:1 large text) in both light and dark themes
- [ ] Every component state is visually distinct from every other state
- [ ] Token file is the single source of truth referenced by both design tool and code
- [ ] Heuristic audit findings have owners and are tracked, not just noted and forgotten
