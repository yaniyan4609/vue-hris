<script setup>
import { ref, computed, watch, nextTick, onMounted, onUnmounted } from 'vue'
import { Search, ChevronDown, X, AlertCircle, Loader2 } from 'lucide-vue-next'
import BaseCheckbox from './BaseCheckbox.vue'

const props = defineProps({
  modelValue: {
    type: Array,
    default: () => [],
  },
  options: {
    type: Array,
    default: () => [],
    // Each option can be a primitive (string/number)
    // or an object: { label: string, value: any, badge?: string|number, icon?: any, disabled?: boolean }
  },
  asyncSearch: {
    type: Function,
    default: null,
    // Optional async (query) => Array<{ label, value, badge?, icon?, disabled? }>
  },
  debounceMs: {
    type: Number,
    default: 300,
  },
  loading: {
    type: Boolean,
    default: false,
  },
  cacheOptions: {
    type: Array,
    default: () => [],
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
    default: true,
  },
  selectAll: {
    type: Boolean,
    default: true,
  },
  maxChips: {
    type: Number,
    default: 2,
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

const emit = defineEmits(['update:modelValue', 'change', 'clear', 'search'])

const isOpen = ref(false)
const searchQuery = ref('')
const highlightedIndex = ref(0)
const searchInputRef = ref(null)
const containerRef = ref(null)
const listContainerRef = ref(null)
const remoteOptions = ref([])
const internalLoading = ref(false)
const cachedSelectedMap = ref(new Map())
let debounceTimer = null

const normalizeSingleOption = (opt) => {
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
}

const isAsyncMode = computed(() => typeof props.asyncSearch === 'function')
const isBusy = computed(() => props.loading || internalLoading.value)

// Normalize options to uniform { label, value, badge, icon, disabled }
const normalizedOptions = computed(() => {
  const source = isAsyncMode.value && remoteOptions.value.length > 0 ? remoteOptions.value : props.options
  return source.map(normalizeSingleOption)
})

// Keep cache updated whenever options or cacheOptions change
watch(
  [normalizedOptions, () => props.cacheOptions],
  ([opts, cacheOpts]) => {
    for (const opt of opts) {
      cachedSelectedMap.value.set(opt.value, opt)
    }
    if (Array.isArray(cacheOpts)) {
      for (const raw of cacheOpts) {
        const norm = normalizeSingleOption(raw)
        cachedSelectedMap.value.set(norm.value, norm)
      }
    }
  },
  { immediate: true }
)

const selectedSet = computed(() => new Set(props.modelValue || []))

const selectedOptions = computed(() => {
  return (props.modelValue || []).map((val) => {
    const found = normalizedOptions.value.find((o) => o.value === val)
    if (found) return found
    if (cachedSelectedMap.value.has(val)) return cachedSelectedMap.value.get(val)
    return { label: String(val), value: val, badge: null, icon: null, disabled: false }
  })
})

const visibleChips = computed(() => {
  return selectedOptions.value.slice(0, props.maxChips)
})

const overflowCount = computed(() => {
  return Math.max(0, selectedOptions.value.length - props.maxChips)
})

// Filtered options based on search query
const filteredOptions = computed(() => {
  if (isAsyncMode.value) {
    return normalizedOptions.value
  }
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

const runSearch = async (query) => {
  emit('search', query)
  if (!isAsyncMode.value) return
  internalLoading.value = true
  try {
    const res = await props.asyncSearch(query)
    if (Array.isArray(res)) {
      remoteOptions.value = res
    }
  } finally {
    internalLoading.value = false
  }
}

watch(searchQuery, (q) => {
  clearTimeout(debounceTimer)
  debounceTimer = setTimeout(() => {
    runSearch(q)
  }, props.debounceMs)
})

const enabledFilteredOptions = computed(() => {
  return filteredOptions.value.filter((opt) => !opt.disabled)
})

const isAllFilteredSelected = computed(() => {
  if (enabledFilteredOptions.value.length === 0) return false
  return enabledFilteredOptions.value.every((opt) => selectedSet.value.has(opt.value))
})

const isFilteredIndeterminate = computed(() => {
  if (enabledFilteredOptions.value.length === 0) return false
  const count = enabledFilteredOptions.value.filter((opt) => selectedSet.value.has(opt.value)).length
  return count > 0 && count < enabledFilteredOptions.value.length
})

watch(filteredOptions, (opts) => {
  highlightedIndex.value = opts.length > 0 ? 0 : -1
})

const toggleDropdown = () => {
  if (props.disabled) return
  isOpen.value = !isOpen.value
}

watch(isOpen, async (val) => {
  if (val) {
    searchQuery.value = ''
    highlightedIndex.value = 0
    if (isAsyncMode.value && remoteOptions.value.length === 0) {
      runSearch('')
    }
    await nextTick()
    if (props.searchable) {
      searchInputRef.value?.focus()
    }
  }
})

const toggleOption = (opt) => {
  if (opt.disabled) return
  cachedSelectedMap.value.set(opt.value, opt)
  const next = [...(props.modelValue || [])]
  const idx = next.indexOf(opt.value)
  if (idx !== -1) {
    next.splice(idx, 1)
  } else {
    next.push(opt.value)
  }
  emit('update:modelValue', next)
  emit('change', next)
}

const removeChip = (val, e) => {
  e.stopPropagation()
  if (props.disabled) return
  const next = (props.modelValue || []).filter((v) => v !== val)
  emit('update:modelValue', next)
  emit('change', next)
}

const toggleSelectAllFiltered = () => {
  const targetValues = enabledFilteredOptions.value.map((o) => {
    cachedSelectedMap.value.set(o.value, o)
    return o.value
  })
  let next
  if (isAllFilteredSelected.value) {
    const removeSet = new Set(targetValues)
    next = (props.modelValue || []).filter((v) => !removeSet.has(v))
  } else {
    next = Array.from(new Set([...(props.modelValue || []), ...targetValues]))
  }
  emit('update:modelValue', next)
  emit('change', next)
}

const handleClearAll = (e) => {
  e?.stopPropagation?.()
  if (props.disabled) return
  emit('update:modelValue', [])
  emit('clear')
  emit('change', [])
}

const handleClickOutside = (e) => {
  if (containerRef.value && !containerRef.value.contains(e.target)) {
    isOpen.value = false
  }
}

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
  } else if (e.key === 'Enter' || (e.key === ' ' && document.activeElement !== searchInputRef.value)) {
    e.preventDefault()
    if (highlightedIndex.value >= 0 && highlightedIndex.value < filteredOptions.value.length) {
      toggleOption(filteredOptions.value[highlightedIndex.value])
    }
  } else if (e.key === 'Escape') {
    e.preventDefault()
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
  clearTimeout(debounceTimer)
  document.removeEventListener('click', handleClickOutside)
  document.removeEventListener('keydown', handleKeydown)
})

const sizeClasses = computed(() => {
  const map = {
    sm: 'min-h-9 py-1 text-xs px-2.5 rounded-xl gap-1.5',
    md: 'min-h-10 py-1.5 text-xs px-3 rounded-xl gap-2',
    lg: 'min-h-11 py-1.5 text-sm px-3.5 rounded-xl gap-2.5',
  }
  return map[props.size] || map.md
})
</script>

<template>
  <div :class="widthClass">
    <!-- Optional Field Label -->
    <label
      v-if="label"
      class="block text-xs font-semibold text-neutral-700 dark:text-neutral-300 mb-1"
    >
      {{ label }}
      <span v-if="required" class="text-rose-500 ml-0.5">*</span>
    </label>

    <div
      ref="containerRef"
      class="relative inline-block w-full select-none"
    >
      <!-- Trigger Button -->
      <button
        type="button"
        :disabled="disabled"
        @click="toggleDropdown"
        class="w-full flex items-center justify-between border transition-all duration-150 cursor-pointer shadow-2xs text-left outline-none focus-visible:ring-2 disabled:opacity-50 disabled:cursor-not-allowed group"
        :class="[
          sizeClasses,
          error
            ? 'border-rose-400 dark:border-rose-500/80 bg-rose-50/30 dark:bg-rose-950/10'
            : isOpen
            ? 'bg-neutral-50/90 dark:bg-[var(--theme-surface-raised)] border-neutral-300 dark:border-[var(--theme-border-strong)] ring-2 ring-neutral-300/60 dark:ring-[var(--theme-border-strong)]'
            : 'bg-white hover:bg-neutral-50/70 dark:bg-[var(--theme-surface-raised)] dark:hover:bg-[var(--theme-surface-muted)] border-neutral-200 dark:border-[var(--theme-border)] hover:border-neutral-300 dark:hover:border-[var(--theme-border-strong)]',
        ]"
        aria-haspopup="listbox"
        :aria-expanded="isOpen"
      >
        <!-- Left Content: Icon + Chips / Placeholder -->
        <div class="flex items-center gap-1.5 min-w-0 flex-1 flex-wrap">
          <component
            :is="leadingIcon"
            v-if="leadingIcon"
            class="w-3.5 h-3.5 text-neutral-400 group-hover:text-neutral-600 dark:group-hover:text-neutral-300 shrink-0 transition-colors mr-0.5"
          />

          <template v-if="selectedOptions.length > 0">
            <span
              v-for="chip in visibleChips"
              :key="chip.value"
              class="inline-flex items-center gap-1 px-2 py-0.5 rounded-lg text-[11px] font-semibold bg-[var(--theme-primary-soft)] text-[var(--theme-primary-soft-text)] border border-neutral-200/60 dark:border-[var(--theme-border-strong)] max-w-[160px]"
            >
              <span class="truncate">{{ chip.label }}</span>
              <span
                role="button"
                tabindex="-1"
                @click="removeChip(chip.value, $event)"
                class="p-0.5 -mr-0.5 rounded hover:bg-black/10 dark:hover:bg-white/10 transition cursor-pointer"
                :title="`Hapus ${chip.label}`"
              >
                <X class="w-2.5 h-2.5" />
              </span>
            </span>

            <span
              v-if="overflowCount > 0"
              class="inline-flex items-center px-1.5 py-0.5 rounded-lg text-[10.5px] font-bold bg-neutral-100 dark:bg-[var(--theme-surface-muted)] text-neutral-700 dark:text-neutral-300 border border-neutral-200/80 dark:border-[var(--theme-border-strong)]"
            >
              +{{ overflowCount }} lainnya
            </span>
          </template>

          <span
            v-else
            class="truncate text-neutral-400 dark:text-neutral-500 font-normal text-xs"
          >
            {{ placeholder }}
          </span>
        </div>

        <!-- Right Content: Clear Button & Chevron -->
        <div class="flex items-center gap-1 shrink-0 ml-1.5">
          <Loader2
            v-if="isBusy"
            class="w-3.5 h-3.5 text-[var(--theme-primary)] animate-spin"
          />

          <span
            v-if="clearable && selectedOptions.length > 0"
            role="button"
            tabindex="-1"
            @click="handleClearAll"
            class="p-0.5 rounded-md text-neutral-400 hover:text-neutral-700 dark:hover:text-neutral-200 hover:bg-neutral-200/60 dark:hover:bg-[var(--theme-surface-muted)] transition cursor-pointer"
            title="Bersihkan semua pilihan"
          >
            <X class="w-3 h-3" />
          </span>

          <ChevronDown
            class="w-3.5 h-3.5 text-neutral-400 transition-transform duration-200 ease-out"
            :class="isOpen ? 'rotate-180 text-neutral-800 dark:text-white' : ''"
          />
        </div>
      </button>

      <!-- Floating Dropdown Popover -->
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
          class="absolute top-full mt-1.5 w-full min-w-[240px] max-w-[360px] z-50 rounded-2xl bg-white/95 dark:bg-[var(--theme-surface-raised)]/95 backdrop-blur-md border border-neutral-200/90 dark:border-[var(--theme-border)] shadow-xl p-1.5 space-y-1.5"
          :class="align === 'right' ? 'right-0' : 'left-0'"
          role="listbox"
          aria-multiselectable="true"
        >
          <!-- Search Input Bar -->
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

          <!-- Select All / Clear Header Bar -->
          <div
            v-if="selectAll && enabledFilteredOptions.length > 0 && !isBusy"
            class="flex items-center justify-between px-2.5 py-1.5 rounded-xl bg-neutral-50/80 dark:bg-[var(--theme-layer-2)] border border-neutral-200/60 dark:border-[var(--theme-border)] text-[11px]"
          >
            <BaseCheckbox
              :model-value="isAllFilteredSelected"
              :indeterminate="isFilteredIndeterminate"
              size="sm"
              label="Pilih Semua"
              @change="toggleSelectAllFiltered"
            />
            <span class="text-[10.5px] font-mono text-neutral-500 dark:text-neutral-400">
              {{ selectedOptions.length }} dipilih
            </span>
          </div>

          <!-- Options List -->
          <div
            ref="listContainerRef"
            class="max-h-56 overflow-y-auto space-y-0.5 pr-0.5 custom-scroll"
          >
            <!-- Async Loading Indicator -->
            <div
              v-if="isBusy"
              class="py-5 px-2 flex flex-col items-center justify-center gap-1.5 text-xs text-neutral-500 dark:text-neutral-400"
            >
              <Loader2 class="w-4 h-4 text-[var(--theme-primary)] animate-spin" />
              <span>Memuat pilihan...</span>
            </div>

            <template v-else>
              <div
                v-for="(opt, idx) in filteredOptions"
                :key="opt.value"
                :data-index="idx"
                @click="toggleOption(opt)"
                @mouseenter="highlightedIndex = idx"
                class="px-2.5 py-1.5 rounded-xl text-xs flex items-center justify-between gap-2 transition-all select-none"
                :class="[
                  opt.disabled
                    ? 'opacity-40 cursor-not-allowed'
                    : 'cursor-pointer',
                  selectedSet.has(opt.value)
                    ? 'bg-neutral-100/90 dark:bg-[var(--theme-surface-muted)] font-semibold text-neutral-900 dark:text-white'
                    : highlightedIndex === idx
                    ? 'bg-neutral-50 dark:bg-[var(--theme-surface-muted)] text-neutral-900 dark:text-white'
                    : 'text-neutral-700 dark:text-neutral-300 hover:bg-neutral-50 dark:hover:bg-[var(--theme-surface-muted)] hover:text-neutral-900 dark:hover:text-white',
                ]"
                role="option"
                :aria-selected="selectedSet.has(opt.value)"
              >
                <!-- Checkbox + Option Icon + Label -->
                <div class="flex items-center gap-2.5 min-w-0 flex-1 pointer-events-none">
                  <BaseCheckbox
                    :model-value="selectedSet.has(opt.value)"
                    :disabled="opt.disabled"
                    size="sm"
                  />
                  <component
                    :is="opt.icon"
                    v-if="opt.icon"
                    class="w-3.5 h-3.5 text-neutral-400 shrink-0"
                  />
                  <span class="truncate">{{ opt.label }}</span>
                </div>

                <!-- Optional Right Badge -->
                <span
                  v-if="opt.badge !== null && opt.badge !== undefined"
                  class="text-[10px] font-semibold px-1.5 py-0.5 rounded-full bg-neutral-100 dark:bg-[var(--theme-surface-muted)] text-neutral-500 dark:text-neutral-400 border border-neutral-200/60 dark:border-[var(--theme-border-strong)] shrink-0"
                >
                  {{ opt.badge }}
                </span>
              </div>

              <!-- Empty State when search returns 0 results -->
              <div
                v-if="filteredOptions.length === 0"
                class="py-4 px-2 text-center text-xs text-neutral-400 dark:text-neutral-500"
              >
                <p>{{ emptyText }}</p>
              </div>
            </template>
          </div>
        </div>
      </Transition>
    </div>

    <!-- Error / Hint Footer -->
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
