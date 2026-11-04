<script setup>
import { computed } from 'vue'
import { X } from 'lucide-vue-next'

const props = defineProps({
  variant: {
    type: String,
    default: 'neutral', // 'neutral' | 'success' | 'danger' | 'warning' | 'info' | 'purple'
    validator: (v) => ['neutral', 'success', 'danger', 'warning', 'info', 'purple'].includes(v),
  },
  size: {
    type: String,
    default: 'sm', // 'sm' | 'md'
  },
  dot: {
    type: Boolean,
    default: false,
  },
  pulse: {
    type: Boolean,
    default: false,
  },
  removable: {
    type: Boolean,
    default: false,
  },
})

const emit = defineEmits(['remove'])

const variantClasses = computed(() => {
  const map = {
    neutral:
      'bg-neutral-100 text-neutral-700 dark:bg-[var(--theme-surface-muted)] dark:text-neutral-300 border-neutral-200/80 dark:border-[var(--theme-border-strong)]',
    success:
      'bg-emerald-50 text-emerald-700 dark:bg-emerald-950/40 dark:text-emerald-400 border-emerald-200/80 dark:border-emerald-900/40',
    danger:
      'bg-rose-50 text-rose-700 dark:bg-rose-950/40 dark:text-rose-400 border-rose-200/80 dark:border-rose-900/40',
    warning:
      'bg-amber-50 text-amber-800 dark:bg-amber-950/40 dark:text-amber-400 border-amber-200/80 dark:border-amber-900/40',
    info:
      'bg-sky-50 text-sky-700 dark:bg-sky-950/40 dark:text-sky-400 border-sky-200/80 dark:border-sky-900/40',
    purple:
      'bg-purple-50 text-purple-700 dark:bg-purple-950/40 dark:text-purple-400 border-purple-200/80 dark:border-purple-900/40',
  }
  return map[props.variant] || map.neutral
})

const dotColor = computed(() => {
  const map = {
    neutral: 'bg-neutral-500',
    success: 'bg-emerald-500',
    danger: 'bg-rose-500',
    warning: 'bg-amber-500',
    info: 'bg-sky-500',
    purple: 'bg-purple-500',
  }
  return map[props.variant] || 'bg-neutral-500'
})

const sizeClasses = computed(() => {
  const map = {
    sm: 'text-[10.5px] px-2 py-0.5 gap-1.5',
    md: 'text-xs px-2.5 py-1 gap-2',
  }
  return map[props.size] || map.sm
})
</script>

<template>
  <span
    class="inline-flex items-center font-semibold rounded-full border shadow-2xs select-none transition-colors"
    :class="[variantClasses, sizeClasses]"
  >
    <!-- Dot / Pulse Indicator -->
    <span v-if="dot || pulse" class="relative flex h-1.5 w-1.5 shrink-0">
      <span
        v-if="pulse"
        class="animate-ping absolute inline-flex h-full w-full rounded-full opacity-75"
        :class="dotColor"
      />
      <span class="relative inline-flex rounded-full h-1.5 w-1.5" :class="dotColor" />
    </span>

    <!-- Badge Content -->
    <span class="truncate leading-none">
      <slot />
    </span>

    <!-- Optional Close / Remove Button -->
    <button
      v-if="removable"
      type="button"
      @click.stop="emit('remove')"
      class="p-0.5 -mr-0.5 rounded-full hover:bg-black/10 dark:hover:bg-white/10 transition cursor-pointer text-current"
      title="Hapus"
    >
      <X class="w-3 h-3" />
    </button>
  </span>
</template>

