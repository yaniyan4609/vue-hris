<script setup>
import { computed, useSlots } from 'vue'
import { ArrowUp, ArrowDown, ArrowUpDown, X, AlertTriangle, RefreshCw } from 'lucide-vue-next'
import BaseCheckbox from './BaseCheckbox.vue'
import BasePagination from './BasePagination.vue'
import BaseEmptyState from './BaseEmptyState.vue'
import BaseSkeleton from './BaseSkeleton.vue'
import BaseButton from './BaseButton.vue'

const props = defineProps({
  columns: {
    type: Array,
    required: true,
    // Each column: { key: string, label: string, sortable?: boolean, align?: 'left'|'center'|'right', width?: string, headerClass?: string, cellClass?: string }
  },
  rows: {
    type: Array,
    required: true,
  },
  totalItems: {
    type: Number,
    default: null,
  },
  rowKey: {
    type: String,
    default: 'id',
  },
  selectable: {
    type: Boolean,
    default: false,
  },
  selectedIds: {
    type: Array,
    default: () => [],
  },
  showIndex: {
    type: Boolean,
    default: true,
  },
  sortKey: {
    type: String,
    default: '',
  },
  sortOrder: {
    type: String,
    default: 'asc', // 'asc' | 'desc'
  },
  currentPage: {
    type: Number,
    default: 1,
  },
  itemsPerPage: {
    type: [Number, String],
    default: 15,
  },
  perPageOptions: {
    type: Array,
    default: () => [15, 30, 50, 'all'],
  },
  paginated: {
    type: Boolean,
    default: true,
  },
  serverSide: {
    type: Boolean,
    default: false,
  },
  loading: {
    type: Boolean,
    default: false,
  },
  error: {
    type: String,
    default: '',
  },
  errorTitle: {
    type: String,
    default: 'Gagal Memuat Data',
  },
  emptyTitle: {
    type: String,
    default: 'Data Tidak Ditemukan',
  },
  emptyDescription: {
    type: String,
    default: 'Tidak ada data yang cocok dengan kriteria filter atau pencarian Anda.',
  },
  emptyActionText: {
    type: String,
    default: 'Reset Filter',
  },
  emptyIcon: {
    type: [Object, Function],
    default: null,
  },
  selectedLabel: {
    type: String,
    default: 'Data Terpilih',
  },
  compactMobilePagination: {
    type: Boolean,
    default: false,
  },
})

const emit = defineEmits([
  'update:selectedIds',
  'update:sortKey',
  'update:sortOrder',
  'update:currentPage',
  'update:itemsPerPage',
  'sort',
  'change',
  'retry',
  'row-click',
  'empty-action',
])

const slots = useSlots()

const hasActionsSlot = computed(() => Boolean(slots.actions))

const computedTotalItems = computed(() => {
  return props.totalItems !== null && props.totalItems !== undefined
    ? props.totalItems
    : props.rows.length
})

const totalColumnsCount = computed(() => {
  let count = props.columns.length
  if (props.selectable) count++
  if (props.showIndex) count++
  if (hasActionsSlot.value) count++
  return count
})

const getRowNumber = (idx) => {
  if (!props.paginated || props.itemsPerPage === 'all') return idx + 1
  return (props.currentPage - 1) * Number(props.itemsPerPage) + idx + 1
}

const getRowId = (row) => row[props.rowKey]

// Multi-select logic
const isAllPageSelected = computed(() => {
  if (!props.selectable || props.rows.length === 0) return false
  return props.rows.every((row) => props.selectedIds.includes(getRowId(row)))
})

const isPageIndeterminate = computed(() => {
  if (!props.selectable || props.rows.length === 0) return false
  const selectedOnPage = props.rows.filter((row) => props.selectedIds.includes(getRowId(row))).length
  return selectedOnPage > 0 && selectedOnPage < props.rows.length
})

const toggleSelectAll = () => {
  if (!props.selectable) return
  const pageIds = props.rows.map((r) => getRowId(r))
  let next
  if (isAllPageSelected.value) {
    next = props.selectedIds.filter((id) => !pageIds.includes(id))
  } else {
    next = Array.from(new Set([...props.selectedIds, ...pageIds]))
  }
  emit('update:selectedIds', next)
}

const toggleSelectRow = (id) => {
  if (!props.selectable) return
  const next = [...props.selectedIds]
  const idx = next.indexOf(id)
  if (idx !== -1) {
    next.splice(idx, 1)
  } else {
    next.push(id)
  }
  emit('update:selectedIds', next)
}

const clearSelection = () => {
  emit('update:selectedIds', [])
}

const handleSort = (col) => {
  if (!col.sortable) return
  let nextOrder = 'asc'
  if (props.sortKey === col.key) {
    nextOrder = props.sortOrder === 'asc' ? 'desc' : 'asc'
  }
  emit('update:sortKey', col.key)
  emit('update:sortOrder', nextOrder)
  emit('sort', { key: col.key, order: nextOrder })
  emit('change', {
    page: props.currentPage,
    itemsPerPage: props.itemsPerPage,
    sortKey: col.key,
    sortOrder: nextOrder,
  })
}

const handlePageUpdate = (page) => {
  emit('update:currentPage', page)
  emit('change', {
    page,
    itemsPerPage: props.itemsPerPage,
    sortKey: props.sortKey,
    sortOrder: props.sortOrder,
  })
}

const handleItemsPerPageUpdate = (perPage) => {
  emit('update:itemsPerPage', perPage)
  emit('change', {
    page: 1,
    itemsPerPage: perPage,
    sortKey: props.sortKey,
    sortOrder: props.sortOrder,
  })
}

const alignClass = (align) => {
  if (align === 'center') return 'text-center'
  if (align === 'right') return 'text-right'
  return 'text-left'
}
</script>

<template>
  <!-- ======================================================== -->
  <!-- DESKTOP TABLE VIEW (hidden md:flex flex-1 min-h-0)       -->
  <!-- ======================================================== -->
  <div class="hidden md:flex flex-1 min-h-0 flex-col rounded-2xl border border-neutral-200 dark:border-[var(--theme-border)] bg-white dark:bg-[var(--theme-surface)] shadow-2xs overflow-hidden">
    <!-- Area Scroll Mandiri -->
    <div class="flex-1 min-h-0 overflow-y-auto overflow-x-auto">
      <table class="w-full text-left border-collapse text-xs">
        <!-- STICKY THEAD -->
        <thead class="sticky top-0 z-20 bg-neutral-100/95 dark:bg-[var(--theme-surface-raised)]/95 backdrop-blur-md border-b border-neutral-200 dark:border-[var(--theme-border)] shadow-2xs">
          <tr class="text-neutral-600 dark:text-neutral-400 font-semibold select-none text-[11px] uppercase tracking-wider">
            <!-- Checkbox Select All -->
            <th v-if="selectable" class="py-2.5 px-3 text-center w-10">
              <div class="flex items-center justify-center">
                <BaseCheckbox
                  :model-value="isAllPageSelected"
                  :indeterminate="isPageIndeterminate"
                  size="sm"
                  title="Pilih Semua di Halaman Ini"
                  @change="toggleSelectAll"
                />
              </div>
            </th>

            <!-- Nomor Urut -->
            <th v-if="showIndex" class="py-2.5 px-3 text-center w-11">No</th>

            <!-- Dynamic Columns -->
            <th
              v-for="col in columns"
              :key="col.key"
              class="py-2.5 px-3 transition"
              :class="[
                alignClass(col.align),
                col.width || '',
                col.headerClass || '',
                col.sortable ? 'cursor-pointer hover:text-neutral-900 dark:hover:text-white' : '',
              ]"
              @click="handleSort(col)"
            >
              <slot :name="`header-${col.key}`" :column="col">
                <div
                  class="inline-flex items-center gap-1.5"
                  :class="col.align === 'center' ? 'justify-center' : col.align === 'right' ? 'justify-end' : ''"
                >
                  <span>{{ col.label }}</span>
                  <template v-if="col.sortable">
                    <ArrowUp
                      v-if="sortKey === col.key && sortOrder === 'asc'"
                      class="w-3.5 h-3.5 text-[var(--theme-primary)]"
                    />
                    <ArrowDown
                      v-else-if="sortKey === col.key && sortOrder === 'desc'"
                      class="w-3.5 h-3.5 text-[var(--theme-primary)]"
                    />
                    <ArrowUpDown v-else class="w-3.5 h-3.5 opacity-40" />
                  </template>
                </div>
              </slot>
            </th>

            <!-- Actions Column -->
            <th v-if="hasActionsSlot" class="py-2.5 px-4 text-right w-24">Aksi</th>
          </tr>
        </thead>

        <!-- TABLE BODY -->
        <tbody class="divide-y divide-neutral-200/70 dark:divide-[var(--theme-border)]">
          <!-- Loading Skeleton Rows -->
          <template v-if="loading">
            <tr v-for="n in 5" :key="`skel-${n}`">
              <td
                v-for="c in totalColumnsCount"
                :key="`skel-${n}-${c}`"
                class="py-3 px-3"
              >
                <BaseSkeleton height="14px" />
              </td>
            </tr>
          </template>

          <!-- Error State -->
          <tr v-else-if="error">
            <td :colspan="totalColumnsCount" class="py-12 px-6">
              <div class="flex flex-col items-center justify-center text-center max-w-sm mx-auto">
                <div class="w-12 h-12 rounded-2xl bg-rose-50 dark:bg-rose-950/30 border border-rose-200/80 dark:border-rose-900/50 flex items-center justify-center mb-3 text-rose-600 dark:text-rose-400">
                  <AlertTriangle class="w-5 h-5" />
                </div>
                <p class="text-sm font-bold text-neutral-900 dark:text-white">{{ errorTitle }}</p>
                <p class="text-xs text-neutral-500 dark:text-neutral-400 mt-1 mb-4">{{ error }}</p>
                <BaseButton size="sm" variant="secondary" @click="emit('retry')">
                  <RefreshCw class="w-3.5 h-3.5" /> Coba Lagi
                </BaseButton>
              </div>
            </td>
          </tr>

          <!-- Data Rows -->
          <template v-else-if="rows.length > 0">
            <tr
              v-for="(row, idx) in rows"
              :key="getRowId(row) ?? idx"
              class="hover:bg-neutral-50/80 dark:hover:bg-[var(--theme-surface-raised)] transition-colors"
              :class="
                selectable && selectedIds.includes(getRowId(row))
                  ? 'bg-[var(--theme-primary-soft)]/40 dark:bg-[var(--theme-primary-soft)]/20'
                  : ''
              "
              @click="emit('row-click', row)"
            >
              <!-- Row Checkbox -->
              <td v-if="selectable" class="py-2.5 px-3 text-center" @click.stop>
                <div class="flex items-center justify-center">
                  <BaseCheckbox
                    :model-value="selectedIds.includes(getRowId(row))"
                    size="sm"
                    @change="toggleSelectRow(getRowId(row))"
                  />
                </div>
              </td>

              <!-- Row Number -->
              <td v-if="showIndex" class="py-2.5 px-3 text-center font-bold text-neutral-400 text-xs">
                {{ getRowNumber(idx) }}
              </td>

              <!-- Dynamic Cells -->
              <td
                v-for="col in columns"
                :key="col.key"
                class="py-2.5 px-3"
                :class="[alignClass(col.align), col.cellClass || '']"
              >
                <slot
                  :name="`cell-${col.key}`"
                  :row="row"
                  :index="idx"
                  :row-number="getRowNumber(idx)"
                  :value="row[col.key]"
                >
                  {{ row[col.key] }}
                </slot>
              </td>

              <!-- Actions Cell -->
              <td v-if="hasActionsSlot" class="py-2.5 px-4 text-right whitespace-nowrap" @click.stop>
                <slot name="actions" :row="row" :index="idx" />
              </td>
            </tr>
          </template>

          <!-- Empty State -->
          <tr v-else>
            <td :colspan="totalColumnsCount" class="p-0">
              <BaseEmptyState
                :icon="emptyIcon"
                :title="emptyTitle"
                :description="emptyDescription"
                :action-text="emptyActionText"
                @action="emit('empty-action')"
              />
            </td>
          </tr>
        </tbody>
      </table>
    </div>

    <!-- Pinned Footer Pagination -->
    <div
      v-if="paginated"
      class="py-2.5 px-4 border-t border-neutral-200 dark:border-[var(--theme-border)] bg-neutral-50/70 dark:bg-[var(--theme-layer-2)] shrink-0"
    >
      <BasePagination
        :current-page="currentPage"
        :total-items="computedTotalItems"
        :items-per-page="itemsPerPage"
        :per-page-options="perPageOptions"
        @update:current-page="handlePageUpdate"
        @update:items-per-page="handleItemsPerPageUpdate"
      />
    </div>
  </div>

  <!-- ======================================================== -->
  <!-- MOBILE CARD LIST VIEW (md:hidden flex-1 min-h-0)         -->
  <!-- ======================================================== -->
  <div class="md:hidden flex-1 min-h-0 flex flex-col overflow-hidden">
    <div class="flex-1 min-h-0 overflow-y-auto space-y-2.5 pr-0.5">
      <!-- Loading Mobile -->
      <template v-if="loading">
        <BaseSkeleton v-for="n in 3" :key="`mskel-${n}`" variant="card" />
      </template>

      <!-- Error State Mobile -->
      <div
        v-else-if="error"
        class="p-5 rounded-2xl bg-white dark:bg-[var(--theme-surface)] border border-rose-200 dark:border-rose-900/50 text-center"
      >
        <AlertTriangle class="w-6 h-6 text-rose-500 mx-auto mb-2" />
        <p class="text-xs font-bold text-neutral-900 dark:text-white">{{ errorTitle }}</p>
        <p class="text-[11px] text-neutral-500 dark:text-neutral-400 mt-1 mb-3">{{ error }}</p>
        <BaseButton size="sm" variant="secondary" @click="emit('retry')">
          <RefreshCw class="w-3.5 h-3.5" /> Coba Lagi
        </BaseButton>
      </div>

      <!-- Empty State Mobile -->
      <div
        v-else-if="rows.length === 0"
        class="p-4 rounded-2xl bg-white dark:bg-[var(--theme-surface)] border border-neutral-200 dark:border-[var(--theme-border)]"
      >
        <BaseEmptyState
          :icon="emptyIcon"
          :title="emptyTitle"
          :description="emptyDescription"
          :action-text="emptyActionText"
          compact
          @action="emit('empty-action')"
        />
      </div>

      <!-- Mobile Cards -->
      <template v-else>
        <template v-for="(row, idx) in rows" :key="getRowId(row) ?? idx">
          <slot
            name="mobile-card"
            :row="row"
            :index="idx"
            :row-number="getRowNumber(idx)"
            :is-selected="selectable && selectedIds.includes(getRowId(row))"
            :toggle-select="() => toggleSelectRow(getRowId(row))"
          >
            <!-- Default Mobile Card Fallback -->
            <div
              class="p-3.5 rounded-2xl bg-white dark:bg-[var(--theme-surface)] border border-neutral-200 dark:border-[var(--theme-border)] shadow-xs space-y-2.5"
              :class="selectable && selectedIds.includes(getRowId(row)) ? 'ring-2 ring-[var(--theme-primary)]' : ''"
            >
              <div class="flex items-center justify-between gap-2">
                <div class="flex items-center gap-2">
                  <BaseCheckbox
                    v-if="selectable"
                    :model-value="selectedIds.includes(getRowId(row))"
                    size="sm"
                    @change="toggleSelectRow(getRowId(row))"
                  />
                  <span
                    v-if="showIndex"
                    class="w-6 h-6 rounded-lg bg-neutral-100 dark:bg-[var(--theme-surface-muted)] text-neutral-700 dark:text-neutral-300 font-bold text-xs flex items-center justify-center shrink-0"
                  >
                    #{{ getRowNumber(idx) }}
                  </span>
                </div>
                <div v-if="hasActionsSlot">
                  <slot name="actions" :row="row" :index="idx" />
                </div>
              </div>

              <div class="space-y-1.5 text-xs">
                <div
                  v-for="col in columns"
                  :key="col.key"
                  class="flex items-start justify-between gap-2"
                >
                  <span class="text-neutral-400 text-[11px] font-semibold uppercase">{{ col.label }}</span>
                  <div class="text-right font-medium text-neutral-800 dark:text-neutral-200">
                    <slot
                      :name="`cell-${col.key}`"
                      :row="row"
                      :index="idx"
                      :row-number="getRowNumber(idx)"
                      :value="row[col.key]"
                    >
                      {{ row[col.key] }}
                    </slot>
                  </div>
                </div>
              </div>
            </div>
          </slot>
        </template>
      </template>
    </div>

    <!-- Mobile Pinned Pagination -->
    <div
      v-if="paginated && (!compactMobilePagination || (itemsPerPage !== 'all' && computedTotalItems > Number(itemsPerPage)))"
      class="pt-2 border-t border-neutral-200 dark:border-[var(--theme-border)] shrink-0"
    >
      <BasePagination
        :current-page="currentPage"
        :total-items="computedTotalItems"
        :items-per-page="itemsPerPage"
        :per-page-options="perPageOptions"
        :compact="compactMobilePagination"
        @update:current-page="handlePageUpdate"
        @update:items-per-page="handleItemsPerPageUpdate"
      />
    </div>
  </div>

  <!-- ======================================================== -->
  <!-- FLOATING BULK ACTION BAR                                 -->
  <!-- ======================================================== -->
  <Transition
    enter-active-class="transition duration-200 ease-out"
    enter-from-class="opacity-0 translate-y-6"
    enter-to-class="opacity-100 translate-y-0"
    leave-active-class="transition duration-150 ease-in"
    leave-from-class="opacity-100 translate-y-0"
    leave-to-class="opacity-0 translate-y-6"
  >
    <div
      v-if="selectable && selectedIds.length > 0"
      class="fixed bottom-6 left-1/2 -translate-x-1/2 z-40 bg-white/95 dark:bg-[var(--theme-surface-raised)]/95 backdrop-blur-md border border-neutral-300 dark:border-[var(--theme-border-strong)] shadow-2xl rounded-2xl px-4 py-2.5 flex items-center gap-3 text-xs will-change-transform max-w-[calc(100vw-32px)]"
    >
      <div class="flex items-center gap-2 pr-2 border-r border-neutral-200 dark:border-[var(--theme-border)]">
        <span class="w-5 h-5 rounded-full bg-[var(--theme-primary)] text-[var(--theme-primary-text)] flex items-center justify-center font-bold text-[11px]">
          {{ selectedIds.length }}
        </span>
        <span class="font-semibold text-neutral-800 dark:text-neutral-200 hidden sm:inline">
          {{ selectedLabel }}
        </span>
      </div>

      <!-- Custom Bulk Action Buttons Slot -->
      <slot
        name="bulk-actions"
        :selected-ids="selectedIds"
        :clear-selection="clearSelection"
      />

      <!-- Batal Pilih -->
      <button
        type="button"
        @click="clearSelection"
        class="p-1 rounded-lg text-neutral-400 hover:text-neutral-700 dark:hover:text-neutral-200 transition cursor-pointer"
        title="Batal Pilih Semua"
      >
        <X class="w-4 h-4" />
      </button>
    </div>
  </Transition>
</template>
