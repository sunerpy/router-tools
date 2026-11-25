<script setup lang="ts">
defineProps<{ title?: string; eyebrow?: string }>();
</script>

<template>
  <section class="app-card">
    <header v-if="title || eyebrow || $slots.actions" class="app-card__header">
      <div class="app-card__heading">
        <p v-if="eyebrow" class="app-card__eyebrow">{{ eyebrow }}</p>
        <h2 v-if="title" class="app-card__title">{{ title }}</h2>
      </div>
      <div v-if="$slots.actions" class="cluster app-card__actions"><slot name="actions" /></div>
    </header>
    <div class="app-card__body"><slot /></div>
  </section>
</template>

<style scoped>
.app-card {
  min-width: 0;
  padding: var(--rt-card-inset);
  background: var(--rt-surface);
  border: var(--rt-border-width) solid var(--rt-border);
  border-radius: var(--rt-r-panel);
}
.app-card__header {
  display: flex;
  flex-wrap: wrap;
  align-items: flex-start;
  justify-content: space-between;
  gap: var(--rt-space-3);
  margin-bottom: var(--rt-space-4);
}
.app-card__heading {
  min-width: 0;
}
.app-card__eyebrow {
  margin: 0;
  color: var(--rt-fg-muted);
  font-size: var(--rt-text-sm);
}
.app-card__title {
  margin: 0;
  color: var(--rt-fg);
  font-size: var(--rt-text-lg);
  font-weight: var(--rt-weight-strong);
  overflow-wrap: anywhere;
}
.app-card__actions {
  gap: var(--rt-space-2);
}
.app-card__body {
  min-width: 0;
}
</style>
