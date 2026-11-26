<script setup lang="ts">
import { computed, inject } from "vue";
import ConfirmDialog from "./ConfirmDialog.vue";

const store = inject("operations") as any;
const visible = computed({
  get: () => Boolean(store.hostKeyPrompt),
  set: () => store.resolveHostKey(false),
});
const fromAgent = computed(() => store.hostKeyPrompt?.source === "agent");
const description = computed(() => {
  const prompt = store.hostKeyPrompt;
  if (!prompt) return "";
  const origin = fromAgent.value ? `来源：Agent 发起的操作 ${prompt.operation_id}\n` : "";
  const waiting = store.hostKeyWaiting
    ? `\n另有 ${store.hostKeyWaiting} 个主机密钥等待确认，处理完这个后逐个显示。`
    : "";
  return `${origin}主机：${prompt.address}\n算法：${prompt.algorithm}\n指纹：${prompt.fingerprint}\n请核对完整指纹。仅在确认其属于该路由器时接受；接受后会写入 known_hosts，之后自动信任这个密钥。${waiting}`;
});
</script>

<template>
  <ConfirmDialog
    v-model="visible"
    :title="fromAgent ? 'Agent 操作遇到未知的 SSH 主机密钥' : '未知 SSH 主机密钥'"
    :description="description"
    mono-description
    confirm-label="接受并保存此密钥"
    @confirm="store.resolveHostKey(true)"
    @cancel="store.resolveHostKey(false)"
  />
</template>
