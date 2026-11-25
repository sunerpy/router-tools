<script setup lang="ts">
import { inject, onBeforeUnmount, ref } from "vue";
import { Activity, RotateCcw, Trash2 } from "lucide-vue-next";
import AppButton from "../components/AppButton.vue";
import AppCard from "../components/AppCard.vue";
import AppToggle from "../components/AppToggle.vue";
import ConfirmDialog from "../components/ConfirmDialog.vue";
import OperationPanel from "../components/OperationPanel.vue";
import SecretInput from "../components/SecretInput.vue";
import { required, useFieldErrors } from "../composables/useFieldErrors";

const store = inject("operations") as any;
const dialogOpen = ref(false);
const action = ref("PersistSoft");
const { errors, check } = useFieldErrors();
onBeforeUnmount(() => store.clearSecrets());
function ask(name: string) {
  if (!check([required("persist-root", store.rootPassword, "请输入 root 密码")])) return;
  action.value = name;
  dialogOpen.value = true;
}
function run() {
  store.confirmed = true;
  dialogOpen.value = false;
  void store.run(action.value);
}
</script>

<template>
  <div class="stack">
    <AppCard eyebrow="需要 root 密码" title="软固化与重启验证">
      <div class="stack">
        <div class="persistence__field">
          <SecretInput
            id="persist-root"
            v-model="store.rootPassword"
            label="Root 密码"
            :error="errors['persist-root']"
            required
          />
        </div>
        <AppToggle v-model="store.allowTelnet" note="明文"
          >允许中间阶段 Telnet 回退（最终仍验证 SSH）</AppToggle
        >
        <div class="cluster">
          <AppButton variant="primary" :disabled="store.busy" @click="ask('PersistSoft')">
            <Activity :size="16" aria-hidden="true" />安装软固化
          </AppButton>
          <AppButton :disabled="store.busy" @click="ask('RebootVerify')">
            <RotateCcw :size="16" aria-hidden="true" />重启并验证
          </AppButton>
          <AppButton variant="ghost" :disabled="store.busy" @click="ask('PersistRemove')">
            <Trash2 :size="16" aria-hidden="true" />移除软固化
          </AppButton>
        </div>
        <p class="persistence__note">
          安装校验只证明文件和 UCI 写对了；只有「重启并验证」能证明重启后仍然生效。
        </p>
      </div>
    </AppCard>
    <OperationPanel />
    <ConfirmDialog
      v-model="dialogOpen"
      title="确认设备状态变更"
      description="安装或移除只作用于工具管理的 auto_ssh 配置；重启会中断当前连接。安装校验不等于重启验证。"
      @confirm="run"
      @cancel="store.clearSecrets()"
    />
  </div>
</template>

<style scoped>
.persistence__field {
  max-width: var(--rt-dialog-width);
}
.persistence__note {
  margin: 0;
  color: var(--rt-fg-muted);
  font-size: var(--rt-text-xs);
}
</style>
