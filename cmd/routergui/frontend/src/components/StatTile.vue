<script setup lang="ts">
import type { Component } from "vue";
import type { Tone } from "../services/summary";

withDefaults(
  defineProps<{
    tone?: Tone | "highlight";
    title: string;
    meta?: string;
    value: string;
    detail?: string;
    icon?: Component;
  }>(),
  { tone: "neutral", meta: "", detail: "", icon: undefined },
);
</script>

<template>
  <section class="stat-tile" :class="`stat-tile--${tone}`" :data-tone="tone">
    <div class="stat-tile__head">
      <span class="stat-tile__chip" aria-hidden="true">
        <component :is="icon" v-if="icon" :size="16" />
      </span>
      <strong class="stat-tile__title">{{ title }}</strong>
      <span v-if="meta" class="stat-tile__meta mono">{{ meta }}</span>
    </div>
    <p class="stat-tile__value">{{ value }}</p>
    <p v-if="detail" class="stat-tile__detail">{{ detail }}</p>
    <slot />
  </section>
</template>

<style scoped>
.stat-tile {
  --tile-hue: var(--rt-fg-muted);
  --tile-chip-bg: var(--rt-inset-strong);
  --tile-chip-fg: var(--rt-fg-muted);
  --tile-muted: var(--rt-fg-muted);
  display: grid;
  align-content: start;
  gap: var(--rt-space-2);
  min-width: 0;
  padding: var(--rt-space-4) var(--rt-card-inset);
  color: var(--rt-fg);
  background: color-mix(in srgb, var(--tile-hue) var(--rt-tile-tint), var(--rt-surface));
  border: var(--rt-border-width) solid
    color-mix(in srgb, var(--tile-hue) var(--rt-tile-edge), var(--rt-surface));
  border-radius: var(--rt-r-panel);
}
.stat-tile--ok {
  --tile-hue: var(--rt-ok);
  --tile-chip-bg: var(--rt-ok-soft);
  --tile-chip-fg: var(--rt-ok-text);
}
.stat-tile--info {
  --tile-hue: var(--rt-accent);
  --tile-chip-bg: var(--rt-accent-soft);
  --tile-chip-fg: var(--rt-accent-text);
}
.stat-tile--warn {
  --tile-hue: var(--rt-warning);
  --tile-chip-bg: var(--rt-warning-soft);
  --tile-chip-fg: var(--rt-warning-text);
}
.stat-tile--danger {
  --tile-hue: var(--rt-danger);
  --tile-chip-bg: var(--rt-danger-soft);
  --tile-chip-fg: var(--rt-danger-text);
}
.stat-tile--highlight {
  --tile-chip-bg: var(--rt-highlight-chip);
  --tile-chip-fg: var(--rt-highlight-icon);
  --tile-muted: var(--rt-highlight-muted);
  color: var(--rt-highlight-fg);
  background:
    radial-gradient(
      circle at 88% 12%,
      color-mix(in srgb, var(--rt-accent) var(--rt-highlight-glow), transparent),
      transparent 70%
    ),
    linear-gradient(135deg, var(--rt-highlight-from), var(--rt-highlight-to));
  border-color: var(--rt-highlight-border);
}
.stat-tile__head {
  display: flex;
  align-items: center;
  gap: var(--rt-space-2);
  min-width: 0;
}
.stat-tile__chip {
  display: grid;
  flex: none;
  place-items: center;
  width: var(--rt-chip-size);
  height: var(--rt-chip-size);
  border-radius: var(--rt-r-panel);
  color: var(--tile-chip-fg);
  background: var(--tile-chip-bg);
}
.stat-tile__title {
  min-width: 0;
  font-weight: var(--rt-weight-strong);
}
.stat-tile__meta {
  margin-left: auto;
  color: var(--tile-muted);
  font-size: var(--rt-text-xs);
}
.stat-tile__value {
  margin: var(--rt-space-1) 0 0;
  font-size: var(--rt-text-2xl);
  font-weight: var(--rt-weight-strong);
  line-height: var(--rt-leading-tight);
  overflow-wrap: anywhere;
}
.stat-tile__detail {
  margin: 0;
  color: var(--tile-muted);
  font-size: var(--rt-text-xs);
  overflow-wrap: anywhere;
}
</style>
