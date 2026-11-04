<script setup>
import { computed } from 'vue'

const props = defineProps({
  variant: {
    type: String,
    default: 'text', // 'text' | 'circular' | 'rectangular' | 'card'
    validator: (v) => ['text', 'circular', 'rectangular', 'card'].includes(v),
  },
  width: {
    type: String,
    default: '',
  },
  height: {
    type: String,
    default: '',
  },
  lines: {
    type: Number,
    default: 1, // For multi-line text skeleton
  },
  shimmer: {
    type: Boolean,
    default: true,
  },
})

const variantClasses = computed(() => {
  const map = {
    text: 'h-3.5 w-full rounded-md',
    circular: 'rounded-full shrink-0',
    rectangular: 'rounded-xl',
    card: 'rounded-2xl p-4 space-y-3',
  }
  return map[props.variant] || map.text
})

const animClass = computed(() => {
  return props.shimmer ? 'skeleton-shimmer' : 'bg-neutral-200/80 dark:bg-[var(--theme-surface-muted)] animate-pulse'
})

const customStyle = computed(() => {
  const style = {}
  if (props.width) style.width = props.width
  if (props.height) style.height = props.height
  return style
})
</script>

<template>
  <!-- Card Skeleton Container -->
  <div
    v-if="variant === 'card'"
    class="border border-neutral-200/80 dark:border-[var(--theme-border)] bg-white dark:bg-[var(--theme-surface)] shadow-2xs"
    :class="variantClasses"
    :style="customStyle"
  >
    <div class="flex items-center gap-3">
      <div class="w-8 h-8 rounded-xl shrink-0" :class="animClass" />
      <div class="space-y-1.5 flex-1">
        <div class="h-3 w-1/3 rounded-md" :class="animClass" />
        <div class="h-2.5 w-1/2 rounded-md" :class="animClass" />
      </div>
    </div>
    <div class="h-8 w-2/3 rounded-lg" :class="animClass" />
  </div>

  <!-- Multi-line Text Skeleton -->
  <div v-else-if="variant === 'text' && lines > 1" class="space-y-2 w-full">
    <div
      v-for="i in lines"
      :key="i"
      class="rounded-md"
      :class="[
        animClass,
        height ? '' : 'h-3.5',
        i === lines ? 'w-2/3' : 'w-full',
      ]"
      :style="customStyle"
    />
  </div>

  <!-- Standard Single Skeleton Item (Text, Circular, Rectangular) -->
  <div
    v-else
    class="transition-all select-none"
    :class="[variantClasses, animClass]"
    :style="customStyle"
  />
</template>

