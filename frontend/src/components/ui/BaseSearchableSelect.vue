<script setup>
import { ref, computed, watch, nextTick, onMounted, onUnmounted } from 'vue'
import { Search, ChevronDown, Check, X } from 'lucide-vue-next'

const props = defineProps({
  modelValue: {
    type: [String, Number, Object, null],
    default: '',
  },
  options: {
    type: Array,
    default: () => [],
    // Each option can be a primitive (string/number)
    // or an object: { label: string, value: any, badge?: string|number, icon?: any, disabled?: boolean }
  },
  placeholder: {
    type: String,
    default: 'Pilih...',
  },
  searchPlaceholder: {
    type: String,
    default: 'Cari pilihan...',
  },
  searchable: {
    type: Boolean,
    default: true,
  },
  clearable: {
    type: Boolean,
    default: false,
  },
  clearValue: {
    type: [String, Number, Object, null],
    default: '',
  },
  disabled: {
    type: Boolean,
    default: false,
  },
  size: {
    type: String,
    default: 'sm', // 'sm' | 'md' | 'lg'
  },
  align: {
    type: String,
    default: 'left', // 'left' | 'right'
  },
  leadingIcon: {
    type: [Object, Function],
    default: null,
  },
  emptyText: {
    type: String,
    default: 'Tidak ada hasil ditemukan',
  },
  widthClass: {
    type: String,
    default: 'w-full',
  },
})

const emit = defineEmits(['update:modelValue', 'change', 'clear'])

const isOpen = ref(false)
const searchQuery = ref('')
const highlightedIndex = ref(0)
const searchInputRef = ref(null)
const containerRef = ref(null)
const listContainerRef = ref(null)

// Normalize options to uniform { label, value, badge, icon, disabled }
const normalizedOptions = computed(() => {
  return props.options.map((opt) => {
    if (typeof opt === 'object' && opt !== null) {
      return {
        label: opt.label ?? String(opt.value ?? ''),
        value: opt.value ?? opt.label,
        badge: opt.badge ?? null,
        icon: opt.icon ?? null,
        disabled: Boolean(opt.disabled),
      }
    }
    return {
      label: String(opt),
      value: opt,
      badge: null,
      icon: null,
      disabled: false,
    }
  })
})

// Current selected item
const selectedOption = computed(() => {
  return normalizedOptions.value.find((opt) => opt.value === props.modelValue) || null
})

// Filtered options based on search query
const filteredOptions = computed(() => {
  if (!props.searchable || !searchQuery.value.trim()) {
    return normalizedOptions.value
  }
  const q = searchQuery.value.toLowerCase().trim()
  return normalizedOptions.value.filter((opt) => {
    const labelMatch = opt.label.toLowerCase().includes(q)
    const valMatch = typeof opt.value === 'string' && opt.value.toLowerCase().includes(q)
    return labelMatch || valMatch
  })
})

// Reset highlighted index when filtered options change
watch(filteredOptions, (opts) => {
  highlightedIndex.value = opts.length > 0 ? 0 : -1
})

// Toggle dropdown open/closed
const toggleDropdown = () => {
  if (props.disabled) return
  isOpen.value = !isOpen.value
}

// Watch isOpen to focus search input and reset query
watch(isOpen, async (val) => {
  if (val) {
    searchQuery.value = ''
    highlightedIndex.value = filteredOptions.value.findIndex((o) => o.value === props.modelValue)
    if (highlightedIndex.value === -1) highlightedIndex.value = 0
    await nextTick()
    if (props.searchable) {
      searchInputRef.value?.focus()
    }
  }
})

// Select an option
const selectOption = (opt) => {
  if (opt.disabled) return
  emit('update:modelValue', opt.value)
  emit('change', opt.value)
  isOpen.value = false
}

// Clear selected option
const handleClear = (e) => {
  e.stopPropagation()
  emit('update:modelValue', props.clearValue)
  emit('clear')
  emit('change', props.clearValue)
}

// Close on outside click
const handleClickOutside = (e) => {
  if (containerRef.value && !containerRef.value.contains(e.target)) {
    isOpen.value = false
  }
}

// Keyboard navigation
const handleKeydown = (e) => {
  if (!isOpen.value) {
    if (containerRef.value && containerRef.value.contains(document.activeElement)) {
      if (['ArrowDown', 'ArrowUp', 'Enter', ' '].includes(e.key)) {
        e.preventDefault()
        isOpen.value = true
      }
    }
    return
  }

  if (e.key === 'ArrowDown') {
    e.preventDefault()
    if (filteredOptions.value.length === 0) return
    highlightedIndex.value = (highlightedIndex.value + 1) % filteredOptions.value.length
    scrollHighlightedIntoView()
  } else if (e.key === 'ArrowUp') {
    e.preventDefault()
    if (filteredOptions.value.length === 0) return
    highlightedIndex.value =
      (highlightedIndex.value - 1 + filteredOptions.value.length) % filteredOptions.value.length
    scrollHighlightedIntoView()
  } else if (e.key === 'Enter') {
    e.preventDefault()
    if (highlightedIndex.value >= 0 && highlightedIndex.value < filteredOptions.value.length) {
      selectOption(filteredOptions.value[highlightedIndex.value])
    }
  } else if (e.key === 'Escape') {
    e.preventDefault()
    // Cegah Esc ikut menutup modal/slide-over induk (listener mereka ada di window)
    e.stopPropagation()
    isOpen.value = false
  }
}

const scrollHighlightedIntoView = () => {
  nextTick(() => {
    if (!listContainerRef.value) return
    const el = listContainerRef.value.querySelector(`[data-index="${highlightedIndex.value}"]`)
    if (el) {
      el.scrollIntoView({ block: 'nearest' })
    }
  })
}

onMounted(() => {
  document.addEventListener('click', handleClickOutside)
  document.addEventListener('keydown', handleKeydown)
})

onUnmounted(() => {
  document.removeEventListener('click', handleClickOutside)
  document.removeEventListener('keydown', handleKeydown)
})

// Size classes
const sizeClasses = computed(() => {
  const map = {
    sm: 'h-9 text-xs px-3 rounded-xl gap-2',
    md: 'h-10 text-xs px-3.5 rounded-xl gap-2.5 font-medium',
    lg: 'h-11 text-sm px-4 rounded-xl gap-3 font-medium',
  }
  return map[props.size] || map.sm
})
</script>

<template>
  <div
    ref="containerRef"
    class="relative inline-block select-none"
    :class="widthClass"
  >
    <!-- Trigger Button (Soft Elegant Surface) -->
    <button
      type="button"
      :disabled="disabled"
      @click="toggleDropdown"
      class="w-full flex items-center justify-between border transition-all duration-150 cursor-pointer shadow-2xs text-left outline-none focus-visible:ring-2 disabled:opacity-50 disabled:cursor-not-allowed group"
      :class="[
        sizeClasses,
        isOpen
          ? 'bg-neutral-50/90 dark:bg-[var(--theme-surface-raised)] border-neutral-300 dark:border-[var(--theme-border-strong)] ring-2 ring-neutral-300/60 dark:ring-[var(--theme-border-strong)]'
          : 'bg-white hover:bg-neutral-50/70 dark:bg-[var(--theme-surface-raised)] dark:hover:bg-[var(--theme-surface-muted)] border-neutral-200 dark:border-[var(--theme-border)] hover:border-neutral-300 dark:hover:border-[var(--theme-border-strong)]',
      ]"
      :title="selectedOption ? selectedOption.label : placeholder"
      aria-haspopup="listbox"
      :aria-expanded="isOpen"
    >
      <!-- Left Content: Icon + Label / Placeholder -->
      <div class="flex items-center gap-2 min-w-0 flex-1">
        <!-- Leading Icon -->
        <component
          :is="leadingIcon"
          v-if="leadingIcon"
          class="w-3.5 h-3.5 text-neutral-400 group-hover:text-neutral-600 dark:group-hover:text-neutral-300 shrink-0 transition-colors"
        />

        <!-- Selected Option Icon if any -->
        <component
          :is="selectedOption.icon"
          v-else-if="selectedOption?.icon"
          class="w-3.5 h-3.5 text-neutral-500 shrink-0"
        />

        <!-- Label / Placeholder -->
        <span
          v-if="selectedOption && selectedOption.label"
          class="truncate font-semibold text-neutral-900 dark:text-neutral-100 text-xs"
        >
          {{ selectedOption.label }}
        </span>
        <span
          v-else
          class="truncate text-neutral-400 dark:text-neutral-500 font-normal text-xs"
        >
          {{ placeholder }}
        </span>

        <!-- Optional Badge on Selected -->
        <span
          v-if="selectedOption?.badge !== null && selectedOption?.badge !== undefined"
          class="text-[10px] font-bold px-1.5 py-0.5 rounded-md bg-neutral-100 dark:bg-[var(--theme-surface-muted)] text-neutral-600 dark:text-neutral-400 shrink-0 ml-auto mr-1 hidden sm:inline-block border border-neutral-200/60 dark:border-[var(--theme-border-strong)]"
        >
          {{ selectedOption.badge }}
        </span>
      </div>

      <!-- Right Content: Clear Button & Chevron -->
      <div class="flex items-center gap-1 shrink-0 ml-1.5">
        <!-- Clearable X Button -->
        <button
          v-if="clearable && selectedOption && selectedOption.value !== clearValue"
          type="button"
          @click="handleClear"
          class="p-0.5 rounded-md text-neutral-400 hover:text-neutral-700 dark:hover:text-neutral-200 hover:bg-neutral-200/60 dark:hover:bg-[var(--theme-surface-muted)] transition cursor-pointer"
          title="Hapus pilihan"
        >
          <X class="w-3 h-3" />
        </button>

        <!-- Chevron Down Icon with Smooth Rotation -->
        <ChevronDown
          class="w-3.5 h-3.5 text-neutral-400 transition-transform duration-200 ease-out"
          :class="isOpen ? 'rotate-180 text-neutral-800 dark:text-white' : ''"
        />
      </div>
    </button>

    <!-- Floating Dropdown Popover (Soft elevated card with backdrop blur) -->
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
        class="absolute top-full mt-1.5 w-full min-w-[230px] max-w-[340px] z-50 rounded-2xl bg-white/95 dark:bg-[var(--theme-surface-raised)]/95 backdrop-blur-md border border-neutral-200/90 dark:border-[var(--theme-border)] shadow-xl p-1.5 space-y-1.5"
        :class="align === 'right' ? 'right-0' : 'left-0'"
        role="listbox"
      >
        <!-- Search Input Bar (Shown if searchable=true) -->
        <div v-if="searchable" class="relative">
          <Search class="w-3.5 h-3.5 text-neutral-400 absolute left-2.5 top-1/2 -translate-y-1/2 pointer-events-none" />
          <input
            ref="searchInputRef"
            v-model="searchQuery"
            type="text"
            :placeholder="searchPlaceholder"
            class="w-full pl-8 pr-7 py-1.5 rounded-xl text-xs bg-neutral-50 dark:bg-[var(--theme-layer-2)] border border-neutral-200/80 dark:border-[var(--theme-border)] text-neutral-900 dark:text-white placeholder:text-neutral-400 focus:outline-none focus:ring-2 focus:ring-neutral-300 dark:focus:ring-[var(--theme-border-strong)] transition"
          />
          <button
            v-if="searchQuery"
            type="button"
            @click="searchQuery = ''"
            class="absolute right-2 top-1/2 -translate-y-1/2 p-0.5 text-neutral-400 hover:text-neutral-700 dark:hover:text-neutral-200 cursor-pointer"
          >
            <X class="w-3 h-3" />
          </button>
        </div>

        <!-- Options List -->
        <div
          ref="listContainerRef"
          class="max-h-56 overflow-y-auto space-y-0.5 pr-0.5 custom-scroll"
        >
          <div
            v-for="(opt, idx) in filteredOptions"
            :key="opt.value"
            :data-index="idx"
            @click="selectOption(opt)"
            @mouseenter="highlightedIndex = idx"
            class="px-2.5 py-1.5 rounded-xl text-xs flex items-center justify-between transition-all select-none"
            :class="[
              opt.disabled
                ? 'opacity-40 cursor-not-allowed'
                : 'cursor-pointer',
              selectedOption?.value === opt.value
                ? 'bg-neutral-100 dark:bg-[var(--theme-surface-muted)] font-semibold text-neutral-900 dark:text-white'
                : highlightedIndex === idx
                ? 'bg-neutral-50 dark:bg-[var(--theme-surface-muted)] text-neutral-900 dark:text-white'
                : 'text-neutral-700 dark:text-neutral-300 hover:bg-neutral-50 dark:hover:bg-[var(--theme-surface-muted)] hover:text-neutral-900 dark:hover:text-white',
            ]"
            role="option"
            :aria-selected="selectedOption?.value === opt.value"
          >
            <!-- Option Icon + Label -->
            <div class="flex items-center gap-2 min-w-0 flex-1">
              <component
                :is="opt.icon"
                v-if="opt.icon"
                class="w-3.5 h-3.5 text-neutral-400 shrink-0"
              />
              <span class="truncate">{{ opt.label }}</span>
            </div>

            <!-- Right Indicator: Badge / Checkmark -->
            <div class="flex items-center gap-1.5 shrink-0 ml-2">
              <span
                v-if="opt.badge !== null"
                class="text-[10px] font-semibold px-1.5 py-0.5 rounded-full bg-neutral-100 dark:bg-[var(--theme-surface-muted)] text-neutral-500 dark:text-neutral-400 border border-neutral-200/60 dark:border-[var(--theme-border-strong)]"
              >
                {{ opt.badge }}
              </span>

              <div class="w-4 h-4 flex items-center justify-center">
                <Check
                  v-if="selectedOption?.value === opt.value"
                  class="w-3.5 h-3.5 text-emerald-600 dark:text-emerald-400 shrink-0"
                />
              </div>
            </div>
          </div>

          <!-- Empty State when search returns 0 results -->
          <div
            v-if="filteredOptions.length === 0"
            class="py-4 px-2 text-center text-xs text-neutral-400 dark:text-neutral-500 space-y-1"
          >
            <p>{{ emptyText }}</p>
          </div>
        </div>
      </div>
    </Transition>
  </div>
</template>

<style scoped>
/* Soft Custom Scrollbar */
.custom-scroll::-webkit-scrollbar {
  width: 4px;
}
.custom-scroll::-webkit-scrollbar-track {
  background: transparent;
}
.custom-scroll::-webkit-scrollbar-thumb {
  background: rgba(160, 160, 160, 0.25);
  border-radius: 9999px;
}
.custom-scroll::-webkit-scrollbar-thumb:hover {
  background: rgba(160, 160, 160, 0.4);
}
</style>
