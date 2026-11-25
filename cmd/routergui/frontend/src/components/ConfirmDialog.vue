<script setup lang="ts">
import { computed, ref, useId, watch } from "vue";
import { NCard, NInput, NModal } from "naive-ui";
import { ShieldAlert } from "lucide-vue-next";
import AppButton from "./AppButton.vue";

const props = withDefaults(
  defineProps<{
    modelValue: boolean;
    title: string;
    description?: string;
    danger?: boolean;
    requireText?: string;
    confirmLabel?: string;
    monoDescription?: boolean;
  }>(),
  { description: "", confirmLabel: "确认", monoDescription: false },
);
const emit = defineEmits<{
  "update:modelValue": [value: boolean];
  confirm: [];
  cancel: [];
}>();
const acknowledgement = ref("");
const ackID = useId();
const descriptionID = useId();
const cancelButton = ref<InstanceType<typeof AppButton> | null>(null);
watch(
  () => props.modelValue,
  (visible) => {
    if (visible) acknowledgement.value = "";
  },
  { immediate: true },
);
function focusCancel() {
  cancelButton.value?.focus();
}
const canConfirm = computed(
  () => !props.requireText || acknowledgement.value === props.requireText,
);
function confirm() {
  if (canConfirm.value) emit("confirm");
}
function cancel() {
  emit("cancel");
  emit("update:modelValue", false);
}
</script>

<template>
  <NModal
    :show="modelValue"
    :mask-closable="false"
    :auto-focus="false"
    @after-enter="focusCancel"
    @update:show="(value) => !value && cancel()"
  >
    <NCard
      class="confirm-dialog"
      :class="{ 'confirm-dialog--danger': danger }"
      role="dialog"
      aria-modal="true"
      :aria-label="title"
      :aria-describedby="description ? descriptionID : undefined"
      :bordered="false"
    >
      <div class="confirm-dialog__layout">
        <h2 class="confirm-dialog__title">
          <ShieldAlert v-if="danger" :size="20" aria-hidden="true" />{{ title }}
        </h2>
        <p
          v-if="description"
          :id="descriptionID"
          class="confirm-dialog__description"
          :class="{ 'confirm-dialog__description--mono': monoDescription }"
        >
          {{ description }}
        </p>
        <div v-if="requireText" class="confirm-dialog__ack">
          <label :for="ackID">输入 {{ requireText }} 以继续</label>
          <NInput
            v-model:value="acknowledgement"
            :input-props="{ id: ackID, autocomplete: 'off', spellcheck: 'false' }"
            placeholder="RD08-DEEP"
          />
        </div>
        <div class="cluster confirm-dialog__actions">
          <AppButton ref="cancelButton" data-testid="confirm-cancel" @click="cancel"
            >取消</AppButton
          >
          <AppButton
            data-testid="confirm-danger"
            :variant="danger ? 'danger' : 'primary'"
            :disabled="!canConfirm"
            @click="confirm"
          >
            {{ confirmLabel }}
          </AppButton>
        </div>
      </div>
    </NCard>
  </NModal>
</template>

<style scoped>
.confirm-dialog {
  width: min(var(--rt-dialog-width), calc(100vw - var(--rt-space-6)));
  border: var(--rt-border-width) solid var(--rt-border);
  border-radius: var(--rt-r-panel);
  box-shadow: var(--rt-shadow-dialog);
}
.confirm-dialog__layout {
  display: grid;
  gap: var(--rt-space-4);
}
.confirm-dialog__title {
  display: flex;
  align-items: center;
  gap: var(--rt-space-2);
  margin: 0;
  color: var(--rt-fg);
  font-size: var(--rt-text-xl);
  font-weight: var(--rt-weight-strong);
}
.confirm-dialog--danger .confirm-dialog__title {
  color: var(--rt-danger-text);
}
.confirm-dialog__description {
  margin: 0;
  color: var(--rt-fg);
  overflow-wrap: anywhere;
}
.confirm-dialog__ack {
  display: grid;
  gap: var(--rt-space-2);
  font-size: var(--rt-text-sm);
  font-weight: var(--rt-weight-medium);
}
.confirm-dialog__description--mono {
  font-family: var(--rt-font-mono);
  white-space: pre-wrap;
}
.confirm-dialog__actions {
  justify-content: flex-end;
  gap: var(--rt-space-2);
}
</style>
