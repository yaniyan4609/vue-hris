<script setup>
import { ref, computed } from 'vue'
import { Eye, EyeOff, X } from 'lucide-vue-next'

const props = defineProps({
  modelValue: {
    type: [String, Number],
    default: '',
  },
  label: {
    type: String,
    default: '',
  },
  required: {
    type: Boolean,
    default: false,
  },
  type: {
    type: String,
    default: 'text',
  },
  placeholder: {
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
  leadingIcon: {
    type: [Object, Function],
    default: null,
  },
  trailingIcon: {
    type: [Object, Function],
    default: null,
  },
  prefix: {
    type: String,
    default: '',
  },
  suffix: {
    type: String,
    default: '', // e.g. 'mdpl', 'Rp', 'Kg', '%'
  },
  clearable: {
    type: Boolean,
    default: false,
  },
  size: {
    type: String,
    default: 'md', // 'sm' | 'md' | 'lg'
  },
  currency: {
    type: Boolean,
    default: false, // Auto-format Rupiah (e.g. 1000 -> 1.000)
  },
  inputClass: {
    type: String,
    default: '',
  },
})

const emit = defineEmits(['update:modelValue', 'clear', 'focus', 'blur', 'enter'])

const isPasswordVisible = ref(false)
const isFocused = ref(false)

const isCurrencyMode = computed(() => props.currency || props.type === 'currency')

const computedType = computed(() => {
  if (isCurrencyMode.value) return 'text'
  if (props.type === 'password') {
    return isPasswordVisible.value ? 'text' : 'password'
  }
  return props.type
})

const formatCurrencyString = (val) => {
  if (val === null || val === undefined || val === '') return ''
  const digits = String(val).replace(/\D/g, '')
  if (!digits) return ''
  return digits.replace(/\B(?=(\d{3})+(?!\d))/g, '.')
}

const displayValue = computed(() => {
  if (isCurrencyMode.value) {
    return formatCurrencyString(props.modelValue)
  }
  return props.modelValue
})

const handleInput = (e) => {
  if (isCurrencyMode.value) {
    const rawDigits = e.target.value.replace(/\D/g, '')
    const num = rawDigits ? parseInt(rawDigits, 10) : 0
    emit('update:modelValue', num)
    e.target.value = formatCurrencyString(rawDigits)
  } else {
    emit('update:modelValue', e.target.value)
  }
}

const handleClear = () => {
  emit('update:modelValue', '')
  emit('clear')
}

const sizeClasses = computed(() => {
  const map = {
    sm: 'h-8 text-xs py-1.5',
    md: 'h-9.5 text-xs py-2',
    lg: 'h-11 text-sm py-2.5',
  }
  return map[props.size] || map.md
})
</script>

<template>
  <div class="w-full space-y-1 text-left">
    <!-- Label with Required Asterisk -->
    <label
      v-if="label"
      class="block text-xs font-semibold text-neutral-700 dark:text-neutral-300 select-none"
    >
      {{ label }}
      <span v-if="required" class="text-rose-500 font-bold ml-0.5">*</span>
    </label>

    <!-- Input Wrapper Card -->
    <div
      class="relative flex items-center w-full rounded-xl border transition-all duration-150 shadow-2xs group"
      :class="[
        error
          ? 'border-rose-500 dark:border-rose-500 ring-2 ring-rose-500/20 dark:ring-rose-500/20 bg-rose-50/10'
          : isFocused
          ? 'border-neutral-400 dark:border-[var(--theme-border-strong)] ring-2 ring-neutral-300/60 dark:ring-[var(--theme-border-strong)] bg-white dark:bg-[var(--theme-surface-raised)]'
          : 'border-neutral-200 dark:border-[var(--theme-border)] hover:border-neutral-300 dark:hover:border-[var(--theme-border-strong)] bg-white dark:bg-[var(--theme-surface-raised)]',
        disabled ? 'opacity-50 cursor-not-allowed bg-neutral-100 dark:bg-[var(--theme-surface)]' : '',
      ]"
    >
      <!-- Leading Icon / Prefix -->
      <div v-if="leadingIcon || prefix" class="pl-3 pr-1 flex items-center pointer-events-none shrink-0 text-neutral-400">
        <component :is="leadingIcon" v-if="leadingIcon" class="w-4 h-4" />
        <span v-if="prefix" class="text-xs font-semibold text-neutral-500 select-none">{{ prefix }}</span>
      </div>

      <!-- Core HTML Input -->
      <input
        :type="computedType"
        :value="displayValue"
        :placeholder="placeholder"
        :disabled="disabled"
        :readonly="readonly"
        @input="handleInput"
        @focus="isFocused = true; emit('focus', $event)"
        @blur="isFocused = false; emit('blur', $event)"
        @keydown.enter="emit('enter', $event)"
        class="w-full bg-transparent px-3 text-neutral-900 dark:text-white placeholder:text-neutral-400 dark:placeholder:text-neutral-500 focus:outline-none disabled:cursor-not-allowed"
        :class="[
          sizeClasses,
          (leadingIcon || prefix) ? 'pl-1.5' : '',
          (clearable || type === 'password' || trailingIcon || suffix) ? 'pr-1.5' : '',
          inputClass,
        ]"
      />

      <!-- Right Actions: Clear, Eye Toggle, Trailing Icon, Suffix -->
      <div class="pr-2.5 flex items-center gap-1 shrink-0">
        <!-- Clear Button -->
        <button
          v-if="clearable && modelValue && !disabled && !readonly"
          type="button"
          @click="handleClear"
          class="p-1 rounded-md text-neutral-400 hover:text-neutral-700 dark:hover:text-neutral-200 hover:bg-neutral-100 dark:hover:bg-[var(--theme-surface-muted)] transition cursor-pointer"
          title="Bersihkan input"
        >
          <X class="w-3.5 h-3.5" />
        </button>

        <!-- Password Eye Toggle -->
        <button
          v-if="type === 'password' && !disabled"
          type="button"
          @click="isPasswordVisible = !isPasswordVisible"
          class="p-1 rounded-md text-neutral-400 hover:text-neutral-700 dark:hover:text-neutral-200 hover:bg-neutral-100 dark:hover:bg-[var(--theme-surface-muted)] transition cursor-pointer"
          :title="isPasswordVisible ? 'Sembunyikan password' : 'Lihat password'"
        >
          <EyeOff v-if="isPasswordVisible" class="w-3.5 h-3.5" />
          <Eye v-else class="w-3.5 h-3.5" />
        </button>

        <!-- Custom Trailing Icon -->
        <component
          :is="trailingIcon"
          v-if="trailingIcon"
          class="w-4 h-4 text-neutral-400 pointer-events-none"
        />

        <!-- Unit Suffix Badge (e.g. 'mdpl', 'Rp', 'Kg') -->
        <span
          v-if="suffix"
          class="text-[10px] font-bold px-1.5 py-0.5 rounded-md bg-neutral-100 dark:bg-[var(--theme-surface-muted)] text-neutral-600 dark:text-neutral-400 border border-neutral-200/60 dark:border-[var(--theme-border-strong)] select-none"
        >
          {{ suffix }}
        </span>
      </div>
    </div>

    <!-- Error Message with Soft Slide-in Transition -->
    <Transition
      enter-active-class="transition duration-150 ease-out"
      enter-from-class="opacity-0 -translate-y-1"
      enter-to-class="opacity-100 translate-y-0"
      leave-active-class="transition duration-100 ease-in"
      leave-from-class="opacity-100 translate-y-0"
      leave-to-class="opacity-0 -translate-y-1"
    >
      <p v-if="error" class="text-[11px] text-rose-500 font-medium pl-0.5 flex items-center gap-1">
        <span>{{ error }}</span>
      </p>
    </Transition>

    <!-- Helper / Hint Text -->
    <p v-if="hint && !error" class="text-[11px] text-neutral-500 dark:text-neutral-400 pl-0.5">
      {{ hint }}
    </p>
  </div>
</template>

