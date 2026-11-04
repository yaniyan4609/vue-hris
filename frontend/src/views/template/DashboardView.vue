<script setup>
import { ref, computed, onMounted, onUnmounted, nextTick, watch } from 'vue'
import { useRouter } from 'vue-router'
import BaseButton from '../../components/ui/BaseButton.vue'
import BaseBadge from '../../components/ui/BaseBadge.vue'
import BaseModal from '../../components/ui/BaseModal.vue'
import BaseSkeleton from '../../components/ui/BaseSkeleton.vue'
import BaseCountUp from '../../components/ui/BaseCountUp.vue'
import BaseStatCard from '../../components/ui/BaseStatCard.vue'
import BaseDatePicker from '../../components/ui/BaseDatePicker.vue'
import BaseSlideOver from '../../components/ui/BaseSlideOver.vue'
import BaseInvoiceModal from '../../components/ui/BaseInvoiceModal.vue'
import BaseTimeline from '../../components/ui/BaseTimeline.vue'
import { useToast } from '../../composables/useToast'
import {
  kpiSummary,
  trenDataPresets,
  layananData,
  statusDistribusi,
  ringkasanKeuangan,
  utilisasiMesin,
  inventarisWarning,
  logistikDanKaryawan,
  aktivitasOperasional,
  orderTerbaruList,
} from '../../data/dashboardData'
import {
  ShoppingBag,
  Wallet,
  RefreshCw,
  Users,
  Cpu,
  AlertTriangle,
  TrendingUp,
  BarChart3,
  PieChart,
  ArrowUp,
  ArrowDown,
  ArrowRight,
  Clock,
  Truck,
  Package,
  CheckCircle2,
  ChevronDown,
  Check,
  X,
  Printer,
  Eye,
  Sliders,
  DollarSign,
  Activity,
  Layers,
  Sparkles,
  Info,
  Waves,
} from 'lucide-vue-next'

const router = useRouter()
const toast = useToast()

// ========================================================
// 1. SKELETON LOADING & REFRESH SYSTEM
// ========================================================
const isLoading = ref(true)
const isRefreshing = ref(false)

const simulateLoading = async (duration = 750) => {
  isLoading.value = true
  await new Promise((resolve) => setTimeout(resolve, duration))
  isLoading.value = false
}

const handleRefresh = async () => {
  isRefreshing.value = true
  await simulateLoading(700)
  isRefreshing.value = false
  toast.success('Metrik dan stream operasional berhasil diperbarui!', 'Dashboard Live')
}

// ========================================================
// 2. STATE & FILTER KONTROL INTERAKTIF
// ========================================================
const selectedTrenPeriod = ref('7hari') // '7hari' | '14hari' | 'bulan'
const selectedLayananPeriod = ref('hari-ini') // 'hari-ini' | 'minggu-ini'
const activeChartHoverIdx = ref(6) // Default to latest point (27 Jan)
const hoveredDonutSlice = ref(null)

// Date Range Filter
const dashboardDateRange = ref({
  start: '2025-01-21',
  end: '2025-01-27',
})

const handleDateRangeChange = (val) => {
  if (val && val.start && val.end) {
    toast.info(`Rentang tanggal difilter: ${val.start} s/d ${val.end}`, 'Filter Tanggal Dashboard')
  }
}

// Current Trend Data
const currentTren = computed(() => {
  return trenDataPresets[selectedTrenPeriod.value] || trenDataPresets['7hari']
})

// Reset hover index when period changes
watch(selectedTrenPeriod, () => {
  activeChartHoverIdx.value = currentTren.value.dates.length - 1
})

// Quick Detail Modal State for Order Row
const selectedOrder = ref(null)
const isOrderSlideOverOpen = ref(false)
const isInvoiceModalOpen = ref(false)
const invoiceModalData = ref(null)

const openOrderDetail = (order) => {
  selectedOrder.value = order
  isOrderSlideOverOpen.value = true
}

const handlePrintNota = (order) => {
  invoiceModalData.value = {
    noInvoice: 'INV-' + (order.noOrder ? order.noOrder.replace('#', '') : '2025-0842'),
    tanggal: '27 Jan 2025',
    jatuhTempo: '03 Feb 2025',
    status: order.status === 'Selesai' ? 'LUNAS' : 'PENDING',
    metodePembayaran: 'QRIS / Transfer Bank Mandiri',
    customer: {
      nama: order.pelanggan || 'Pelanggan Walk-in',
      kontak: 'PIC Pemesan',
      telepon: '0812-8877-6655',
      alamat: 'Jl. Percetakan Negara No. 45, Jakarta Pusat',
    },
    items: [
      {
        nama: order.layanan,
        spesifikasi: `Kuantitas ${order.berat}, Pengerjaan Standar Operasional`,
        qty: 1,
        harga: order.total,
        total: order.total,
      },
    ],
    subtotal: order.total,
    diskon: 0,
    ppn: Math.round(order.total * 0.11),
    totalBayar: Math.round(order.total * 1.11),
    catatan: 'Terima kasih telah mempercayai layanan percetakan & cetak cepat Digaji.id.',
  }
  isInvoiceModalOpen.value = true
}

const getOrderTimeline = (order) => [
  {
    id: 1,
    title: `Order Masuk: ${order.noOrder}`,
    description: `Order layanan ${order.layanan} (${order.berat}) senilai Rp ${order.total.toLocaleString('id-ID')} dicatat sistem.`,
    time: order.waktu || 'Hari ini',
    variant: 'primary',
    author: 'Kasir Front Office',
  },
  {
    id: 2,
    title: 'Status Produksi: ' + order.status,
    description: 'Pekerjaan dialokasikan ke mesin cetak dan antrean operator.',
    time: '20 menit lalu',
    variant: order.status === 'Selesai' ? 'success' : 'warning',
    author: 'Operator Mesin',
  },
]

// ========================================================
// 3. SVG CHART HELPERS & WAVE ANIMATION (Bergelombang Naik-Turun)
// ========================================================
const svgWidth = 600
const svgHeight = 220
const padX = 40
const padYTop = 25
const padYBottom = 35

// Wave state & animation loop
const isWaveActive = ref(true)
const wavePhase = ref(0)
const waveAmplitude = ref(4.2) // Amplitude gelombang (pixel)
const isHoveringChart = ref(false)
let waveFrameId = null

const startWaveLoop = () => {
  if (waveFrameId) cancelAnimationFrame(waveFrameId)

  const step = () => {
    if (isWaveActive.value && !isHoveringChart.value && !isLoading.value) {
      wavePhase.value = (wavePhase.value + 0.038) % (Math.PI * 2)
    }
    waveFrameId = requestAnimationFrame(step)
  }
  waveFrameId = requestAnimationFrame(step)
}

const stopWaveLoop = () => {
  if (waveFrameId) {
    cancelAnimationFrame(waveFrameId)
    waveFrameId = null
  }
}

const toggleWave = () => {
  isWaveActive.value = !isWaveActive.value
  toast.info(
    isWaveActive.value ? 'Animasi gelombang diaktifkan' : 'Animasi gelombang dijeda',
    'Diagram Wave'
  )
}

// Map Omzet Values to Y Coordinate
const getOmzetY = (val) => {
  const max = currentTren.value.maxOmzet
  const plotH = svgHeight - padYTop - padYBottom
  return padYTop + (1 - val / max) * plotH
}

// Map Order Values to Y Coordinate
const getOrderY = (val) => {
  const max = currentTren.value.maxOrder
  const plotH = svgHeight - padYTop - padYBottom
  return padYTop + (1 - val / max) * plotH
}

// X Coordinate for Point i
const getPointX = (index, total) => {
  const plotW = svgWidth - padX * 2
  return padX + (index / (total - 1)) * plotW
}

// Points for Omzet Area/Line with Real Harmonic Wave Undulation
const omzetPoints = computed(() => {
  const vals = currentTren.value.omzet
  const total = vals.length
  return vals.map((val, idx) => {
    const baseY = getOmzetY(val)
    const waveOffset = isWaveActive.value && !isLoading.value
      ? Math.sin(wavePhase.value + idx * 0.85) * waveAmplitude.value
      : 0
    return {
      x: getPointX(idx, total),
      y: baseY + waveOffset,
      baseY,
      val,
    }
  })
})

// Points for Order Line with Counterpoint Harmonic Wave Undulation
const orderPoints = computed(() => {
  const vals = currentTren.value.order
  const total = vals.length
  return vals.map((val, idx) => {
    const baseY = getOrderY(val)
    const waveOffset = isWaveActive.value && !isLoading.value
      ? Math.cos(wavePhase.value + idx * 0.85 + 0.4) * (waveAmplitude.value * 0.75)
      : 0
    return {
      x: getPointX(idx, total),
      y: baseY + waveOffset,
      baseY,
      val,
    }
  })
})

// Generate Smooth Cubic Bezier SVG Path
const getSmoothPath = (pts) => {
  if (!pts || pts.length === 0) return ''
  if (pts.length === 1) return `M ${pts[0].x} ${pts[0].y}`

  let d = `M ${pts[0].x.toFixed(1)} ${pts[0].y.toFixed(1)}`
  for (let i = 0; i < pts.length - 1; i++) {
    const p0 = pts[i === 0 ? 0 : i - 1]
    const p1 = pts[i]
    const p2 = pts[i + 1]
    const p3 = pts[i + 2 < pts.length ? i + 2 : i + 1]

    const cp1x = p1.x + (p2.x - p0.x) / 6
    const cp1y = p1.y + (p2.y - p0.y) / 6
    const cp2x = p2.x - (p3.x - p1.x) / 6
    const cp2y = p2.y - (p3.y - p1.y) / 6

    d += ` C ${cp1x.toFixed(1)} ${cp1y.toFixed(1)}, ${cp2x.toFixed(1)} ${cp2y.toFixed(1)}, ${p2.x.toFixed(1)} ${p2.y.toFixed(1)}`
  }
  return d
}

// Omzet Area Path (closed polygon for gradient fill)
const omzetAreaPath = computed(() => {
  const pts = omzetPoints.value
  if (pts.length === 0) return ''
  const linePath = getSmoothPath(pts)
  const bottomY = svgHeight - padYBottom
  const firstX = pts[0].x
  const lastX = pts[pts.length - 1].x
  return `${linePath} L ${lastX.toFixed(1)} ${bottomY} L ${firstX.toFixed(1)} ${bottomY} Z`
})

// Omzet Line Stroke Path
const omzetLinePath = computed(() => getSmoothPath(omzetPoints.value))

// Order Line Stroke Path
const orderLinePath = computed(() => getSmoothPath(orderPoints.value))

// ========================================================
// 4. DONUT CHART SVG COMPUTATIONS
// ========================================================
const donutRadius = 50
const donutCircumference = 2 * Math.PI * donutRadius // ~314.159

const donutSlices = computed(() => {
  let accumulatedPct = 0
  return statusDistribusi.map((item) => {
    const strokeDasharray = `${(item.pct / 100) * donutCircumference} ${donutCircumference}`
    const strokeDashoffset = -((accumulatedPct / 100) * donutCircumference)
    accumulatedPct += item.pct
    return {
      ...item,
      strokeDasharray,
      strokeDashoffset,
    }
  })
})

onMounted(() => {
  simulateLoading(750)
  startWaveLoop()
})

onUnmounted(() => {
  stopWaveLoop()
})
</script>

<template>
  <div class="space-y-4 sm:space-y-5 w-full max-w-7xl mx-auto select-none pb-8">
    
    <!-- ======================================================== -->
    <!-- TOP HEADER: TITLE, BADGES, REFRESH & SKELETON DEMO       -->
    <!-- ======================================================== -->
    <div class="flex flex-col sm:flex-row sm:items-center justify-between gap-3 pb-2 border-b border-neutral-200 dark:border-[var(--theme-border)] anim-fade-up">
      <div class="flex items-center gap-3">
        <div class="w-9 h-9 sm:w-10 sm:h-10 rounded-xl bg-[var(--theme-primary)] text-[var(--theme-primary-text)] flex items-center justify-center shrink-0 shadow-2xs">
          <Activity class="w-5 h-5" />
        </div>
        <div>
          <div class="flex items-center gap-2">
            <h1 class="text-lg sm:text-xl font-bold text-neutral-900 dark:text-white tracking-tight leading-none">
              Dashboard ERP & Payroll Digaji.id
            </h1>
            <BaseBadge variant="success" size="sm" dot pulse>Live Sync</BaseBadge>
          </div>
          <p class="text-xs text-neutral-500 dark:text-neutral-400 mt-1">
            Ringkasan realisasi payroll, alokasi anggaran SDM per divisi, kepatuhan regulasi, dan stabilitas modul ERP.
          </p>
        </div>
      </div>

      <div class="flex items-center gap-2 self-start sm:self-center flex-wrap">
        <!-- Button Tes Skeleton Loading -->
        <BaseButton
          variant="outline"
          size="sm"
          @click="simulateLoading(800)"
          :disabled="isLoading"
          title="Lihat Efek Shimmer Skeleton Loading"
        >
          <Sparkles class="w-3.5 h-3.5 text-amber-500" />
          <span class="hidden sm:inline">Tes Skeleton</span>
        </BaseButton>

        <!-- Button Segarkan Data -->
        <BaseButton
          variant="outline"
          size="sm"
          @click="handleRefresh"
          :disabled="isRefreshing || isLoading"
          title="Sinkronisasi Data Terbaru"
        >
          <RefreshCw class="w-3.5 h-3.5 text-neutral-500" :class="isRefreshing ? 'animate-spin' : ''" />
          <span class="hidden sm:inline">{{ isRefreshing ? 'Menyinkronkan...' : 'Segarkan Data' }}</span>
        </BaseButton>

        <BaseButton
          variant="primary"
          size="sm"
          @click="router.push('/template/data-2')"
        >
          <Users class="w-3.5 h-3.5" />
          <span>Data Karyawan</span>
        </BaseButton>
      </div>
    </div>

    <!-- ======================================================== -->
    <!-- BARIS 1: 6 TOP STAT KPI CARDS (DENGAN BASESTATCARD)     -->
    <!-- ======================================================== -->
    <div class="grid grid-cols-2 md:grid-cols-3 lg:grid-cols-6 gap-2.5 sm:gap-3 anim-fade-up">
      <!-- KPI 1: Total Payroll Batch -->
      <BaseStatCard
        title="Batch Payroll Bulan Ini"
        :value="kpiSummary.totalOrder.value"
        unit="batch"
        :icon="ShoppingBag"
        tone="sky"
        :trend="kpiSummary.totalOrder.growth"
        trend-direction="up"
        trend-label="dari periode lalu"
        :loading="isLoading"
      />

      <!-- KPI 2: Realisasi Gaji Disalurkan -->
      <BaseStatCard
        title="Realisasi Gaji Disalurkan"
        :value="kpiSummary.omzet.value"
        prefix="Rp "
        :icon="Wallet"
        tone="emerald"
        :trend="kpiSummary.omzet.growth"
        trend-direction="up"
        trend-label="dari bulan lalu"
        :loading="isLoading"
      />

      <!-- KPI 3: Tiket Klaim & Lembur -->
      <BaseStatCard
        title="Klaim & Pengajuan"
        :value="kpiSummary.orderDiproses.value"
        unit="tiket"
        :icon="RefreshCw"
        tone="amber"
        subtitle="Sedang antrean verifikasi HR"
        :loading="isLoading"
      />

      <!-- KPI 4: Karyawan Aktif -->
      <BaseStatCard
        title="Total Karyawan Aktif"
        :value="1248"
        unit="pegawai"
        :icon="Users"
        tone="indigo"
        :trend="kpiSummary.pelangganAktif.growth"
        trend-direction="up"
        trend-label="6 cabang aktif"
        :loading="isLoading"
      />

      <!-- KPI 5: Sistem ERP & Modul -->
      <BaseStatCard
        title="Sistem ERP & Modul"
        :value="kpiSummary.mesinAktif.aktif"
        :secondary-value="' / ' + kpiSummary.mesinAktif.total"
        unit="modul"
        :icon="Cpu"
        tone="cyan"
        :subtitle="kpiSummary.mesinAktif.subtitle"
        :loading="isLoading"
      />

      <!-- KPI 6: Evaluasi PKWT Segera -->
      <BaseStatCard
        title="Evaluasi PKWT"
        :value="kpiSummary.stokMenipis.count"
        unit="staf"
        :icon="AlertTriangle"
        tone="rose"
        :subtitle="kpiSummary.stokMenipis.warning"
        :loading="isLoading"
      />
    </div>

    <!-- ======================================================== -->
    <!-- BARIS 2: TIGA DIAGRAM LENGKAP & INTERAKTIF              -->
    <!-- 1. Tren Omzet & Order (Dual Line Curve Bergelombang)    -->
    <!-- 2. Volume Order per Layanan (Horizontal Bar)            -->
    <!-- 3. Distribusi Status Order (Donut Chart)                -->
    <!-- ======================================================== -->
    <div class="grid grid-cols-1 lg:grid-cols-12 gap-3 anim-fade-up anim-delay-2">
      
      <!-- ====================================================== -->
      <!-- DIAGRAM 1: Tren Omzet & Order                          -->
      <!-- ====================================================== -->
      <div class="lg:col-span-5 p-4 sm:p-4.5 rounded-2xl bg-white dark:bg-[var(--theme-surface)] border border-neutral-200 dark:border-[var(--theme-border)] shadow-2xs flex flex-col justify-between space-y-3">
        <!-- Skeleton Chart 1 -->
        <template v-if="isLoading">
          <div class="flex items-center justify-between gap-2">
            <div class="flex items-center gap-2">
              <BaseSkeleton variant="circular" width="16px" height="16px" />
              <BaseSkeleton width="130px" height="15px" />
            </div>
            <div class="flex items-center gap-1.5 shrink-0">
              <BaseSkeleton width="34px" height="34px" class="rounded-xl" />
              <BaseSkeleton width="150px" height="34px" class="rounded-xl" />
            </div>
          </div>
          <div class="flex items-center gap-4">
            <BaseSkeleton width="70px" height="12px" />
            <BaseSkeleton width="80px" height="12px" />
          </div>
          <div class="h-44 w-full rounded-xl skeleton-shimmer flex items-center justify-center">
            <div class="w-10 h-10 rounded-full border-2 border-neutral-300 dark:border-neutral-600 border-t-transparent animate-spin opacity-30" />
          </div>
        </template>

        <!-- Real Chart 1 dengan Gelombang Interaktif -->
        <template v-else>
          <!-- Chart Header (1 Single Horizontal Row) -->
          <div class="flex items-center justify-between gap-2 min-w-0">
            <div class="flex items-center gap-1.5 min-w-0">
              <TrendingUp class="w-4 h-4 text-[var(--theme-primary)] shrink-0" />
              <h2 class="text-xs sm:text-sm font-bold text-neutral-900 dark:text-white truncate" title="Tren Realisasi Payroll">
                Tren Realisasi Payroll
              </h2>
            </div>

            <!-- Periode Controls & Wave Toggle (Icon Saja & 1 Horizontal Line) -->
            <div class="flex items-center gap-1.5 shrink-0">
              <!-- Wave Toggle Button (Icon Saja) -->
              <button
                type="button"
                @click="toggleWave"
                class="w-8.5 h-8.5 rounded-xl border flex items-center justify-center transition cursor-pointer shrink-0"
                :class="isWaveActive ? 'bg-sky-50 dark:bg-sky-950/40 text-sky-600 dark:text-sky-400 border-sky-300 dark:border-sky-800' : 'bg-neutral-100 dark:bg-[var(--theme-surface-raised)] text-neutral-400 border-neutral-200 dark:border-[var(--theme-border)] hover:text-neutral-600 dark:hover:text-neutral-200'"
                :title="isWaveActive ? 'Matikan Animasi Gelombang' : 'Aktifkan Animasi Gelombang'"
              >
                <Waves class="w-4 h-4" :class="isWaveActive ? 'animate-pulse' : ''" />
              </button>

              <!-- BaseDatePicker Range Mode -->
              <div class="w-38 sm:w-44 shrink-0">
                <BaseDatePicker
                  v-model="dashboardDateRange"
                  range
                  size="sm"
                  placeholder="Rentang tanggal..."
                  @change="handleDateRangeChange"
                />
              </div>
            </div>
          </div>

          <!-- Legend Bar -->
          <div class="flex items-center justify-between text-[11px] text-neutral-500 dark:text-neutral-400">
            <div class="flex items-center gap-4">
              <div class="flex items-center gap-1.5">
                <span class="w-2.5 h-2.5 rounded-full bg-[var(--theme-primary)] inline-block" />
                <span>Realisasi Gaji (Rp)</span>
              </div>
              <div class="flex items-center gap-1.5">
                <span class="w-2.5 h-2.5 rounded-full border-2 border-sky-400 bg-white dark:bg-[var(--theme-surface)] inline-block" />
                <span>Batch Transaksi</span>
              </div>
            </div>
            <span v-if="isWaveActive" class="text-[10px] text-sky-500 font-mono tracking-tight hidden sm:inline">
              ~ wave undulating ~
            </span>
          </div>

          <!-- Interactive SVG Chart Area dengan Undulating Wave -->
          <div
            class="relative w-full h-44 select-none"
            @mouseenter="isHoveringChart = true"
            @mouseleave="isHoveringChart = false"
          >
            <svg
              class="w-full h-full overflow-visible"
              :viewBox="`0 0 ${svgWidth} ${svgHeight}`"
              preserveAspectRatio="none"
            >
              <defs>
                <linearGradient id="omzetGradient" x1="0" y1="0" x2="0" y2="1">
                  <stop offset="0%" stop-color="var(--theme-primary)" stop-opacity="0.30" />
                  <stop offset="100%" stop-color="var(--theme-primary)" stop-opacity="0.00" />
                </linearGradient>
              </defs>

              <!-- Horizontal Grid lines -->
              <line :x1="padX" :y1="getOmzetY(10000000)" :x2="svgWidth - padX" :y2="getOmzetY(10000000)" stroke="currentColor" class="text-neutral-200/70 dark:text-[var(--theme-border)]" stroke-dasharray="3 3" />
              <line :x1="padX" :y1="getOmzetY(6000000)" :x2="svgWidth - padX" :y2="getOmzetY(6000000)" stroke="currentColor" class="text-neutral-200/70 dark:text-[var(--theme-border)]" stroke-dasharray="3 3" />
              <line :x1="padX" :y1="getOmzetY(2000000)" :x2="svgWidth - padX" :y2="getOmzetY(2000000)" stroke="currentColor" class="text-neutral-200/70 dark:text-[var(--theme-border)]" stroke-dasharray="3 3" />
              <line :x1="padX" :y1="svgHeight - padYBottom" :x2="svgWidth - padX" :y2="svgHeight - padYBottom" stroke="currentColor" class="text-neutral-300 dark:text-[var(--theme-border-strong)]" />

              <!-- Y-Axis Ticks (Omzet) -->
              <text :x="padX - 8" :y="getOmzetY(10000000) + 3" text-anchor="end" class="text-[9.5px] fill-neutral-400 dark:fill-neutral-500 font-mono">10Jt</text>
              <text :x="padX - 8" :y="getOmzetY(6000000) + 3" text-anchor="end" class="text-[9.5px] fill-neutral-400 dark:fill-neutral-500 font-mono">6Jt</text>
              <text :x="padX - 8" :y="getOmzetY(2000000) + 3" text-anchor="end" class="text-[9.5px] fill-neutral-400 dark:fill-neutral-500 font-mono">2Jt</text>

              <!-- Y-Axis Ticks Right (Order) -->
              <text :x="svgWidth - padX + 8" :y="getOrderY(200) + 3" text-anchor="start" class="text-[9.5px] fill-neutral-400 dark:fill-neutral-500 font-mono">200</text>
              <text :x="svgWidth - padX + 8" :y="getOrderY(120) + 3" text-anchor="start" class="text-[9.5px] fill-neutral-400 dark:fill-neutral-500 font-mono">120</text>
              <text :x="svgWidth - padX + 8" :y="getOrderY(40) + 3" text-anchor="start" class="text-[9.5px] fill-neutral-400 dark:fill-neutral-500 font-mono">40</text>

              <!-- Area Path (Omzet) with Wave Undulation -->
              <path :d="omzetAreaPath" fill="url(#omzetGradient)" />

              <!-- Omzet Line (Primary Theme Stroke) with Wave Undulation -->
              <path
                :d="omzetLinePath"
                fill="none"
                stroke="var(--theme-primary)"
                stroke-width="2.5"
                stroke-linecap="round"
              />

              <!-- Order Line (Sky Blue Stroke) with Harmonic Wave -->
              <path
                :d="orderLinePath"
                fill="none"
                stroke="#0284c7"
                stroke-width="1.8"
                stroke-linecap="round"
                stroke-dasharray="4 2"
              />

              <!-- Interactive Vertical Hover Guideline -->
              <line
                v-if="omzetPoints[activeChartHoverIdx]"
                :x1="omzetPoints[activeChartHoverIdx].x"
                :y1="padYTop"
                :x2="omzetPoints[activeChartHoverIdx].x"
                :y2="svgHeight - padYBottom"
                stroke="var(--theme-primary)"
                stroke-width="1.5"
                stroke-dasharray="3 3"
                class="opacity-60"
              />

              <!-- Points on Omzet Line -->
              <circle
                v-for="(pt, idx) in omzetPoints"
                :key="'omz-' + idx"
                :cx="pt.x"
                :cy="pt.y"
                :r="activeChartHoverIdx === idx ? 5.5 : 3"
                fill="var(--theme-primary)"
                class="transition-all cursor-pointer"
                stroke="white"
                stroke-width="1.5"
                @mouseenter="activeChartHoverIdx = idx"
              />

              <!-- Points on Order Line -->
              <circle
                v-for="(pt, idx) in orderPoints"
                :key="'ord-' + idx"
                :cx="pt.x"
                :cy="pt.y"
                :r="activeChartHoverIdx === idx ? 4.5 : 2.5"
                fill="#0284c7"
                class="transition-all cursor-pointer"
                stroke="white"
                stroke-width="1"
                @mouseenter="activeChartHoverIdx = idx"
              />

              <!-- X-Axis Labels (Dates) -->
              <text
                v-for="(d, idx) in currentTren.dates"
                :key="'date-' + idx"
                :x="getPointX(idx, currentTren.dates.length)"
                :y="svgHeight - 12"
                text-anchor="middle"
                class="text-[10px] fill-neutral-500 dark:fill-neutral-400 font-medium"
              >
                {{ d }}
              </text>
            </svg>

            <!-- Interactive Hover Tooltip Box -->
            <div
              v-if="omzetPoints[activeChartHoverIdx]"
              class="absolute top-1 pointer-events-none transition-all duration-150 p-2.5 rounded-xl bg-white/95 dark:bg-[var(--theme-surface-raised)]/95 backdrop-blur-xs border border-neutral-200 dark:border-[var(--theme-border)] shadow-lg text-xs"
              :style="{
                left: `${Math.min(75, Math.max(15, (activeChartHoverIdx / (currentTren.dates.length - 1)) * 100))}%`,
                transform: 'translateX(-50%)',
              }"
            >
              <div class="font-bold text-neutral-900 dark:text-white border-b border-neutral-100 dark:border-[var(--theme-border)] pb-1 mb-1">
                {{ currentTren.dates[activeChartHoverIdx] }}
              </div>
              <div class="flex items-center gap-2 text-[11px] text-neutral-600 dark:text-neutral-300">
                <span class="w-2 h-2 rounded-full bg-[var(--theme-primary)] shrink-0" />
                <span>Realisasi Gaji:</span>
                <span class="font-bold text-neutral-900 dark:text-white font-mono">
                  Rp {{ (currentTren.omzet[activeChartHoverIdx]).toLocaleString('id-ID') }}
                </span>
              </div>
              <div class="flex items-center gap-2 text-[11px] text-neutral-600 dark:text-neutral-300 mt-0.5">
                <span class="w-2 h-2 rounded-full bg-sky-500 shrink-0" />
                <span>Batch Transaksi:</span>
                <span class="font-bold text-neutral-900 dark:text-white font-mono">
                  {{ currentTren.order[activeChartHoverIdx] }} batch
                </span>
              </div>
            </div>
          </div>
        </template>
      </div>

      <!-- ====================================================== -->
      <!-- DIAGRAM 2: Volume Order per Layanan (Horizontal Bar)   -->
      <!-- ====================================================== -->
      <div class="lg:col-span-4 p-4 sm:p-4.5 rounded-2xl bg-white dark:bg-[var(--theme-surface)] border border-neutral-200 dark:border-[var(--theme-border)] shadow-2xs flex flex-col justify-between space-y-3">
        <!-- Skeleton Chart 2 -->
        <template v-if="isLoading">
          <div class="flex items-center justify-between">
            <div class="flex items-center gap-2">
              <BaseSkeleton variant="circular" width="16px" height="16px" />
              <BaseSkeleton width="135px" height="15px" />
            </div>
            <BaseSkeleton width="50px" height="18px" class="rounded-md" />
          </div>
          <div class="space-y-3 pt-1">
            <div v-for="i in 5" :key="'bar-skel-' + i" class="space-y-1">
              <div class="flex items-center justify-between">
                <BaseSkeleton width="75px" height="12px" />
                <BaseSkeleton width="45px" height="12px" />
              </div>
              <BaseSkeleton height="8px" class="rounded-full" />
            </div>
          </div>
          <div class="pt-1 border-t border-neutral-100 dark:border-[var(--theme-border)]">
            <BaseSkeleton width="100%" height="10px" />
          </div>
        </template>

        <!-- Real Chart 2 -->
        <template v-else>
          <!-- Header -->
          <div class="flex items-center justify-between gap-2">
            <div class="flex items-center gap-2">
              <BarChart3 class="w-4 h-4 text-blue-500" />
              <h2 class="text-xs sm:text-sm font-bold text-neutral-900 dark:text-white">
                Distribusi SDM per Divisi
              </h2>
            </div>
            <span class="text-[10.5px] font-semibold text-neutral-400 bg-neutral-100 dark:bg-[var(--theme-surface-muted)] px-2 py-0.5 rounded-md">
              Aktif
            </span>
          </div>

          <!-- Horizontal Bar List dengan Animated Expansion & CountUp -->
          <div class="space-y-2.5 pt-1">
            <div
              v-for="layanan in layananData"
              :key="layanan.id"
              class="space-y-1 group"
            >
              <div class="flex items-center justify-between text-xs">
                <span class="font-medium text-neutral-700 dark:text-neutral-300 group-hover:text-neutral-900 dark:group-hover:text-white transition">
                  {{ layanan.label }}
                </span>
                <span class="font-bold font-mono text-neutral-900 dark:text-white">
                  <BaseCountUp :to="layanan.count" suffix=" " />
                  <span class="text-[10px] font-normal text-neutral-400 font-sans">staf</span>
                </span>
              </div>

              <!-- Animated Progress Bar (Ekspansi Halus) -->
              <div class="h-2 rounded-full bg-neutral-100 dark:bg-[var(--theme-surface-muted)] overflow-hidden flex">
                <div
                  class="h-full rounded-full transition-all duration-1000 ease-out"
                  :style="{
                    width: `${layanan.pct}%`,
                    backgroundColor: layanan.id === 'tech' ? 'var(--theme-primary)' : layanan.color,
                  }"
                />
              </div>
            </div>
          </div>

          <!-- Axis helper label -->
          <div class="flex items-center justify-between text-[10px] text-neutral-400 font-mono pt-1 border-t border-neutral-100 dark:border-[var(--theme-border)]">
            <span>0</span>
            <span>100</span>
            <span>200</span>
            <span>300</span>
            <span>450 pegawai</span>
          </div>
        </template>
      </div>

      <!-- ====================================================== -->
      <!-- DIAGRAM 3: Distribusi Status Order (Donut Chart)       -->
      <!-- ====================================================== -->
      <div class="lg:col-span-3 p-4 sm:p-4.5 rounded-2xl bg-white dark:bg-[var(--theme-surface)] border border-neutral-200 dark:border-[var(--theme-border)] shadow-2xs flex flex-col justify-between space-y-3">
        <!-- Skeleton Chart 3 -->
        <template v-if="isLoading">
          <div class="flex items-center justify-between">
            <div class="flex items-center gap-2">
              <BaseSkeleton variant="circular" width="16px" height="16px" />
              <BaseSkeleton width="100px" height="15px" />
            </div>
            <BaseSkeleton width="55px" height="18px" class="rounded-md" />
          </div>
          <div class="flex items-center justify-center py-2">
            <div class="w-28 h-28 rounded-full border-[12px] border-neutral-200 dark:border-[var(--theme-border)] skeleton-shimmer flex items-center justify-center">
              <BaseSkeleton width="40px" height="14px" />
            </div>
          </div>
          <div class="space-y-2 pt-1 border-t border-neutral-100 dark:border-[var(--theme-border)]">
            <div v-for="i in 4" :key="'donut-skel-' + i" class="flex items-center justify-between">
              <BaseSkeleton width="70px" height="12px" />
              <BaseSkeleton width="45px" height="12px" />
            </div>
          </div>
        </template>

        <!-- Real Chart 3 -->
        <template v-else>
          <!-- Header -->
          <div class="flex items-center justify-between gap-2">
            <div class="flex items-center gap-2">
              <PieChart class="w-4 h-4 text-emerald-500" />
              <h2 class="text-xs sm:text-sm font-bold text-neutral-900 dark:text-white">
                Status Pengajuan & Klaim
              </h2>
            </div>
            <BaseBadge variant="neutral" size="sm">142 Tiket</BaseBadge>
          </div>

          <!-- Circular Donut Chart dengan Hover & Smooth Stroke Animation -->
          <div class="flex items-center justify-center relative py-1">
            <svg class="w-32 h-32 -rotate-90 transform" viewBox="0 0 120 120">
              <!-- Background Ring -->
              <circle
                cx="60"
                cy="60"
                :r="donutRadius"
                fill="transparent"
                stroke="currentColor"
                class="text-neutral-100 dark:text-[var(--theme-surface-muted)]"
                stroke-width="14"
              />

              <!-- Slices with stroke-dasharray and animated transitions -->
              <circle
                v-for="slice in donutSlices"
                :key="slice.id"
                cx="60"
                cy="60"
                :r="donutRadius"
                fill="transparent"
                :stroke="slice.color"
                :stroke-width="hoveredDonutSlice === slice.id ? 17 : 14"
                :stroke-dasharray="slice.strokeDasharray"
                :stroke-dashoffset="slice.strokeDashoffset"
                stroke-linecap="round"
                class="transition-all duration-500 cursor-pointer"
                @mouseenter="hoveredDonutSlice = slice.id"
                @mouseleave="hoveredDonutSlice = null"
              />
            </svg>

            <!-- Center Text in Donut dengan CountUp -->
            <div class="absolute inset-0 flex flex-col items-center justify-center pointer-events-none text-center">
              <span class="text-xl font-bold font-mono text-neutral-900 dark:text-white leading-none">
                <BaseCountUp :to="142" />
              </span>
              <span class="text-[10px] text-neutral-400 tracking-wider uppercase font-semibold mt-0.5">
                Tiket
              </span>
            </div>
          </div>

          <!-- Legend with values & CountUp -->
          <div class="space-y-1.5 pt-1 border-t border-neutral-100 dark:border-[var(--theme-border)] text-xs">
            <div
              v-for="st in statusDistribusi"
              :key="st.id"
              class="flex items-center justify-between cursor-pointer transition p-1 rounded-lg"
              :class="hoveredDonutSlice === st.id ? 'bg-neutral-100 dark:bg-[var(--theme-surface-muted)]' : ''"
              @mouseenter="hoveredDonutSlice = st.id"
              @mouseleave="hoveredDonutSlice = null"
            >
              <div class="flex items-center gap-2">
                <span class="w-2.5 h-2.5 rounded-full shrink-0" :style="{ backgroundColor: st.color }" />
                <span class="text-neutral-700 dark:text-neutral-300 font-medium">{{ st.label }}</span>
              </div>
              <div class="font-mono text-neutral-900 dark:text-white font-semibold text-[11.5px]">
                <BaseCountUp :to="st.count" /> <span class="text-neutral-400 font-normal">({{ st.pct }}%)</span>
              </div>
            </div>
          </div>
        </template>
      </div>

    </div>

    <!-- ======================================================== -->
    <!-- BARIS 3: 4 KARTU RINGKASAN OPERASIONAL & KEUANGAN        -->
    <!-- 1. Ringkasan Keuangan                                    -->
    <!-- 2. Utilisasi Mesin (Gauge Radial)                        -->
    <!-- 3. Stok & Inventaris (Peringatan)                        -->
    <!-- 4. Pickup & Delivery + Absensi Karyawan                  -->
    <!-- ======================================================== -->
    <div class="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-4 gap-3 anim-fade-up anim-delay-3">
      
      <!-- ====================================================== -->
      <!-- KARTU 1: Ringkasan Finansial ERP                       -->
      <!-- ====================================================== -->
      <div class="p-4 rounded-2xl bg-white dark:bg-[var(--theme-surface)] border border-neutral-200 dark:border-[var(--theme-border)] shadow-2xs flex flex-col justify-between space-y-3">
        <template v-if="isLoading">
          <div class="flex items-center gap-2">
            <BaseSkeleton variant="circular" width="16px" height="16px" />
            <BaseSkeleton width="130px" height="15px" />
          </div>
          <div class="space-y-3 pt-1">
            <div v-for="i in 3" :key="'keu-skel-' + i" class="flex items-center justify-between">
              <BaseSkeleton width="80px" height="12px" />
              <BaseSkeleton width="90px" height="14px" />
            </div>
          </div>
        </template>

        <template v-else>
          <div class="flex items-center justify-between">
            <div class="flex items-center gap-2">
              <DollarSign class="w-4 h-4 text-emerald-500" />
              <h3 class="text-xs font-bold text-neutral-900 dark:text-white">
                Finansial & Arus Kas <span class="font-normal text-neutral-400">(Bulan Ini)</span>
              </h3>
            </div>
          </div>

          <div class="space-y-2 divide-y divide-neutral-100 dark:divide-[var(--theme-border)] text-xs">
            <div class="flex items-center justify-between pt-1">
              <span class="flex items-center gap-1.5 text-neutral-500">
                <ArrowUp class="w-3.5 h-3.5 text-emerald-500" /> Pendapatan
              </span>
              <span class="font-bold font-mono text-neutral-900 dark:text-white">
                <BaseCountUp :to="ringkasanKeuangan.pemasukan" prefix="Rp " />
              </span>
            </div>

            <div class="flex items-center justify-between pt-2">
              <span class="flex items-center gap-1.5 text-neutral-500">
                <ArrowDown class="w-3.5 h-3.5 text-rose-500" /> Beban Payroll & Ops
              </span>
              <span class="font-bold font-mono text-neutral-900 dark:text-white">
                <BaseCountUp :to="ringkasanKeuangan.pengeluaran" prefix="Rp " />
              </span>
            </div>

            <div class="flex items-center justify-between pt-2">
              <span class="font-bold text-neutral-800 dark:text-neutral-200">
                Arus Kas Bersih (Net)
              </span>
              <div class="text-right">
                <span class="font-bold font-mono text-sm text-[var(--theme-primary)] block">
                  <BaseCountUp :to="ringkasanKeuangan.labaKotor" prefix="Rp " />
                </span>
                <span class="text-[10px] text-emerald-600 font-semibold">
                  {{ ringkasanKeuangan.labaGrowthPct }} Margin
                </span>
              </div>
            </div>
          </div>
        </template>
      </div>

      <!-- ====================================================== -->
      <!-- KARTU 2: Utilisasi Cloud & Modul ERP (Gauge Radial)    -->
      <!-- ====================================================== -->
      <div class="p-4 rounded-2xl bg-white dark:bg-[var(--theme-surface)] border border-neutral-200 dark:border-[var(--theme-border)] shadow-2xs flex flex-col justify-between space-y-3">
        <template v-if="isLoading">
          <div class="flex items-center justify-between">
            <div class="flex items-center gap-2">
              <BaseSkeleton variant="circular" width="16px" height="16px" />
              <BaseSkeleton width="90px" height="15px" />
            </div>
            <BaseSkeleton width="50px" height="18px" class="rounded-md" />
          </div>
          <div class="flex items-center gap-4 py-1">
            <div class="w-16 h-16 rounded-full border-[6px] border-neutral-200 dark:border-[var(--theme-border)] skeleton-shimmer" />
            <div class="flex-1 space-y-2">
              <BaseSkeleton width="100%" height="11px" />
              <BaseSkeleton width="85%" height="11px" />
              <BaseSkeleton width="70%" height="11px" />
            </div>
          </div>
        </template>

        <template v-else>
          <div class="flex items-center justify-between">
            <div class="flex items-center gap-2">
              <Cpu class="w-4 h-4 text-cyan-500" />
              <h3 class="text-xs font-bold text-neutral-900 dark:text-white">
                Cloud & Modul ERP
              </h3>
            </div>
            <BaseBadge variant="info" size="sm">10 Service</BaseBadge>
          </div>

          <div class="flex items-center gap-4">
            <!-- Radial Gauge Mini dengan Animated Stroke -->
            <div class="relative w-16 h-16 shrink-0 flex items-center justify-center">
              <svg class="w-full h-full -rotate-90" viewBox="0 0 60 60">
                <circle cx="30" cy="30" r="24" fill="none" stroke="currentColor" class="text-neutral-100 dark:text-[var(--theme-surface-muted)]" stroke-width="6" />
                <circle
                  cx="30"
                  cy="30"
                  r="24"
                  fill="none"
                  stroke="var(--theme-primary)"
                  stroke-width="6"
                  stroke-dasharray="150.8"
                  :stroke-dashoffset="150.8 * (1 - utilisasiMesin.utilisasiPct / 100)"
                  stroke-linecap="round"
                  class="transition-all duration-1000 ease-out"
                />
              </svg>
              <div class="absolute inset-0 flex flex-col items-center justify-center text-center">
                <span class="text-xs font-bold font-mono text-neutral-900 dark:text-white leading-none">
                  <BaseCountUp :to="utilisasiMesin.utilisasiPct" suffix="%" />
                </span>
                <span class="text-[8px] text-neutral-400 font-semibold">SLA</span>
              </div>
            </div>

            <!-- Key Metrics List dengan CountUp -->
            <div class="flex-1 space-y-1 text-xs">
              <div class="flex items-center justify-between">
                <span class="text-neutral-500 text-[11px] flex items-center gap-1.5">
                  <span class="w-1.5 h-1.5 rounded-full bg-emerald-500" /> Modul Online:
                </span>
                <span class="font-bold text-neutral-900 dark:text-white font-mono">
                  <BaseCountUp :to="utilisasiMesin.aktifUnit" /> modul
                </span>
              </div>
              <div class="flex items-center justify-between">
                <span class="text-neutral-500 text-[11px]">Permintaan API:</span>
                <span class="font-bold text-neutral-900 dark:text-white font-mono">
                  <BaseCountUp :to="utilisasiMesin.totalSiklus" suffix="x" />
                </span>
              </div>
              <div class="flex items-center justify-between">
                <span class="text-neutral-500 text-[11px]">Uptime SLA:</span>
                <span class="font-bold text-neutral-900 dark:text-white font-mono">
                  <BaseCountUp :to="utilisasiMesin.totalKg" :decimals="1" suffix="%" />
                </span>
              </div>
            </div>
          </div>
        </template>
      </div>

      <!-- ====================================================== -->
      <!-- KARTU 3: Kepatuhan & Regulasi HR                        -->
      <!-- ====================================================== -->
      <div class="p-4 rounded-2xl bg-white dark:bg-[var(--theme-surface)] border border-neutral-200 dark:border-[var(--theme-border)] shadow-2xs flex flex-col justify-between space-y-3">
        <template v-if="isLoading">
          <div class="flex items-center justify-between">
            <div class="flex items-center gap-2">
              <BaseSkeleton variant="circular" width="16px" height="16px" />
              <BaseSkeleton width="110px" height="15px" />
            </div>
            <BaseSkeleton width="40px" height="14px" />
          </div>
          <div class="space-y-2 pt-1">
            <div v-for="i in 4" :key="'inv-skel-' + i" class="flex items-center justify-between py-1">
              <div class="space-y-1">
                <BaseSkeleton width="90px" height="12px" />
                <BaseSkeleton width="50px" height="10px" />
              </div>
              <BaseSkeleton width="48px" height="18px" class="rounded-md" />
            </div>
          </div>
        </template>

        <template v-else>
          <div class="flex items-center justify-between">
            <div class="flex items-center gap-2">
              <ShieldCheck class="w-4 h-4 text-emerald-500" />
              <h3 class="text-xs font-bold text-neutral-900 dark:text-white">
                Kepatuhan & Regulasi HR
              </h3>
            </div>
            <button
              type="button"
              @click="toast.info('Membuka modul audit kepatuhan lengkap...')"
              class="text-[10.5px] font-semibold text-[var(--theme-primary)] hover:underline flex items-center gap-0.5 cursor-pointer"
            >
              <span>Semua</span>
              <ArrowRight class="w-2.5 h-2.5" />
            </button>
          </div>

          <div class="space-y-1.5 text-xs">
            <div
              v-for="item in inventarisWarning"
              :key="item.id"
              class="flex items-center justify-between py-1 border-b border-neutral-100 dark:border-[var(--theme-border)] last:border-b-0"
            >
              <div class="min-w-0 pr-2">
                <span class="font-medium text-neutral-800 dark:text-neutral-200 truncate block text-[11.5px]">
                  {{ item.nama }}
                </span>
                <span class="text-[10px] text-neutral-400 font-mono">
                  {{ item.sisa }}
                </span>
              </div>
              <BaseBadge :variant="item.badgeVariant" size="sm">
                {{ item.status }}
              </BaseBadge>
            </div>
          </div>
        </template>
      </div>

      <!-- ====================================================== -->
      <!-- KARTU 4: Presensi & Distribusi Tim                      -->
      <!-- ====================================================== -->
      <div class="p-4 rounded-2xl bg-white dark:bg-[var(--theme-surface)] border border-neutral-200 dark:border-[var(--theme-border)] shadow-2xs flex flex-col justify-between space-y-3">
        <template v-if="isLoading">
          <div class="flex items-center gap-2">
            <BaseSkeleton variant="circular" width="16px" height="16px" />
            <BaseSkeleton width="120px" height="15px" />
          </div>
          <div class="grid grid-cols-2 gap-2">
            <div class="p-2.5 rounded-xl skeleton-shimmer h-14" />
            <div class="p-2.5 rounded-xl skeleton-shimmer h-14" />
          </div>
          <div class="pt-1 border-t border-neutral-100 dark:border-[var(--theme-border)] space-y-1.5">
            <BaseSkeleton width="80%" height="11px" />
            <BaseSkeleton height="6px" class="rounded-full" />
          </div>
        </template>

        <template v-else>
          <div class="flex items-center justify-between">
            <div class="flex items-center gap-2">
              <Users class="w-4 h-4 text-sky-500" />
              <h3 class="text-xs font-bold text-neutral-900 dark:text-white">
                Presensi & Distribusi Tim
              </h3>
            </div>
          </div>

          <!-- 2 Column WFO & WFH dengan CountUp -->
          <div class="grid grid-cols-2 gap-2 text-xs">
            <div class="p-2 rounded-xl bg-neutral-50 dark:bg-[var(--theme-surface-raised)] border border-neutral-100 dark:border-[var(--theme-border)]">
              <span class="text-[10px] text-neutral-400 block font-semibold uppercase">WFO (Kantor)</span>
              <span class="font-bold font-mono text-neutral-900 dark:text-white block mt-0.5 text-sm">
                <BaseCountUp :to="logistikDanKaryawan.pickup.count" suffix=" " />
                <span class="text-[10px] font-normal text-neutral-500">pegawai</span>
              </span>
              <span class="text-[10px] text-emerald-600 font-semibold">{{ logistikDanKaryawan.pickup.growth }}</span>
            </div>

            <div class="p-2 rounded-xl bg-neutral-50 dark:bg-[var(--theme-surface-raised)] border border-neutral-100 dark:border-[var(--theme-border)]">
              <span class="text-[10px] text-neutral-400 block font-semibold uppercase">WFH (Remote)</span>
              <span class="font-bold font-mono text-neutral-900 dark:text-white block mt-0.5 text-sm">
                <BaseCountUp :to="logistikDanKaryawan.delivery.count" suffix=" " />
                <span class="text-[10px] font-normal text-neutral-500">pegawai</span>
              </span>
              <span class="text-[10px] text-emerald-600 font-semibold">{{ logistikDanKaryawan.delivery.growth }}</span>
            </div>
          </div>

          <!-- Absensi Karyawan Bar dengan Animation -->
          <div class="pt-1.5 border-t border-neutral-100 dark:border-[var(--theme-border)] space-y-1">
            <div class="flex items-center justify-between text-xs">
              <span class="text-neutral-500 text-[11px]">Presensi Hari Ini:</span>
              <span class="font-bold font-mono text-neutral-900 dark:text-white text-[11px]">
                <BaseCountUp :to="logistikDanKaryawan.absensi.hadir" /> / {{ logistikDanKaryawan.absensi.total }} hadir
              </span>
            </div>
            <div class="h-1.5 rounded-full bg-neutral-100 dark:bg-[var(--theme-surface-muted)] overflow-hidden">
              <div
                class="h-full bg-emerald-500 rounded-full transition-all duration-1000 ease-out"
                :style="{ width: `${logistikDanKaryawan.absensi.pct}%` }"
              />
            </div>
          </div>
        </template>
      </div>

    </div>

    <!-- ======================================================== -->
    <!-- BARIS 4: TIMELINE AKTIVITAS OPERASIONAL & TABEL ORDER    -->
    <!-- 1. Timeline Aktivitas Real-Time (Left Col ~35%)           -->
    <!-- 2. Tabel Order Terbaru (Right Col ~65%)                  -->
    <!-- ======================================================== -->
    <div class="grid grid-cols-1 lg:grid-cols-12 gap-3 anim-fade-up anim-delay-4">
      
      <!-- ====================================================== -->
      <!-- TIMELINE AKTIVITAS OPERASIONAL (5 Col Desktop)         -->
      <!-- ====================================================== -->
      <div class="lg:col-span-5 p-4 sm:p-4.5 rounded-2xl bg-white dark:bg-[var(--theme-surface)] border border-neutral-200 dark:border-[var(--theme-border)] shadow-2xs flex flex-col justify-between space-y-3">
        <template v-if="isLoading">
          <div class="flex items-center justify-between">
            <div class="flex items-center gap-2">
              <BaseSkeleton variant="circular" width="16px" height="16px" />
              <BaseSkeleton width="130px" height="15px" />
            </div>
            <BaseSkeleton width="60px" height="14px" />
          </div>
          <div class="space-y-4 pt-2">
            <div v-for="i in 5" :key="'time-skel-' + i" class="flex items-start gap-3">
              <BaseSkeleton variant="circular" width="28px" height="28px" />
              <div class="flex-1 space-y-1.5">
                <div class="flex justify-between">
                  <BaseSkeleton width="110px" height="13px" />
                  <BaseSkeleton width="35px" height="11px" />
                </div>
                <BaseSkeleton width="85%" height="11px" />
              </div>
            </div>
          </div>
        </template>

        <template v-else>
          <div class="flex items-center justify-between">
            <div class="flex items-center gap-2">
              <Clock class="w-4 h-4 text-[var(--theme-primary)]" />
              <h2 class="text-xs sm:text-sm font-bold text-neutral-900 dark:text-white">
                Aktivitas Sistem ERP <span class="font-normal text-neutral-400">(Payroll & SDM)</span>
              </h2>
            </div>
            <button
              type="button"
              @click="toast.info('Menampilkan log audit seluruh aktivitas ERP...')"
              class="text-xs font-semibold text-[var(--theme-primary)] hover:underline flex items-center gap-0.5 cursor-pointer"
            >
              <span>Lihat Semua</span>
              <ArrowRight class="w-3 h-3" />
            </button>
          </div>

          <!-- Vertical Timeline List -->
          <div class="relative space-y-3.5 pl-2 pt-1 before:absolute before:left-5.5 before:top-3 before:bottom-3 before:w-0.5 before:bg-neutral-200 dark:before:bg-[var(--theme-surface-muted)]">
            <div
              v-for="act in aktivitasOperasional"
              :key="act.id"
              class="relative flex items-start gap-3 group"
            >
              <!-- Timeline Icon Bubble -->
              <div
                class="w-7 h-7 rounded-full bg-white dark:bg-[var(--theme-surface-muted)] border-2 border-neutral-300 dark:border-[var(--theme-border-strong)] flex items-center justify-center shrink-0 z-10 group-hover:scale-110 transition-transform shadow-xs"
                :class="
                  act.color === 'emerald' ? 'text-emerald-600 border-emerald-400' :
                  act.color === 'blue' ? 'text-blue-600 border-blue-400' :
                  act.color === 'sky' ? 'text-sky-600 border-sky-400' :
                  act.color === 'purple' ? 'text-purple-600 border-purple-400' :
                  'text-amber-600 border-amber-400'
                "
              >
                <ShoppingBag v-if="act.icon === 'cart'" class="w-3.5 h-3.5" />
                <Wallet v-else-if="act.icon === 'wallet'" class="w-3.5 h-3.5" />
                <Truck v-else-if="act.icon === 'truck'" class="w-3.5 h-3.5" />
                <Cpu v-else-if="act.icon === 'machine'" class="w-3.5 h-3.5" />
                <Package v-else class="w-3.5 h-3.5" />
              </div>

              <!-- Content -->
              <div class="flex-1 min-w-0">
                <div class="flex items-center justify-between gap-1">
                  <span class="text-xs font-bold text-neutral-900 dark:text-white truncate">
                    {{ act.judul }}
                  </span>
                  <span class="text-[10px] text-neutral-400 font-mono shrink-0">
                    {{ act.waktu }}
                  </span>
                </div>
                <p class="text-[11px] text-neutral-500 dark:text-neutral-400 mt-0.5 leading-snug truncate">
                  {{ act.deskripsi }}
                </p>
              </div>
            </div>
          </div>
        </template>
      </div>

      <!-- ====================================================== -->
      <!-- TABEL ORDER TERBARU (7 Col Desktop)                    -->
      <!-- ====================================================== -->
      <div class="lg:col-span-7 p-4 sm:p-4.5 rounded-2xl bg-white dark:bg-[var(--theme-surface)] border border-neutral-200 dark:border-[var(--theme-border)] shadow-2xs flex flex-col justify-between space-y-3">
        <div class="flex items-center justify-between">
          <div class="flex items-center gap-2">
            <ShoppingBag class="w-4 h-4 text-blue-500" />
            <h2 class="text-xs sm:text-sm font-bold text-neutral-900 dark:text-white">
              Transaksi & Pengajuan Payroll Terbaru
            </h2>
          </div>
          <button
            type="button"
            @click="router.push('/template/data-2')"
            class="text-xs font-semibold text-[var(--theme-primary)] hover:underline flex items-center gap-0.5 cursor-pointer"
          >
            <span>Buka Data Lengkap</span>
            <ArrowRight class="w-3 h-3" />
          </button>
        </div>

        <!-- Table Container -->
        <div class="overflow-x-auto rounded-xl border border-neutral-200/80 dark:border-[var(--theme-border)]">
          <table class="w-full text-left text-xs">
            <thead class="bg-neutral-50/90 dark:bg-[var(--theme-surface-raised)] text-neutral-500 dark:text-neutral-400 font-semibold border-b border-neutral-200 dark:border-[var(--theme-border)] text-[10.5px]">
              <tr>
                <th class="py-2 px-3">No. Tiket</th>
                <th class="py-2 px-3">Nama Pegawai</th>
                <th class="py-2 px-3">Komponen / Klaim</th>
                <th class="py-2 px-2.5 text-center">Divisi</th>
                <th class="py-2 px-3 text-right">Nominal</th>
                <th class="py-2 px-2.5 text-center">Status</th>
                <th class="py-2 px-2.5 text-center">Aksi</th>
              </tr>
            </thead>

            <!-- Skeleton Table Body -->
            <tbody v-if="isLoading" class="divide-y divide-neutral-100 dark:divide-[var(--theme-border)]">
              <tr v-for="i in 5" :key="'tbl-skel-' + i">
                <td class="py-3 px-3"><BaseSkeleton width="85px" height="14px" /></td>
                <td class="py-3 px-3"><BaseSkeleton width="95px" height="14px" /></td>
                <td class="py-3 px-3"><BaseSkeleton width="80px" height="14px" /></td>
                <td class="py-3 px-2.5 text-center"><BaseSkeleton width="40px" height="14px" class="mx-auto" /></td>
                <td class="py-3 px-3 text-right"><BaseSkeleton width="65px" height="14px" class="ml-auto" /></td>
                <td class="py-3 px-2.5 text-center"><BaseSkeleton width="55px" height="18px" class="rounded-md mx-auto" /></td>
                <td class="py-3 px-2.5 text-center"><BaseSkeleton variant="circular" width="18px" height="18px" class="mx-auto" /></td>
              </tr>
            </tbody>

            <!-- Real Loaded Table Body -->
            <tbody v-else class="divide-y divide-neutral-100 dark:divide-[var(--theme-border)] text-neutral-700 dark:text-neutral-300 text-[11px]">
              <tr
                v-for="order in orderTerbaruList"
                :key="order.noOrder"
                @click="openOrderDetail(order)"
                class="hover:bg-neutral-50/60 dark:hover:bg-[var(--theme-surface-raised)] transition cursor-pointer group"
              >
                <td class="py-2.5 px-3 font-mono font-medium text-neutral-900 dark:text-white whitespace-nowrap">
                  {{ order.noOrder }}
                </td>
                <td class="py-2.5 px-3 font-medium whitespace-nowrap">
                  {{ order.pelanggan }}
                </td>
                <td class="py-2.5 px-3 text-neutral-600 dark:text-neutral-400 whitespace-nowrap">
                  {{ order.layanan }}
                </td>
                <td class="py-2.5 px-2.5 text-center font-mono text-neutral-500 whitespace-nowrap">
                  {{ order.berat }}
                </td>
                <td class="py-2.5 px-3 text-right font-bold font-mono text-neutral-900 dark:text-white whitespace-nowrap">
                  <BaseCountUp :to="order.total" prefix="Rp " />
                </td>
                <td class="py-2.5 px-2.5 text-center whitespace-nowrap">
                  <BaseBadge :variant="order.statusVariant" size="sm">
                    {{ order.status }}
                  </BaseBadge>
                </td>
                <td class="py-2.5 px-2.5 text-center whitespace-nowrap" @click.stop>
                  <button
                    type="button"
                    @click="openOrderDetail(order)"
                    class="p-1 rounded-lg text-neutral-400 hover:text-neutral-800 dark:hover:text-white hover:bg-neutral-100 dark:hover:bg-[var(--theme-surface-muted)] transition cursor-pointer"
                    title="Lihat Detail Order"
                  >
                    <Eye class="w-3.5 h-3.5" />
                  </button>
                </td>
              </tr>
            </tbody>
          </table>
        </div>
      </div>

    </div>

    <!-- ======================================================== -->
    <!-- SLIDE-OVER QUICK DETAIL ORDER (BaseSlideOver.vue)        -->
    <!-- ======================================================== -->
    <BaseSlideOver
      v-model="isOrderSlideOverOpen"
      :title="selectedOrder ? `Rincian Order ${selectedOrder.noOrder}` : 'Rincian Order'"
      :subtitle="selectedOrder?.pelanggan"
      size="md"
    >
      <template #badge>
        <BaseBadge v-if="selectedOrder" :variant="selectedOrder.statusVariant" size="sm">
          {{ selectedOrder.status }}
        </BaseBadge>
      </template>

      <div v-if="selectedOrder" class="space-y-5 text-xs">
        <div class="p-4 rounded-2xl bg-neutral-50 dark:bg-[var(--theme-surface-raised)] border border-neutral-100 dark:border-[var(--theme-border)] space-y-3">
          <div class="flex items-center justify-between">
            <span class="text-neutral-400 uppercase font-semibold text-[10px]">Nama Pegawai</span>
            <span class="text-sm font-bold text-neutral-900 dark:text-white">{{ selectedOrder.pelanggan }}</span>
          </div>
          <div class="flex items-center justify-between">
            <span class="text-neutral-400 uppercase font-semibold text-[10px]">Komponen / Klaim</span>
            <span class="font-semibold text-neutral-800 dark:text-neutral-200">{{ selectedOrder.layanan }}</span>
          </div>
          <div class="flex items-center justify-between">
            <span class="text-neutral-400 uppercase font-semibold text-[10px]">Divisi Kerja</span>
            <span class="font-mono font-bold text-neutral-900 dark:text-white">{{ selectedOrder.berat }}</span>
          </div>
          <div class="flex items-center justify-between pt-2 border-t border-neutral-200 dark:border-[var(--theme-border)]">
            <span class="font-bold text-neutral-900 dark:text-white">Nominal Transaksi</span>
            <span class="text-base font-bold font-mono text-[var(--theme-primary)]">
              Rp {{ selectedOrder.total.toLocaleString('id-ID') }}
            </span>
          </div>
        </div>

        <!-- Activity Timeline for Order -->
        <div class="space-y-2">
          <span class="text-[10px] uppercase font-bold text-neutral-400 tracking-wider">Timeline Jejak Audit Transaksi</span>
          <div class="p-4 rounded-2xl bg-white dark:bg-[var(--theme-surface-raised)] border border-neutral-200 dark:border-[var(--theme-border)]">
            <BaseTimeline :items="getOrderTimeline(selectedOrder)" compact />
          </div>
        </div>
      </div>

      <template #footer="{ close }">
        <BaseButton variant="outline" size="sm" @click="close">
          Tutup
        </BaseButton>
        <BaseButton variant="primary" size="sm" @click="handlePrintNota(selectedOrder); close();">
          <Printer class="w-3.5 h-3.5" />
          <span>Cetak Faktur</span>
        </BaseButton>
      </template>
    </BaseSlideOver>

    <!-- ======================================================== -->
    <!-- FAKTUR / INVOICE PRINT MODAL (BaseInvoiceModal.vue)      -->
    <!-- ======================================================== -->
    <BaseInvoiceModal
      v-if="invoiceModalData"
      v-model="isInvoiceModalOpen"
      :invoice-data="invoiceModalData"
    />

  </div>
</template>
