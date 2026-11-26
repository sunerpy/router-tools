<script setup lang="ts">
import { computed } from "vue";
import { KeyRound, ShieldAlert, ShieldOff } from "lucide-vue-next";
import AppBadge from "./AppBadge.vue";
import { CREDENTIAL_KIND } from "../services/guideText";
import type { SavedState } from "../composables/useGuide";

const props = defineProps<{ kind: "web-admin" | "root"; state: SavedState }>();
const presentation = computed(() => {
  const name = CREDENTIAL_KIND[props.kind];
  switch (props.state) {
    case "saved":
      return { text: `${name}已保存`, tone: "ok" as const, icon: KeyRound };
    case "rejected":
      return { text: `${name}已被拒绝`, tone: "warn" as const, icon: ShieldAlert };
    default:
      return { text: `${name}未保存`, tone: "neutral" as const, icon: ShieldOff };
  }
});
</script>

<template>
  <AppBadge :tone="presentation.tone" :icon="presentation.icon">{{ presentation.text }}</AppBadge>
</template>
