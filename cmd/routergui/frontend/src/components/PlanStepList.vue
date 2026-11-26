<script setup lang="ts">
import { computed, type Component } from "vue";
import { Activity, Database, RotateCw, ShieldCheck } from "lucide-vue-next";
import AppBadge from "./AppBadge.vue";
import StatusBadge from "./StatusBadge.vue";
import { ACTION, MAINTENANCE, STEP_TITLE, effectText, reasonText } from "../services/guideText";
import type { GuideStep, GuideStepID, GuideStepResult } from "../services/api";

const props = defineProps<{ steps?: GuideStep[]; results?: GuideStepResult[] }>();

const ICON: Record<GuideStepID, Component> = {
  enable_ssh: ShieldCheck,
  backup: Database,
  soft_install: Activity,
  reboot_verify: RotateCw,
};

const rows = computed(() => {
  if (props.results) {
    return props.results.map((result) => ({
      id: result.id,
      action: result.action,
      reason: result.reason ? reasonText(result.reason) : "",
      effects: [] as string[],
      maintenance: result.maintenance
        ? (MAINTENANCE[result.maintenance] ?? result.maintenance)
        : "",
      status: result.status,
      error: result.error,
    }));
  }
  return (props.steps ?? []).map((step) => ({
    id: step.id,
    action: step.action,
    reason: reasonText(step.reason),
    effects: (step.effects ?? []).map(effectText),
    maintenance: step.maintenance ? (MAINTENANCE[step.maintenance] ?? step.maintenance) : "",
    status: "",
    error: undefined,
  }));
});
</script>

<template>
  <ol class="plan-steps">
    <li v-for="row in rows" :key="row.id" class="plan-step">
      <span class="plan-step__chip" aria-hidden="true"
        ><component :is="ICON[row.id]" :size="18"
      /></span>
      <div class="plan-step__body">
        <div class="plan-step__head">
          <strong>{{ STEP_TITLE[row.id] ?? row.id }}</strong>
          <StatusBadge v-if="results" :status="row.status" />
          <AppBadge v-else :tone="ACTION[row.action]?.tone ?? 'neutral'">
            {{ ACTION[row.action]?.label ?? row.action }}
          </AppBadge>
        </div>
        <p v-if="row.reason" class="plan-step__reason">{{ row.reason }}</p>
        <ul v-if="row.effects.length" class="plan-step__effects">
          <li v-for="effect in row.effects" :key="effect">{{ effect }}</li>
        </ul>
        <p v-if="row.maintenance" class="plan-step__reason">需要的维护：{{ row.maintenance }}</p>
        <p v-if="row.error" class="plan-step__error">
          <span class="mono">{{ row.error.code }}</span
          >：{{ row.error.message }}
        </p>
      </div>
    </li>
  </ol>
</template>

<style scoped>
.plan-steps {
  display: grid;
  gap: var(--rt-space-2);
  margin: 0;
  padding: 0;
  list-style: none;
}
.plan-step {
  display: grid;
  grid-template-columns: auto minmax(0, 1fr);
  gap: var(--rt-space-3);
  padding: var(--rt-space-3);
  border: var(--rt-border-width) solid var(--rt-border);
  border-radius: var(--rt-r-control);
}
.plan-step__chip {
  display: inline-grid;
  place-items: center;
  width: var(--rt-control-height-compact);
  height: var(--rt-control-height-compact);
  color: var(--rt-fg-muted);
  background: var(--rt-inset-strong);
  border-radius: var(--rt-r-control);
}
.plan-step__body {
  display: grid;
  gap: var(--rt-space-1);
  min-width: 0;
}
.plan-step__head {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  justify-content: space-between;
  gap: var(--rt-space-2);
}
.plan-step__reason,
.plan-step__error {
  margin: 0;
  color: var(--rt-fg-muted);
  overflow-wrap: anywhere;
}
.plan-step__error {
  color: var(--rt-danger-text);
}
.plan-step__effects {
  display: grid;
  gap: var(--rt-space-1);
  margin: 0;
  padding-left: var(--rt-space-5);
  color: var(--rt-fg);
  font-size: var(--rt-text-sm);
}
</style>
