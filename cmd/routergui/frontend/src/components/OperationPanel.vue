<script setup lang="ts">
import { inject } from "vue";
import { Square } from "lucide-vue-next";
import AppAlert from "./AppAlert.vue";
import AppButton from "./AppButton.vue";
import AppCard from "./AppCard.vue";
import OperationProgress from "./OperationProgress.vue";
import ReportViewer from "./ReportViewer.vue";

const store = inject("operations") as any;
</script>

<template>
  <AppCard title="运行状态">
    <template v-if="store.busy" #actions>
      <AppButton variant="danger" size="sm" @click="store.cancel">
        <Square :size="14" aria-hidden="true" />取消当前操作
      </AppButton>
    </template>
    <div class="stack">
      <AppAlert v-if="store.error" type="error" title="操作未完成" :message="store.error" />
      <OperationProgress :status="store.status" :events="store.events" />
      <ReportViewer v-if="store.report" :report="store.report" />
    </div>
  </AppCard>
</template>
