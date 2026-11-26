<script setup lang="ts">
import { computed, inject, onBeforeUnmount, onMounted, ref, type Ref } from "vue";
import { RouterLink } from "vue-router";
import { ClipboardCheck, Play, ScanSearch, ShieldAlert, Square } from "lucide-vue-next";
import AppAlert from "../components/AppAlert.vue";
import AppButton from "../components/AppButton.vue";
import AppCard from "../components/AppCard.vue";
import AppToggle from "../components/AppToggle.vue";
import ConfirmDialog from "../components/ConfirmDialog.vue";
import CredentialState from "../components/CredentialState.vue";
import DeviceSelector from "../components/DeviceSelector.vue";
import OperationProgress from "../components/OperationProgress.vue";
import PlanStepList from "../components/PlanStepList.vue";
import SecretInput from "../components/SecretInput.vue";
import StatusBadge from "../components/StatusBadge.vue";
import TargetForm from "../components/TargetForm.vue";
import { profileRule, targetRules, useFieldErrors } from "../composables/useFieldErrors";
import { STEP_TITLE, effectText, errorTitle } from "../services/guideText";
import { formatTime, guideStatus, operationLabel } from "../services/summary";
import type { CatalogEntry, GuideStep, GuideStepID, ProfileMatch } from "../services/api";
import type { useGuide } from "../composables/useGuide";

type Guide = ReturnType<typeof useGuide>;

const store = inject("operations") as any;
const guide = inject("guide") as Guide;
const catalog = inject("catalog") as Ref<CatalogEntry[]>;
const { errors, check } = useFieldErrors();
const confirmOpen = ref(false);
const ackOpen = ref(false);

onMounted(() => {
  guide.resetChoices();
  void guide.loadCredentials();
});
onBeforeUnmount(() => guide.clearSecrets());

const plan = computed(() => guide.plan);
const status = computed(() => guideStatus(guide.phase, guide.run, guide.error));
const OUTCOME_TEXT: Record<string, string> = {
  in_progress: "没有正常结束",
  partial: "部分完成",
  unknown: "结果未知",
  failed: "失败且可能已写入",
};
const recoveryMessage = computed(() => {
  const record = plan.value?.state.recovery;
  const what = record ? operationLabel(record.operation) : "操作";
  const how = record ? (OUTCOME_TEXT[record.status] ?? record.status) : "结果不确定";
  return `${plan.value?.host ?? ""} 上次的「${what}」${how}。请按 RECOVERY.md 人工检查路由器，确认后才能继续修改。`;
});
const recoveryTimes = computed(() => {
  const record = plan.value?.state.recovery;
  if (!record) return "";
  const parts: string[] = [];
  if (record.started_at) parts.push(`开始于 ${formatTime(record.started_at)}`);
  if (record.finished_at) parts.push(`结束于 ${formatTime(record.finished_at)}`);
  if (record.acknowledged_at) parts.push(`上次确认于 ${formatTime(record.acknowledged_at)}`);
  return parts.join("，");
});
const recoveryBlocked = computed(() => plan.value?.error?.code === "RECOVERY_REVIEW_REQUIRED");
const enableStep = computed(() => plan.value?.steps.find((step) => step.id === "enable_ssh"));
const asksCurrentRoot = computed(() => enableStep.value?.action === "needs_input");
// A plan is confirmed once: after its run the router has changed, so the page offers a new detection.
// Editing what the plan was detected with also takes the run away until the next detection.
const runnable = computed(
  () =>
    Boolean(plan.value?.ready && plan.value?.changes && plan.value?.confirmation) &&
    guide.phase !== "done" &&
    !guide.stale,
);
// The confirmation names the router it covers: the target and identity the plan was detected for.
const confirmTarget = computed(() => {
  const confirmation = plan.value?.confirmation;
  if (!confirmation) return "";
  const { target, identity } = confirmation;
  return `${target.host}（Web ${target.web_port} · SSH ${target.ssh_port}）· ${identity.hardware} 固件 ${identity.rom_version} · ${identity.profile_id}`;
});
const nothingToDo = computed(() =>
  Boolean(plan.value?.ready && !plan.value?.changes && !plan.value?.error),
);
const exactCandidates = computed(() =>
  (plan.value?.candidates ?? []).filter((candidate: ProfileMatch) => candidate.exact),
);
const writes = computed(() =>
  (plan.value?.steps ?? [])
    .filter((step: GuideStep) => step.action === "run" || step.action === "decide_after")
    .map((step: GuideStep) => ({
      id: step.id as GuideStepID,
      later: step.action === "decide_after",
      effects: (step.effects ?? []).map(effectText),
    })),
);

function detect() {
  const rules = [
    ...targetRules(store.target),
    profileRule("device-model", store.profileID, catalog.value),
  ];
  if (check(rules)) void guide.detect();
}

function requestRun() {
  const fieldErrors = guide.validate();
  const rules = Object.entries(fieldErrors).map(
    ([id, message]) => [id, false, message as string] as [string, boolean, string],
  );
  if (check(rules)) confirmOpen.value = true;
}

function run() {
  confirmOpen.value = false;
  void guide.apply();
}

function acknowledge() {
  ackOpen.value = false;
  void guide.acknowledge();
}

function switchProfile(id: string) {
  store.profileID = id;
  void guide.detect();
}

const stateRows = computed(() => {
  const value = plan.value;
  if (!value) return [];
  const soft: Record<string, string> = {
    not_installed: "未安装",
    installed_verified: "已安装并通过校验",
    file_drift: "脚本内容漂移",
    mode_drift: "脚本不可执行",
    uci_drift: "防火墙配置漂移",
    conflict: "外来配置",
    orphan_file: "残留脚本",
  };
  const root: Record<string, string> = {
    verified: "已验证",
    auth_failed: "密码被拒绝",
    not_checked: "未检查",
  };
  const port: Record<string, string> = { open: "开放", closed: "关闭", unreachable: "不可达" };
  const goals = value.goals ?? [];
  let softText = "未选择";
  if (goals.includes("soft")) {
    softText = value.state.soft ? (soft[value.state.soft] ?? value.state.soft) : "SSH 可用后检查";
  }
  let backupText = goals.includes("backup") ? "没有" : "未选择";
  if (value.state.backup) backupText = formatTime(value.state.backup.created_at);
  return [
    ["设备", value.device.display_name || value.device.hardware || "未识别"],
    ["固件", value.device.rom_version || "未知"],
    ["Profile", value.profile.id || "未匹配"],
    ["SSH 端口", (value.state.ssh && port[value.state.ssh]) || "未探测"],
    ["root SSH", root[value.state.root_ssh] ?? value.state.root_ssh],
    ["软固化", softText],
    ["托管备份", backupText],
  ];
});
</script>

<template>
  <div class="stack">
    <div class="split">
      <AppCard eyebrow="第 1 步" title="选择路由器">
        <div class="stack">
          <DeviceSelector
            id="device"
            v-model="store.profileID"
            :catalog="catalog"
            :disabled="guide.busy"
            :error="errors['device-model']"
          />
          <TargetForm :errors="errors" />
        </div>
      </AppCard>
      <AppCard eyebrow="第 2 步" title="要完成的目标">
        <div class="stack">
          <AppToggle :model-value="true" disabled note="必选">开启并验证 SSH</AppToggle>
          <AppToggle v-model="guide.goals.backup" :disabled="guide.busy">备份全部分区</AppToggle>
          <AppToggle v-model="guide.goals.soft" :disabled="guide.busy"
            >软固化（重启后保持 SSH）</AppToggle
          >
          <AppToggle
            v-model="guide.goals.reboot"
            :disabled="guide.busy || !guide.goals.soft"
            note="会重启路由器，网络中断几分钟"
            >重启验证</AppToggle
          >
          <AppToggle v-model="guide.useSaved" :disabled="guide.busy" note="不会显示密码"
            >使用已保存的密码</AppToggle
          >
          <div class="cluster">
            <CredentialState kind="web-admin" :state="guide.savedState('web-admin')" />
            <CredentialState kind="root" :state="guide.savedState('root')" />
          </div>
          <SecretInput
            id="guide-detect-root"
            v-model="guide.rootPassword"
            label="当前 root 密码（可选）"
            hint="SSH 已开启时用于确认 root 登录；首次开启时留空。检测发出后输入框会清空。"
            autocomplete="current-password"
          />
          <div class="cluster">
            <AppButton
              variant="primary"
              :loading="guide.phase === 'detecting'"
              :disabled="guide.busy"
              @click="detect"
              ><component :is="ScanSearch" :size="16" aria-hidden="true" />检测
            </AppButton>
            <AppButton v-if="guide.phase === 'detecting'" @click="guide.cancel()"
              ><component :is="Square" :size="16" aria-hidden="true" />取消检测</AppButton
            >
            <span class="guide__hint">只读取设备状态，不做任何修改。</span>
          </div>
        </div>
      </AppCard>
    </div>

    <AppAlert
      v-if="guide.planChanged"
      type="warning"
      title="路由器状态已变化"
      message="确认之后路由器发生了变化，没有执行任何写入。下面是重新检测的结果，请重新确认。"
    />
    <AppAlert
      v-if="guide.error && !plan"
      type="error"
      title="检测没有完成"
      :message="guide.error"
    />
    <AppAlert
      v-if="plan && guide.stale && guide.phase !== 'running'"
      type="warning"
      title="检测之后改了设置"
      message="地址、端口、设备、目标或「使用已保存的密码」与检测时不同。重新检测后才能执行。"
    >
      <div class="cluster">
        <AppButton :disabled="guide.busy" @click="detect"
          ><ScanSearch :size="16" aria-hidden="true" />重新检测</AppButton
        >
      </div>
    </AppAlert>

    <template v-if="plan">
      <AppAlert
        v-if="recoveryBlocked"
        type="warning"
        title="上次变更的结果不确定"
        :message="recoveryMessage"
      >
        <p v-if="recoveryTimes" class="guide__hint">{{ recoveryTimes }}</p>
        <p v-if="guide.ackError" class="guide__error" role="alert">
          确认没有完成：{{ guide.ackError }}
        </p>
        <div class="cluster">
          <AppButton @click="ackOpen = true"
            ><component
              :is="ClipboardCheck"
              :size="16"
              aria-hidden="true"
            />我已检查路由器</AppButton
          >
          <RouterLink to="/reports">查看运行报告</RouterLink>
        </div>
      </AppAlert>
      <AppAlert
        v-else-if="plan.error"
        type="error"
        :title="errorTitle(plan.error.code)"
        :message="`${plan.error.code}：${plan.error.message}`"
      >
        <div v-if="plan.candidates?.length" class="stack">
          <p class="guide__hint">
            该型号已注册的固件：{{ plan.candidates.map((c) => c.profile.firmware).join("、") }}
          </p>
          <div v-if="exactCandidates.length" class="cluster">
            <AppButton
              v-for="candidate in exactCandidates"
              :key="candidate.profile.id"
              @click="switchProfile(candidate.profile.id)"
            >
              切换到 {{ candidate.profile.id }} 并重新检测
            </AppButton>
          </div>
        </div>
      </AppAlert>

      <div class="split">
        <AppCard eyebrow="计划" title="将要做的事">
          <div class="stack">
            <PlanStepList :steps="plan.steps" />
            <p v-if="nothingToDo" class="guide__done">所选目标都已完成，不需要任何修改。</p>
          </div>
        </AppCard>
        <AppCard eyebrow="检测结果" title="路由器现状">
          <dl class="guide__facts">
            <template v-for="[label, value] in stateRows" :key="label">
              <dt>{{ label }}</dt>
              <dd :class="{ mono: label === 'Profile' || label === '固件' }">{{ value }}</dd>
            </template>
          </dl>
          <p class="guide__hint mono">{{ plan.host }}</p>
        </AppCard>
      </div>

      <AppCard v-if="asksCurrentRoot" eyebrow="需要输入" title="确认当前 root 密码">
        <div class="stack">
          <p class="guide__hint">
            SSH 已开启。输入当前 root
            密码后重新检测；不知道当前密码时，可以选择通过开启流程把它改为新密码。
          </p>
          <SecretInput
            id="guide-current-root"
            v-model="guide.rootPassword"
            label="当前 root 密码"
            autocomplete="current-password"
          />
          <AppToggle v-model="guide.resetRoot" note="会修改 root 密码"
            >不知道当前密码，重置为新密码</AppToggle
          >
          <div class="cluster">
            <AppButton variant="primary" :disabled="guide.busy" @click="detect"
              ><component :is="ScanSearch" :size="16" aria-hidden="true" />重新检测</AppButton
            >
          </div>
        </div>
      </AppCard>

      <AppCard v-if="runnable" eyebrow="第 3 步" title="确认并执行">
        <div class="stack">
          <template v-if="guide.needs.admin">
            <SecretInput
              id="guide-admin"
              v-model="guide.adminPassword"
              label="Web 管理密码"
              :error="errors['guide-admin']"
              autocomplete="current-password"
              required
            />
          </template>
          <template v-if="guide.needs.newRoot">
            <SecretInput
              id="guide-new-root"
              v-model="guide.newRootPassword"
              label="新 root 密码"
              :error="errors['guide-new-root']"
              autocomplete="new-password"
              required
            />
            <SecretInput
              id="guide-new-root-repeat"
              v-model="guide.newRootRepeat"
              label="再次输入新 root 密码"
              :error="errors['guide-new-root-repeat']"
              autocomplete="new-password"
              required
            />
          </template>
          <SecretInput
            v-if="guide.needs.currentRoot"
            id="guide-root"
            v-model="guide.rootPassword"
            label="当前 root 密码"
            :error="errors['guide-root']"
            autocomplete="current-password"
            required
          />
          <AppToggle v-model="guide.remember" note="路由器接受后加密保存在本机">记住密码</AppToggle>
          <div class="cluster">
            <AppButton variant="primary" :disabled="guide.busy" @click="requestRun"
              ><component :is="Play" :size="16" aria-hidden="true" />执行</AppButton
            >
            <span class="guide__hint"
              >执行前会再次检测；遇到第一个未成功的步骤即停止，不会自动重试。</span
            >
          </div>
        </div>
      </AppCard>
    </template>

    <AppCard
      v-if="guide.phase === 'running' || guide.run || (guide.phase === 'done' && guide.error)"
      eyebrow="执行"
      title="执行结果"
    >
      <template #actions>
        <StatusBadge :status="status" />
      </template>
      <div class="stack">
        <template v-if="guide.phase === 'running'">
          <div v-for="step in writes" :key="step.id" class="stack">
            <strong>{{ STEP_TITLE[step.id] }}</strong>
            <OperationProgress :events="guide.stepEvents[step.id] ?? []" />
          </div>
          <div class="cluster">
            <AppButton @click="guide.cancel()"
              ><component :is="Square" :size="16" aria-hidden="true" />取消运行</AppButton
            >
            <span class="guide__hint">可能已经写入时，结果保留为部分完成或结果未知。</span>
          </div>
        </template>
        <AppAlert v-else-if="!guide.run" type="error" title="执行没有开始" :message="guide.error" />
        <template v-else>
          <AppAlert
            v-if="guide.run.error && guide.run.status === 'failed'"
            type="error"
            :title="errorTitle(guide.run.error.code, '执行没有完成')"
            :message="`${guide.run.error.code}：${guide.run.error.message}`"
          />
          <PlanStepList :results="guide.run.steps" />
          <AppAlert
            v-if="guide.run.status === 'partial' || guide.run.status === 'unknown'"
            type="warning"
            title="不要从头重跑"
            :message="`部分步骤已经修改了路由器。查看各步骤的报告，按 RECOVERY.md 检查后再继续。${guide.run.error ? `（${guide.run.error.code}）` : ''}`"
          />
          <div class="cluster">
            <AppButton :disabled="guide.busy" @click="detect"
              ><ScanSearch :size="16" aria-hidden="true" />重新检测</AppButton
            >
            <RouterLink to="/reports">在运行报告中查看完整记录</RouterLink>
          </div>
        </template>
      </div>
    </AppCard>

    <ConfirmDialog
      v-model="confirmOpen"
      title="确认执行"
      description="将按顺序执行以下步骤，遇到第一个未成功的步骤即停止。"
      confirm-label="确认执行"
      @confirm="run"
      @cancel="guide.clearSecrets()"
    >
      <p class="guide__target mono">{{ confirmTarget }}</p>
      <ol class="guide__writes">
        <li v-for="step in writes" :key="step.id">
          <strong>{{ STEP_TITLE[step.id] }}</strong>
          <span v-if="step.later" class="guide__hint">（SSH 可用后仍需要时才执行）</span>
          <ul>
            <li v-for="effect in step.effects" :key="effect">{{ effect }}</li>
          </ul>
        </li>
      </ol>
    </ConfirmDialog>
    <ConfirmDialog
      v-model="ackOpen"
      title="确认已检查路由器"
      :description="`确认只记录你已检查过 ${plan?.host ?? ''} 的实际状态，不会修复任何东西。之后的修改会从这个状态继续。`"
      confirm-label="我已检查"
      @confirm="acknowledge"
    >
      <p class="guide__hint">
        <ShieldAlert :size="16" aria-hidden="true" /> 不确定时不要确认，先查看运行报告和
        RECOVERY.md。
      </p>
    </ConfirmDialog>
  </div>
</template>

<style scoped>
.guide__hint {
  margin: 0;
  color: var(--rt-fg-muted);
  overflow-wrap: anywhere;
}
.guide__error {
  margin: 0;
  color: var(--rt-danger-text);
  overflow-wrap: anywhere;
}
.guide__done {
  margin: 0;
  color: var(--rt-ok-text);
}
.guide__facts {
  display: grid;
  grid-template-columns: max-content minmax(0, 1fr);
  gap: var(--rt-space-2) var(--rt-space-4);
  margin: 0;
}
.guide__facts dt {
  color: var(--rt-fg-muted);
}
.guide__facts dd {
  margin: 0;
  overflow-wrap: anywhere;
}
.guide__target {
  margin: 0;
  overflow-wrap: anywhere;
}
.guide__writes {
  display: grid;
  gap: var(--rt-space-3);
  margin: 0;
  padding-left: var(--rt-space-5);
}
.guide__writes ul {
  margin: var(--rt-space-1) 0 0;
  padding-left: var(--rt-space-5);
  color: var(--rt-fg-muted);
}
</style>
