<script setup>
import { computed } from 'vue'
import {
  CheckCircle2,
  Clock,
  AlertTriangle,
  XCircle,
  Info,
  CircleDot,
} from 'lucide-vue-next'

const props = defineProps({
  items: {
    type: Array,
    required: true,
    // Array of: { id, title, description, time, author, variant, icon, badge }
  },
  compact: {
    type: Boolean,
    default: false,
  },
})

const getVariantClasses = (variant = 'neutral') => {
  switch (variant) {
    case 'primary':
      return {
        dotBg: 'bg-[var(--theme-primary-soft)] text-[var(--theme-primary)] border-[var(--theme-primary)]',
        badgeBg: 'bg-[var(--theme-primary-soft)] text-[var(--theme-primary)]',
        line: 'border-[var(--theme-primary)]',
      }
    case 'success':
      return {
        dotBg: 'bg-emerald-50 dark:bg-emerald-950/50 text-emerald-600 dark:text-emerald-400 border-emerald-500',
        badgeBg: 'bg-emerald-50 dark:bg-emerald-950/40 text-emerald-600 dark:text-emerald-400',
        line: 'border-emerald-500',
      }
    case 'warning':
      return {
        dotBg: 'bg-amber-50 dark:bg-amber-950/50 text-amber-600 dark:text-amber-400 border-amber-500',
        badgeBg: 'bg-amber-50 dark:bg-amber-950/40 text-amber-600 dark:text-amber-400',
        line: 'border-amber-500',
      }
    case 'danger':
      return {
        dotBg: 'bg-rose-50 dark:bg-rose-950/50 text-rose-600 dark:text-rose-400 border-rose-500',
        badgeBg: 'bg-rose-50 dark:bg-rose-950/40 text-rose-600 dark:text-rose-400',
        line: 'border-rose-500',
      }
    case 'info':
      return {
        dotBg: 'bg-sky-50 dark:bg-sky-950/50 text-sky-600 dark:text-sky-400 border-sky-500',
        badgeBg: 'bg-sky-50 dark:bg-sky-950/40 text-sky-600 dark:text-sky-400',
        line: 'border-sky-500',
      }
    default:
      return {
        dotBg: 'bg-neutral-100 dark:bg-[var(--theme-surface-muted)] text-neutral-600 dark:text-neutral-400 border-neutral-300 dark:border-[var(--theme-border-strong)]',
        badgeBg: 'bg-neutral-100 dark:bg-[var(--theme-surface-muted)] text-neutral-600 dark:text-neutral-400',
        line: 'border-neutral-200 dark:border-[var(--theme-border)]',
      }
  }
}

const getDefaultIcon = (variant = 'neutral') => {
  switch (variant) {
    case 'success':
      return CheckCircle2
    case 'warning':
      return AlertTriangle
    case 'danger':
      return XCircle
    case 'info':
      return Info
    case 'primary':
      return CircleDot
    default:
      return Clock
  }
}
</script>

<template>
  <div class="relative w-full select-none" :class="compact ? 'space-y-3' : 'space-y-4'">
    <div
      v-for="(item, idx) in items"
      :key="item.id || idx"
      class="relative flex items-start gap-3.5 group"
    >
      <!-- Connecting Vertical Line (hidden for the last item) -->
      <div
        v-if="idx < items.length - 1"
        class="absolute left-[15px] top-[26px] bottom-[-16px] w-[2px] bg-neutral-200 dark:bg-[var(--theme-surface-muted)] group-hover:bg-neutral-300 dark:group-hover:bg-[var(--theme-surface-muted)] transition-colors"
      />

      <!-- Node Icon Halo -->
      <div
        class="relative w-8 h-8 rounded-xl border flex items-center justify-center shrink-0 shadow-2xs z-10 transition-transform group-hover:scale-105"
        :class="getVariantClasses(item.variant).dotBg"
      >
        <component
          :is="item.icon || getDefaultIcon(item.variant)"
          class="w-4 h-4 stroke-[2]"
        />
      </div>

      <!-- Content Box -->
      <div class="flex-1 min-w-0 pt-0.5">
        <div class="flex items-center justify-between gap-2 flex-wrap">
          <div class="flex items-center gap-2">
            <h4 class="text-xs sm:text-sm font-bold text-neutral-900 dark:text-white leading-tight">
              {{ item.title }}
            </h4>
            <span
              v-if="item.badge"
              class="text-[10px] font-semibold px-2 py-0.5 rounded-md"
              :class="getVariantClasses(item.variant).badgeBg"
            >
              {{ item.badge }}
            </span>
          </div>

          <!-- Timestamp -->
          <span v-if="item.time" class="text-[10.5px] font-mono text-neutral-400 shrink-0">
            {{ item.time }}
          </span>
        </div>

        <!-- Description -->
        <p
          v-if="item.description"
          class="text-xs text-neutral-600 dark:text-neutral-400 mt-1 leading-relaxed"
        >
          {{ item.description }}
        </p>

        <!-- Optional Author / Meta -->
        <div v-if="item.author" class="flex items-center gap-1.5 mt-1.5 text-[11px] text-neutral-400">
          <span>Oleh:</span>
          <span class="font-medium text-neutral-700 dark:text-neutral-300">{{ item.author }}</span>
        </div>

        <!-- Custom Slot -->
        <div v-if="$slots.default" class="mt-2">
          <slot :item="item" :index="idx" />
        </div>
      </div>
    </div>
  </div>
</template>

