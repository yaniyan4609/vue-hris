<script setup>
import { computed } from 'vue'

const props = defineProps({
  variant: {
    type: String,
    default: 'primary', // 'primary' | 'secondary' | 'outline' | 'danger' | 'ghost'
  },
  size: {
    type: String,
    default: 'md', // 'sm' | 'md' | 'lg'
  },
  disabled: {
    type: Boolean,
    default: false,
  },
  type: {
    type: String,
    default: 'button',
  },
})

const variantClasses = computed(() => {
  const map = {
    primary: 'bg-[var(--theme-primary)] hover:bg-[var(--theme-primary-hover)] active:bg-[var(--theme-primary-pressed)] text-[var(--theme-primary-text)] shadow-2xs font-semibold',
    secondary: 'bg-neutral-800 hover:bg-neutral-900 text-white dark:bg-[var(--theme-surface-muted)] dark:hover:bg-[var(--theme-surface-muted)] dark:text-white shadow-2xs font-semibold',
    outline: 'border border-neutral-300 dark:border-[var(--theme-border)] bg-white hover:bg-neutral-50 text-neutral-800 dark:bg-[var(--theme-surface-raised)] dark:text-neutral-200 dark:hover:bg-[var(--theme-surface-muted)] shadow-2xs',
    danger: 'bg-rose-600 hover:bg-rose-700 text-white shadow-2xs font-semibold',
    ghost: 'text-neutral-600 hover:bg-neutral-100 hover:text-neutral-900 dark:text-neutral-300 dark:hover:bg-[var(--theme-surface-muted)] dark:hover:text-white',
  }
  return map[props.variant] || map.primary
})

const sizeClasses = computed(() => {
  const map = {
    sm: 'text-xs px-3 py-1.5 rounded-xl gap-1.5',
    md: 'text-xs px-3.5 py-2 rounded-xl gap-2 font-medium',
    lg: 'text-sm px-4.5 py-2.5 rounded-xl gap-2 font-medium',
  }
  return map[props.size] || map.md
})
</script>

<template>
  <button
    :type="type"
    :disabled="disabled"
    :class="[
      'inline-flex items-center justify-center transition-all duration-150 outline-none select-none cursor-pointer focus-visible:ring-2 disabled:opacity-50 disabled:pointer-events-none active:scale-[0.98]',
      variantClasses,
      sizeClasses,
    ]"
  >
    <slot />
  </button>
</template>
