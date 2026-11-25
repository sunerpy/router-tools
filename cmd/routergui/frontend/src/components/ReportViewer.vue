<script setup lang="ts">
import { computed, ref, watch } from "vue";
import { Check, Copy, Download } from "lucide-vue-next";
import AppButton from "./AppButton.vue";

const props = defineProps<{ report: unknown }>();
const copied = ref(false);
const formatted = computed(() => JSON.stringify(props.report, null, 2) ?? "null");
watch(
  () => props.report,
  () => {
    copied.value = false;
  },
);

async function copyReport() {
  await navigator.clipboard.writeText(formatted.value);
  copied.value = true;
}
function downloadReport() {
  const blob = new Blob([formatted.value], { type: "application/json" });
  const url = URL.createObjectURL(blob);
  const anchor = document.createElement("a");
  anchor.href = url;
  anchor.download = "router-report.json";
  anchor.click();
  URL.revokeObjectURL(url);
}
</script>

<template>
  <section class="report-viewer" aria-label="运行报告">
    <div class="cluster report-viewer__actions">
      <AppButton size="sm" :aria-label="copied ? '已复制' : '复制报告'" @click="copyReport">
        <Check v-if="copied" :size="16" aria-hidden="true" />
        <Copy v-else :size="16" aria-hidden="true" />
        {{ copied ? "已复制" : "复制报告" }}
      </AppButton>
      <AppButton size="sm" variant="ghost" aria-label="下载报告" @click="downloadReport">
        <Download :size="16" aria-hidden="true" />下载报告
      </AppButton>
    </div>
    <pre class="report-viewer__content" tabindex="0">{{ formatted }}</pre>
  </section>
</template>

<style scoped>
.report-viewer {
  display: grid;
  gap: var(--rt-space-3);
  min-height: 0;
}
.report-viewer__actions {
  gap: var(--rt-space-2);
}
.report-viewer__content {
  max-height: var(--rt-report-max-height);
  padding: var(--rt-space-4);
  margin: 0;
  overflow: auto;
  color: var(--rt-fg);
  background: var(--rt-inset);
  border: var(--rt-border-width) solid var(--rt-border);
  border-radius: var(--rt-r-control);
  font-size: var(--rt-text-sm);
  white-space: pre-wrap;
  overflow-wrap: anywhere;
}
</style>
