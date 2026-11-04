<script setup>
import BaseBadge from './BaseBadge.vue'

defineProps({
  items: {
    type: Array,
    required: true,
    // Each item: { key?, label, value, mono?, badge?, badgeVariant?, span?: 1|2 }
  },
  columns: {
    type: Number,
    default: 2, // 1 | 2 | 3
  },
  variant: {
    type: String,
    default: 'cards', // 'cards' | 'divided'
  },
})
</script>

<template>
  <!-- Cards Grid Variant (Default Digaji.id SlideOver style) -->
  <dl
    v-if="variant === 'cards'"
    class="grid gap-2.5 text-xs"
    :class="[
      columns === 1 ? 'grid-cols-1' : columns === 3 ? 'grid-cols-1 sm:grid-cols-3' : 'grid-cols-1 sm:grid-cols-2',
    ]"
  >
    <div
      v-for="(item, idx) in items"
      :key="item.key || idx"
      class="p-3 rounded-xl bg-neutral-50 dark:bg-[var(--theme-surface-raised)] border border-neutral-200/60 dark:border-[var(--theme-border)]"
      :class="item.span === 2 && columns >= 2 ? 'sm:col-span-2' : ''"
    >
      <dt class="text-[11px] font-medium text-neutral-400 dark:text-neutral-500">
        {{ item.label }}
      </dt>
      <dd
        class="mt-0.5 font-semibold text-neutral-800 dark:text-neutral-200 flex items-center justify-between gap-2"
        :class="item.mono ? 'font-mono font-bold' : ''"
      >
        <slot :name="`value-${item.key || idx}`" :item="item">
          <span class="truncate">{{ item.value ?? '—' }}</span>
          <BaseBadge v-if="item.badge" :variant="item.badgeVariant || 'neutral'">
            {{ item.badge }}
          </BaseBadge>
        </slot>
      </dd>
    </div>
  </dl>

  <!-- Divided List Variant -->
  <dl
    v-else
    class="divide-y divide-neutral-200/80 dark:divide-[var(--theme-border)] rounded-2xl border border-neutral-200 dark:border-[var(--theme-border)] bg-white dark:bg-[var(--theme-surface)] overflow-hidden text-xs"
  >
    <div
      v-for="(item, idx) in items"
      :key="item.key || idx"
      class="px-4 py-2.5 flex items-center justify-between gap-4"
    >
      <dt class="text-neutral-500 dark:text-neutral-400 font-medium">
        {{ item.label }}
      </dt>
      <dd
        class="font-semibold text-neutral-900 dark:text-white text-right flex items-center gap-2"
        :class="item.mono ? 'font-mono font-bold' : ''"
      >
        <slot :name="`value-${item.key || idx}`" :item="item">
          <span>{{ item.value ?? '—' }}</span>
          <BaseBadge v-if="item.badge" :variant="item.badgeVariant || 'neutral'">
            {{ item.badge }}
          </BaseBadge>
        </slot>
      </dd>
    </div>
  </dl>
</template>

