<script setup>
import { ref, computed } from 'vue'
import { useRoute } from 'vue-router'
import BaseButton from '../../components/ui/BaseButton.vue'
import BaseInput from '../../components/ui/BaseInput.vue'
import BaseCurrencyInput from '../../components/ui/BaseCurrencyInput.vue'
import BaseCountUp from '../../components/ui/BaseCountUp.vue'
import BaseSkeleton from '../../components/ui/BaseSkeleton.vue'
import BaseTextarea from '../../components/ui/BaseTextarea.vue'
import BaseSwitch from '../../components/ui/BaseSwitch.vue'
import BaseBadge from '../../components/ui/BaseBadge.vue'
import BaseModal from '../../components/ui/BaseModal.vue'
import MobileModal from '../../components/ui/MobileModal.vue'
import ConfirmationModal from '../../components/ui/ConfirmationModal.vue'
import BaseDatePicker from '../../components/ui/BaseDatePicker.vue'
import BaseEmptyState from '../../components/ui/BaseEmptyState.vue'
import BaseFileUploader from '../../components/ui/BaseFileUploader.vue'
import BaseTimeline from '../../components/ui/BaseTimeline.vue'
import BaseSlideOver from '../../components/ui/BaseSlideOver.vue'
import BaseInvoiceModal from '../../components/ui/BaseInvoiceModal.vue'
import BaseCsvImporter from '../../components/ui/BaseCsvImporter.vue'
import BasePasswordInput from '../../components/ui/BasePasswordInput.vue'
import BaseAlert from '../../components/ui/BaseAlert.vue'
import BaseTabs from '../../components/ui/BaseTabs.vue'
import BaseNumberInput from '../../components/ui/BaseNumberInput.vue'
import BaseTimePicker from '../../components/ui/BaseTimePicker.vue'
import BaseMaskedInput from '../../components/ui/BaseMaskedInput.vue'
import BaseRadio from '../../components/ui/BaseRadio.vue'
import BaseCheckbox from '../../components/ui/BaseCheckbox.vue'
import BaseTreeSelect from '../../components/ui/BaseTreeSelect.vue'
import BaseAccordion from '../../components/ui/BaseAccordion.vue'
import BaseBreadcrumb from '../../components/ui/BaseBreadcrumb.vue'
import BaseSelectableCard from '../../components/ui/BaseSelectableCard.vue'
import BaseProgress from '../../components/ui/BaseProgress.vue'
import BaseCalendar from '../../components/ui/BaseCalendar.vue'
import BaseCameraCapture from '../../components/ui/BaseCameraCapture.vue'
import BaseMapPolygonEditor from '../../components/ui/BaseMapPolygonEditor.vue'
import { useToast } from '../../composables/useToast'
import { useBackground } from '../../composables/useBackground'
import { useThemeColor } from '../../composables/useThemeColor'
import {
  Sliders,
  Palette,
  Shield,
  Bell,
  Grid,
  CircleDot,
  Image as ImageIcon,
  Save,
  Check,
  Layers,
  Monitor,
  Smartphone,
  Trash2,
  KeyRound,
  CheckCircle2,
  AlertTriangle,
  Info,
  XCircle,
  Sparkles,
  Calendar,
  FolderSearch,
  Upload,
  Printer,
  Eye,
  FileSpreadsheet,
} from 'lucide-vue-next'

const route = useRoute()
const toast = useToast()
const { bgType, solidColor, imageUrl, setBgType, setSolidColor } = useBackground()
const { themeColor, themePalettes, setThemeColor } = useThemeColor()

const handleThemeColorSelect = (paletteId) => {
  setThemeColor(paletteId)
  const pal = themePalettes[paletteId]
  toast.success(`Tema ${pal.name} berhasil diterapkan!`, 'Tema Warna')
}

const tokenListPink = [
  { token: 'Primary Brand', hex: '#E85D88', desc: 'Identitas utama tema Pink' },
  { token: 'Primary Action', hex: '#C83D6A', desc: 'Tombol utama berteks putih' },
  { token: 'Primary Hover', hex: '#AF2D58', desc: 'Hover tombol primary' },
  { token: 'Primary Pressed', hex: '#912044', desc: 'Tombol saat ditekan' },
  { token: 'Primary Soft', hex: '#FCE7EF', desc: 'Menu aktif, badge, ikon' },
  { token: 'Background', hex: '#FFF8FB', desc: 'Background utama aplikasi' },
  { token: 'Surface', hex: '#FFFFFF', desc: 'Card, modal, sidebar' },
  { token: 'Surface Muted', hex: '#FFF0F5', desc: 'Area sekunder' },
  { token: 'Border', hex: '#F4DDE6', desc: 'Border card dan input' },
  { token: 'Text Primary', hex: '#241923', desc: 'Heading dan teks utama' },
  { token: 'Text Secondary', hex: '#66515F', desc: 'Deskripsi dan label' },
  { token: 'Text Muted', hex: '#806B79', desc: 'Metadata dan placeholder' },
]

const tokenListBlue = [
  { token: 'Primary', hex: '#007AFF', desc: 'CTA utama, link, ikon aktif, tab aktif' },
  { token: 'Primary Hover', hex: '#0062CC', desc: 'Hover tombol primary' },
  { token: 'Primary Pressed', hex: '#004C99', desc: 'Tombol saat ditekan' },
  { token: 'Primary Soft', hex: '#E5F1FF', desc: 'Latar menu aktif, badge, ikon' },
  { token: 'Background', hex: '#F8FAFC', desc: 'Background utama aplikasi' },
  { token: 'Surface', hex: '#FFFFFF', desc: 'Card, modal, sidebar' },
  { token: 'Surface Muted', hex: '#F1F5F9', desc: 'Area sekunder' },
  { token: 'Border', hex: '#E2E8F0', desc: 'Garis pemisah, border input' },
  { token: 'Text Primary', hex: '#0F172A', desc: 'Heading dan teks utama' },
  { token: 'Text Secondary', hex: '#475569', desc: 'Deskripsi dan label' },
  { token: 'Text Muted', hex: '#64748B', desc: 'Metadata dan informasi tambahan' },
]

const tokenListDefault = [
  { token: 'Primary (Light)', hex: '#18181b', desc: 'CTA utama, sidebar aktif di mode terang' },
  { token: 'Primary (Dark)', hex: '#FFFFFF', desc: 'Tombol utama di pure neutral dark' },
  { token: 'Primary Soft', hex: '#F4F4F5', desc: 'Latar netral soft & badge' },
  { token: 'Background (Light)', hex: '#F8FAFC', desc: 'Dasar Pertama mode terang' },
  { token: 'Background (Dark)', hex: '#09090b', desc: 'Pure neutral dark (Zero Blue Tint)' },
  { token: 'Surface (Light)', hex: '#FFFFFF', desc: 'Card & modal Dasar Kedua' },
  { token: 'Surface (Dark)', hex: '#141417', desc: 'Floating card dark' },
  { token: 'Border', hex: '#E4E4E7 / #27272A', desc: 'Garis pembatas & border input' },
  { token: 'Text Primary', hex: '#18181b / #F4F4F5', desc: 'Heading dan teks utama' },
  { token: 'Text Secondary', hex: '#52525B / #A1A1AA', desc: 'Deskripsi dan label' },
]

const tokenListTerracotta = [
  { token: 'Primary Brand', hex: '#c03a18', desc: 'Identitas utama Terracotta Rust' },
  { token: 'Primary Action', hex: '#c03a18', desc: 'Tombol CTA utama berteks putih' },
  { token: 'Primary Hover', hex: '#ab3315', desc: 'Hover tombol primary' },
  { token: 'Primary Pressed', hex: '#8f270d', desc: 'Tombol saat ditekan' },
  { token: 'Primary Soft', hex: '#FDF2EE', desc: 'Menu aktif, badge, background tag' },
  { token: 'Background', hex: '#F3F1EC', desc: 'Background utama warm sand linen' },
  { token: 'Surface', hex: '#FFFFFF', desc: 'Card, modal, sidebar Dasar Kedua' },
  { token: 'Surface Muted', hex: '#F4F1EA', desc: 'Area sekunder warm ivory' },
  { token: 'Border', hex: '#E1DBD1', desc: 'Garis pemisah & border card' },
  { token: 'Text Primary', hex: '#211D17', desc: 'Heading & teks utama espresso black' },
  { token: 'Text Secondary', hex: '#5C564C', desc: 'Deskripsi & label warm stone' },
  { token: 'Text Muted', hex: '#8C8578', desc: 'Metadata & placeholder' },
]

const currentTokens = computed(() => {
  if (themeColor.value === 'pink') return tokenListPink
  if (themeColor.value === 'blue') return tokenListBlue
  if (themeColor.value === 'terracotta') return tokenListTerracotta
  return tokenListDefault
})

// Active tab determined by route or fallback
const activeTab = computed(() => {
  if (route.path.includes('dasar-pertama')) return 'dasar1'
  if (route.path.includes('modal')) return 'modal'
  if (route.path.includes('notifikasi')) return 'notifikasi'
  if (route.path.includes('keamanan')) return 'keamanan'
  return 'umum'
})

// General Settings Form State
const appName = ref('Digaji.id Multi-use UI')
const appDesc = ref('Template antarmuka Vue 3 modular dengan arsitektur 2 Layer (Dasar Pertama & Kedua) dan sistem tema pure neutral dark.')
const autoSaveEnabled = ref(true)
const debugMode = ref(false)
const demoNominal = ref(1000)
const demoCountVal = ref(7850000)
const demoSkelToggle = ref(false)
const demoSingleDate = ref('2025-01-27')
const demoRangeDate = ref({ start: '2025-01-20', end: '2025-01-27' })
const demoUploadedFiles = ref([])
const demoSlideOverOpen = ref(false)
const demoInvoiceOpen = ref(false)
const demoCsvOpen = ref(false)

// Showcase State untuk Komponen Prioritas 1 & 2
const demoTabsValue = ref('pills')
const demoNumberVal = ref(15)
const demoTimeVal = ref('08:30')
const demoTimeRange = ref({ start: '08:00', end: '17:00' })
const demoNik = ref('3201012345670001')
const demoNpwp = ref('092542943407000')
const demoPhone = ref('081234567890')
const demoRadioVal = ref('bulanan')
const demoCheckboxGroup = ref(['slip', 'wa'])
const demoTreeVal = ref(['dept-frontend'])
const demoProgressVal = ref(65)
const demoCardSelected = ref('paket-pro')
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
      { id: 'dept-frontend', label: 'Frontend Engineering' },
      { id: 'dept-backend', label: 'Backend & Cloud' },
      { id: 'dept-qa', label: 'QA & Automation' },
    ],
  },
  {
    id: 'div-hr',
    label: 'Divisi Human Capital & Payroll',
    children: [
      { id: 'dept-recruitment', label: 'Talent Acquisition' },
      { id: 'dept-payroll', label: 'Kompensasi & Benefit' },
    ],
  },
]

const demoTimelineItems = [
  { id: 1, title: 'Pembayaran Dikonfirmasi Lunas', description: 'Metode transfer QRIS BCA terverifikasi.', time: '10:30 WIB', variant: 'success', author: 'Payment Gateway' },
  { id: 2, title: 'File RIP Masuk Antrean Mesin', description: 'Resolusi 1200 DPI, profil CMYK disesuaikan.', time: '11:15 WIB', variant: 'primary', author: 'Operator Offset' },
  { id: 3, title: 'Pemeriksaan Quality Control', description: 'Hasil cetak dan presisi potong lolos uji 100%.', time: '13:00 WIB', variant: 'info', author: 'Tim QC' },
]

// Modals Showcase State (Moved from Beranda)
const isDesktopModalOpen = ref(false)
const isMobileModalOpen = ref(false)
const showSavePopup = ref(false)
const showDeletePopup = ref(false)

// Security Form State
const twoFactorAuth = ref(false)
const currentPassword = ref('')
const newPassword = ref('')

// Toast Notification Preferences State
const soundNotification = ref(true)
const autoDismissToast = ref(true)

// Toast System Triggers
const triggerSuccess = () => toast.success('Perubahan disimpan')
const triggerError = () => toast.error('Gagal menyimpan data')
const triggerWarning = () => toast.warning('Harap periksa pengaturan Anda')
const triggerInfo = () => toast.info('Template UI/UX siap digunakan')

const triggerSaveSuccessDemo = async () => {
  await toast.promise(
    new Promise((resolve) => setTimeout(resolve, 1400)),
    {
      loading: 'Menyimpan...',
      success: 'Perubahan disimpan',
      error: 'Gagal menyimpan perubahan',
    }
  )
}

const triggerSaveErrorDemo = async () => {
  try {
    await toast.promise(
      new Promise((_, reject) =>
        setTimeout(() => reject(new Error('Koneksi terputus')), 1400)
      ),
      {
        loading: 'Menyimpan...',
        success: 'Perubahan disimpan',
        error: 'Gagal menyimpan perubahan',
      }
    )
  } catch {
    // Handled in toast.promise
  }
}

const presetColors = [
  '#f8fafc',
  '#f4f4f5',
  '#e5e5e5',
  '#09090b',
  '#141417',
  '#18181b',
  '#27272a',
]

const handleSaveGeneral = () => {
  toast.success('Pengaturan umum berhasil disimpan!', 'Pengaturan')
}

const handleSavePassword = () => {
  if (!currentPassword.value || !newPassword.value) {
    toast.error('Harap isi semua kolom password', 'Validasi Gagal')
    return
  }
  currentPassword.value = ''
  newPassword.value = ''
  toast.success('Kata sandi berhasil diperbarui!', 'Keamanan')
}
</script>

<template>
  <div class="space-y-6 w-full max-w-4xl mx-auto">
    <!-- Header Setting -->
    <div class="border-b border-neutral-200 dark:border-[var(--theme-border)] pb-4 flex items-center justify-between">
      <div>
        <div class="flex items-center gap-2 mb-1">
          <span class="text-[10px] font-bold tracking-widest uppercase px-2 py-0.5 rounded-md bg-neutral-200 text-neutral-800 dark:bg-[var(--theme-surface-muted)] dark:text-neutral-300">
            SETTING
          </span>
          <BaseBadge variant="neutral" size="sm">
            {{ activeTab.toUpperCase() }}
          </BaseBadge>
        </div>
        <h1 class="text-2xl font-bold text-neutral-900 dark:text-[var(--theme-text-primary)] tracking-tight">
          Pengaturan Sistem
        </h1>
        <p class="text-xs text-neutral-500 dark:text-neutral-400 mt-0.5">
          Kelola kustomisasi Dasar Pertama, showcase modal & pop-up, toast, serta keamanan akun.
        </p>
      </div>
    </div>

    <!-- Tab Content with Fluid Transition -->
    <Transition name="tab-slide" mode="out-in">
      <div :key="activeTab" class="w-full space-y-6">
        
        <!-- ======================================================== -->
        <!-- 1. SUBMENU: PENGATURAN UMUM                              -->
        <!-- ======================================================== -->
        <div v-if="activeTab === 'umum'" class="space-y-5">
          <!-- Card Form Pengaturan Umum Menggunakan BaseInput & BaseTextarea -->
          <div class="p-5 rounded-2xl border border-neutral-200 dark:border-[var(--theme-border)] bg-white dark:bg-[var(--theme-surface)] shadow-2xs space-y-4">
            <h2 class="text-sm font-bold text-neutral-900 dark:text-white flex items-center gap-2">
              <Sliders class="w-4 h-4 text-neutral-700 dark:text-neutral-300" />
              <span>Profil & Informasi Aplikasi</span>
            </h2>

            <div class="space-y-3.5 max-w-xl">
              <BaseInput
                v-model="appName"
                label="Nama Aplikasi / Template"
                placeholder="Masukkan nama aplikasi..."
                required
                clearable
              />

              <BaseTextarea
                v-model="appDesc"
                label="Deskripsi Sistem"
                placeholder="Tuliskan deskripsi ringkas..."
                :rows="3"
                :maxlength="250"
                hint="Maksimal 250 karakter, otomatis menyesuaikan tinggi textarea."
              />
            </div>
          </div>

          <!-- Showcase Komponen BaseCurrencyInput.vue (Auto Rupiah & Terbilang) -->
          <div class="p-5 rounded-2xl border border-neutral-200 dark:border-[var(--theme-border)] bg-white dark:bg-[var(--theme-surface)] shadow-2xs space-y-4">
            <div class="flex items-center justify-between">
              <div>
                <h2 class="text-sm font-bold text-neutral-900 dark:text-white flex items-center gap-2">
                  <span class="w-2 h-2 rounded-full bg-[var(--theme-primary)]" />
                  <span>Komponen Baru: BaseCurrencyInput (Auto Rupiah)</span>
                </h2>
                <p class="text-xs text-neutral-500 mt-0.5">
                  Input angka otomatis berformat mata uang Rupiah Indonesia dengan pemisah ribuan titik (contoh: 1000 menjadi 1.000).
                </p>
              </div>
              <BaseBadge variant="success" size="sm">Auto Format</BaseBadge>
            </div>

            <div class="space-y-3.5 max-w-xl">
              <BaseCurrencyInput
                v-model="demoNominal"
                label="Uji Coba Input Nominal Rupiah"
                placeholder="0"
                required
                suffix="/ transaksi"
                hint="Ketik angka tanpa titik, format otomatis merapikan ribuan (misal: 1000 -> 1.000)."
                terbilang
              />

              <!-- Preview State Output -->
              <div class="p-3 rounded-xl bg-neutral-50 dark:bg-[var(--theme-surface-raised)] border border-neutral-200/80 dark:border-[var(--theme-border)] text-xs space-y-1.5">
                <div class="flex items-center justify-between">
                  <span class="text-neutral-500 font-medium">Nilai Murni v-model (Number):</span>
                  <span class="font-mono font-bold text-neutral-900 dark:text-white">{{ demoNominal }}</span>
                </div>
                <div class="flex items-center justify-between">
                  <span class="text-neutral-500 font-medium">Tipe Data:</span>
                  <span class="font-mono text-emerald-600 dark:text-emerald-400 font-bold">{{ typeof demoNominal }}</span>
                </div>
                <!-- Quick Test Buttons -->
                <div class="pt-2 border-t border-neutral-200/60 dark:border-[var(--theme-border)] flex items-center gap-2 flex-wrap">
                  <span class="text-[11px] text-neutral-400 font-medium">Uji Cepat:</span>
                  <button
                    v-for="amt in [1000, 50000, 1500000, 75000000]"
                    :key="amt"
                    type="button"
                    @click="demoNominal = amt"
                    class="px-2 py-0.5 rounded-md bg-white dark:bg-[var(--theme-surface-muted)] border border-neutral-200 dark:border-[var(--theme-border-strong)] text-[10.5px] font-mono text-neutral-700 dark:text-neutral-300 hover:border-neutral-400 transition cursor-pointer"
                  >
                    {{ amt.toLocaleString('id-ID') }}
                  </button>
                </div>
              </div>
            </div>
          </div>

          <!-- Showcase Komponen BaseCountUp.vue & BaseSkeleton.vue -->
          <div class="p-5 rounded-2xl border border-neutral-200 dark:border-[var(--theme-border)] bg-white dark:bg-[var(--theme-surface)] shadow-2xs space-y-4">
            <div class="flex items-center justify-between">
              <div>
                <h2 class="text-sm font-bold text-neutral-900 dark:text-white flex items-center gap-2">
                  <span class="w-2 h-2 rounded-full bg-[var(--theme-primary)]" />
                  <span>Komponen Baru: BaseCountUp & BaseSkeleton</span>
                </h2>
                <p class="text-xs text-neutral-500 mt-0.5">
                  Animasi angka bergulir (count-up ticker) dengan kurva halus dan shimmer wave skeleton loader untuk transisi data.
                </p>
              </div>
              <BaseBadge variant="purple" size="sm">Animasi & UI Base</BaseBadge>
            </div>

            <div class="grid grid-cols-1 sm:grid-cols-2 gap-4">
              <!-- Demo BaseCountUp -->
              <div class="p-4 rounded-xl bg-neutral-50 dark:bg-[var(--theme-surface-raised)] border border-neutral-200/80 dark:border-[var(--theme-border)] space-y-3">
                <div class="flex items-center justify-between">
                  <span class="text-xs font-bold text-neutral-700 dark:text-neutral-300">Live BaseCountUp</span>
                  <BaseBadge variant="info" size="sm">Ease-out Cubic</BaseBadge>
                </div>
                <div class="text-2xl font-bold font-mono text-[var(--theme-primary)]">
                  <BaseCountUp :to="demoCountVal" prefix="Rp " />
                </div>
                <div class="flex items-center gap-2 flex-wrap">
                  <button
                    v-for="val in [142, 50000, 7850000, 250000000]"
                    :key="val"
                    type="button"
                    @click="demoCountVal = val"
                    class="px-2 py-0.5 rounded-md bg-white dark:bg-[var(--theme-surface-muted)] border border-neutral-200 dark:border-[var(--theme-border-strong)] text-[10.5px] font-mono hover:border-neutral-400 transition cursor-pointer"
                  >
                    Rp {{ val.toLocaleString('id-ID') }}
                  </button>
                </div>
              </div>

              <!-- Demo BaseSkeleton -->
              <div class="p-4 rounded-xl bg-neutral-50 dark:bg-[var(--theme-surface-raised)] border border-neutral-200/80 dark:border-[var(--theme-border)] space-y-3">
                <div class="flex items-center justify-between">
                  <span class="text-xs font-bold text-neutral-700 dark:text-neutral-300">Live BaseSkeleton (Shimmer)</span>
                  <button
                    type="button"
                    @click="demoSkelToggle = !demoSkelToggle"
                    class="text-[10.5px] font-semibold text-[var(--theme-primary)] hover:underline cursor-pointer"
                  >
                    {{ demoSkelToggle ? 'Tampilkan Asli' : 'Simulasi Loading' }}
                  </button>
                </div>

                <div v-if="demoSkelToggle" class="space-y-2">
                  <BaseSkeleton width="60%" height="14px" />
                  <BaseSkeleton width="90%" height="11px" />
                  <BaseSkeleton width="40%" height="11px" />
                </div>
                <div v-else class="space-y-1 text-xs">
                  <div class="font-bold text-neutral-900 dark:text-white">PT Digaji Solusi Indonesia</div>
                  <div class="text-neutral-500">Sistem manajemen penggajian dan absensi terintegrasi.</div>
                  <div class="text-[10px] text-emerald-600 font-semibold">● 100% Layanan Normal</div>
                </div>
              </div>
            </div>
          </div>

          <!-- Showcase Komponen BaseDatePicker & BaseEmptyState -->
          <div class="p-5 rounded-2xl border border-neutral-200 dark:border-[var(--theme-border)] bg-white dark:bg-[var(--theme-surface)] shadow-2xs space-y-5">
            <div class="flex items-center justify-between">
              <div>
                <h2 class="text-sm font-bold text-neutral-900 dark:text-white flex items-center gap-2">
                  <Calendar class="w-4 h-4 text-[var(--theme-primary)]" />
                  <span>Komponen Baru: BaseDatePicker & BaseEmptyState</span>
                </h2>
                <p class="text-xs text-neutral-500 mt-0.5">
                  Kalender pemilihan tanggal mandiri (single & rentang tanggal dengan preset) dan status kosong interaktif.
                </p>
              </div>
              <BaseBadge variant="neutral" size="sm">Zero External Libs</BaseBadge>
            </div>

            <!-- Grid 2 Kolom untuk DatePicker Single vs Range -->
            <div class="grid grid-cols-1 md:grid-cols-2 gap-4">
              <!-- Mode Single Date -->
              <div class="p-4 rounded-xl bg-neutral-50 dark:bg-[var(--theme-surface-raised)] border border-neutral-200/80 dark:border-[var(--theme-border)] space-y-3">
                <div class="flex items-center justify-between">
                  <span class="text-xs font-bold text-neutral-800 dark:text-neutral-200">1. Single Date Picker</span>
                  <BaseBadge variant="primary" size="sm">Single Mode</BaseBadge>
                </div>
                <BaseDatePicker
                  v-model="demoSingleDate"
                  label="Pilih Tanggal"
                  placeholder="Pilih satu tanggal..."
                />
                <div class="p-2.5 rounded-lg bg-white dark:bg-[var(--theme-layer-2)] border border-neutral-200 dark:border-[var(--theme-border)] text-xs font-mono flex items-center justify-between">
                  <span class="text-neutral-400">v-model:</span>
                  <span class="font-bold text-[var(--theme-primary)]">{{ demoSingleDate || '(kosong)' }}</span>
                </div>
              </div>

              <!-- Mode Range Date -->
              <div class="p-4 rounded-xl bg-neutral-50 dark:bg-[var(--theme-surface-raised)] border border-neutral-200/80 dark:border-[var(--theme-border)] space-y-3">
                <div class="flex items-center justify-between">
                  <span class="text-xs font-bold text-neutral-800 dark:text-neutral-200">2. Date Range Picker (Presets)</span>
                  <BaseBadge variant="success" size="sm">Range Mode</BaseBadge>
                </div>
                <BaseDatePicker
                  v-model="demoRangeDate"
                  range
                  label="Pilih Rentang Tanggal"
                  placeholder="Pilih rentang tanggal..."
                />
                <div class="p-2.5 rounded-lg bg-white dark:bg-[var(--theme-layer-2)] border border-neutral-200 dark:border-[var(--theme-border)] text-xs font-mono flex items-center justify-between">
                  <span class="text-neutral-400">v-model:</span>
                  <span class="font-bold text-emerald-600 dark:text-emerald-400">
                    {{ demoRangeDate?.start ? `${demoRangeDate.start} s/d ${demoRangeDate.end}` : '(kosong)' }}
                  </span>
                </div>
              </div>
            </div>

            <!-- Demo BaseEmptyState -->
            <div class="p-4 rounded-xl bg-neutral-50 dark:bg-[var(--theme-surface-raised)] border border-neutral-200/80 dark:border-[var(--theme-border)] space-y-3">
              <div class="flex items-center justify-between">
                <span class="text-xs font-bold text-neutral-800 dark:text-neutral-200">3. Live BaseEmptyState (Ilustrasi Soft Badge)</span>
                <BaseBadge variant="neutral" size="sm">Modular Empty State</BaseBadge>
              </div>

              <div class="p-4 rounded-xl bg-white dark:bg-[var(--theme-layer-2)] border border-dashed border-neutral-200 dark:border-[var(--theme-border)]">
                <BaseEmptyState
                  :icon="FolderSearch"
                  title="Belum Ada Dokumen Lampiran"
                  description="Silakan unggah berkas pendukung atau mulai pencarian berkas baru pada folder kerja."
                  actionText="Unggah Dokumen Baru"
                  compact
                  @action="toast.info('Aksi unggah dokumen baru berhasil dipicu!', 'BaseEmptyState')"
                />
              </div>
            </div>
          </div>

          <!-- Showcase Komponen Enterprise: FileUploader, Timeline, SlideOver, Invoice -->
          <div class="p-5 rounded-2xl border border-neutral-200 dark:border-[var(--theme-border)] bg-white dark:bg-[var(--theme-surface)] shadow-2xs space-y-5">
            <div class="flex items-center justify-between">
              <div>
                <h2 class="text-sm font-bold text-neutral-900 dark:text-white flex items-center gap-2">
                  <Upload class="w-4 h-4 text-[var(--theme-primary)]" />
                  <span>Komponen Enterprise: FileUploader, Timeline & Drawer</span>
                </h2>
                <p class="text-xs text-neutral-500 mt-0.5">
                  Komponen drag & drop berkas, garis waktu jejak audit (timeline), slide-over drawer, dan faktur cetak resmi.
                </p>
              </div>
              <BaseBadge variant="success" size="sm">Enterprise Ready</BaseBadge>
            </div>

            <!-- Grid 2 Kolom: FileUploader vs Timeline -->
            <div class="grid grid-cols-1 md:grid-cols-2 gap-4">
              <!-- Live BaseFileUploader -->
              <div class="p-4 rounded-xl bg-neutral-50 dark:bg-[var(--theme-surface-raised)] border border-neutral-200/80 dark:border-[var(--theme-border)] space-y-3">
                <div class="flex items-center justify-between">
                  <span class="text-xs font-bold text-neutral-800 dark:text-neutral-200">1. BaseFileUploader (Drag & Drop)</span>
                  <BaseBadge variant="neutral" size="sm">Maks 5 MB</BaseBadge>
                </div>
                <BaseFileUploader
                  v-model="demoUploadedFiles"
                  label="Unggah Berkas Pendukung"
                  hint="Seret berkas gambar atau PDF ke sini"
                  accept="image/*,.pdf,.xlsx"
                  :max-size-m-b="5"
                />
              </div>

              <!-- Live BaseTimeline -->
              <div class="p-4 rounded-xl bg-neutral-50 dark:bg-[var(--theme-surface-raised)] border border-neutral-200/80 dark:border-[var(--theme-border)] space-y-3">
                <div class="flex items-center justify-between">
                  <span class="text-xs font-bold text-neutral-800 dark:text-neutral-200">2. BaseTimeline (Jejak Audit Aktivitas)</span>
                  <BaseBadge variant="primary" size="sm">Audit Log</BaseBadge>
                </div>
                <div class="p-3 rounded-xl bg-white dark:bg-[var(--theme-surface)] border border-neutral-200/80 dark:border-[var(--theme-border)]">
                  <BaseTimeline :items="demoTimelineItems" compact />
                </div>
              </div>
            </div>

            <!-- Interactive Trigger Demos: Slide-Over, Invoice & CSV Importer -->
            <div class="p-4 rounded-xl bg-neutral-50 dark:bg-[var(--theme-surface-raised)] border border-neutral-200/80 dark:border-[var(--theme-border)] space-y-3">
              <div class="flex items-center justify-between">
                <span class="text-xs font-bold text-neutral-800 dark:text-neutral-200">3. Demo Langsung Slide-Over Drawer & Faktur Cetak</span>
                <span class="text-[11px] text-neutral-400">Klik tombol di bawah untuk menguji</span>
              </div>
              <div class="flex items-center gap-2.5 flex-wrap">
                <BaseButton variant="outline" size="sm" @click="demoSlideOverOpen = true">
                  <Eye class="w-3.5 h-3.5" />
                  <span>Buka Slide-Over Drawer</span>
                </BaseButton>

                <BaseButton variant="outline" size="sm" @click="demoInvoiceOpen = true">
                  <Printer class="w-3.5 h-3.5" />
                  <span>Buka Faktur Invoice Resmi</span>
                </BaseButton>

                <BaseButton variant="outline" size="sm" @click="demoCsvOpen = true">
                  <FileSpreadsheet class="w-3.5 h-3.5 text-emerald-600" />
                  <span>Buka Wizard Impor CSV</span>
                </BaseButton>
              </div>
            </div>
          </div>

          <!-- Card Preferensi Sistem Menggunakan BaseSwitch -->
          <div class="p-5 rounded-2xl border border-neutral-200 dark:border-[var(--theme-border)] bg-white dark:bg-[var(--theme-surface)] shadow-2xs space-y-4">
            <h2 class="text-sm font-bold text-neutral-900 dark:text-white flex items-center gap-2">
              <Sparkles class="w-4 h-4 text-neutral-700 dark:text-neutral-300" />
              <span>Preferensi Fitur Sistem</span>
            </h2>

            <div class="space-y-4 divide-y divide-neutral-100 dark:divide-[var(--theme-border)] pt-1">
              <BaseSwitch
                v-model="autoSaveEnabled"
                label="Simpan Otomatis (Auto-save)"
                description="Simpan setiap perubahan formulir secara instan ke state penyimpanan lokal."
              />

              <div class="pt-4">
                <BaseSwitch
                  v-model="debugMode"
                  label="Mode Pengembang / Debug Log"
                  description="Aktifkan pencatatan pesan debug pada konsol peramban."
                />
              </div>
            </div>
          </div>

          <!-- Showcase BaseTabs & BaseBreadcrumb -->
          <div class="p-5 rounded-2xl border border-neutral-200 dark:border-[var(--theme-border)] bg-white dark:bg-[var(--theme-surface)] shadow-2xs space-y-4">
            <h2 class="text-sm font-bold text-neutral-900 dark:text-white">Navigasi Tab & Breadcrumb (BaseTabs & BaseBreadcrumb)</h2>
            <BaseBreadcrumb
              :items="[
                { label: 'Template' },
                { label: 'Pengaturan' },
                { label: 'Umum', active: true },
              ]"
            />
            <BaseTabs
              v-model="demoTabsValue"
              variant="pills"
              :tabs="[
                { value: 'pills', label: 'Tab Format Pills', badge: '3' },
                { value: 'underline', label: 'Tab Format Underline' },
                { value: 'cards', label: 'Tab Format Cards' },
              ]"
            >
              <template #panel-pills>
                <p class="text-xs text-neutral-500 py-1">Konten aktif varian <strong>Pills</strong>: Desain kapsul lembut untuk navigasi sub-menu internal.</p>
              </template>
              <template #panel-underline>
                <p class="text-xs text-neutral-500 py-1">Konten aktif varian <strong>Underline</strong>: Desain garis bawah aksen tema untuk konten tab panjang.</p>
              </template>
              <template #panel-cards>
                <p class="text-xs text-neutral-500 py-1">Konten aktif varian <strong>Cards</strong>: Tombol kartu solid untuk kategori mandiri.</p>
              </template>
            </BaseTabs>
          </div>

          <!-- Showcase Input Angka, Jam, & Masked Input -->
          <div class="p-5 rounded-2xl border border-neutral-200 dark:border-[var(--theme-border)] bg-white dark:bg-[var(--theme-surface)] shadow-2xs space-y-4">
            <h2 class="text-sm font-bold text-neutral-900 dark:text-white">Input Angka, Jam, & Format Bertanda Baca (Masked Input)</h2>
            <div class="grid grid-cols-1 sm:grid-cols-3 gap-3.5">
              <BaseNumberInput
                v-model="demoNumberVal"
                label="Toleransi Presensi (BaseNumberInput)"
                suffix="menit"
                :min="0"
                :max="60"
              />
              <BaseTimePicker
                v-model="demoTimeVal"
                label="Jam Masuk Kerja (BaseTimePicker)"
              />
              <BaseTimePicker
                v-model="demoTimeRange"
                range
                label="Rentang Shift (Range Time)"
              />
            </div>
            <div class="grid grid-cols-1 sm:grid-cols-3 gap-3.5 pt-1">
              <BaseMaskedInput
                v-model="demoNik"
                preset="nik"
                label="NIK 16 Digit (BaseMaskedInput)"
              />
              <BaseMaskedInput
                v-model="demoNpwp"
                preset="npwp"
                label="NPWP 15 Digit (BaseMaskedInput)"
              />
              <BaseMaskedInput
                v-model="demoPhone"
                preset="phone"
                label="Nomor WhatsApp (BaseMaskedInput)"
              />
            </div>
          </div>

          <!-- Showcase Radio & Checkbox Groups, Tree Select, Accordion -->
          <div class="p-5 rounded-2xl border border-neutral-200 dark:border-[var(--theme-border)] bg-white dark:bg-[var(--theme-surface)] shadow-2xs space-y-4">
            <h2 class="text-sm font-bold text-neutral-900 dark:text-white">Pilihan Grup, Tree Hierarkis & Accordion</h2>
            <div class="grid grid-cols-1 sm:grid-cols-2 gap-4">
              <div>
                <label class="block text-xs font-semibold text-neutral-700 dark:text-neutral-300 mb-2">Periode Cut-Off (BaseRadio Group)</label>
                <BaseRadio
                  v-model="demoRadioVal"
                  direction="horizontal"
                  :options="[
                    { value: 'bulanan', label: 'Bulanan (Tgl 25)' },
                    { value: 'mingguan', label: 'Mingguan (Jumat)' },
                  ]"
                />
              </div>
              <div>
                <label class="block text-xs font-semibold text-neutral-700 dark:text-neutral-300 mb-2">Kanal Notifikasi (BaseCheckbox Group)</label>
                <BaseCheckbox
                  v-model="demoCheckboxGroup"
                  direction="horizontal"
                  :options="[
                    { value: 'slip', label: 'Kirim Slip Email' },
                    { value: 'wa', label: 'Kirim WhatsApp' },
                  ]"
                />
              </div>
            </div>

            <div class="grid grid-cols-1 sm:grid-cols-2 gap-4 pt-2">
              <BaseTreeSelect
                v-model="demoTreeVal"
                :nodes="demoTreeNodes"
                multiple
                label="Struktur Organisasi (BaseTreeSelect)"
              />
              <div class="space-y-1">
                <label class="block text-xs font-semibold text-neutral-700 dark:text-neutral-300 mb-1">Panduan Pengaturan (BaseAccordion)</label>
                <BaseAccordion
                  :items="[
                    { id: 'acc1', title: 'Kebijakan Lembur Standar Kemnaker', content: 'Jam kerja lembur dihitung 1,5x untuk jam pertama dan 2x untuk jam berikutnya.' },
                    { id: 'acc2', title: 'Aturan Pajak PPh 21 TER', content: 'Kategori TER A, B, dan C otomatis dipetakan sesuai status PTKP pegawai.' },
                  ]"
                  default-open="acc1"
                />
              </div>
            </div>
          </div>

          <!-- Showcase Kartu Pilihan, Progres Bar & Kalender -->
          <div class="p-5 rounded-2xl border border-neutral-200 dark:border-[var(--theme-border)] bg-white dark:bg-[var(--theme-surface)] shadow-2xs space-y-4">
            <h2 class="text-sm font-bold text-neutral-900 dark:text-white">Kartu Pilihan Paket (BaseSelectableCard) & Progres (BaseProgress)</h2>
            <div class="grid grid-cols-1 sm:grid-cols-2 gap-3">
              <BaseSelectableCard
                v-model="demoCardSelected"
                value="paket-starter"
                title="Paket Starter HRIS"
                description="Hingga 50 karyawan aktif dengan modul presensi GPS & slip gaji digital."
                badge="Dasar"
              />
              <BaseSelectableCard
                v-model="demoCardSelected"
                value="paket-pro"
                title="Paket Enterprise Multi-Cabang"
                description="Unlimited karyawan, approval bertingkat, integrasi BPJS & geofence multi-polygon."
                badge="Rekomendasi"
                badge-variant="info"
              />
            </div>

            <div class="pt-2">
              <BaseProgress
                :value="demoProgressVal"
                :max="100"
                label="Kapasitas Lisensi Karyawan Digunakan"
                variant="primary"
                striped
              />
            </div>

            <!-- Kalender Bulanan Agenda -->
            <div class="pt-2">
              <BaseCalendar
                v-model="demoCalendarDate"
                :holidays="[
                  { date: '2025-01-01', label: 'Tahun Baru' },
                  { date: '2025-01-27', label: 'Isra Miraj' },
                  { date: '2025-01-29', label: 'Tahun Baru Imlek' },
                ]"
                :events="[
                  { id: 'ev1', date: '2025-01-25', title: 'Cut-off Payroll Bulanan', variant: 'danger' },
                  { id: 'ev2', date: '2025-01-28', title: 'Transfer Gaji Batch A', variant: 'success' },
                ]"
              />
            </div>
          </div>

          <!-- Showcase Kamera Selfie & Geofence Multi-Polygon -->
          <div class="p-5 rounded-2xl border border-neutral-200 dark:border-[var(--theme-border)] bg-white dark:bg-[var(--theme-surface)] shadow-2xs space-y-5">
            <div>
              <h2 class="text-sm font-bold text-neutral-900 dark:text-white">Kamera Selfie Presensi & Geofence Map Editor</h2>
              <p class="text-xs text-neutral-500 mt-0.5">
                Pengambilan foto selfie langsung via WebRTC dengan bingkai wajah dan editor geofence poligon tanpa library eksternal.
              </p>
            </div>

            <div class="grid grid-cols-1 lg:grid-cols-2 gap-5 items-start">
              <div>
                <BaseCameraCapture
                  v-model="demoSelfiePhoto"
                  watermark-text="Kantor Pusat Jakarta • GPS: Valid"
                />
              </div>
              <div>
                <BaseMapPolygonEditor
                  v-model="demoGeofence"
                />
              </div>
            </div>
          </div>

          <div class="flex justify-end">
            <BaseButton variant="primary" size="md" @click="handleSaveGeneral">
              <Save class="w-4 h-4" />
              Simpan Perubahan
            </BaseButton>
          </div>
        </div>

        <!-- ======================================================== -->
        <!-- 2. SUBMENU: DASAR PERTAMA (THEME COLOR & BACKGROUND)     -->
        <!-- ======================================================== -->
        <div v-else-if="activeTab === 'dasar1'" class="space-y-5">
          <!-- CARD 1: THEME COLOR PALETTE PICKER -->
          <div class="p-5 rounded-2xl border border-neutral-200 dark:border-[var(--theme-border)] bg-white dark:bg-[var(--theme-surface)] shadow-2xs space-y-4">
            <div class="flex items-start justify-between">
              <div>
                <div class="flex items-center gap-2">
                  <h2 class="text-sm font-bold text-neutral-900 dark:text-white flex items-center gap-2">
                    <Sparkles class="w-4 h-4 text-neutral-700 dark:text-neutral-300" />
                    <span>Pilihan Warna Tema (Theme Color)</span>
                  </h2>
                  <span
                    class="text-[10px] font-bold px-2 py-0.5 rounded-full uppercase"
                    :class="
                      themeColor === 'pink' ? 'bg-pink-100 text-pink-700 dark:bg-pink-950/60 dark:text-pink-300' :
                      themeColor === 'blue' ? 'bg-blue-100 text-blue-700 dark:bg-blue-950/60 dark:text-blue-300' :
                      themeColor === 'terracotta' ? 'bg-amber-100 text-[#8f270d] dark:bg-amber-950/60 dark:text-[#fcd9cf]' :
                      'bg-neutral-100 text-neutral-700 dark:bg-neutral-800 dark:text-neutral-300'
                    "
                  >
                    Aktif: {{ themePalettes[themeColor]?.name || themeColor }}
                  </span>
                </div>
                <p class="text-xs text-neutral-500 mt-1">
                  Pilih skema warna utama sistem. Warna tema langsung diterapkan ke tombol primary, icon sidebar, switch toggle, badge, dan aksen UI lainnya.
                </p>
              </div>
            </div>

            <!-- 4 Theme Color Cards (Responsive Grid) -->
            <div class="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-4 gap-3 pt-1">
              <!-- Default Hitam / Netral -->
              <button
                type="button"
                @click="handleThemeColorSelect('default')"
                class="group text-left p-3.5 rounded-xl border-2 transition-all cursor-pointer relative flex flex-col justify-between"
                :class="themeColor === 'default'
                  ? 'border-neutral-900 dark:border-white bg-neutral-50 dark:bg-[var(--theme-surface-muted)] shadow-sm'
                  : 'border-neutral-200 dark:border-[var(--theme-border)] hover:border-neutral-300 dark:hover:border-neutral-600 bg-white dark:bg-[var(--theme-surface)]'"
              >
                <div>
                  <div class="flex items-center justify-between mb-2">
                    <span class="text-xs font-bold text-neutral-900 dark:text-white">
                      Hitam / Netral
                    </span>
                    <span
                      v-if="themeColor === 'default'"
                      class="w-5 h-5 rounded-full bg-neutral-900 text-white dark:bg-white dark:text-neutral-950 flex items-center justify-center shrink-0"
                    >
                      <Check class="w-3 h-3 stroke-[3]" />
                    </span>
                  </div>
                  <p class="text-[11px] text-neutral-500 dark:text-neutral-400 leading-snug">
                    Monokrom & Pure Neutral Dark (Zero Blue Tint).
                  </p>
                </div>
                <div class="flex items-center gap-1.5 mt-3 pt-2.5 border-t border-neutral-100 dark:border-[var(--theme-border)]">
                  <span class="w-4 h-4 rounded-full bg-[#18181b] border border-neutral-400" title="#18181b" />
                  <span class="w-4 h-4 rounded-full bg-[#27272a]" title="#27272a" />
                  <span class="w-4 h-4 rounded-full bg-[#f4f4f5] border border-neutral-300" title="#f4f4f5" />
                  <span class="w-4 h-4 rounded-full bg-[#09090b] border border-neutral-700" title="#09090b" />
                  <span class="text-[10px] text-neutral-400 font-mono ml-auto">Default</span>
                </div>
              </button>

              <!-- Theme 1: Rose Berry (Pink) -->
              <button
                type="button"
                @click="handleThemeColorSelect('pink')"
                class="group text-left p-3.5 rounded-xl border-2 transition-all cursor-pointer relative flex flex-col justify-between"
                :class="themeColor === 'pink'
                  ? 'border-[#E85D88] bg-pink-50/50 dark:bg-pink-950/20 shadow-sm'
                  : 'border-neutral-200 dark:border-[var(--theme-border)] hover:border-pink-300 dark:hover:border-pink-800 bg-white dark:bg-[var(--theme-surface)]'"
              >
                <div>
                  <div class="flex items-center justify-between mb-2">
                    <span class="text-xs font-bold text-neutral-900 dark:text-white flex items-center gap-1.5">
                      <span class="w-2.5 h-2.5 rounded-full bg-[#E85D88]" />
                      Rose Berry (Pink)
                    </span>
                    <span
                      v-if="themeColor === 'pink'"
                      class="w-5 h-5 rounded-full bg-[#C83D6A] text-white flex items-center justify-center shrink-0"
                    >
                      <Check class="w-3 h-3 stroke-[3]" />
                    </span>
                  </div>
                  <p class="text-[11px] text-neutral-500 dark:text-neutral-400 leading-snug">
                    Identitas modern soft pink berry dengan aksen elegan.
                  </p>
                </div>
                <div class="flex items-center gap-1.5 mt-3 pt-2.5 border-t border-neutral-100 dark:border-[var(--theme-border)]">
                  <span class="w-4 h-4 rounded-full bg-[#E85D88]" title="Primary Brand: #E85D88" />
                  <span class="w-4 h-4 rounded-full bg-[#C83D6A]" title="Primary Action: #C83D6A" />
                  <span class="w-4 h-4 rounded-full bg-[#FCE7EF] border border-pink-200" title="Primary Soft: #FCE7EF" />
                  <span class="w-4 h-4 rounded-full bg-[#FFF8FB] border border-pink-200" title="Background: #FFF8FB" />
                  <span class="text-[10px] text-[#C83D6A] font-mono ml-auto">#E85D88</span>
                </div>
              </button>

              <!-- Theme 2: Apple Slate (Blue) -->
              <button
                type="button"
                @click="handleThemeColorSelect('blue')"
                class="group text-left p-3.5 rounded-xl border-2 transition-all cursor-pointer relative flex flex-col justify-between"
                :class="themeColor === 'blue'
                  ? 'border-[#007AFF] bg-blue-50/50 dark:bg-blue-950/20 shadow-sm'
                  : 'border-neutral-200 dark:border-[var(--theme-border)] hover:border-blue-300 dark:hover:border-blue-800 bg-white dark:bg-[var(--theme-surface)]'"
              >
                <div>
                  <div class="flex items-center justify-between mb-2">
                    <span class="text-xs font-bold text-neutral-900 dark:text-white flex items-center gap-1.5">
                      <span class="w-2.5 h-2.5 rounded-full bg-[#007AFF]" />
                      Apple Slate (Blue)
                    </span>
                    <span
                      v-if="themeColor === 'blue'"
                      class="w-5 h-5 rounded-full bg-[#007AFF] text-white flex items-center justify-center shrink-0"
                    >
                      <Check class="w-3 h-3 stroke-[3]" />
                    </span>
                  </div>
                  <p class="text-[11px] text-neutral-500 dark:text-neutral-400 leading-snug">
                    CTA tajam Apple iOS Slate Blue untuk tampilan clean & pro.
                  </p>
                </div>
                <div class="flex items-center gap-1.5 mt-3 pt-2.5 border-t border-neutral-100 dark:border-[var(--theme-border)]">
                  <span class="w-4 h-4 rounded-full bg-[#007AFF]" title="Primary: #007AFF" />
                  <span class="w-4 h-4 rounded-full bg-[#0062CC]" title="Primary Hover: #0062CC" />
                  <span class="w-4 h-4 rounded-full bg-[#E5F1FF] border border-blue-200" title="Primary Soft: #E5F1FF" />
                  <span class="w-4 h-4 rounded-full bg-[#F8FAFC] border border-blue-200" title="Background: #F8FAFC" />
                  <span class="text-[10px] text-[#007AFF] font-mono ml-auto">#007AFF</span>
                </div>
              </button>

              <!-- Theme 3: Terracotta / Warm Sand (VisionID) -->
              <button
                type="button"
                @click="handleThemeColorSelect('terracotta')"
                class="group text-left p-3.5 rounded-xl border-2 transition-all cursor-pointer relative flex flex-col justify-between"
                :class="themeColor === 'terracotta'
                  ? 'border-[#c03a18] bg-amber-50/50 dark:bg-amber-950/20 shadow-sm'
                  : 'border-neutral-200 dark:border-[var(--theme-border)] hover:border-amber-400 dark:hover:border-amber-800 bg-white dark:bg-[var(--theme-surface)]'"
              >
                <div>
                  <div class="flex items-center justify-between mb-2">
                    <span class="text-xs font-bold text-neutral-900 dark:text-white flex items-center gap-1.5">
                      <span class="w-2.5 h-2.5 rounded-full bg-[#c03a18]" />
                      Terracotta / Sand
                    </span>
                    <span
                      v-if="themeColor === 'terracotta'"
                      class="w-5 h-5 rounded-full bg-[#c03a18] text-white flex items-center justify-center shrink-0"
                    >
                      <Check class="w-3 h-3 stroke-[3]" />
                    </span>
                  </div>
                  <p class="text-[11px] text-neutral-500 dark:text-neutral-400 leading-snug">
                    Terracotta Rust & Warm Sand Linen terinspirasi landing page VisionID.
                  </p>
                </div>
                <div class="flex items-center gap-1.5 mt-3 pt-2.5 border-t border-neutral-100 dark:border-[var(--theme-border)]">
                  <span class="w-4 h-4 rounded-full bg-[#c03a18]" title="Primary Brand & Action: #c03a18" />
                  <span class="w-4 h-4 rounded-full bg-[#ab3315]" title="Primary Hover: #ab3315" />
                  <span class="w-4 h-4 rounded-full bg-[#FDF2EE] border border-amber-200" title="Primary Soft: #FDF2EE" />
                  <span class="w-4 h-4 rounded-full bg-[#F3F1EC] border border-stone-300" title="Background: #F3F1EC" />
                  <span class="text-[10px] text-[#c03a18] font-mono ml-auto">#c03a18</span>
                </div>
              </button>
            </div>

            <!-- Token Matrix Table Viewer -->
            <div class="pt-3 border-t border-neutral-100 dark:border-[var(--theme-border)]">
              <div class="flex items-center justify-between mb-2">
                <span class="text-xs font-bold text-neutral-700 dark:text-neutral-300">
                  Token Desain Tema Aktif:
                </span>
                <span class="text-[10px] text-neutral-400">
                  CSS Variables Auto-Cascade
                </span>
              </div>
              <div class="max-h-48 overflow-y-auto rounded-xl border border-neutral-200 dark:border-[var(--theme-border)] text-xs">
                <table class="w-full text-left">
                  <thead class="bg-neutral-50 dark:bg-[var(--theme-surface-raised)] text-neutral-500 dark:text-neutral-400 border-b border-neutral-200 dark:border-[var(--theme-border)] text-[10.5px]">
                    <tr>
                      <th class="py-1.5 px-3 font-semibold">Token</th>
                      <th class="py-1.5 px-3 font-semibold">HEX / Value</th>
                      <th class="py-1.5 px-3 font-semibold">Keterangan</th>
                    </tr>
                  </thead>
                  <tbody class="divide-y divide-neutral-100 dark:divide-[var(--theme-border)] text-neutral-700 dark:text-neutral-300 text-[11px]">
                    <tr v-for="t in currentTokens" :key="t.token" class="hover:bg-neutral-50/50 dark:hover:bg-[var(--theme-surface-raised)]">
                      <td class="py-1.5 px-3 font-medium">{{ t.token }}</td>
                      <td class="py-1.5 px-3 font-mono text-[10px] flex items-center gap-1.5">
                        <span
                          v-if="t.hex.startsWith('#')"
                          class="w-3 h-3 rounded-full border border-black/10 shrink-0 inline-block"
                          :style="{ backgroundColor: t.hex }"
                        />
                        <span>{{ t.hex }}</span>
                      </td>
                      <td class="py-1.5 px-3 text-neutral-500 dark:text-neutral-400">{{ t.desc }}</td>
                    </tr>
                  </tbody>
                </table>
              </div>
            </div>
          </div>

          <!-- CARD 2: KUSTOMISASI DASAR PERTAMA (BACKGROUND) -->
          <div class="p-5 rounded-2xl border border-neutral-200 dark:border-[var(--theme-border)] bg-white dark:bg-[var(--theme-surface)] shadow-2xs space-y-4">
            <div>
              <h2 class="text-sm font-bold text-neutral-900 dark:text-white flex items-center gap-2">
                <Palette class="w-4 h-4 text-neutral-700 dark:text-neutral-300" />
                <span>Kustomisasi Dasar Pertama (Base Layer 1)</span>
              </h2>
              <p class="text-xs text-neutral-500 mt-1">
                Pilih jenis pola latar belakang di belakang kartu aplikasi utama (Dasar Kedua).
              </p>
            </div>

            <!-- Mode Pilihan -->
            <div class="grid grid-cols-2 sm:grid-cols-4 gap-3">
              <button
                type="button"
                @click="setBgType('grid'); toast.info('Mode Grid aktif', 'Background')"
                class="flex flex-col items-center justify-center p-3.5 rounded-xl border transition cursor-pointer text-center"
                :class="bgType === 'grid' ? 'border-neutral-900 bg-neutral-900 text-white dark:border-white dark:bg-white dark:text-neutral-950 font-bold' : 'border-neutral-200 dark:border-[var(--theme-border)] hover:bg-neutral-50 dark:hover:bg-[var(--theme-surface-muted)] text-neutral-700 dark:text-neutral-300'"
              >
                <Grid class="w-5 h-5 mb-1.5" />
                <span class="text-xs">Grid Kotak</span>
              </button>

              <button
                type="button"
                @click="setBgType('dots'); toast.info('Mode Titik-titik aktif', 'Background')"
                class="flex flex-col items-center justify-center p-3.5 rounded-xl border transition cursor-pointer text-center"
                :class="bgType === 'dots' ? 'border-neutral-900 bg-neutral-900 text-white dark:border-white dark:bg-white dark:text-neutral-950 font-bold' : 'border-neutral-200 dark:border-[var(--theme-border)] hover:bg-neutral-50 dark:hover:bg-[var(--theme-surface-muted)] text-neutral-700 dark:text-neutral-300'"
              >
                <CircleDot class="w-5 h-5 mb-1.5" />
                <span class="text-xs">Titik-titik (Dots)</span>
              </button>

              <button
                type="button"
                @click="setBgType('solid'); setSolidColor('#18181b'); toast.info('Mode Warna Solid aktif', 'Background')"
                class="flex flex-col items-center justify-center p-3.5 rounded-xl border transition cursor-pointer text-center"
                :class="bgType === 'solid' ? 'border-neutral-900 bg-neutral-900 text-white dark:border-white dark:bg-white dark:text-neutral-950 font-bold' : 'border-neutral-200 dark:border-[var(--theme-border)] hover:bg-neutral-50 dark:hover:bg-[var(--theme-surface-muted)] text-neutral-700 dark:text-neutral-300'"
              >
                <Palette class="w-5 h-5 mb-1.5" />
                <span class="text-xs">Warna Solid</span>
              </button>

              <button
                type="button"
                @click="setBgType('image'); toast.info('Mode Gambar aktif', 'Background')"
                class="flex flex-col items-center justify-center p-3.5 rounded-xl border transition cursor-pointer text-center"
                :class="bgType === 'image' ? 'border-neutral-900 bg-neutral-900 text-white dark:border-white dark:bg-white dark:text-neutral-950 font-bold' : 'border-neutral-200 dark:border-[var(--theme-border)] hover:bg-neutral-50 dark:hover:bg-[var(--theme-surface-muted)] text-neutral-700 dark:text-neutral-300'"
              >
                <ImageIcon class="w-5 h-5 mb-1.5" />
                <span class="text-xs">Gambar (Image)</span>
              </button>
            </div>

            <!-- Opsi Tambahan untuk Solid Color -->
            <div v-if="bgType === 'solid'" class="pt-3 border-t border-neutral-100 dark:border-[var(--theme-border)] space-y-2">
              <label class="block text-xs font-semibold text-neutral-700 dark:text-neutral-300">
                Pilih Palet Warna Latar:
              </label>
              <div class="flex flex-wrap items-center gap-2">
                <button
                  v-for="color in presetColors"
                  :key="color"
                  type="button"
                  @click="setSolidColor(color)"
                  class="w-8 h-8 rounded-full border-2 transition cursor-pointer flex items-center justify-center shadow-xs"
                  :style="{ backgroundColor: color }"
                  :class="solidColor === color ? 'border-neutral-900 dark:border-white scale-110' : 'border-neutral-300 dark:border-neutral-600'"
                >
                  <Check v-if="solidColor === color" class="w-4 h-4 text-neutral-700 dark:text-white" />
                </button>
              </div>
            </div>

            <!-- Opsi Tambahan untuk Image -->
            <div v-if="bgType === 'image'" class="pt-3 border-t border-neutral-100 dark:border-[var(--theme-border)] space-y-2 max-w-lg">
              <BaseInput
                v-model="imageUrl"
                label="URL Gambar Background"
                placeholder="https://images.unsplash.com/..."
                clearable
              />
            </div>
          </div>
        </div>

        <!-- ======================================================== -->
        <!-- 3. SUBMENU: MODAL & KONFIRMASI (SHOWCASE BASE MODAL)     -->
        <!-- ======================================================== -->
        <div v-else-if="activeTab === 'modal'" class="space-y-5">
          <!-- Modal Dialog Showcase -->
          <div class="p-5 rounded-2xl border border-neutral-200 dark:border-[var(--theme-border)] bg-white dark:bg-[var(--theme-surface)] shadow-2xs space-y-4">
            <div class="flex items-center justify-between">
              <div>
                <h2 class="text-sm font-bold text-neutral-900 dark:text-white flex items-center gap-2">
                  <Layers class="w-4 h-4 text-neutral-700 dark:text-neutral-300" />
                  <span>Modal Standar & Mobile Sheet</span>
                </h2>
                <p class="text-xs text-neutral-500 mt-0.5">
                  Komponen modal responsif dengan backdrop blur, escape listener, dan animasi halus.
                </p>
              </div>
            </div>

            <div class="flex flex-wrap items-center gap-3 pt-1">
              <BaseButton variant="primary" size="sm" @click="isDesktopModalOpen = true">
                <Monitor class="w-3.5 h-3.5" />
                <span>Buka Desktop Modal</span>
              </BaseButton>

              <BaseButton variant="secondary" size="sm" @click="isMobileModalOpen = true">
                <Smartphone class="w-3.5 h-3.5" />
                <span>Buka Mobile Modal (Slide Drawer)</span>
              </BaseButton>
            </div>
          </div>

          <!-- Alert Confirmation Pop-up (Simpan & Hapus) -->
          <div class="space-y-3">
            <h3 class="text-xs font-bold uppercase tracking-wider text-neutral-500 dark:text-neutral-400">
              Alert Confirmation Pop-up (Cloud Badge & Sparkle Lines)
            </h3>

            <div class="grid grid-cols-1 md:grid-cols-2 gap-4">
              <!-- Card 1: Confirmation Simpan -->
              <div class="p-5 rounded-2xl border border-neutral-200 dark:border-[var(--theme-border)] bg-white dark:bg-[var(--theme-surface)] shadow-2xs flex flex-col justify-between space-y-4">
                <div class="flex items-start justify-between">
                  <BaseBadge variant="info">Confirmation — Simpan</BaseBadge>
                  <div class="w-2 h-2 rounded-full bg-blue-500" />
                </div>

                <div class="space-y-1">
                  <h4 class="text-sm font-bold text-neutral-900 dark:text-white">
                    Konfirmasi Simpan Data
                  </h4>
                  <p class="text-xs text-neutral-500 dark:text-neutral-400">
                    Popup konfirmasi sebelum data disimpan ke server, dengan ikon disket dan tombol konfirmasi biru.
                  </p>
                </div>

                <BaseButton variant="outline" size="sm" class="w-full" @click="showSavePopup = true">
                  <Save class="w-3.5 h-3.5 text-blue-600 dark:text-blue-400" />
                  Uji Coba Popup Simpan
                </BaseButton>
              </div>

              <!-- Card 2: Confirmation Delete -->
              <div class="p-5 rounded-2xl border border-neutral-200 dark:border-[var(--theme-border)] bg-white dark:bg-[var(--theme-surface)] shadow-2xs flex flex-col justify-between space-y-4">
                <div class="flex items-start justify-between">
                  <BaseBadge variant="danger">Confirmation — Delete</BaseBadge>
                  <div class="w-2 h-2 rounded-full bg-rose-500" />
                </div>

                <div class="space-y-1">
                  <h4 class="text-sm font-bold text-neutral-900 dark:text-white">
                    Konfirmasi Hapus Data
                  </h4>
                  <p class="text-xs text-neutral-500 dark:text-neutral-400">
                    Popup peringatan sebelum data dihapus secara permanen, dengan icon tempat sampah dan sparkle rays.
                  </p>
                </div>

                <BaseButton variant="outline" size="sm" class="w-full" @click="showDeletePopup = true">
                  <Trash2 class="w-3.5 h-3.5 text-rose-500" />
                  Uji Coba Popup Hapus
                </BaseButton>
              </div>
            </div>
          </div>
        </div>

        <!-- ======================================================== -->
        <!-- 4. SUBMENU: NOTIFIKASI & TOAST                           -->
        <!-- ======================================================== -->
        <div v-else-if="activeTab === 'notifikasi'" class="space-y-5">
          <!-- BaseAlert Banner & Inline Showcase -->
          <div class="p-5 rounded-2xl border border-neutral-200 dark:border-[var(--theme-border)] bg-white dark:bg-[var(--theme-surface)] shadow-2xs space-y-3">
            <h2 class="text-sm font-bold text-neutral-900 dark:text-white">Peringatan Banner & Inline (BaseAlert)</h2>
            <BaseAlert
              variant="warning"
              banner
              title="Pemberitahuan Sistem Multi-Tenant"
              description="Sinkronisasi otomatis dengan server BPJS Ketenagakerjaan akan dilakukan malam ini pukul 23:00 WIB."
              dismissible
            />
            <div class="grid grid-cols-1 sm:grid-cols-2 gap-3 pt-1">
              <BaseAlert
                variant="info"
                title="Pembaruan Skema Gaji"
                description="Perhitungan PPh 21 TER 2024 telah aktif secara default."
                action-text="Pelajari"
                @action="toast.info('Membuka dokumentasi PPh 21 TER')"
              />
              <BaseAlert
                variant="danger"
                title="Izin Kamera Ditolak"
                description="Presensi selfie membutuhkan izin kamera di browser."
                action-text="Coba Lagi"
                @action="toast.warning('Meminta ulang izin kamera...')"
              />
            </div>
          </div>

          <!-- Toast Notification Service Showcase -->
          <div class="p-5 rounded-2xl border border-neutral-200 dark:border-[var(--theme-border)] bg-white dark:bg-[var(--theme-surface)] shadow-2xs space-y-4">
            <div class="flex items-center justify-between">
              <div class="flex items-center gap-2 text-xs font-bold text-neutral-900 dark:text-white">
                <Bell class="w-4 h-4 text-neutral-600 dark:text-neutral-300" />
                <span>Toast Notification Service (useToast)</span>
              </div>
              <BaseBadge variant="success" dot pulse>Aktif</BaseBadge>
            </div>

            <p class="text-xs text-neutral-500 leading-relaxed">
              Transisi in-place dalam 1 toast card: status <strong>Loading (Menyimpan...)</strong> dengan spinner circle berganti langsung ke <strong>Berhasil (✓) / Gagal (×)</strong> tanpa spawning toast ganda.
            </p>

            <!-- Featured Save Demo Buttons -->
            <div class="grid grid-cols-1 sm:grid-cols-2 gap-3 pt-1">
              <button
                type="button"
                @click="triggerSaveSuccessDemo"
                class="flex items-center justify-center gap-2 py-2.5 px-4 rounded-xl text-xs font-semibold bg-neutral-900 text-white hover:bg-neutral-800 dark:bg-white dark:text-neutral-950 dark:hover:bg-neutral-100 transition cursor-pointer shadow-sm active:scale-[0.99]"
              >
                <span class="w-2 h-2 rounded-full bg-emerald-400 animate-pulse" />
                <span>Simpan (Loading &rarr; Berhasil)</span>
              </button>

              <button
                type="button"
                @click="triggerSaveErrorDemo"
                class="flex items-center justify-center gap-2 py-2.5 px-4 rounded-xl text-xs font-semibold bg-neutral-100 text-neutral-800 hover:bg-neutral-200 dark:bg-[var(--theme-surface-muted)] dark:text-neutral-200 dark:hover:bg-[var(--theme-surface-muted)] border border-neutral-300 dark:border-[var(--theme-border-strong)] transition cursor-pointer shadow-xs active:scale-[0.99]"
              >
                <span class="w-2 h-2 rounded-full bg-rose-400 animate-pulse" />
                <span>Simpan (Loading &rarr; Gagal)</span>
              </button>
            </div>

            <!-- Standard Toast Types -->
            <div class="pt-3 border-t border-neutral-100 dark:border-[var(--theme-border)] space-y-2">
              <span class="text-[11px] font-semibold text-neutral-500 block">Toast Sekali Panggil:</span>
              <div class="flex flex-wrap items-center gap-2">
                <BaseButton variant="outline" size="sm" @click="triggerSuccess">
                  <CheckCircle2 class="w-3.5 h-3.5 text-emerald-500" />
                  <span>Success</span>
                </BaseButton>
                <BaseButton variant="outline" size="sm" @click="triggerError">
                  <XCircle class="w-3.5 h-3.5 text-rose-500" />
                  <span>Error</span>
                </BaseButton>
                <BaseButton variant="outline" size="sm" @click="triggerWarning">
                  <AlertTriangle class="w-3.5 h-3.5 text-amber-500" />
                  <span>Warning</span>
                </BaseButton>
                <BaseButton variant="outline" size="sm" @click="triggerInfo">
                  <Info class="w-3.5 h-3.5 text-sky-500" />
                  <span>Info</span>
                </BaseButton>
              </div>
            </div>
          </div>

          <!-- Preferensi Notifikasi -->
          <div class="p-5 rounded-2xl border border-neutral-200 dark:border-[var(--theme-border)] bg-white dark:bg-[var(--theme-surface)] shadow-2xs space-y-4">
            <h3 class="text-sm font-bold text-neutral-900 dark:text-white">Preferensi Toast</h3>
            <div class="space-y-4 divide-y divide-neutral-100 dark:divide-[var(--theme-border)]">
              <BaseSwitch
                v-model="soundNotification"
                label="Suara Efek Notifikasi"
                description="Putar suara lembut saat toast notifikasi baru muncul."
              />
              <div class="pt-4">
                <BaseSwitch
                  v-model="autoDismissToast"
                  label="Tutup Otomatis (Auto-dismiss)"
                  description="Toast otomatis menghilang setelah 3 detik."
                />
              </div>
            </div>
          </div>
        </div>

        <!-- ======================================================== -->
        <!-- 5. SUBMENU: KEAMANAN & AKUN                              -->
        <!-- ======================================================== -->
        <div v-else-if="activeTab === 'keamanan'" class="space-y-5">
          <!-- 2FA Switcher -->
          <div class="p-5 rounded-2xl border border-neutral-200 dark:border-[var(--theme-border)] bg-white dark:bg-[var(--theme-surface)] shadow-2xs space-y-4">
            <div class="flex items-center gap-2">
              <Shield class="w-4 h-4 text-emerald-500" />
              <h2 class="text-sm font-bold text-neutral-900 dark:text-white">Autentikasi & Perlindungan Akun</h2>
            </div>
            
            <BaseSwitch
              v-model="twoFactorAuth"
              label="Autentikasi Dua Faktor (2FA)"
              description="Meminta kode verifikasi unik setiap kali masuk dari perangkat baru."
            />
          </div>

          <!-- Form Ubah Password Menggunakan BaseInput dengan Eye Toggle -->
          <div class="p-5 rounded-2xl border border-neutral-200 dark:border-[var(--theme-border)] bg-white dark:bg-[var(--theme-surface)] shadow-2xs space-y-4">
            <div class="flex items-center gap-2">
              <KeyRound class="w-4 h-4 text-neutral-700 dark:text-neutral-300" />
              <h2 class="text-sm font-bold text-neutral-900 dark:text-white">Perbarui Kata Sandi</h2>
            </div>

            <div class="space-y-3.5 max-w-md">
              <BasePasswordInput
                v-model="currentPassword"
                label="Kata Sandi Saat Ini"
                placeholder="Masukkan kata sandi lama..."
                required
              />

              <BasePasswordInput
                v-model="newPassword"
                label="Kata Sandi Baru"
                placeholder="Minimal 8 karakter..."
                required
                show-strength
                hint="Gunakan kombinasi huruf, angka, dan simbol untuk keamanan maksimal."
              />

              <div class="pt-2">
                <BaseButton variant="primary" size="sm" @click="handleSavePassword">
                  Perbarui Kata Sandi
                </BaseButton>
              </div>
            </div>
          </div>
        </div>

      </div>
    </Transition>

    <!-- Modals (Teleported) -->
    <BaseModal
      v-model="isDesktopModalOpen"
      :adaptive="false"
      title="Modal Desktop Dialog"
      subtitle="Komponen BaseModal dengan backdrop-blur dan transisi skala halus"
      size="md"
    >
      <div class="space-y-3 text-xs text-neutral-600 dark:text-neutral-300">
        <p>
          Ini adalah contoh <strong>BaseModal.vue</strong> yang dapat dikonfigurasi ukurannya (sm, md, lg, xl, full).
        </p>
        <div class="p-3 rounded-xl bg-neutral-100 dark:bg-[var(--theme-surface-raised)] border border-neutral-200 dark:border-[var(--theme-border)]">
          Mendukung tombol Esc, click-outside backdrop, serta auto-lock scroll pada body dokumen.
        </div>
      </div>
      <template #footer="{ close }">
        <BaseButton variant="outline" size="sm" @click="close">Tutup</BaseButton>
        <BaseButton variant="primary" size="sm" @click="close; toast.success('Aksi modal dikonfirmasi!')">
          Konfirmasi
        </BaseButton>
      </template>
    </BaseModal>

    <MobileModal
      v-model="isMobileModalOpen"
      title="Mobile Drawer Sheet"
      subtitle="Modal geser dari bawah (Bottom Sheet)"
    >
      <div class="space-y-3 text-xs text-neutral-600 dark:text-neutral-300 py-2">
        <p>Komponen modal drawer bawah yang sangat nyaman diakses menggunakan smartphone.</p>
        <BaseButton variant="primary" size="md" class="w-full" @click="isMobileModalOpen = false">
          Selesai
        </BaseButton>
      </div>
    </MobileModal>

    <ConfirmationModal
      v-model="showSavePopup"
      type="save"
      badge-text="Confirmation — Simpan"
      title="Apakah Anda yakin ingin menyimpan perubahan?"
      message="Perubahan akan segera diproses ke dalam sistem setelah Anda menekan tombol simpan."
      confirm-text="Simpan"
      cancel-text="Batal"
      @confirm="triggerSaveSuccessDemo"
    />

    <ConfirmationModal
      v-model="showDeletePopup"
      type="delete"
      badge-text="Confirmation — Delete"
      title="Apakah Anda yakin ingin menghapus data ini?"
      message="Tindakan ini tidak dapat dibatalkan. Semua data terkait akan dihapus secara permanen."
      confirm-text="Hapus"
      cancel-text="Batal"
      @confirm="toast.error('Data berhasil dihapus dari sistem.', 'Dihapus')"
    />

    <!-- Demo Slide-Over Drawer -->
    <BaseSlideOver
      v-model="demoSlideOverOpen"
      title="Panel Slide-Over Drawer"
      subtitle="Drawer mulus dari sisi kanan layar"
      size="md"
    >
      <template #badge>
        <BaseBadge variant="success" size="sm">Live Demo</BaseBadge>
      </template>

      <div class="space-y-4 text-xs text-neutral-600 dark:text-neutral-300">
        <p>
          Komponen <code>BaseSlideOver.vue</code> sangat cocok untuk melihat detail pesanan, profil lengkap pengguna, atau log tanpa meninggalkan halaman saat ini.
        </p>

        <div class="p-4 rounded-xl bg-neutral-50 dark:bg-[var(--theme-surface-raised)] border border-neutral-200 dark:border-[var(--theme-border)] space-y-2">
          <span class="font-bold text-neutral-900 dark:text-white">Aktivitas Terkini:</span>
          <BaseTimeline :items="demoTimelineItems" compact />
        </div>
      </div>

      <template #footer="{ close }">
        <BaseButton variant="outline" size="sm" @click="close">Tutup</BaseButton>
        <BaseButton variant="primary" size="sm" @click="close(); toast.success('Aksi drawer berhasil dijalankan!')">
          Simpan Data
        </BaseButton>
      </template>
    </BaseSlideOver>

    <!-- Demo Invoice Modal -->
    <BaseInvoiceModal
      v-model="demoInvoiceOpen"
    />

    <!-- Demo CSV Importer Modal -->
    <BaseCsvImporter
      v-model="demoCsvOpen"
      @imported="toast.success('Simulasi data CSV berhasil diimpor!')"
    />
  </div>
</template>

<style scoped>
.tab-slide-enter-active,
.tab-slide-leave-active {
  transition: opacity 0.2s ease-out, transform 0.2s cubic-bezier(0.16, 1, 0.3, 1);
}
.tab-slide-enter-from {
  opacity: 0;
  transform: translateY(6px);
}
.tab-slide-leave-to {
  opacity: 0;
  transform: translateY(-6px);
}
</style>
