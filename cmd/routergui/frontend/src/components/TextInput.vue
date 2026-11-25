<script setup lang="ts">
import FormField from "./FormField.vue";

defineProps<{
  id: string;
  label: string;
  modelValue: string | number;
  hint?: string;
  error?: string;
  required?: boolean;
  disabled?: boolean;
  readonly?: boolean;
  mono?: boolean;
  placeholder?: string;
  type?: "text" | "number";
  min?: number;
  max?: number;
}>();
const emit = defineEmits<{ "update:modelValue": [value: string] }>();
</script>

<template>
  <FormField :id="id" :label="label" :hint="hint" :error="error" :required="required">
    <div class="rt-field">
      <input
        :id="id"
        class="rt-field__control"
        :class="{ mono }"
        :value="modelValue"
        :type="type || 'text'"
        :inputmode="type === 'number' ? 'numeric' : undefined"
        :min="min"
        :max="max"
        :placeholder="placeholder"
        :disabled="disabled"
        :readonly="readonly"
        spellcheck="false"
        :aria-describedby="error ? `${id}-error` : hint ? `${id}-hint` : undefined"
        :aria-invalid="Boolean(error) || undefined"
        :aria-required="required || undefined"
        @input="emit('update:modelValue', ($event.target as HTMLInputElement).value)"
      />
    </div>
  </FormField>
</template>
