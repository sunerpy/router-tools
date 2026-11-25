<script setup lang="ts">
import { computed, inject } from "vue";
import ConfirmDialog from "./ConfirmDialog.vue";

const store = inject("operations") as any;
const visible = computed({
  get: () => Boolean(store.hostKeyPrompt),
  set: () => store.resolveHostKey(false),
});
const description = computed(() => {
  const prompt = store.hostKeyPrompt;
  return prompt
    ? `主机：${prompt.address}\n算法：${prompt.algorithm}\n指纹：${prompt.fingerprint}\n请核对完整指纹。仅在确认其属于该路由器时接受；接受后会写入 known_hosts，之后自动信任这个密钥。`
    : "";
});
</script>

<template>
  <ConfirmDialog
    v-model="visible"
    title="未知 SSH 主机密钥"
    :description="description"
    mono-description
    confirm-label="接受并保存此密钥"
    @confirm="store.resolveHostKey(true)"
    @cancel="store.resolveHostKey(false)"
  />
</template>
