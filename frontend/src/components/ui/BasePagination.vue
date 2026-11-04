<script setup>
import { computed } from 'vue'
import { ChevronLeft, ChevronRight, ChevronsLeft, ChevronsRight } from 'lucide-vue-next'

const props = defineProps({
  currentPage: {
    type: Number,
    required: true,
  },
  totalItems: {
    type: Number,
    required: true,
  },
  itemsPerPage: {
    type: [Number, String],
    default: 15,
  },
  perPageOptions: {
    type: Array,
    default: () => [15, 30, 50, 'all'],
  },
  compact: {
    type: Boolean,
    default: false,
  },
})

const emit = defineEmits(['update:currentPage', 'update:itemsPerPage', 'change'])

const totalPages = computed(() => {
  if (props.itemsPerPage === 'all') return 1
  const perPage = Number(props.itemsPerPage) || 15
  return Math.max(1, Math.ceil(props.totalItems / perPage))
})

const startItem = computed(() => {
  if (props.totalItems === 0) return 0
  if (props.itemsPerPage === 'all') return 1
  return (props.currentPage - 1) * Number(props.itemsPerPage) + 1
})

const endItem = computed(() => {
  if (props.itemsPerPage === 'all') return props.totalItems
  return Math.min(props.totalItems, props.currentPage * Number(props.itemsPerPage))
})

const setPage = (page) => {
  const target = Math.max(1, Math.min(totalPages.value, page))
  if (target !== props.currentPage) {
    emit('update:currentPage', target)
    emit('change', { page: target, itemsPerPage: props.itemsPerPage })
  }
}

const handlePerPageChange = (e) => {
  const val = e.target.value === 'all' ? 'all' : Number(e.target.value)
  emit('update:itemsPerPage', val)
  emit('update:currentPage', 1)
  emit('change', { page: 1, itemsPerPage: val })
}

// Visible page numbers array with smart ellipsis (e.g. 1 ... 4 5 6 ... 10)
const visiblePages = computed(() => {
  const total = totalPages.value
  const current = props.currentPage
  if (total <= 5) {
    return Array.from({ length: total }, (_, i) => i + 1)
  }

  const pages = []
  if (current <= 3) {
    pages.push(1, 2, 3, 4, '...', total)
  } else if (current >= total - 2) {
    pages.push(1, '...', total - 3, total - 2, total - 1, total)
  } else {
    pages.push(1, '...', current - 1, current, current + 1, '...', total)
  }
  return pages
})
</script>

<template>
  <div class="flex flex-col sm:flex-row items-center justify-between gap-3 text-xs select-none">
    <!-- Info & Items Per Page Selector -->
    <div class="flex items-center gap-3 text-neutral-500 dark:text-neutral-400 text-[11px] sm:text-xs">
      <span>
        Menampilkan
        <strong class="text-neutral-800 dark:text-neutral-100 font-semibold font-mono">
          {{ startItem }}-{{ endItem }}
        </strong>
        dari
        <strong class="text-neutral-800 dark:text-neutral-100 font-semibold font-mono">
          {{ totalItems }}
        </strong>
        data
      </span>

      <!-- Per Page Selector Dropdown -->
      <div v-if="perPageOptions.length > 0" class="flex items-center gap-1.5 pl-3 border-l border-neutral-200 dark:border-[var(--theme-border)]">
        <span class="hidden sm:inline">Baris:</span>
        <select
          :value="itemsPerPage"
          @change="handlePerPageChange"
          class="py-0.5 px-2 rounded-lg bg-white dark:bg-[var(--theme-surface-raised)] border border-neutral-200 dark:border-[var(--theme-border)] text-neutral-800 dark:text-neutral-200 text-xs font-semibold cursor-pointer focus:outline-none focus:ring-2 focus:ring-neutral-400 dark:focus:ring-[var(--theme-border-strong)]"
        >
          <option
            v-for="opt in perPageOptions"
            :key="opt"
            :value="opt"
          >
            {{ opt === 'all' ? 'Semua' : opt }}
          </option>
        </select>
      </div>
    </div>

    <!-- Pagination Page Controls -->
    <div v-if="itemsPerPage !== 'all'" class="flex items-center gap-1">
      <!-- First Page Button (optional/desktop) -->
      <button
        v-if="!compact && totalPages > 5"
        type="button"
        @click="setPage(1)"
        :disabled="currentPage === 1"
        class="hidden sm:flex p-1 rounded-lg border border-neutral-200 dark:border-[var(--theme-border)] bg-white dark:bg-[var(--theme-surface-raised)] text-neutral-700 dark:text-neutral-300 disabled:opacity-30 disabled:cursor-not-allowed hover:bg-neutral-50 dark:hover:bg-[var(--theme-surface-muted)] cursor-pointer transition shadow-2xs"
        title="Halaman Pertama"
      >
        <ChevronsLeft class="w-3.5 h-3.5" />
      </button>

      <!-- Prev Button -->
      <button
        type="button"
        @click="setPage(currentPage - 1)"
        :disabled="currentPage === 1"
        class="p-1 px-1.5 rounded-lg border border-neutral-200 dark:border-[var(--theme-border)] bg-white dark:bg-[var(--theme-surface-raised)] text-neutral-700 dark:text-neutral-300 disabled:opacity-30 disabled:cursor-not-allowed hover:bg-neutral-50 dark:hover:bg-[var(--theme-surface-muted)] cursor-pointer transition shadow-2xs flex items-center gap-1"
        title="Sebelumnya"
      >
        <ChevronLeft class="w-3.5 h-3.5" />
        <span v-if="compact" class="text-xs">Prev</span>
      </button>

      <!-- Numbered Page Pills (Desktop Only if not compact) -->
      <div v-if="!compact" class="hidden sm:flex items-center gap-1 mx-1">
        <template v-for="(p, idx) in visiblePages" :key="idx">
          <span
            v-if="p === '...'"
            class="px-1 text-neutral-400 dark:text-neutral-500 font-mono text-xs select-none"
          >
            ...
          </span>
          <button
            v-else
            type="button"
            @click="setPage(p)"
            class="min-w-7 h-7 px-1.5 rounded-lg text-xs font-semibold transition cursor-pointer flex items-center justify-center"
            :class="[
              currentPage === p
                ? 'bg-neutral-900 text-white dark:bg-white dark:text-neutral-950 font-bold shadow-2xs'
                : 'text-neutral-700 dark:text-neutral-300 hover:bg-neutral-100 dark:hover:bg-[var(--theme-surface-muted)]',
            ]"
          >
            {{ p }}
          </button>
        </template>
      </div>

      <!-- Current / Total Indicator on Mobile / Compact -->
      <span v-else class="px-2 font-bold text-xs text-neutral-800 dark:text-neutral-200 font-mono">
        {{ currentPage }} / {{ totalPages }}
      </span>

      <!-- Next Button -->
      <button
        type="button"
        @click="setPage(currentPage + 1)"
        :disabled="currentPage === totalPages"
        class="p-1 px-1.5 rounded-lg border border-neutral-200 dark:border-[var(--theme-border)] bg-white dark:bg-[var(--theme-surface-raised)] text-neutral-700 dark:text-neutral-300 disabled:opacity-30 disabled:cursor-not-allowed hover:bg-neutral-50 dark:hover:bg-[var(--theme-surface-muted)] cursor-pointer transition shadow-2xs flex items-center gap-1"
        title="Selanjutnya"
      >
        <span v-if="compact" class="text-xs">Next</span>
        <ChevronRight class="w-3.5 h-3.5" />
      </button>

      <!-- Last Page Button (optional/desktop) -->
      <button
        v-if="!compact && totalPages > 5"
        type="button"
        @click="setPage(totalPages)"
        :disabled="currentPage === totalPages"
        class="hidden sm:flex p-1 rounded-lg border border-neutral-200 dark:border-[var(--theme-border)] bg-white dark:bg-[var(--theme-surface-raised)] text-neutral-700 dark:text-neutral-300 disabled:opacity-30 disabled:cursor-not-allowed hover:bg-neutral-50 dark:hover:bg-[var(--theme-surface-muted)] cursor-pointer transition shadow-2xs"
        title="Halaman Terakhir"
      >
        <ChevronsRight class="w-3.5 h-3.5" />
      </button>
    </div>
  </div>
</template>

