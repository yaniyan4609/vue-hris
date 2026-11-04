<script setup>
import { computed } from 'vue'

const props = defineProps({
  modelValue: {
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

const toggle = () => {
  if (props.disabled) return
  const newVal = !props.modelValue
  emit('update:modelValue', newVal)
  emit('change', newVal)
}

const trackSize = computed(() => {
  const map = {
    sm: 'w-8 h-4.5 p-0.5',
    md: 'w-10 h-5.5 p-0.5',
    lg: 'w-12 h-6.5 p-0.5',
  }
  return map[props.size] || map.md
})

const thumbSize = computed(() => {
  const map = {
    sm: 'w-3.5 h-3.5',
    md: 'w-4.5 h-4.5',
    lg: 'w-5.5 h-5.5',
  }
  return map[props.size] || map.md
})

const thumbTranslate = computed(() => {
  if (!props.modelValue) return 'translate-x-0'
  const map = {
    sm: 'translate-x-3.5',
    md: 'translate-x-4.5',
    lg: 'translate-x-5.5',
  }
  return map[props.size] || 'translate-x-4.5'
})
</script>

<template>
  <label
    class="inline-flex items-center justify-between gap-3 select-none cursor-pointer group"
    :class="disabled ? 'opacity-40 cursor-not-allowed' : ''"
  >
    <!-- Left Label and Description -->
    <div v-if="label || description" class="min-w-0 flex-1">
      <span class="block text-xs font-semibold text-neutral-800 dark:text-neutral-200 group-hover:text-neutral-900 dark:group-hover:text-white transition-colors">
        {{ label }}
      </span>
      <span v-if="description" class="block text-[11px] text-neutral-500 dark:text-neutral-400 mt-0.5">
        {{ description }}
      </span>
    </div>

    <!-- Apple-Style Smooth Gliding Toggle Switch -->
    <button
      type="button"
      role="switch"
      :aria-checked="modelValue"
      :disabled="disabled"
      @click="toggle"
      @keydown.space.prevent="toggle"
      @keydown.enter.prevent="toggle"
      class="relative shrink-0 rounded-full transition-colors duration-200 focus:outline-none focus-visible:ring-2 focus-visible:ring-neutral-400 cursor-pointer shadow-inner"
      :class="[
        trackSize,
        modelValue
          ? 'bg-[var(--theme-primary)]'
          : 'bg-neutral-200 dark:bg-[var(--theme-surface-muted)] hover:bg-neutral-300 dark:hover:bg-[var(--theme-surface-muted)]',
        disabled ? 'cursor-not-allowed' : '',
      ]"
    >
      <!-- Circular Gliding Thumb -->
      <span
        class="block rounded-full shadow-md transition-transform duration-200 ease-out"
        :class="[
          thumbSize,
          thumbTranslate,
          modelValue ? 'bg-[var(--theme-primary-text)]' : 'bg-white dark:bg-neutral-300',
        ]"
      />
    </button>
  </label>
</template>

