<script setup lang="ts">
defineProps<{ modelValue: boolean; disabled?: boolean; note?: string; id?: string }>();
const emit = defineEmits<{ "update:modelValue": [value: boolean] }>();
</script>

<template>
  <label class="app-toggle" :class="{ 'app-toggle--disabled': disabled }">
    <input
      :id="id"
      class="app-toggle__input"
      type="checkbox"
      role="switch"
      :checked="modelValue"
      :disabled="disabled"
      @change="emit('update:modelValue', ($event.target as HTMLInputElement).checked)"
    />
    <span class="app-toggle__track" aria-hidden="true"></span>
    <span class="app-toggle__label"><slot /></span>
    <span v-if="note" class="app-toggle__note">{{ note }}</span>
  </label>
</template>

<style scoped>
.app-toggle {
  position: relative;
  display: inline-flex;
  align-items: center;
  gap: var(--rt-space-2) var(--rt-space-3);
  min-height: var(--rt-control-height-compact);
  cursor: pointer;
}
.app-toggle__input {
  position: absolute;
  width: 1px;
  height: 1px;
  margin: 0;
  opacity: 0;
}
.app-toggle__track {
  position: relative;
  flex: none;
  width: var(--rt-switch-width);
  height: var(--rt-switch-height);
  border-radius: var(--rt-r-pill);
  background: var(--rt-toggle-off);
  transition: background-color var(--rt-motion-fast) var(--rt-motion-ease);
}
.app-toggle__track::after {
  content: "";
  position: absolute;
  top: calc((var(--rt-switch-height) - var(--rt-switch-thumb)) / 2);
  left: calc((var(--rt-switch-height) - var(--rt-switch-thumb)) / 2);
  width: var(--rt-switch-thumb);
  height: var(--rt-switch-thumb);
  border-radius: var(--rt-r-pill);
  background: var(--rt-thumb);
  box-shadow: var(--rt-shadow-thumb);
  transition: transform var(--rt-motion-fast) var(--rt-motion-ease);
}
.app-toggle__input:checked + .app-toggle__track {
  background: var(--rt-toggle-on);
}
.app-toggle__input:checked + .app-toggle__track::after {
  transform: translateX(calc(var(--rt-switch-width) - var(--rt-switch-height)));
}
.app-toggle__input:focus-visible + .app-toggle__track {
  outline: var(--rt-focus-width) solid var(--rt-focus);
  outline-offset: var(--rt-focus-width);
}
.app-toggle__label {
  min-width: 0;
  overflow-wrap: anywhere;
}
.app-toggle__note {
  color: var(--rt-warning-text);
  font-size: var(--rt-text-xs);
  font-weight: var(--rt-weight-medium);
}
.app-toggle--disabled {
  cursor: not-allowed;
  opacity: 0.45;
}
@media (pointer: coarse) {
  .app-toggle {
    min-height: var(--rt-touch-target);
  }
}
</style>
