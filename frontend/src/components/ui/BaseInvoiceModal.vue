<script setup>
import { computed } from 'vue'
import BaseModal from './BaseModal.vue'
import BaseButton from './BaseButton.vue'
import BaseBadge from './BaseBadge.vue'
import { Printer, Download, CheckCircle2, ShieldCheck, QrCode } from 'lucide-vue-next'

const props = defineProps({
  modelValue: {
    type: Boolean,
    default: false,
  },
  invoiceData: {
    type: Object,
    default: () => ({
      noInvoice: 'INV-2025-0842',
      tanggal: '27 Jan 2025',
      jatuhTempo: '03 Feb 2025',
      status: 'LUNAS',
      metodePembayaran: 'QRIS / Transfer Mandiri',
      customer: {
        nama: 'PT Sinar Harapan Abadi',
        kontak: 'Bpk. Hendra Wijaya',
        telepon: '0812-8877-6655',
        alamat: 'Jl. Sudirman Kav. 28, Jakarta Pusat',
      },
      perusahaan: {
        nama: 'Digaji.id Percetakan & Manufaktur',
        telepon: '(021) 555-7890',
        email: 'billing@digaji.id',
        alamat: 'Gedung Cyber 2 Tower Lt. 12, Jakarta',
      },
      items: [
        {
          nama: 'Brosur Full Color A4 Art Paper 150gr',
          spesifikasi: 'Cetak 2 Sisi, Lipat 3, Laminasi Doff',
          qty: 2500,
          harga: 1250,
          total: 3125000,
        },
        {
          nama: 'Kemasan Box Corrugated E-Flute Custom',
          spesifikasi: 'Ukuran 25x15x10 cm, Cetak Full UV Spot',
          qty: 500,
          harga: 4200,
          total: 2100000,
        },
        {
          nama: 'Stiker Vinyl Die-Cut Waterproof',
          spesifikasi: 'Diameter 6 cm, Tinta UV Outdoor',
          qty: 1000,
          harga: 650,
          total: 650000,
        },
      ],
      subtotal: 5875000,
      diskon: 200000,
      ppn: 624250,
      totalBayar: 6299250,
      catatan: 'Barang yang sudah diterima dan dicek QC tidak dapat dikembalikan. Terima kasih telah mempercayai layanan kami.',
    }),
  },
})

const emit = defineEmits(['update:modelValue'])

const formatRp = (num) => {
  return 'Rp ' + Number(num || 0).toLocaleString('id-ID')
}

const handlePrint = () => {
  window.print()
}
</script>

<template>
  <BaseModal
    :model-value="modelValue"
    @update:model-value="emit('update:modelValue', $event)"
    size="xl"
  >
    <template #header>
      <div class="flex items-center gap-2">
        <Printer class="w-4 h-4 text-[var(--theme-primary)]" />
        <h3 class="text-sm font-bold text-neutral-900 dark:text-white">
          Faktur Pembayaran Resmi
        </h3>
        <BaseBadge variant="success" size="sm">
          {{ invoiceData.status }}
        </BaseBadge>
      </div>
    </template>

    <!-- Printable Invoice Paper Surface -->
    <div
      id="printable-invoice"
      class="p-5 sm:p-8 rounded-2xl bg-white dark:bg-[var(--theme-surface)] text-neutral-800 dark:text-neutral-200 border border-neutral-200 dark:border-[var(--theme-border)] shadow-xs space-y-6 text-xs sm:text-sm select-none"
    >
      <!-- Invoice Top Bar: Logo & Invoice Metadata -->
      <div class="flex flex-col sm:flex-row items-start sm:items-center justify-between gap-4 pb-6 border-b border-neutral-200 dark:border-[var(--theme-border)]">
        <div>
          <div class="flex items-center gap-2.5">
            <img
              src="/logo-digaji.png"
              alt="Digaji.id Logo"
              class="w-8 h-8 rounded-xl object-contain shadow-xs shrink-0"
            />
            <span class="font-bold text-base sm:text-lg text-neutral-900 dark:text-white tracking-tight">
              {{ invoiceData.perusahaan?.nama || 'Digaji.id Printing' }}
            </span>
          </div>
          <p class="text-[11px] text-neutral-500 mt-1 max-w-xs">
            {{ invoiceData.perusahaan?.alamat }} &bull; {{ invoiceData.perusahaan?.telepon }}
          </p>
        </div>

        <div class="text-left sm:text-right">
          <span class="text-[10px] uppercase font-bold tracking-widest text-[var(--theme-primary)] block">
            FAKTUR TAGIHAN
          </span>
          <h2 class="text-lg sm:text-xl font-bold font-mono text-neutral-900 dark:text-white mt-0.5">
            {{ invoiceData.noInvoice }}
          </h2>
          <div class="flex flex-col sm:items-end text-[11px] text-neutral-500 mt-1 space-y-0.5">
            <span>Tanggal Terbit: <strong class="text-neutral-700 dark:text-neutral-300">{{ invoiceData.tanggal }}</strong></span>
            <span>Jatuh Tempo: <strong class="text-neutral-700 dark:text-neutral-300">{{ invoiceData.jatuhTempo }}</strong></span>
          </div>
        </div>
      </div>

      <!-- Bill To & Payment Method Info -->
      <div class="grid grid-cols-1 sm:grid-cols-2 gap-4 p-4 rounded-xl bg-neutral-50 dark:bg-[var(--theme-surface-raised)] border border-neutral-200/80 dark:border-[var(--theme-border)] text-xs">
        <div>
          <span class="text-[10px] uppercase font-bold text-neutral-400 block mb-1">Ditagihkan Kepada:</span>
          <div class="font-bold text-sm text-neutral-900 dark:text-white">{{ invoiceData.customer?.nama }}</div>
          <div class="text-neutral-600 dark:text-neutral-400 mt-0.5">{{ invoiceData.customer?.kontak }} ({{ invoiceData.customer?.telepon }})</div>
          <div class="text-neutral-500 text-[11px] mt-0.5">{{ invoiceData.customer?.alamat }}</div>
        </div>

        <div class="sm:border-l sm:border-neutral-200 dark:sm:border-[var(--theme-border)] sm:pl-4">
          <span class="text-[10px] uppercase font-bold text-neutral-400 block mb-1">Status & Pembayaran:</span>
          <div class="flex items-center gap-2">
            <span class="font-bold text-neutral-900 dark:text-white">{{ invoiceData.metodePembayaran }}</span>
            <span class="px-2 py-0.5 text-[10px] font-bold rounded-md bg-emerald-100 dark:bg-emerald-950/60 text-emerald-700 dark:text-emerald-400">
              {{ invoiceData.status }}
            </span>
          </div>
          <div class="flex items-center gap-1 text-[11px] text-emerald-600 dark:text-emerald-400 mt-1.5 font-medium">
            <CheckCircle2 class="w-3.5 h-3.5" />
            <span>Terverifikasi Otomatis Melalui Gateway</span>
          </div>
        </div>
      </div>

      <!-- Items Table -->
      <div class="overflow-x-auto rounded-xl border border-neutral-200 dark:border-[var(--theme-border)]">
        <table class="w-full text-left text-xs">
          <thead class="bg-neutral-100 dark:bg-[var(--theme-surface-raised)] text-neutral-600 dark:text-neutral-400 font-bold border-b border-neutral-200 dark:border-[var(--theme-border)]">
            <tr>
              <th class="py-2.5 px-3 w-10 text-center">No</th>
              <th class="py-2.5 px-3">Deskripsi Layanan / Produk</th>
              <th class="py-2.5 px-3 text-right">Qty</th>
              <th class="py-2.5 px-3 text-right">Harga Satuan</th>
              <th class="py-2.5 px-3 text-right">Total</th>
            </tr>
          </thead>
          <tbody class="divide-y divide-neutral-100 dark:divide-[var(--theme-border)]">
            <tr
              v-for="(item, idx) in invoiceData.items"
              :key="idx"
              class="hover:bg-neutral-50/50 dark:hover:bg-[var(--theme-surface-raised)]"
            >
              <td class="py-3 px-3 text-center font-mono text-neutral-400 font-medium">
                {{ idx + 1 }}
              </td>
              <td class="py-3 px-3">
                <div class="font-bold text-neutral-900 dark:text-white">{{ item.nama }}</div>
                <div class="text-[11px] text-neutral-500">{{ item.spesifikasi }}</div>
              </td>
              <td class="py-3 px-3 text-right font-mono font-medium">
                {{ item.qty.toLocaleString('id-ID') }}
              </td>
              <td class="py-3 px-3 text-right font-mono text-neutral-600 dark:text-neutral-400">
                {{ formatRp(item.harga) }}
              </td>
              <td class="py-3 px-3 text-right font-mono font-bold text-neutral-900 dark:text-white">
                {{ formatRp(item.total) }}
              </td>
            </tr>
          </tbody>
        </table>
      </div>

      <!-- Summary & Total Calculation -->
      <div class="flex flex-col sm:flex-row justify-between items-start gap-4 pt-2">
        <!-- Notes & Signature Info -->
        <div class="text-xs text-neutral-500 space-y-2 max-w-sm">
          <p class="italic text-[11px] leading-relaxed">
            * {{ invoiceData.catatan }}
          </p>
          <div class="flex items-center gap-2 pt-2 text-[10.5px] text-neutral-400">
            <ShieldCheck class="w-4 h-4 text-emerald-500" />
            <span>Dokumen ini sah dan dicetak secara elektronik oleh sistem Digaji.id.</span>
          </div>
        </div>

        <!-- Totals Calculation Block -->
        <div class="w-full sm:w-64 space-y-1.5 text-xs">
          <div class="flex justify-between text-neutral-600 dark:text-neutral-400">
            <span>Subtotal:</span>
            <span class="font-mono font-medium text-neutral-800 dark:text-neutral-200">{{ formatRp(invoiceData.subtotal) }}</span>
          </div>
          <div v-if="invoiceData.diskon" class="flex justify-between text-rose-600 dark:text-rose-400">
            <span>Diskon Promosi:</span>
            <span class="font-mono font-medium">-{{ formatRp(invoiceData.diskon) }}</span>
          </div>
          <div class="flex justify-between text-neutral-600 dark:text-neutral-400">
            <span>PPN (11%):</span>
            <span class="font-mono font-medium text-neutral-800 dark:text-neutral-200">{{ formatRp(invoiceData.ppn) }}</span>
          </div>
          <div class="pt-2 border-t border-neutral-200 dark:border-[var(--theme-border)] flex justify-between items-baseline font-bold text-sm sm:text-base text-neutral-900 dark:text-white">
            <span>Total Tagihan:</span>
            <span class="font-mono text-[var(--theme-primary)]">{{ formatRp(invoiceData.totalBayar) }}</span>
          </div>
        </div>
      </div>
    </div>

    <!-- Modal Footer Actions (Non-printable) -->
    <template #footer>
      <div class="flex items-center justify-between w-full">
        <span class="text-xs text-neutral-400 hidden sm:inline">
          Tekan tombol di samping untuk mencetak atau simpan sebagai PDF
        </span>
        <div class="flex items-center gap-2 ml-auto">
          <BaseButton variant="outline" size="sm" @click="emit('update:modelValue', false)">
            Tutup
          </BaseButton>
          <BaseButton variant="primary" size="sm" @click="handlePrint">
            <Printer class="w-3.5 h-3.5" />
            <span>Cetak / Unduh PDF</span>
          </BaseButton>
        </div>
      </div>
    </template>
  </BaseModal>
</template>

<style>
@media print {
  /* Sembunyikan semua elemen kecuali modal faktur */
  body * {
    visibility: hidden;
  }
  #printable-invoice,
  #printable-invoice * {
    visibility: visible;
  }
  #printable-invoice {
    position: absolute;
    left: 0;
    top: 0;
    width: 100%;
    margin: 0;
    padding: 20px;
    background: #ffffff !important;
    color: #000000 !important;
    border: none !important;
    box-shadow: none !important;
  }
}
</style>

