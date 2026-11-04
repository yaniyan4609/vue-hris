<script setup>
import { computed } from 'vue'
import { Info, CheckCircle2, AlertTriangle, ShieldAlert, X } from 'lucide-vue-next'

const props = defineProps({
  variant: {
    type: String,
    default: 'info', // 'info' | 'success' | 'warning' | 'danger' | 'neutral'
  },
  title: {
    type: String,
    default: '',
  },
  description: {
    type: String,
    default: '',
  },
  icon: {
    type: [Object, Function],
    default: null,
  },
  dismissible: {
    type: Boolean,
    default: false,
  },
  banner: {
    type: Boolean,
    default: false,
  },
  actionText: {
    type: String,
    default: '',
  },
})

const emit = defineEmits(['dismiss', 'action'])

const resolvedIcon = computed(() => {
  if (props.icon) return props.icon
  const map = {
    info: Info,
    success: CheckCircle2,
    warning: AlertTriangle,
    danger: ShieldAlert,
    neutral: Info,
  }
  return map[props.variant] || Info
})

const containerClasses = computed(() => {
  const map = {
    info: 'bg-sky-50/80 dark:bg-sky-950/25 border-sky-200/80 dark:border-sky-900/50 text-sky-900 dark:text-sky-200',
    success: 'bg-emerald-50/80 dark:bg-emerald-950/25 border-emerald-200/80 dark:border-emerald-900/50 text-emerald-900 dark:text-emerald-200',
    warning: 'bg-amber-50/80 dark:bg-amber-950/25 border-amber-200/80 dark:border-amber-900/50 text-amber-900 dark:text-amber-200',
    danger: 'bg-rose-50/80 dark:bg-rose-950/25 border-rose-200/80 dark:border-rose-900/50 text-rose-900 dark:text-rose-200',
    neutral: 'bg-neutral-50 dark:bg-[var(--theme-surface-raised)] border-neutral-200 dark:border-[var(--theme-border)] text-neutral-800 dark:text-neutral-200',
  }
  return map[props.variant] || map.info
})

const iconBoxClasses = computed(() => {
  const map = {
    info: 'bg-sky-100/80 dark:bg-sky-900/40 text-sky-600 dark:text-sky-400',
    success: 'bg-emerald-100/80 dark:bg-emerald-900/40 text-emerald-600 dark:text-emerald-400',
    warning: 'bg-amber-100/80 dark:bg-amber-900/40 text-amber-600 dark:text-amber-400',
    danger: 'bg-rose-100/80 dark:bg-rose-900/40 text-rose-600 dark:text-rose-400',
    neutral: 'bg-neutral-200/70 dark:bg-[var(--theme-surface-muted)] text-neutral-600 dark:text-neutral-300',
  }
  return map[props.variant] || map.info
})
</script>

<template>
  <div
    role="alert"
    class="flex items-start gap-3 border transition-all"
    :class="[
      containerClasses,
      banner ? 'w-full rounded-xl px-4 py-2.5' : 'rounded-2xl p-3.5',
    ]"
  >
    <!-- Icon -->
    <div
      class="w-7 h-7 rounded-xl flex items-center justify-center shrink-0 mt-0.5"
      :class="iconBoxClasses"
    >
      <component :is="resolvedIcon" class="w-4 h-4" />
    </div>

    <!-- Content -->
    <div class="flex-1 min-w-0">
      <div class="flex flex-wrap items-center justify-between gap-2">
        <p v-if="title" class="text-xs font-bold leading-snug">
          {{ title }}
        </p>
      </div>
      <div
        v-if="description || $slots.default"
        class="text-[11.5px] opacity-90 leading-relaxed"
        :class="title ? 'mt-0.5' : ''"
      >
        <slot>{{ description }}</slot>
      </div>

      <!-- Optional Actions -->
      <div v-if="actionText || $slots.actions" class="mt-2 flex items-center gap-2">
        <slot name="actions">
          <button
            type="button"
            @click="emit('action')"
            class="px-2.5 py-1 rounded-lg text-[11px] font-bold bg-white/80 dark:bg-black/25 hover:bg-white dark:hover:bg-black/40 border border-current/15 transition cursor-pointer"
          >
            {{ actionText }}
          </button>
        </slot>
      </div>
    </div>

    <!-- Dismiss Button -->
    <button
      v-if="dismissible"
      type="button"
      @click="emit('dismiss')"
      class="p-1 rounded-lg opacity-60 hover:opacity-100 hover:bg-black/5 dark:hover:bg-white/10 transition cursor-pointer shrink-0"
      title="Tutup"
    >
      <X class="w-3.5 h-3.5" />
    </button>
  </div>
</template>

