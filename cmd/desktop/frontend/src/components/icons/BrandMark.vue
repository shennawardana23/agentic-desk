<!--
  Inline copy of brand/icon-desktop.svg (repo master left untouched). The
  master hardcodes stroke="#4A005F" (dark purple), which disappears against
  a dark-mode background — every path here uses stroke="currentColor" so
  the mark reads correctly in both themes.

  Used standalone (sidebar brand button, ChatView hero/thinking marks), not
  inside a nav-item that needs to react to active/hover color changes — so
  it fixes its own color to var(--ink) rather than leaving `color` to
  inherit. If you ever place this inside something that must recolor it
  (like SarzaMark's nav-item usage), override .brand-mark-icon { color: ... }
  from the parent's scoped style, or add a prop here instead of removing
  the default.

  `bold` thickens every path's stroke — at ~16px (e.g. the chat "Thinking"
  indicator) the master's 1-unit stroke anti-aliases down to a near-invisible
  hairline in both themes; a heavier stroke keeps it legible at that size
  without changing the mark's look at its normal 22-28px+ render sizes.
-->
<template>
  <svg class="brand-mark-icon" :class="{ 'is-bold': bold }" viewBox="0 0 131 131" fill="none" xmlns="http://www.w3.org/2000/svg" role="img" aria-hidden="true">
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
</template>

<script setup>
defineProps({
  bold: { type: Boolean, default: false },
})
</script>

<style scoped>
.brand-mark-icon {
  display: block;
  width: 100%;
  height: 100%;
  color: var(--ink);
}

.brand-mark-icon.is-bold path {
  stroke-width: 3.5;
}
</style>
