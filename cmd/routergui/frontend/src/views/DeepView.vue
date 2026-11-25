<script setup lang="ts">
import { computed, inject, onBeforeUnmount, onMounted, ref } from "vue";
import { ChevronRight, CircleDot, Copy, FileText, ShieldAlert } from "lucide-vue-next";
import AppAlert from "../components/AppAlert.vue";
import AppBadge from "../components/AppBadge.vue";
import AppButton from "../components/AppButton.vue";
import AppCard from "../components/AppCard.vue";
import AppIconButton from "../components/AppIconButton.vue";
import AppStepper from "../components/AppStepper.vue";
import AppToggle from "../components/AppToggle.vue";
import ConfirmDialog from "../components/ConfirmDialog.vue";
import EmptyState from "../components/EmptyState.vue";
import OperationPanel from "../components/OperationPanel.vue";
import SecretInput from "../components/SecretInput.vue";
import StatusBadge from "../components/StatusBadge.vue";
import TextInput from "../components/TextInput.vue";
import { displayStatus, formatTime, latestEntry } from "../services/summary";
import { required, useFieldErrors } from "../composables/useFieldErrors";

const store = inject("operations") as any;
const step = ref(1);
const dialogOpen = ref(false);
// The deep flow's Telnet acknowledgement is its own, always unset when the page opens: the
// shared "allow Telnet" option of other pages must never pre-satisfy it (DESIGN.md §4).
const telnetAcknowledged = ref(false);
const { errors, check } = useFieldErrors();
function confirmEvidence() {
  const ok = check([
    required("deep-backup", store.backupDir, "请输入已通过校验的当前启动备份目录"),
    required("deep-run", store.runDir, "请输入私有 journal 运行目录"),
  ]);
  if (ok) step.value = 2;
}
onMounted(() => {
  store.refreshHistory().catch(() => {});
});
onBeforeUnmount(() => {
  telnetAcknowledged.value = false;
  store.clearSecrets();
});

function runDeep() {
  dialogOpen.value = false;
  const allowTelnet = telnetAcknowledged.value;
  telnetAcknowledged.value = false;
  void store.run("DeepPersist", { AllowTelnet: allowTelnet });
}

const ready = computed(
  () =>
    telnetAcknowledged.value &&
    store.backupDir &&
    store.runDir &&
    store.rootPassword &&
    store.ackDeep === "RD08-DEEP",
);
const stepperCurrent = computed(() => (dialogOpen.value ? 3 : step.value));

// The checks the tool performs before the first write (RISKS.md). They are not shown as passed:
// only the report of a run carries that evidence.
const checks = [
  { title: "备份目录通过本地完整性校验", detail: "manifest、SHA256SUMS、每个分区的长度与哈希" },
  { title: "在线身份、完整 MTD 布局与 boot_id 一致", detail: "与备份记录逐项比对" },
  { title: "运行目录中没有同主机的 journal", detail: "已有 journal 时拒绝从头开始" },
  { title: "所需工具存在且 crash 分区可读", detail: "缺任何一项都在写入前停止" },
];

const lastRun = computed(() => latestEntry(store.history, "deep-persist"));
const lastReport = computed(() => lastRun.value?.report as any);
const nonRetry = computed(() => ["unknown", "partial"].includes(lastReport.value?.status));
const copied = ref(false);
async function copyJournal() {
  if (!lastReport.value?.journal_path) return;
  await navigator.clipboard.writeText(lastReport.value.journal_path);
  copied.value = true;
}
</script>

<template>
  <div class="stack">
    <AppAlert
      type="error"
      title="可能导致设备无法启动"
      message="中断、断电或固件差异都可能让路由器无法启动。写入超时按“结果未知”处理，不会自动重试；已有 journal 时拒绝从头开始。"
    >
      <p class="deep__instruction">
        中断后不要从头重跑：保留 journal、运行报告和备份目录，按 RECOVERY.md
        人工检查；不要更换运行目录来绕过 journal 检查。
      </p>
    </AppAlert>

    <div class="split">
      <AppCard eyebrow="执行前检查" title="深度固化流程">
        <template #actions>
          <AppBadge tone="danger" :icon="ShieldAlert">高风险操作</AppBadge>
        </template>
        <div class="stack">
          <AppStepper
            :steps="['前置证据', '凭据与明文风险', '最终确认']"
            :current="stepperCurrent"
          />

          <template v-if="step === 1">
            <ul class="deep__checks">
              <li v-for="check in checks" :key="check.title" class="deep__check">
                <CircleDot :size="18" aria-hidden="true" />
                <div>
                  {{ check.title }}<span class="deep__check-detail">{{ check.detail }}</span>
                </div>
                <AppBadge tone="neutral">执行时检查</AppBadge>
              </li>
            </ul>
            <TextInput
              id="deep-backup"
              v-model="store.backupDir"
              label="已通过校验的当前启动备份目录"
              :error="errors['deep-backup']"
              mono
              required
            />
            <TextInput
              id="deep-run"
              v-model="store.runDir"
              label="私有 journal 运行目录"
              hint="已有同主机 journal 时会拒绝执行。"
              :error="errors['deep-run']"
              mono
              required
            />
            <div class="cluster">
              <AppButton variant="primary" @click="confirmEvidence"
                >确认备份与恢复边界<ChevronRight :size="16" aria-hidden="true"
              /></AppButton>
            </div>
          </template>

          <template v-else>
            <SecretInput id="deep-root" v-model="store.rootPassword" label="Root 密码" required />
            <AppToggle v-model="telnetAcknowledged" note="明文"
              >我了解 Telnet 为明文协议，并明确允许中间阶段使用</AppToggle
            >
            <TextInput
              id="deep-ack"
              v-model="store.ackDeep"
              label="输入 RD08-DEEP 确认三次重启与分区写擦"
              mono
              required
            />
            <div class="cluster">
              <AppButton
                variant="danger"
                :disabled="!ready || store.busy"
                @click="dialogOpen = true"
              >
                <ShieldAlert :size="16" aria-hidden="true" />进行最终确认
              </AppButton>
              <AppButton variant="ghost" @click="step = 1">返回上一步</AppButton>
              <span class="deep__hint">最终确认对话框默认聚焦“取消”</span>
            </div>
          </template>
        </div>
      </AppCard>

      <AppCard
        :eyebrow="lastRun ? `上次运行 · ${formatTime(lastRun.time)}` : '上次运行'"
        title="深度固化记录"
      >
        <template v-if="lastReport" #actions>
          <StatusBadge :status="displayStatus(lastReport)" />
        </template>
        <EmptyState
          v-if="!lastReport"
          title="本次运行还没有深度固化记录"
          message="执行后，这里保留结果、journal 路径和非重试提示。"
        />
        <div v-else class="stack">
          <p v-if="nonRetry" class="deep__callout" role="alert">
            <strong>不要从头重跑。</strong>先保留 journal，人工检查 crash
            分区和启动状态，再按恢复边界处理。
          </p>
          <p v-if="lastReport.error" class="deep__error">
            <span class="mono">{{ lastReport.error.code }}</span> {{ lastReport.error.message }}
          </p>
          <div v-if="lastReport.journal_path" class="deep__journal">
            <FileText :size="16" aria-hidden="true" />
            <span class="mono deep__journal-path">{{ lastReport.journal_path }}</span>
            <AppIconButton
              :label="copied ? '已复制路径' : '复制 journal 路径'"
              @click="copyJournal"
            >
              <Copy :size="16" aria-hidden="true" />
            </AppIconButton>
          </div>
          <RouterLink to="/reports" class="deep__link"
            >在运行报告中查看完整 JSON<ChevronRight :size="14" aria-hidden="true"
          /></RouterLink>
        </div>
      </AppCard>
    </div>

    <OperationPanel />
    <ConfirmDialog
      v-model="dialogOpen"
      title="最终深度固化确认"
      description="仅在独立授权、供电稳定且已有官方恢复方案时继续。写超时不会自动重试。"
      require-text="RD08-DEEP"
      danger
      confirm-label="执行深度固化"
      @confirm="runDeep"
    />
  </div>
</template>

<style scoped>
.deep__checks {
  display: grid;
  padding: 0;
  margin: 0;
  list-style: none;
}
.deep__instruction {
  margin: var(--rt-space-2) 0 0;
  font-weight: var(--rt-weight-medium);
}
.deep__check {
  display: grid;
  grid-template-columns: auto minmax(0, 1fr) auto;
  align-items: center;
  gap: var(--rt-space-3);
  padding: var(--rt-space-3) 0;
  color: var(--rt-fg);
}
.deep__check + .deep__check {
  border-top: var(--rt-border-width) solid var(--rt-border);
}
.deep__check > svg {
  color: var(--rt-fg-muted);
}
.deep__check-detail {
  display: block;
  color: var(--rt-fg-muted);
  font-size: var(--rt-text-xs);
}
.deep__hint {
  color: var(--rt-fg-muted);
  font-size: var(--rt-text-xs);
}
.deep__callout {
  padding: var(--rt-space-3) var(--rt-space-4);
  margin: 0;
  color: var(--rt-fg);
  background: var(--rt-warning-soft);
  border-radius: var(--rt-r-control);
  font-size: var(--rt-text-sm);
}
.deep__callout strong {
  color: var(--rt-warning-text);
}
.deep__error {
  margin: 0;
  color: var(--rt-danger-text);
  font-size: var(--rt-text-sm);
  overflow-wrap: anywhere;
}
.deep__journal {
  display: flex;
  align-items: center;
  gap: var(--rt-space-2);
  padding: var(--rt-space-2) var(--rt-space-3);
  background: var(--rt-inset);
  border: var(--rt-border-width) solid var(--rt-border);
  border-radius: var(--rt-r-control);
  font-size: var(--rt-text-xs);
}
.deep__journal-path {
  flex: 1;
  min-width: 0;
  overflow-wrap: anywhere;
}
.deep__link {
  display: inline-flex;
  align-items: center;
  gap: var(--rt-space-1);
  font-size: var(--rt-text-sm);
  font-weight: var(--rt-weight-medium);
  text-decoration: none;
}
@media (max-width: 480px) {
  .deep__check {
    grid-template-columns: auto minmax(0, 1fr);
  }
  .deep__check > :last-child {
    grid-column: 2;
    justify-self: start;
  }
}
</style>
