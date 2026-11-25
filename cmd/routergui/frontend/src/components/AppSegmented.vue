<script setup lang="ts">
import { useId } from "vue";
import { Check } from "lucide-vue-next";

defineProps<{
  modelValue: string;
  options: Array<{ value: string; label: string }>;
  label: string;
}>();
const emit = defineEmits<{ "update:modelValue": [value: string] }>();
const name = useId();
</script>

<template>
  <div class="app-segmented" role="radiogroup" :aria-label="label">
    <label
      v-for="option in options"
      :key="option.value"
      class="app-segmented__option"
      :class="{ 'app-segmented__option--selected': option.value === modelValue }"
    >
      <input
        class="app-segmented__input"
        type="radio"
        :name="name"
        :value="option.value"
        :checked="option.value === modelValue"
        @change="emit('update:modelValue', option.value)"
      />
      <Check v-if="option.value === modelValue" :size="14" aria-hidden="true" />
      <span>{{ option.label }}</span>
    </label>
  </div>
</template>

<style scoped>
.app-segmented {
  display: inline-flex;
  flex-wrap: wrap;
  justify-self: start;
  max-width: 100%;
  gap: var(--rt-space-1);
  padding: var(--rt-space-1);
  background: var(--rt-inset);
  border: var(--rt-border-width) solid var(--rt-border);
  border-radius: var(--rt-r-control);
}
.app-segmented__option {
  position: relative;
  display: inline-flex;
  align-items: center;
  gap: var(--rt-space-1);
  min-height: var(--rt-control-height-compact);
  padding: 0 var(--rt-space-4);
  border-radius: var(--rt-r-control);
  color: var(--rt-fg-muted);
  font-weight: var(--rt-weight-medium);
  cursor: pointer;
}
.app-segmented__option:hover {
  color: var(--rt-fg);
}
.app-segmented__option--selected {
  color: var(--rt-fg);
  background: var(--rt-surface);
  outline: var(--rt-border-width) solid var(--rt-border-strong);
  font-weight: var(--rt-weight-strong);
}
.app-segmented__option:has(:focus-visible) {
  outline: var(--rt-focus-width) solid var(--rt-focus);
  outline-offset: var(--rt-focus-width);
}
.app-segmented__input {
  position: absolute;
  width: 1px;
  height: 1px;
  margin: 0;
  opacity: 0;
}
@media (pointer: coarse) {
  .app-segmented__option {
    min-height: var(--rt-touch-target);
  }
}
</style>
