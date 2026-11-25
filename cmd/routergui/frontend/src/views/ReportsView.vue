<script setup lang="ts">
import { computed, inject, onMounted, ref } from "vue";
import { RefreshCw } from "lucide-vue-next";
import AppButton from "../components/AppButton.vue";
import AppCard from "../components/AppCard.vue";
import EmptyState from "../components/EmptyState.vue";
import HistoryList from "../components/HistoryList.vue";
import ReportViewer from "../components/ReportViewer.vue";
import StatusBadge from "../components/StatusBadge.vue";
import { recentOperations } from "../services/summary";

const store = inject("operations") as any;
const selectedID = ref("");
const rows = computed(() => recentOperations(store.history, store.history.length));
const selected = computed(
  () =>
    store.history.find((entry: any) => entry.operation_id === selectedID.value) ?? store.history[0],
);
const selectedRow = computed(() =>
  rows.value.find((row) => row.id === selected.value?.operation_id),
);

async function refresh() {
  try {
    await store.refreshHistory();
  } catch {
    // The empty state already explains what to do; bindings may be unavailable in a static page.
  }
}
onMounted(refresh);
</script>

<template>
  <div class="stack">
    <EmptyState
      v-if="!store.history.length"
      title="尚无操作报告"
      message="先运行设备预检。本次运行以来的每个操作报告都会保留在这里。"
    >
      <div class="cluster reports__empty-actions">
        <AppButton size="sm" @click="refresh"
          ><RefreshCw :size="14" aria-hidden="true" />刷新报告</AppButton
        >
      </div>
    </EmptyState>
    <div v-else class="list-detail">
      <AppCard :title="`操作记录 · ${store.history.length}`">
        <template #actions>
          <AppButton size="sm" variant="ghost" @click="refresh"
            ><RefreshCw :size="14" aria-hidden="true" />刷新</AppButton
          >
        </template>
        <HistoryList
          :rows="rows"
          selectable
          :selected="selected?.operation_id"
          @select="(id) => (selectedID = id)"
        />
      </AppCard>
      <AppCard
        v-if="selected"
        :eyebrow="selectedRow?.meta"
        :title="selectedRow?.title ?? selected.operation"
      >
        <template #actions>
          <StatusBadge :status="selectedRow?.status" />
        </template>
        <ReportViewer :report="selected.report" />
      </AppCard>
    </div>
  </div>
</template>

<style scoped>
.reports__empty-actions {
  margin-top: var(--rt-space-2);
}
</style>
