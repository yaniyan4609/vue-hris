<script setup>
import { computed, onMounted, onUnmounted, ref, watch } from 'vue'
import { Building2, Plus, RefreshCw } from 'lucide-vue-next'
import { apiRequest } from '../composables/useApi'
import { useToast } from '../composables/useToast'
import BaseBadge from '../components/ui/BaseBadge.vue'
import BaseButton from '../components/ui/BaseButton.vue'
import BaseDataTable from '../components/ui/BaseDataTable.vue'
import BaseInput from '../components/ui/BaseInput.vue'
import BaseModal from '../components/ui/BaseModal.vue'
import BaseSearchableSelect from '../components/ui/BaseSearchableSelect.vue'

const toast = useToast()
const tenants = ref([])
const total = ref(0)
const page = ref(1)
const pageSize = ref(15)
const search = ref('')
const status = ref('all')
const loading = ref(false)
const selected = ref(null)
const formOpen = ref(false)
const columns = [
  { key: 'name', label: 'Perusahaan', sortable: true },
  { key: 'slug', label: 'Slug' },
  { key: 'status', label: 'Status' },
]
const statusOptions = [
  { label: 'Semua status', value: 'all' },
  { label: 'Aktif', value: 'active' },
  { label: 'Trial', value: 'trial' },
  { label: 'Verifikasi', value: 'pending_verification' },
  { label: 'Grace', value: 'grace' },
  { label: 'Ditangguhkan', value: 'suspended' },
  { label: 'Berhenti', value: 'stopped' },
]
const stats = computed(() => [
  { label: 'Total sesuai filter', value: total.value },
  { label: 'Aktif di halaman', value: tenants.value.filter((t) => t.status === 'active').length },
  { label: 'Trial di halaman', value: tenants.value.filter((t) => t.status === 'trial').length },
  { label: 'Lainnya di halaman', value: tenants.value.filter((t) => !['active', 'trial'].includes(t.status)).length },
])
let searchTimer

async function load() {
  loading.value = true
  try {
    const query = new URLSearchParams({ page: String(page.value), pageSize: String(pageSize.value), q: search.value })
    if (status.value !== 'all') query.set('status', status.value)
    const result = await apiRequest(`/platform/tenants?${query}`)
    tenants.value = result.items || []
    total.value = result.total || 0
  } catch (error) {
    tenants.value = []
    total.value = 0
    toast.error(error.message, 'Gagal memuat perusahaan')
  } finally { loading.value = false }
}
onMounted(load)
onUnmounted(() => clearTimeout(searchTimer))
watch([page, pageSize], load)
watch(status, () => { if (page.value === 1) load(); else page.value = 1 })
watch(search, () => {
  clearTimeout(searchTimer)
  searchTimer = setTimeout(() => { if (page.value === 1) load(); else page.value = 1 }, 250)
})
</script>

<template>
  <div class="h-full flex flex-col min-h-0 overflow-hidden gap-2.5 w-full max-w-7xl mx-auto select-none relative">
    <div class="shrink-0 flex items-center justify-between gap-3 pb-2 border-b border-neutral-200 dark:border-[var(--theme-border)]">
      <div class="flex items-center gap-2 min-w-0"><Building2 class="w-5 h-5 text-[var(--theme-primary)]" /><h1 class="text-lg font-bold text-neutral-900 dark:text-white">Perusahaan</h1><BaseBadge variant="neutral">{{ total }}</BaseBadge></div>
      <div class="flex gap-2"><BaseButton variant="outline" size="sm" title="Muat ulang" aria-label="Muat ulang" :disabled="loading" @click="load"><RefreshCw class="w-4 h-4" /></BaseButton><BaseButton size="sm" @click="formOpen = true"><Plus class="w-4 h-4" /><span class="hidden sm:inline">Tambah</span></BaseButton></div>
    </div>
    <div class="shrink-0 flex flex-col sm:flex-row gap-2"><BaseInput v-model="search" placeholder="Cari perusahaan atau slug" clearable class="flex-1" /><BaseSearchableSelect v-model="status" :options="statusOptions" :searchable="false" class="sm:w-52" /></div>
    <div class="grid grid-cols-2 sm:grid-cols-4 gap-2 shrink-0"><div v-for="stat in stats" :key="stat.label" class="h-9 flex items-center justify-between gap-2 px-2.5 rounded-xl border border-neutral-200 dark:border-[var(--theme-border)] bg-white dark:bg-[var(--theme-surface)]"><span class="text-[10px] uppercase font-bold text-neutral-500 dark:text-neutral-400 truncate">{{ stat.label }}</span><span class="text-xs font-bold text-neutral-900 dark:text-white">{{ stat.value }}</span></div></div>
    <BaseDataTable :columns="columns" :rows="tenants" :total-items="total" server-side :loading="loading" :current-page="page" :items-per-page="pageSize" empty-title="Belum ada perusahaan" empty-description="Data perusahaan sesuai izin platform akan tampil di sini." @update:current-page="page = $event" @update:items-per-page="pageSize = $event" @row-click="selected = $event">
      <template #cell-status="{ row }"><BaseBadge :variant="row.status === 'active' ? 'success' : 'warning'">{{ statusOptions.find((option) => option.value === row.status)?.label || row.status }}</BaseBadge></template>
      <template #actions="{ row }"><BaseButton variant="outline" size="sm" @click.stop="selected = row">Lihat detail</BaseButton></template>
    </BaseDataTable>
    <BaseModal :model-value="Boolean(selected)" title="Detail Perusahaan" @update:model-value="(open) => { if (!open) selected = null }"><div v-if="selected" class="space-y-3 text-sm"><div><p class="text-xs text-neutral-500">Nama perusahaan</p><p class="font-semibold text-neutral-900 dark:text-white">{{ selected.name }}</p></div><div><p class="text-xs text-neutral-500">Slug</p><p>{{ selected.slug }}</p></div><div><p class="text-xs text-neutral-500">Status</p><BaseBadge :variant="selected.status === 'active' ? 'success' : 'warning'">{{ selected.status }}</BaseBadge></div><p class="text-xs text-neutral-500">Pengelolaan perusahaan tersedia setelah API dan aturan bisnis modul selesai.</p></div></BaseModal>
    <BaseModal v-model="formOpen" title="Tambah Perusahaan" subtitle="Pratinjau formulir"><div class="space-y-3"><BaseInput label="Nama perusahaan" disabled /><BaseInput label="Slug" disabled /><p class="text-xs text-neutral-500">Formulir ini belum menyimpan data.</p></div><template #footer><BaseButton variant="outline" size="sm" @click="formOpen = false">Tutup</BaseButton><BaseButton size="sm" disabled>Simpan · Dalam pengembangan</BaseButton></template></BaseModal>
  </div>
</template>
