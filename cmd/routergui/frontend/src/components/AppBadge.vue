<script setup lang="ts">
import type { Component } from "vue";
import { AlertTriangle, CheckCircle2, Info, MinusCircle, XCircle } from "lucide-vue-next";
import type { Tone } from "../services/summary";

withDefaults(defineProps<{ tone?: Tone; icon?: Component; spin?: boolean }>(), {
  tone: "neutral",
  icon: undefined,
  spin: false,
});
const defaults: Record<Tone, Component> = {
  ok: CheckCircle2,
  warn: AlertTriangle,
  danger: XCircle,
  info: Info,
  neutral: MinusCircle,
};
</script>

<template>
  <span class="app-badge" :class="`app-badge--${tone}`" :data-tone="tone">
    <component
      :is="icon ?? defaults[tone]"
      :size="14"
      :class="{ 'app-badge__spin': spin }"
      aria-hidden="true"
    />
    <slot />
  </span>
</template>

<style scoped>
.app-badge {
  display: inline-flex;
  flex: none;
  align-items: center;
  gap: var(--rt-space-1);
  min-height: var(--rt-badge-height);
  padding: 0 var(--rt-space-3) 0 var(--rt-space-2);
  border-radius: var(--rt-r-pill);
  font-size: var(--rt-text-xs);
  font-weight: var(--rt-weight-medium);
  line-height: var(--rt-leading-tight);
  white-space: nowrap;
}
.app-badge--ok {
  color: var(--rt-ok-text);
  background: var(--rt-ok-soft);
}
.app-badge--warn {
  color: var(--rt-warning-text);
  background: var(--rt-warning-soft);
}
.app-badge--danger {
  color: var(--rt-danger-text);
  background: var(--rt-danger-soft);
}
.app-badge--info {
  color: var(--rt-accent-text);
  background: var(--rt-accent-soft);
}
.app-badge--neutral {
  color: var(--rt-fg-muted);
  background: var(--rt-inset-strong);
}
.app-badge__spin {
  animation: rt-badge-spin 1s linear infinite;
}
@keyframes rt-badge-spin {
  to {
    transform: rotate(360deg);
  }
}
</style>
