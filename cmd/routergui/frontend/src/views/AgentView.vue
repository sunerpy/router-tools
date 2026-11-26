<script setup lang="ts">
import { computed, inject, onMounted, onUnmounted, ref, watch } from "vue";
import { Bot, FolderOutput, KeyRound, PackagePlus, RefreshCw } from "lucide-vue-next";
import AppAlert from "../components/AppAlert.vue";
import AppButton from "../components/AppButton.vue";
import AppCard from "../components/AppCard.vue";
import AppSegmented from "../components/AppSegmented.vue";
import AppToggle from "../components/AppToggle.vue";
import CodeBlock from "../components/CodeBlock.vue";
import ConfirmDialog from "../components/ConfirmDialog.vue";
import EmptyState from "../components/EmptyState.vue";
import FormField from "../components/FormField.vue";
import HistoryList from "../components/HistoryList.vue";
import TextInput from "../components/TextInput.vue";
import { SKILL_HOSTS, STDIO_ONLY_HOSTS, type AgentsStore } from "../composables/useAgents";
import {
  agentActivityRows,
  agentOperationRows,
  isLoopback,
  listenOptions,
} from "../services/agents";
import { formatTime } from "../services/summary";

const agents = inject("agents") as AgentsStore;
const rotateOpen = ref(false);
const cancelID = ref("");
const forceOpen = computed({
  get: () => agents.pendingForce !== null,
  set: (open: boolean) => {
    if (!open) agents.cancelForce();
  },
});
const cancelOpen = computed({
  get: () => cancelID.value !== "",
  set: (open: boolean) => {
    if (!open) cancelID.value = "";
  },
});

onMounted(async () => {
  await agents.load();
  await agents.selectHost(agents.host);
});
// The token is shown once: leaving the page forgets it, like a confirmation that was never answered.
onUnmounted(() => {
  agents.dismissToken();
  agents.cancelForce();
});
watch(
  () => agents.status?.url,
  (url, previous) => {
    if (url && previous && url !== previous) void agents.selectHost(agents.host);
  },
);

const listener = computed(() => agents.status?.listener);
const statusTone = computed(() => {
  if (listener.value?.listening) return "ok";
  return listener.value?.enabled ? "danger" : "neutral";
});
const statusText = computed(() => {
  const current = listener.value;
  if (!current) return "正在读取状态";
  if (current.listening) {
    return current.allow_writes
      ? `正在监听 ${current.address} · Agent 可以写入路由器`
      : `正在监听 ${current.address} · 只读`;
  }
  if (current.enabled) return "已开启，但没有在监听";
  return "已关闭";
});
const addressOptions = computed(() =>
  listenOptions(agents.status?.local_addresses ?? [], agents.draft.listen_host),
);
const lanWarning = computed(() => !isLoopback(agents.draft.listen_host));
const hostOptions = computed(() =>
  (agents.status?.agents ?? []).map((host) => ({
    value: host.id,
    label: host.id === "generic" ? "其他客户端" : host.name,
  })),
);
const hostName = computed(
  () => hostOptions.value.find((option) => option.value === agents.host)?.label ?? agents.host,
);
const stdioOnly = computed(() => STDIO_ONLY_HOSTS.includes(agents.host));
const hasSkillDir = computed(() => SKILL_HOSTS.includes(agents.host));
// Go resolves the target with the hosts' override variables, exactly as the installation does.
const skillTarget = computed(() => agents.status?.skill_paths?.[agents.host] ?? "");
const skillAlert = computed(() => {
  const report = agents.skillReport;
  if (!report || report.status === "differs") return null;
  switch (report.status) {
    case "installed":
      return { type: "info" as const, title: "Skill 已安装", message: report.path };
    case "updated":
      return { type: "info" as const, title: "Skill 已替换为当前版本", message: report.path };
    case "unchanged":
      return { type: "info" as const, title: "已安装的 Skill 就是当前版本", message: report.path };
    default:
      return {
        type: "error" as const,
        title: "没有安装 Skill",
        message: report.message || report.path,
      };
  }
});
const operationRows = computed(() => agentOperationRows(agents.status?.operations ?? []));
const runningIDs = computed(
  () =>
    new Set(
      (agents.status?.operations ?? [])
        .filter((operation) => !operation.done)
        .map((operation) => operation.operation_id),
    ),
);
const activityRows = computed(() => agentActivityRows(agents.status?.activity ?? [], 20));
// Turning the service off only disconnects agents; operations they started keep running until they end or are
// canceled (the listener and the operations have separate lifetimes).
const continuesNotice = computed(() => runningIDs.value.size > 0 && !agents.draft.enabled);

function rotate() {
  if (listener.value?.token_set) {
    rotateOpen.value = true;
    return;
  }
  void agents.rotate();
}
function confirmRotate() {
  rotateOpen.value = false;
  void agents.rotate();
}
function confirmCancel() {
  const id = cancelID.value;
  cancelID.value = "";
  if (id) void agents.cancel(id);
}
</script>

<template>
  <div class="stack">
    <AppCard eyebrow="MCP · Streamable HTTP" title="服务">
      <template #actions>
        <AppButton size="sm" variant="ghost" @click="agents.load()"
          ><RefreshCw :size="14" aria-hidden="true" />刷新</AppButton
        >
      </template>
      <div class="stack">
        <p class="agents__status" role="status">
          <span class="lamp" :class="`lamp--${statusTone}`" aria-hidden="true"></span>
          <strong>{{ statusText }}</strong>
        </p>
        <AppAlert
          v-if="listener?.last_error && listener.last_error !== agents.applyError"
          type="error"
          title="上次的设置没有生效"
          :message="listener.last_error"
        />
        <AppAlert
          v-if="agents.loadError"
          type="error"
          title="无法读取 Agent 接入状态"
          :message="agents.loadError"
        />
        <AppAlert
          v-if="agents.applyError"
          type="error"
          title="没有应用更改"
          :message="agents.applyError"
        />
        <AppToggle id="agent-enabled" v-model="agents.draft.enabled">允许 Agent 连接</AppToggle>
        <AppAlert
          v-if="continuesNotice"
          type="info"
          title="正在运行的 Agent 操作会继续"
          message="关闭服务只断开 Agent 的连接，不会停止已经开始的操作。需要停止时，在下面的「Agent 操作」中点「取消」。"
        />
        <div class="agents__fields">
          <FormField
            id="agent-host"
            label="监听地址"
            hint="仅本机最安全；其他地址让局域网内的 Agent 也能连接。"
          >
            <div class="rt-field">
              <select
                id="agent-host"
                v-model="agents.draft.listen_host"
                class="rt-field__control mono"
                aria-describedby="agent-host-hint"
              >
                <option v-for="option in addressOptions" :key="option.value" :value="option.value">
                  {{ option.label }}
                </option>
              </select>
            </div>
          </FormField>
          <TextInput
            id="agent-port"
            v-model="agents.draft.port"
            type="number"
            label="端口"
            hint="1024–65535"
            :min="1024"
            :max="65535"
            :error="agents.errors['agent-port']"
            mono
          />
        </div>
        <AppAlert
          v-if="lanWarning"
          type="warning"
          title="明文：令牌和密码会经过网络"
          message="监听本机以外的地址时，访问令牌和工具参数（包括密码）以明文经过局域网。只在可信的网络中使用，用完关掉。"
        />
        <AppToggle id="agent-writes" v-model="agents.draft.allow_writes" note="高风险"
          >允许 Agent 写入路由器</AppToggle
        >
        <AppAlert
          v-if="agents.draft.allow_writes"
          type="warning"
          title="Agent 可以修改路由器"
          message="开启后，Agent 可以执行引导计划（开启 SSH、软固化、重启验证）和移除软固化。深度固化、恢复确认和忘记密码始终只能由你在这个窗口操作。"
        />
        <div class="cluster">
          <AppButton
            variant="primary"
            :loading="agents.applying"
            :disabled="!agents.dirty"
            @click="agents.apply()"
            >应用更改</AppButton
          >
          <AppButton
            variant="ghost"
            :disabled="!agents.dirty || agents.applying"
            @click="agents.reset()"
            >撤销更改</AppButton
          >
        </div>
        <CodeBlock label="Agent 连接地址" :content="agents.status?.url ?? ''" />
      </div>
    </AppCard>

    <AppCard eyebrow="只保存哈希" title="访问令牌">
      <template #actions>
        <AppButton size="sm" :loading="agents.rotating" @click="rotate"
          ><KeyRound :size="14" aria-hidden="true" />生成新令牌</AppButton
        >
      </template>
      <div class="stack">
        <template v-if="agents.token">
          <AppAlert
            type="warning"
            title="只显示这一次"
            message="复制后把它设为启动 Agent 的环境变量 ROUTERCTL_MCP_TOKEN。离开这个页面或点「我已保存」后，无法再次查看。"
          />
          <CodeBlock label="访问令牌" :content="agents.token" />
          <div class="cluster">
            <AppButton variant="primary" @click="agents.dismissToken()">我已保存</AppButton>
          </div>
        </template>
        <p v-else class="agents__note">
          {{
            listener?.token_set
              ? `已设置 · 创建于 ${formatTime(listener.token_created_at ?? "")}。令牌只显示一次，忘记了就生成新的。`
              : "尚未生成。第一次开启服务时会自动生成，并在这里显示一次。"
          }}
        </p>
        <p class="agents__note">
          在启动 Agent 的终端中设置：PowerShell 用
          <span class="mono">$env:ROUTERCTL_MCP_TOKEN = '令牌'</span>，bash / zsh 用
          <span class="mono">export ROUTERCTL_MCP_TOKEN=令牌</span>。
        </p>
      </div>
    </AppCard>

    <AppCard eyebrow="配置与 Skill" title="连接 Agent">
      <div class="stack">
        <AppSegmented
          :model-value="agents.host"
          :options="hostOptions"
          label="Agent"
          @update:model-value="agents.selectHost"
        />
        <template v-if="stdioOnly">
          <AppAlert
            type="info"
            title="WorkBuddy 只支持 stdio"
            message="WorkBuddy 的文档只给出了 stdio 方式，连不上这个窗口的服务。在这台电脑上安装 routerctl 后，运行下面的命令，按输出在「专家·技能·连接器 → 连接器 → 自定义连接器」中添加。"
          />
          <CodeBlock label="命令" content="routerctl mcp config --agent workbuddy" />
        </template>
        <AppAlert
          v-else-if="agents.setupError"
          type="error"
          title="无法生成配置"
          :message="agents.setupError"
        />
        <template v-else-if="agents.setup">
          <p v-if="agents.setup.file" class="agents__note">
            写入文件 <span class="mono">{{ agents.setup.file }}</span>
          </p>
          <CodeBlock label="配置片段" :content="agents.setup.snippet" />
          <CodeBlock
            v-if="agents.setup.command"
            label="或运行命令"
            :content="agents.setup.command"
          />
          <ul v-if="agents.setup.notes?.length" class="agents__notes">
            <li v-for="note in agents.setup.notes" :key="note">{{ note }}</li>
          </ul>
        </template>
        <section class="agents__skill" aria-labelledby="agent-skill-title">
          <h3 id="agent-skill-title" class="agents__subtitle">Skill</h3>
          <template v-if="hasSkillDir">
            <p class="agents__note">
              <template v-if="skillTarget"
                >安装到 <span class="mono">{{ skillTarget }}</span
                >。</template
              ><template v-else>安装到 {{ hostName }} 的 Skill 目录。</template>它告诉 Agent
              先检测、取得你的同意、只执行一次计划，遇到部分完成时停止。
            </p>
            <div class="cluster">
              <AppButton :loading="agents.skillBusy" @click="agents.installSkill()"
                ><PackagePlus :size="14" aria-hidden="true" />安装 Skill</AppButton
              >
            </div>
          </template>
          <template v-else>
            <p class="agents__note">
              {{ hostName }} 没有文档化的 Skill 目录。导出到一个文件夹后，在 {{ hostName }} 中导入
              <span class="mono">router-tools</span> 文件夹。
            </p>
            <div class="cluster">
              <AppButton :loading="agents.skillBusy" @click="agents.exportSkill()"
                ><FolderOutput :size="14" aria-hidden="true" />导出到文件夹…</AppButton
              >
            </div>
          </template>
          <AppAlert
            v-if="skillAlert"
            :type="skillAlert.type"
            :title="skillAlert.title"
            :message="skillAlert.message"
          />
          <AppAlert
            v-if="agents.skillError"
            type="error"
            title="没有安装 Skill"
            :message="agents.skillError"
          />
        </section>
      </div>
    </AppCard>

    <AppCard :title="`Agent 操作 · ${operationRows.length}`">
      <EmptyState
        v-if="!operationRows.length"
        :icon="Bot"
        title="还没有 Agent 发起的操作"
        message="Agent 调用检测、备份或引导执行时，操作会出现在这里；运行中的操作可以随时取消。"
      />
      <HistoryList v-else :rows="operationRows">
        <template #actions="{ row }">
          <AppButton v-if="runningIDs.has(row.id)" size="sm" @click="cancelID = row.id"
            >取消</AppButton
          >
        </template>
      </HistoryList>
    </AppCard>

    <AppCard title="最近的调用">
      <div class="stack">
        <EmptyState
          v-if="!activityRows.length"
          title="还没有 Agent 调用"
          message="Agent 连接后，每次工具调用和每个操作的结束都会记录在这里：只有工具、状态和时间，不含参数和密码。"
        />
        <HistoryList v-else :rows="activityRows" />
        <p class="agents__note">只保留最近 200 条，这里显示最新的 20 条；退出应用后清空。</p>
      </div>
    </AppCard>

    <ConfirmDialog
      v-model="rotateOpen"
      title="生成新的访问令牌"
      description="旧令牌立即失效。已经连接的 Agent 需要换成新令牌后才能继续调用。"
      confirm-label="生成新令牌"
      @confirm="confirmRotate"
    />
    <ConfirmDialog
      v-model="forceOpen"
      title="替换已安装的 Skill"
      :description="`${agents.pendingForce ? agents.skillReport?.path : ''} 与这个版本的 Skill 不同。替换会重写 Skill 自带的文件，保留你放在该目录中的其他文件。`"
      confirm-label="替换"
      @confirm="agents.confirmForce()"
      @cancel="agents.cancelForce()"
    />
    <ConfirmDialog
      v-model="cancelOpen"
      title="取消 Agent 操作"
      description="取消后，正在进行的写入可能只完成一部分。结果为部分完成或结果未知时，路由器会被阻断，直到你检查并确认。"
      confirm-label="取消操作"
      @confirm="confirmCancel"
    />
  </div>
</template>

<style scoped>
.agents__status {
  display: flex;
  align-items: center;
  gap: var(--rt-space-2);
  margin: 0;
  overflow-wrap: anywhere;
}
.agents__fields {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(min(100%, var(--rt-field-min)), 1fr));
  gap: var(--rt-space-3);
}
.agents__note {
  margin: 0;
  color: var(--rt-fg-muted);
  font-size: var(--rt-text-sm);
  overflow-wrap: anywhere;
}
.agents__notes {
  display: grid;
  gap: var(--rt-space-1);
  margin: 0;
  padding-left: var(--rt-space-5);
  color: var(--rt-fg-muted);
  font-size: var(--rt-text-sm);
}
.agents__skill {
  display: grid;
  gap: var(--rt-space-3);
  padding-top: var(--rt-space-4);
  border-top: var(--rt-border-width) solid var(--rt-border);
}
.agents__subtitle {
  margin: 0;
  font-size: var(--rt-text-md);
  font-weight: var(--rt-weight-strong);
}
</style>
