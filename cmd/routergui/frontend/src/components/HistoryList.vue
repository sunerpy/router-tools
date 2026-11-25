<script setup lang="ts">
import type { Component } from "vue";
import {
  Activity,
  Database,
  FileClock,
  HeartPulse,
  KeyRound,
  RotateCcw,
  ShieldAlert,
  ShieldCheck,
} from "lucide-vue-next";
import StatusBadge from "./StatusBadge.vue";
import type { OperationRow } from "../services/summary";

defineProps<{ rows: OperationRow[]; selectable?: boolean; selected?: string }>();
const emit = defineEmits<{ select: [id: string] }>();

const ICONS: Record<string, Component> = {
  doctor: HeartPulse,
  verify: KeyRound,
  enable: ShieldCheck,
  "persist soft": Activity,
  "persist remove": Activity,
  "reboot-verify": RotateCcw,
  "backup create": Database,
  "backup verify": Database,
  "backup verify online": Database,
  "deep-persist": ShieldAlert,
};
const TONES: Record<string, string> = {
  succeeded: "ok",
  partial: "warn",
  unknown: "warn",
  failed: "danger",
};
</script>

<template>
  <ul class="history-list">
    <li v-for="row in rows" :key="row.id">
      <component
        :is="selectable ? 'button' : 'div'"
        class="history-list__row"
        :type="selectable ? 'button' : undefined"
        :aria-current="selectable && row.id === selected ? 'true' : undefined"
        @click="selectable && emit('select', row.id)"
      >
        <span
          class="history-list__chip"
          :class="`history-list__chip--${TONES[row.status] ?? 'neutral'}`"
          aria-hidden="true"
        >
          <component :is="ICONS[row.operation] ?? FileClock" :size="18" />
        </span>
        <span class="history-list__text">
          <span class="history-list__title">{{ row.title }}</span>
          <span class="history-list__meta mono">{{ row.meta }}</span>
        </span>
        <span class="history-list__end">
          <StatusBadge :status="row.status" />
          <slot name="actions" :row="row" />
        </span>
      </component>
    </li>
  </ul>
</template>

<style scoped>
.history-list {
  display: grid;
  padding: 0;
  margin: 0;
  list-style: none;
}
.history-list li + li {
  border-top: var(--rt-border-width) solid var(--rt-border);
}
.history-list__row {
  display: grid;
  grid-template-columns: auto minmax(0, 1fr) auto;
  align-items: center;
  gap: var(--rt-space-3);
  width: 100%;
  min-height: var(--rt-row-height);
  padding: var(--rt-space-2) var(--rt-space-3);
  color: var(--rt-fg);
  background: transparent;
  border: 0;
  border-radius: var(--rt-r-control);
  text-align: left;
}
button.history-list__row:hover {
  background: var(--rt-hover);
}
.history-list__row[aria-current="true"] {
  background: var(--rt-nav-active);
  box-shadow: inset var(--rt-indicator-width) 0 0 var(--rt-accent-text);
}
.history-list__row[aria-current="true"] .history-list__title {
  font-weight: var(--rt-weight-strong);
}
.history-list__chip {
  display: grid;
  place-items: center;
  width: var(--rt-chip-size-lg);
  height: var(--rt-chip-size-lg);
  border-radius: var(--rt-r-panel);
  color: var(--rt-fg-muted);
  background: var(--rt-inset-strong);
}
.history-list__chip--ok {
  color: var(--rt-ok-text);
  background: var(--rt-ok-soft);
}
.history-list__chip--warn {
  color: var(--rt-warning-text);
  background: var(--rt-warning-soft);
}
.history-list__chip--danger {
  color: var(--rt-danger-text);
  background: var(--rt-danger-soft);
}
.history-list__text {
  display: grid;
  min-width: 0;
}
.history-list__title {
  font-weight: var(--rt-weight-medium);
}
.history-list__meta {
  color: var(--rt-fg-muted);
  font-size: var(--rt-text-xs);
  overflow-wrap: anywhere;
}
.history-list__end {
  display: flex;
  flex-wrap: wrap;
  justify-content: flex-end;
  align-items: center;
  gap: var(--rt-space-2);
}
@media (max-width: 480px) {
  .history-list__row {
    grid-template-columns: auto minmax(0, 1fr);
  }
  .history-list__end {
    grid-column: 2;
    justify-content: flex-start;
  }
}
</style>
