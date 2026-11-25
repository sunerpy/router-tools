<script setup lang="ts">
defineProps<{
  id: string;
  label: string;
  hint?: string;
  error?: string;
  required?: boolean;
}>();
</script>

<template>
  <div class="form-field">
    <label :for="id" class="form-field__label">
      {{ label }}<span v-if="required" class="form-field__required" aria-hidden="true"> *</span>
    </label>
    <slot :described-by="`${id}-hint ${id}-error`" />
    <p v-if="hint" :id="`${id}-hint`" class="form-field__hint">{{ hint }}</p>
    <p v-if="error" :id="`${id}-error`" class="form-field__error" role="alert">{{ error }}</p>
  </div>
</template>

<style scoped>
.form-field {
  display: grid;
  align-content: start;
  gap: var(--rt-space-2);
  min-width: 0;
}
.form-field__label {
  color: var(--rt-fg);
  font-size: var(--rt-text-sm);
  font-weight: var(--rt-weight-medium);
}
.form-field__required {
  color: var(--rt-fg-muted);
}
.form-field__hint,
.form-field__error {
  margin: 0;
  font-size: var(--rt-text-xs);
  overflow-wrap: anywhere;
}
.form-field__hint {
  color: var(--rt-fg-muted);
}
.form-field__error {
  color: var(--rt-danger-text);
}
</style>
