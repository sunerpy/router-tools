<script setup lang="ts">
import { ref } from "vue";
import { LoaderCircle } from "lucide-vue-next";

const button = ref<HTMLButtonElement | null>(null);
defineExpose({ focus: () => button.value?.focus() });

const props = withDefaults(
  defineProps<{
    variant?: "primary" | "secondary" | "ghost" | "danger";
    size?: "md" | "sm";
    loading?: boolean;
    disabled?: boolean;
    type?: "button" | "submit" | "reset";
    block?: boolean;
  }>(),
  { variant: "secondary", size: "md", type: "button", block: false },
);

const emit = defineEmits<{ click: [event: MouseEvent] }>();
function onClick(event: MouseEvent) {
  if (!props.loading && !props.disabled) emit("click", event);
}
</script>

<template>
  <button
    ref="button"
    class="app-button"
    :class="[`app-button--${variant}`, `app-button--${size}`, { 'app-button--block': block }]"
    :type="type"
    :disabled="disabled || loading"
    :aria-busy="loading || undefined"
    @click="onClick"
  >
    <LoaderCircle v-if="loading" class="app-button__spinner" :size="16" aria-hidden="true" />
    <slot />
  </button>
</template>

<style scoped>
.app-button {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  gap: var(--rt-space-2);
  min-height: var(--rt-control-height);
  padding: 0 var(--rt-space-4);
  border: var(--rt-border-width) solid transparent;
  border-radius: var(--rt-r-control);
  font-weight: var(--rt-weight-strong);
  white-space: nowrap;
}
.app-button--sm {
  min-height: var(--rt-control-height-compact);
  padding: 0 var(--rt-space-3);
  font-size: var(--rt-text-sm);
}
.app-button--primary {
  color: var(--rt-primary-fg);
  background: var(--rt-primary);
}
.app-button--primary:hover:not(:disabled) {
  background: color-mix(in srgb, var(--rt-primary) 86%, var(--rt-canvas));
}
.app-button--primary:active:not(:disabled) {
  background: color-mix(in srgb, var(--rt-primary) 74%, var(--rt-canvas));
}
.app-button--secondary {
  color: var(--rt-fg);
  background: var(--rt-field);
  border-color: var(--rt-border-strong);
}
.app-button--secondary:hover:not(:disabled),
.app-button--ghost:hover:not(:disabled) {
  background: var(--rt-hover);
}
.app-button--secondary:active:not(:disabled),
.app-button--ghost:active:not(:disabled) {
  background: var(--rt-pressed);
}
.app-button--ghost {
  color: var(--rt-fg);
  background: transparent;
}
.app-button--danger {
  color: var(--rt-danger-fg);
  background: var(--rt-danger);
}
.app-button--danger:hover:not(:disabled) {
  background: color-mix(in srgb, var(--rt-danger) 86%, var(--rt-fg));
}
.app-button--danger:active:not(:disabled) {
  background: color-mix(in srgb, var(--rt-danger) 74%, var(--rt-fg));
}
.app-button:disabled {
  opacity: 0.45;
}
.app-button--block {
  width: 100%;
}
.app-button__spinner {
  animation: rt-spin 1s linear infinite;
}
@keyframes rt-spin {
  to {
    transform: rotate(360deg);
  }
}
@media (pointer: coarse) {
  .app-button {
    min-height: var(--rt-touch-target);
  }
}
/* DESIGN.md §2: at 480 px and below primary and destructive actions fill the width. */
@media (max-width: 480px) {
  .app-button--primary,
  .app-button--danger {
    flex: 1 1 100%;
    width: 100%;
  }
}
</style>
