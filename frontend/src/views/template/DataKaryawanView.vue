<script setup>
import { ref, computed, watch } from 'vue'
import BaseButton from '../../components/ui/BaseButton.vue'
import BaseModal from '../../components/ui/BaseModal.vue'
import ConfirmationModal from '../../components/ui/ConfirmationModal.vue'
import BaseSearchableSelect from '../../components/ui/BaseSearchableSelect.vue'
import BaseInput from '../../components/ui/BaseInput.vue'
import BaseCurrencyInput from '../../components/ui/BaseCurrencyInput.vue'
import BaseDatePicker from '../../components/ui/BaseDatePicker.vue'
import BaseEmptyState from '../../components/ui/BaseEmptyState.vue'
import BaseTextarea from '../../components/ui/BaseTextarea.vue'
import BaseSwitch from '../../components/ui/BaseSwitch.vue'
import BaseBadge from '../../components/ui/BaseBadge.vue'
import BasePagination from '../../components/ui/BasePagination.vue'
import BaseDropdownMenu from '../../components/ui/BaseDropdownMenu.vue'
import BaseSlideOver from '../../components/ui/BaseSlideOver.vue'
import BaseTimeline from '../../components/ui/BaseTimeline.vue'
import BaseCsvImporter from '../../components/ui/BaseCsvImporter.vue'
import BaseFileUploader from '../../components/ui/BaseFileUploader.vue'
import BaseCheckbox from '../../components/ui/BaseCheckbox.vue'
import BaseMultiSelect from '../../components/ui/BaseMultiSelect.vue'
import BaseDataTable from '../../components/ui/BaseDataTable.vue'
import BaseFormField from '../../components/ui/BaseFormField.vue'
import BaseAvatar from '../../components/ui/BaseAvatar.vue'
import BaseDescriptionList from '../../components/ui/BaseDescriptionList.vue'
import BaseStepper from '../../components/ui/BaseStepper.vue'
import { useToast } from '../../composables/useToast'
import { karyawanList } from '../../data/karyawanData'
import { formatRupiah } from '../../utils/format.js'
import {
  Users,
  Plus,
  Pencil,
  Trash2,
  Search,
  ArrowUpDown,
  ArrowUp,
  ArrowDown,
  RotateCcw,
  MapPin,
  Check,
  X,
  TrendingUp,
  Coins,
  CheckCircle2,
  Clock,
  Briefcase,
  FileText,
  Info,
  Building2,
  CreditCard,
  Phone,
  Mail,
  ShieldCheck,
  Download,
  CheckSquare,
  Upload,
  Eye,
} from 'lucide-vue-next'

const toast = useToast()

// ========================================================
// 1. DATA STATE & CRUD
// ========================================================
const initialKaryawan = karyawanList
const listKaryawan = ref([...karyawanList])

// ========================================================
// 2. SEARCH, FILTER & SORTING
// ========================================================
const searchQuery = ref('')
const selectedDivisi = ref('all')
const selectedStatus = ref('all')

const sortKey = ref('gaji')
const sortOrder = ref('desc')

const currentPage = ref(1)
const itemsPerPage = ref(10)

// Multi-select & Batch Actions State
const selectedIds = ref([])
const isBatchDeleteModalOpen = ref(false)

// Dropdown Options: form tambah/edit karyawan (tanpa opsi "Semua")
const formDivisiOptions = [
  { value: 'Teknologi & IT', label: 'Teknologi & IT' },
  { value: 'Desain & Kreatif', label: 'Desain & Kreatif' },
  { value: 'Keuangan & Pajak', label: 'Keuangan & Pajak' },
  { value: 'SDM & HR', label: 'SDM & HR' },
  { value: 'Pemasaran & Bisnis', label: 'Pemasaran & Bisnis' },
  { value: 'Operasional', label: 'Operasional' },
]

const formStatusOptions = [
  { value: 'Tetap', label: 'Tetap' },
  { value: 'Kontrak', label: 'Kontrak' },
  { value: 'Probasi', label: 'Probasi' },
  { value: 'Freelance', label: 'Freelance' },
]

// Opsi Multi-Select Fasilitas & Benefit Pegawai
const formFasilitasOptions = [
  { value: 'BPJS Kesehatan', label: 'BPJS Kesehatan', badge: 'Wajib' },
  { value: 'BPJS Ketenagakerjaan', label: 'BPJS Ketenagakerjaan', badge: 'Wajib' },
  { value: 'Asuransi Swasta', label: 'Asuransi Kesehatan Swasta' },
  { value: 'Laptop Kerja', label: 'Fasilitas Laptop / MacBook' },
  { value: 'Tunjangan Transport', label: 'Tunjangan Transport & Makan' },
  { value: 'Work From Anywhere', label: 'Fleksibilitas WFA / Hybrid' },
]

// Konfigurasi Kolom BaseDataTable
const tableColumns = [
  { key: 'nama', label: 'Pegawai', sortable: true, cellClass: 'whitespace-nowrap' },
  { key: 'divisi', label: 'Jabatan & Divisi', sortable: true, cellClass: 'whitespace-nowrap' },
  { key: 'gaji', label: 'Gaji Pokok', sortable: true, cellClass: 'whitespace-nowrap' },
  { key: 'status', label: 'Status', sortable: true, cellClass: 'whitespace-nowrap' },
  { key: 'lokasi', label: 'Lokasi', sortable: false, cellClass: 'whitespace-nowrap text-neutral-600 dark:text-neutral-400' },
  { key: 'bpjsAktif', label: 'Asuransi', sortable: false, align: 'center', cellClass: 'whitespace-nowrap' },
]

// Dropdown Options: Divisi (Searchable)
const divisiOptions = computed(() => {
  const counts = {}
  listKaryawan.value.forEach((k) => {
    counts[k.divisi] = (counts[k.divisi] || 0) + 1
  })

  return [
    { value: 'all', label: 'Semua Divisi', badge: listKaryawan.value.length },
    { value: 'Teknologi & IT', label: 'Teknologi & IT', badge: counts['Teknologi & IT'] || 0 },
    { value: 'Desain & Kreatif', label: 'Desain & Kreatif', badge: counts['Desain & Kreatif'] || 0 },
    { value: 'Keuangan & Pajak', label: 'Keuangan & Pajak', badge: counts['Keuangan & Pajak'] || 0 },
    { value: 'SDM & HR', label: 'SDM & HR', badge: counts['SDM & HR'] || 0 },
    { value: 'Pemasaran & Bisnis', label: 'Pemasaran & Bisnis', badge: counts['Pemasaran & Bisnis'] || 0 },
    { value: 'Operasional', label: 'Operasional', badge: counts['Operasional'] || 0 },
  ]
})

// Dropdown Options: Status Kerja
const statusOptions = computed(() => {
  const counts = {}
  listKaryawan.value.forEach((k) => {
    counts[k.status] = (counts[k.status] || 0) + 1
  })

  return [
    { value: 'all', label: 'Semua Status', badge: listKaryawan.value.length },
    { value: 'Tetap', label: 'Karyawan Tetap', badge: counts['Tetap'] || 0 },
    { value: 'Kontrak', label: 'Kontrak', badge: counts['Kontrak'] || 0 },
    { value: 'Probasi', label: 'Probasi', badge: counts['Probasi'] || 0 },
    { value: 'Freelance', label: 'Freelance', badge: counts['Freelance'] || 0 },
  ]
})

const handleSort = (key) => {
  if (sortKey.value === key) {
    sortOrder.value = sortOrder.value === 'asc' ? 'desc' : 'asc'
  } else {
    sortKey.value = key
    sortOrder.value = key === 'gaji' ? 'desc' : 'asc'
  }
  currentPage.value = 1
}

watch([searchQuery, selectedDivisi, selectedStatus, itemsPerPage], () => {
  currentPage.value = 1
})

// Filtered & Sorted List
const processedList = computed(() => {
  let list = [...listKaryawan.value]

  // Filter Divisi
  if (selectedDivisi.value && selectedDivisi.value !== 'all') {
    list = list.filter((k) => k.divisi === selectedDivisi.value)
  }

  // Filter Status
  if (selectedStatus.value && selectedStatus.value !== 'all') {
    list = list.filter((k) => k.status === selectedStatus.value)
  }

  // Search
  if (searchQuery.value.trim()) {
    const q = searchQuery.value.toLowerCase().trim()
    list = list.filter(
      (k) =>
        k.nama.toLowerCase().includes(q) ||
        k.nip.toLowerCase().includes(q) ||
        k.jabatan.toLowerCase().includes(q) ||
        k.email.toLowerCase().includes(q) ||
        k.lokasi.toLowerCase().includes(q) ||
        k.divisi.toLowerCase().includes(q)
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

const totalPages = computed(() => {
  if (itemsPerPage.value === 'all') return 1
  return Math.max(1, Math.ceil(processedList.value.length / Number(itemsPerPage.value)))
})

const paginatedList = computed(() => {
  if (itemsPerPage.value === 'all') return processedList.value
  const start = (currentPage.value - 1) * Number(itemsPerPage.value)
  return processedList.value.slice(start, start + Number(itemsPerPage.value))
})

// Multi-select Logic
const isAllSelected = computed(() => {
  if (paginatedList.value.length === 0) return false
  return paginatedList.value.every((k) => selectedIds.value.includes(k.id))
})

const toggleSelectAll = () => {
  if (isAllSelected.value) {
    const pageIds = paginatedList.value.map((k) => k.id)
    selectedIds.value = selectedIds.value.filter((id) => !pageIds.includes(id))
  } else {
    const pageIds = paginatedList.value.map((k) => k.id)
    const set = new Set([...selectedIds.value, ...pageIds])
    selectedIds.value = Array.from(set)
  }
}

const toggleSelect = (id) => {
  const idx = selectedIds.value.indexOf(id)
  if (idx !== -1) {
    selectedIds.value.splice(idx, 1)
  } else {
    selectedIds.value.push(id)
  }
}

// Stats Calculations
const totalPayroll = computed(() => {
  return listKaryawan.value.reduce((acc, k) => acc + (k.gaji || 0) + (k.tunjangan || 0), 0)
})

const totalTetap = computed(() => {
  return listKaryawan.value.filter((k) => k.status === 'Tetap').length
})

const totalKontrak = computed(() => {
  return listKaryawan.value.filter((k) => k.status === 'Kontrak' || k.status === 'Probasi').length
})

// Status Badge Variant for BaseBadge
const getStatusBadgeVariant = (status) => {
  const map = {
    Tetap: 'success',
    Kontrak: 'warning',
    Probasi: 'info',
    Freelance: 'purple',
  }
  return map[status] || 'neutral'
}

// ========================================================
// 3. EXPORT CSV & BATCH ACTIONS
// ========================================================
const exportToCSV = (items = listKaryawan.value, filename = 'data-pegawai-digaji.csv') => {
  if (!items || items.length === 0) {
    toast.error('Tidak ada data untuk diekspor')
    return
  }

  const headers = ['NIP', 'Nama Pegawai', 'Email', 'No Telepon', 'Divisi', 'Jabatan', 'Gaji Pokok', 'Tunjangan', 'Status', 'Lokasi', 'Tanggal Masuk', 'BPJS']
  const rows = items.map((k) => [
    `"${k.nip || ''}"`,
    `"${k.nama || ''}"`,
    `"${k.email || ''}"`,
    `"${k.telepon || ''}"`,
    `"${k.divisi || ''}"`,
    `"${k.jabatan || ''}"`,
    k.gaji || 0,
    k.tunjangan || 0,
    `"${k.status || ''}"`,
    `"${k.lokasi || ''}"`,
    `"${k.tanggalMasuk || ''}"`,
    k.bpjsAktif ? 'Aktif' : 'Tidak',
  ])

  const csvContent = '\uFEFF' + [headers.join(','), ...rows.map((r) => r.join(','))].join('\r\n')
  const blob = new Blob([csvContent], { type: 'text/csv;charset=utf-8;' })
  const url = URL.createObjectURL(blob)
  const link = document.createElement('a')
  link.setAttribute('href', url)
  link.setAttribute('download', filename)
  document.body.appendChild(link)
  link.click()
  document.body.removeChild(link)
  URL.revokeObjectURL(url)

  toast.success(`Berhasil mengunduh ${items.length} data pegawai ke file CSV!`, 'Ekspor CSV')
}

const exportSelected = () => {
  const items = listKaryawan.value.filter((k) => selectedIds.value.includes(k.id))
  exportToCSV(items, `data-pegawai-terpilih-${items.length}.csv`)
}

const handleBatchDeleteConfirm = () => {
  const count = selectedIds.value.length
  listKaryawan.value = listKaryawan.value.filter((k) => !selectedIds.value.includes(k.id))
  selectedIds.value = []
  isBatchDeleteModalOpen.value = false
  toast.error(`${count} pegawai terpilih telah dihapus dari sistem.`, 'Hapus Massal')
}

// ========================================================
// 4. MODAL CRUD (TAMBAH & EDIT PEGAWAI)
// ========================================================
const isFormModalOpen = ref(false)
const isEditMode = ref(false)
const formData = ref({
  id: null,
  nip: '',
  nama: '',
  email: '',
  telepon: '',
  divisi: 'Teknologi & IT',
  jabatan: '',
  gaji: '',
  tunjangan: '',
  status: 'Tetap',
  lokasi: 'Jakarta Selatan',
  tanggalMasuk: '2024-01-01',
  bpjsAktif: true,
  fasilitas: ['BPJS Kesehatan', 'BPJS Ketenagakerjaan', 'Laptop Kerja'],
  rekening: '',
  catatan: '',
})
const formErrors = ref({})

const openAddModal = () => {
  isEditMode.value = false
  const nextNum = String(listKaryawan.value.length + 1).padStart(3, '0')
  formData.value = {
    id: null,
    nip: `EMP-2024-${nextNum}`,
    nama: '',
    email: '',
    telepon: '',
    divisi: 'Teknologi & IT',
    jabatan: '',
    gaji: '',
    tunjangan: '',
    status: 'Tetap',
    lokasi: 'Jakarta Selatan',
    tanggalMasuk: '2024-01-01',
    bpjsAktif: true,
    fasilitas: ['BPJS Kesehatan', 'BPJS Ketenagakerjaan', 'Laptop Kerja'],
    rekening: 'BCA - ',
    catatan: '',
  }
  formErrors.value = {}
  isFormModalOpen.value = true
}

const openEditModal = (item) => {
  isEditMode.value = true
  formData.value = {
    ...item,
    fasilitas: Array.isArray(item.fasilitas)
      ? [...item.fasilitas]
      : ['BPJS Kesehatan', 'BPJS Ketenagakerjaan', 'Laptop Kerja'],
  }
  formErrors.value = {}
  isFormModalOpen.value = true
}

const validateForm = () => {
  const errs = {}
  if (!formData.value.nama?.trim()) errs.nama = 'Nama lengkap wajib diisi'
  if (!formData.value.nip?.trim()) errs.nip = 'NIP wajib diisi'
  if (!formData.value.email?.trim()) errs.email = 'Email kantor wajib diisi'
  if (!formData.value.jabatan?.trim()) errs.jabatan = 'Jabatan wajib diisi'
  if (!formData.value.gaji || Number(formData.value.gaji) <= 0) {
    errs.gaji = 'Gaji pokok wajib berupa nominal valid'
  }
  formErrors.value = errs
  return Object.keys(errs).length === 0
}

const handleSaveForm = async () => {
  if (!validateForm()) {
    toast.error('Harap lengkapi formulir dengan data yang valid', 'Validasi Gagal')
    return
  }

  const payload = {
    ...formData.value,
    gaji: Number(formData.value.gaji),
    tunjangan: Number(formData.value.tunjangan) || 0,
    avatar: formData.value.nama
      .split(' ')
      .map((w) => w[0])
      .slice(0, 2)
      .join('')
      .toUpperCase(),
  }

  isFormModalOpen.value = false

  if (isEditMode.value) {
    await toast.promise(
      new Promise((resolve) => {
        setTimeout(() => {
          const idx = listKaryawan.value.findIndex((k) => k.id === payload.id)
          if (idx !== -1) listKaryawan.value[idx] = payload
          resolve(payload)
        }, 600)
      }),
      {
        loading: 'Menyimpan perubahan data...',
        success: `Data pegawai ${payload.nama} berhasil diperbarui!`,
        error: 'Gagal memperbarui data pegawai',
      }
    )
  } else {
    await toast.promise(
      new Promise((resolve) => {
        setTimeout(() => {
          payload.id = Date.now()
          listKaryawan.value.unshift(payload)
          resolve(payload)
        }, 600)
      }),
      {
        loading: 'Mendaftarkan pegawai baru...',
        success: `Pegawai ${payload.nama} berhasil ditambahkan!`,
        error: 'Gagal menambahkan pegawai',
      }
    )
  }
}

// ========================================================
// 5. DELETE MODAL CONFIRMATION
// ========================================================
const isDeleteModalOpen = ref(false)
const itemToDelete = ref(null)

const openDeleteConfirm = (item) => {
  itemToDelete.value = item
  isDeleteModalOpen.value = true
}

const handleDeleteConfirm = () => {
  if (!itemToDelete.value) return
  const deleted = itemToDelete.value
  listKaryawan.value = listKaryawan.value.filter((k) => k.id !== deleted.id)
  isDeleteModalOpen.value = false
  itemToDelete.value = null
  toast.error(`Pegawai ${deleted.nama} (${deleted.nip}) telah dihapus dari sistem.`, 'Data Dihapus')
}

// Reset Default
const handleResetDefault = () => {
  listKaryawan.value = [...initialKaryawan]
  searchQuery.value = ''
  selectedDivisi.value = 'all'
  selectedStatus.value = 'all'
  selectedIds.value = []
  sortKey.value = 'gaji'
  sortOrder.value = 'desc'
  toast.info('Data telah dikembalikan ke daftar awal.')
}

// CSV Importer State
const isCsvImporterOpen = ref(false)
const targetFieldsForImport = [
  { key: 'nama', label: 'Nama Lengkap', required: true },
  { key: 'nip', label: 'NIP Pegawai', required: false },
  { key: 'jabatan', label: 'Jabatan / Posisi', required: true },
  { key: 'divisi', label: 'Divisi', required: false },
  { key: 'email', label: 'Email Kantor', required: false },
  { key: 'telepon', label: 'Nomor Telepon', required: false },
  { key: 'gaji', label: 'Gaji Pokok', required: false },
  { key: 'status', label: 'Status Pegawai', required: false },
]

const handleCsvImported = (importedRows) => {
  if (!importedRows || importedRows.length === 0) return

  let count = 0
  importedRows.forEach((row, idx) => {
    if (!row.nama) return
    listKaryawan.value.unshift({
      id: Date.now() + idx,
      nip: row.nip || 'EMP-' + Math.floor(1000 + Math.random() * 9000),
      nama: row.nama,
      jabatan: row.jabatan || 'Staf Operasional',
      divisi: row.divisi || 'Operasional',
      email: row.email || `${row.nama.toLowerCase().replace(/\s+/g, '.')}@digaji.id`,
      telepon: row.telepon || '0812-' + Math.floor(10000000 + Math.random() * 90000000),
      gaji: Number(row.gaji) || 5500000,
      tunjangan: 750000,
      status: row.status || 'Tetap',
      tanggalMasuk: '2025-01-27',
      lokasi: 'Jakarta Pusat',
      bpjs: true,
      avatar: row.nama
        .split(' ')
        .map((w) => w[0])
        .slice(0, 2)
        .join('')
        .toUpperCase(),
    })
    count++
  })

  toast.success(`Berhasil mengimpor ${count} data pegawai baru dari CSV!`, 'Impor CSV Berhasil')
}

// Slide-Over Detail Drawer State
const isSlideOverOpen = ref(false)
const selectedEmployee = ref(null)

const openEmployeeDetail = (emp) => {
  selectedEmployee.value = emp
  isSlideOverOpen.value = true
}

// Sample Employee Timeline
const getEmployeeTimeline = (emp) => [
  {
    id: 1,
    title: 'Penyesuaian Gaji Pokok Berkala',
    description: `Kenaikan kompensasi disetujui sebesar Rp ${(emp.gaji || 0).toLocaleString('id-ID')} per bulan.`,
    time: '15 Jan 2025',
    variant: 'success',
    author: 'HR Management',
  },
  {
    id: 2,
    title: `Promosi Jabatan: ${emp.jabatan}`,
    description: `Ditempatkan pada Divisi ${emp.divisi} di kantor ${emp.lokasi}.`,
    time: '01 Des 2024',
    variant: 'primary',
    author: 'Direksi Operasional',
  },
  {
    id: 3,
    title: 'Verifikasi Berkas & Onboarding Tuntas',
    description: `NIP resmi ${emp.nip} diterbitkan, kepesertaan BPJS Ketenagakerjaan aktif.`,
    time: emp.tanggalMasuk || '10 Jan 2024',
    variant: 'info',
    author: 'HR Staff',
  },
]

// Action items for BaseDropdownMenu
const getRowActionItems = (item) => [
  {
    label: 'Lihat Profil & Timeline',
    icon: Eye,
    action: () => openEmployeeDetail(item),
  },
  {
    label: 'Ubah Data Pegawai',
    icon: Pencil,
    action: () => openEditModal(item),
  },
  {
    label: 'Hapus dari Sistem',
    icon: Trash2,
    danger: true,
    action: () => openDeleteConfirm(item),
  },
]
</script>

<template>
  <!-- Main View Container -->
  <div class="h-full flex flex-col min-h-0 overflow-hidden gap-2.5 w-full max-w-7xl mx-auto select-none relative">
    
    <!-- Top Compact Header: Title & Action Buttons -->
    <div class="flex items-center justify-between gap-3 pb-2 border-b border-neutral-200 dark:border-[var(--theme-border)] shrink-0">
      <div class="flex items-center gap-2.5 min-w-0">
        <div class="w-8 h-8 rounded-xl bg-neutral-900 text-white dark:bg-white dark:text-neutral-950 flex items-center justify-center shrink-0 shadow-2xs">
          <Users class="w-4 h-4" />
        </div>
        <div class="min-w-0">
          <div class="flex items-center gap-2">
            <h1 class="text-base sm:text-lg font-bold text-neutral-900 dark:text-[var(--theme-text-primary)] leading-none truncate">
              Data Karyawan & Payroll
            </h1>
            <BaseBadge variant="success" size="sm">
              {{ listKaryawan.length }} Pegawai
            </BaseBadge>
          </div>
          <p class="text-[11px] text-neutral-500 dark:text-neutral-400 mt-0.5 truncate">
            Pengelolaan data sumber daya manusia, jabatan, struktur divisi, serta kompensasi payroll.
          </p>
        </div>
      </div>

      <!-- Action Buttons -->
      <div class="flex items-center gap-2 shrink-0">
        <BaseButton variant="outline" size="sm" @click="isCsvImporterOpen = true" title="Impor data dari berkas CSV">
          <Upload class="w-3.5 h-3.5 text-emerald-600 dark:text-emerald-400" />
          <span class="hidden sm:inline">Impor CSV</span>
        </BaseButton>

        <BaseButton variant="outline" size="sm" @click="exportToCSV()" title="Ekspor seluruh data ke file CSV">
          <Download class="w-3.5 h-3.5 text-neutral-500" />
          <span class="hidden sm:inline">Ekspor CSV</span>
        </BaseButton>

        <BaseButton variant="outline" size="sm" @click="handleResetDefault" title="Kembalikan data default">
          <RotateCcw class="w-3.5 h-3.5 text-neutral-500" />
          <span class="hidden sm:inline">Reset</span>
        </BaseButton>

        <BaseButton variant="primary" size="sm" @click="openAddModal">
          <Plus class="w-3.5 h-3.5" />
          <span>Tambah</span>
        </BaseButton>
      </div>
    </div>

    <!-- Filter Bar: Search + Searchable Select Divisi + Status -->
    <div class="flex flex-col sm:flex-row items-stretch sm:items-center gap-2 shrink-0">
      <!-- Search Input -->
      <div class="relative flex-1">
        <Search class="w-4 h-4 text-neutral-400 absolute left-3 top-1/2 -translate-y-1/2 pointer-events-none" />
        <input
          v-model="searchQuery"
          type="text"
          placeholder="Cari nama pegawai, NIP, jabatan, atau divisi..."
          class="w-full pl-9 pr-8 py-2 rounded-xl text-xs bg-white dark:bg-[var(--theme-surface-raised)] border border-neutral-200 dark:border-[var(--theme-border)] text-neutral-900 dark:text-white placeholder:text-neutral-400 focus:outline-none focus:ring-2 focus:ring-neutral-400 dark:focus:ring-[var(--theme-border-strong)] transition shadow-2xs"
        />
        <button
          v-if="searchQuery"
          type="button"
          @click="searchQuery = ''"
          class="absolute right-2.5 top-1/2 -translate-y-1/2 text-neutral-400 hover:text-neutral-600 dark:hover:text-neutral-200"
        >
          <X class="w-3.5 h-3.5" />
        </button>
      </div>

      <!-- Filter Divisi: BaseSearchableSelect -->
      <div class="w-full sm:w-56 shrink-0">
        <BaseSearchableSelect
          v-model="selectedDivisi"
          :options="divisiOptions"
          placeholder="Filter Divisi..."
          clearable
        />
      </div>

      <!-- Filter Status Kerja: BaseSearchableSelect -->
      <div class="w-full sm:w-48 shrink-0">
        <BaseSearchableSelect
          v-model="selectedStatus"
          :options="statusOptions"
          placeholder="Filter Status..."
          align="right"
          clearable
        />
      </div>
    </div>

    <!-- Quick Stats Summary Strip -->
    <div class="grid grid-cols-2 sm:grid-cols-4 gap-2 shrink-0">
      <div class="h-9 px-2.5 rounded-xl bg-white dark:bg-[var(--theme-surface)] border border-neutral-200 dark:border-[var(--theme-border)] shadow-2xs flex items-center gap-2">
        <div class="w-6 h-6 rounded-lg bg-neutral-100 dark:bg-[var(--theme-surface-muted)] text-neutral-700 dark:text-neutral-300 flex items-center justify-center shrink-0">
          <Users class="w-3.5 h-3.5" />
        </div>
        <div class="flex items-center gap-1.5 min-w-0 truncate text-xs">
          <span class="text-[10px] uppercase font-bold text-neutral-400 tracking-wider whitespace-nowrap">Total:</span>
          <span class="font-bold text-neutral-900 dark:text-white truncate">{{ listKaryawan.length }} Pegawai</span>
        </div>
      </div>

      <div class="h-9 px-2.5 rounded-xl bg-white dark:bg-[var(--theme-surface)] border border-neutral-200 dark:border-[var(--theme-border)] shadow-2xs flex items-center gap-2">
        <div class="w-6 h-6 rounded-lg bg-emerald-50 dark:bg-emerald-950/40 text-emerald-600 dark:text-emerald-400 flex items-center justify-center shrink-0">
          <Coins class="w-3.5 h-3.5" />
        </div>
        <div class="flex items-center gap-1.5 min-w-0 truncate text-xs">
          <span class="text-[10px] uppercase font-bold text-emerald-400/80 tracking-wider whitespace-nowrap">Payroll:</span>
          <span class="font-bold text-emerald-600 dark:text-emerald-400 truncate">{{ formatRupiah(totalPayroll) }}</span>
        </div>
      </div>

      <div class="h-9 px-2.5 rounded-xl bg-white dark:bg-[var(--theme-surface)] border border-neutral-200 dark:border-[var(--theme-border)] shadow-2xs flex items-center gap-2">
        <div class="w-6 h-6 rounded-lg bg-sky-50 dark:bg-sky-950/40 text-sky-600 dark:text-sky-400 flex items-center justify-center shrink-0">
          <CheckCircle2 class="w-3.5 h-3.5" />
        </div>
        <div class="flex items-center gap-1.5 min-w-0 truncate text-xs">
          <span class="text-[10px] uppercase font-bold text-sky-400/80 tracking-wider whitespace-nowrap">Tetap:</span>
          <span class="font-bold text-sky-600 dark:text-sky-400 truncate">{{ totalTetap }} Orang</span>
        </div>
      </div>

      <div class="h-9 px-2.5 rounded-xl bg-white dark:bg-[var(--theme-surface)] border border-neutral-200 dark:border-[var(--theme-border)] shadow-2xs flex items-center gap-2">
        <div class="w-6 h-6 rounded-lg bg-amber-50 dark:bg-amber-950/40 text-amber-600 dark:text-amber-400 flex items-center justify-center shrink-0">
          <Clock class="w-3.5 h-3.5" />
        </div>
        <div class="flex items-center gap-1.5 min-w-0 truncate text-xs">
          <span class="text-[10px] uppercase font-bold text-amber-400/80 tracking-wider whitespace-nowrap">Kontrak:</span>
          <span class="font-bold text-amber-600 dark:text-amber-400 truncate">{{ totalKontrak }} Orang</span>
        </div>
      </div>
    </div>

    <!-- ======================================================== -->
    <!-- BASE DATA TABLE (DESKTOP + MOBILE + BULK ACTION BAR)     -->
    <!-- ======================================================== -->
    <BaseDataTable
      :columns="tableColumns"
      :rows="paginatedList"
      :total-items="processedList.length"
      selectable
      v-model:selected-ids="selectedIds"
      v-model:sort-key="sortKey"
      v-model:sort-order="sortOrder"
      v-model:current-page="currentPage"
      v-model:items-per-page="itemsPerPage"
      :per-page-options="[10, 20, 30, 'all']"
      selected-label="Pegawai Terpilih"
      empty-title="Data Pegawai Tidak Ditemukan"
      empty-description="Tidak ada karyawan yang cocok dengan kriteria filter atau pencarian Anda."
      empty-action-text="Reset Filter Pencarian"
      @empty-action="handleResetDefault"
    >
      <!-- Pegawai (Avatar + Nama + NIP) -->
      <template #cell-nama="{ row }">
        <div class="flex items-center gap-2.5">
          <div class="w-8 h-8 rounded-xl bg-neutral-100 dark:bg-[var(--theme-surface-muted)] border border-neutral-200/80 dark:border-[var(--theme-border-strong)] text-neutral-800 dark:text-neutral-200 font-bold text-xs flex items-center justify-center shrink-0">
            {{ row.avatar }}
          </div>
          <div class="min-w-0">
            <span class="font-semibold text-neutral-900 dark:text-white block leading-tight truncate">
              {{ row.nama }}
            </span>
            <span class="text-[10.5px] font-mono text-neutral-400 block mt-0.5">
              {{ row.nip }}
            </span>
          </div>
        </div>
      </template>

      <!-- Jabatan & Divisi -->
      <template #cell-divisi="{ row }">
        <span class="font-medium text-neutral-800 dark:text-neutral-200 block leading-tight">
          {{ row.jabatan }}
        </span>
        <span class="text-[10.5px] text-neutral-400 block mt-0.5">
          {{ row.divisi }}
        </span>
      </template>

      <!-- Gaji Pokok & Tunjangan -->
      <template #cell-gaji="{ row }">
        <span class="font-bold text-neutral-900 dark:text-white font-mono block leading-tight">
          {{ formatRupiah(row.gaji) }}
        </span>
        <span class="text-[10px] text-emerald-600 dark:text-emerald-400 font-mono block mt-0.5">
          +{{ formatRupiah(row.tunjangan) }} tunj.
        </span>
      </template>

      <!-- Status Kerja BaseBadge -->
      <template #cell-status="{ row }">
        <BaseBadge
          :variant="getStatusBadgeVariant(row.status)"
          :dot="row.status === 'Tetap' || row.status === 'Kontrak'"
          :pulse="row.status === 'Tetap'"
        >
          {{ row.status }}
        </BaseBadge>
      </template>

      <!-- Lokasi -->
      <template #cell-lokasi="{ row }">
        <div class="flex items-center gap-1.5">
          <MapPin class="w-3.5 h-3.5 text-neutral-400 shrink-0" />
          <span>{{ row.lokasi }}</span>
        </div>
      </template>

      <!-- BPJS Aktif BaseBadge -->
      <template #cell-bpjsAktif="{ row }">
        <BaseBadge :variant="row.bpjsAktif ? 'success' : 'neutral'" size="sm">
          {{ row.bpjsAktif ? 'BPJS Aktif' : 'Non-BPJS' }}
        </BaseBadge>
      </template>

      <!-- Aksi -->
      <template #actions="{ row }">
        <div class="flex items-center justify-end gap-1">
          <button
            type="button"
            @click="openEditModal(row)"
            class="p-1.5 rounded-lg text-neutral-600 hover:text-neutral-900 hover:bg-neutral-100 dark:text-neutral-300 dark:hover:text-white dark:hover:bg-[var(--theme-surface-muted)] transition cursor-pointer"
            title="Edit Pegawai"
          >
            <Pencil class="w-3.5 h-3.5" />
          </button>
          <BaseDropdownMenu :items="getRowActionItems(row)" align="right" />
        </div>
      </template>

      <!-- Kartu Mobile -->
      <template #mobile-card="{ row, isSelected, toggleSelect: onToggleSelect }">
        <div
          class="p-3.5 rounded-2xl bg-white dark:bg-[var(--theme-surface)] border border-neutral-200 dark:border-[var(--theme-border)] shadow-xs space-y-3"
          :class="isSelected ? 'ring-2 ring-[var(--theme-primary)]' : ''"
        >
          <!-- Top Row: Checkbox, Avatar, Nama, Status -->
          <div class="flex items-start justify-between gap-2">
            <div class="flex items-center gap-2.5">
              <BaseCheckbox
                :model-value="isSelected"
                size="sm"
                class="mt-0.5"
                @change="onToggleSelect"
              />
              <div class="w-8 h-8 rounded-xl bg-neutral-100 dark:bg-[var(--theme-surface-muted)] font-bold text-xs flex items-center justify-center shrink-0">
                {{ row.avatar }}
              </div>
              <div>
                <h3 class="text-sm font-bold text-neutral-900 dark:text-white leading-tight">
                  {{ row.nama }}
                </h3>
                <div class="flex items-center gap-1.5 mt-0.5">
                  <span class="text-[10.5px] font-mono text-neutral-400">{{ row.nip }}</span>
                  <span class="text-neutral-300">&bull;</span>
                  <span class="text-[10.5px] text-neutral-500">{{ row.divisi }}</span>
                </div>
              </div>
            </div>

            <BaseBadge
              :variant="getStatusBadgeVariant(row.status)"
              :dot="row.status === 'Tetap' || row.status === 'Kontrak'"
              :pulse="row.status === 'Tetap'"
              size="sm"
            >
              {{ row.status }}
            </BaseBadge>
          </div>

          <!-- Mid Row: Kompensasi Gaji & Lokasi -->
          <div class="p-2.5 rounded-xl bg-neutral-50 dark:bg-[var(--theme-surface-raised)] border border-neutral-100 dark:border-[var(--theme-border)] flex items-center justify-between text-xs">
            <div>
              <span class="text-[10px] text-neutral-400 block font-semibold uppercase">Gaji Pokok</span>
              <span class="font-bold font-mono text-neutral-900 dark:text-white">
                {{ formatRupiah(row.gaji) }}
              </span>
            </div>
            <div class="text-right">
              <span class="text-[10px] text-neutral-400 block font-semibold uppercase">Lokasi</span>
              <span class="text-neutral-700 dark:text-neutral-300 font-medium">
                {{ row.lokasi }}
              </span>
            </div>
          </div>

          <!-- Bottom Action Buttons Mobile -->
          <div class="flex items-center justify-between pt-1">
            <BaseBadge :variant="row.bpjsAktif ? 'success' : 'neutral'" size="sm">
              {{ row.bpjsAktif ? 'BPJS Aktif' : 'Non-BPJS' }}
            </BaseBadge>

            <div class="flex items-center gap-1.5">
              <button
                type="button"
                @click="openEditModal(row)"
                class="px-2.5 py-1 rounded-lg text-xs font-semibold bg-neutral-100 dark:bg-[var(--theme-surface-muted)] text-neutral-700 dark:text-neutral-300 hover:bg-neutral-200 transition cursor-pointer"
              >
                Edit
              </button>
              <button
                type="button"
                @click="openDeleteConfirm(row)"
                class="p-1 rounded-lg text-rose-500 hover:bg-rose-50 dark:hover:bg-rose-950/30 transition cursor-pointer"
              >
                <Trash2 class="w-3.5 h-3.5" />
              </button>
            </div>
          </div>
        </div>
      </template>

      <!-- Tombol Aksi Massal pada Floating Bulk Action Bar -->
      <template #bulk-actions="{ selectedIds: ids }">
        <BaseButton variant="outline" size="sm" @click="exportSelected">
          <Download class="w-3.5 h-3.5" />
          <span>Ekspor CSV</span>
        </BaseButton>

        <BaseButton variant="danger" size="sm" @click="isBatchDeleteModalOpen = true">
          <Trash2 class="w-3.5 h-3.5" />
          <span>Hapus ({{ ids.length }})</span>
        </BaseButton>
      </template>
    </BaseDataTable>

    <!-- ======================================================== -->
    <!-- MODAL FORM CRUD: TAMBAH & EDIT PEGAWAI                   -->
    <!-- Auto-Adaptive: Otomatis render MobileModal di smartphone -->
    <!-- dan BaseModal di desktop                                 -->
    <!-- ======================================================== -->
    <BaseModal
      v-model="isFormModalOpen"
      :title="isEditMode ? 'Edit Data Pegawai' : 'Tambah Pegawai Baru'"
      :subtitle="isEditMode ? 'Perbarui informasi gaji dan jabatan pegawai' : 'Masukkan informasi lengkap pegawai ke sistem payroll'"
      size="lg"
    >
      <form @submit.prevent="handleSaveForm" class="space-y-4 py-1">
        <!-- Baris 1: Nama & NIP -->
        <div class="grid grid-cols-1 sm:grid-cols-2 gap-3.5">
          <BaseInput
            v-model="formData.nama"
            label="Nama Lengkap Pegawai"
            required
            placeholder="Contoh: Rizky Pratama"
            :error="formErrors.nama"
            clearable
          />

          <BaseInput
            v-model="formData.nip"
            label="Nomor Induk Pegawai (NIP)"
            required
            placeholder="EMP-2024-025"
            :error="formErrors.nip"
          />
        </div>

        <!-- Baris 2: Email & Nomor WhatsApp -->
        <div class="grid grid-cols-1 sm:grid-cols-2 gap-3.5">
          <BaseInput
            v-model="formData.email"
            type="email"
            label="Email Kantor"
            required
            placeholder="nama@digaji.id"
            :error="formErrors.email"
            :leading-icon="Mail"
          />

          <BaseInput
            v-model="formData.telepon"
            type="tel"
            label="Nomor WhatsApp"
            placeholder="0812-3456-7890"
            :leading-icon="Phone"
          />
        </div>

        <!-- Baris 3: Jabatan, Divisi & Status Kerja -->
        <div class="grid grid-cols-1 sm:grid-cols-3 gap-3.5">
          <BaseInput
            v-model="formData.jabatan"
            label="Jabatan / Posisi"
            required
            placeholder="Lead Frontend Engineer"
            :error="formErrors.jabatan"
          />

          <BaseFormField label="Divisi / Departemen">
            <BaseSearchableSelect
              v-model="formData.divisi"
              :options="formDivisiOptions"
              size="md"
            />
          </BaseFormField>

          <BaseFormField label="Status Kerja">
            <BaseSearchableSelect
              v-model="formData.status"
              :options="formStatusOptions"
              :searchable="false"
              size="md"
              align="right"
            />
          </BaseFormField>
        </div>

        <!-- Baris 4: Kompensasi Payroll & Tanggal Masuk (BaseDatePicker) -->
        <div class="grid grid-cols-1 sm:grid-cols-3 gap-3.5">
          <BaseCurrencyInput
            v-model="formData.gaji"
            label="Gaji Pokok"
            required
            suffix="/ bln"
            placeholder="15.000.000"
            :error="formErrors.gaji"
            terbilang
          />

          <BaseCurrencyInput
            v-model="formData.tunjangan"
            label="Tunjangan Kinerja"
            suffix="/ bln"
            placeholder="2.500.000"
          />

          <!-- BaseDatePicker Integrasi -->
          <BaseDatePicker
            v-model="formData.tanggalMasuk"
            label="Tanggal Bergabung"
            placeholder="Pilih tanggal..."
          />
        </div>

        <!-- Baris 5: Lokasi & Rekening Payroll -->
        <div class="grid grid-cols-1 sm:grid-cols-2 gap-3.5">
          <BaseInput
            v-model="formData.lokasi"
            label="Lokasi Penempatan"
            placeholder="Jakarta Selatan"
            :leading-icon="MapPin"
          />

          <BaseInput
            v-model="formData.rekening"
            label="Nomor Rekening Payroll"
            placeholder="BCA - 8820194812"
            :leading-icon="CreditCard"
          />
        </div>

        <!-- Baris 5b: Fasilitas & Benefit Tambahan (BaseMultiSelect) -->
        <BaseMultiSelect
          v-model="formData.fasilitas"
          :options="formFasilitasOptions"
          label="Fasilitas & Benefit Pegawai"
          placeholder="Pilih satu atau lebih fasilitas..."
          search-placeholder="Cari fasilitas atau tunjangan..."
          :max-chips="3"
          hint="Pilih paket asuransi dan fasilitas kerja yang diterima pegawai."
        />

        <!-- Baris 6: Catatan Karyawan (BaseTextarea) -->
        <BaseTextarea
          v-model="formData.catatan"
          label="Catatan & Ringkasan Performa"
          placeholder="Tuliskan keahlian teknis atau catatan HR..."
          :rows="2"
          :maxlength="250"
        />

        <!-- Berkas Dokumen & Pasfoto (BaseFileUploader) -->
        <BaseFileUploader
          label="Unggah Pasfoto / Dokumen Pendukung (Opsional)"
          hint="Format berkas: JPG, PNG, atau PDF (maks. 5 MB)"
          accept="image/*,.pdf"
          :multiple="false"
        />

        <!-- Baris 7: Switch BPJS Asuransi (BaseSwitch) -->
        <div class="p-3.5 rounded-xl bg-neutral-50 dark:bg-[var(--theme-surface-raised)] border border-neutral-200/80 dark:border-[var(--theme-border)]">
          <BaseSwitch
            v-model="formData.bpjsAktif"
            label="Kepesertaan BPJS Ketenagakerjaan & Kesehatan"
            description="Aktifkan pemotongan premi asuransi resmi dan jaminan pensiun."
          />
        </div>
      </form>

      <template #footer="{ close }">
        <BaseButton variant="outline" size="sm" @click="close">
          Batal
        </BaseButton>
        <BaseButton variant="primary" size="sm" @click="handleSaveForm">
          <Check class="w-3.5 h-3.5" />
          <span>{{ isEditMode ? 'Simpan Perubahan' : 'Daftarkan Pegawai' }}</span>
        </BaseButton>
      </template>
    </BaseModal>

    <!-- Confirmation Modal Single Delete -->
    <ConfirmationModal
      v-model="isDeleteModalOpen"
      type="delete"
      title="Hapus Pegawai"
      :message="itemToDelete ? `Apakah Anda yakin ingin menghapus data pegawai ${itemToDelete.nama} (${itemToDelete.nip})? Data yang dihapus tidak dapat dipulihkan.` : ''"
      confirm-text="Hapus Pegawai"
      cancel-text="Batal"
      @confirm="handleDeleteConfirm"
    />

    <!-- Confirmation Modal Batch Delete -->
    <ConfirmationModal
      v-model="isBatchDeleteModalOpen"
      type="delete"
      title="Hapus Pegawai Terpilih"
      :message="`Apakah Anda yakin ingin menghapus ${selectedIds.length} data pegawai yang dipilih sekaligus? Tindakan ini tidak dapat dibatalkan.`"
      :confirm-text="`Hapus ${selectedIds.length} Pegawai`"
      cancel-text="Batal"
      @confirm="handleBatchDeleteConfirm"
    />

    <!-- ======================================================== -->
    <!-- WIZARD IMPOR DATA CSV (BaseCsvImporter.vue)             -->
    <!-- ======================================================== -->
    <BaseCsvImporter
      v-model="isCsvImporterOpen"
      title="Impor Data Pegawai dari Berkas CSV"
      :target-fields="targetFieldsForImport"
      @imported="handleCsvImported"
    />

    <!-- ======================================================== -->
    <!-- SLIDE-OVER DETAIL PEGAWAI & TIMELINE (BaseSlideOver.vue) -->
    <!-- ======================================================== -->
    <BaseSlideOver
      v-model="isSlideOverOpen"
      :title="selectedEmployee?.nama || 'Profil Pegawai'"
      :subtitle="selectedEmployee ? `${selectedEmployee.nip} • ${selectedEmployee.jabatan}` : ''"
      size="md"
    >
      <template #badge>
        <BaseBadge
          v-if="selectedEmployee"
          :variant="getStatusBadgeVariant(selectedEmployee.status)"
          size="sm"
        >
          {{ selectedEmployee.status }}
        </BaseBadge>
      </template>

      <div v-if="selectedEmployee" class="space-y-5 text-xs">
        <!-- Avatar & Header Identity Card -->
        <div class="p-4 rounded-2xl bg-neutral-50 dark:bg-[var(--theme-surface-raised)] border border-neutral-200 dark:border-[var(--theme-border)] flex items-center gap-3.5">
          <BaseAvatar
            :name="selectedEmployee.nama"
            size="lg"
            status="online"
            tone="primary"
          />
          <div class="min-w-0">
            <h4 class="font-bold text-sm sm:text-base text-neutral-900 dark:text-white truncate">
              {{ selectedEmployee.nama }}
            </h4>
            <div class="flex items-center gap-2 mt-0.5 text-neutral-500 text-[11px]">
              <span>{{ selectedEmployee.divisi }}</span>
              <span>&bull;</span>
              <span>{{ selectedEmployee.lokasi }}</span>
            </div>
          </div>
        </div>

        <!-- Approval Stepper Flow -->
        <div class="p-3.5 rounded-2xl bg-white dark:bg-[var(--theme-surface-raised)] border border-neutral-200 dark:border-[var(--theme-border)] space-y-2">
          <span class="text-[10px] uppercase font-bold text-neutral-400 tracking-wider">Status Alur Persetujuan</span>
          <BaseStepper
            :steps="[
              { id: 1, title: 'Input HR', approver: 'Sari Rahayu', status: 'completed' },
              { id: 2, title: 'Audit Finansial', approver: 'Budi Santoso', status: 'completed' },
              { id: 3, title: 'Disetujui Direksi', approver: 'Direktur Utama', status: 'current' },
            ]"
            :current-step="2"
            size="sm"
          />
        </div>

        <!-- Contact & Profile Metadata (BaseDescriptionList) -->
        <div class="space-y-2">
          <span class="text-[10px] uppercase font-bold text-neutral-400 tracking-wider">Kontak & Kepegawaian</span>
          <BaseDescriptionList
            :columns="2"
            :items="[
              { label: 'Email Kantor', value: selectedEmployee.email },
              { label: 'Nomor Telepon', value: selectedEmployee.telepon, mono: true },
              { label: 'Tanggal Bergabung', value: selectedEmployee.tanggalMasuk },
              {
                label: 'Asuransi BPJS',
                value: selectedEmployee.bpjs ? 'Aktif (Kesehatan & TK)' : 'Tidak Terdaftar',
                badge: selectedEmployee.bpjs ? 'Aktif' : 'Non-BPJS',
                badgeVariant: selectedEmployee.bpjs ? 'success' : 'neutral',
              },
            ]"
          />
        </div>

        <!-- Compensation Payroll Card -->
        <div class="p-4 rounded-2xl bg-white dark:bg-[var(--theme-surface-raised)] border border-neutral-200 dark:border-[var(--theme-border)] space-y-2.5 shadow-2xs">
          <span class="text-[10px] uppercase font-bold text-neutral-400 tracking-wider">Kompensasi Bulanan</span>
          <div class="flex items-center justify-between text-xs">
            <span class="text-neutral-500">Gaji Pokok:</span>
            <span class="font-mono font-bold text-neutral-900 dark:text-white">{{ formatRupiah(selectedEmployee.gaji) }}</span>
          </div>
          <div class="flex items-center justify-between text-xs">
            <span class="text-neutral-500">Tunjangan Kinerja:</span>
            <span class="font-mono font-bold text-neutral-900 dark:text-white">{{ formatRupiah(selectedEmployee.tunjangan) }}</span>
          </div>
          <div class="pt-2 border-t border-neutral-100 dark:border-[var(--theme-border)] flex items-center justify-between text-sm">
            <span class="font-bold text-neutral-900 dark:text-white">Take Home Pay:</span>
            <span class="font-mono font-bold text-[var(--theme-primary)]">
              {{ formatRupiah((selectedEmployee.gaji || 0) + (selectedEmployee.tunjangan || 0)) }}
            </span>
          </div>
        </div>

        <!-- Career History Timeline Component -->
        <div class="space-y-2 pt-1">
          <span class="text-[10px] uppercase font-bold text-neutral-400 tracking-wider">Jejak Karier & Riwayat Payroll</span>
          <div class="p-4 rounded-2xl bg-neutral-50/50 dark:bg-[var(--theme-surface-raised)] border border-neutral-200 dark:border-[var(--theme-border)]">
            <BaseTimeline :items="getEmployeeTimeline(selectedEmployee)" compact />
          </div>
        </div>
      </div>

      <template #footer="{ close }">
        <BaseButton variant="outline" size="sm" @click="close">Tutup</BaseButton>
        <BaseButton variant="primary" size="sm" @click="openEditModal(selectedEmployee); close();">
          <Pencil class="w-3.5 h-3.5" />
          <span>Edit Data</span>
        </BaseButton>
      </template>
    </BaseSlideOver>
  </div>
</template>
