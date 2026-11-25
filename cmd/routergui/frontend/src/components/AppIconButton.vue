<script setup lang="ts">
import { LoaderCircle } from "lucide-vue-next";

const props = defineProps<{ label: string; disabled?: boolean; loading?: boolean }>();
const emit = defineEmits<{ click: [event: MouseEvent] }>();
function onClick(event: MouseEvent) {
  if (!props.loading && !props.disabled) emit("click", event);
}
</script>

<template>
  <button
    type="button"
    class="app-icon-button"
    :aria-label="label"
    :title="label"
    :disabled="disabled || loading"
    :aria-busy="loading || undefined"
    @click="onClick"
  >
    <LoaderCircle v-if="loading" class="app-icon-button__spinner" :size="16" aria-hidden="true" />
    <slot v-else />
  </button>
</template>

<style scoped>
.app-icon-button {
  display: inline-grid;
  flex: none;
  place-items: center;
  width: var(--rt-control-height-compact);
  height: var(--rt-control-height-compact);
  padding: 0;
  color: var(--rt-fg);
  background: var(--rt-field);
  border: var(--rt-border-width) solid var(--rt-border);
  border-radius: var(--rt-r-control);
}
.app-icon-button:hover:not(:disabled) {
  background: var(--rt-hover);
}
.app-icon-button:active:not(:disabled) {
  background: var(--rt-pressed);
}
.app-icon-button:disabled {
  opacity: 0.45;
}
.app-icon-button__spinner {
  animation: rt-icon-spin 1s linear infinite;
}
@keyframes rt-icon-spin {
  to {
    transform: rotate(360deg);
  }
}
@media (pointer: coarse) {
  .app-icon-button {
    width: var(--rt-touch-target);
    height: var(--rt-touch-target);
  }
}
</style>
