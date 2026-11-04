<script setup>
import { computed } from 'vue'
import { AlertCircle } from 'lucide-vue-next'

const props = defineProps({
  modelValue: {
    type: String,
    default: '',
  },
  preset: {
    type: String,
    default: 'nik', // 'nik' | 'npwp' | 'phone' | 'rekening' | 'custom'
  },
  mask: {
    type: String,
    default: '',
    // '#' represents a digit, e.g. '#### #### #### ####'
  },
  emitRaw: {
    type: Boolean,
    default: true,
  },
  label: {
    type: String,
    default: '',
  },
  required: {
    type: Boolean,
    default: false,
  },
  placeholder: {
    type: String,
    default: '',
  },
  leadingIcon: {
    type: [Object, Function],
    default: null,
  },
  disabled: {
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
})

const emit = defineEmits(['update:modelValue', 'change', 'focus', 'blur'])

const resolvedMask = computed(() => {
  if (props.mask) return props.mask
  const map = {
    nik: '#### #### #### ####',
    npwp: '##.###.###.#-###.###',
    phone: '####-####-#####',
    rekening: '####-####-####-####',
  }
  return map[props.preset] || '#### #### #### ####'
})

const resolvedPlaceholder = computed(() => {
  if (props.placeholder) return props.placeholder
  const map = {
    nik: '3201 0123 4567 0001',
    npwp: '09.254.294.3-407.000',
    phone: '0812-3456-7890',
    rekening: '1234-5678-9012-3456',
  }
  return map[props.preset] || resolvedMask.value.replace(/#/g, '0')
})

const applyMask = (rawDigits) => {
  const digits = String(rawDigits || '').replace(/\D/g, '')
  const pattern = resolvedMask.value
  let result = ''
  let digitIdx = 0

  for (let i = 0; i < pattern.length && digitIdx < digits.length; i++) {
    if (pattern[i] === '#') {
      result += digits[digitIdx]
      digitIdx++
    } else {
      result += pattern[i]
    }
  }
  return result
}

const maxDigits = computed(() => {
  return (resolvedMask.value.match(/#/g) || []).length
})

const displayValue = computed(() => {
  return applyMask(props.modelValue)
})

const handleInput = (e) => {
  const digits = e.target.value.replace(/\D/g, '').slice(0, maxDigits.value)
  const masked = applyMask(digits)
  e.target.value = masked
  const out = props.emitRaw ? digits : masked
  emit('update:modelValue', out)
  emit('change', out)
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
      class="flex items-center gap-2 border transition-all duration-150 shadow-2xs"
      :class="[
        sizeClasses,
        disabled ? 'opacity-50 cursor-not-allowed bg-neutral-100 dark:bg-[var(--theme-layer-2)]' : 'bg-white dark:bg-[var(--theme-surface-raised)]',
        error
          ? 'border-rose-400 dark:border-rose-500/80 focus-within:ring-2 focus-within:ring-rose-400/40'
          : 'border-neutral-200 dark:border-[var(--theme-border)] hover:border-neutral-300 dark:hover:border-[var(--theme-border-strong)] focus-within:border-[var(--theme-primary)] focus-within:ring-2 focus-within:ring-[var(--theme-primary)]/20',
      ]"
    >
      <component
        :is="leadingIcon"
        v-if="leadingIcon"
        class="w-3.5 h-3.5 text-neutral-400 shrink-0"
      />

      <input
        type="text"
        inputmode="numeric"
        :value="displayValue"
        :placeholder="resolvedPlaceholder"
        :disabled="disabled"
        @input="handleInput"
        @focus="emit('focus', $event)"
        @blur="emit('blur', $event)"
        class="w-full bg-transparent font-mono font-semibold tracking-wide text-neutral-900 dark:text-white placeholder:text-neutral-400 dark:placeholder:text-neutral-500 focus:outline-none"
      />
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

