<script setup>
import BaseBadge from './BaseBadge.vue'

const props = defineProps({
  modelValue: {
    type: [String, Number],
    required: true,
  },
  tabs: {
    type: Array,
    required: true,
    // Each tab: { value, label, icon?, badge?, badgeVariant?, disabled? }
  },
  variant: {
    type: String,
    default: 'underline', // 'underline' | 'pills' | 'cards'
  },
  size: {
    type: String,
    default: 'md', // 'sm' | 'md'
  },
  fullWidth: {
    type: Boolean,
    default: false,
  },
})

const emit = defineEmits(['update:modelValue', 'change'])

const selectTab = (tab) => {
  if (tab.disabled || tab.value === props.modelValue) return
  emit('update:modelValue', tab.value)
  emit('change', tab.value)
}
</script>

<template>
  <div class="w-full">
    <!-- Tab Header Bar -->
    <div
      class="flex items-center justify-between gap-2"
      :class="[
        variant === 'underline'
          ? 'border-b border-neutral-200 dark:border-[var(--theme-border)]'
          : '',
      ]"
    >
      <div
        role="tablist"
        class="flex items-center overflow-x-auto no-scrollbar"
        :class="[
          fullWidth ? 'w-full' : '',
          variant === 'underline' ? 'gap-1 -mb-px' : '',
          variant === 'pills'
            ? 'p-1 rounded-xl bg-neutral-100/90 dark:bg-[var(--theme-layer-2)] border border-neutral-200/70 dark:border-[var(--theme-border)] gap-1'
            : '',
          variant === 'cards' ? 'gap-1.5' : '',
        ]"
      >
        <button
          v-for="tab in tabs"
          :key="String(tab.value)"
          type="button"
          role="tab"
          :aria-selected="modelValue === tab.value"
          :disabled="tab.disabled"
          @click="selectTab(tab)"
          class="inline-flex items-center justify-center gap-2 font-semibold transition-all duration-150 whitespace-nowrap select-none focus:outline-none focus-visible:ring-2 focus-visible:ring-[var(--theme-primary)]"
          :class="[
            fullWidth ? 'flex-1' : '',
            size === 'sm' ? 'text-[11px] px-2.5 py-1.5' : 'text-xs px-3.5 py-2',
            tab.disabled ? 'opacity-40 cursor-not-allowed' : 'cursor-pointer',
            variant === 'underline'
              ? modelValue === tab.value
                ? 'border-b-2 border-[var(--theme-primary)] text-neutral-900 dark:text-white'
                : 'border-b-2 border-transparent text-neutral-500 dark:text-neutral-400 hover:text-neutral-800 dark:hover:text-neutral-200 hover:border-neutral-300 dark:hover:border-[var(--theme-border-strong)]'
              : '',
            variant === 'pills'
              ? modelValue === tab.value
                ? 'rounded-lg bg-white dark:bg-[var(--theme-surface-raised)] text-neutral-900 dark:text-white shadow-xs border border-neutral-200/80 dark:border-[var(--theme-border-strong)]'
                : 'rounded-lg text-neutral-600 dark:text-neutral-400 hover:text-neutral-900 dark:hover:text-white'
              : '',
            variant === 'cards'
              ? modelValue === tab.value
                ? 'rounded-xl bg-[var(--theme-primary)] text-[var(--theme-primary-text)] shadow-xs'
                : 'rounded-xl bg-white dark:bg-[var(--theme-surface)] border border-neutral-200 dark:border-[var(--theme-border)] text-neutral-600 dark:text-neutral-400 hover:bg-neutral-50 dark:hover:bg-[var(--theme-surface-raised)]'
              : '',
          ]"
        >
          <component
            :is="tab.icon"
            v-if="tab.icon"
            class="w-3.5 h-3.5 shrink-0"
          />
          <span>{{ tab.label }}</span>
          <BaseBadge
            v-if="tab.badge !== undefined && tab.badge !== null"
            :variant="tab.badgeVariant || (modelValue === tab.value ? 'info' : 'neutral')"
          >
            {{ tab.badge }}
          </BaseBadge>
        </button>
      </div>

      <!-- Optional Right Actions Slot -->
      <div v-if="$slots.actions" class="shrink-0">
        <slot name="actions" />
      </div>
    </div>

    <!-- Tab Panels -->
    <div v-if="$slots.default || $slots[`panel-${modelValue}`]" class="mt-3">
      <slot :name="`panel-${modelValue}`">
        <slot :active-tab="modelValue" />
      </slot>
    </div>
  </div>
</template>

