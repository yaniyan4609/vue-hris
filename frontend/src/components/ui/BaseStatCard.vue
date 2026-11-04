<script setup>
import { computed } from 'vue'
import { ArrowUp, ArrowDown, AlertTriangle } from 'lucide-vue-next'
import BaseCountUp from './BaseCountUp.vue'
import BaseSkeleton from './BaseSkeleton.vue'

const props = defineProps({
  title: {
    type: String,
    required: true,
  },
  value: {
    type: Number,
    default: 0,
  },
  secondaryValue: {
    type: [String, Number],
    default: '',
  },
  prefix: {
    type: String,
    default: '',
  },
  suffix: {
    type: String,
    default: '',
  },
  unit: {
    type: String,
    default: '',
  },
  decimals: {
    type: Number,
    default: 0,
  },
  icon: {
    type: [Object, Function],
    default: null,
  },
  tone: {
    type: String,
    default: 'sky', // 'primary' | 'sky' | 'emerald' | 'amber' | 'indigo' | 'cyan' | 'rose' | 'neutral'
  },
  trend: {
    type: String,
    default: '',
  },
  trendDirection: {
    type: String,
    default: 'up', // 'up' | 'down' | 'neutral'
  },
  trendLabel: {
    type: String,
    default: '',
  },
  subtitle: {
    type: String,
    default: '',
  },
  loading: {
    type: Boolean,
    default: false,
  },
})

const iconBoxClasses = computed(() => {
  const map = {
    primary: 'bg-[var(--theme-primary-soft)] text-[var(--theme-primary-soft-text)]',
    sky: 'bg-sky-50 dark:bg-sky-950/40 text-sky-600 dark:text-sky-400',
    emerald: 'bg-emerald-50 dark:bg-emerald-950/40 text-emerald-600 dark:text-emerald-400',
    amber: 'bg-amber-50 dark:bg-amber-950/40 text-amber-600 dark:text-amber-400',
    indigo: 'bg-indigo-50 dark:bg-indigo-950/40 text-indigo-600 dark:text-indigo-400',
    cyan: 'bg-cyan-50 dark:bg-cyan-950/40 text-cyan-600 dark:text-cyan-400',
    rose: 'bg-rose-50 dark:bg-rose-950/40 text-rose-600 dark:text-rose-400',
    neutral: 'bg-neutral-100 dark:bg-[var(--theme-surface-muted)] text-neutral-600 dark:text-neutral-300',
  }
  return map[props.tone] || map.sky
})
</script>

<template>
  <!-- Skeleton Mode -->
  <div
    v-if="loading"
    class="p-3.5 rounded-2xl bg-white dark:bg-[var(--theme-surface)] border border-neutral-200 dark:border-[var(--theme-border)] shadow-2xs space-y-3"
  >
    <div class="flex items-center justify-between">
      <BaseSkeleton width="60%" height="11px" />
      <BaseSkeleton variant="rectangular" width="28px" height="28px" class="rounded-lg" />
    </div>
    <div class="space-y-1.5 pt-1">
      <BaseSkeleton width="75%" height="22px" />
      <BaseSkeleton width="50%" height="11px" />
    </div>
  </div>

  <!-- Loaded Card -->
  <div
    v-else
    class="p-3.5 rounded-2xl bg-white dark:bg-[var(--theme-surface)] border border-neutral-200 dark:border-[var(--theme-border)] shadow-2xs transition flex flex-col justify-between space-y-2 group"
    :class="
      tone === 'rose'
        ? 'hover:border-rose-300 dark:hover:border-rose-900/50'
        : 'hover:border-neutral-300 dark:hover:border-neutral-700'
    "
  >
    <div class="flex items-center justify-between gap-2">
      <span class="text-[11px] font-semibold text-neutral-500 dark:text-neutral-400 truncate">
        {{ title }}
      </span>
      <div
        v-if="icon"
        class="w-7 h-7 rounded-lg flex items-center justify-center shrink-0 group-hover:scale-105 transition-transform"
        :class="iconBoxClasses"
      >
        <component :is="icon" class="w-3.5 h-3.5" />
      </div>
    </div>

    <div>
      <div class="flex items-baseline gap-1.5 leading-none">
        <span
          class="font-bold font-mono"
          :class="[
            prefix ? 'text-base sm:text-lg' : 'text-lg sm:text-xl',
            tone === 'rose' ? 'text-rose-600 dark:text-rose-400' : 'text-neutral-900 dark:text-white',
          ]"
        >
          <BaseCountUp :to="value" :prefix="prefix" :suffix="suffix" :decimals="decimals" />
          <template v-if="secondaryValue !== '' && secondaryValue !== null && secondaryValue !== undefined">
            {{ secondaryValue }}
          </template>
        </span>
        <span v-if="unit" class="text-xs font-normal text-neutral-500 font-sans">
          {{ unit }}
        </span>
      </div>

      <!-- Footer: Trend or Subtitle -->
      <div
        v-if="trend"
        class="flex items-center gap-1 text-[10.5px] font-medium mt-1.5 truncate"
        :class="[
          trendDirection === 'down'
            ? 'text-rose-600 dark:text-rose-400'
            : trendDirection === 'neutral'
            ? 'text-sky-600 dark:text-sky-400'
            : 'text-emerald-600 dark:text-emerald-400',
        ]"
      >
        <ArrowUp v-if="trendDirection === 'up'" class="w-3 h-3 stroke-[3] shrink-0" />
        <ArrowDown v-else-if="trendDirection === 'down'" class="w-3 h-3 stroke-[3] shrink-0" />
        <span>{{ trend }}</span>
        <span v-if="trendLabel" class="text-neutral-400 dark:text-neutral-500 font-normal truncate">
          {{ trendLabel }}
        </span>
      </div>

      <div
        v-else-if="subtitle"
        class="flex items-center gap-1 text-[10.5px] mt-1.5 truncate"
        :class="
          tone === 'rose'
            ? 'text-rose-600 dark:text-rose-400 font-medium'
            : tone === 'cyan'
            ? 'text-sky-600 dark:text-sky-400 font-medium'
            : 'text-neutral-400 dark:text-neutral-500'
        "
      >
        <AlertTriangle v-if="tone === 'rose'" class="w-2.5 h-2.5 shrink-0" />
        <span class="truncate">{{ subtitle }}</span>
      </div>
    </div>
  </div>
</template>

