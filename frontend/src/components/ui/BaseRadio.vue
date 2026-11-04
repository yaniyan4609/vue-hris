<script setup>
import { computed } from 'vue'

const props = defineProps({
  modelValue: {
    type: [String, Number, Boolean, Object],
    default: null,
  },
  value: {
    type: [String, Number, Boolean, Object],
    default: null,
  },
  options: {
    type: Array,
    default: null,
    // Optional group mode: [{ value, label, description?, disabled? }]
  },
  name: {
    type: String,
    default: '',
  },
  label: {
    type: String,
    default: '',
  },
  description: {
    type: String,
    default: '',
  },
  disabled: {
    type: Boolean,
    default: false,
  },
  size: {
    type: String,
    default: 'md', // 'sm' | 'md' | 'lg'
  },
  direction: {
    type: String,
    default: 'vertical', // 'vertical' | 'horizontal'
  },
})

const emit = defineEmits(['update:modelValue', 'change'])

const isGroupMode = computed(() => Array.isArray(props.options) && props.options.length > 0)

const normalizedGroupOptions = computed(() => {
  if (!isGroupMode.value) return []
  return props.options.map((opt) => {
    if (typeof opt === 'object' && opt !== null) {
      return {
        label: opt.label ?? String(opt.value ?? ''),
        value: opt.value ?? opt.label,
        description: opt.description ?? '',
        disabled: Boolean(opt.disabled) || props.disabled,
      }
    }
    return {
      label: String(opt),
      value: opt,
      description: '',
      disabled: props.disabled,
    }
  })
})

const isSelected = computed(() => props.modelValue === props.value)

const selectSingle = () => {
  if (props.disabled) return
  emit('update:modelValue', props.value)
  emit('change', props.value)
}

const selectOption = (opt) => {
  if (opt.disabled) return
  emit('update:modelValue', opt.value)
  emit('change', opt.value)
}

const circleSizeClasses = computed(() => {
  const map = {
    sm: 'w-4 h-4',
    md: 'w-4.5 h-4.5',
    lg: 'w-5 h-5',
  }
  return map[props.size] || map.md
})

const dotSizeClasses = computed(() => {
  const map = {
    sm: 'w-1.5 h-1.5',
    md: 'w-2 h-2',
    lg: 'w-2.5 h-2.5',
  }
  return map[props.size] || map.md
})
</script>

<template>
  <!-- Group Mode -->
  <div
    v-if="isGroupMode"
    role="radiogroup"
    class="flex"
    :class="direction === 'horizontal' ? 'flex-row flex-wrap items-center gap-4' : 'flex-col gap-2.5'"
  >
    <label
      v-for="opt in normalizedGroupOptions"
      :key="String(opt.value)"
      class="inline-flex items-start gap-2.5 select-none group"
      :class="opt.disabled ? 'opacity-40 cursor-not-allowed' : 'cursor-pointer'"
      @click.prevent="selectOption(opt)"
    >
      <button
        type="button"
        role="radio"
        :name="name || undefined"
        :aria-checked="modelValue === opt.value"
        :disabled="opt.disabled"
        @keydown.space.prevent.stop="selectOption(opt)"
        @keydown.enter.prevent.stop="selectOption(opt)"
        class="relative shrink-0 rounded-full inline-flex items-center justify-center border transition-all duration-150 focus:outline-none focus-visible:ring-2 focus-visible:ring-[var(--theme-primary)] focus-visible:ring-offset-1 dark:focus-visible:ring-offset-[var(--theme-surface)] shadow-2xs mt-0.5"
        :class="[
          circleSizeClasses,
          modelValue === opt.value
            ? 'border-[var(--theme-primary)] bg-white dark:bg-[var(--theme-surface-raised)]'
            : 'bg-white dark:bg-[var(--theme-surface-raised)] border-neutral-300 dark:border-[var(--theme-border-strong)] group-hover:border-neutral-400 dark:group-hover:border-neutral-500',
          opt.disabled ? 'cursor-not-allowed' : 'cursor-pointer active:scale-95',
        ]"
      >
        <span
          v-if="modelValue === opt.value"
          :class="dotSizeClasses"
          class="rounded-full bg-[var(--theme-primary)] transition-transform duration-150 scale-100"
        />
      </button>

      <div class="min-w-0 flex-1 leading-snug">
        <span class="block text-xs font-semibold text-neutral-800 dark:text-neutral-200 group-hover:text-neutral-900 dark:group-hover:text-white transition-colors">
          {{ opt.label }}
        </span>
        <span
          v-if="opt.description"
          class="block text-[11px] text-neutral-500 dark:text-neutral-400 mt-0.5"
        >
          {{ opt.description }}
        </span>
      </div>
    </label>
  </div>

  <!-- Single Radio Mode -->
  <label
    v-else
    class="inline-flex items-start gap-2.5 select-none group"
    :class="disabled ? 'opacity-40 cursor-not-allowed' : 'cursor-pointer'"
    @click.prevent="selectSingle"
  >
    <button
      type="button"
      role="radio"
      :name="name || undefined"
      :aria-checked="isSelected"
      :disabled="disabled"
      @keydown.space.prevent.stop="selectSingle"
      @keydown.enter.prevent.stop="selectSingle"
      class="relative shrink-0 rounded-full inline-flex items-center justify-center border transition-all duration-150 focus:outline-none focus-visible:ring-2 focus-visible:ring-[var(--theme-primary)] focus-visible:ring-offset-1 dark:focus-visible:ring-offset-[var(--theme-surface)] shadow-2xs"
      :class="[
        circleSizeClasses,
        label || description ? 'mt-0.5' : '',
        isSelected
          ? 'border-[var(--theme-primary)] bg-white dark:bg-[var(--theme-surface-raised)]'
          : 'bg-white dark:bg-[var(--theme-surface-raised)] border-neutral-300 dark:border-[var(--theme-border-strong)] group-hover:border-neutral-400 dark:group-hover:border-neutral-500',
        disabled ? 'cursor-not-allowed' : 'cursor-pointer active:scale-95',
      ]"
    >
      <span
        v-if="isSelected"
        :class="dotSizeClasses"
        class="rounded-full bg-[var(--theme-primary)] transition-transform duration-150 scale-100"
      />
    </button>

    <div v-if="label || description || $slots.default" class="min-w-0 flex-1 leading-snug">
      <span
        v-if="label || $slots.default"
        class="block text-xs font-semibold text-neutral-800 dark:text-neutral-200 group-hover:text-neutral-900 dark:group-hover:text-white transition-colors"
      >
        <slot>{{ label }}</slot>
      </span>
      <span
        v-if="description"
        class="block text-[11px] text-neutral-500 dark:text-neutral-400 mt-0.5"
      >
        {{ description }}
      </span>
    </div>
  </label>
</template>

