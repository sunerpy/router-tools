<script setup lang="ts">
import { computed, inject, onBeforeUnmount, onMounted, ref } from "vue";
import { RouterLink } from "vue-router";
import {
  ChevronRight,
  CircleDot,
  Copy,
  FileText,
  FolderOpen,
  ScanSearch,
  ShieldAlert,
} from "lucide-vue-next";
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
import { required, targetRules, useFieldErrors } from "../composables/useFieldErrors";
import { newOperationID, toPort } from "../composables/useOperations";
import { CREDENTIAL_KIND, CREDENTIAL_NOTE, DEEP_BLOCKED } from "../services/guideText";
import { displayStatus, formatTime, latestEntry } from "../services/summary";
import type { DeepStatusReport, RouterAPI } from "../services/api";

const store = inject("operations") as any;
const api = inject("api") as RouterAPI;
const step = ref(1);
const dialogOpen = ref(false);
// The deep flow's Telnet acknowledgement is its own, always unset when the page opens: no other
// page's option may pre-satisfy it (DESIGN.md §4).
const telnetAcknowledged = ref(false);
const status = ref<DeepStatusReport | null>(null);
const statusError = ref("");
const checking = ref(false);
const statusRoot = ref("");
const useSaved = ref(true);
const { errors, check } = useFieldErrors();

onMounted(() => {
  store.refreshHistory().catch(() => {});
  store.runDir = "";
});
onBeforeUnmount(() => {
  telnetAcknowledged.value = false;
  statusRoot.value = "";
  store.clearSecrets();
});

const policy = computed(() => status.value?.profile.deep ?? null);
const phrase = computed(() => policy.value?.acknowledgement ?? "");
const reboots = computed(() => policy.value?.reboot_transitions ?? 0);

async function readStatus() {
  if (!check(targetRules(store.target))) return;
  checking.value = true;
  statusError.value = "";
  try {
    status.value = await api.DeepStatus({
      OperationID: newOperationID(),
      Target: {
        host: store.target.host,
        web_port: toPort(store.target.web_port),
        ssh_port: toPort(store.target.ssh_port),
        telnet_port: toPort(store.target.telnet_port),
      },
      ProfileID: store.profileID,
      AdminPassword: "",
      RootPassword: statusRoot.value,
      Transport: "ssh",
      AllowTelnet: false,
      Confirmed: false,
      WithAdminAuth: false,
      WithRootVerify: false,
      OutputDir: "",
      BackupDir: "",
      RunDir: "",
      AckDeep: "",
      ToolVersion: "dev",
      HostKey: { ...store.hostKey },
      UseSavedPassword: useSaved.value,
      RememberPassword: false,
    });
    if (status.value?.error)
      statusError.value = status.value.error.message || status.value.error.code;
  } catch (caught) {
    status.value = null;
    statusError.value = caught instanceof Error ? caught.message : String(caught);
  } finally {
    statusRoot.value = "";
    checking.value = false;
  }
}

async function pickBackup() {
  try {
    const directory = await api.SelectDirectory();
    if (directory) store.backupDir = directory;
  } catch {
    // The dialog is optional; the path can be typed.
  }
}

function confirmEvidence() {
  if (!phrase.value) {
    statusError.value = "先读取深度固化状态，以确定这台设备的确认短语和重启次数。";
    return;
  }
  if (check([required("deep-backup", store.backupDir, "请选择已通过校验的当前启动备份目录")]))
    step.value = 2;
}

// Canceling the final confirmation clears the credentials and the acknowledgement (DESIGN.md §4).
function cancelFinal() {
  store.clearSecrets();
  telnetAcknowledged.value = false;
}

const FLAGS: Record<string, string> = { none: "未设置", all_set: "已全部设置", mixed: "部分设置" };
// Root verification and inspection need an open SSH port and a root password that SSH accepted.
const rootText = computed(() => {
  const value = status.value;
  if (!value) return "";
  if (value.ssh && value.ssh !== "open") return "SSH 端口没有开放，无法检查";
  if (value.root_ssh === "auth_failed") return "root 密码被拒绝";
  if (value.root_ssh === "verified") return "已验证";
  return "未检查（需要 root 密码）";
});

function runDeep() {
  dialogOpen.value = false;
  const allowTelnet = telnetAcknowledged.value;
  telnetAcknowledged.value = false;
  // The managed journal directory is always used: an empty run directory selects it in Go.
  void store.run("DeepPersist", {
    AllowTelnet: allowTelnet,
    RunDir: "",
    UseSavedPassword: false,
    RememberPassword: false,
  });
}

const ready = computed(
  () =>
    Boolean(phrase.value) &&
    telnetAcknowledged.value &&
    Boolean(store.backupDir) &&
    Boolean(store.rootPassword) &&
    store.ackDeep === phrase.value,
);
const stepperCurrent = computed(() => (dialogOpen.value ? 3 : step.value));

// The checks the tool performs before the first write (RISKS.md). They are not shown as passed:
// only the report of a run carries that evidence.
const checks = [
  { title: "备份目录通过本地完整性校验", detail: "manifest、SHA256SUMS、每个分区的长度与哈希" },
  { title: "在线身份、完整 MTD 布局与 boot_id 一致", detail: "与备份记录逐项比对" },
  { title: "托管目录中没有这台路由器的 journal", detail: "已有 journal 时拒绝从头开始" },
  { title: "所需工具存在且 crash 分区可读", detail: "缺任何一项都在写入前停止" },
];

const lastRun = computed(() => latestEntry(store.history, "deep-persist"));
const lastReport = computed(() => lastRun.value?.report as any);
const nonRetry = computed(() => ["unknown", "partial"].includes(lastReport.value?.status));
const copied = ref(false);
async function copyJournal(path: string) {
  await navigator.clipboard.writeText(path);
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
        中断后不要从头重跑：保留 journal、运行报告和备份目录，按 RECOVERY.md 人工检查。
      </p>
    </AppAlert>

    <AppCard eyebrow="只读" title="深度固化状态">
      <div class="stack">
        <div class="deep__status-form">
          <SecretInput
            id="deep-status-root"
            v-model="statusRoot"
            label="root 密码（用于只读检查，可选）"
            :hint="
              useSaved ? '留空时使用已保存的 root 密码；只走 SSH。' : '只走 SSH，不做任何写入。'
            "
          />
          <AppToggle v-model="useSaved" note="只用于只读检查">使用已保存的密码</AppToggle>
          <div class="cluster">
            <AppButton :loading="checking" :disabled="store.busy" @click="readStatus">
              <ScanSearch :size="16" aria-hidden="true" />读取状态
            </AppButton>
          </div>
        </div>
        <AppAlert v-if="statusError" type="error" title="状态没有读取完整" :message="statusError" />
        <template v-if="status && !status.error">
          <dl class="deep__facts">
            <dt>Profile</dt>
            <dd class="mono">{{ status.profile.id }}</dd>
            <dt>确认短语</dt>
            <dd class="mono">{{ phrase }}</dd>
            <dt>重启次数</dt>
            <dd>{{ reboots }} 次</dd>
            <dt>root SSH</dt>
            <dd>{{ rootText }}</dd>
            <dt>启动标志</dt>
            <dd>{{ status.inspection ? FLAGS[status.inspection.flags] : "未检查" }}</dd>
            <dt>Journal</dt>
            <dd class="deep__journal">
              <FileText :size="16" aria-hidden="true" />
              <span class="mono deep__journal-path">{{ status.journal_path }}</span>
              <AppBadge :tone="status.journal_exists ? 'warn' : 'neutral'">{{
                status.journal_exists ? "已存在" : "尚无"
              }}</AppBadge>
            </dd>
          </dl>
          <p
            v-for="note in status.credentials ?? []"
            :key="note.kind + note.action"
            class="deep__hint"
          >
            {{ CREDENTIAL_KIND[note.kind] ?? note.kind }}：{{
              CREDENTIAL_NOTE[note.action] ?? note.action
            }}
          </p>
          <ul v-if="status.blocked.length" class="deep__blocked">
            <li v-for="reason in status.blocked" :key="reason">
              <ShieldAlert :size="16" aria-hidden="true" />{{ DEEP_BLOCKED[reason] ?? reason }}
            </li>
          </ul>
          <AppBadge v-else-if="status.ready" tone="ok">读取到的前置条件都满足</AppBadge>
        </template>
      </div>
    </AppCard>

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
              <li v-for="item in checks" :key="item.title" class="deep__check">
                <CircleDot :size="18" aria-hidden="true" />
                <div>
                  {{ item.title }}<span class="deep__check-detail">{{ item.detail }}</span>
                </div>
                <AppBadge tone="neutral">执行时检查</AppBadge>
              </li>
            </ul>
            <fieldset v-if="status?.backups.length" class="deep__backups">
              <legend>托管备份（已通过本地校验）</legend>
              <label v-for="item in status.backups" :key="item.path" class="deep__backup">
                <input
                  v-model="store.backupDir"
                  type="radio"
                  name="deep-backup-choice"
                  :value="item.path"
                />
                <span>{{ formatTime(item.created_at) }}</span>
                <span class="mono deep__journal-path">{{ item.path }}</span>
              </label>
            </fieldset>
            <div class="deep__path">
              <TextInput
                id="deep-backup"
                v-model="store.backupDir"
                label="已通过校验的当前启动备份目录"
                :error="errors['deep-backup']"
                mono
                required
              />
              <AppButton @click="pickBackup"
                ><FolderOpen :size="16" aria-hidden="true" />选择目录</AppButton
              >
            </div>
            <p class="deep__hint">
              journal 固定写入上面显示的托管目录，不能更换目录来绕过已有 journal。
            </p>
            <p class="deep__hint">
              备份必须在路由器这次启动之后创建；路由器重启过（包括向导中的重启验证）就需要新的备份。可以先在「维护」中在线比对；已有托管备份时向导不会再备份，需要新备份时运行
              <span class="mono">routerctl backup create</span>。
            </p>
            <div class="cluster">
              <AppButton variant="primary" @click="confirmEvidence"
                >确认备份与恢复边界<ChevronRight :size="16" aria-hidden="true"
              /></AppButton>
            </div>
          </template>

          <template v-else>
            <SecretInput
              id="deep-root"
              v-model="store.rootPassword"
              label="Root 密码"
              hint="深度固化不会使用已保存的密码。"
              required
            />
            <AppToggle v-model="telnetAcknowledged" note="明文"
              >我了解 Telnet 为明文协议，并明确允许中间阶段使用</AppToggle
            >
            <TextInput
              id="deep-ack"
              v-model="store.ackDeep"
              :label="`输入 ${phrase} 确认 ${reboots} 次重启与分区写擦`"
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
              @click="copyJournal(lastReport.journal_path)"
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
      :description="`仅在独立授权、供电稳定且已有官方恢复方案时继续。设备会重启 ${reboots} 次；写超时不会自动重试。`"
      :require-text="phrase"
      danger
      confirm-label="执行深度固化"
      @confirm="runDeep"
      @cancel="cancelFinal"
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
  margin: 0;
  color: var(--rt-fg-muted);
  font-size: var(--rt-text-xs);
}
.deep__status-form {
  display: grid;
  gap: var(--rt-space-3);
}
.deep__facts {
  display: grid;
  grid-template-columns: max-content minmax(0, 1fr);
  gap: var(--rt-space-2) var(--rt-space-4);
  margin: 0;
}
.deep__facts dt {
  color: var(--rt-fg-muted);
}
.deep__facts dd {
  margin: 0;
  min-width: 0;
  overflow-wrap: anywhere;
}
.deep__blocked {
  display: grid;
  gap: var(--rt-space-2);
  margin: 0;
  padding: 0;
  list-style: none;
  color: var(--rt-warning-text);
}
.deep__blocked li {
  display: flex;
  align-items: flex-start;
  gap: var(--rt-space-2);
}
.deep__backups {
  display: grid;
  gap: var(--rt-space-2);
  margin: 0;
  padding: var(--rt-space-3);
  border: var(--rt-border-width) solid var(--rt-border);
  border-radius: var(--rt-r-control);
}
.deep__backups legend {
  padding: 0 var(--rt-space-1);
  color: var(--rt-fg-muted);
  font-size: var(--rt-text-sm);
}
.deep__backup {
  display: grid;
  grid-template-columns: auto auto minmax(0, 1fr);
  align-items: center;
  gap: var(--rt-space-2);
}
.deep__path {
  display: grid;
  grid-template-columns: minmax(0, 1fr) auto;
  align-items: end;
  gap: var(--rt-space-2);
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
  .deep__path,
  .deep__backup {
    grid-template-columns: minmax(0, 1fr);
  }
}
</style>
