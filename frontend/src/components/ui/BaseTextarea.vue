<script setup>
import { ref, computed, watch, nextTick, onMounted } from 'vue'

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
    default: '',
  },
  rows: {
    type: Number,
    default: 3,
  },
  maxlength: {
    type: Number,
    default: null,
  },
  autoExpand: {
    type: Boolean,
    default: true,
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
})

const emit = defineEmits(['update:modelValue', 'focus', 'blur'])

const textareaRef = ref(null)
const isFocused = ref(false)

const charCount = computed(() => {
  return (props.modelValue || '').length
})

const resizeTextarea = () => {
  if (!props.autoExpand || !textareaRef.value) return
  textareaRef.value.style.height = 'auto'
  textareaRef.value.style.height = `${Math.max(textareaRef.value.scrollHeight, 72)}px`
}

const handleInput = (e) => {
  emit('update:modelValue', e.target.value)
  if (props.autoExpand) {
    nextTick(resizeTextarea)
  }
}

watch(
  () => props.modelValue,
  () => {
    if (props.autoExpand) {
      nextTick(resizeTextarea)
    }
  }
)

onMounted(() => {
  if (props.autoExpand) {
    nextTick(resizeTextarea)
  }
})
</script>

<template>
  <div class="w-full space-y-1 text-left">
    <!-- Label & Character Counter Header -->
    <div class="flex items-center justify-between select-none">
      <label
        v-if="label"
        class="block text-xs font-semibold text-neutral-700 dark:text-neutral-300"
      >
        {{ label }}
        <span v-if="required" class="text-rose-500 font-bold ml-0.5">*</span>
      </label>

      <!-- Counter Top Right if Maxlength given -->
      <span
        v-if="maxlength"
        class="text-[10px] font-mono text-neutral-400 dark:text-neutral-500"
        :class="charCount >= maxlength ? 'text-rose-500 font-bold' : ''"
      >
        {{ charCount }} / {{ maxlength }}
      </span>
    </div>

    <!-- Textarea Card Container -->
    <div
      class="relative w-full rounded-xl border transition-all duration-150 shadow-2xs overflow-hidden"
      :class="[
        error
          ? 'border-rose-500 ring-2 ring-rose-500/20 bg-rose-50/10'
          : isFocused
          ? 'border-neutral-400 dark:border-[var(--theme-border-strong)] ring-2 ring-neutral-300/60 dark:ring-[var(--theme-border-strong)] bg-white dark:bg-[var(--theme-surface-raised)]'
          : 'border-neutral-200 dark:border-[var(--theme-border)] hover:border-neutral-300 dark:hover:border-[var(--theme-border-strong)] bg-white dark:bg-[var(--theme-surface-raised)]',
        disabled ? 'opacity-50 cursor-not-allowed bg-neutral-100 dark:bg-[var(--theme-surface)]' : '',
      ]"
    >
      <textarea
        ref="textareaRef"
        :value="modelValue"
        :placeholder="placeholder"
        :rows="rows"
        :maxlength="maxlength"
        :disabled="disabled"
        :readonly="readonly"
        @input="handleInput"
        @focus="isFocused = true; emit('focus', $event)"
        @blur="isFocused = false; emit('blur', $event)"
        class="w-full bg-transparent px-3 py-2 text-xs text-neutral-900 dark:text-white placeholder:text-neutral-400 dark:placeholder:text-neutral-500 focus:outline-none resize-none disabled:cursor-not-allowed"
      />
    </div>

    <!-- Error Message -->
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

    <!-- Hint -->
    <p v-if="hint && !error" class="text-[11px] text-neutral-500 dark:text-neutral-400 pl-0.5">
      {{ hint }}
    </p>
  </div>
</template>

