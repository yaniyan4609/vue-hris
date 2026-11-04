<script setup>
import { computed } from 'vue'

const props = defineProps({
  value: {
    type: Number,
    default: 0,
  },
  max: {
    type: Number,
    default: 100,
  },
  label: {
    type: String,
    default: '',
  },
  showValue: {
    type: Boolean,
    default: true,
  },
  valueFormat: {
    type: String,
    default: 'percent', // 'percent' | 'fraction'
  },
  variant: {
    type: String,
    default: 'primary', // 'primary' | 'success' | 'warning' | 'danger' | 'info'
  },
  size: {
    type: String,
    default: 'md', // 'sm' | 'md' | 'lg'
  },
  striped: {
    type: Boolean,
    default: false,
  },
  indeterminate: {
    type: Boolean,
    default: false,
  },
})

const percentage = computed(() => {
  if (props.max <= 0) return 0
  return Math.min(100, Math.max(0, Math.round((props.value / props.max) * 100)))
})

const formattedValue = computed(() => {
  if (props.valueFormat === 'fraction') {
    return `${props.value} / ${props.max}`
  }
  return `${percentage.value}%`
})

const trackSizeClasses = computed(() => {
  const map = {
    sm: 'h-1.5',
    md: 'h-2.5',
    lg: 'h-3.5',
  }
  return map[props.size] || map.md
})

const barColorClasses = computed(() => {
  const map = {
    primary: 'bg-[var(--theme-primary)]',
    success: 'bg-emerald-500',
    warning: 'bg-amber-500',
    danger: 'bg-rose-500',
    info: 'bg-sky-500',
  }
  return map[props.variant] || map.primary
})
</script>

<template>
  <div class="w-full space-y-1.5">
    <div v-if="label || showValue" class="flex items-center justify-between gap-2 text-xs">
      <span v-if="label" class="font-semibold text-neutral-700 dark:text-neutral-300 truncate">
        {{ label }}
      </span>
      <span v-if="showValue" class="font-mono font-bold text-neutral-600 dark:text-neutral-400 shrink-0">
        {{ formattedValue }}
      </span>
    </div>

    <div
      role="progressbar"
      :aria-valuenow="indeterminate ? undefined : value"
      :aria-valuemin="0"
      :aria-valuemax="max"
      class="w-full rounded-full bg-neutral-100 dark:bg-[var(--theme-surface-muted)] overflow-hidden border border-neutral-200/60 dark:border-[var(--theme-border)]"
      :class="trackSizeClasses"
    >
      <div
        class="h-full rounded-full transition-all duration-300 ease-out"
        :class="[
          barColorClasses,
          striped ? 'progress-striped' : '',
          indeterminate ? 'w-1/3 animate-pulse' : '',
        ]"
        :style="indeterminate ? {} : { width: `${percentage}%` }"
      />
    </div>
  </div>
</template>

<style scoped>
.progress-striped {
  background-image: linear-gradient(
    45deg,
    rgba(255, 255, 255, 0.2) 25%,
    transparent 25%,
    transparent 50%,
    rgba(255, 255, 255, 0.2) 50%,
    rgba(255, 255, 255, 0.2) 75%,
    transparent 75%,
    transparent
  );
  background-size: 16px 16px;
}
</style>

