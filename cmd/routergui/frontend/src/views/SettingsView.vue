<script setup lang="ts">
import { computed, inject } from "vue";
import AppCard from "../components/AppCard.vue";
import AppSegmented from "../components/AppSegmented.vue";
import AppToggle from "../components/AppToggle.vue";
import TargetForm from "../components/TargetForm.vue";
import TextInput from "../components/TextInput.vue";
import type { ThemeController, ThemePreference } from "../composables/useTheme";

const store = inject("operations") as any;
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
    <AppCard eyebrow="保存在本机" title="连接与 Profile">
      <TargetForm />
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
</style>
