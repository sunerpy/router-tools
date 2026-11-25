<script setup lang="ts">
import { Check } from "lucide-vue-next";

defineProps<{ steps: string[]; current: number }>();
</script>

<template>
  <ol class="app-stepper" aria-label="步骤">
    <li
      v-for="(step, index) in steps"
      :key="step"
      class="app-stepper__step"
      :data-state="index + 1 < current ? 'done' : index + 1 === current ? 'current' : 'pending'"
      :aria-current="index + 1 === current ? 'step' : undefined"
    >
      <span class="app-stepper__marker" aria-hidden="true">
        <Check v-if="index + 1 < current" :size="14" />
        <template v-else>{{ index + 1 }}</template>
      </span>
      <span class="app-stepper__label">{{ step }}</span>
      <span v-if="index + 1 < current" class="sr-only">（已完成）</span>
    </li>
  </ol>
</template>

<style scoped>
.app-stepper {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(min(100%, var(--rt-step-min)), 1fr));
  gap: var(--rt-space-2);
  padding: 0;
  margin: 0;
  list-style: none;
}
.app-stepper__step {
  display: flex;
  align-items: center;
  gap: var(--rt-space-2);
  min-width: 0;
  min-height: var(--rt-control-height);
  padding: var(--rt-space-2) var(--rt-space-3);
  color: var(--rt-fg-muted);
  background: var(--rt-inset);
  border: var(--rt-border-width) solid var(--rt-border);
  border-radius: var(--rt-r-control);
  font-size: var(--rt-text-sm);
}
.app-stepper__step[data-state="done"] {
  color: var(--rt-fg);
}
.app-stepper__step[data-state="current"] {
  color: var(--rt-fg);
  background: var(--rt-surface);
  border-color: var(--rt-accent-text);
  font-weight: var(--rt-weight-strong);
}
.app-stepper__marker {
  display: grid;
  flex: none;
  place-items: center;
  width: var(--rt-space-5);
  height: var(--rt-space-5);
  border-radius: var(--rt-r-pill);
  color: var(--rt-fg-muted);
  background: var(--rt-inset-strong);
  font-size: var(--rt-text-xs);
  font-weight: var(--rt-weight-strong);
}
.app-stepper__step[data-state="done"] .app-stepper__marker {
  color: var(--rt-ok-text);
  background: var(--rt-ok-soft);
}
.app-stepper__step[data-state="current"] .app-stepper__marker {
  color: var(--rt-primary-fg);
  background: var(--rt-primary);
}
.app-stepper__label {
  min-width: 0;
  overflow-wrap: anywhere;
}
</style>
