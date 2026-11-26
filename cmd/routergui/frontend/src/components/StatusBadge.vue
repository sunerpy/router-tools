<script setup lang="ts">
import { computed, type Component } from "vue";
import {
  AlertTriangle,
  Ban,
  CheckCircle2,
  Circle,
  LoaderCircle,
  MinusCircle,
  XCircle,
} from "lucide-vue-next";
import AppBadge from "./AppBadge.vue";
import type { Tone } from "../services/summary";

const props = defineProps<{ status?: string }>();

const PRESENTATION: Record<string, { label: string; tone: Tone; icon: Component; spin?: boolean }> =
  {
    succeeded: { label: "成功", tone: "ok", icon: CheckCircle2 },
    running: { label: "运行中", tone: "info", icon: LoaderCircle, spin: true },
    partial: { label: "部分完成", tone: "warn", icon: AlertTriangle },
    unknown: { label: "结果未知", tone: "warn", icon: AlertTriangle },
    failed: { label: "失败", tone: "danger", icon: XCircle },
    skipped: { label: "已跳过", tone: "neutral", icon: MinusCircle },
    not_run: { label: "未执行", tone: "neutral", icon: Circle },
    idle: { label: "未运行", tone: "neutral", icon: Circle },
    canceling: { label: "正在取消", tone: "neutral", icon: LoaderCircle, spin: true },
    canceled: { label: "已取消", tone: "neutral", icon: Ban },
  };

const presentation = computed(
  () =>
    PRESENTATION[props.status || "idle"] ?? {
      label: props.status as string,
      tone: "neutral" as Tone,
      icon: MinusCircle,
    },
);
</script>

<template>
  <AppBadge
    :tone="presentation.tone"
    :icon="presentation.icon"
    :spin="presentation.spin"
    :data-status="status || 'idle'"
    >{{ presentation.label }}</AppBadge
  >
</template>
