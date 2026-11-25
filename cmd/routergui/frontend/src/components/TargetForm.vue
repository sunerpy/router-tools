<script setup lang="ts">
import { inject } from "vue";
import TextInput from "./TextInput.vue";

const store = inject("operations") as any;
defineProps<{ errors?: Record<string, string> }>();
</script>

<template>
  <div class="target-form-container">
    <div class="target-form">
      <TextInput
        id="target-host"
        :error="errors?.['target-host']"
        v-model="store.target.host"
        class="target-form__host"
        label="路由器地址"
        mono
        required
      />
      <TextInput
        id="target-web-port"
        :error="errors?.['target-web-port']"
        v-model="store.target.web_port"
        label="Web 端口"
        type="number"
        :min="1"
        :max="65535"
        mono
        required
      />
      <TextInput
        id="target-ssh-port"
        :error="errors?.['target-ssh-port']"
        v-model="store.target.ssh_port"
        label="SSH 端口"
        type="number"
        :min="1"
        :max="65535"
        mono
        required
      />
      <TextInput
        id="target-telnet-port"
        :error="errors?.['target-telnet-port']"
        v-model="store.target.telnet_port"
        label="Telnet 端口"
        type="number"
        :min="1"
        :max="65535"
        mono
        required
      />
      <TextInput
        id="target-profile"
        :error="errors?.['target-profile']"
        v-model="store.profileID"
        class="target-form__profile"
        label="设备 Profile"
        hint="auto 只在唯一精确匹配时继续，其他固件会被拒绝。"
        mono
        required
      />
    </div>
  </div>
</template>

<style scoped>
/* Container-local layout: the same form sits in a narrow split column (dashboard) and a full-width
   card (settings), so it reacts to its own width rather than the viewport (DESIGN.md §2). */
.target-form-container {
  container-type: inline-size;
}
.target-form {
  display: grid;
  grid-template-columns:
    minmax(0, 1.4fr) repeat(3, minmax(0, 0.8fr))
    minmax(0, 1fr);
  align-items: start;
  gap: var(--rt-space-3);
}
@container (max-width: 720px) {
  .target-form {
    grid-template-columns: repeat(3, minmax(0, 1fr));
  }
  .target-form__host,
  .target-form__profile {
    grid-column: span 3;
  }
}
@container (max-width: 360px) {
  .target-form {
    grid-template-columns: minmax(0, 1fr);
  }
  .target-form__host,
  .target-form__profile {
    grid-column: auto;
  }
}
</style>
