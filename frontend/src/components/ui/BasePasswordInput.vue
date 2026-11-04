<script setup>
import { ref, computed } from 'vue'
import { Lock, Eye, EyeOff, Check, AlertCircle } from 'lucide-vue-next'

const props = defineProps({
  modelValue: {
    type: String,
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
    default: 'Masukkan kata sandi...',
  },
  showStrength: {
    type: Boolean,
    default: false,
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

const emit = defineEmits(['update:modelValue', 'focus', 'blur', 'enter'])

const isVisible = ref(false)

const criteria = computed(() => {
  const val = props.modelValue || ''
  return [
    { key: 'length', label: 'Min. 8 karakter', met: val.length >= 8 },
    { key: 'upper', label: 'Huruf besar (A-Z)', met: /[A-Z]/.test(val) },
    { key: 'number', label: 'Angka (0-9)', met: /\d/.test(val) },
    { key: 'symbol', label: 'Simbol (!@#$)', met: /[^A-Za-z0-9]/.test(val) },
  ]
})

const strengthScore = computed(() => {
  if (!props.modelValue) return 0
  return criteria.value.filter((c) => c.met).length
})

const strengthInfo = computed(() => {
  const score = strengthScore.value
  const map = {
    0: { label: 'Belum diisi', color: 'bg-neutral-200 dark:bg-[var(--theme-surface-muted)]', text: 'text-neutral-400' },
    1: { label: 'Lemah', color: 'bg-rose-500', text: 'text-rose-600 dark:text-rose-400' },
    2: { label: 'Cukup', color: 'bg-amber-500', text: 'text-amber-600 dark:text-amber-400' },
    3: { label: 'Kuat', color: 'bg-sky-500', text: 'text-sky-600 dark:text-sky-400' },
    4: { label: 'Sangat Kuat', color: 'bg-emerald-500', text: 'text-emerald-600 dark:text-emerald-400' },
  }
  return map[score] || map[0]
})

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
      <Lock class="w-3.5 h-3.5 text-neutral-400 shrink-0" />

      <input
        :type="isVisible ? 'text' : 'password'"
        :value="modelValue"
        :placeholder="placeholder"
        :disabled="disabled"
        @input="emit('update:modelValue', $event.target.value)"
        @focus="emit('focus', $event)"
        @blur="emit('blur', $event)"
        @keydown.enter="emit('enter', $event)"
        class="w-full bg-transparent text-neutral-900 dark:text-white placeholder:text-neutral-400 dark:placeholder:text-neutral-500 focus:outline-none"
      />

      <button
        type="button"
        :disabled="disabled"
        @click="isVisible = !isVisible"
        class="p-1 rounded-lg text-neutral-400 hover:text-neutral-700 dark:hover:text-neutral-200 transition cursor-pointer shrink-0"
        :title="isVisible ? 'Sembunyikan kata sandi' : 'Tampilkan kata sandi'"
      >
        <EyeOff v-if="isVisible" class="w-3.5 h-3.5" />
        <Eye v-else class="w-3.5 h-3.5" />
      </button>
    </div>

    <!-- Strength Meter -->
    <div v-if="showStrength" class="mt-2 space-y-1.5">
      <div class="flex items-center justify-between text-[11px]">
        <span class="text-neutral-500 dark:text-neutral-400">Kekuatan Kata Sandi:</span>
        <span class="font-bold" :class="strengthInfo.text">{{ strengthInfo.label }}</span>
      </div>
      <div class="grid grid-cols-4 gap-1.5">
        <div
          v-for="bar in 4"
          :key="bar"
          class="h-1.5 rounded-full transition-colors duration-200"
          :class="
            bar <= strengthScore
              ? strengthInfo.color
              : 'bg-neutral-200 dark:bg-[var(--theme-surface-muted)]'
          "
        />
      </div>
      <div class="grid grid-cols-2 gap-1 pt-0.5">
        <div
          v-for="item in criteria"
          :key="item.key"
          class="flex items-center gap-1.5 text-[10.5px]"
          :class="item.met ? 'text-emerald-600 dark:text-emerald-400 font-semibold' : 'text-neutral-400 dark:text-neutral-500'"
        >
          <Check class="w-3 h-3 shrink-0" />
          <span>{{ item.label }}</span>
        </div>
      </div>
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

