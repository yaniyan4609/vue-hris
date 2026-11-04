<script setup>
import { ref, computed, watch } from 'vue'

const props = defineProps({
  src: {
    type: String,
    default: '',
  },
  name: {
    type: String,
    default: '',
  },
  initials: {
    type: String,
    default: '',
  },
  size: {
    type: String,
    default: 'md', // 'xs' | 'sm' | 'md' | 'lg' | 'xl'
  },
  shape: {
    type: String,
    default: 'rounded', // 'rounded' | 'circle'
  },
  status: {
    type: String,
    default: '', // '' | 'online' | 'offline' | 'busy' | 'away'
  },
  tone: {
    type: String,
    default: 'neutral', // 'primary' | 'neutral' | 'emerald' | 'sky' | 'amber' | 'rose' | 'purple'
  },
})

const imgError = ref(false)

watch(
  () => props.src,
  () => {
    imgError.value = false
  }
)

const computedInitials = computed(() => {
  if (props.initials) return props.initials.slice(0, 2).toUpperCase()
  if (!props.name) return '?'
  const parts = props.name.trim().split(/\s+/)
  if (parts.length === 1) return parts[0].slice(0, 2).toUpperCase()
  return (parts[0][0] + parts[parts.length - 1][0]).toUpperCase()
})

const sizeClasses = computed(() => {
  const map = {
    xs: 'w-6 h-6 text-[10px]',
    sm: 'w-8 h-8 text-[11px]',
    md: 'w-9 h-9 text-xs',
    lg: 'w-11 h-11 text-sm',
    xl: 'w-14 h-14 text-base',
  }
  return map[props.size] || map.md
})

const shapeClasses = computed(() => {
  if (props.shape === 'circle') return 'rounded-full'
  const map = {
    xs: 'rounded-lg',
    sm: 'rounded-xl',
    md: 'rounded-xl',
    lg: 'rounded-2xl',
    xl: 'rounded-2xl',
  }
  return map[props.size] || 'rounded-xl'
})

const toneClasses = computed(() => {
  const map = {
    primary: 'bg-[var(--theme-primary)] text-[var(--theme-primary-text)] border-transparent',
    neutral: 'bg-neutral-100 dark:bg-[var(--theme-surface-muted)] text-neutral-700 dark:text-neutral-200 border-neutral-200/80 dark:border-[var(--theme-border-strong)]',
    emerald: 'bg-emerald-50 dark:bg-emerald-950/40 text-emerald-700 dark:text-emerald-300 border-emerald-200/70 dark:border-emerald-800/50',
    sky: 'bg-sky-50 dark:bg-sky-950/40 text-sky-700 dark:text-sky-300 border-sky-200/70 dark:border-sky-800/50',
    amber: 'bg-amber-50 dark:bg-amber-950/40 text-amber-700 dark:text-amber-300 border-amber-200/70 dark:border-amber-800/50',
    rose: 'bg-rose-50 dark:bg-rose-950/40 text-rose-700 dark:text-rose-300 border-rose-200/70 dark:border-rose-800/50',
    purple: 'bg-purple-50 dark:bg-purple-950/40 text-purple-700 dark:text-purple-300 border-purple-200/70 dark:border-purple-800/50',
  }
  return map[props.tone] || map.neutral
})

const statusColorClasses = computed(() => {
  const map = {
    online: 'bg-emerald-500',
    offline: 'bg-neutral-400',
    busy: 'bg-rose-500',
    away: 'bg-amber-500',
  }
  return map[props.status] || ''
})
</script>

<template>
  <div class="relative inline-flex items-center justify-center shrink-0 select-none">
    <div
      class="inline-flex items-center justify-center font-bold border overflow-hidden shadow-2xs"
      :class="[sizeClasses, shapeClasses, toneClasses]"
      :title="name || undefined"
    >
      <img
        v-if="src && !imgError"
        :src="src"
        :alt="name || 'Avatar'"
        class="w-full h-full object-cover"
        @error="imgError = true"
      />
      <span v-else>{{ computedInitials }}</span>
    </div>

    <!-- Optional Status Dot -->
    <span
      v-if="status"
      class="absolute -bottom-0.5 -right-0.5 w-2.5 h-2.5 rounded-full ring-2 ring-white dark:ring-[var(--theme-surface)]"
      :class="statusColorClasses"
    />
  </div>
</template>

