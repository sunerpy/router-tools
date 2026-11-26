<script setup lang="ts">
import { computed, ref, watch } from "vue";
import { Check, Copy } from "lucide-vue-next";
import AppButton from "./AppButton.vue";

const props = defineProps<{ label: string; content: string }>();
const state = ref<"idle" | "copied" | "failed">("idle");
// Chinese text gets a space before a Latin word: 复制 Agent 连接地址, 复制配置片段.
const spaced = computed(() => (/^[A-Za-z0-9]/.test(props.label) ? ` ${props.label}` : props.label));
watch(
  () => props.content,
  () => {
    state.value = "idle";
  },
);

async function copy() {
  try {
    await navigator.clipboard.writeText(props.content);
    state.value = "copied";
  } catch {
    state.value = "failed";
  }
}
</script>

<template>
  <section class="code-block" :aria-label="label">
    <div class="code-block__head">
      <span class="code-block__label">{{ label }}</span>
      <AppButton
        size="sm"
        variant="ghost"
        :aria-label="state === 'copied' ? `已复制${spaced}` : `复制${spaced}`"
        @click="copy"
      >
        <Check v-if="state === 'copied'" :size="14" aria-hidden="true" />
        <Copy v-else :size="14" aria-hidden="true" />
        {{ state === "copied" ? "已复制" : "复制" }}
      </AppButton>
    </div>
    <pre class="code-block__content mono" tabindex="0">{{ content }}</pre>
    <p class="code-block__status" aria-live="polite">
      {{ state === "copied" ? "已复制" : state === "failed" ? "无法复制，请手动选择文本" : "" }}
    </p>
  </section>
</template>

<style scoped>
.code-block {
  display: grid;
  gap: var(--rt-space-2);
  min-width: 0;
}
.code-block__head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: var(--rt-space-2);
  min-width: 0;
}
.code-block__label {
  color: var(--rt-fg-muted);
  font-size: var(--rt-text-sm);
  overflow-wrap: anywhere;
}
.code-block__content {
  max-height: var(--rt-report-max-height);
  margin: 0;
  padding: var(--rt-space-3) var(--rt-space-4);
  overflow: auto;
  color: var(--rt-fg);
  background: var(--rt-inset);
  border: var(--rt-border-width) solid var(--rt-border);
  border-radius: var(--rt-r-control);
  font-size: var(--rt-text-sm);
  white-space: pre-wrap;
  overflow-wrap: anywhere;
}
.code-block__content:focus-visible {
  outline: var(--rt-focus-width) solid var(--rt-focus);
  outline-offset: var(--rt-focus-width);
}
.code-block__status {
  margin: 0;
  color: var(--rt-fg-muted);
  font-size: var(--rt-text-xs);
}
</style>
