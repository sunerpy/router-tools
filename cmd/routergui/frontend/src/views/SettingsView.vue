<script setup lang="ts">
import { computed, inject, onMounted, ref, type Ref } from "vue";
import { KeyRound, Trash2 } from "lucide-vue-next";
import AppAlert from "../components/AppAlert.vue";
import AppButton from "../components/AppButton.vue";
import AppCard from "../components/AppCard.vue";
import AppIconButton from "../components/AppIconButton.vue";
import ConfirmDialog from "../components/ConfirmDialog.vue";
import CredentialState from "../components/CredentialState.vue";
import DeviceSelector from "../components/DeviceSelector.vue";
import EmptyState from "../components/EmptyState.vue";
import AppSegmented from "../components/AppSegmented.vue";
import AppToggle from "../components/AppToggle.vue";
import TargetForm from "../components/TargetForm.vue";
import TextInput from "../components/TextInput.vue";
import type { ThemeController, ThemePreference } from "../composables/useTheme";
import { CREDENTIAL_KIND } from "../services/guideText";
import { formatTime } from "../services/summary";
import type { CatalogEntry, CredentialEntry, RouterAPI } from "../services/api";

const store = inject("operations") as any;
const api = inject("api") as RouterAPI;
const catalog = inject("catalog") as Ref<CatalogEntry[]>;
const saved = ref<CredentialEntry[]>([]);
const vaultError = ref("");
// A failed forget is shown on its own: reloading the list must not hide it.
const forgetError = ref("");
const forgetAllOpen = ref(false);

async function loadSaved() {
  try {
    const report = await api.CredentialStatus();
    saved.value = report.entries ?? [];
    vaultError.value = report.error ? report.error.message || report.error.code : "";
  } catch (caught) {
    vaultError.value = caught instanceof Error ? caught.message : String(caught);
  }
}
onMounted(loadSaved);

async function runForget(call: () => Promise<{ error?: { code: string; message: string } }>) {
  forgetError.value = "";
  let failure = "";
  try {
    const report = await call();
    if (report.error) failure = `${report.error.code}：${report.error.message}`;
  } catch (caught) {
    failure = caught instanceof Error ? caught.message : String(caught);
  }
  await loadSaved();
  forgetError.value = failure;
}

function forget(entry: CredentialEntry) {
  return runForget(() =>
    api.ForgetCredential({
      Kind: entry.kind,
      Scope: entry.scope,
      Host: entry.host,
      Port: entry.port,
    }),
  );
}

function forgetAll() {
  forgetAllOpen.value = false;
  return runForget(() => api.ForgetAllCredentials());
}
const theme = inject("theme") as ThemeController;
const appearance = computed({
  get: () => theme.preference.value,
  set: (value: string) => theme.setPreference(value as ThemePreference),
});
const themeOptions = [
  { value: "light", label: "浅色 · 白瓷" },
  { value: "dark", label: "深色 · 石墨" },
  { value: "system", label: "跟随系统" },
];
</script>

<template>
  <div class="stack">
    <AppCard eyebrow="保存在本机" title="连接与设备">
      <div class="stack">
        <DeviceSelector id="settings-device" v-model="store.profileID" :catalog="catalog" />
        <TargetForm />
      </div>
    </AppCard>
    <AppCard eyebrow="加密保存在本机" title="已保存的密码">
      <template v-if="saved.length" #actions>
        <AppButton variant="danger" size="sm" @click="forgetAllOpen = true">
          <Trash2 :size="14" aria-hidden="true" />全部忘记
        </AppButton>
      </template>
      <div class="stack">
        <AppAlert v-if="vaultError" type="error" title="无法读取保存的密码" :message="vaultError" />
        <AppAlert v-if="forgetError" type="error" title="没有忘记成功" :message="forgetError" />
        <EmptyState
          v-if="!saved.length && !vaultError"
          title="还没有保存的密码"
          message="在向导中打开「记住密码」，路由器接受密码后会加密保存在这里。"
        />
        <ul v-else class="settings__saved">
          <li
            v-for="entry in saved"
            :key="`${entry.kind}-${entry.scope}-${entry.host}-${entry.port}`"
            class="settings__saved-row"
          >
            <KeyRound :size="18" aria-hidden="true" />
            <div class="settings__saved-body">
              <CredentialState
                :kind="entry.kind"
                :state="entry.rejected_at ? 'rejected' : 'saved'"
              />
              <span class="mono">{{ entry.host }}:{{ entry.port }} · {{ entry.scope }}</span>
              <span class="settings__note">保存于 {{ formatTime(entry.saved_at) }}</span>
            </div>
            <AppIconButton
              :label="`忘记 ${entry.host} 的${CREDENTIAL_KIND[entry.kind]}`"
              @click="forget(entry)"
            >
              <Trash2 :size="16" aria-hidden="true" />
            </AppIconButton>
          </li>
        </ul>
        <p class="settings__note">
          密码用 AES-256-GCM 加密保存在用户配置目录。以你的用户身份运行的任何程序都能解密；Windows
          上单独拷走的密钥文件在其他账户和电脑上无法使用。这不等同于主密码；卸载应用不会删除它们。
        </p>
      </div>
    </AppCard>
    <AppCard eyebrow="SSH" title="主机密钥">
      <div class="stack">
        <div class="settings__fields">
          <TextInput
            id="known-hosts"
            v-model="store.hostKey.KnownHostsPath"
            label="known_hosts 文件"
            hint="留空时使用系统用户配置目录。"
            mono
          />
          <TextInput
            id="host-key-pin"
            v-model="store.hostKey.PinSHA256"
            label="固定 SHA-256 指纹"
            hint="格式：SHA256:<不带结尾 padding 的 Base64>。密钥变化不会被接受。"
            mono
          />
        </div>
        <AppToggle v-model="store.hostKey.AcceptNew" note="不推荐长期使用"
          >自动接受此前未知的主机密钥</AppToggle
        >
      </div>
    </AppCard>
    <AppCard eyebrow="外观" title="界面主题">
      <div class="stack">
        <AppSegmented v-model="appearance" :options="themeOptions" label="界面主题" />
        <p class="settings__note">
          默认使用浅色（白瓷）。选择“跟随系统”时，系统切到深色会自动使用石墨主题。只有这个选择会保存在本机。
        </p>
      </div>
    </AppCard>
    <ConfirmDialog
      v-model="forgetAllOpen"
      title="忘记全部保存的密码"
      description="将删除所有保存的密码和保险库密钥。之后需要重新输入密码。"
      confirm-label="全部忘记"
      danger
      @confirm="forgetAll"
    />
  </div>
</template>

<style scoped>
.settings__fields {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(min(100%, var(--rt-field-min)), 1fr));
  gap: var(--rt-space-3);
}
.settings__note {
  margin: 0;
  color: var(--rt-fg-muted);
  font-size: var(--rt-text-xs);
}
.settings__saved {
  display: grid;
  gap: var(--rt-space-2);
  margin: 0;
  padding: 0;
  list-style: none;
}
.settings__saved-row {
  display: grid;
  grid-template-columns: auto minmax(0, 1fr) auto;
  align-items: center;
  gap: var(--rt-space-3);
  padding: var(--rt-space-2) var(--rt-space-3);
  border: var(--rt-border-width) solid var(--rt-border);
  border-radius: var(--rt-r-control);
}
.settings__saved-body {
  display: grid;
  justify-items: start;
  gap: var(--rt-space-1);
  min-width: 0;
  overflow-wrap: anywhere;
}
</style>
