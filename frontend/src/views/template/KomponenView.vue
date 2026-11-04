<script setup>
import { ref, computed } from 'vue'
import {
  Search,
  Component,
  Sliders,
  Layers,
  BarChart3,
  Compass,
  Sparkles,
  Check,
  Copy,
  ExternalLink,
  Info,
  Calendar,
  Clock,
  MapPin,
  Lock,
  Camera,
  FolderTree,
  User,
  ShoppingBag,
  Wallet,
  AlertTriangle,
  ChevronRight,
  Eye,
  Trash2,
} from 'lucide-vue-next'

// Import All Base Components with Relative Paths
import BaseButton from '../../components/ui/BaseButton.vue'
import BaseBadge from '../../components/ui/BaseBadge.vue'
import BaseInput from '../../components/ui/BaseInput.vue'
import BaseCurrencyInput from '../../components/ui/BaseCurrencyInput.vue'
import BaseNumberInput from '../../components/ui/BaseNumberInput.vue'
import BasePasswordInput from '../../components/ui/BasePasswordInput.vue'
import BaseMaskedInput from '../../components/ui/BaseMaskedInput.vue'
import BaseTextarea from '../../components/ui/BaseTextarea.vue'
import BaseSwitch from '../../components/ui/BaseSwitch.vue'
import BaseCheckbox from '../../components/ui/BaseCheckbox.vue'
import BaseRadio from '../../components/ui/BaseRadio.vue'
import BaseFormField from '../../components/ui/BaseFormField.vue'
import BaseSearchableSelect from '../../components/ui/BaseSearchableSelect.vue'
import BaseMultiSelect from '../../components/ui/BaseMultiSelect.vue'
import BaseDatePicker from '../../components/ui/BaseDatePicker.vue'
import BaseTimePicker from '../../components/ui/BaseTimePicker.vue'
import BaseFileUploader from '../../components/ui/BaseFileUploader.vue'
import BaseModal from '../../components/ui/BaseModal.vue'
import ConfirmationModal from '../../components/ui/ConfirmationModal.vue'
import BaseSlideOver from '../../components/ui/BaseSlideOver.vue'
import BasePopover from '../../components/ui/BasePopover.vue'
import BaseTooltip from '../../components/ui/BaseTooltip.vue'
import BaseInvoiceModal from '../../components/ui/BaseInvoiceModal.vue'
import BaseCsvImporter from '../../components/ui/BaseCsvImporter.vue'
import BaseDataTable from '../../components/ui/BaseDataTable.vue'
import BasePagination from '../../components/ui/BasePagination.vue'
import BaseAvatar from '../../components/ui/BaseAvatar.vue'
import BaseStatCard from '../../components/ui/BaseStatCard.vue'
import BaseAlert from '../../components/ui/BaseAlert.vue'
import BaseTimeline from '../../components/ui/BaseTimeline.vue'
import BaseEmptyState from '../../components/ui/BaseEmptyState.vue'
import BaseSkeleton from '../../components/ui/BaseSkeleton.vue'
import BaseCountUp from '../../components/ui/BaseCountUp.vue'
import BaseTabs from '../../components/ui/BaseTabs.vue'
import BaseSegmentedControl from '../../components/ui/BaseSegmentedControl.vue'
import BaseBreadcrumb from '../../components/ui/BaseBreadcrumb.vue'
import BaseStepper from '../../components/ui/BaseStepper.vue'
import BaseProgress from '../../components/ui/BaseProgress.vue'
import BaseAccordion from '../../components/ui/BaseAccordion.vue'
import BaseSelectableCard from '../../components/ui/BaseSelectableCard.vue'
import BaseDescriptionList from '../../components/ui/BaseDescriptionList.vue'
import BaseTree from '../../components/ui/BaseTree.vue'
import BaseTreeSelect from '../../components/ui/BaseTreeSelect.vue'
import BaseCalendar from '../../components/ui/BaseCalendar.vue'
import BaseCameraCapture from '../../components/ui/BaseCameraCapture.vue'
import BaseMapPolygonEditor from '../../components/ui/BaseMapPolygonEditor.vue'
import { useToast } from '../../composables/useToast'

const toast = useToast()

// ========================================================
// 1. DAFTAR MASTER KOMPONEN & KATEGORI
// ========================================================
const categories = [
  { id: 'all', label: 'Semua Komponen' },
  { id: 'form', label: 'Input & Form' },
  { id: 'dialog', label: 'Dialog & Panel' },
  { id: 'data', label: 'Data & Status' },
  { id: 'nav', label: 'Navigasi & Tata Letak' },
  { id: 'special', label: 'Struktur & Media' },
]

const componentCatalog = [
  // Form & Inputs
  {
    id: 'BaseButton',
    name: 'BaseButton',
    category: 'form',
    desc: 'Tombol aksi standar dengan varian primary, secondary, outline, danger, dan ghost.',
    props: 'variant, size, disabled, type',
  },
  {
    id: 'BaseBadge',
    name: 'BaseBadge',
    category: 'data',
    desc: 'Label status visual berwarna tema (success, warning, danger, info, purple, neutral).',
    props: 'variant, size, dot, pulse, removable',
  },
  {
    id: 'BaseInput',
    name: 'BaseInput',
    category: 'form',
    desc: 'Field input teks tunggal dengan ikon leading/trailing, prefix, suffix, dan tombol clear.',
    props: 'modelValue, label, required, type, placeholder, disabled, readonly, error, hint, clearable, size',
  },
  {
    id: 'BaseCurrencyInput',
    name: 'BaseCurrencyInput',
    category: 'form',
    desc: 'Input otomatis format mata uang Rupiah Indonesia lengkap dengan terbilang dinamis.',
    props: 'modelValue, label, required, prefix, suffix, terbilang, emitFormatted, error, hint',
  },
  {
    id: 'BaseNumberInput',
    name: 'BaseNumberInput',
    category: 'form',
    desc: 'Input angka non-uang dengan tombol stepper − dan +, min, max, step, dan suffix.',
    props: 'modelValue, label, required, min, max, step, prefix, suffix, controls, error, hint',
  },
  {
    id: 'BasePasswordInput',
    name: 'BasePasswordInput',
    category: 'form',
    desc: 'Input kata sandi dengan toggle intip dan 4-bar meteran kekuatan kata sandi.',
    props: 'modelValue, label, required, placeholder, showStrength, error, hint',
  },
  {
    id: 'BaseMaskedInput',
    name: 'BaseMaskedInput',
    category: 'form',
    desc: 'Input format bertanda baca (NIK 16 digit, NPWP 15 digit, no telepon, no rekening).',
    props: 'modelValue, preset, mask, emitRaw, label, required, placeholder, error, hint',
  },
  {
    id: 'BaseTextarea',
    name: 'BaseTextarea',
    category: 'form',
    desc: 'Area teks multi-baris dengan auto-expand tinggi otomatis dan counter karakter.',
    props: 'modelValue, label, required, rows, maxlength, autoExpand, error, hint',
  },
  {
    id: 'BaseSwitch',
    name: 'BaseSwitch',
    category: 'form',
    desc: 'Tombol saklar toggle boolean dengan label dan deskripsi.',
    props: 'modelValue, label, description, disabled, size',
  },
  {
    id: 'BaseCheckbox',
    name: 'BaseCheckbox',
    category: 'form',
    desc: 'Kotak centang kustom dengan mode boolean tunggal, array value, atau mode grup options.',
    props: 'modelValue, value, options, direction, indeterminate, label, description, disabled, size',
  },
  {
    id: 'BaseRadio',
    name: 'BaseRadio',
    category: 'form',
    desc: 'Radio button kustom mode tunggal atau grup vertikal/horizontal.',
    props: 'modelValue, value, options, direction, name, label, description, disabled, size',
  },
  {
    id: 'BaseFormField',
    name: 'BaseFormField',
    category: 'form',
    desc: 'Pembungkus label, tanda bintang merah wajib *, error, hint, dan tooltip bantuan.',
    props: 'label, required, error, hint, forId, helpText',
  },
  {
    id: 'BaseSearchableSelect',
    name: 'BaseSearchableSelect',
    category: 'form',
    desc: 'Dropdown pilihan tunggal dengan input pencarian, keyboard navigation, dan clearable.',
    props: 'modelValue, options, placeholder, searchable, clearable, size, align',
  },
  {
    id: 'BaseMultiSelect',
    name: 'BaseMultiSelect',
    category: 'form',
    desc: 'Dropdown multi-pilihan dengan chips, bar pilih semua, dan mode async search.',
    props: 'modelValue, options, asyncSearch, debounceMs, loading, selectAll, maxChips, size',
  },
  {
    id: 'BaseDatePicker',
    name: 'BaseDatePicker',
    category: 'form',
    desc: 'Kalender pemilih tanggal mandiri single dan rentang (range) dengan preset cepat.',
    props: 'modelValue, range, label, presets, clearable, size, placement',
  },
  {
    id: 'BaseTimePicker',
    name: 'BaseTimePicker',
    category: 'form',
    desc: 'Pemilih jam format HH:mm dan rentang shift masuk-pulang dengan preset tombol.',
    props: 'modelValue, range, label, minuteStep, presets, clearable, size',
  },
  {
    id: 'BaseFileUploader',
    name: 'BaseFileUploader',
    category: 'form',
    desc: 'Area upload file drag-and-drop dengan validasi ukuran MB dan pratinjau tipe file.',
    props: 'modelValue, label, hint, accept, maxSizeMB, multiple, disabled',
  },

  // Dialog & Panel
  {
    id: 'BaseModal',
    name: 'BaseModal',
    category: 'dialog',
    desc: 'Modal dialog adaptif (dialog tengah di desktop, bottom sheet di mobile).',
    props: 'modelValue, title, subtitle, size, adaptive, breakpoint, closeOnBackdrop',
  },
  {
    id: 'ConfirmationModal',
    name: 'ConfirmationModal',
    category: 'dialog',
    desc: 'Popup konfirmasi simpan atau hapus data dengan efek aksen tematik.',
    props: 'modelValue, type, badgeText, title, message, confirmText, cancelText',
  },
  {
    id: 'BaseSlideOver',
    name: 'BaseSlideOver',
    category: 'dialog',
    desc: 'Panel laci meluncur dari kanan layar untuk rincian data tanpa berpindah halaman.',
    props: 'modelValue, title, subtitle, size, closeOnEsc, closeOnClickOutside',
  },
  {
    id: 'BasePopover',
    name: 'BasePopover',
    category: 'dialog',
    desc: 'Kartu melayang interaktif (floating card) dengan slot trigger, header, body, footer.',
    props: 'modelValue, placement, widthClass, trigger, closeOnEsc, closeOnClickOutside',
  },
  {
    id: 'BaseTooltip',
    name: 'BaseTooltip',
    category: 'dialog',
    desc: 'Tooltip teks melayang saat kursor hover atau fokus tombol.',
    props: 'text, placement, delay, maxWidthClass',
  },

  // Data & Status
  {
    id: 'BaseDataTable',
    name: 'BaseDataTable',
    category: 'data',
    desc: 'Tabel enterprise sticky thead, sorting, mobile card, error retry, dan bulk actions.',
    props: 'columns, rows, totalItems, selectable, selectedIds, sortKey, sortOrder, paginated, error',
  },
  {
    id: 'BaseStatCard',
    name: 'BaseStatCard',
    category: 'data',
    desc: 'Kartu KPI ringkas dengan animasi BaseCountUp, tren persentase, dan skeleton.',
    props: 'title, value, prefix, suffix, unit, tone, trend, trendDirection, loading',
  },
  {
    id: 'BaseAlert',
    name: 'BaseAlert',
    category: 'data',
    desc: 'Pesan banner sistem atau inline (info, success, warning, danger, neutral).',
    props: 'variant, title, description, icon, dismissible, banner, actionText',
  },
  {
    id: 'BaseAvatar',
    name: 'BaseAvatar',
    category: 'data',
    desc: 'Foto avatar profil dengan inisial otomatis 2 huruf dan titik status online.',
    props: 'src, name, initials, size, shape, status, tone',
  },
  {
    id: 'BaseProgress',
    name: 'BaseProgress',
    category: 'data',
    desc: 'Bilah progres visual dengan format persentase/fraksi, varian warna, dan striped.',
    props: 'value, max, label, showValue, valueFormat, variant, size, striped, indeterminate',
  },
  {
    id: 'BaseTimeline',
    name: 'BaseTimeline',
    category: 'data',
    desc: 'Garis waktu audit jejak aktivitas berurutan dengan node status bertema.',
    props: 'items, compact',
  },
  {
    id: 'BaseEmptyState',
    name: 'BaseEmptyState',
    category: 'data',
    desc: 'Status kosong bila data tidak ditemukan, lengkap dengan tombol reset aksi.',
    props: 'title, description, icon, actionText, compact',
  },
  {
    id: 'BaseSkeleton',
    name: 'BaseSkeleton',
    category: 'data',
    desc: 'Placeholder pemuatan data dengan animasi sapuan kilau shimmer wave.',
    props: 'variant, width, height, lines, shimmer',
  },
  {
    id: 'BaseCountUp',
    name: 'BaseCountUp',
    category: 'data',
    desc: 'Animasi angka menghitung mulus dari nol ke target nilai.',
    props: 'to, from, duration, decimals, prefix, suffix, formatNumber',
  },

  // Navigasi & Tata Letak
  {
    id: 'BaseTabs',
    name: 'BaseTabs',
    category: 'nav',
    desc: 'Tab navigasi 3 format (underline, pills, cards) dengan slot panel dinamis.',
    props: 'modelValue, tabs, variant, size, fullWidth',
  },
  {
    id: 'BaseStepper',
    name: 'BaseStepper',
    category: 'nav',
    desc: 'Indikator langkah alur persetujuan horizontal (completed, current, upcoming, rejected).',
    props: 'steps, currentStep, clickable, size',
  },
  {
    id: 'BaseBreadcrumb',
    name: 'BaseBreadcrumb',
    category: 'nav',
    desc: 'Navigasi breadcrumb berjenjang dengan ikon beranda dan tautan router.',
    props: 'items, showHome',
  },
  {
    id: 'BaseAccordion',
    name: 'BaseAccordion',
    category: 'nav',
    desc: 'Panel accordion buka-tutup untuk FAQ, regulasi, atau rincian panjang.',
    props: 'items, modelValue, multiple, defaultOpen',
  },
  {
    id: 'BaseSelectableCard',
    name: 'BaseSelectableCard',
    category: 'nav',
    desc: 'Kartu pilihan yang dapat diklik dengan indikator centang bulat dan badge.',
    props: 'modelValue, value, multiple, title, description, badge, icon',
  },
  {
    id: 'BaseDescriptionList',
    name: 'BaseDescriptionList',
    category: 'nav',
    desc: 'Daftar rincian pasangan label dan nilai dalam format kartu grid atau baris terbagi.',
    props: 'items, columns, variant',
  },

  // Spesial & Media
  {
    id: 'BaseTree',
    name: 'BaseTree',
    category: 'special',
    desc: 'Pohon hierarkis node dengan expand/collapse dan checkbox cascade.',
    props: 'nodes, modelValue, selectable, multiple, cascade, searchQuery',
  },
  {
    id: 'BaseTreeSelect',
    name: 'BaseTreeSelect',
    category: 'special',
    desc: 'Dropdown pemilih pohon hierarki organisasi di dalam floating popover.',
    props: 'modelValue, nodes, multiple, cascade, label, placeholder, searchable',
  },
  {
    id: 'BaseCalendar',
    name: 'BaseCalendar',
    category: 'special',
    desc: 'Kalender bulanan 7 kolom mandiri dengan hari libur nasional dan event pills.',
    props: 'modelValue, events, holidays, selectable',
  },
  {
    id: 'BaseCameraCapture',
    name: 'BaseCameraCapture',
    category: 'special',
    desc: 'Kamera selfie presensi langsung via WebRTC dengan bingkai wajah dan watermark canvas.',
    props: 'modelValue, facingMode, aspectRatio, watermarkText, mirrored, label',
  },
  {
    id: 'BaseMapPolygonEditor',
    name: 'BaseMapPolygonEditor',
    category: 'special',
    desc: 'Editor peta poligon geofence berbasis SVG & OpenStreetMap tiles tanpa library eksternal.',
    props: 'modelValue, center, zoom, editable, height, label',
  },
]

// State Pencarian & Pilihan
const searchQuery = ref('')
const selectedCategory = ref('all')
const selectedComponentId = ref('BaseButton')
const copiedCode = ref(false)

const filteredList = computed(() => {
  const q = searchQuery.value.trim().toLowerCase()
  return componentCatalog.filter((item) => {
    const matchCat = selectedCategory.value === 'all' || item.category === selectedCategory.value
    const matchQuery = !q || item.name.toLowerCase().includes(q) || item.desc.toLowerCase().includes(q)
    return matchCat && matchQuery
  })
})

const activeComponent = computed(() => {
  return componentCatalog.find((c) => c.id === selectedComponentId.value) || componentCatalog[0]
})

const selectComponent = (id) => {
  selectedComponentId.value = id
}

// Copy Code Helper
const copyCodeSnippet = (text) => {
  navigator.clipboard.writeText(text)
  copiedCode.value = true
  toast.success('Contoh kode disalin ke clipboard!')
  setTimeout(() => {
    copiedCode.value = false
  }, 2000)
}

// ========================================================
// 2. LIVE INTERACTIVE DEMO STATES
// ========================================================
const demoInputVal = ref('')
const demoCurrencyVal = ref(15000000)
const demoNumberVal = ref(15)
const demoPasswordVal = ref('Digaji#2026')
const demoNikVal = ref('3201012345670001')
const demoTextareaVal = ref('Karyawan memiliki performa istimewa.')
const demoSwitchVal = ref(true)
const demoCheckboxVal = ref(['notif-wa', 'notif-email'])
const demoRadioVal = ref('bulanan')
const demoSelectVal = ref('Tetap')
const demoMultiSelectVal = ref(['bpjs', 'laptop'])
const demoDateVal = ref('2025-01-27')
const demoTimeVal = ref('08:30')
const demoModalOpen = ref(false)
const demoConfirmOpen = ref(false)
const demoSlideOpen = ref(false)
const demoTabsVal = ref('tab1')
const demoProgressVal = ref(70)
const demoCardVal = ref('pro')
const demoCalendarDate = ref('2025-01-27')
const demoSelfiePhoto = ref('')
const demoGeofence = ref([
  [
    { lat: -6.2088, lng: 106.8456 },
    { lat: -6.2088, lng: 106.8476 },
    { lat: -6.2104, lng: 106.8476 },
    { lat: -6.2104, lng: 106.8456 },
  ],
])

const demoTreeNodes = [
  {
    id: 'div-tech',
    label: 'Divisi Teknologi & Produk',
    children: [
      { id: 'dept-fe', label: 'Frontend Engineering' },
      { id: 'dept-be', label: 'Backend & Cloud' },
    ],
  },
  {
    id: 'div-hc',
    label: 'Divisi Human Capital',
    children: [
      { id: 'dept-payroll', label: 'Payroll & Kompensasi' },
    ],
  },
]
const demoTreeVal = ref(['dept-fe'])
</script>

<template>
  <div class="h-full flex flex-col min-h-0 overflow-hidden gap-2.5 w-full max-w-7xl mx-auto select-none relative">
    <!-- Top Header Bar -->
    <div class="flex flex-col sm:flex-row sm:items-center justify-between gap-2.5 shrink-0 pb-2 border-b border-neutral-200 dark:border-[var(--theme-border)]">
      <div class="flex items-center gap-2.5">
        <div class="w-9 h-9 rounded-xl bg-[var(--theme-primary)] text-[var(--theme-primary-text)] flex items-center justify-center shrink-0 shadow-2xs">
          <Component class="w-4.5 h-4.5" />
        </div>
        <div>
          <div class="flex items-center gap-2">
            <h1 class="text-base sm:text-lg font-bold text-neutral-900 dark:text-white tracking-tight leading-none">
              Katalog & Playground Komponen UI
            </h1>
            <BaseBadge variant="success" size="sm">{{ componentCatalog.length }} Komponen</BaseBadge>
          </div>
          <p class="text-xs text-neutral-500 dark:text-neutral-400 mt-0.5">
            Eksplorasi seluruh komponen Base buatan sendiri, uji interaksi langsung, dan salin kode pemakaian.
          </p>
        </div>
      </div>

      <!-- Quick Category Pills -->
      <div class="flex items-center gap-1 overflow-x-auto no-scrollbar shrink-0">
        <button
          v-for="cat in categories"
          :key="cat.id"
          type="button"
          @click="selectedCategory = cat.id"
          class="px-2.5 py-1 rounded-xl text-xs font-semibold whitespace-nowrap transition cursor-pointer"
          :class="
            selectedCategory === cat.id
              ? 'bg-[var(--theme-primary)] text-[var(--theme-primary-text)] shadow-2xs'
              : 'bg-neutral-100 dark:bg-[var(--theme-layer-2)] text-neutral-600 dark:text-neutral-300 hover:bg-neutral-200/70 dark:hover:bg-[var(--theme-surface-raised)]'
          "
        >
          {{ cat.label }}
        </button>
      </div>
    </div>

    <!-- Main Workspace: Master-Detail Layout -->
    <div class="flex-1 min-h-0 flex flex-col md:flex-row gap-3 overflow-hidden">
      <!-- ======================================================== -->
      <!-- SISI KIRI: DAFTAR ITEM NAMA KOMPONEN                     -->
      <!-- ======================================================== -->
      <div class="w-full md:w-72 lg:w-80 shrink-0 flex flex-col rounded-2xl border border-neutral-200 dark:border-[var(--theme-border)] bg-white dark:bg-[var(--theme-surface)] shadow-2xs overflow-hidden max-h-48 md:max-h-full">
        <!-- Search Input Bar -->
        <div class="p-2.5 border-b border-neutral-200/80 dark:border-[var(--theme-border)] bg-neutral-50/70 dark:bg-[var(--theme-layer-2)] shrink-0">
          <div class="relative">
            <Search class="w-3.5 h-3.5 text-neutral-400 absolute left-2.5 top-1/2 -translate-y-1/2 pointer-events-none" />
            <input
              v-model="searchQuery"
              type="text"
              placeholder="Cari komponen..."
              class="w-full pl-8 pr-3 py-1.5 rounded-xl text-xs bg-white dark:bg-[var(--theme-surface-raised)] border border-neutral-200/80 dark:border-[var(--theme-border)] text-neutral-900 dark:text-white placeholder:text-neutral-400 focus:outline-none focus:ring-2 focus:ring-[var(--theme-primary)]/20"
            />
          </div>
        </div>

        <!-- Scrollable List Items -->
        <div class="flex-1 min-h-0 overflow-y-auto p-1.5 space-y-1 custom-scroll">
          <button
            v-for="item in filteredList"
            :key="item.id"
            type="button"
            @click="selectComponent(item.id)"
            class="w-full px-3 py-2 rounded-xl text-left text-xs transition flex items-center justify-between gap-2 select-none cursor-pointer group"
            :class="
              selectedComponentId === item.id
                ? 'bg-[var(--theme-primary)] text-[var(--theme-primary-text)] font-bold shadow-xs'
                : 'text-neutral-700 dark:text-neutral-300 hover:bg-neutral-100 dark:hover:bg-[var(--theme-surface-raised)] hover:text-neutral-900 dark:hover:text-white'
            "
          >
            <div class="min-w-0 flex-1">
              <span class="block truncate font-mono text-[11.5px]">{{ item.name }}</span>
              <span
                class="block truncate text-[10.5px] opacity-75 font-sans font-normal mt-0.5"
                :class="selectedComponentId === item.id ? 'text-[var(--theme-primary-text)]' : 'text-neutral-400 dark:text-neutral-500'"
              >
                {{ item.desc }}
              </span>
            </div>

            <ChevronRight
              class="w-3.5 h-3.5 shrink-0 opacity-60 transition-transform group-hover:translate-x-0.5"
              :class="selectedComponentId === item.id ? 'opacity-100' : ''"
            />
          </button>

          <div
            v-if="filteredList.length === 0"
            class="p-4 text-center text-xs text-neutral-400 dark:text-neutral-500"
          >
            Tidak ada komponen yang cocok.
          </div>
        </div>
      </div>

      <!-- ======================================================== -->
      <!-- SISI KANAN: PRATINJAU INTERAKTIF & PROPS DETAIL          -->
      <!-- ======================================================== -->
      <div class="flex-1 min-h-0 flex flex-col rounded-2xl border border-neutral-200 dark:border-[var(--theme-border)] bg-white dark:bg-[var(--theme-surface)] shadow-2xs overflow-hidden">
        <!-- Component Header Info -->
        <div class="px-5 py-3.5 border-b border-neutral-200/80 dark:border-[var(--theme-border)] bg-neutral-50/70 dark:bg-[var(--theme-layer-2)] shrink-0 flex items-center justify-between gap-3">
          <div class="min-w-0">
            <div class="flex items-center gap-2">
              <h2 class="text-sm sm:text-base font-bold text-neutral-900 dark:text-white font-mono">
                {{ activeComponent.name }}.vue
              </h2>
              <BaseBadge variant="neutral" size="sm">
                {{ categories.find(c => c.id === activeComponent.category)?.label }}
              </BaseBadge>
            </div>
            <p class="text-xs text-neutral-500 dark:text-neutral-400 mt-0.5 truncate">
              {{ activeComponent.desc }}
            </p>
          </div>

          <div class="shrink-0 flex items-center gap-1.5">
            <BaseButton
              size="sm"
              variant="outline"
              @click="copyCodeSnippet(`<${activeComponent.name} />`)"
              title="Salin Tag Komponen"
            >
              <Copy class="w-3.5 h-3.5" />
              <span class="hidden sm:inline">{{ copiedCode ? 'Tersalin' : 'Salin Tag' }}</span>
            </BaseButton>
          </div>
        </div>

        <!-- Scrollable Playground Body -->
        <div class="flex-1 min-h-0 overflow-y-auto p-5 space-y-6 custom-scroll">
          <!-- LIVE INTERACTIVE PREVIEW CARD -->
          <div class="space-y-2.5">
            <div class="flex items-center justify-between">
              <span class="text-[11px] font-bold uppercase tracking-wider text-neutral-400">
                Uji Interaksi Langsung (Live Interactive Preview)
              </span>
              <BaseBadge variant="success" size="sm" dot pulse>Live Reactive</BaseBadge>
            </div>

            <div class="p-5 rounded-2xl border border-neutral-200/90 dark:border-[var(--theme-border)] bg-neutral-50/50 dark:bg-[var(--theme-layer-2)]/50 min-h-[160px] flex items-center justify-center">
              <!-- RENDER SPECIFIC COMPONENT LIVE -->

              <!-- 1. BaseButton -->
              <div v-if="activeComponent.id === 'BaseButton'" class="flex flex-wrap items-center gap-2.5">
                <BaseButton variant="primary" size="md" @click="toast.success('Tombol Primary diklik!')">
                  Primary Button
                </BaseButton>
                <BaseButton variant="secondary" size="md" @click="toast.info('Tombol Secondary diklik!')">
                  Secondary
                </BaseButton>
                <BaseButton variant="outline" size="md">
                  Outline
                </BaseButton>
                <BaseButton variant="danger" size="md">
                  Danger
                </BaseButton>
                <BaseButton variant="ghost" size="md">
                  Ghost
                </BaseButton>
                <BaseButton variant="primary" size="sm" disabled>
                  Disabled
                </BaseButton>
              </div>

              <!-- 2. BaseBadge -->
              <div v-else-if="activeComponent.id === 'BaseBadge'" class="flex flex-wrap items-center gap-2">
                <BaseBadge variant="success" dot pulse>Tetap</BaseBadge>
                <BaseBadge variant="warning">Kontrak</BaseBadge>
                <BaseBadge variant="danger">Ditolak</BaseBadge>
                <BaseBadge variant="info">Dalam Proses</BaseBadge>
                <BaseBadge variant="purple">Enterprise</BaseBadge>
                <BaseBadge variant="neutral">Draft</BaseBadge>
              </div>

              <!-- 3. BaseInput -->
              <div v-else-if="activeComponent.id === 'BaseInput'" class="w-full max-w-sm space-y-2">
                <BaseInput
                  v-model="demoInputVal"
                  label="Nama Karyawan"
                  placeholder="Ketik nama lengkap..."
                  required
                  clearable
                />
                <span class="text-[11px] font-mono text-neutral-400">v-model: {{ demoInputVal || '(kosong)' }}</span>
              </div>

              <!-- 4. BaseCurrencyInput -->
              <div v-else-if="activeComponent.id === 'BaseCurrencyInput'" class="w-full max-w-sm space-y-2">
                <BaseCurrencyInput
                  v-model="demoCurrencyVal"
                  label="Gaji Pokok Karyawan"
                  suffix="/ bulan"
                  terbilang
                />
              </div>

              <!-- 5. BaseNumberInput -->
              <div v-else-if="activeComponent.id === 'BaseNumberInput'" class="w-full max-w-xs space-y-2">
                <BaseNumberInput
                  v-model="demoNumberVal"
                  label="Toleransi Terlambat"
                  suffix="menit"
                  :min="0"
                  :max="60"
                />
              </div>

              <!-- 6. BasePasswordInput -->
              <div v-else-if="activeComponent.id === 'BasePasswordInput'" class="w-full max-w-sm space-y-2">
                <BasePasswordInput
                  v-model="demoPasswordVal"
                  label="Kata Sandi Akun"
                  show-strength
                />
              </div>

              <!-- 7. BaseMaskedInput -->
              <div v-else-if="activeComponent.id === 'BaseMaskedInput'" class="w-full max-w-sm space-y-2">
                <BaseMaskedInput
                  v-model="demoNikVal"
                  preset="nik"
                  label="NIK 16 Digit (Auto Masking)"
                />
                <span class="text-[11px] font-mono text-neutral-400">Digit Murni v-model: {{ demoNikVal }}</span>
              </div>

              <!-- 8. BaseTextarea -->
              <div v-else-if="activeComponent.id === 'BaseTextarea'" class="w-full max-w-sm space-y-2">
                <BaseTextarea
                  v-model="demoTextareaVal"
                  label="Catatan HR"
                  placeholder="Tuliskan catatan..."
                  :rows="2"
                  :maxlength="200"
                />
              </div>

              <!-- 9. BaseSwitch -->
              <div v-else-if="activeComponent.id === 'BaseSwitch'" class="w-full max-w-sm">
                <BaseSwitch
                  v-model="demoSwitchVal"
                  label="Kirim Slip Gaji Otomatis"
                  description="Kirim email notifikasi PDF saat payroll selesai diproses."
                />
              </div>

              <!-- 10. BaseCheckbox -->
              <div v-else-if="activeComponent.id === 'BaseCheckbox'" class="space-y-3">
                <BaseCheckbox
                  v-model="demoCheckboxVal"
                  direction="horizontal"
                  :options="[
                    { value: 'notif-wa', label: 'Notifikasi WhatsApp' },
                    { value: 'notif-email', label: 'Notifikasi Email' },
                    { value: 'notif-sms', label: 'Notifikasi SMS' },
                  ]"
                />
                <span class="text-[11px] font-mono text-neutral-400 block">Dipilih: {{ demoCheckboxVal }}</span>
              </div>

              <!-- 11. BaseRadio -->
              <div v-else-if="activeComponent.id === 'BaseRadio'" class="space-y-3">
                <BaseRadio
                  v-model="demoRadioVal"
                  direction="horizontal"
                  :options="[
                    { value: 'bulanan', label: 'Skema Bulanan' },
                    { value: 'mingguan', label: 'Skema Mingguan' },
                  ]"
                />
                <span class="text-[11px] font-mono text-neutral-400 block">v-model: {{ demoRadioVal }}</span>
              </div>

              <!-- 12. BaseFormField -->
              <div v-else-if="activeComponent.id === 'BaseFormField'" class="w-full max-w-sm">
                <BaseFormField label="Divisi Karyawan" required help-text="Pilih divisi penempatan kerja resmi.">
                  <BaseSearchableSelect
                    v-model="demoSelectVal"
                    :options="['Tetap', 'Kontrak', 'Probasi']"
                    size="md"
                  />
                </BaseFormField>
              </div>

              <!-- 13. BaseSearchableSelect -->
              <div v-else-if="activeComponent.id === 'BaseSearchableSelect'" class="w-full max-w-sm">
                <BaseSearchableSelect
                  v-model="demoSelectVal"
                  :options="['Tetap', 'Kontrak', 'Magang', 'Freelance']"
                  size="md"
                />
              </div>

              <!-- 14. BaseMultiSelect -->
              <div v-else-if="activeComponent.id === 'BaseMultiSelect'" class="w-full max-w-sm">
                <BaseMultiSelect
                  v-model="demoMultiSelectVal"
                  :options="[
                    { value: 'bpjs', label: 'BPJS Ketenagakerjaan', badge: 'Wajib' },
                    { value: 'laptop', label: 'MacBook Pro Kantor' },
                    { value: 'transport', label: 'Tunjangan Transportasi' },
                    { value: 'pulsa', label: 'Kredit Pulsa Data' },
                  ]"
                  placeholder="Pilih fasilitas..."
                />
              </div>

              <!-- 15. BaseDatePicker -->
              <div v-else-if="activeComponent.id === 'BaseDatePicker'" class="w-full max-w-sm">
                <BaseDatePicker
                  v-model="demoDateVal"
                  label="Pilih Tanggal Presensi"
                />
              </div>

              <!-- 16. BaseTimePicker -->
              <div v-else-if="activeComponent.id === 'BaseTimePicker'" class="w-full max-w-xs">
                <BaseTimePicker
                  v-model="demoTimeVal"
                  label="Jam Masuk Kerja"
                />
              </div>

              <!-- 17. BaseFileUploader -->
              <div v-else-if="activeComponent.id === 'BaseFileUploader'" class="w-full max-w-md">
                <BaseFileUploader
                  label="Unggah Berkas KTP / NPWP"
                  hint="Format: JPG, PNG, atau PDF (maks. 5MB)"
                />
              </div>

              <!-- 18. BaseModal -->
              <div v-else-if="activeComponent.id === 'BaseModal'" class="text-center space-y-2">
                <BaseButton variant="primary" @click="demoModalOpen = true">
                  Buka Dialog BaseModal
                </BaseButton>
                <BaseModal v-model="demoModalOpen" title="Contoh BaseModal Dialog" subtitle="Adaptif desktop & mobile drawer" size="md">
                  <p class="text-xs text-neutral-600 dark:text-neutral-300">
                    Modal adaptif yang otomatis bertransformasi menjadi Bottom Sheet di layar HP.
                  </p>
                </BaseModal>
              </div>

              <!-- 19. ConfirmationModal -->
              <div v-else-if="activeComponent.id === 'ConfirmationModal'" class="text-center space-y-2">
                <BaseButton variant="danger" @click="demoConfirmOpen = true">
                  Uji Coba ConfirmationModal
                </BaseButton>
                <ConfirmationModal
                  v-model="demoConfirmOpen"
                  type="delete"
                  title="Hapus Data Karyawan?"
                  message="Tindakan ini tidak dapat dibatalkan."
                  @confirm="toast.error('Data berhasil dihapus!')"
                />
              </div>

              <!-- 20. BaseSlideOver -->
              <div v-else-if="activeComponent.id === 'BaseSlideOver'" class="text-center space-y-2">
                <BaseButton variant="outline" @click="demoSlideOpen = true">
                  Buka BaseSlideOver Drawer
                </BaseButton>
                <BaseSlideOver v-model="demoSlideOpen" title="Detail Pegawai" subtitle="Panel geser samping" size="md">
                  <p class="text-xs text-neutral-600 dark:text-neutral-300">Panel drawer samping yang elegan.</p>
                </BaseSlideOver>
              </div>

              <!-- 21. BasePopover & BaseTooltip -->
              <div v-else-if="activeComponent.id === 'BasePopover' || activeComponent.id === 'BaseTooltip'" class="flex items-center gap-4">
                <BaseTooltip text="Ini adalah tooltip bantuan instan">
                  <BaseButton variant="outline" size="sm">Hover Tooltip</BaseButton>
                </BaseTooltip>
                <BasePopover placement="bottom-end" width-class="w-64">
                  <template #trigger="{ toggle }">
                    <BaseButton variant="primary" size="sm" @click="toggle">Buka Popover</BaseButton>
                  </template>
                  <p class="text-xs p-1">Konten melayang di dalam BasePopover.</p>
                </BasePopover>
              </div>

              <!-- 22. BaseDataTable -->
              <div v-else-if="activeComponent.id === 'BaseDataTable'" class="w-full h-56">
                <BaseDataTable
                  :columns="[
                    { key: 'nama', label: 'Nama Pegawai', sortable: true },
                    { key: 'divisi', label: 'Divisi' },
                    { key: 'status', label: 'Status' },
                  ]"
                  :rows="[
                    { id: 1, nama: 'Rizky Pratama', divisi: 'Engineering', status: 'Tetap' },
                    { id: 2, nama: 'Nadia Putri', divisi: 'Design', status: 'Kontrak' },
                  ]"
                  :total-items="2"
                  selectable
                  :items-per-page="5"
                >
                  <template #cell-status="{ value }">
                    <BaseBadge :variant="value === 'Tetap' ? 'success' : 'warning'">{{ value }}</BaseBadge>
                  </template>
                </BaseDataTable>
              </div>

              <!-- 23. BaseStatCard -->
              <div v-else-if="activeComponent.id === 'BaseStatCard'" class="w-full max-w-xs">
                <BaseStatCard
                  title="Realisasi Gaji"
                  :value="184500000"
                  prefix="Rp "
                  :icon="Wallet"
                  tone="emerald"
                  trend="+12.5%"
                  trend-direction="up"
                  trend-label="bulan ini"
                />
              </div>

              <!-- 24. BaseAlert -->
              <div v-else-if="activeComponent.id === 'BaseAlert'" class="w-full max-w-md space-y-2">
                <BaseAlert
                  variant="info"
                  title="Pembaruan Skema Payroll"
                  description="Sistem otomatis menghitung tarif PPh 21 TER 2024."
                  action-text="Pelajari"
                  dismissible
                />
              </div>

              <!-- 25. BaseAvatar -->
              <div v-else-if="activeComponent.id === 'BaseAvatar'" class="flex items-center gap-3">
                <BaseAvatar name="Rizky Pratama" size="sm" status="online" tone="primary" />
                <BaseAvatar name="Nadia Putri" size="md" status="busy" tone="emerald" />
                <BaseAvatar name="Budi Santoso" size="lg" status="away" tone="amber" />
                <BaseAvatar name="Ahmad Dahlan" size="xl" tone="rose" />
              </div>

              <!-- 26. BaseTabs -->
              <div v-else-if="activeComponent.id === 'BaseTabs'" class="w-full max-w-md">
                <BaseTabs
                  v-model="demoTabsVal"
                  variant="pills"
                  :tabs="[
                    { value: 'tab1', label: 'Ringkasan', badge: '3' },
                    { value: 'tab2', label: 'Kompensasi' },
                    { value: 'tab3', label: 'Riwayat' },
                  ]"
                />
              </div>

              <!-- 27. BaseStepper -->
              <div v-else-if="activeComponent.id === 'BaseStepper'" class="w-full max-w-lg">
                <BaseStepper
                  :steps="[
                    { id: 1, title: 'Input HR', approver: 'Sari R.', status: 'completed' },
                    { id: 2, title: 'Verifikasi Finansial', approver: 'Budi S.', status: 'current' },
                    { id: 3, title: 'Disetujui Direksi', approver: 'Direktur', status: 'upcoming' },
                  ]"
                  :current-step="1"
                />
              </div>

              <!-- 28. BaseProgress -->
              <div v-else-if="activeComponent.id === 'BaseProgress'" class="w-full max-w-sm space-y-3">
                <BaseProgress :value="demoProgressVal" :max="100" label="Kapasitas Karyawan Digunakan" variant="primary" striped />
                <div class="flex items-center gap-2">
                  <BaseButton size="sm" variant="outline" @click="demoProgressVal = Math.max(0, demoProgressVal - 10)">-10%</BaseButton>
                  <BaseButton size="sm" variant="outline" @click="demoProgressVal = Math.min(100, demoProgressVal + 10)">+10%</BaseButton>
                </div>
              </div>

              <!-- 29. BaseAccordion -->
              <div v-else-if="activeComponent.id === 'BaseAccordion'" class="w-full max-w-md">
                <BaseAccordion
                  :items="[
                    { id: 'acc1', title: 'Kebijakan Lembur Kemnaker', content: 'Jam kerja lembur dihitung 1,5x untuk jam pertama dan 2x berikutnya.' },
                    { id: 'acc2', title: 'Aturan PPh 21 TER', content: 'Kategori TER A, B, C disesuaikan dengan status PTKP karyawan.' },
                  ]"
                  default-open="acc1"
                />
              </div>

              <!-- 30. BaseSelectableCard -->
              <div v-else-if="activeComponent.id === 'BaseSelectableCard'" class="grid grid-cols-1 sm:grid-cols-2 gap-3 w-full max-w-md">
                <BaseSelectableCard v-model="demoCardVal" value="starter" title="Paket Starter" description="Hingga 50 pegawai." />
                <BaseSelectableCard v-model="demoCardVal" value="pro" title="Paket Pro" description="Pegawai tanpa batas." badge="Rekomendasi" badge-variant="info" />
              </div>

              <!-- 31. BaseDescriptionList -->
              <div v-else-if="activeComponent.id === 'BaseDescriptionList'" class="w-full max-w-md">
                <BaseDescriptionList
                  :columns="2"
                  :items="[
                    { label: 'Email Kantor', value: 'rizky@digaji.id' },
                    { label: 'Telepon', value: '0812-3456-7890', mono: true },
                    { label: 'Status', value: 'Tetap', badge: 'Aktif', badgeVariant: 'success' },
                    { label: 'Penempatan', value: 'Jakarta' },
                  ]"
                />
              </div>

              <!-- 32. BaseTree / BaseTreeSelect -->
              <div v-else-if="activeComponent.id === 'BaseTree' || activeComponent.id === 'BaseTreeSelect'" class="w-full max-w-sm">
                <BaseTreeSelect
                  v-model="demoTreeVal"
                  :nodes="demoTreeNodes"
                  multiple
                  label="Struktur Organisasi"
                />
              </div>

              <!-- 33. BaseCalendar -->
              <div v-else-if="activeComponent.id === 'BaseCalendar'" class="w-full max-w-lg">
                <BaseCalendar
                  v-model="demoCalendarDate"
                  :holidays="[{ date: '2025-01-01', label: 'Tahun Baru' }]"
                  :events="[{ id: 'e1', date: '2025-01-25', title: 'Cut-off Payroll', variant: 'danger' }]"
                />
              </div>

              <!-- 34. BaseCameraCapture -->
              <div v-else-if="activeComponent.id === 'BaseCameraCapture'" class="w-full max-w-xs">
                <BaseCameraCapture
                  v-model="demoSelfiePhoto"
                  watermark-text="Kantor Pusat Jakarta"
                />
              </div>

              <!-- 35. BaseMapPolygonEditor -->
              <div v-else-if="activeComponent.id === 'BaseMapPolygonEditor'" class="w-full max-w-md">
                <BaseMapPolygonEditor
                  v-model="demoGeofence"
                  height="260px"
                />
              </div>

              <!-- Fallback Default for any other component -->
              <div v-else class="text-center py-6">
                <Component class="w-8 h-8 text-neutral-400 mx-auto mb-2" />
                <p class="text-xs font-semibold text-neutral-700 dark:text-neutral-300">
                  Komponen {{ activeComponent.name }} Siap Dipakai
                </p>
                <p class="text-[11px] text-neutral-500 mt-1">
                  Lihat dokumentasi props dan contoh kode di bawah.
                </p>
              </div>
            </div>
          </div>

          <!-- CODE SNIPPET BOX -->
          <div class="space-y-2">
            <div class="flex items-center justify-between">
              <span class="text-[11px] font-bold uppercase tracking-wider text-neutral-400">
                Contoh Kode Pemakaian (Template Snippet)
              </span>
              <button
                type="button"
                @click="copyCodeSnippet(`<template>\n  <${activeComponent.name} />\n</template>`)"
                class="inline-flex items-center gap-1 text-[11px] text-[var(--theme-primary)] hover:underline cursor-pointer font-medium"
              >
                <Copy class="w-3 h-3" />
                <span>Salin Template</span>
              </button>
            </div>

            <div class="p-3.5 rounded-2xl bg-neutral-900 text-neutral-100 font-mono text-xs overflow-x-auto shadow-inner border border-neutral-800">
              <pre class="leading-relaxed"><code>&lt;script setup&gt;
import {{ activeComponent.name }} from '../../components/ui/{{ activeComponent.name }}.vue'
&lt;/script&gt;

&lt;template&gt;
  &lt;{{ activeComponent.name }} /&gt;
&lt;/template&gt;</code></pre>
            </div>
          </div>

          <!-- PROPS & EMITS SPECIFICATION TABLE -->
          <div class="space-y-2">
            <span class="text-[11px] font-bold uppercase tracking-wider text-neutral-400">
              Daftar Properti Terdaftar (defineProps)
            </span>
            <div class="p-3.5 rounded-2xl border border-neutral-200 dark:border-[var(--theme-border)] bg-neutral-50/50 dark:bg-[var(--theme-layer-2)]/50 text-xs">
              <p class="font-mono font-semibold text-neutral-800 dark:text-neutral-200 leading-relaxed">
                {{ activeComponent.props }}
              </p>
            </div>
          </div>
        </div>
      </div>
    </div>
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
