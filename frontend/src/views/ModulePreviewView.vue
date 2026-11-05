<script setup>
import { computed, ref, watch } from 'vue'
import { useRoute } from 'vue-router'
import { Plus, Search, SlidersHorizontal, Database, Clock3, CheckCircle2, AlertCircle } from 'lucide-vue-next'
import { menuByPath, pageByPath } from '../data/menuData'
import BaseBadge from '../components/ui/BaseBadge.vue'
import BaseButton from '../components/ui/BaseButton.vue'
import BaseDataTable from '../components/ui/BaseDataTable.vue'
import BaseEmptyState from '../components/ui/BaseEmptyState.vue'
import BaseInput from '../components/ui/BaseInput.vue'
import BaseModal from '../components/ui/BaseModal.vue'
import BaseSearchableSelect from '../components/ui/BaseSearchableSelect.vue'

const route = useRoute()
const module = computed(() => pageByPath(route.path))
const section = computed(() => menuByPath(route.path))
const search = ref('')
const status = ref('all')
const page = ref(1)
const pageSize = ref(15)
const formOpen = ref(false)
watch(() => route.path, () => { search.value = ''; status.value = 'all'; page.value = 1; formOpen.value = false })

const finance = new Set(['billing', 'plans', 'loans', 'reimbursements', 'payroll-runs', 'pay-components', 'tax-bpjs', 'payslips', 'bank-export'])
const people = new Set(['staff', 'contracts', 'mutations', 'discipline', 'terminations', 'assets', 'attendance-today', 'attendance-history', 'attendance-correction', 'attendance-monitor', 'roster', 'leave-requests', 'overtime', 'directory', 'employee-documents'])
const settings = computed(() => module.value?.kind === 'settings')
const columns = computed(() => {
  if (finance.has(module.value?.id)) return [{ key: 'reference', label: 'Nomor / Referensi', sortable: true }, { key: 'name', label: 'Nama', sortable: true }, { key: 'amount', label: 'Nominal', align: 'right' }, { key: 'status', label: 'Status' }, { key: 'date', label: 'Tanggal' }]
  if (people.has(module.value?.id)) return [{ key: 'name', label: 'Nama', sortable: true }, { key: 'employee', label: 'Karyawan', sortable: true }, { key: 'unit', label: 'Unit / Kantor' }, { key: 'status', label: 'Status' }, { key: 'date', label: 'Tanggal' }]
  return [{ key: 'code', label: 'Kode / Nomor', sortable: true }, { key: 'name', label: 'Nama / Judul', sortable: true }, { key: 'description', label: 'Keterangan' }, { key: 'status', label: 'Status' }, { key: 'date', label: 'Diperbarui' }]
})
const formFields = computed(() => finance.has(module.value?.id) ? ['Nomor / Referensi', 'Nama', 'Nominal'] : people.has(module.value?.id) ? ['Nama', 'Karyawan', 'Unit / Kantor'] : ['Kode / Nomor', 'Nama / Judul', 'Keterangan'])
const filterOptions = [{ label: 'Semua status', value: 'all' }, { label: 'Aktif', value: 'active' }, { label: 'Menunggu', value: 'pending' }, { label: 'Selesai', value: 'done' }]
const stats = [
  { label: 'Total', icon: Database },
  { label: 'Aktif', icon: CheckCircle2 },
  { label: 'Menunggu', icon: Clock3 },
  { label: 'Perlu Tindakan', icon: AlertCircle },
]
</script>

<template>
  <div v-if="module && !settings" class="h-full flex flex-col min-h-0 overflow-hidden gap-2.5 w-full max-w-7xl mx-auto select-none relative">
    <div class="flex items-center justify-between gap-3 pb-2 border-b border-neutral-200 dark:border-[var(--theme-border)] shrink-0">
      <div class="min-w-0"><div class="flex items-center gap-2"><h1 class="text-base sm:text-lg font-bold text-neutral-900 dark:text-white truncate">{{ module.label }}</h1><BaseBadge variant="warning" size="sm">Dalam pengembangan</BaseBadge></div><p class="text-xs text-neutral-500 dark:text-neutral-400 mt-0.5">{{ section?.label }} · Data akan tampil sesuai izin dan cakupan akses Anda.</p></div>
      <BaseButton variant="primary" size="sm" @click="formOpen = true"><Plus class="w-4 h-4" /><span class="hidden sm:inline">Tambah</span></BaseButton>
    </div>
    <div class="flex flex-col sm:flex-row items-stretch sm:items-center gap-2 shrink-0">
      <BaseInput v-model="search" placeholder="Cari data..." :leading-icon="Search" class="flex-1" />
      <BaseSearchableSelect v-model="status" :options="filterOptions" :searchable="false" align="right" class="sm:w-44" />
      <BaseButton variant="outline" size="sm" title="Filter tambahan dalam pengembangan" disabled><SlidersHorizontal class="w-4 h-4" />Filter</BaseButton>
    </div>
    <div class="grid grid-cols-2 sm:grid-cols-4 gap-2 shrink-0">
      <div v-for="stat in stats" :key="stat.label" class="h-9 px-2.5 flex items-center gap-2 rounded-xl border border-neutral-200 dark:border-[var(--theme-border)] bg-white dark:bg-[var(--theme-surface)] shadow-2xs"><component :is="stat.icon" class="w-4 h-4 text-[var(--theme-primary)]" /><span class="text-[10px] uppercase font-bold text-neutral-500 dark:text-neutral-400 truncate">{{ stat.label }}</span><span class="ml-auto font-bold text-xs text-neutral-900 dark:text-white">0</span></div>
    </div>
    <BaseDataTable :columns="columns" :rows="[]" :total-items="0" :current-page="page" :items-per-page="pageSize" :show-index="true" :empty-title="search || status !== 'all' ? 'Tidak ada hasil' : 'Belum ada data'" :empty-description="search || status !== 'all' ? 'Ubah kata pencarian atau filter untuk melihat hasil lain.' : 'Modul ini sudah tersedia untuk ditinjau. Penyimpanan data akan diaktifkan pada tahap implementasi proses bisnis.'" :empty-action-text="search || status !== 'all' ? 'Reset Filter' : ''" :empty-icon="Database" @update:current-page="page = $event" @update:items-per-page="pageSize = $event" @empty-action="search = ''; status = 'all'" />
    <BaseModal v-model="formOpen" :title="`Tambah ${module.label}`" subtitle="Pratinjau struktur formulir" size="lg">
      <div class="space-y-3 py-2"><BaseInput v-for="field in formFields" :key="field" :label="field" :placeholder="`Masukkan ${field.toLowerCase()}`" disabled /><p class="text-xs text-neutral-500 dark:text-neutral-400">Formulir ini belum menyimpan data. Field dan validasi final mengikuti aturan bisnis modul.</p></div>
      <template #footer><BaseButton variant="outline" size="sm" @click="formOpen = false">Tutup</BaseButton><BaseButton size="sm" disabled>Simpan · Dalam pengembangan</BaseButton></template>
    </BaseModal>
  </div>
  <div v-else-if="module" class="space-y-6 w-full max-w-4xl mx-auto pb-8">
    <div class="border-b border-neutral-200 dark:border-[var(--theme-border)] pb-4"><div class="flex items-center gap-2"><h1 class="text-xl font-bold text-neutral-900 dark:text-white">{{ module.label }}</h1><BaseBadge variant="warning" size="sm">Dalam pengembangan</BaseBadge></div><p class="mt-1 text-xs text-neutral-500 dark:text-neutral-400">Pengaturan ini akan tersedia setelah aturan dan API modul selesai.</p></div>
    <div class="rounded-2xl border border-neutral-200 dark:border-[var(--theme-border)] bg-white dark:bg-[var(--theme-surface)] p-5"><BaseEmptyState title="Pengaturan belum tersedia" description="Halaman sudah terhubung ke menu dan izin. Konfigurasi akan diaktifkan pada tahap modul terkait." compact /></div>
  </div>
</template>
