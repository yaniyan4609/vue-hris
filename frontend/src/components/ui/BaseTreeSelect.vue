<script setup>
import { ref, computed, onMounted, onUnmounted } from 'vue'
import { Search, ChevronDown, X, AlertCircle, Network } from 'lucide-vue-next'
import BaseTree from './BaseTree.vue'

const props = defineProps({
  modelValue: {
    type: [Array, String, Number],
    default: () => [],
  },
  nodes: {
    type: Array,
    default: () => [],
  },
  multiple: {
    type: Boolean,
    default: false,
  },
  cascade: {
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
    default: 'Pilih struktur organisasi...',
  },
  searchable: {
    type: Boolean,
    default: true,
  },
  clearable: {
    type: Boolean,
    default: true,
  },
  size: {
    type: String,
    default: 'md', // 'sm' | 'md' | 'lg'
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

const emit = defineEmits(['update:modelValue', 'change', 'clear'])

const isOpen = ref(false)
const searchQuery = ref('')
const containerRef = ref(null)

const nodeMap = computed(() => {
  const map = new Map()
  const walk = (list) => {
    for (const n of list) {
      map.set(n.id, n)
      if (n.children && n.children.length > 0) walk(n.children)
    }
  }
  walk(props.nodes)
  return map
})

const selectedLabels = computed(() => {
  if (props.multiple) {
    const arr = Array.isArray(props.modelValue) ? props.modelValue : []
    return arr.map((id) => nodeMap.value.get(id)?.label || String(id))
  }
  if (props.modelValue !== null && props.modelValue !== undefined && props.modelValue !== '') {
    return [nodeMap.value.get(props.modelValue)?.label || String(props.modelValue)]
  }
  return []
})

const handleUpdate = (val) => {
  emit('update:modelValue', val)
  emit('change', val)
  if (!props.multiple) {
    isOpen.value = false
  }
}

const handleClear = (e) => {
  e?.stopPropagation?.()
  const empty = props.multiple ? [] : ''
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
  const map = {
    sm: 'min-h-9 py-1 text-xs px-2.5 rounded-xl gap-1.5',
    md: 'min-h-10 py-1.5 text-xs px-3 rounded-xl gap-2',
    lg: 'min-h-11 py-1.5 text-sm px-3.5 rounded-xl gap-2.5',
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

    <div ref="containerRef" class="relative inline-block w-full select-none">
      <button
        type="button"
        @click="isOpen = !isOpen"
        class="w-full flex items-center justify-between border transition-all duration-150 cursor-pointer shadow-2xs text-left outline-none focus-visible:ring-2"
        :class="[
          sizeClasses,
          error
            ? 'border-rose-400 dark:border-rose-500/80 bg-rose-50/30 dark:bg-rose-950/10'
            : isOpen
            ? 'bg-neutral-50/90 dark:bg-[var(--theme-surface-raised)] border-neutral-300 dark:border-[var(--theme-border-strong)] ring-2 ring-neutral-300/60 dark:ring-[var(--theme-border-strong)]'
            : 'bg-white hover:bg-neutral-50/70 dark:bg-[var(--theme-surface-raised)] dark:hover:bg-[var(--theme-surface-muted)] border-neutral-200 dark:border-[var(--theme-border)] hover:border-neutral-300 dark:hover:border-[var(--theme-border-strong)]',
        ]"
      >
        <div class="flex items-center gap-1.5 min-w-0 flex-1 flex-wrap">
          <Network class="w-3.5 h-3.5 text-neutral-400 shrink-0 mr-0.5" />
          <template v-if="selectedLabels.length > 0">
            <span
              v-for="(lbl, idx) in selectedLabels.slice(0, 2)"
              :key="idx"
              class="inline-flex items-center px-2 py-0.5 rounded-lg text-[11px] font-semibold bg-[var(--theme-primary-soft)] text-[var(--theme-primary-soft-text)] border border-neutral-200/60 dark:border-[var(--theme-border-strong)] max-w-[160px] truncate"
            >
              {{ lbl }}
            </span>
            <span
              v-if="selectedLabels.length > 2"
              class="inline-flex items-center px-1.5 py-0.5 rounded-lg text-[10.5px] font-bold bg-neutral-100 dark:bg-[var(--theme-surface-muted)] text-neutral-700 dark:text-neutral-300 border border-neutral-200/80 dark:border-[var(--theme-border-strong)]"
            >
              +{{ selectedLabels.length - 2 }} unit
            </span>
          </template>
          <span v-else class="truncate text-neutral-400 dark:text-neutral-500 text-xs">
            {{ placeholder }}
          </span>
        </div>

        <div class="flex items-center gap-1 shrink-0 ml-1.5">
          <span
            v-if="clearable && selectedLabels.length > 0"
            role="button"
            tabindex="-1"
            @click="handleClear"
            class="p-0.5 rounded-md text-neutral-400 hover:text-neutral-700 dark:hover:text-neutral-200 hover:bg-neutral-200/60 dark:hover:bg-[var(--theme-surface-muted)] transition cursor-pointer"
          >
            <X class="w-3 h-3" />
          </span>
          <ChevronDown
            class="w-3.5 h-3.5 text-neutral-400 transition-transform duration-200"
            :class="isOpen ? 'rotate-180 text-neutral-800 dark:text-white' : ''"
          />
        </div>
      </button>

      <!-- Floating Tree Popover -->
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
          class="absolute top-full mt-1.5 left-0 w-full min-w-[260px] z-50 rounded-2xl bg-white/95 dark:bg-[var(--theme-surface-raised)]/95 backdrop-blur-md border border-neutral-200/90 dark:border-[var(--theme-border)] shadow-xl p-2 space-y-2"
        >
          <div v-if="searchable" class="relative">
            <Search class="w-3.5 h-3.5 text-neutral-400 absolute left-2.5 top-1/2 -translate-y-1/2 pointer-events-none" />
            <input
              v-model="searchQuery"
              type="text"
              placeholder="Cari divisi / departemen / jabatan..."
              class="w-full pl-8 pr-7 py-1.5 rounded-xl text-xs bg-neutral-50 dark:bg-[var(--theme-layer-2)] border border-neutral-200/80 dark:border-[var(--theme-border)] text-neutral-900 dark:text-white placeholder:text-neutral-400 focus:outline-none focus:ring-2 focus:ring-neutral-300 dark:focus:ring-[var(--theme-border-strong)]"
            />
          </div>

          <div class="max-h-60 overflow-y-auto pr-0.5 custom-scroll">
            <BaseTree
              :nodes="nodes"
              :model-value="modelValue"
              selectable
              :multiple="multiple"
              :cascade="cascade"
              :search-query="searchQuery"
              @update:model-value="handleUpdate"
            />
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

