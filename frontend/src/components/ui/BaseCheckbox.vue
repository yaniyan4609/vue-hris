<script setup>
import { computed } from 'vue'
import { Check, Minus } from 'lucide-vue-next'

const props = defineProps({
  modelValue: {
    type: [Boolean, Array],
    default: false,
  },
  value: {
    type: [String, Number, Object, Boolean],
    default: null,
  },
  options: {
    type: Array,
    default: null,
    // Optional group mode: [{ value, label, description?, disabled? }]
  },
  direction: {
    type: String,
    default: 'vertical', // 'vertical' | 'horizontal'
  },
  indeterminate: {
    type: Boolean,
    default: false,
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

const isChecked = computed(() => {
  if (Array.isArray(props.modelValue)) {
    return props.modelValue.includes(props.value)
  }
  return Boolean(props.modelValue)
})

const isOptionChecked = (val) => {
  return Array.isArray(props.modelValue) && props.modelValue.includes(val)
}

const toggle = () => {
  if (props.disabled) return

  if (Array.isArray(props.modelValue)) {
    const next = [...props.modelValue]
    const idx = next.indexOf(props.value)
    if (idx !== -1) {
      next.splice(idx, 1)
    } else {
      next.push(props.value)
    }
    emit('update:modelValue', next)
    emit('change', next)
  } else {
    const next = !props.modelValue
    emit('update:modelValue', next)
    emit('change', next)
  }
}

const toggleGroupOption = (opt) => {
  if (opt.disabled) return
  const current = Array.isArray(props.modelValue) ? [...props.modelValue] : []
  const idx = current.indexOf(opt.value)
  if (idx !== -1) {
    current.splice(idx, 1)
  } else {
    current.push(opt.value)
  }
  emit('update:modelValue', current)
  emit('change', current)
}

const boxSizeClasses = computed(() => {
  const map = {
    sm: 'w-4 h-4 rounded-[5px]',
    md: 'w-4.5 h-4.5 rounded-md',
    lg: 'w-5 h-5 rounded-md',
  }
  return map[props.size] || map.md
})

const iconSizeClasses = computed(() => {
  const map = {
    sm: 'w-2.5 h-2.5 stroke-[3]',
    md: 'w-3 h-3 stroke-[3]',
    lg: 'w-3.5 h-3.5 stroke-[3]',
  }
  return map[props.size] || map.md
})
</script>

<template>
  <!-- Group Mode -->
  <div
    v-if="isGroupMode"
    role="group"
    class="flex"
    :class="direction === 'horizontal' ? 'flex-row flex-wrap items-center gap-4' : 'flex-col gap-2.5'"
  >
    <label
      v-for="opt in normalizedGroupOptions"
      :key="String(opt.value)"
      class="inline-flex items-start gap-2.5 select-none group"
      :class="opt.disabled ? 'opacity-40 cursor-not-allowed' : 'cursor-pointer'"
      @click.prevent="toggleGroupOption(opt)"
    >
      <button
        type="button"
        role="checkbox"
        :aria-checked="isOptionChecked(opt.value)"
        :disabled="opt.disabled"
        @keydown.space.prevent.stop="toggleGroupOption(opt)"
        @keydown.enter.prevent.stop="toggleGroupOption(opt)"
        class="relative shrink-0 inline-flex items-center justify-center border transition-all duration-150 focus:outline-none focus-visible:ring-2 focus-visible:ring-[var(--theme-primary)] focus-visible:ring-offset-1 dark:focus-visible:ring-offset-[var(--theme-surface)] shadow-2xs mt-0.5"
        :class="[
          boxSizeClasses,
          isOptionChecked(opt.value)
            ? 'bg-[var(--theme-primary)] border-[var(--theme-primary)] text-[var(--theme-primary-text)]'
            : 'bg-white dark:bg-[var(--theme-surface-raised)] border-neutral-300 dark:border-[var(--theme-border-strong)] group-hover:border-neutral-400 dark:group-hover:border-neutral-500',
          opt.disabled ? 'cursor-not-allowed' : 'cursor-pointer active:scale-95',
        ]"
      >
        <Check
          v-if="isOptionChecked(opt.value)"
          :class="iconSizeClasses"
          class="transition-transform duration-150 scale-100"
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

  <!-- Single Checkbox Mode -->
  <label
    v-else
    class="inline-flex items-start gap-2.5 select-none group"
    :class="disabled ? 'opacity-40 cursor-not-allowed' : 'cursor-pointer'"
    @click.prevent="toggle"
  >
    <!-- Custom Styled Checkbox Button -->
    <button
      type="button"
      role="checkbox"
      :aria-checked="indeterminate ? 'mixed' : isChecked"
      :disabled="disabled"
      @keydown.space.prevent.stop="toggle"
      @keydown.enter.prevent.stop="toggle"
      class="relative shrink-0 inline-flex items-center justify-center border transition-all duration-150 focus:outline-none focus-visible:ring-2 focus-visible:ring-[var(--theme-primary)] focus-visible:ring-offset-1 dark:focus-visible:ring-offset-[var(--theme-surface)] shadow-2xs"
      :class="[
        boxSizeClasses,
        label || description ? 'mt-0.5' : '',
        isChecked || indeterminate
          ? 'bg-[var(--theme-primary)] border-[var(--theme-primary)] text-[var(--theme-primary-text)]'
          : 'bg-white dark:bg-[var(--theme-surface-raised)] border-neutral-300 dark:border-[var(--theme-border-strong)] group-hover:border-neutral-400 dark:group-hover:border-neutral-500',
        disabled ? 'cursor-not-allowed' : 'cursor-pointer active:scale-95',
      ]"
    >
      <Minus
        v-if="indeterminate"
        :class="iconSizeClasses"
        class="transition-transform duration-150 scale-100"
      />
      <Check
        v-else-if="isChecked"
        :class="iconSizeClasses"
        class="transition-transform duration-150 scale-100"
      />
    </button>

    <!-- Optional Label & Description -->
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
