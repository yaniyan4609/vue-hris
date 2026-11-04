<script setup>
import { AlertCircle, HelpCircle } from 'lucide-vue-next'
import BaseTooltip from './BaseTooltip.vue'

defineProps({
  label: {
    type: String,
    default: '',
  },
  required: {
    type: Boolean,
    default: false,
  },
  error: {
    type: String,
    default: '',
  },
  hint: {
    type: String,
    default: '',
  },
  forId: {
    type: String,
    default: '',
  },
  helpText: {
    type: String,
    default: '',
  },
})
</script>

<template>
  <div class="w-full">
    <!-- Label Row -->
    <div v-if="label || $slots['label-extra']" class="flex items-center justify-between gap-2 mb-1">
      <label
        v-if="label"
        :for="forId || undefined"
        class="inline-flex items-center gap-1 text-xs font-semibold text-neutral-700 dark:text-neutral-300"
      >
        <span>{{ label }}</span>
        <span v-if="required" class="text-rose-500">*</span>
        <BaseTooltip v-if="helpText" :text="helpText">
          <HelpCircle class="w-3.5 h-3.5 text-neutral-400 hover:text-neutral-600 dark:hover:text-neutral-200 transition cursor-help" />
        </BaseTooltip>
      </label>

      <div v-if="$slots['label-extra']" class="text-[11px]">
        <slot name="label-extra" />
      </div>
    </div>

    <!-- Form Control Slot -->
    <slot />

    <!-- Error / Hint Footer -->
    <p
      v-if="error"
      class="mt-1 flex items-center gap-1 text-[11px] font-medium text-rose-600 dark:text-rose-400"
    >
      <AlertCircle class="w-3 h-3 shrink-0" />
      <span>{{ error }}</span>
    </p>
    <p
      v-else-if="hint"
      class="mt-1 text-[11px] text-neutral-500 dark:text-neutral-400"
    >
      {{ hint }}
    </p>
  </div>
</template>

