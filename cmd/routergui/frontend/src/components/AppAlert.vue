<script setup lang="ts">
import { AlertTriangle, Info, OctagonAlert } from "lucide-vue-next";

withDefaults(
  defineProps<{
    type?: "info" | "warning" | "error";
    title: string;
    message: string;
  }>(),
  { type: "info" },
);
const icons = { info: Info, warning: AlertTriangle, error: OctagonAlert };
</script>

<template>
  <aside
    class="app-alert"
    :class="`app-alert--${type}`"
    :role="type === 'error' ? 'alert' : 'status'"
  >
    <component :is="icons[type]" class="app-alert__icon" :size="20" aria-hidden="true" />
    <div class="app-alert__body">
      <strong class="app-alert__title">{{ title }}</strong>
      <p class="app-alert__message">{{ message }}</p>
      <slot />
    </div>
  </aside>
</template>

<style scoped>
.app-alert {
  --alert-bg: var(--rt-accent-soft);
  --alert-ink: var(--rt-accent-text);
  display: flex;
  gap: var(--rt-space-3);
  padding: var(--rt-space-4) var(--rt-card-inset);
  color: var(--rt-fg);
  background: var(--alert-bg);
  border: var(--rt-border-width) solid color-mix(in srgb, var(--alert-ink) 30%, transparent);
  border-radius: var(--rt-r-panel);
}
.app-alert--warning {
  --alert-bg: var(--rt-warning-soft);
  --alert-ink: var(--rt-warning-text);
}
.app-alert--error {
  --alert-bg: var(--rt-danger-soft);
  --alert-ink: var(--rt-danger-text);
}
.app-alert__icon {
  margin-top: 1px;
  color: var(--alert-ink);
}
.app-alert__body {
  min-width: 0;
}
.app-alert__title {
  color: var(--alert-ink);
  font-weight: var(--rt-weight-strong);
}
.app-alert__message {
  margin: var(--rt-space-1) 0 0;
  white-space: pre-line;
  overflow-wrap: anywhere;
}
</style>
