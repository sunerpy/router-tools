<script setup lang="ts">
import { computed, inject, onBeforeUnmount, ref } from "vue";
import { FileCheck, FolderOpen, HeartPulse, ShieldCheck, Trash2, Wifi } from "lucide-vue-next";
import AppButton from "../components/AppButton.vue";
import AppCard from "../components/AppCard.vue";
import AppToggle from "../components/AppToggle.vue";
import ConfirmDialog from "../components/ConfirmDialog.vue";
import OperationPanel from "../components/OperationPanel.vue";
import SecretInput from "../components/SecretInput.vue";
import TextInput from "../components/TextInput.vue";
import {
  required,
  targetRules,
  useFieldErrors,
  type FieldRule,
} from "../composables/useFieldErrors";
import type { OperationName, RouterAPI } from "../services/api";

const store = inject("operations") as any;
const api = inject("api") as RouterAPI;
const { errors, check } = useFieldErrors();
const useSaved = ref(true);
const removeOpen = ref(false);
onBeforeUnmount(() => store.clearSecrets());

// A root password may stay empty when saved passwords are allowed; the application then fills it or
// reports that none is saved, before any device access that needs it.
const rootRule = (): FieldRule =>
  useSaved.value
    ? ["tools-root", true, ""]
    : required("tools-root", store.rootPassword, "请输入 root 密码");

function start(name: OperationName, extra: FieldRule[] = []) {
  if (!check([...targetRules(store.target), ...extra])) return;
  void store.run(name, { UseSavedPassword: useSaved.value, AllowTelnet: false });
}

function askRemove() {
  if (check([...targetRules(store.target), rootRule()])) removeOpen.value = true;
}
function remove() {
  removeOpen.value = false;
  store.confirmed = true;
  void store.run("PersistRemove", { UseSavedPassword: useSaved.value, AllowTelnet: false });
}

async function pickBackup() {
  try {
    const directory = await api.SelectDirectory();
    if (directory) store.backupDir = directory;
  } catch {
    // The dialog is optional; the path can be typed.
  }
}
const backupRule = (): FieldRule =>
  required("tools-backup", store.backupDir, "请选择或输入备份目录");
const doctorOptions = computed(() => ({
  WithAdminAuth: store.withAdminAuth,
  WithRootVerify: store.withRootVerify,
}));
</script>

<template>
  <div class="stack">
    <AppCard eyebrow="凭据" title="本页使用的密码">
      <div class="stack">
        <SecretInput
          id="tools-root"
          v-model="store.rootPassword"
          label="root 密码"
          :hint="useSaved ? '留空时使用已保存的 root 密码；没有保存时会提示。' : undefined"
          :error="errors['tools-root']"
        />
        <AppToggle v-model="useSaved" note="不会显示密码">使用已保存的密码</AppToggle>
      </div>
    </AppCard>

    <div class="split split--even">
      <AppCard eyebrow="会修改路由器" title="移除软固化">
        <div class="stack">
          <p class="tools__note">
            只删除工具管理的开机脚本和 firewall.auto_ssh 配置，不恢复旧 root 密码，也不关闭 NVRAM
            中的 SSH 开关。
          </p>
          <div class="cluster">
            <AppButton variant="danger" :disabled="store.busy" @click="askRemove">
              <Trash2 :size="16" aria-hidden="true" />移除软固化
            </AppButton>
          </div>
        </div>
      </AppCard>

      <AppCard eyebrow="只读" title="验证 root 登录">
        <div class="stack">
          <p class="tools__note">
            用密码认证的 SSH 登录并检查 id -u 为 0；Telnet 结果不算 SSH 成功。
          </p>
          <div class="cluster">
            <AppButton :disabled="store.busy" @click="start('Verify', [rootRule()])">
              <ShieldCheck :size="16" aria-hidden="true" />验证 root 登录
            </AppButton>
          </div>
        </div>
      </AppCard>

      <AppCard eyebrow="只读" title="校验备份">
        <div class="stack">
          <div class="tools__path">
            <TextInput
              id="tools-backup"
              v-model="store.backupDir"
              label="备份目录"
              :error="errors['tools-backup']"
              mono
            />
            <AppButton :disabled="store.busy" @click="pickBackup">
              <FolderOpen :size="16" aria-hidden="true" />选择目录
            </AppButton>
          </div>
          <div class="cluster">
            <AppButton :disabled="store.busy" @click="start('BackupVerify', [backupRule()])">
              <FileCheck :size="16" aria-hidden="true" />本地校验
            </AppButton>
            <AppButton
              :disabled="store.busy"
              @click="start('BackupVerifyOnline', [backupRule(), rootRule()])"
            >
              <Wifi :size="16" aria-hidden="true" />在线比对
            </AppButton>
          </div>
        </div>
      </AppCard>

      <AppCard eyebrow="只读" title="原始预检">
        <div class="stack">
          <AppToggle v-model="store.withAdminAuth">同时验证 Web 管理密码</AppToggle>
          <SecretInput
            v-if="store.withAdminAuth"
            id="tools-admin"
            v-model="store.adminPassword"
            label="Web 管理密码"
            :hint="useSaved ? '留空时使用已保存的 Web 管理密码。' : undefined"
          />
          <AppToggle v-model="store.withRootVerify">同时验证 root 登录</AppToggle>
          <div class="cluster">
            <AppButton
              :disabled="store.busy"
              @click="start('Doctor', store.withRootVerify ? [rootRule()] : [])"
            >
              <HeartPulse :size="16" aria-hidden="true" />运行预检
            </AppButton>
            <span class="tools__note">{{
              doctorOptions.WithAdminAuth || doctorOptions.WithRootVerify
                ? "会登录但不做修改"
                : "不需要密码"
            }}</span>
          </div>
        </div>
      </AppCard>
    </div>

    <OperationPanel />

    <ConfirmDialog
      v-model="removeOpen"
      title="确认移除软固化"
      description="将删除工具管理的开机脚本并提交防火墙配置。重启后 SSH 不再自动恢复。"
      confirm-label="移除"
      @confirm="remove"
      @cancel="store.clearSecrets()"
    />
  </div>
</template>

<style scoped>
.tools__note {
  margin: 0;
  color: var(--rt-fg-muted);
  overflow-wrap: anywhere;
}
.tools__path {
  display: grid;
  grid-template-columns: minmax(0, 1fr) auto;
  align-items: end;
  gap: var(--rt-space-2);
}
@media (max-width: 480px) {
  .tools__path {
    grid-template-columns: minmax(0, 1fr);
  }
}
</style>
