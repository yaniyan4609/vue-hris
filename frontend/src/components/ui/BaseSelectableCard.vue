<script setup>
import { computed } from 'vue'
import { Check } from 'lucide-vue-next'
import BaseBadge from './BaseBadge.vue'

const props = defineProps({
  modelValue: {
    type: [Boolean, String, Number, Array],
    default: false,
  },
  value: {
    type: [String, Number, Boolean],
    default: true,
  },
  multiple: {
    type: Boolean,
    default: false,
  },
  title: {
    type: String,
    default: '',
  },
  description: {
    type: String,
    default: '',
  },
  badge: {
    type: String,
    default: '',
  },
  badgeVariant: {
    type: String,
    default: 'neutral',
  },
  icon: {
    type: [Object, Function],
    default: null,
  },
  disabled: {
    type: Boolean,
    default: false,
  },
})

const emit = defineEmits(['update:modelValue', 'change'])

const isSelected = computed(() => {
  if (props.multiple || Array.isArray(props.modelValue)) {
    return Array.isArray(props.modelValue) && props.modelValue.includes(props.value)
  }
  if (typeof props.modelValue === 'boolean' && props.value === true) {
    return props.modelValue
  }
  return props.modelValue === props.value
})

const toggle = () => {
  if (props.disabled) return
  let next
  if (props.multiple || Array.isArray(props.modelValue)) {
    const arr = Array.isArray(props.modelValue) ? [...props.modelValue] : []
    const idx = arr.indexOf(props.value)
    if (idx !== -1) arr.splice(idx, 1)
    else arr.push(props.value)
    next = arr
  } else if (typeof props.modelValue === 'boolean' && props.value === true) {
    next = !props.modelValue
  } else {
    next = props.value
  }
  emit('update:modelValue', next)
  emit('change', next)
}
</script>

<template>
  <div
    role="button"
    tabindex="0"
    :aria-pressed="isSelected"
    @click="toggle"
    @keydown.space.prevent="toggle"
    @keydown.enter.prevent="toggle"
    class="p-3.5 rounded-2xl border transition-all duration-150 select-none text-left flex flex-col justify-between gap-2.5"
    :class="[
      disabled ? 'opacity-40 cursor-not-allowed' : 'cursor-pointer',
      isSelected
        ? 'bg-[var(--theme-primary-soft)]/30 dark:bg-[var(--theme-primary-soft)]/15 border-[var(--theme-primary)] ring-2 ring-[var(--theme-primary)]/20 shadow-xs'
        : 'bg-white dark:bg-[var(--theme-surface)] border-neutral-200 dark:border-[var(--theme-border)] hover:border-neutral-300 dark:hover:border-[var(--theme-border-strong)] shadow-2xs',
    ]"
  >
    <div class="flex items-start justify-between gap-3">
      <div class="flex items-start gap-2.5 min-w-0 flex-1">
        <div
          v-if="icon"
          class="w-8 h-8 rounded-xl flex items-center justify-center shrink-0 transition-colors"
          :class="
            isSelected
              ? 'bg-[var(--theme-primary)] text-[var(--theme-primary-text)]'
              : 'bg-neutral-100 dark:bg-[var(--theme-surface-muted)] text-neutral-600 dark:text-neutral-300'
          "
        >
          <component :is="icon" class="w-4 h-4" />
        </div>

        <div class="min-w-0 flex-1">
          <div class="flex items-center gap-2 flex-wrap">
            <p class="text-xs font-bold text-neutral-900 dark:text-white">{{ title }}</p>
            <BaseBadge v-if="badge" :variant="badgeVariant">{{ badge }}</BaseBadge>
          </div>
          <p v-if="description" class="text-[11px] text-neutral-500 dark:text-neutral-400 mt-0.5 leading-relaxed">
            {{ description }}
          </p>
          <div v-if="$slots.default" class="mt-1.5 text-xs">
            <slot />
          </div>
        </div>
      </div>

      <!-- Right Check Indicator -->
      <div class="flex items-center gap-2 shrink-0">
        <slot name="right" />
        <div
          class="w-4.5 h-4.5 rounded-full border flex items-center justify-center transition-colors"
          :class="
            isSelected
              ? 'bg-[var(--theme-primary)] border-[var(--theme-primary)] text-[var(--theme-primary-text)]'
              : 'border-neutral-300 dark:border-[var(--theme-border-strong)] bg-white dark:bg-[var(--theme-surface-raised)]'
          "
        >
          <Check v-if="isSelected" class="w-3 h-3 stroke-[3]" />
        </div>
      </div>
    </div>

    <div
      v-if="$slots.footer"
      class="pt-2 border-t border-neutral-200/70 dark:border-[var(--theme-border)] text-[11px]"
    >
      <slot name="footer" />
    </div>
  </div>
</template>

