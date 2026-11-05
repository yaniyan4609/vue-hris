<script setup>
import { computed, onMounted, onUnmounted, ref, watch } from 'vue'
import { Plus, RefreshCw, Users } from 'lucide-vue-next'
import { apiRequest } from '../composables/useApi'
import { useToast } from '../composables/useToast'
import { adminCapabilities } from '../composables/useCapabilities'
import BaseButton from '../components/ui/BaseButton.vue'
import BaseInput from '../components/ui/BaseInput.vue'
import BaseSearchableSelect from '../components/ui/BaseSearchableSelect.vue'
import BaseModal from '../components/ui/BaseModal.vue'
import BaseEmptyState from '../components/ui/BaseEmptyState.vue'
import BaseBadge from '../components/ui/BaseBadge.vue'
import BasePagination from '../components/ui/BasePagination.vue'

const toast = useToast()
const items = ref([])
const total = ref(0)
const page = ref(1)
const pageSize = ref(15)
const offices = ref([])
const positions = ref([])
const loading = ref(false)
const saving = ref(false)
const open = ref(false)
const editingId = ref('')
const search = ref('')
const form = ref({ employeeNumber: '', displayName: '', status: 'active', baseOfficeId: '', positionId: '', reportsToId: '' })
const statusOptions = [
  { label: 'Aktif', value: 'active' }, { label: 'Masa percobaan', value: 'probation' },
  { label: 'Cuti', value: 'leave' },
]
const officeOptions = computed(() => offices.value.map((item) => ({ label: item.name, value: item.id })))
const positionOptions = computed(() => positions.value.map((item) => ({ label: item.name, value: item.id })))
const managerOptions = computed(() => [{ label: 'Tanpa atasan langsung', value: '' }, ...items.value.filter((item) => item.id !== editingId.value).map((item) => ({ label: item.displayName, value: item.id }))])
function statusLabel(value) { return statusOptions.find((item) => item.value === value)?.label || value }
function statusVariant(value) { return value === 'active' ? 'success' : value === 'probation' ? 'info' : value === 'leave' ? 'warning' : 'neutral' }

async function load() {
  loading.value = true
  try {
    const query = new URLSearchParams({ page: String(page.value), pageSize: String(pageSize.value), q: search.value })
    const [employees, officeList, positionList] = await Promise.all([
      apiRequest(`/admin/employees?${query}`),
      apiRequest('/admin/organization/offices?pageSize=100'),
      apiRequest('/admin/organization/positions?pageSize=100'),
    ])
    items.value = employees.items; total.value = employees.total; offices.value = officeList.items; positions.value = positionList.items
  } catch (error) { items.value = []; total.value = 0; toast.error(error.message, 'Gagal memuat karyawan') }
  finally { loading.value = false }
}
onMounted(load)
watch([page, pageSize], load)
let searchTimer
watch(search, () => {
  clearTimeout(searchTimer)
  searchTimer = setTimeout(() => {
    if (page.value === 1) load()
    else page.value = 1
  }, 250)
})
onUnmounted(() => clearTimeout(searchTimer))
function startCreate() { editingId.value = ''; form.value = { employeeNumber: '', displayName: '', status: 'active', baseOfficeId: '', positionId: '', reportsToId: '' }; open.value = true }
function startEdit(item) { editingId.value = item.id; form.value = { employeeNumber: item.employeeNumber, displayName: item.displayName, status: item.status, baseOfficeId: item.baseOfficeId, positionId: item.positionId, reportsToId: item.reportsToId || '' }; open.value = true }
async function save() {
  if (saving.value) return
  saving.value = true
  try {
    await apiRequest(`/admin/employees${editingId.value ? `/${editingId.value}` : ''}`, { method: editingId.value ? 'PATCH' : 'POST', body: JSON.stringify(form.value) })
    open.value = false
    toast.success(`Karyawan berhasil ${editingId.value ? 'diubah' : 'ditambahkan'}.`)
    await load()
  } catch (error) { toast.error(error.message, 'Gagal menyimpan') }
  finally { saving.value = false }
}
</script>

<template>
  <div class="h-full flex flex-col min-h-0 overflow-hidden gap-2.5 w-full max-w-7xl mx-auto select-none relative">
    <div class="shrink-0 flex items-center justify-between gap-3">
      <div class="flex items-center gap-2.5 min-w-0"><div class="w-8 h-8 rounded-xl bg-[var(--theme-primary-soft)] text-[var(--theme-primary)] flex items-center justify-center"><Users class="w-4 h-4" /></div><div><h1 class="text-lg sm:text-xl font-bold text-neutral-900 dark:text-white">Direktori Karyawan</h1><p class="text-xs text-neutral-500 dark:text-neutral-400">Data karyawan perusahaan</p></div><BaseBadge variant="neutral">{{ total }}</BaseBadge></div>
      <div class="flex gap-2"><BaseButton variant="outline" size="sm" :disabled="loading" title="Muat ulang" aria-label="Muat ulang" @click="load"><RefreshCw class="w-4 h-4" /></BaseButton><BaseButton v-if="adminCapabilities['karyawan.buat']" size="sm" @click="startCreate"><Plus class="w-4 h-4" /><span class="hidden sm:inline">Tambah</span></BaseButton></div>
    </div>
    <div class="shrink-0 rounded-2xl border border-neutral-200 dark:border-[var(--theme-border)] bg-white dark:bg-[var(--theme-surface)] p-3"><BaseInput v-model="search" size="sm" placeholder="Cari nama atau nomor karyawan" clearable /></div>
    <div class="hidden md:flex flex-1 min-h-0 flex-col rounded-2xl border border-neutral-200 dark:border-[var(--theme-border)] bg-white dark:bg-[var(--theme-surface)] overflow-hidden">
      <div class="flex-1 min-h-0 overflow-auto"><table class="w-full text-xs text-left"><thead class="sticky top-0 z-10 bg-neutral-50 dark:bg-[var(--theme-surface-raised)] border-b border-neutral-200 dark:border-[var(--theme-border)] text-neutral-500 dark:text-neutral-400"><tr><th class="p-3">No. Karyawan</th><th class="p-3">Nama</th><th class="p-3">Kantor</th><th class="p-3">Jabatan</th><th class="p-3">Status</th><th class="p-3 text-right">Aksi</th></tr></thead><tbody><tr v-for="item in items" :key="item.id" class="border-b border-neutral-100 dark:border-[var(--theme-border)]"><td class="p-3 font-mono">{{ item.employeeNumber }}</td><td class="p-3 font-semibold text-neutral-900 dark:text-white">{{ item.displayName }}</td><td class="p-3">{{ item.officeName }}</td><td class="p-3">{{ item.positionName }}</td><td class="p-3"><BaseBadge :variant="statusVariant(item.status)">{{ statusLabel(item.status) }}</BaseBadge></td><td class="p-3 text-right"><BaseButton v-if="adminCapabilities['karyawan.ubah']" variant="outline" size="sm" @click="startEdit(item)">Ubah</BaseButton></td></tr></tbody></table><BaseEmptyState v-if="!loading && !items.length" title="Karyawan tidak ditemukan" :description="search ? 'Coba kata kunci lain.' : 'Tambahkan karyawan pertama untuk memulai direktori.'" compact /></div>
      <div class="shrink-0 border-t border-neutral-200 dark:border-[var(--theme-border)] p-2"><BasePagination v-model:current-page="page" v-model:items-per-page="pageSize" :total-items="total" :per-page-options="[15, 30, 50, 100]" /></div>
    </div>
    <div class="md:hidden flex-1 min-h-0 flex flex-col overflow-hidden"><div class="flex-1 min-h-0 overflow-y-auto space-y-2.5"><div v-for="item in items" :key="item.id" class="rounded-2xl border border-neutral-200 dark:border-[var(--theme-border)] bg-white dark:bg-[var(--theme-surface)] p-3"><div class="flex items-start justify-between gap-2"><div><p class="text-xs font-semibold text-neutral-900 dark:text-white">{{ item.displayName }}</p><p class="text-[11px] text-neutral-500 dark:text-neutral-400 mt-1">{{ item.employeeNumber }}</p></div><BaseBadge :variant="statusVariant(item.status)">{{ statusLabel(item.status) }}</BaseBadge></div><p class="text-xs text-neutral-600 dark:text-neutral-300 mt-3">{{ item.positionName }} · {{ item.officeName }}</p><BaseButton v-if="adminCapabilities['karyawan.ubah']" variant="outline" size="sm" class="mt-3" @click="startEdit(item)">Ubah</BaseButton></div><BaseEmptyState v-if="!loading && !items.length" title="Karyawan tidak ditemukan" compact /></div><div class="shrink-0 pt-2"><BasePagination v-model:current-page="page" v-model:items-per-page="pageSize" :total-items="total" :per-page-options="[15, 30, 50, 100]" compact /></div></div>

    <BaseModal v-model="open" :title="editingId ? 'Ubah Karyawan' : 'Tambah Karyawan'" subtitle="Isi identitas dan penempatan kerja.">
      <div class="space-y-4">
        <BaseInput v-model="form.employeeNumber" label="Nomor karyawan" placeholder="Contoh: EMP-001" required />
        <BaseInput v-model="form.displayName" label="Nama karyawan" placeholder="Nama lengkap" required />
        <div><p class="text-xs font-semibold mb-1">Status</p><BaseSearchableSelect v-model="form.status" :options="statusOptions" :searchable="false" size="md" /></div>
        <div><p class="text-xs font-semibold mb-1">Kantor utama</p><BaseSearchableSelect v-model="form.baseOfficeId" :options="officeOptions" placeholder="Pilih kantor" size="md" /></div>
        <div><p class="text-xs font-semibold mb-1">Jabatan</p><BaseSearchableSelect v-model="form.positionId" :options="positionOptions" placeholder="Pilih jabatan" size="md" /></div>
        <div><p class="text-xs font-semibold mb-1">Atasan langsung</p><BaseSearchableSelect v-model="form.reportsToId" :options="managerOptions" placeholder="Pilih atasan" size="md" /></div>
      </div>
      <template #footer><BaseButton variant="outline" size="sm" @click="open = false">Batal</BaseButton><BaseButton size="sm" :disabled="saving || !form.employeeNumber || !form.displayName || !form.baseOfficeId || !form.positionId" @click="save">{{ saving ? 'Menyimpan...' : 'Simpan' }}</BaseButton></template>
    </BaseModal>
  </div>
</template>
