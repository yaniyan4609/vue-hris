<script setup>
import { ref, computed } from 'vue'
import BaseButton from '../../components/ui/BaseButton.vue'
import BaseBadge from '../../components/ui/BaseBadge.vue'
import BaseInput from '../../components/ui/BaseInput.vue'
import BaseSearchableSelect from '../../components/ui/BaseSearchableSelect.vue'
import BaseCurrencyInput from '../../components/ui/BaseCurrencyInput.vue'
import BaseModal from '../../components/ui/BaseModal.vue'
import BaseSlideOver from '../../components/ui/BaseSlideOver.vue'
import BaseInvoiceModal from '../../components/ui/BaseInvoiceModal.vue'
import BaseTimeline from '../../components/ui/BaseTimeline.vue'
import BaseEmptyState from '../../components/ui/BaseEmptyState.vue'
import { useToast } from '../../composables/useToast'
import {
  KanbanSquare,
  Plus,
  Search,
  Printer,
  Eye,
  Clock,
  CheckCircle2,
  AlertTriangle,
  Package,
  Layers,
  ArrowRight,
  User,
  Phone,
  Calendar,
  DollarSign,
  GripVertical,
} from 'lucide-vue-next'

const toast = useToast()

// ========================================================
// 1. DATA STATE: ORDER PIPELINE
// ========================================================
const columns = [
  {
    id: 'pending',
    title: 'Menunggu Pembayaran',
    color: 'border-amber-400 text-amber-600 dark:text-amber-400 bg-amber-50 dark:bg-amber-950/30',
    dotBg: 'bg-amber-400',
  },
  {
    id: 'queue',
    title: 'Antrean Produksi',
    color: 'border-sky-400 text-sky-600 dark:text-sky-400 bg-sky-50 dark:bg-sky-950/30',
    dotBg: 'bg-sky-400',
  },
  {
    id: 'qc',
    title: 'Quality Control',
    color: 'border-purple-400 text-purple-600 dark:text-purple-400 bg-purple-50 dark:bg-purple-950/30',
    dotBg: 'bg-purple-400',
  },
  {
    id: 'completed',
    title: 'Selesai & Dikirim',
    color: 'border-emerald-400 text-emerald-600 dark:text-emerald-400 bg-emerald-50 dark:bg-emerald-950/30',
    dotBg: 'bg-emerald-400',
  },
]

const cards = ref([
  {
    id: 'ORD-9021',
    customer: 'PT Mandiri Graha',
    kontak: 'Ibu Ratna',
    telepon: '0812-3344-5566',
    layanan: 'Brosur Full Color A4',
    kategori: 'Offset',
    qty: '2.500 pcs',
    nominal: 3125000,
    deadline: '28 Jan 2025',
    status: 'pending',
    prioritas: 'Tinggi',
    timeline: [
      { id: 1, title: 'Order Dibuat oleh Sistem', time: '26 Jan 14:20', variant: 'primary', author: 'Sistem Web' },
      { id: 2, title: 'Menunggu Bukti Transfer BCA', time: '26 Jan 14:25', variant: 'warning', author: 'Finansial' },
    ],
  },
  {
    id: 'ORD-9022',
    customer: 'CV Sumber Rezeki',
    kontak: 'Bpk. Budi Santoso',
    telepon: '0857-1122-3344',
    layanan: 'Spanduk Flexi Korea 440g',
    kategori: 'Outdoor',
    qty: '12 pcs (3x1 m)',
    nominal: 1450000,
    deadline: '27 Jan 2025',
    status: 'queue',
    prioritas: 'Mendesak',
    timeline: [
      { id: 1, title: 'Pembayaran Lunas via QRIS', time: '27 Jan 09:10', variant: 'success', author: 'Payment Gateway' },
      { id: 2, title: 'File RIP Masuk Antrean Mesin Roland #02', time: '27 Jan 09:30', variant: 'primary', author: 'Operator Mesin' },
    ],
  },
  {
    id: 'ORD-9023',
    customer: 'Toko Kopi Kenangan Kita',
    kontak: 'Kak Nadia',
    telepon: '0819-7788-9900',
    layanan: 'Stiker Cup Vinyl Die-Cut',
    kategori: 'Merchandise',
    qty: '5.000 pcs',
    nominal: 1850000,
    deadline: '29 Jan 2025',
    status: 'queue',
    prioritas: 'Normal',
    timeline: [
      { id: 1, title: 'Pembayaran DP 50% Diterima', time: '27 Jan 10:15', variant: 'success', author: 'Kasir Utama' },
      { id: 2, title: 'Proses Cetak di Mesin UV Flatbed', time: '27 Jan 11:00', variant: 'info', author: 'Operator UV' },
    ],
  },
  {
    id: 'ORD-9024',
    customer: 'Yayasan Bina Nusantara',
    kontak: 'Ibu Santi Dewi',
    telepon: '0813-4455-6677',
    layanan: 'Buku Agenda Kulit Emboss',
    kategori: 'Offset',
    qty: '300 buku',
    nominal: 7500000,
    deadline: '30 Jan 2025',
    status: 'qc',
    prioritas: 'Tinggi',
    timeline: [
      { id: 1, title: 'Penjilidan & Lem Panas Selesai', time: '27 Jan 08:00', variant: 'success', author: 'Divisi Finishing' },
      { id: 2, title: 'Pemeriksaan Hasil Cetak & Jahitan (QC)', time: '27 Jan 13:45', variant: 'info', author: 'Tim Quality Control' },
    ],
  },
  {
    id: 'ORD-9025',
    customer: 'PT Global Niaga Sejahtera',
    kontak: 'Bpk. Ahmad Fauzi',
    telepon: '0811-2233-4455',
    layanan: 'Kemasan Box Corrugated E-Flute',
    kategori: 'Offset',
    qty: '1.000 box',
    nominal: 4200000,
    deadline: '27 Jan 2025',
    status: 'completed',
    prioritas: 'Normal',
    timeline: [
      { id: 1, title: 'Lolos Quality Control 100%', time: '27 Jan 11:30', variant: 'success', author: 'Tim QC' },
      { id: 2, title: 'Diserahkan ke Kurir Logistik Lalamove', time: '27 Jan 14:00', variant: 'success', author: 'Tim Logistik' },
    ],
  },
])

// ========================================================
// 2. SEARCH & FILTER
// ========================================================
const searchQuery = ref('')
const selectedKategori = ref('all')
const kategoriFilterOptions = [
  { value: 'all', label: 'Semua Kategori' },
  { value: 'Offset', label: 'Offset' },
  { value: 'Outdoor', label: 'Outdoor' },
  { value: 'Merchandise', label: 'Merchandise' },
]

const filteredCards = computed(() => {
  return cards.value.filter((card) => {
    const matchSearch =
      card.id.toLowerCase().includes(searchQuery.value.toLowerCase()) ||
      card.customer.toLowerCase().includes(searchQuery.value.toLowerCase()) ||
      card.layanan.toLowerCase().includes(searchQuery.value.toLowerCase())

    const matchKategori =
      selectedKategori.value === 'all' || card.kategori === selectedKategori.value

    return matchSearch && matchKategori
  })
})

const getCardsByStatus = (statusId) => {
  return filteredCards.value.filter((c) => c.status === statusId)
}

const getColumnTotalNominal = (statusId) => {
  const list = cards.value.filter((c) => c.status === statusId)
  return list.reduce((sum, item) => sum + (item.nominal || 0), 0)
}

// ========================================================
// 3. NATIVE DRAG & DROP
// ========================================================
const draggedCard = ref(null)
const dragOverCol = ref(null)

const onDragStart = (card) => {
  draggedCard.value = card
}

const onDragOver = (e, colId) => {
  e.preventDefault()
  dragOverCol.value = colId
}

const onDragLeave = () => {
  dragOverCol.value = null
}

const onDrop = (colId) => {
  if (draggedCard.value && draggedCard.value.status !== colId) {
    const prevStatus = draggedCard.value.status
    draggedCard.value.status = colId

    // Add timeline log
    const colObj = columns.find((c) => c.id === colId)
    draggedCard.value.timeline.unshift({
      id: Date.now(),
      title: `Status Dipindahkan ke ${colObj.title}`,
      time: 'Baru saja',
      variant: colId === 'completed' ? 'success' : 'primary',
      author: 'Admin Operasional',
    })

    toast.success(`Order ${draggedCard.value.id} dipindahkan ke ${colObj.title}!`, 'Pipeline Update')
  }
  draggedCard.value = null
  dragOverCol.value = null
}

// Move via button click (mobile-friendly)
const moveCardStatus = (card, nextStatus) => {
  card.status = nextStatus
  const colObj = columns.find((c) => c.id === nextStatus)
  card.timeline.unshift({
    id: Date.now(),
    title: `Status Berubah: ${colObj.title}`,
    time: 'Baru saja',
    variant: nextStatus === 'completed' ? 'success' : 'primary',
    author: 'Admin Operasional',
  })
  toast.success(`Order ${card.id} berhasil diperbarui!`, 'Pipeline Status')
}

// ========================================================
// 4. SLIDE-OVER DETAIL & INVOICE MODAL
// ========================================================
const isDetailOpen = ref(false)
const selectedCard = ref(null)

const openDetail = (card) => {
  selectedCard.value = card
  isDetailOpen.value = true
}

const isInvoiceOpen = ref(false)
const invoiceTarget = ref(null)

const openInvoice = (card) => {
  invoiceTarget.value = {
    noInvoice: 'INV-' + card.id.replace('ORD-', '2025-'),
    tanggal: '27 Jan 2025',
    jatuhTempo: card.deadline,
    status: card.status === 'completed' || card.status === 'qc' ? 'LUNAS' : 'PENDING',
    metodePembayaran: 'Transfer Bank Mandiri / QRIS',
    customer: {
      nama: card.customer,
      kontak: card.kontak,
      telepon: card.telepon,
      alamat: 'Jl. Ahmad Yani No. 88, Sektor Bisnis',
    },
    items: [
      {
        nama: card.layanan,
        spesifikasi: `Kategori ${card.kategori}, Prioritas ${card.prioritas}`,
        qty: parseInt(card.qty) || 1,
        harga: card.nominal,
        total: card.nominal,
      },
    ],
    subtotal: card.nominal,
    diskon: 0,
    ppn: Math.round(card.nominal * 0.11),
    totalBayar: Math.round(card.nominal * 1.11),
    catatan: 'Pesanan resmi telah diproses melalui Kanban Pipeline Digaji.id.',
  }
  isInvoiceOpen.value = true
}

// ========================================================
// 5. TAMBAH ORDER BARU CEPAT
// ========================================================
const isAddModalOpen = ref(false)
const newOrderForm = ref({
  customer: '',
  kontak: '',
  telepon: '',
  layanan: '',
  kategori: 'Offset',
  qty: '1.000 pcs',
  nominal: 1500000,
  deadline: '29 Jan 2025',
})

const submitNewOrder = () => {
  if (!newOrderForm.value.customer || !newOrderForm.value.layanan) {
    toast.error('Harap lengkapi nama pelanggan dan jenis layanan', 'Validasi')
    return
  }

  const newId = 'ORD-' + Math.floor(1000 + Math.random() * 9000)
  cards.value.unshift({
    id: newId,
    customer: newOrderForm.value.customer,
    kontak: newOrderForm.value.kontak || 'PIC Pemesan',
    telepon: newOrderForm.value.telepon || '0812-0000-0000',
    layanan: newOrderForm.value.layanan,
    kategori: newOrderForm.value.kategori,
    qty: newOrderForm.value.qty,
    nominal: Number(newOrderForm.value.nominal),
    deadline: newOrderForm.value.deadline,
    status: 'pending',
    prioritas: 'Normal',
    timeline: [
      { id: Date.now(), title: 'Order Ditambahkan Manual', time: 'Baru saja', variant: 'primary', author: 'Admin' },
    ],
  })

  toast.success(`Order baru ${newId} berhasil ditambahkan ke antrean!`, 'Order Dibuat')
  isAddModalOpen.value = false
  newOrderForm.value = {
    customer: '',
    kontak: '',
    telepon: '',
    layanan: '',
    kategori: 'Offset',
    qty: '1.000 pcs',
    nominal: 1500000,
    deadline: '29 Jan 2025',
  }
}
</script>

<template>
  <div class="h-full flex flex-col space-y-4 select-none">
    
    <!-- Top Action Bar -->
    <div class="flex flex-col sm:flex-row sm:items-center justify-between gap-3 pb-2 border-b border-neutral-200 dark:border-[var(--theme-border)] shrink-0">
      <div class="flex items-center gap-3">
        <div class="w-9 h-9 sm:w-10 sm:h-10 rounded-xl bg-[var(--theme-primary)] text-[var(--theme-primary-text)] flex items-center justify-center shrink-0 shadow-2xs">
          <KanbanSquare class="w-5 h-5" />
        </div>
        <div>
          <div class="flex items-center gap-2">
            <h1 class="text-base sm:text-lg font-bold text-neutral-900 dark:text-white tracking-tight leading-none">
              Pipeline Alur Pesanan (Kanban)
            </h1>
            <BaseBadge variant="neutral" size="sm">
              {{ filteredCards.length }} Total Order
            </BaseBadge>
          </div>
          <p class="text-xs text-neutral-500 mt-1">
            Geser kartu untuk memperbarui tahapan produksi, pratinjau detail, atau cetak faktur invoice.
          </p>
        </div>
      </div>

      <!-- Search, Filter & Tambah Pesanan -->
      <div class="flex items-center gap-2 flex-wrap">
        <div class="relative w-44 sm:w-56">
          <Search class="w-3.5 h-3.5 absolute left-3 top-1/2 -translate-y-1/2 text-neutral-400 pointer-events-none" />
          <input
            v-model="searchQuery"
            type="text"
            placeholder="Cari order atau klien..."
            class="w-full text-xs py-1.5 pl-8 pr-3 rounded-xl border border-neutral-200 dark:border-[var(--theme-border)] bg-white dark:bg-[var(--theme-surface)] text-neutral-800 dark:text-neutral-200 focus:outline-none focus:ring-1 focus:ring-[var(--theme-primary)]"
          />
        </div>

        <BaseSearchableSelect
          v-model="selectedKategori"
          :options="kategoriFilterOptions"
          :searchable="false"
          align="right"
          width-class="w-44"
        />

        <BaseButton variant="primary" size="sm" @click="isAddModalOpen = true">
          <Plus class="w-3.5 h-3.5" />
          <span>Tambah Order</span>
        </BaseButton>
      </div>
    </div>

    <!-- Kanban Board Grid (4 Kolom dengan Scroll Horizontal di Mobile) -->
    <div class="flex-1 min-h-0 overflow-x-auto pb-4">
      <div class="grid grid-cols-1 md:grid-cols-2 xl:grid-cols-4 gap-3.5 h-full min-w-[320px] xl:min-w-0">
        
        <!-- Column Loop -->
        <div
          v-for="col in columns"
          :key="col.id"
          @dragover="onDragOver($event, col.id)"
          @dragleave="onDragLeave"
          @drop="onDrop(col.id)"
          class="flex flex-col rounded-2xl bg-neutral-100/70 dark:bg-[var(--theme-surface)] border border-neutral-200 dark:border-[var(--theme-border)] p-3 transition-colors h-full"
          :class="dragOverCol === col.id ? 'ring-2 ring-[var(--theme-primary)] bg-[var(--theme-primary-soft)]' : ''"
        >
          <!-- Column Header -->
          <div class="flex items-center justify-between pb-2.5 mb-2.5 border-b border-neutral-200/80 dark:border-[var(--theme-border)] shrink-0">
            <div class="flex items-center gap-2">
              <span class="w-2.5 h-2.5 rounded-full shrink-0" :class="col.dotBg" />
              <h3 class="text-xs font-bold text-neutral-900 dark:text-white">
                {{ col.title }}
              </h3>
            </div>

            <!-- Card Counter Badge -->
            <span class="text-[10.5px] font-mono font-bold px-2 py-0.5 rounded-lg bg-white dark:bg-[var(--theme-surface-muted)] text-neutral-700 dark:text-neutral-300 border border-neutral-200 dark:border-[var(--theme-border-strong)]">
              {{ getCardsByStatus(col.id).length }}
            </span>
          </div>

          <!-- Column Total Rupiah Header Metric -->
          <div class="px-2 py-1.5 rounded-xl bg-white/60 dark:bg-[var(--theme-surface-raised)] border border-neutral-200/60 dark:border-[var(--theme-border)] text-[11px] flex items-center justify-between mb-3 shrink-0">
            <span class="text-neutral-400">Total Nilai:</span>
            <span class="font-bold font-mono text-neutral-800 dark:text-neutral-200">
              Rp {{ getColumnTotalNominal(col.id).toLocaleString('id-ID') }}
            </span>
          </div>

          <!-- Cards Scrollable Container inside Column -->
          <div class="flex-1 min-h-0 overflow-y-auto space-y-2.5 pr-0.5">
            <!-- Order Cards -->
            <div
              v-for="card in getCardsByStatus(col.id)"
              :key="card.id"
              draggable="true"
              @dragstart="onDragStart(card)"
              @click="openDetail(card)"
              class="p-3.5 rounded-xl bg-white dark:bg-[var(--theme-surface-raised)] border border-neutral-200 dark:border-[var(--theme-border)] shadow-2xs hover:border-neutral-300 dark:hover:border-neutral-600 transition-all cursor-pointer space-y-2.5 group relative"
            >
              <!-- Card Top: ID & Category Badge -->
              <div class="flex items-center justify-between gap-2">
                <div class="flex items-center gap-1.5">
                  <GripVertical class="w-3.5 h-3.5 text-neutral-300 dark:text-neutral-600 group-hover:text-neutral-500 transition shrink-0" />
                  <span class="font-mono font-bold text-xs text-[var(--theme-primary)]">
                    #{{ card.id }}
                  </span>
                </div>

                <div class="flex items-center gap-1">
                  <span
                    class="text-[9.5px] font-bold px-1.5 py-0.5 rounded-md uppercase"
                    :class="card.prioritas === 'Mendesak' ? 'bg-rose-100 text-rose-700 dark:bg-rose-950/60 dark:text-rose-300' : 'bg-neutral-100 text-neutral-600 dark:bg-neutral-800 dark:text-neutral-400'"
                  >
                    {{ card.prioritas }}
                  </span>
                  <BaseBadge variant="neutral" size="sm">{{ card.kategori }}</BaseBadge>
                </div>
              </div>

              <!-- Customer & Layanan -->
              <div>
                <h4 class="text-xs font-bold text-neutral-900 dark:text-white line-clamp-1">
                  {{ card.customer }}
                </h4>
                <p class="text-[11px] text-neutral-500 mt-0.5 line-clamp-1">
                  {{ card.layanan }} &bull; {{ card.qty }}
                </p>
              </div>

              <!-- Nominal & Deadline -->
              <div class="flex items-center justify-between pt-2 border-t border-neutral-100 dark:border-[var(--theme-border)] text-[11px]">
                <span class="font-mono font-bold text-neutral-900 dark:text-white">
                  Rp {{ card.nominal.toLocaleString('id-ID') }}
                </span>
                <span class="text-neutral-400 font-mono text-[10px] flex items-center gap-1">
                  <Clock class="w-3 h-3" />
                  {{ card.deadline }}
                </span>
              </div>

              <!-- Card Action Hover Toolbar -->
              <div class="flex items-center justify-between gap-1 pt-1">
                <!-- Move to Next Column Quick Button -->
                <div class="flex items-center gap-1">
                  <button
                    v-if="col.id === 'pending'"
                    type="button"
                    @click.stop="moveCardStatus(card, 'queue')"
                    class="text-[10px] py-1 px-2 rounded-lg bg-sky-50 dark:bg-sky-950/40 text-sky-600 dark:text-sky-400 font-semibold hover:bg-sky-100 transition"
                  >
                    Mulai Produksi &rarr;
                  </button>
                  <button
                    v-else-if="col.id === 'queue'"
                    type="button"
                    @click.stop="moveCardStatus(card, 'qc')"
                    class="text-[10px] py-1 px-2 rounded-lg bg-purple-50 dark:bg-purple-950/40 text-purple-600 dark:text-purple-400 font-semibold hover:bg-purple-100 transition"
                  >
                    Kirim ke QC &rarr;
                  </button>
                  <button
                    v-else-if="col.id === 'qc'"
                    type="button"
                    @click.stop="moveCardStatus(card, 'completed')"
                    class="text-[10px] py-1 px-2 rounded-lg bg-emerald-50 dark:bg-emerald-950/40 text-emerald-600 dark:text-emerald-400 font-semibold hover:bg-emerald-100 transition"
                  >
                    Tandai Selesai &rarr;
                  </button>
                </div>

                <!-- Invoice Button -->
                <button
                  type="button"
                  @click.stop="openInvoice(card)"
                  class="p-1 rounded-lg text-neutral-400 hover:text-neutral-700 dark:hover:text-neutral-200 hover:bg-neutral-100 dark:hover:bg-[var(--theme-surface-muted)] transition"
                  title="Cetak Faktur / Struk"
                >
                  <Printer class="w-3.5 h-3.5" />
                </button>
              </div>
            </div>

            <!-- Empty Column State -->
            <div
              v-if="getCardsByStatus(col.id).length === 0"
              class="py-8 text-center text-xs text-neutral-400 border border-dashed border-neutral-300 dark:border-[var(--theme-border)] rounded-xl space-y-1"
            >
              <Package class="w-5 h-5 mx-auto text-neutral-300 dark:text-neutral-600 mb-1" />
              <p class="font-medium">Tidak ada kartu</p>
              <p class="text-[10px] text-neutral-400">Tarik kartu ke sini</p>
            </div>
          </div>
        </div>

      </div>
    </div>

    <!-- ======================================================== -->
    <!-- SLIDE-OVER DETAIL PESANAN DENGAN TIMELINE                -->
    <!-- ======================================================== -->
    <BaseSlideOver
      v-model="isDetailOpen"
      :title="selectedCard ? `Detail Pesanan #${selectedCard.id}` : ''"
      :subtitle="selectedCard?.customer"
      size="md"
    >
      <template #badge>
        <BaseBadge
          v-if="selectedCard"
          :variant="selectedCard.status === 'completed' ? 'success' : selectedCard.status === 'qc' ? 'purple' : 'warning'"
          size="sm"
        >
          {{ selectedCard.status.toUpperCase() }}
        </BaseBadge>
      </template>

      <div v-if="selectedCard" class="space-y-5 text-xs">
        <!-- Customer & Contact Card -->
        <div class="p-3.5 rounded-2xl bg-neutral-50 dark:bg-[var(--theme-surface-raised)] border border-neutral-200 dark:border-[var(--theme-border)] space-y-2">
          <span class="text-[10px] uppercase font-bold text-neutral-400 tracking-wider">Informasi Pemesan</span>
          <div class="flex items-center justify-between">
            <div>
              <h4 class="font-bold text-sm text-neutral-900 dark:text-white">{{ selectedCard.customer }}</h4>
              <p class="text-neutral-500 mt-0.5">Kontak: {{ selectedCard.kontak }} &bull; {{ selectedCard.telepon }}</p>
            </div>
          </div>
        </div>

        <!-- Order Specification Box -->
        <div class="space-y-2">
          <span class="text-[10px] uppercase font-bold text-neutral-400 tracking-wider">Spesifikasi Order</span>
          <div class="grid grid-cols-2 gap-2 text-xs">
            <div class="p-2.5 rounded-xl border border-neutral-200 dark:border-[var(--theme-border)] bg-white dark:bg-[var(--theme-surface)]">
              <span class="text-neutral-400 text-[10px] block">Layanan:</span>
              <span class="font-bold text-neutral-900 dark:text-white">{{ selectedCard.layanan }}</span>
            </div>
            <div class="p-2.5 rounded-xl border border-neutral-200 dark:border-[var(--theme-border)] bg-white dark:bg-[var(--theme-surface)]">
              <span class="text-neutral-400 text-[10px] block">Jumlah (Qty):</span>
              <span class="font-bold text-neutral-900 dark:text-white">{{ selectedCard.qty }}</span>
            </div>
            <div class="p-2.5 rounded-xl border border-neutral-200 dark:border-[var(--theme-border)] bg-white dark:bg-[var(--theme-surface)]">
              <span class="text-neutral-400 text-[10px] block">Total Nominal:</span>
              <span class="font-bold font-mono text-[var(--theme-primary)]">Rp {{ selectedCard.nominal.toLocaleString('id-ID') }}</span>
            </div>
            <div class="p-2.5 rounded-xl border border-neutral-200 dark:border-[var(--theme-border)] bg-white dark:bg-[var(--theme-surface)]">
              <span class="text-neutral-400 text-[10px] block">Batas Waktu (Deadline):</span>
              <span class="font-bold text-neutral-900 dark:text-white">{{ selectedCard.deadline }}</span>
            </div>
          </div>
        </div>

        <!-- Activity Timeline Audit Log Component -->
        <div class="space-y-2 pt-2">
          <span class="text-[10px] uppercase font-bold text-neutral-400 tracking-wider">Riwayat & Timeline Proses</span>
          <div class="p-4 rounded-2xl bg-white dark:bg-[var(--theme-surface-raised)] border border-neutral-200 dark:border-[var(--theme-border)]">
            <BaseTimeline :items="selectedCard.timeline" compact />
          </div>
        </div>
      </div>

      <template #footer="{ close }">
        <BaseButton variant="outline" size="sm" @click="close">Tutup</BaseButton>
        <BaseButton variant="primary" size="sm" @click="openInvoice(selectedCard); close();">
          <Printer class="w-3.5 h-3.5" />
          <span>Cetak Faktur</span>
        </BaseButton>
      </template>
    </BaseSlideOver>

    <!-- ======================================================== -->
    <!-- INVOICE & STRUK MODAL COMPONENT                          -->
    <!-- ======================================================== -->
    <BaseInvoiceModal
      v-if="invoiceTarget"
      v-model="isInvoiceOpen"
      :invoice-data="invoiceTarget"
    />

    <!-- ======================================================== -->
    <!-- MODAL TAMBAH ORDER BARU CEPAT                            -->
    <!-- ======================================================== -->
    <BaseModal v-model="isAddModalOpen" title="Tambah Pesanan ke Kanban Pipeline" size="md">
      <div class="space-y-3.5 text-xs">
        <BaseInput
          v-model="newOrderForm.customer"
          label="Nama Perusahaan / Klien"
          placeholder="Contoh: PT Sumber Pangan Jaya"
          required
        />
        <div class="grid grid-cols-2 gap-3">
          <BaseInput
            v-model="newOrderForm.kontak"
            label="Nama Kontak / PIC"
            placeholder="Ibu Sarah"
          />
          <BaseInput
            v-model="newOrderForm.telepon"
            label="Nomor Telepon / WA"
            placeholder="0812-xxxx-xxxx"
          />
        </div>
        <BaseInput
          v-model="newOrderForm.layanan"
          label="Nama Layanan / Produk"
          placeholder="Contoh: Cetak Brosur A4 Art Paper"
          required
        />
        <div class="grid grid-cols-2 gap-3">
          <BaseCurrencyInput
            v-model="newOrderForm.nominal"
            label="Nominal Transaksi (Rp)"
            required
          />
          <BaseInput
            v-model="newOrderForm.qty"
            label="Jumlah / Qty"
            placeholder="Contoh: 1.000 lembar"
          />
        </div>
      </div>

      <template #footer>
        <BaseButton variant="outline" size="sm" @click="isAddModalOpen = false">Batal</BaseButton>
        <BaseButton variant="primary" size="sm" @click="submitNewOrder">
          <Plus class="w-3.5 h-3.5" />
          <span>Simpan Pesanan</span>
        </BaseButton>
      </template>
    </BaseModal>

  </div>
</template>

