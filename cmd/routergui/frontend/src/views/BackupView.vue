<script setup lang="ts">
import { inject, onBeforeUnmount } from "vue";
import { Database, FileCheck, Wifi } from "lucide-vue-next";
import AppAlert from "../components/AppAlert.vue";
import AppButton from "../components/AppButton.vue";
import AppCard from "../components/AppCard.vue";
import OperationPanel from "../components/OperationPanel.vue";
import SecretInput from "../components/SecretInput.vue";
import TextInput from "../components/TextInput.vue";
import { required, useFieldErrors } from "../composables/useFieldErrors";

const store = inject("operations") as any;
const { errors, check } = useFieldErrors();
onBeforeUnmount(() => store.clearSecrets());
const rootRule = () => required("backup-root", store.rootPassword, "请输入 root 密码");
function create() {
  if (
    check([required("backup-output", store.outputDir, "请输入一个尚不存在的新目录"), rootRule()])
  ) {
    void store.run("BackupCreate");
  }
}
function verify(online: boolean) {
  const rules = [required("backup-dir", store.backupDir, "请输入要校验的备份目录")];
  if (online) rules.push(rootRule());
  if (check(rules)) void store.run(online ? "BackupVerifyOnline" : "BackupVerify");
}
</script>

<template>
  <div class="stack">
    <AppAlert
      type="info"
      title="备份不是原子快照"
      message="逐分区运行时读取不代表同一时刻的一致快照，也不保证可救砖。备份期间停止其他写入。"
    />
    <div class="split split--even">
      <AppCard eyebrow="需要 root 密码" title="创建备份">
        <div class="stack">
          <TextInput
            id="backup-output"
            v-model="store.outputDir"
            label="新备份目录"
            hint="目录必须尚不存在，已有目录不会被覆盖。"
            :error="errors['backup-output']"
            mono
            required
          />
          <SecretInput
            id="backup-root"
            v-model="store.rootPassword"
            label="Root 密码"
            :error="errors['backup-root']"
            required
          />
          <div class="cluster">
            <AppButton
              variant="primary"
              :disabled="store.busy && store.operation !== 'BackupCreate'"
              :loading="store.busy && store.operation === 'BackupCreate'"
              @click="create"
            >
              <Database :size="16" aria-hidden="true" />创建全分区备份
            </AppButton>
          </div>
        </div>
      </AppCard>
      <AppCard eyebrow="只读" title="校验备份">
        <div class="stack">
          <TextInput
            id="backup-dir"
            v-model="store.backupDir"
            label="已有备份目录"
            :error="errors['backup-dir']"
            mono
            required
          />
          <div class="cluster">
            <AppButton :disabled="store.busy" @click="verify(false)">
              <FileCheck :size="16" aria-hidden="true" />本地完整性校验
            </AppButton>
            <AppButton :disabled="store.busy" @click="verify(true)">
              <Wifi :size="16" aria-hidden="true" />在线布局与 boot_id 校验
            </AppButton>
          </div>
          <p class="backup__note">
            在线校验通过只读 SSH 命令比对设备身份、完整布局和 boot_id，需要上方的 root 密码。
          </p>
        </div>
      </AppCard>
    </div>
    <OperationPanel />
  </div>
</template>

<style scoped>
.backup__note {
  margin: 0;
  color: var(--rt-fg-muted);
  font-size: var(--rt-text-xs);
}
</style>
