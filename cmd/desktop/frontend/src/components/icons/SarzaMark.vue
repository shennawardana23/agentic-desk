<!--
  Inline copy of brand/icon-chat-sarza.svg (repo master left untouched).
  The master hardcodes fill/stroke="#4A005F" (dark purple), invisible in
  dark mode — every shape here uses currentColor instead.

  Unlike BrandMark, this component does NOT fix its own `color` — it's used
  as the Chat nav-item icon (AppSidebar), which must recolor on hover/active
  the same way its sibling <span v-html> icons do (via `color: currentColor`
  inherited from .nav-item / .nav-item.is-active). Fixing color here would
  silently break that reactivity, so leave it inheriting.
-->
<template>
  <svg class="sarza-mark-icon" viewBox="0 0 64 64" xmlns="http://www.w3.org/2000/svg" role="img" aria-hidden="true">
    <!-- Antenna -->
    <circle cx="32" cy="7" r="4" fill="currentColor"/>
    <line x1="32" y1="10" x2="32" y2="14" stroke="currentColor" stroke-width="3" stroke-linecap="round"/>
    <!-- Face — identical to icon-chat.svg -->
    <rect x="6" y="14" width="52" height="38" rx="10" fill="none" stroke="currentColor" stroke-width="3"/>
    <!-- Forehead mark — icon-desktop.svg's snowflake, scaled down and
         centered between the antenna base and the eye line. -->
    <g transform="translate(25 16) scale(0.107)" fill="none" stroke="currentColor" stroke-width="1.8" stroke-miterlimit="10">
      <path d="M53.7421 59.6211L29.8369 60.0105L2 32.1736L25.9052 31.7842L53.7421 59.6211Z"/>
      <path d="M59.6211 53.7421L60.0105 29.8183L32.1736 2L31.7842 25.9052L59.6211 53.7421Z"/>
      <path d="M77.2579 71.3789L101.163 70.9895L129 98.8264L105.095 99.2158L77.2579 71.3789Z"/>
      <path d="M71.3789 77.2579L70.9895 101.182L98.8264 129L99.2158 105.095L71.3789 77.2579Z"/>
      <path d="M59.6211 77.2579L60.0105 101.163L32.1736 129L31.7842 105.095L59.6211 77.2579Z"/>
      <path d="M53.7421 71.3789L29.8369 70.9895L2 98.8264L25.9052 99.2158L53.7421 71.3789Z"/>
      <path d="M71.3789 53.7421L70.9895 29.8369L98.8264 2L99.2158 25.9052L71.3789 53.7421Z"/>
      <path d="M77.2579 59.6211L101.182 60.0105L129 32.1736L105.095 31.7842L77.2579 59.6211Z"/>
      <path d="M69.9324 69.9324H61.0676V61.0676H69.9324V69.9324Z"/>
    </g>
    <!-- Eyes — same as icon-chat.svg -->
    <circle cx="22" cy="36" r="4" fill="currentColor"/>
    <circle cx="42" cy="36" r="4" fill="currentColor"/>
    <!-- Nose -->
    <rect x="30" y="34" width="4" height="4" rx="1" fill="currentColor"/>
  </svg>
</template>

<style scoped>
.sarza-mark-icon {
  display: block;
  width: 100%;
  height: 100%;
}
</style>
