<script setup lang="ts">
import { computed } from "vue";
import FormField from "./FormField.vue";
import type { CatalogEntry } from "../services/api";

const props = defineProps<{
  catalog: CatalogEntry[];
  modelValue: string;
  id: string;
  disabled?: boolean;
  /** A validation message for the model choice, linked to its select. */
  error?: string;
}>();
const emit = defineEmits<{ "update:modelValue": [value: string] }>();

const AUTO = "auto";
// The model is derived from the profile ID; a stored ID that is not registered keeps its value and is
// flagged, never rewritten (DESIGN.md §3).
const current = computed(() => {
  for (const entry of props.catalog) {
    const profile = entry.profiles.find((candidate) => candidate.id === props.modelValue);
    if (profile) return { model: entry.model_id, profile: profile.id };
  }
  return null;
});
const unregistered = computed(
  () => props.modelValue !== AUTO && props.modelValue !== "" && current.value === null,
);
const modelValueSelected = computed(() => current.value?.model ?? (unregistered.value ? "" : AUTO));
const profiles = computed(
  () => props.catalog.find((entry) => entry.model_id === current.value?.model)?.profiles ?? [],
);

function chooseModel(model: string) {
  if (model === AUTO) {
    emit("update:modelValue", AUTO);
    return;
  }
  const entry = props.catalog.find((candidate) => candidate.model_id === model);
  if (entry?.profiles[0]) emit("update:modelValue", entry.profiles[0].id);
}
</script>

<template>
  <div class="device-selector">
    <FormField
      :id="`${id}-model`"
      label="设备型号"
      hint="自动识别只在唯一精确匹配时继续。"
      :error="error"
    >
      <div class="rt-field">
        <select
          :id="`${id}-model`"
          class="rt-field__control"
          :value="modelValueSelected"
          :disabled="disabled"
          :aria-invalid="error ? 'true' : undefined"
          :aria-describedby="error ? `${id}-model-hint ${id}-model-error` : `${id}-model-hint`"
          @change="chooseModel(($event.target as HTMLSelectElement).value)"
        >
          <option v-if="unregistered" value="" disabled>请重新选择</option>
          <option :value="AUTO">自动识别</option>
          <option v-for="entry in catalog" :key="entry.model_id" :value="entry.model_id">
            {{ entry.vendor }} {{ entry.model }}（{{ entry.hardware }}）
          </option>
        </select>
      </div>
    </FormField>
    <FormField :id="`${id}-firmware`" label="固件版本" hint="只列出已注册、可精确匹配的固件。">
      <div class="rt-field">
        <select
          :id="`${id}-firmware`"
          class="rt-field__control mono"
          :value="current?.profile ?? ''"
          :disabled="disabled || !current"
          :aria-describedby="`${id}-firmware-hint`"
          @change="emit('update:modelValue', ($event.target as HTMLSelectElement).value)"
        >
          <option v-if="!current" value="">按设备识别</option>
          <option v-for="profile in profiles" :key="profile.id" :value="profile.id">
            {{ profile.firmware }}
          </option>
        </select>
      </div>
    </FormField>
    <p v-if="unregistered" class="device-selector__warning" role="status">
      已保存的 profile <span class="mono">{{ modelValue }}</span> 未注册，请重新选择型号与固件。
    </p>
  </div>
</template>

<style scoped>
.device-selector {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: var(--rt-space-3);
}
.device-selector__warning {
  grid-column: 1 / -1;
  margin: 0;
  color: var(--rt-warning-text);
  overflow-wrap: anywhere;
}
@media (max-width: 480px) {
  .device-selector {
    grid-template-columns: minmax(0, 1fr);
  }
}
</style>
