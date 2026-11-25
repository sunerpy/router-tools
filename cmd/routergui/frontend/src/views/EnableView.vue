<script setup lang="ts">
import { inject, onBeforeUnmount, ref } from "vue";
import { ShieldCheck } from "lucide-vue-next";
import AppAlert from "../components/AppAlert.vue";
import AppButton from "../components/AppButton.vue";
import AppCard from "../components/AppCard.vue";
import ConfirmDialog from "../components/ConfirmDialog.vue";
import OperationPanel from "../components/OperationPanel.vue";
import SecretInput from "../components/SecretInput.vue";
import { required, useFieldErrors } from "../composables/useFieldErrors";

const store = inject("operations") as any;
const dialogOpen = ref(false);
const { errors, check } = useFieldErrors();
onBeforeUnmount(() => store.clearSecrets());
function ask() {
  const ok = check([
    required("enable-admin", store.adminPassword, "请输入 Web 管理密码"),
    required("enable-root", store.rootPassword, "请输入要设置的 root 密码"),
  ]);
  if (ok) dialogOpen.value = true;
}
function enable() {
  store.confirmed = true;
  dialogOpen.value = false;
  void store.run("EnableSSH");
}
</script>

<template>
  <div class="stack">
    <AppAlert
      type="warning"
      title="开启并非完全可逆"
      message="NVRAM 开关和 root 密码不会自动恢复；端口开放不代表成功，最终必须验证 SSH root。"
    />
    <AppCard eyebrow="需要 Web 管理密码" title="临时开启 SSH">
      <div class="stack">
        <div class="enable__fields">
          <SecretInput
            id="enable-admin"
            v-model="store.adminPassword"
            label="Web 管理密码"
            :error="errors['enable-admin']"
            required
          />
          <SecretInput
            id="enable-root"
            v-model="store.rootPassword"
            label="新的 root 密码"
            hint="使用独立、高强度的路由器 root 密码。"
            :error="errors['enable-root']"
            required
          />
        </div>
        <div class="cluster">
          <AppButton
            variant="primary"
            :disabled="store.busy"
            :loading="store.busy && store.operation === 'EnableSSH'"
            @click="ask"
          >
            <ShieldCheck :size="16" aria-hidden="true" />检查并开启 SSH
          </AppButton>
          <span class="enable__hint">先检查 root 是否已可登录；已可登录时不会写入 MAC 规则</span>
        </div>
      </div>
    </AppCard>
    <OperationPanel />
    <ConfirmDialog
      v-model="dialogOpen"
      title="确认开启 SSH"
      description="将提交 NVRAM 开关、修改 Dropbear、设置 root 密码，并添加后清理三条 MAC 规则。"
      confirm-label="确认开启"
      @confirm="enable"
      @cancel="store.clearSecrets()"
    />
  </div>
</template>

<style scoped>
.enable__fields {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(min(100%, var(--rt-field-min)), 1fr));
  gap: var(--rt-space-3);
}
.enable__hint {
  color: var(--rt-fg-muted);
  font-size: var(--rt-text-xs);
}
</style>
