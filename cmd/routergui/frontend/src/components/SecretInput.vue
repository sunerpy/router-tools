<script setup lang="ts">
import { ref } from "vue";
import { Eye, EyeOff } from "lucide-vue-next";
import FormField from "./FormField.vue";

defineProps<{
  id: string;
  label: string;
  modelValue: string;
  hint?: string;
  error?: string;
  required?: boolean;
  disabled?: boolean;
}>();
const emit = defineEmits<{ "update:modelValue": [value: string] }>();
const visible = ref(false);
</script>

<template>
  <FormField :id="id" :label="label" :hint="hint" :error="error" :required="required">
    <div class="rt-field">
      <input
        :id="id"
        class="rt-field__control secret-input__control"
        :value="modelValue"
        :type="visible ? 'text' : 'password'"
        autocomplete="current-password"
        autocapitalize="off"
        spellcheck="false"
        :disabled="disabled"
        :aria-describedby="error ? `${id}-error` : hint ? `${id}-hint` : undefined"
        :aria-invalid="Boolean(error) || undefined"
        :aria-required="required || undefined"
        @input="emit('update:modelValue', ($event.target as HTMLInputElement).value)"
      />
      <button
        class="secret-input__toggle"
        type="button"
        :aria-label="visible ? '隐藏密码' : '显示密码'"
        :disabled="disabled"
        @click="visible = !visible"
      >
        <EyeOff v-if="visible" :size="16" aria-hidden="true" />
        <Eye v-else :size="16" aria-hidden="true" />
      </button>
    </div>
  </FormField>
</template>

<style scoped>
.secret-input__control {
  font-family: var(--rt-font-mono);
}
.secret-input__toggle {
  display: grid;
  flex: 0 0 var(--rt-control-height);
  place-items: center;
  width: var(--rt-control-height);
  height: var(--rt-control-height);
  margin: calc(-1 * var(--rt-border-width));
  color: var(--rt-fg-muted);
  background: transparent;
  border: 0;
  border-radius: var(--rt-r-control);
}
@media (pointer: coarse) {
  .secret-input__toggle {
    flex-basis: var(--rt-touch-target);
    width: var(--rt-touch-target);
    height: var(--rt-touch-target);
  }
}
.secret-input__toggle:hover:not(:disabled) {
  color: var(--rt-fg);
  background: var(--rt-hover);
}
</style>
