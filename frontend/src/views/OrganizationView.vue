<script setup>
import { computed, onUnmounted, ref, watch } from 'vue'
import { useRoute } from 'vue-router'
import { Building2, Plus, RefreshCw } from 'lucide-vue-next'
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

const route = useRoute()
const toast = useToast()
const types = {
  regions: { label: 'Wilayah', parent: null },
  offices: { label: 'Kantor', parent: 'regions', parentLabel: 'Wilayah' },
  divisions: { label: 'Divisi', parent: null },
  departments: { label: 'Departemen', parent: 'divisions', parentLabel: 'Divisi' },
  positions: { label: 'Jabatan', parent: 'departments', parentLabel: 'Departemen' },
}
const kind = computed(() => String(route.params.kind || 'regions'))
const config = computed(() => types[kind.value] || types.regions)
const items = ref([])
const total = ref(0)
const page = ref(1)
const pageSize = ref(15)
const search = ref('')
const parents = ref([])
const loading = ref(false)
const saving = ref(false)
const open = ref(false)
const editingId = ref('')
const form = ref({ code: '', name: '', parentId: '', timezone: 'Asia/Jakarta' })
let loadVersion = 0
let parentsVersion = 0

async function load() {
  const version = ++loadVersion
  loading.value = true
  try {
    const query = new URLSearchParams({ page: String(page.value), pageSize: String(pageSize.value), q: search.value })
    const data = await apiRequest(`/admin/organization/${kind.value}?${query}`)
    if (version !== loadVersion) return
    items.value = data.items
    total.value = data.total
  } catch (error) {
    if (version !== loadVersion) return
    items.value = []
    total.value = 0
    toast.error(error.message, 'Gagal memuat organisasi')
  } finally { if (version === loadVersion) loading.value = false }
}
async function loadParents() {
  const version = ++parentsVersion
  if (!config.value.parent) { parents.value = []; return }
  try {
    const data = await apiRequest(`/admin/organization/${config.value.parent}?pageSize=100`)
    if (version === parentsVersion) parents.value = data.items
  } catch (error) { if (version === parentsVersion) { parents.value = []; toast.error(error.message, 'Gagal memuat struktur induk') } }
}
let searchTimer
let resettingSearch = false
watch(kind, () => {
  clearTimeout(searchTimer)
  page.value = 1
  if (search.value) { resettingSearch = true; search.value = '' }
  loadParents()
}, { immediate: true })
watch([kind, page, pageSize], load, { immediate: true })
watch(search, () => {
  if (resettingSearch) { resettingSearch = false; return }
  clearTimeout(searchTimer)
  searchTimer = setTimeout(() => {
    if (page.value === 1) load()
    else page.value = 1
  }, 250)
})
onUnmounted(() => clearTimeout(searchTimer))
const parentOptions = computed(() => parents.value.map((item) => ({ label: item.name, value: item.id })))
function parentName(id) { return parents.value.find((item) => item.id === id)?.name || '—' }
function startCreate() { editingId.value = ''; form.value = { code: '', name: '', parentId: '', timezone: 'Asia/Jakarta' }; open.value = true }
function startEdit(item) { editingId.value = item.id; form.value = { code: item.code, name: item.name, parentId: item.parentId || '', timezone: item.timezone || 'Asia/Jakarta' }; open.value = true }
async function save() {
  if (saving.value) return
  saving.value = true
  try {
    const path = `/admin/organization/${kind.value}${editingId.value ? `/${editingId.value}` : ''}`
    await apiRequest(path, { method: editingId.value ? 'PATCH' : 'POST', body: JSON.stringify(form.value) })
    open.value = false
    toast.success(`${config.value.label} berhasil ${editingId.value ? 'diubah' : 'ditambahkan'}.`)
    await load()
  } catch (error) { toast.error(error.message, 'Gagal menyimpan') }
  finally { saving.value = false }
}
</script>

<template>
  <div class="h-full flex flex-col min-h-0 overflow-hidden gap-2.5 w-full max-w-7xl mx-auto select-none relative">
    <div class="shrink-0 flex items-center justify-between gap-3">
      <div class="flex items-center gap-2.5 min-w-0">
        <div class="w-8 h-8 rounded-xl bg-[var(--theme-primary-soft)] text-[var(--theme-primary)] flex items-center justify-center"><Building2 class="w-4 h-4" /></div>
        <div><h1 class="text-lg sm:text-xl font-bold text-neutral-900 dark:text-white">{{ config.label }}</h1><p class="text-xs text-neutral-500 dark:text-neutral-400">Struktur organisasi {{ config.label.toLowerCase() }}</p></div>
        <BaseBadge variant="neutral">{{ total }}</BaseBadge>
      </div>
      <div class="flex gap-2">
        <BaseButton variant="outline" size="sm" :disabled="loading" title="Muat ulang" aria-label="Muat ulang" @click="load"><RefreshCw class="w-4 h-4" /></BaseButton>
        <BaseButton v-if="adminCapabilities['organisasi.buat']" size="sm" @click="startCreate"><Plus class="w-4 h-4" /><span class="hidden sm:inline">Tambah</span></BaseButton>
      </div>
    </div>
    <div class="shrink-0 rounded-2xl border border-neutral-200 dark:border-[var(--theme-border)] bg-white dark:bg-[var(--theme-surface)] p-3"><BaseInput v-model="search" size="sm" :placeholder="`Cari kode atau nama ${config.label.toLowerCase()}`" clearable /></div>
    <div class="hidden md:flex flex-1 min-h-0 flex-col rounded-2xl border border-neutral-200 dark:border-[var(--theme-border)] bg-white dark:bg-[var(--theme-surface)] overflow-hidden">
      <div class="flex-1 min-h-0 overflow-auto">
        <table class="w-full text-left text-xs">
          <thead class="sticky top-0 z-10 bg-neutral-50 dark:bg-[var(--theme-surface-raised)] text-neutral-500 dark:text-neutral-400 border-b border-neutral-200 dark:border-[var(--theme-border)]"><tr><th class="p-3">Kode</th><th class="p-3">Nama</th><th v-if="config.parent" class="p-3">{{ config.parentLabel }}</th><th v-if="kind === 'offices'" class="p-3">Zona waktu</th><th class="p-3 text-right">Aksi</th></tr></thead>
          <tbody><tr v-for="item in items" :key="item.id" class="border-b border-neutral-100 dark:border-[var(--theme-border)]"><td class="p-3 font-mono">{{ item.code }}</td><td class="p-3 font-semibold text-neutral-900 dark:text-white">{{ item.name }}</td><td v-if="config.parent" class="p-3">{{ parentName(item.parentId) }}</td><td v-if="kind === 'offices'" class="p-3">{{ item.timezone }}</td><td class="p-3 text-right"><BaseButton v-if="adminCapabilities['organisasi.ubah']" variant="outline" size="sm" @click="startEdit(item)">Ubah</BaseButton></td></tr></tbody>
        </table>
        <BaseEmptyState v-if="!loading && !items.length" :title="`Belum ada ${config.label.toLowerCase()}`" description="Tambahkan data pertama untuk melengkapi struktur organisasi." compact />
      </div>
      <div class="shrink-0 border-t border-neutral-200 dark:border-[var(--theme-border)] p-2"><BasePagination v-model:current-page="page" v-model:items-per-page="pageSize" :total-items="total" :per-page-options="[15, 30, 50, 100]" /></div>
    </div>
    <div class="md:hidden flex-1 min-h-0 flex flex-col overflow-hidden"><div class="flex-1 min-h-0 overflow-y-auto space-y-2.5">
      <div v-for="item in items" :key="item.id" class="rounded-2xl border border-neutral-200 dark:border-[var(--theme-border)] bg-white dark:bg-[var(--theme-surface)] p-3 flex items-start justify-between gap-2">
        <div class="min-w-0"><p class="text-xs font-semibold text-neutral-900 dark:text-white">{{ item.name }}</p><p class="text-[11px] text-neutral-500 dark:text-neutral-400 mt-1">{{ item.code }}<span v-if="config.parent"> · {{ parentName(item.parentId) }}</span></p></div>
        <BaseButton v-if="adminCapabilities['organisasi.ubah']" variant="outline" size="sm" @click="startEdit(item)">Ubah</BaseButton>
      </div>
      <BaseEmptyState v-if="!loading && !items.length" :title="`Belum ada ${config.label.toLowerCase()}`" compact />
    </div><div class="shrink-0 pt-2"><BasePagination v-model:current-page="page" v-model:items-per-page="pageSize" :total-items="total" :per-page-options="[15, 30, 50, 100]" compact /></div></div>

    <BaseModal v-model="open" :title="`${editingId ? 'Ubah' : 'Tambah'} ${config.label}`" subtitle="Isi data organisasi sesuai struktur perusahaan.">
      <form id="organization-form" class="space-y-4" @submit.prevent="save">
        <BaseInput v-model="form.code" label="Kode" placeholder="Contoh: PUSAT" required />
        <BaseInput v-model="form.name" label="Nama" :placeholder="`Nama ${config.label.toLowerCase()}`" required />
        <div v-if="config.parent"><p class="text-xs font-semibold mb-1">{{ config.parentLabel }}</p><BaseSearchableSelect v-model="form.parentId" :options="parentOptions" :placeholder="`Pilih ${config.parentLabel.toLowerCase()}`" size="md" /></div>
        <BaseInput v-if="kind === 'offices'" v-model="form.timezone" label="Zona waktu" placeholder="Asia/Jakarta" />
      </form>
      <template #footer><BaseButton variant="outline" size="sm" @click="open = false">Batal</BaseButton><BaseButton size="sm" :disabled="saving || !form.code || !form.name || (config.parent && !form.parentId)" @click="save">{{ saving ? 'Menyimpan...' : 'Simpan' }}</BaseButton></template>
    </BaseModal>
  </div>
</template>
