<script setup>
import { ref, computed, onMounted, onUnmounted } from 'vue'
import { Clock, X, ChevronDown, AlertCircle } from 'lucide-vue-next'

const props = defineProps({
  modelValue: {
    type: [String, Object],
    default: '',
    // String 'HH:mm' or { start: 'HH:mm', end: 'HH:mm' } when range=true
  },
  range: {
    type: Boolean,
    default: false,
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
    default: 'Pilih jam...',
  },
  minuteStep: {
    type: Number,
    default: 5,
  },
  presets: {
    type: Array,
    default: () => ['08:00', '09:00', '12:00', '17:00'],
  },
  clearable: {
    type: Boolean,
    default: true,
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
    default: 'md', // 'sm' | 'md'
  },
  placement: {
    type: String,
    default: 'auto', // 'auto' | 'left' | 'right'
  },
})

const emit = defineEmits(['update:modelValue', 'change', 'clear'])

const isOpen = ref(false)
const containerRef = ref(null)
const activeRangeSide = ref('start') // 'start' | 'end'

const hours = computed(() =>
  Array.from({ length: 24 }, (_, i) => String(i).padStart(2, '0'))
)

const minutes = computed(() => {
  const step = Math.max(1, props.minuteStep)
  const list = []
  for (let m = 0; m < 60; m += step) {
    list.push(String(m).padStart(2, '0'))
  }
  return list
})

const parsedSingle = computed(() => {
  if (typeof props.modelValue === 'string' && /^\d{2}:\d{2}$/.test(props.modelValue)) {
    const [h, m] = props.modelValue.split(':')
    return { h, m }
  }
  return { h: '08', m: '00' }
})

const parsedRange = computed(() => {
  const obj = typeof props.modelValue === 'object' && props.modelValue !== null
    ? props.modelValue
    : { start: '08:00', end: '17:00' }
  const start = /^\d{2}:\d{2}$/.test(obj.start || '') ? obj.start : '08:00'
  const end = /^\d{2}:\d{2}$/.test(obj.end || '') ? obj.end : '17:00'
  return { start, end }
})

const currentActiveTime = computed(() => {
  if (!props.range) return parsedSingle.value
  const target = activeRangeSide.value === 'start' ? parsedRange.value.start : parsedRange.value.end
  const [h, m] = target.split(':')
  return { h, m }
})

const hasValue = computed(() => {
  if (props.range) {
    return Boolean(props.modelValue && (props.modelValue.start || props.modelValue.end))
  }
  return Boolean(props.modelValue)
})

const displayLabel = computed(() => {
  if (!hasValue.value) return ''
  if (props.range) {
    return `${parsedRange.value.start} – ${parsedRange.value.end}`
  }
  return String(props.modelValue)
})

const setTime = (h, m) => {
  const formatted = `${h}:${m}`
  if (props.range) {
    const next = {
      start: activeRangeSide.value === 'start' ? formatted : parsedRange.value.start,
      end: activeRangeSide.value === 'end' ? formatted : parsedRange.value.end,
    }
    emit('update:modelValue', next)
    emit('change', next)
  } else {
    emit('update:modelValue', formatted)
    emit('change', formatted)
  }
}

const selectHour = (h) => {
  setTime(h, currentActiveTime.value.m)
}

const selectMinute = (m) => {
  setTime(currentActiveTime.value.h, m)
}

const applyPreset = (preset) => {
  if (typeof preset === 'string' && /^\d{2}:\d{2}$/.test(preset)) {
    const [h, m] = preset.split(':')
    setTime(h, m)
    if (!props.range) {
      isOpen.value = false
    }
  }
}

const handleClear = (e) => {
  e?.stopPropagation?.()
  if (props.disabled) return
  const empty = props.range ? { start: '', end: '' } : ''
  emit('update:modelValue', empty)
  emit('clear')
  emit('change', empty)
}

const handleClickOutside = (e) => {
  if (containerRef.value && !containerRef.value.contains(e.target)) {
    isOpen.value = false
  }
}

onMounted(() => {
  document.addEventListener('click', handleClickOutside)
})

onUnmounted(() => {
  document.removeEventListener('click', handleClickOutside)
})

const sizeClasses = computed(() => {
  return props.size === 'sm'
    ? 'h-9 text-xs px-2.5 rounded-xl gap-1.5'
    : 'h-10 text-xs px-3 rounded-xl gap-2'
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

    <div ref="containerRef" class="relative inline-block w-full select-none">
      <button
        type="button"
        :disabled="disabled"
        @click="!disabled && (isOpen = !isOpen)"
        class="w-full flex items-center justify-between border transition-all duration-150 cursor-pointer shadow-2xs text-left outline-none focus-visible:ring-2 disabled:opacity-50 disabled:cursor-not-allowed"
        :class="[
          sizeClasses,
          error
            ? 'border-rose-400 dark:border-rose-500/80 bg-rose-50/30 dark:bg-rose-950/10'
            : isOpen
            ? 'bg-neutral-50/90 dark:bg-[var(--theme-surface-raised)] border-neutral-300 dark:border-[var(--theme-border-strong)] ring-2 ring-neutral-300/60 dark:ring-[var(--theme-border-strong)]'
            : 'bg-white hover:bg-neutral-50/70 dark:bg-[var(--theme-surface-raised)] dark:hover:bg-[var(--theme-surface-muted)] border-neutral-200 dark:border-[var(--theme-border)] hover:border-neutral-300 dark:hover:border-[var(--theme-border-strong)]',
        ]"
      >
        <div class="flex items-center gap-2 min-w-0 flex-1">
          <Clock class="w-3.5 h-3.5 text-neutral-400 shrink-0" />
          <span
            v-if="hasValue"
            class="truncate font-mono font-bold text-neutral-900 dark:text-white"
          >
            {{ displayLabel }}
          </span>
          <span v-else class="truncate text-neutral-400 dark:text-neutral-500">
            {{ placeholder }}
          </span>
        </div>

        <div class="flex items-center gap-1 shrink-0 ml-1">
          <span
            v-if="clearable && hasValue"
            role="button"
            tabindex="-1"
            @click="handleClear"
            class="p-0.5 rounded-md text-neutral-400 hover:text-neutral-700 dark:hover:text-neutral-200 hover:bg-neutral-200/60 dark:hover:bg-[var(--theme-surface-muted)] transition cursor-pointer"
            title="Hapus jam"
          >
            <X class="w-3 h-3" />
          </span>
          <ChevronDown
            class="w-3.5 h-3.5 text-neutral-400 transition-transform duration-200"
            :class="isOpen ? 'rotate-180 text-neutral-800 dark:text-white' : ''"
          />
        </div>
      </button>

      <!-- Floating Time Picker Popover -->
      <Transition
        enter-active-class="transition duration-150 ease-out"
        enter-from-class="opacity-0 scale-95 -translate-y-1"
        enter-to-class="opacity-100 scale-100 translate-y-0"
        leave-active-class="transition duration-100 ease-in"
        leave-from-class="opacity-100 scale-100 translate-y-0"
        leave-to-class="opacity-0 scale-95 -translate-y-1"
      >
        <div
          v-if="isOpen"
          class="absolute top-full mt-1.5 z-50 w-64 rounded-2xl bg-white/95 dark:bg-[var(--theme-surface-raised)]/95 backdrop-blur-md border border-neutral-200/90 dark:border-[var(--theme-border)] shadow-xl p-2.5 space-y-2.5"
          :class="placement === 'right' ? 'right-0' : 'left-0'"
        >
          <!-- Range Switcher -->
          <div
            v-if="range"
            class="grid grid-cols-2 gap-1 p-1 rounded-xl bg-neutral-100 dark:bg-[var(--theme-layer-2)] text-[11px] font-semibold"
          >
            <button
              type="button"
              @click="activeRangeSide = 'start'"
              class="py-1 rounded-lg transition cursor-pointer"
              :class="
                activeRangeSide === 'start'
                  ? 'bg-white dark:bg-[var(--theme-surface-raised)] text-neutral-900 dark:text-white shadow-2xs'
                  : 'text-neutral-500 hover:text-neutral-800 dark:hover:text-neutral-200'
              "
            >
              Mulai ({{ parsedRange.start }})
            </button>
            <button
              type="button"
              @click="activeRangeSide = 'end'"
              class="py-1 rounded-lg transition cursor-pointer"
              :class="
                activeRangeSide === 'end'
                  ? 'bg-white dark:bg-[var(--theme-surface-raised)] text-neutral-900 dark:text-white shadow-2xs'
                  : 'text-neutral-500 hover:text-neutral-800 dark:hover:text-neutral-200'
              "
            >
              Selesai ({{ parsedRange.end }})
            </button>
          </div>

          <!-- Presets -->
          <div v-if="presets && presets.length > 0" class="flex flex-wrap gap-1">
            <button
              v-for="p in presets"
              :key="p"
              type="button"
              @click="applyPreset(p)"
              class="px-2 py-1 rounded-lg text-[11px] font-mono font-semibold bg-neutral-100 hover:bg-neutral-200/70 dark:bg-[var(--theme-layer-2)] dark:hover:bg-[var(--theme-surface-muted)] text-neutral-700 dark:text-neutral-200 transition cursor-pointer"
            >
              {{ p }}
            </button>
          </div>

          <!-- Dual Columns: Jam & Menit -->
          <div class="grid grid-cols-2 gap-2 pt-1 border-t border-neutral-200/70 dark:border-[var(--theme-border)]">
            <div>
              <p class="text-[10px] font-bold uppercase tracking-wider text-neutral-400 text-center mb-1">Jam</p>
              <div class="max-h-40 overflow-y-auto space-y-0.5 pr-0.5 custom-scroll">
                <button
                  v-for="h in hours"
                  :key="h"
                  type="button"
                  @click="selectHour(h)"
                  class="w-full py-1 rounded-lg text-xs font-mono font-semibold transition cursor-pointer"
                  :class="
                    currentActiveTime.h === h
                      ? 'bg-[var(--theme-primary)] text-[var(--theme-primary-text)]'
                      : 'text-neutral-700 dark:text-neutral-300 hover:bg-neutral-100 dark:hover:bg-[var(--theme-surface-muted)]'
                  "
                >
                  {{ h }}
                </button>
              </div>
            </div>

            <div>
              <p class="text-[10px] font-bold uppercase tracking-wider text-neutral-400 text-center mb-1">Menit</p>
              <div class="max-h-40 overflow-y-auto space-y-0.5 pr-0.5 custom-scroll">
                <button
                  v-for="m in minutes"
                  :key="m"
                  type="button"
                  @click="selectMinute(m)"
                  class="w-full py-1 rounded-lg text-xs font-mono font-semibold transition cursor-pointer"
                  :class="
                    currentActiveTime.m === m
                      ? 'bg-[var(--theme-primary)] text-[var(--theme-primary-text)]'
                      : 'text-neutral-700 dark:text-neutral-300 hover:bg-neutral-100 dark:hover:bg-[var(--theme-surface-muted)]'
                  "
                >
                  {{ m }}
                </button>
              </div>
            </div>
          </div>
        </div>
      </Transition>
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

<style scoped>
.custom-scroll::-webkit-scrollbar {
  width: 4px;
}
.custom-scroll::-webkit-scrollbar-thumb {
  background: rgba(160, 160, 160, 0.25);
  border-radius: 9999px;
}
</style>

