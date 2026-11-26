<script setup lang="ts">
import { computed, inject, ref } from "vue";
import { ChevronRight, Copy, Globe, Server, Terminal } from "lucide-vue-next";
import AppAlert from "../components/AppAlert.vue";
import AppBadge from "../components/AppBadge.vue";
import AppButton from "../components/AppButton.vue";
import AppCard from "../components/AppCard.vue";
import AppIconButton from "../components/AppIconButton.vue";
import AppSegmented from "../components/AppSegmented.vue";
import AppStepper from "../components/AppStepper.vue";
import AppToggle from "../components/AppToggle.vue";
import ConfirmDialog from "../components/ConfirmDialog.vue";
import CredentialState from "../components/CredentialState.vue";
import DeviceSelector from "../components/DeviceSelector.vue";
import PlanStepList from "../components/PlanStepList.vue";
import EmptyState from "../components/EmptyState.vue";
import HistoryList from "../components/HistoryList.vue";
import OperationProgress from "../components/OperationProgress.vue";
import ReportViewer from "../components/ReportViewer.vue";
import SecretInput from "../components/SecretInput.vue";
import StatTile from "../components/StatTile.vue";
import StatusBadge from "../components/StatusBadge.vue";
import TextInput from "../components/TextInput.vue";
import type { ThemeController, ThemePreference } from "../composables/useTheme";
import type { CatalogEntry, GuideStep, GuideStepResult } from "../services/api";

const theme = inject("theme") as ThemeController;
const appearance = computed({
  get: () => theme.preference.value,
  set: (value: string) => theme.setPreference(value as ThemePreference),
});
const secret = ref("");
const text = ref("192.168.31.1");
const longIdentifier = ref("SHA256:" + "A1b2C3d4".repeat(10));
const toggles = ref({ on: true, off: false });
const dialogOpen = ref(false);
const selected = ref("e1");
const statuses = [
  "idle",
  "running",
  "succeeded",
  "partial",
  "unknown",
  "failed",
  "skipped",
  "not_run",
  "canceling",
  "canceled",
];
const report = { status: "partial", operation: "enable", message: "X".repeat(500) };
const sampleCatalog: CatalogEntry[] = [
  {
    model_id: "xiaomi-rd08",
    vendor: "Xiaomi",
    model: "BE6500 Pro",
    hardware: "RD08",
    profiles: [
      {
        id: "xiaomi-rd08-1.1.96",
        model_id: "xiaomi-rd08",
        vendor: "Xiaomi",
        model: "BE6500 Pro",
        hardware: "RD08",
        firmware: "1.1.96",
        capabilities: [],
      },
    ],
  },
];
const profileChoice = ref("auto");
const staleProfile = ref("xiaomi-rd08-9.9.9");
const planSteps: GuideStep[] = [
  {
    id: "enable_ssh",
    action: "run",
    reason: "ssh_closed",
    effects: ["temporary_mac_rules", "nvram_commit", "root_password_change"],
  },
  { id: "backup", action: "skip", reason: "backup_verified" },
  {
    id: "soft_install",
    action: "decide_after",
    reason: "after_ssh",
    effects: ["script_write", "uci_commit"],
  },
  { id: "reboot_verify", action: "needs_input", reason: "root_password_missing" },
];
const blockedSteps: GuideStep[] = [
  { id: "soft_install", action: "blocked", reason: "soft_conflict", maintenance: "manual_review" },
  { id: "reboot_verify", action: "unsupported", reason: "capability_missing" },
];
const results: GuideStepResult[] = [
  { id: "enable_ssh", action: "run", status: "succeeded" },
  {
    id: "backup",
    action: "run",
    status: "failed",
    error: { code: "BACKUP_INCOMPLETE", message: "第 4 个分区读取长度不符" },
  },
  { id: "soft_install", action: "not_run", status: "not_run" },
];
const events = [
  { stage: "PRECHECK", status: "succeeded" },
  { stage: "CRASH_WRITE_STARTED", status: "running" },
  { stage: "CRASH_WRITE_TIMEOUT", status: "unknown" },
  { stage: "failed", status: "failed" },
];
const rows = [
  {
    id: "d1",
    operation: "doctor",
    title: "设备预检",
    meta: "14:32:07 · 192.168.31.1 · 1.2 s",
    status: "succeeded",
  },
  {
    id: "e1",
    operation: "enable",
    title: "开启 SSH",
    meta: "13:58:41 · MAC_CLEANUP_FAILED",
    status: "partial",
  },
  {
    id: "x1",
    operation: "deep-persist",
    title: "深度固化",
    meta: "昨天 20:55 · REMOTE_RESULT_UNKNOWN",
    status: "unknown",
  },
  {
    id: "f1",
    operation: "doctor",
    title: "设备预检",
    meta: "昨天 20:40 · WEB_UNREACHABLE",
    status: "failed",
  },
];
</script>

<template>
  <div class="primitive-page stack">
    <AppCard eyebrow="开发用" title="主题">
      <div class="cluster">
        <AppSegmented
          v-model="appearance"
          :options="[
            { value: 'light', label: '浅色 · 白瓷' },
            { value: 'dark', label: '深色 · 石墨' },
            { value: 'system', label: '跟随系统' },
          ]"
          label="展示主题"
        />
        <span class="primitive-page__note">先在两种主题下核对这些原语，再组装业务页面。</span>
      </div>
    </AppCard>

    <AppCard title="按钮">
      <div class="stack">
        <div class="cluster">
          <AppButton variant="primary">主要操作</AppButton>
          <AppButton>次要操作</AppButton>
          <AppButton variant="ghost">文字按钮</AppButton>
          <AppButton variant="danger" @click="dialogOpen = true">危险确认</AppButton>
        </div>
        <div class="cluster">
          <AppButton variant="primary" loading>加载中</AppButton>
          <AppButton disabled>禁用</AppButton>
          <AppButton size="sm"><Copy :size="14" aria-hidden="true" />小按钮</AppButton>
          <AppIconButton label="复制"><Copy :size="16" aria-hidden="true" /></AppIconButton>
        </div>
      </div>
    </AppCard>

    <AppCard title="状态与徽标">
      <div class="stack">
        <div class="cluster">
          <StatusBadge v-for="status in statuses" :key="status" :status="status" />
        </div>
        <div class="cluster">
          <AppBadge tone="ok">精确匹配</AppBadge>
          <AppBadge tone="info">端口开放</AppBadge>
          <AppBadge tone="warn">已允许明文</AppBadge>
          <AppBadge tone="danger">高风险操作</AppBadge>
          <AppBadge tone="neutral">执行时检查</AppBadge>
          <span class="chip">deep_persist</span>
          <span class="lamp lamp--ok" aria-hidden="true"></span>
          <span class="lamp lamp--warn" aria-hidden="true"></span>
          <span class="lamp lamp--danger" aria-hidden="true"></span>
        </div>
      </div>
    </AppCard>

    <AppCard title="表单">
      <div class="stack">
        <div class="cluster primitive-page__toggles">
          <AppToggle v-model="toggles.on">已开启的开关</AppToggle>
          <AppToggle v-model="toggles.off" note="明文">允许 Telnet</AppToggle>
          <AppToggle :model-value="false" disabled>禁用的开关</AppToggle>
        </div>
        <div class="primitive-page__fields">
          <TextInput id="primitive-text" v-model="text" label="路由器地址" mono required />
          <TextInput
            id="primitive-error"
            model-value="999999"
            label="Web 端口"
            mono
            error="端口必须在 1–65535 之间"
          />
          <TextInput id="primitive-disabled" model-value="auto" label="已禁用" disabled />
          <SecretInput
            id="primitive-secret"
            v-model="secret"
            label="秘密输入"
            hint="不会保存到本地。"
            required
          />
        </div>
        <TextInput id="primitive-long" v-model="longIdentifier" label="80 个字符的连续标识" mono />
      </div>
    </AppCard>

    <AppAlert type="info" title="提示" message="备份不是原子快照，不保证可救砖。" />
    <AppAlert type="warning" title="警告" message="NVRAM 开关和 root 密码不会自动恢复。" />
    <AppAlert
      type="error"
      title="可能导致设备无法启动"
      :message="'中断、断电或固件差异都可能让路由器无法启动。'.repeat(4)"
    />

    <div class="tile-grid">
      <StatTile
        tone="ok"
        title="Web 管理"
        meta="HTTP 80"
        value="已验证"
        detail="Web 管理密码正确"
        :icon="Globe"
      />
      <StatTile
        tone="info"
        title="SSH"
        meta="22"
        value="端口开放"
        detail="未验证 root · 开放端口不算认证"
        :icon="Terminal"
      />
      <StatTile
        tone="warn"
        title="Telnet"
        meta="23"
        value="端口开放"
        detail="明文协议"
        :icon="Server"
      />
      <StatTile
        tone="highlight"
        title="下一步"
        value="开启 SSH"
        detail="需要 Web 管理密码和新的 root 密码"
        :icon="ChevronRight"
      />
      <StatTile
        tone="danger"
        title="Web 管理"
        meta="HTTP 80"
        value="无法连接"
        detail="检查地址、端口和网络后重试"
        :icon="Globe"
      />
      <StatTile
        tone="neutral"
        title="Telnet"
        meta="23"
        value="已关闭"
        detail="只在明确允许的步骤中使用"
        :icon="Server"
      />
    </div>

    <AppCard title="引导元件">
      <div class="stack">
        <DeviceSelector id="primitive-device" v-model="profileChoice" :catalog="sampleCatalog" />
        <DeviceSelector id="primitive-stale" v-model="staleProfile" :catalog="sampleCatalog" />
        <div class="cluster">
          <CredentialState kind="web-admin" state="saved" />
          <CredentialState kind="root" state="rejected" />
          <CredentialState kind="root" state="none" />
        </div>
        <PlanStepList :steps="planSteps" />
        <PlanStepList :steps="blockedSteps" />
        <PlanStepList :results="results" />
      </div>
    </AppCard>
    <AppCard title="步骤与记录">
      <div class="stack">
        <AppStepper :steps="['前置证据', '凭据与明文风险', '最终确认']" :current="2" />
        <HistoryList
          :rows="rows"
          selectable
          :selected="selected"
          @select="(id) => (selected = id)"
        />
      </div>
    </AppCard>

    <AppCard title="进度与报告">
      <div class="stack">
        <OperationProgress status="unknown" :events="events" />
        <ReportViewer :report="report" />
      </div>
    </AppCard>

    <EmptyState title="尚无报告" message="先运行设备预检。" />
    <ConfirmDialog
      v-model="dialogOpen"
      title="深度固化"
      description="这会写入并擦除分区，且重启三次。"
      require-text="RD08-DEEP"
      danger
      confirm-label="确认执行"
      @confirm="dialogOpen = false"
    />
  </div>
</template>

<style scoped>
.primitive-page__note {
  color: var(--rt-fg-muted);
  font-size: var(--rt-text-xs);
}
.primitive-page__toggles {
  gap: var(--rt-space-2) var(--rt-space-5);
}
.primitive-page__fields {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(min(100%, var(--rt-field-min)), 1fr));
  gap: var(--rt-space-3);
}
</style>
