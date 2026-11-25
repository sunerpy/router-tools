<script setup lang="ts">
import { computed, type Component } from "vue";
import { AlertTriangle, CheckCircle2, CircleDot, LoaderCircle, XCircle } from "lucide-vue-next";
import StatusBadge from "./StatusBadge.vue";
import { stageList } from "../services/summary";

const props = defineProps<{
  status?: string;
  events: Array<{ stage: string; status: string }>;
}>();

const stages = computed(() => stageList(props.events));

const ICONS: Record<string, { icon: Component; tone: string }> = {
  succeeded: { icon: CheckCircle2, tone: "ok" },
  running: { icon: LoaderCircle, tone: "info" },
  partial: { icon: AlertTriangle, tone: "warn" },
  unknown: { icon: AlertTriangle, tone: "warn" },
  failed: { icon: XCircle, tone: "danger" },
};
</script>

<template>
  <section class="operation-progress" aria-label="操作进度">
    <div class="operation-progress__head">
      <strong>操作进度</strong>
      <StatusBadge :status="status" />
    </div>
    <p v-if="stages.length === 0" class="operation-progress__empty">
      运行后，这里会显示每个已验证阶段。
    </p>
    <ol v-else class="operation-progress__list">
      <li
        v-for="(event, index) in stages"
        :key="`${event.stage}-${index}`"
        class="operation-progress__event"
        :data-tone="(ICONS[event.status] ?? ICONS.running).tone"
      >
        <component
          :is="(ICONS[event.status] ?? { icon: CircleDot }).icon"
          class="operation-progress__icon"
          :size="16"
          aria-hidden="true"
        />
        <span class="operation-progress__stage mono">{{ event.stage }}</span>
        <StatusBadge :status="event.status" />
      </li>
    </ol>
    <p
      v-if="status === 'unknown' || status === 'partial'"
      class="operation-progress__warning"
      role="alert"
    >
      <AlertTriangle :size="16" aria-hidden="true" />
      <span
        ><strong>不要从头重跑。</strong
        >结果不是可自动重试的成功状态，请保留报告并人工检查设备。</span
      >
    </p>
  </section>
</template>

<style scoped>
.operation-progress {
  display: grid;
  gap: var(--rt-space-3);
  min-height: 0;
}
.operation-progress__head {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: var(--rt-space-3);
}
.operation-progress__empty {
  margin: 0;
  color: var(--rt-fg-muted);
}
.operation-progress__list {
  display: grid;
  max-height: var(--rt-report-max-height);
  padding: 0;
  margin: 0;
  overflow: auto;
  list-style: none;
}
.operation-progress__event {
  display: grid;
  grid-template-columns: auto minmax(0, 1fr) auto;
  align-items: center;
  gap: var(--rt-space-3);
  min-height: var(--rt-control-height);
  padding: var(--rt-space-1) 0;
  color: var(--rt-fg-muted);
}
.operation-progress__event + .operation-progress__event {
  border-top: var(--rt-border-width) solid var(--rt-border);
}
.operation-progress__event[data-tone="ok"] .operation-progress__icon {
  color: var(--rt-ok-text);
}
.operation-progress__event[data-tone="warn"] .operation-progress__icon {
  color: var(--rt-warning-text);
}
.operation-progress__event[data-tone="danger"] .operation-progress__icon {
  color: var(--rt-danger-text);
}
.operation-progress__event[data-tone="info"] .operation-progress__icon {
  color: var(--rt-accent-text);
}
.operation-progress__stage {
  color: var(--rt-fg);
  font-size: var(--rt-text-sm);
  overflow-wrap: anywhere;
}
.operation-progress__warning {
  display: flex;
  gap: var(--rt-space-2);
  padding: var(--rt-space-3) var(--rt-space-4);
  margin: 0;
  color: var(--rt-fg);
  background: var(--rt-warning-soft);
  border-radius: var(--rt-r-control);
  font-size: var(--rt-text-sm);
}
.operation-progress__warning svg,
.operation-progress__warning strong {
  color: var(--rt-warning-text);
}
.operation-progress__warning svg {
  margin-top: calc(var(--rt-space-1) / 2);
}
</style>
