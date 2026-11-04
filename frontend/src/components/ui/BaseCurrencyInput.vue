<script setup>
import { ref, computed, watch, nextTick } from 'vue'
import { X, DollarSign } from 'lucide-vue-next'

const props = defineProps({
  modelValue: {
    type: [Number, String],
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
  placeholder: {
    type: String,
    default: '0',
  },
  prefix: {
    type: String,
    default: 'Rp',
  },
  suffix: {
    type: String,
    default: '', // e.g. '/ bln', '/ hari', 'IDR'
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
  clearable: {
    type: Boolean,
    default: true,
  },
  size: {
    type: String,
    default: 'md', // 'sm' | 'md' | 'lg'
  },
  leadingIcon: {
    type: [Object, Function],
    default: null,
  },
  terbilang: {
    type: Boolean,
    default: false, // Jika true, tampilkan teks terbilang: "Satu juta rupiah"
  },
  emitFormatted: {
    type: Boolean,
    default: false, // Jika false (default), v-model menerima Number (misal: 1000). Jika true, menerima String '1.000'
  },
})

const emit = defineEmits(['update:modelValue', 'clear', 'focus', 'blur', 'enter'])

const inputRef = ref(null)
const isFocused = ref(false)
const displayValue = ref('')

// ========================================================
// FORMATTER & PARSER RUPIAH (INDONESIAN THOUSANDS WITH '.')
// Contoh: 1000 -> 1.000 | 15000000 -> 15.000.000
// ========================================================
const formatNumberWithDots = (val) => {
  if (val === null || val === undefined || val === '') return ''
  const digits = String(val).replace(/\D/g, '')
  if (!digits) return ''
  return digits.replace(/\B(?=(\d{3})+(?!\d))/g, '.')
}

const parseDigitsToNumber = (str) => {
  if (!str) return 0
  const digits = String(str).replace(/\D/g, '')
  return digits ? parseInt(digits, 10) : 0
}

// Sinkronisasi saat modelValue berubah dari luar
watch(
  () => props.modelValue,
  (newVal) => {
    if (newVal === null || newVal === undefined || newVal === '') {
      displayValue.value = ''
      return
    }
    const formatted = formatNumberWithDots(newVal)
    if (formatted !== displayValue.value) {
      displayValue.value = formatted
    }
  },
  { immediate: true }
)

// Penanganan input dengan preservasi posisi kursor agar tidak loncat ke akhir
const handleInput = (e) => {
  const el = e.target
  const rawInput = el.value
  const oldCursor = el.selectionStart || 0

  // Hitung jumlah digit angka sebelum kursor saat ini
  const digitsBeforeCursor = (rawInput.slice(0, oldCursor).match(/\d/g) || []).length

  // Bersihkan dan format dengan pemisah titik
  const digitsOnly = rawInput.replace(/\D/g, '')
  const formatted = formatNumberWithDots(digitsOnly)
  displayValue.value = formatted

  // Emit ke v-model
  if (props.emitFormatted) {
    emit('update:modelValue', formatted)
  } else {
    const numericVal = digitsOnly ? parseInt(digitsOnly, 10) : 0
    emit('update:modelValue', numericVal)
  }

  // Rekalkulasi posisi kursor yang tepat
  nextTick(() => {
    if (!inputRef.value) return
    let newCursor = 0
    let digitCount = 0

    for (let i = 0; i < formatted.length; i++) {
      if (/\d/.test(formatted[i])) {
        digitCount++
      }
      if (digitCount === digitsBeforeCursor) {
        newCursor = i + 1
        break
      }
    }

    if (digitsBeforeCursor === 0) newCursor = 0
    if (newCursor > formatted.length) newCursor = formatted.length

    inputRef.value.setSelectionRange(newCursor, newCursor)
  })
}

const handleClear = () => {
  displayValue.value = ''
  emit('update:modelValue', props.emitFormatted ? '' : 0)
  emit('clear')
  nextTick(() => {
    inputRef.value?.focus()
  })
}

// ========================================================
// HELPER TERBILANG INDONESIA (MISAL: "Satu Juta Rupiah")
// ========================================================
const getTerbilang = (nominal) => {
  const num = typeof nominal === 'number' ? nominal : parseDigitsToNumber(nominal)
  if (!num || num <= 0) return ''

  const satuan = ['', 'Satu', 'Dua', 'Tiga', 'Empat', 'Lima', 'Enam', 'Tujuh', 'Delapan', 'Sembilan', 'Sepuluh', 'Sebelas']

  function konversi(n) {
    if (n < 12) return satuan[n]
    if (n < 20) return konversi(n - 10) + ' Belas'
    if (n < 100) return konversi(Math.floor(n / 10)) + ' Puluh ' + konversi(n % 10)
    if (n < 200) return 'Seratus ' + konversi(n - 100)
    if (n < 1000) return konversi(Math.floor(n / 100)) + ' Ratus ' + konversi(n % 100)
    if (n < 2000) return 'Seribu ' + konversi(n - 1000)
    if (n < 1000000) return konversi(Math.floor(n / 1000)) + ' Ribu ' + konversi(n % 1000)
    if (n < 1000000000) return konversi(Math.floor(n / 1000000)) + ' Juta ' + konversi(n % 1000000)
    if (n < 1000000000000) return konversi(Math.floor(n / 1000000000)) + ' Miliar ' + konversi(n % 1000000000)
    return konversi(Math.floor(n / 1000000000000)) + ' Triliun ' + konversi(n % 1000000000000)
  }

  const hasil = konversi(num).replace(/\s+/g, ' ').trim()
  return hasil ? `${hasil} Rupiah` : ''
}

const terbilangText = computed(() => {
  if (!props.terbilang) return ''
  return getTerbilang(props.modelValue)
})

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
  <div class="w-full space-y-1 text-left select-none">
    <!-- Label with Required Asterisk -->
    <label
      v-if="label"
      class="block text-xs font-semibold text-neutral-700 dark:text-neutral-300 select-none"
    >
      {{ label }}
      <span v-if="required" class="text-rose-500 font-bold ml-0.5">*</span>
    </label>

    <!-- Currency Input Card Wrapper -->
    <div
      class="relative flex items-center w-full rounded-xl border transition-all duration-150 shadow-2xs group"
      :class="[
        error
          ? 'border-rose-500 dark:border-rose-500 ring-2 ring-rose-500/20 bg-rose-50/10'
          : isFocused
          ? 'border-[var(--theme-primary)] ring-2 ring-[var(--theme-primary)]/20 bg-white dark:bg-[var(--theme-surface-raised)]'
          : 'border-neutral-200 dark:border-[var(--theme-border)] hover:border-neutral-300 dark:hover:border-[var(--theme-border-strong)] bg-white dark:bg-[var(--theme-surface-raised)]',
        disabled ? 'opacity-50 cursor-not-allowed bg-neutral-100 dark:bg-[var(--theme-surface)]' : '',
      ]"
    >
      <!-- Leading Prefix (Badge Rp Otomatis) -->
      <div class="pl-3 pr-1.5 flex items-center pointer-events-none shrink-0 select-none">
        <component :is="leadingIcon" v-if="leadingIcon" class="w-4 h-4 text-neutral-400 mr-1.5" />
        <span
          v-if="prefix"
          class="text-xs font-bold font-mono px-1.5 py-0.5 rounded-md bg-neutral-100 dark:bg-[var(--theme-surface-muted)] text-neutral-700 dark:text-neutral-300 border border-neutral-200/80 dark:border-[var(--theme-border-strong)]"
        >
          {{ prefix }}
        </span>
      </div>

      <!-- Core Input Element -->
      <input
        ref="inputRef"
        type="text"
        inputmode="numeric"
        :value="displayValue"
        :placeholder="placeholder"
        :disabled="disabled"
        :readonly="readonly"
        @input="handleInput"
        @focus="isFocused = true; emit('focus', $event)"
        @blur="isFocused = false; emit('blur', $event)"
        @keydown.enter="emit('enter', $event)"
        class="w-full bg-transparent px-2.5 font-mono font-medium text-neutral-900 dark:text-white placeholder:text-neutral-400 dark:placeholder:text-neutral-500 focus:outline-none disabled:cursor-not-allowed"
        :class="[
          sizeClasses,
          (clearable || suffix) ? 'pr-1.5' : '',
        ]"
      />

      <!-- Right Trailing Controls: Clear & Suffix -->
      <div class="pr-2.5 flex items-center gap-1 shrink-0">
        <!-- Clear Button -->
        <button
          v-if="clearable && displayValue && !disabled && !readonly"
          type="button"
          @click="handleClear"
          class="p-1 rounded-md text-neutral-400 hover:text-neutral-700 dark:hover:text-neutral-200 hover:bg-neutral-100 dark:hover:bg-[var(--theme-surface-muted)] transition cursor-pointer"
          title="Bersihkan nominal"
        >
          <X class="w-3.5 h-3.5" />
        </button>

        <!-- Suffix Badge (e.g. '/ bln', 'IDR') -->
        <span
          v-if="suffix"
          class="text-[10px] font-bold px-1.5 py-0.5 rounded-md bg-neutral-100 dark:bg-[var(--theme-surface-muted)] text-neutral-600 dark:text-neutral-400 border border-neutral-200/60 dark:border-[var(--theme-border-strong)] select-none font-mono"
        >
          {{ suffix }}
        </span>
      </div>
    </div>

    <!-- Terbilang Indonesian Text (Opsional) -->
    <p
      v-if="terbilang && terbilangText"
      class="text-[11px] text-[var(--theme-primary)] dark:text-[var(--theme-primary)] font-medium pl-1 italic flex items-center gap-1"
    >
      <span>💬</span>
      <span>{{ terbilangText }}</span>
    </p>

    <!-- Error Message with Slide Animation -->
    <Transition
      enter-active-class="transition duration-150 ease-out"
      enter-from-class="opacity-0 -translate-y-1"
      enter-to-class="opacity-100 translate-y-0"
      leave-active-class="transition duration-100 ease-in"
      leave-from-class="opacity-100 translate-y-0"
      leave-to-class="opacity-0 -translate-y-1"
    >
      <p v-if="error" class="text-[11px] text-rose-500 font-medium pl-0.5">
        {{ error }}
      </p>
    </Transition>

    <!-- Helper / Hint Text -->
    <p v-if="hint && !error && (!terbilang || !terbilangText)" class="text-[11px] text-neutral-500 dark:text-neutral-400 pl-0.5">
      {{ hint }}
    </p>
  </div>
</template>

