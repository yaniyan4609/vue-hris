<script setup>
import { ref, computed, watch } from 'vue'
import BaseButton from '../../components/ui/BaseButton.vue'
import BaseModal from '../../components/ui/BaseModal.vue'
import ConfirmationModal from '../../components/ui/ConfirmationModal.vue'
import BaseSearchableSelect from '../../components/ui/BaseSearchableSelect.vue'
import BaseInput from '../../components/ui/BaseInput.vue'
import BaseBadge from '../../components/ui/BaseBadge.vue'
import BasePagination from '../../components/ui/BasePagination.vue'
import BaseDropdownMenu from '../../components/ui/BaseDropdownMenu.vue'
import BaseEmptyState from '../../components/ui/BaseEmptyState.vue'
import BaseDataTable from '../../components/ui/BaseDataTable.vue'
import BaseFormField from '../../components/ui/BaseFormField.vue'
import { useToast } from '../../composables/useToast'
import { gunungJawaList } from '../../data/gunungJawaData'
import {
  Mountain,
  Plus,
  Pencil,
  Trash2,
  Search,
  ArrowUpDown,
  ArrowUp,
  ArrowDown,
  RotateCcw,
  MapPin,
  Compass,
  Check,
  X,
  ChevronLeft,
  ChevronRight,
  ChevronDown,
  TrendingUp,
  Flame,
  Trees,
  Info,
} from 'lucide-vue-next'

const toast = useToast()

// ========================================================
// 1. DATA AWAL: 159 GUNUNG DI PULAU JAWA
// ========================================================
const initialGunungList = gunungJawaList
const gunungList = ref([...gunungJawaList])

// ========================================================
// 2. SEARCH, FILTER & SORTING (ORDER BY DI THEAD)
// ========================================================
const searchQuery = ref('')
const selectedCategory = ref('all') // 'all' | '>3000' | 'menantang' | 'pemula'
const selectedProvinsi = ref('all') // 'all' | 'Jawa Barat' | 'Jawa Tengah' | 'Jawa Timur' | 'Banten'

// Dropdown Options: Provinsi (Searchable)
const provinsiOptions = computed(() => {
  const counts = {}
  gunungList.value.forEach((g) => {
    counts[g.provinsi] = (counts[g.provinsi] || 0) + 1
  })

  return [
    { value: 'all', label: 'Semua Provinsi', badge: gunungList.value.length },
    { value: 'Jawa Timur', label: 'Jawa Timur', badge: counts['Jawa Timur'] || 0 },
    { value: 'Jawa Barat', label: 'Jawa Barat', badge: counts['Jawa Barat'] || 0 },
    { value: 'Jawa Tengah', label: 'Jawa Tengah', badge: counts['Jawa Tengah'] || 0 },
    { value: 'Banten', label: 'Banten', badge: counts['Banten'] || 0 },
    { value: 'Jawa Tengah dan Jawa Timur', label: 'Jateng & Jatim', badge: counts['Jawa Tengah dan Jawa Timur'] || 0 },
    { value: 'Banten dan Jawa Barat', label: 'Banten & Jabar', badge: counts['Banten dan Jawa Barat'] || 0 },
    { value: 'Yogyakarta', label: 'DI Yogyakarta', badge: counts['Jawa Tengah dan Daerah Istimewa Yogyakarta'] || 0 },
  ]
})

// Dropdown Options: Kategori (Soft Select)
const kategoriOptions = computed(() => {
  return [
    { value: 'all', label: 'Semua Kategori', badge: gunungList.value.length },
    { value: '>3000', label: '> 3.000 mdpl (Atap Jawa)', badge: gunungList.value.filter((g) => g.ketinggian >= 3000).length },
    { value: 'menantang', label: 'Menantang (2000-3000 mdpl)', badge: gunungList.value.filter((g) => g.kategori.includes('Menantang') || g.kategori.includes('Sedang')).length },
    { value: 'pemula', label: 'Ramah Pemula (< 2000 mdpl)', badge: gunungList.value.filter((g) => g.kategori.includes('Pemula') || g.kategori.includes('Wisata')).length },
  ]
})

// Sorting state (Default: Ketinggian Tertinggi ke Terendah)
const sortKey = ref('ketinggian')
const sortOrder = ref('desc') // 'asc' | 'desc'

// Pagination state
const currentPage = ref(1)
const itemsPerPage = ref(15) // 15 | 30 | 50 | 159 (semua)

const tableColumns = [
  { key: 'nama', label: 'Nama Gunung', sortable: true, cellClass: 'font-semibold text-neutral-900 dark:text-white whitespace-nowrap' },
  { key: 'ketinggian', label: 'Ketinggian', sortable: true, cellClass: 'whitespace-nowrap' },
  { key: 'provinsi', label: 'Provinsi', sortable: true, cellClass: 'whitespace-nowrap text-neutral-700 dark:text-neutral-300' },
  { key: 'wilayah', label: 'Wilayah / Lokasi', sortable: true, cellClass: 'text-neutral-600 dark:text-neutral-300 max-w-xs truncate' },
  { key: 'basecamp', label: 'Start / Basecamp', sortable: true, cellClass: 'text-neutral-600 dark:text-neutral-300 max-w-xs truncate' },
  { key: 'kategori', label: 'Kategori', sortable: true, cellClass: 'whitespace-nowrap' },
]

const onTableSort = ({ key, order }) => {
  currentPage.value = 1
  toast.info(
    `Urut berdasarkan ${getColumnLabel(key)} (${order === 'asc' ? 'A-Z / Rendah-Tinggi' : 'Z-A / Tinggi-Rendah'})`,
    'Sorting'
  )
}

const getColumnLabel = (key) => {
  const map = {
    nama: 'Nama Gunung',
    ketinggian: 'Ketinggian',
    provinsi: 'Provinsi',
    wilayah: 'Wilayah',
    basecamp: 'Basecamp',
    kategori: 'Kategori',
  }
  return map[key] || key
}

// Reset halaman saat filter / search berubah
watch([searchQuery, selectedCategory, selectedProvinsi, itemsPerPage], () => {
  currentPage.value = 1
})

// Data terfilter & terurut
const processedList = computed(() => {
  let list = [...gunungList.value]

  // Filter Kategori
  if (selectedCategory.value === '>3000') {
    list = list.filter((item) => item.ketinggian >= 3000)
  } else if (selectedCategory.value === 'pemula') {
    list = list.filter(
      (item) =>
        item.kategori.includes('Pemula') || item.kategori.includes('Wisata')
    )
  } else if (selectedCategory.value === 'menantang') {
    list = list.filter(
      (item) =>
        item.kategori.includes('Menantang') || item.kategori.includes('Sedang')
    )
  }

  // Filter Provinsi
  if (selectedProvinsi.value !== 'all') {
    list = list.filter((item) =>
      item.provinsi.toLowerCase().includes(selectedProvinsi.value.toLowerCase())
    )
  }

  // Filter Search
  if (searchQuery.value.trim()) {
    const q = searchQuery.value.toLowerCase()
    list = list.filter(
      (item) =>
        item.nama.toLowerCase().includes(q) ||
        item.wilayah.toLowerCase().includes(q) ||
        item.basecamp.toLowerCase().includes(q) ||
        item.provinsi.toLowerCase().includes(q) ||
        item.jalur.toLowerCase().includes(q)
    )
  }

  // Sorting
  list.sort((a, b) => {
    let aVal = a[sortKey.value]
    let bVal = b[sortKey.value]

    if (typeof aVal === 'string') {
      const cmp = aVal.localeCompare(bVal)
      return sortOrder.value === 'asc' ? cmp : -cmp
    }

    if (typeof aVal === 'number') {
      return sortOrder.value === 'asc' ? aVal - bVal : bVal - aVal
    }

    return 0
  })

  return list
})

// Data paginasi aktif
const totalPages = computed(() => {
  if (itemsPerPage.value === 'all') return 1
  return Math.max(1, Math.ceil(processedList.value.length / itemsPerPage.value))
})

const paginatedList = computed(() => {
  if (itemsPerPage.value === 'all') return processedList.value
  const start = (currentPage.value - 1) * itemsPerPage.value
  return processedList.value.slice(start, start + itemsPerPage.value)
})

// Kategori Badge Variant for BaseBadge
const getBadgeVariant = (kategori) => {
  if (kategori.includes('> 3.000')) return 'purple'
  if (kategori.includes('Sangat Menantang')) return 'danger'
  if (kategori.includes('Menantang')) return 'warning'
  if (kategori.includes('Pemula') || kategori.includes('Wisata')) return 'success'
  return 'info'
}

// Dropdown Action Items for Table Rows
const getRowActionItems = (item) => [
  {
    label: 'Lihat Detail',
    icon: Info,
    action: () => toast.info(`${item.nama} (${item.ketinggian.toLocaleString('id-ID')} mdpl) - Jalur: ${item.jalur}`, 'Detail Gunung'),
  },
  {
    label: 'Edit Data',
    icon: Pencil,
    action: () => openEditModal(item),
  },
  {
    label: 'Duplikasi',
    icon: Plus,
    action: () => {
      const copy = { ...item, id: Date.now(), nama: `${item.nama} (Salinan)` }
      gunungList.value.unshift(copy)
      toast.success(`Berhasil menduplikasi ${item.nama}`, 'Duplikasi')
    },
  },
  { divider: true },
  {
    label: 'Hapus Data',
    icon: Trash2,
    danger: true,
    action: () => openDeleteConfirm(item),
  },
]

// ========================================================
// 3. MODAL CRUD (TAMBAH & EDIT DENGAN MODAL)
// ========================================================
const isFormModalOpen = ref(false)
const isEditMode = ref(false)
// Dropdown Options: form tambah/edit gunung (tanpa opsi "Semua")
const formProvinsiOptions = [
  { value: 'Jawa Timur', label: 'Jawa Timur' },
  { value: 'Jawa Tengah', label: 'Jawa Tengah' },
  { value: 'Jawa Barat', label: 'Jawa Barat' },
  { value: 'Banten', label: 'Banten' },
  { value: 'DI Yogyakarta', label: 'DI Yogyakarta' },
]

const formKategoriOptions = [
  { value: '> 3.000 mdpl', label: '> 3.000 mdpl' },
  { value: 'Menantang', label: 'Menantang' },
  { value: 'Sedang', label: 'Sedang' },
  { value: 'Ramah Pemula', label: 'Ramah Pemula' },
  { value: 'Wisata Sunrise', label: 'Wisata Sunrise' },
]

const formData = ref({
  id: null,
  nama: '',
  ketinggian: '',
  provinsi: 'Jawa Tengah',
  wilayah: '',
  basecamp: '',
  jalur: '',
  kategori: 'Ramah Pemula',
})
const formErrors = ref({})

const openAddModal = () => {
  isEditMode.value = false
  formData.value = {
    id: null,
    nama: '',
    ketinggian: '',
    provinsi: 'Jawa Tengah',
    wilayah: '',
    basecamp: '',
    jalur: '',
    kategori: 'Ramah Pemula',
  }
  formErrors.value = {}
  isFormModalOpen.value = true
}

const openEditModal = (item) => {
  isEditMode.value = true
  formData.value = { ...item }
  formErrors.value = {}
  isFormModalOpen.value = true
}

const validateForm = () => {
  const errors = {}
  if (!formData.value.nama.trim()) errors.nama = 'Nama gunung wajib diisi'
  if (!formData.value.ketinggian || Number(formData.value.ketinggian) <= 0) {
    errors.ketinggian = 'Ketinggian harus berupa angka positif'
  }
  if (!formData.value.wilayah.trim()) errors.wilayah = 'Wilayah/lokasi wajib diisi'
  if (!formData.value.basecamp.trim()) errors.basecamp = 'Basecamp awal wajib diisi'
  formErrors.value = errors
  return Object.keys(errors).length === 0
}

const handleSaveForm = async () => {
  if (!validateForm()) {
    toast.error('Harap lengkapi semua bidang yang wajib diisi')
    return
  }

  const payload = {
    ...formData.value,
    ketinggian: Number(formData.value.ketinggian),
  }

  isFormModalOpen.value = false

  if (isEditMode.value) {
    await toast.promise(
      new Promise((resolve) => {
        setTimeout(() => {
          const idx = gunungList.value.findIndex((g) => g.id === payload.id)
          if (idx !== -1) {
            gunungList.value[idx] = payload
          }
          resolve(payload)
        }, 800)
      }),
      {
        loading: 'Menyimpan perubahan...',
        success: `Data ${payload.nama} berhasil diperbarui`,
        error: 'Gagal memperbarui data gunung',
      }
    )
  } else {
    await toast.promise(
      new Promise((resolve) => {
        setTimeout(() => {
          payload.id = Date.now()
          gunungList.value.unshift(payload)
          resolve(payload)
        }, 800)
      }),
      {
        loading: 'Menambahkan data gunung...',
        success: `Gunung ${payload.nama} berhasil ditambahkan!`,
        error: 'Gagal menambahkan data gunung',
      }
    )
  }
}

// ========================================================
// 4. DELETE MODAL & CONFIRMATION
// ========================================================
const isDeleteModalOpen = ref(false)
const itemToDelete = ref(null)

const openDeleteConfirm = (item) => {
  itemToDelete.value = item
  isDeleteModalOpen.value = true
}

const handleDeleteConfirm = () => {
  if (!itemToDelete.value) return
  const deletedItem = itemToDelete.value
  gunungList.value = gunungList.value.filter((g) => g.id !== deletedItem.id)
  isDeleteModalOpen.value = false
  itemToDelete.value = null

  toast.error(`Data ${deletedItem.nama} berhasil dihapus dari daftar.`, 'Data Dihapus')
}

// Reset Default
const handleResetDefault = () => {
  gunungList.value = [...initialGunungList]
  searchQuery.value = ''
  selectedCategory.value = 'all'
  selectedProvinsi.value = 'all'
  sortKey.value = 'ketinggian'
  sortOrder.value = 'desc'
  currentPage.value = 1
  itemsPerPage.value = 15
  toast.success('Daftar gunung dikembalikan ke 159 data awal Pulau Jawa', 'Data Direset')
}
</script>

<template>
  <!-- Main View Container: h-full flex flex-col min-h-0 overflow-hidden agar Layer 2 tidak pernah scroll -->
  <div class="h-full flex flex-col min-h-0 overflow-hidden gap-2.5 w-full max-w-7xl mx-auto select-none relative">
    
    <!-- Top Compact Header: Title & Action Buttons -->
    <div class="flex items-center justify-between gap-3 pb-2 border-b border-neutral-200 dark:border-[var(--theme-border)] shrink-0">
      <div class="flex items-center gap-2.5 min-w-0">
        <div class="w-8 h-8 rounded-xl bg-neutral-900 text-white dark:bg-white dark:text-neutral-950 flex items-center justify-center shrink-0 shadow-2xs">
          <Mountain class="w-4 h-4" />
        </div>
        <div class="min-w-0">
          <div class="flex items-center gap-2">
            <h1 class="text-base sm:text-lg font-bold text-neutral-900 dark:text-[var(--theme-text-primary)] leading-none truncate">
              Data Gunung di Pulau Jawa
            </h1>
            <span class="text-[10px] font-semibold text-emerald-600 dark:text-emerald-400 bg-emerald-50 dark:bg-emerald-950/40 px-2 py-0.5 rounded-full border border-emerald-200/60 dark:border-emerald-800/40 shrink-0">
              159 Data
            </span>
          </div>
          <p class="text-[11px] text-neutral-500 dark:text-neutral-400 mt-0.5 truncate">
            Diurutkan dari tertinggi (Semeru 3.676 mdpl) ke terendah
          </p>
        </div>
      </div>

      <!-- Action Buttons -->
      <div class="flex items-center gap-2 shrink-0">
        <BaseButton variant="outline" size="sm" @click="handleResetDefault" title="Kembalikan ke data awal 159 gunung">
          <RotateCcw class="w-3.5 h-3.5 text-neutral-500" />
          <span class="hidden sm:inline">Reset</span>
        </BaseButton>

        <BaseButton variant="primary" size="sm" @click="openAddModal">
          <Plus class="w-3.5 h-3.5" />
          <span>Tambah</span>
        </BaseButton>
      </div>
    </div>

    <!-- ======================================================== -->
    <!-- BARIS 1 (SESUAI WIREFRAME): [SEARCH] [FILTER 1] [FILTER 2] -->
    <!-- ======================================================== -->
    <div class="flex flex-col sm:flex-row items-stretch sm:items-center gap-2 shrink-0">
      <!-- Search Input -->
      <div class="relative flex-1">
        <Search class="w-4 h-4 text-neutral-400 absolute left-3 top-1/2 -translate-y-1/2 pointer-events-none" />
        <input
          v-model="searchQuery"
          type="text"
          placeholder="Cari dari 159 gunung, wilayah, basecamp..."
          class="w-full pl-9 pr-8 py-2 rounded-xl text-xs bg-white dark:bg-[var(--theme-surface-raised)] border border-neutral-200 dark:border-[var(--theme-border)] text-neutral-900 dark:text-white placeholder:text-neutral-400 focus:outline-none focus:ring-2 focus:ring-neutral-400 dark:focus:ring-[var(--theme-border-strong)] transition shadow-2xs"
        />
        <button
          v-if="searchQuery"
          type="button"
          @click="searchQuery = ''"
          class="absolute right-2.5 top-1/2 -translate-y-1/2 p-0.5 text-neutral-400 hover:text-neutral-700 dark:hover:text-neutral-200 cursor-pointer"
        >
          <X class="w-3.5 h-3.5" />
        </button>
      </div>

      <!-- FILTER 1: Dropdown Searchable Provinsi (Soft UI Component) -->
      <div class="w-full sm:w-52 shrink-0">
        <BaseSearchableSelect
          v-model="selectedProvinsi"
          :options="provinsiOptions"
          placeholder="Semua Provinsi"
          search-placeholder="Cari provinsi..."
          :leading-icon="MapPin"
          clear-value="all"
          clearable
        />
      </div>

      <!-- FILTER 2: Dropdown Kategori -->
      <div class="w-full sm:w-56 shrink-0">
        <BaseSearchableSelect
          v-model="selectedCategory"
          :options="kategoriOptions"
          placeholder="Semua Kategori"
          :searchable="false"
          :leading-icon="Flame"
          align="right"
          clear-value="all"
        />
      </div>
    </div>

    <!-- Garis Pembatas (Sesuai Wireframe) -->
    <!-- <div class="border-b border-neutral-200 dark:border-[var(--theme-border)] shrink-0" /> -->

    <!-- ======================================================== -->
    <!-- BARIS 2 (SESUAI WIREFRAME): 4 COMPACT ICON STAT PILLS   -->
    <!-- [ICON TOTAL 1] [ICON TOTAL 2] [ICON TOTAL 3] [ICON TOTAL 4]-->
    <!-- ======================================================== -->
    <div class="grid grid-cols-2 lg:grid-cols-4 gap-2 shrink-0">
      <!-- Pill 1: Total Gunung -->
      <div class="h-9 px-2.5 rounded-xl bg-white dark:bg-[var(--theme-surface)] border border-neutral-200 dark:border-[var(--theme-border)] shadow-2xs flex items-center gap-2">
        <div class="w-6 h-6 rounded-lg bg-neutral-100 dark:bg-[var(--theme-surface-muted)] text-neutral-700 dark:text-neutral-300 flex items-center justify-center shrink-0">
          <Mountain class="w-3.5 h-3.5" />
        </div>
        <div class="flex items-center gap-1.5 min-w-0 truncate text-xs">
          <span class="text-[10px] uppercase font-bold text-neutral-400 tracking-wider whitespace-nowrap">Total:</span>
          <span class="font-bold text-neutral-900 dark:text-white truncate">{{ gunungList.length }} Gunung</span>
        </div>
      </div>

      <!-- Pill 2: Tertinggi (Semeru) -->
      <div class="h-9 px-2.5 rounded-xl bg-white dark:bg-[var(--theme-surface)] border border-neutral-200 dark:border-[var(--theme-border)] shadow-2xs flex items-center gap-2">
        <div class="w-6 h-6 rounded-lg bg-purple-50 dark:bg-purple-950/40 text-purple-600 dark:text-purple-400 flex items-center justify-center shrink-0">
          <TrendingUp class="w-3.5 h-3.5" />
        </div>
        <div class="flex items-center gap-1.5 min-w-0 truncate text-xs">
          <span class="text-[10px] uppercase font-bold text-purple-400/80 tracking-wider whitespace-nowrap">Tertinggi:</span>
          <span class="font-bold text-purple-600 dark:text-purple-400 truncate">Semeru 3.676 mdpl</span>
        </div>
      </div>

      <!-- Pill 3: Puncak > 3.000 mdpl -->
      <div class="h-9 px-2.5 rounded-xl bg-white dark:bg-[var(--theme-surface)] border border-neutral-200 dark:border-[var(--theme-border)] shadow-2xs flex items-center gap-2">
        <div class="w-6 h-6 rounded-lg bg-rose-50 dark:bg-rose-950/40 text-rose-600 dark:text-rose-400 flex items-center justify-center shrink-0">
          <Flame class="w-3.5 h-3.5" />
        </div>
        <div class="flex items-center gap-1.5 min-w-0 truncate text-xs">
          <span class="text-[10px] uppercase font-bold text-rose-400/80 tracking-wider whitespace-nowrap">&gt; 3.000m:</span>
          <span class="font-bold text-rose-600 dark:text-rose-400 truncate">{{ gunungList.filter(g => g.ketinggian >= 3000).length }} Puncak</span>
        </div>
      </div>

      <!-- Pill 4: Ramah Pemula & Wisata -->
      <div class="h-9 px-2.5 rounded-xl bg-white dark:bg-[var(--theme-surface)] border border-neutral-200 dark:border-[var(--theme-border)] shadow-2xs flex items-center gap-2">
        <div class="w-6 h-6 rounded-lg bg-emerald-50 dark:bg-emerald-950/40 text-emerald-600 dark:text-emerald-400 flex items-center justify-center shrink-0">
          <Trees class="w-3.5 h-3.5" />
        </div>
        <div class="flex items-center gap-1.5 min-w-0 truncate text-xs">
          <span class="text-[10px] uppercase font-bold text-emerald-400/80 tracking-wider whitespace-nowrap">Pemula:</span>
          <span class="font-bold text-emerald-600 dark:text-emerald-400 truncate">{{ gunungList.filter(g => g.kategori.includes('Pemula') || g.kategori.includes('Wisata')).length }} Gunung</span>
        </div>
      </div>
    </div>

    <!-- ======================================================== -->
    <!-- BASE DATA TABLE (DESKTOP + MOBILE)                       -->
    <!-- ======================================================== -->
    <BaseDataTable
      :columns="tableColumns"
      :rows="paginatedList"
      :total-items="processedList.length"
      v-model:sort-key="sortKey"
      v-model:sort-order="sortOrder"
      v-model:current-page="currentPage"
      v-model:items-per-page="itemsPerPage"
      compact-mobile-pagination
      :empty-icon="Mountain"
      empty-title="Tidak ada data gunung yang cocok"
      empty-description="Coba ubah kata kunci pencarian, kategori ketinggian, atau provinsi yang dipilih."
      empty-action-text="Bersihkan Filter"
      @sort="onTableSort"
      @empty-action="searchQuery = ''; selectedCategory = 'all'; selectedProvinsi = 'all'"
    >
      <!-- Nama Gunung -->
      <template #cell-nama="{ row }">
        <div class="flex items-center gap-2">
          <span
            class="w-2 h-2 rounded-full shrink-0"
            :class="row.ketinggian >= 3000 ? 'bg-purple-500' : row.ketinggian >= 2000 ? 'bg-amber-500' : 'bg-emerald-500'"
          />
          <span>{{ row.nama }}</span>
        </div>
      </template>

      <!-- Ketinggian -->
      <template #cell-ketinggian="{ row }">
        <span class="font-bold text-neutral-900 dark:text-neutral-100 font-mono bg-neutral-100 dark:bg-[var(--theme-surface-muted)] px-2 py-0.5 rounded-lg border border-neutral-200/80 dark:border-[var(--theme-border-strong)]">
          {{ row.ketinggian.toLocaleString('id-ID') }} mdpl
        </span>
      </template>

      <!-- Provinsi -->
      <template #cell-provinsi="{ row }">
        <span class="font-medium">{{ row.provinsi }}</span>
      </template>

      <!-- Wilayah -->
      <template #cell-wilayah="{ row }">
        <div class="flex items-center gap-1.5 truncate" :title="row.wilayah">
          <MapPin class="w-3.5 h-3.5 text-neutral-400 shrink-0" />
          <span class="truncate">{{ row.wilayah }}</span>
        </div>
      </template>

      <!-- Basecamp -->
      <template #cell-basecamp="{ row }">
        <div class="flex items-center gap-1.5 truncate" :title="row.basecamp">
          <Compass class="w-3.5 h-3.5 text-neutral-400 shrink-0" />
          <span class="truncate">{{ row.basecamp }}</span>
        </div>
      </template>

      <!-- Kategori -->
      <template #cell-kategori="{ row }">
        <BaseBadge :variant="getBadgeVariant(row.kategori)">
          {{ row.kategori }}
        </BaseBadge>
      </template>

      <!-- Aksi: Edit & Dropdown Action Menu -->
      <template #actions="{ row }">
        <div class="flex items-center justify-end gap-1">
          <button
            type="button"
            @click="openEditModal(row)"
            class="p-1.5 rounded-lg text-neutral-600 hover:text-neutral-900 hover:bg-neutral-100 dark:text-neutral-300 dark:hover:text-white dark:hover:bg-[var(--theme-surface-muted)] transition cursor-pointer"
            title="Edit Gunung"
          >
            <Pencil class="w-3.5 h-3.5" />
          </button>
          <BaseDropdownMenu :items="getRowActionItems(row)" align="right" />
        </div>
      </template>

      <!-- Kartu Mobile -->
      <template #mobile-card="{ row, rowNumber }">
        <div
          class="p-3.5 rounded-2xl bg-white dark:bg-[var(--theme-surface)] border border-neutral-200 dark:border-[var(--theme-border)] shadow-xs space-y-2.5"
        >
          <!-- Card Top Bar: No, Nama, & Ketinggian Badge -->
          <div class="flex items-start justify-between gap-2">
            <div class="flex items-center gap-2">
              <span class="w-6 h-6 rounded-lg bg-neutral-100 dark:bg-[var(--theme-surface-muted)] text-neutral-700 dark:text-neutral-300 font-bold text-xs flex items-center justify-center shrink-0">
                #{{ rowNumber }}
              </span>
              <div>
                <h3 class="text-sm font-bold text-neutral-900 dark:text-white leading-tight">
                  {{ row.nama }}
                </h3>
                <div class="flex items-center gap-1.5 mt-0.5">
                  <span class="text-[10px] text-neutral-500 font-medium">
                    {{ row.provinsi }}
                  </span>
                  <span class="text-neutral-300">&bull;</span>
                  <BaseBadge :variant="getBadgeVariant(row.kategori)" size="sm">
                    {{ row.kategori }}
                  </BaseBadge>
                </div>
              </div>
            </div>

            <span class="font-bold text-xs text-neutral-900 dark:text-neutral-100 font-mono bg-neutral-100 dark:bg-[var(--theme-surface-muted)] px-2 py-0.5 rounded-lg border border-neutral-200/80 dark:border-[var(--theme-border-strong)] shrink-0">
              {{ row.ketinggian.toLocaleString('id-ID') }} mdpl
            </span>
          </div>

          <!-- Card Detail Grid Compact -->
          <div class="p-2.5 rounded-xl bg-neutral-50 dark:bg-[var(--theme-surface-raised)] text-xs space-y-1.5 border border-neutral-100 dark:border-[var(--theme-border)]">
            <div class="flex items-start gap-2 text-neutral-700 dark:text-neutral-300">
              <MapPin class="w-3.5 h-3.5 text-neutral-400 shrink-0 mt-0.5" />
              <div class="min-w-0">
                <span class="text-[9.5px] text-neutral-400 block uppercase">Wilayah</span>
                <span class="leading-snug block truncate">{{ row.wilayah }}</span>
              </div>
            </div>

            <div class="flex items-start gap-2 text-neutral-700 dark:text-neutral-300">
              <Compass class="w-3.5 h-3.5 text-neutral-400 shrink-0 mt-0.5" />
              <div class="min-w-0">
                <span class="text-[9.5px] text-neutral-400 block uppercase">Basecamp</span>
                <span class="leading-snug block truncate">{{ row.basecamp }}</span>
              </div>
            </div>
          </div>

          <!-- Card Actions: Edit & Hapus -->
          <div class="grid grid-cols-2 gap-2 pt-1 border-t border-neutral-100 dark:border-[var(--theme-border)]">
            <button
              type="button"
              @click="openEditModal(row)"
              class="w-full py-1.5 px-3 rounded-xl text-xs font-semibold border border-neutral-200 dark:border-[var(--theme-border)] bg-white hover:bg-neutral-50 dark:bg-[var(--theme-surface-raised)] dark:hover:bg-[var(--theme-surface-muted)] text-neutral-700 dark:text-neutral-200 transition flex items-center justify-center gap-1.5 cursor-pointer"
            >
              <Pencil class="w-3.5 h-3.5" />
              <span>Edit</span>
            </button>

            <button
              type="button"
              @click="openDeleteConfirm(row)"
              class="w-full py-1.5 px-3 rounded-xl text-xs font-semibold border border-rose-200/80 dark:border-rose-900/40 bg-rose-50/60 hover:bg-rose-100/80 dark:bg-rose-950/20 dark:hover:bg-rose-950/40 text-rose-600 dark:text-rose-400 transition flex items-center justify-center gap-1.5 cursor-pointer"
            >
              <Trash2 class="w-3.5 h-3.5" />
              <span>Hapus</span>
            </button>
          </div>
        </div>
      </template>
    </BaseDataTable>

    <!-- ======================================================== -->
    <!-- MODAL FORM: TAMBAH & EDIT GUNUNG                         -->
    <!-- ======================================================== -->
    <BaseModal
      v-model="isFormModalOpen"
      :title="isEditMode ? 'Edit Data Gunung' : 'Tambah Data Gunung'"
      :subtitle="isEditMode ? 'Perbarui informasi jalur dan basecamp gunung' : 'Masukkan informasi gunung baru di Pulau Jawa'"
      size="md"
    >
      <form @submit.prevent="handleSaveForm" class="space-y-3.5 py-1">
        <!-- Nama Gunung -->
        <BaseInput
          v-model="formData.nama"
          label="Nama Gunung"
          required
          placeholder="Contoh: Gunung Semeru"
          :error="formErrors.nama"
        />

        <!-- Ketinggian, Provinsi & Kategori Grid -->
        <div class="grid grid-cols-1 sm:grid-cols-3 gap-3">
          <BaseInput
            v-model="formData.ketinggian"
            type="number"
            label="Ketinggian"
            required
            suffix="mdpl"
            placeholder="Contoh: 3676"
            :error="formErrors.ketinggian"
          />

          <BaseFormField label="Provinsi">
            <BaseSearchableSelect
              v-model="formData.provinsi"
              :options="formProvinsiOptions"
              :searchable="false"
              size="md"
            />
          </BaseFormField>

          <BaseFormField label="Kategori">
            <BaseSearchableSelect
              v-model="formData.kategori"
              :options="formKategoriOptions"
              :searchable="false"
              size="md"
              align="right"
            />
          </BaseFormField>
        </div>

        <!-- Wilayah / Kabupaten -->
        <BaseInput
          v-model="formData.wilayah"
          label="Wilayah / Kabupaten"
          required
          placeholder="Contoh: Malang & Lumajang"
          :error="formErrors.wilayah"
        />

        <!-- Basecamp & Jalur -->
        <div class="grid grid-cols-1 sm:grid-cols-2 gap-3">
          <BaseInput
            v-model="formData.basecamp"
            label="Start / Basecamp Awal"
            required
            placeholder="Contoh: Ranupani"
            :error="formErrors.basecamp"
          />

          <BaseInput
            v-model="formData.jalur"
            label="Jalur Pendakian Utama"
            placeholder="Contoh: Jalur Ranupani"
          />
        </div>
      </form>

      <template #footer="{ close }">
        <BaseButton variant="outline" size="sm" @click="close">
          Batal
        </BaseButton>
        <BaseButton variant="primary" size="sm" @click="handleSaveForm">
          <Check class="w-3.5 h-3.5" />
          <span>{{ isEditMode ? 'Simpan Perubahan' : 'Simpan Data' }}</span>
        </BaseButton>
      </template>
    </BaseModal>

    <!-- ======================================================== -->
    <!-- CONFIRMATION MODAL: HAPUS DATA GUNUNG                     -->
    <!-- ======================================================== -->
    <ConfirmationModal
      v-model="isDeleteModalOpen"
      type="delete"
      badge-text="Confirmation — Hapus Data"
      :title="`Hapus ${itemToDelete?.nama || 'Gunung'}?`"
      :message="`Data gunung ${itemToDelete?.nama || ''} (${itemToDelete?.ketinggian?.toLocaleString('id-ID') || ''} mdpl - ${itemToDelete?.provinsi || ''}) akan dihapus secara permanen dari daftar.`"
      confirm-text="Ya, Hapus"
      cancel-text="Batal"
      @confirm="handleDeleteConfirm"
      @cancel="itemToDelete = null"
    />
  </div>
</template>
