<script setup>
import { computed } from 'vue'
import { Minus, Plus, AlertCircle } from 'lucide-vue-next'

const props = defineProps({
  modelValue: {
    type: Number,
    default: 0,
  },
  label: {
    type: String,
    default: '',
  },
  required: {
    type: Boolean,
    default: false,
  },
  min: {
    type: Number,
    default: -Infinity,
  },
  max: {
    type: Number,
    default: Infinity,
  },
  step: {
    type: Number,
    default: 1,
  },
  prefix: {
    type: String,
    default: '',
  },
  suffix: {
    type: String,
    default: '',
  },
  disabled: {
    type: Boolean,
    default: false,
  },
  readonly: {
    type: Boolean,
    default: false,
  },
  error: {
    type: String,
    default: '',
  },
  hint: {
    type: String,
    default: '',
  },
  size: {
    type: String,
    default: 'md', // 'sm' | 'md' | 'lg'
  },
  controls: {
    type: Boolean,
    default: true,
  },
})

const emit = defineEmits(['update:modelValue', 'change', 'focus', 'blur'])

const clamp = (val) => {
  const num = Number.isFinite(val) ? val : 0
  return Math.min(props.max, Math.max(props.min, num))
}

const canDecrement = computed(() => !props.disabled && !props.readonly && (props.modelValue ?? 0) > props.min)
const canIncrement = computed(() => !props.disabled && !props.readonly && (props.modelValue ?? 0) < props.max)

const decrement = () => {
  if (!canDecrement.value) return
  const next = clamp(Number(((props.modelValue ?? 0) - props.step).toFixed(6)))
  emit('update:modelValue', next)
  emit('change', next)
}

const increment = () => {
  if (!canIncrement.value) return
  const next = clamp(Number(((props.modelValue ?? 0) + props.step).toFixed(6)))
  emit('update:modelValue', next)
  emit('change', next)
}

const handleInput = (e) => {
  const raw = e.target.value
  if (raw === '' || raw === '-') return
  const parsed = Number(raw)
  if (!Number.isNaN(parsed)) {
    const next = clamp(parsed)
    emit('update:modelValue', next)
  }
}

const handleBlur = (e) => {
  const parsed = Number(e.target.value)
  const next = Number.isNaN(parsed) ? clamp(0) : clamp(parsed)
  emit('update:modelValue', next)
  emit('change', next)
  emit('blur', e)
}

const sizeClasses = computed(() => {
  const map = {
    sm: 'h-9 text-xs px-2.5 rounded-xl',
    md: 'h-10 text-xs px-3 rounded-xl',
    lg: 'h-11 text-sm px-3.5 rounded-xl',
  }
  return map[props.size] || map.md
})
</script>

<template>
  <div class="w-full">
    <label
      v-if="label"
      class="block text-xs font-semibold text-neutral-700 dark:text-neutral-300 mb-1"
    >
      {{ label }}
      <span v-if="required" class="text-rose-500 ml-0.5">*</span>
    </label>

    <div
      class="flex items-center border transition-all duration-150 shadow-2xs overflow-hidden"
      :class="[
        sizeClasses,
        disabled ? 'opacity-50 cursor-not-allowed bg-neutral-100 dark:bg-[var(--theme-layer-2)]' : 'bg-white dark:bg-[var(--theme-surface-raised)]',
        error
          ? 'border-rose-400 dark:border-rose-500/80 focus-within:ring-2 focus-within:ring-rose-400/40'
          : 'border-neutral-200 dark:border-[var(--theme-border)] hover:border-neutral-300 dark:hover:border-[var(--theme-border-strong)] focus-within:border-[var(--theme-primary)] focus-within:ring-2 focus-within:ring-[var(--theme-primary)]/20',
      ]"
    >
      <!-- Decrement Button -->
      <button
        v-if="controls"
        type="button"
        :disabled="!canDecrement"
        @click="decrement"
        class="p-1.5 -ml-1 rounded-lg text-neutral-500 hover:text-neutral-900 dark:hover:text-white hover:bg-neutral-100 dark:hover:bg-[var(--theme-surface-muted)] disabled:opacity-30 disabled:cursor-not-allowed transition cursor-pointer shrink-0"
        title="Kurangi"
      >
        <Minus class="w-3.5 h-3.5" />
      </button>

      <span
        v-if="prefix"
        class="text-xs font-semibold text-neutral-400 dark:text-neutral-500 pl-1.5 select-none shrink-0"
      >
        {{ prefix }}
      </span>

      <input
        type="number"
        :value="modelValue"
        :min="Number.isFinite(min) ? min : undefined"
        :max="Number.isFinite(max) ? max : undefined"
        :step="step"
        :disabled="disabled"
        :readonly="readonly"
        @input="handleInput"
        @focus="emit('focus', $event)"
        @blur="handleBlur"
        class="w-full bg-transparent text-center font-mono font-bold text-neutral-900 dark:text-white focus:outline-none [appearance:textfield] [&::-webkit-outer-spin-button]:appearance-none [&::-webkit-inner-spin-button]:appearance-none px-1.5"
      />

      <span
        v-if="suffix"
        class="text-[11px] font-medium text-neutral-500 dark:text-neutral-400 pr-1.5 select-none shrink-0"
      >
        {{ suffix }}
      </span>

      <!-- Increment Button -->
      <button
        v-if="controls"
        type="button"
        :disabled="!canIncrement"
        @click="increment"
        class="p-1.5 -mr-1 rounded-lg text-neutral-500 hover:text-neutral-900 dark:hover:text-white hover:bg-neutral-100 dark:hover:bg-[var(--theme-surface-muted)] disabled:opacity-30 disabled:cursor-not-allowed transition cursor-pointer shrink-0"
        title="Tambah"
      >
        <Plus class="w-3.5 h-3.5" />
      </button>
    </div>

    <p
      v-if="error"
      class="mt-1 flex items-center gap-1 text-[11px] font-medium text-rose-600 dark:text-rose-400"
    >
      <AlertCircle class="w-3 h-3 shrink-0" />
      <span>{{ error }}</span>
    </p>
    <p
      v-else-if="hint"
      class="mt-1 text-[11px] text-neutral-500 dark:text-neutral-400"
    >
      {{ hint }}
    </p>
  </div>
</template>

