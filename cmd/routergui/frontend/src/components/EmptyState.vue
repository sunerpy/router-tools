<script setup lang="ts">
import type { Component } from "vue";
import { Info } from "lucide-vue-next";

withDefaults(defineProps<{ title: string; message: string; icon?: Component }>(), {
  icon: undefined,
});
</script>

<template>
  <div class="empty-state">
    <span class="empty-state__chip" aria-hidden="true"
      ><component :is="icon ?? Info" :size="18"
    /></span>
    <div class="empty-state__text">
      <strong>{{ title }}</strong>
      <p>{{ message }}</p>
      <slot />
    </div>
  </div>
</template>

<style scoped>
.empty-state {
  display: flex;
  align-items: flex-start;
  gap: var(--rt-space-3);
  padding: var(--rt-space-4) var(--rt-card-inset);
  color: var(--rt-fg-muted);
  background: var(--rt-inset);
  border: var(--rt-border-width) dashed var(--rt-border-strong);
  border-radius: var(--rt-r-panel);
}
.empty-state__chip {
  display: grid;
  flex: none;
  place-items: center;
  width: var(--rt-chip-size);
  height: var(--rt-chip-size);
  border-radius: var(--rt-r-panel);
  background: var(--rt-inset-strong);
}
.empty-state__text {
  display: grid;
  gap: var(--rt-space-1);
  min-width: 0;
}
.empty-state strong {
  color: var(--rt-fg);
  font-weight: var(--rt-weight-strong);
}
.empty-state p {
  margin: 0;
  overflow-wrap: anywhere;
}
</style>
