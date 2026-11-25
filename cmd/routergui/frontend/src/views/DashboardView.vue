<script setup lang="ts">
import { computed, inject, onBeforeUnmount, onMounted, type Component } from "vue";
import { ChevronRight, Globe, Play, Server, Terminal } from "lucide-vue-next";
import AppBadge from "../components/AppBadge.vue";
import AppButton from "../components/AppButton.vue";
import AppCard from "../components/AppCard.vue";
import AppToggle from "../components/AppToggle.vue";
import EmptyState from "../components/EmptyState.vue";
import FormField from "../components/FormField.vue";
import HistoryList from "../components/HistoryList.vue";
import OperationPanel from "../components/OperationPanel.vue";
import SecretInput from "../components/SecretInput.vue";
import StatTile from "../components/StatTile.vue";
import TargetForm from "../components/TargetForm.vue";
import { required, targetRules, useFieldErrors } from "../composables/useFieldErrors";
import {
  deviceSummary,
  latestEntry,
  nextStep,
  portTiles,
  recentOperations,
} from "../services/summary";

const store = inject("operations") as any;
const { errors, check } = useFieldErrors();
onMounted(() => {
  store.refreshHistory().catch(() => {});
});
onBeforeUnmount(() => store.clearSecrets());

// The newest preflight, with the target it was sent to (not the possibly edited form).
const doctorRun = computed(() => {
  if (store.operation === "Doctor" && store.report) {
    return { report: store.report, target: store.targetFor(store.operationID) };
  }
  const entry = latestEntry(store.history, "doctor");
  return entry
    ? { report: entry.report as any, target: store.targetFor(entry.operation_id) }
    : undefined;
});
const doctor = computed(() => doctorRun.value?.report);
const summary = computed(() => deviceSummary(doctor.value));
const tiles = computed(() =>
  doctor.value ? portTiles(doctor.value, doctorRun.value?.target) : [],
);
const next = computed(() => nextStep(doctor.value, store.history));

function run(name: "Doctor" | "Verify") {
  const rules = targetRules(store.target, store.profileID);
  if (store.withAdminAuth)
    rules.push(required("dashboard-admin", store.adminPassword, "请输入 Web 管理密码"));
  if (store.withRootVerify || name === "Verify") {
    rules.push(required("dashboard-root", store.rootPassword, "请输入 root 密码"));
  }
  if (check(rules)) void store.run(name);
}
const rows = computed(() => recentOperations(store.history, 5));
const tileIcons: Record<string, Component> = { web: Globe, ssh: Terminal, telnet: Server };
</script>

<template>
  <div class="stack">
    <div class="split">
      <AppCard eyebrow="目标设备" title="运行只读预检">
        <template #actions>
          <RouterLink to="/settings" class="dashboard__link"
            >连接设置<ChevronRight :size="14" aria-hidden="true"
          /></RouterLink>
        </template>
        <div class="stack">
          <TargetForm :errors="errors" />
          <div class="cluster dashboard__switches">
            <AppToggle v-model="store.withAdminAuth">验证 Web 管理密码</AppToggle>
            <AppToggle v-model="store.withRootVerify">验证 root Shell</AppToggle>
            <AppToggle v-model="store.allowTelnet" note="明文">允许 Telnet</AppToggle>
          </div>
          <div v-if="store.withAdminAuth || store.withRootVerify" class="dashboard__secrets">
            <SecretInput
              v-if="store.withAdminAuth"
              id="dashboard-admin"
              v-model="store.adminPassword"
              label="Web 管理密码"
              :error="errors['dashboard-admin']"
              required
            />
            <SecretInput
              v-if="store.withRootVerify"
              id="dashboard-root"
              v-model="store.rootPassword"
              label="Root 密码"
              :error="errors['dashboard-root']"
              required
            />
            <FormField v-if="store.withRootVerify" id="dashboard-transport" label="传输">
              <div class="rt-field">
                <select
                  id="dashboard-transport"
                  v-model="store.transport"
                  class="rt-field__control"
                >
                  <option value="ssh">SSH</option>
                  <option value="auto">SSH → Telnet</option>
                  <option value="telnet">Telnet 诊断</option>
                </select>
              </div>
            </FormField>
          </div>
          <div class="cluster">
            <AppButton
              variant="primary"
              :loading="store.busy && store.operation === 'Doctor'"
              :disabled="store.busy && store.operation !== 'Doctor'"
              @click="run('Doctor')"
            >
              <Play
                v-if="!(store.busy && store.operation === 'Doctor')"
                :size="16"
                aria-hidden="true"
              />运行预检
            </AppButton>
            <AppButton
              :disabled="!store.withRootVerify || (store.busy && store.operation !== 'Verify')"
              :loading="store.busy && store.operation === 'Verify'"
              @click="run('Verify')"
              >验证 root</AppButton
            >
            <span class="dashboard__hint">密码只用于本次请求，发出后立即清空</span>
          </div>
        </div>
      </AppCard>

      <AppCard eyebrow="识别结果" :title="summary ? summary.model : '尚未识别设备'">
        <template v-if="summary" #actions>
          <AppBadge :tone="summary.compatible ? 'ok' : 'danger'">{{
            summary.compatible ? "精确匹配" : "不兼容"
          }}</AppBadge>
        </template>
        <EmptyState
          v-if="!summary"
          title="还没有识别结果"
          message="运行预检后，这里显示硬件、固件和匹配的 Profile。"
        />
        <div v-else class="stack">
          <dl class="dashboard__facts">
            <dt>硬件</dt>
            <dd class="mono">{{ summary.hardware || "—" }}</dd>
            <dt>固件</dt>
            <dd class="mono">{{ summary.firmware || "—" }}</dd>
            <dt>Profile</dt>
            <dd class="mono">{{ summary.profileId || "—" }}</dd>
          </dl>
          <div v-if="summary.capabilities.length" class="dashboard__capabilities">
            <p class="dashboard__eyebrow">已实现的能力</p>
            <div class="cluster dashboard__chips">
              <span v-for="capability in summary.capabilities" :key="capability" class="chip">{{
                capability
              }}</span>
            </div>
          </div>
        </div>
      </AppCard>
    </div>

    <div class="tile-grid">
      <template v-if="tiles.length">
        <StatTile
          v-for="tile in tiles"
          :key="tile.key"
          :tone="tile.tone"
          :title="tile.title"
          :meta="tile.meta"
          :value="tile.value"
          :detail="tile.detail"
          :icon="tileIcons[tile.key]"
        />
      </template>
      <EmptyState
        v-else
        class="dashboard__tiles-empty"
        title="端口与认证状态"
        message="预检会分别显示 Web、SSH、Telnet 的状态；开放端口本身不算认证成功。"
      />
      <RouterLink v-if="next.to" :to="next.to" class="dashboard__next">
        <StatTile
          tone="highlight"
          title="下一步"
          :value="next.title"
          :detail="next.detail"
          :icon="ChevronRight"
        />
      </RouterLink>
      <StatTile
        v-else
        tone="highlight"
        title="下一步"
        :value="next.title"
        :detail="next.detail"
        :icon="ChevronRight"
      />
    </div>

    <OperationPanel />

    <AppCard title="最近的操作">
      <template #actions>
        <RouterLink to="/reports" class="dashboard__link"
          >全部报告<ChevronRight :size="14" aria-hidden="true"
        /></RouterLink>
      </template>
      <HistoryList v-if="rows.length" :rows="rows" />
      <EmptyState
        v-else
        title="还没有操作记录"
        message="本次运行的每个操作都会在这里留下一条带状态的记录。"
      />
    </AppCard>
  </div>
</template>

<style scoped>
.dashboard__link {
  display: inline-flex;
  align-items: center;
  gap: var(--rt-space-1);
  font-size: var(--rt-text-sm);
  font-weight: var(--rt-weight-medium);
  text-decoration: none;
}
.dashboard__switches {
  gap: var(--rt-space-2) var(--rt-space-5);
}
.dashboard__secrets {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(min(100%, var(--rt-field-min)), 1fr));
  gap: var(--rt-space-3);
}
.dashboard__hint {
  color: var(--rt-fg-muted);
  font-size: var(--rt-text-xs);
}
.dashboard__facts {
  display: grid;
  grid-template-columns: auto minmax(0, 1fr);
  gap: var(--rt-space-2) var(--rt-space-4);
  margin: 0;
}
.dashboard__facts dt {
  color: var(--rt-fg-muted);
}
.dashboard__facts dd {
  margin: 0;
  overflow-wrap: anywhere;
}
.dashboard__eyebrow {
  margin: 0 0 var(--rt-space-2);
  color: var(--rt-fg-muted);
  font-size: var(--rt-text-sm);
}
.dashboard__chips {
  gap: var(--rt-space-2);
}
.dashboard__tiles-empty {
  grid-column: span 3;
}
.dashboard__next {
  display: grid;
  color: inherit;
  text-decoration: none;
  border-radius: var(--rt-r-panel);
}
.dashboard__next:hover :deep(.stat-tile) {
  filter: brightness(1.08);
}
@media (max-width: 1024px) {
  .dashboard__tiles-empty {
    grid-column: auto;
  }
}
</style>
