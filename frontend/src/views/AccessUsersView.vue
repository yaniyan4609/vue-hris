<script setup>
import { computed, onMounted, onUnmounted, ref, watch } from 'vue'
import { KeyRound, Plus, RefreshCw, ShieldCheck } from 'lucide-vue-next'
import { apiRequest } from '../composables/useApi'
import { useAuth } from '../composables/useAuth'
import { useToast } from '../composables/useToast'
import { adminCapabilities } from '../composables/useCapabilities'
import BaseBadge from '../components/ui/BaseBadge.vue'
import BaseButton from '../components/ui/BaseButton.vue'
import BaseEmptyState from '../components/ui/BaseEmptyState.vue'
import BaseInput from '../components/ui/BaseInput.vue'
import BaseModal from '../components/ui/BaseModal.vue'
import BasePagination from '../components/ui/BasePagination.vue'
import BasePasswordInput from '../components/ui/BasePasswordInput.vue'
import BaseSearchableSelect from '../components/ui/BaseSearchableSelect.vue'
import ScopePicker from '../components/access/ScopePicker.vue'

const toast = useToast()
const { state } = useAuth()
const items = ref([])
const total = ref(0)
const page = ref(1)
const pageSize = ref(15)
const search = ref('')
const status = ref('all')
const loading = ref(false)
const detailLoading = ref(false)
const saving = ref(false)
const selected = ref(null)
const roles = ref([])
const actionOpen = ref(false)
const action = ref('assign')
const revokeRole = ref(null)
const selectedPermission = ref(null)
const scopeOptions = ref({})
const preview = ref(null)
const form = ref({ roleId: '', validFrom: '', validUntil: '', decision: 'inherit', rules: [], currentPassword: '' })
const canManage = computed(() => Boolean(adminCapabilities.value['pengguna.kelola_akses']))
const isSelf = computed(() => selected.value?.user?.id === state.value?.user?.id)
const roleOptions = computed(() => roles.value.map((role) => ({ label: `${role.name} (${role.code})`, value: role.id })))
const decisionOptions = computed(() => [
  { label: 'Ikuti Role', value: 'inherit' },
  ...(!selectedPermission.value?.requiresScope || selectedPermission.value?.scopeOverride ? [{ label: 'Izinkan', value: 'allow' }] : []),
  { label: 'Tolak', value: 'deny' },
])
const stats = computed(() => [
  { label: 'Total sesuai filter', value: total.value },
  { label: 'Aktif di halaman', value: items.value.filter((item) => item.status === 'active').length },
  { label: 'Nonaktif di halaman', value: items.value.filter((item) => item.status === 'disabled').length },
  { label: 'Role aktif di halaman', value: items.value.reduce((sum, item) => sum + item.roleCount, 0) },
])
let loadVersion = 0
let detailVersion = 0
let searchTimer

function today() { return new Date().toLocaleDateString('en-CA') }
function roleStatus(role) {
  if (role.revokedAt) return 'Dicabut'
  if (role.validFrom > today()) return 'Terjadwal'
  if (role.validUntil && role.validUntil < today()) return 'Berakhir'
  return 'Aktif'
}
function roleVariant(role) { return roleStatus(role) === 'Aktif' ? 'success' : 'neutral' }
function sourceText(sources = []) { return sources.map((source) => source.startsWith('role:') ? `Role ${source.slice(5)}` : source === 'user:allow' ? 'Izin khusus' : 'Scope khusus').join(', ') || '—' }
function scopeText(scopes = []) {
  if (!scopes.length) return 'Tanpa scope'
  return scopes.map((scope) => {
    if (scope.allTenant) return 'Seluruh tenant'
    if (scope.self) return 'Diri sendiri'
    if (scope.directReports) return 'Bawahan langsung'
    return Object.entries({ regions: 'Wilayah', offices: 'Kantor', divisions: 'Divisi', departments: 'Departemen', positions: 'Jabatan' })
      .filter(([key]) => scope[key]?.length).map(([key, label]) => `${label} (${scope[key].length})`).join(' + ')
  }).join(' atau ')
}
async function load() {
  const version = ++loadVersion
  loading.value = true
  try {
    const query = new URLSearchParams({ page: String(page.value), pageSize: String(pageSize.value), q: search.value })
    if (status.value !== 'all') query.set('status', status.value)
    const data = await apiRequest(`/admin/access/users?${query}`)
    if (version !== loadVersion) return
    items.value = data.items
    total.value = data.total
  } catch (error) {
    if (version === loadVersion) { items.value = []; total.value = 0; toast.error(error.message, 'Gagal memuat pengguna') }
  } finally { if (version === loadVersion) loading.value = false }
}
async function loadRoles() {
  try { roles.value = await apiRequest('/admin/access/roles') }
  catch (error) { toast.error(error.message, 'Gagal memuat role') }
}
async function loadScopeOptions() {
  try { scopeOptions.value = await apiRequest('/admin/access/scope-options') }
  catch (error) { toast.error(error.message, 'Gagal memuat struktur organisasi') }
}
async function openUser(id) {
  const version = ++detailVersion
  detailLoading.value = true
  try {
    const data = await apiRequest(`/admin/access/users/${id}`)
    if (version === detailVersion) selected.value = data
  } catch (error) { if (version === detailVersion) toast.error(error.message, 'Gagal memuat akses pengguna') }
  finally { if (version === detailVersion) detailLoading.value = false }
}
function closeUser() { detailVersion++; detailLoading.value = false; selected.value = null }
function openAssign() {
  action.value = 'assign'
  form.value = { roleId: '', validFrom: today(), validUntil: '', decision: 'inherit', currentPassword: '' }
  actionOpen.value = true
}
function openRevoke(role) {
  action.value = 'revoke'
  revokeRole.value = role
  form.value = { roleId: '', validFrom: '', validUntil: '', decision: 'inherit', currentPassword: '' }
  actionOpen.value = true
}
function openPermission(permission) {
  action.value = 'permission'
  selectedPermission.value = permission
  form.value = { roleId: '', validFrom: '', validUntil: '', decision: permission.override, currentPassword: '' }
  actionOpen.value = true
  preview.value = null
}
function openScope(permission) {
  action.value = 'scope'
  selectedPermission.value = permission
  form.value = { roleId: '', validFrom: '', validUntil: '', decision: permission.override, rules: permission.scopes?.length ? structuredClone(permission.scopes) : [], currentPassword: '' }
  preview.value = null
  actionOpen.value = true
}
function restoreScope(permission) {
  action.value = 'restore-scope'
  selectedPermission.value = permission
  form.value = { roleId: '', validFrom: '', validUntil: '', decision: permission.override, rules: [], currentPassword: '' }
  actionOpen.value = true
}
async function previewAction() {
  if (!selected.value || !selectedPermission.value) return
  try {
    preview.value = await apiRequest('/admin/access/preview', { method: 'POST', body: JSON.stringify({ userId: selected.value.user.id, permissionCode: selectedPermission.value.code, decision: form.value.decision, ...(action.value === 'scope' ? { rules: form.value.rules } : {}) }) })
  } catch (error) { toast.error(error.message, 'Gagal menghitung pratinjau') }
}
async function saveAction() {
  if (saving.value || !selected.value || !form.value.currentPassword) return
  saving.value = true
  try {
    const userId = selected.value.user.id
    if (action.value === 'assign') {
      await apiRequest(`/admin/access/users/${userId}/roles`, { method: 'POST', body: JSON.stringify({ roleId: form.value.roleId, validFrom: form.value.validFrom, validUntil: form.value.validUntil, currentPassword: form.value.currentPassword }) })
    } else if (action.value === 'revoke') {
      await apiRequest(`/admin/access/users/${userId}/roles/${revokeRole.value.assignmentId}`, { method: 'DELETE', body: JSON.stringify({ currentPassword: form.value.currentPassword }) })
    } else if (action.value === 'scope') {
      await apiRequest(`/admin/access/users/${userId}/scopes/${selectedPermission.value.code}`, { method: 'PUT', body: JSON.stringify({ rules: form.value.rules, currentPassword: form.value.currentPassword }) })
    } else if (action.value === 'restore-scope') {
      await apiRequest(`/admin/access/users/${userId}/scopes/${selectedPermission.value.code}`, { method: 'DELETE', body: JSON.stringify({ currentPassword: form.value.currentPassword }) })
    } else {
      await apiRequest(`/admin/access/users/${userId}/permissions/${selectedPermission.value.code}`, { method: 'PUT', body: JSON.stringify({ decision: form.value.decision, currentPassword: form.value.currentPassword }) })
    }
    actionOpen.value = false
    toast.success(action.value === 'assign' ? 'Role berhasil diberikan.' : action.value === 'revoke' ? 'Role berhasil dicabut.' : action.value === 'scope' || action.value === 'restore-scope' ? 'Scope pengguna berhasil diubah.' : 'Izin pengguna berhasil diubah.')
    await Promise.all([openUser(userId), load()])
  } catch (error) { toast.error(error.message, 'Gagal mengubah akses') }
  finally { saving.value = false; form.value.currentPassword = '' }
}
watch(actionOpen, (open) => { if (!open) form.value.currentPassword = '' })
watch([page, pageSize], load)
watch(status, () => { if (page.value === 1) load(); else page.value = 1 })
watch(search, () => {
  clearTimeout(searchTimer)
  searchTimer = setTimeout(() => { if (page.value === 1) load(); else page.value = 1 }, 250)
})
onMounted(() => { load(); loadRoles(); loadScopeOptions() })
onUnmounted(() => clearTimeout(searchTimer))
</script>

<template>
  <div class="h-full flex flex-col min-h-0 gap-3 w-full max-w-7xl mx-auto">
    <div class="shrink-0 flex items-center justify-between gap-3">
      <div class="flex items-center gap-2.5 min-w-0"><div class="w-8 h-8 rounded-xl bg-[var(--theme-primary-soft)] text-[var(--theme-primary)] flex items-center justify-center"><ShieldCheck class="w-4 h-4" /></div><div><h1 class="text-lg sm:text-xl font-bold text-neutral-900 dark:text-white">Pengguna & Hak Akses</h1><p class="text-xs text-neutral-500 dark:text-neutral-400">Anggota dan role perusahaan ini</p></div></div>
      <BaseButton variant="outline" size="sm" :disabled="loading" title="Muat ulang" aria-label="Muat ulang" @click="load"><RefreshCw class="w-4 h-4" /></BaseButton>
    </div>
    <div class="shrink-0 flex flex-col sm:flex-row gap-2"><BaseInput v-model="search" size="sm" placeholder="Cari nama atau email" clearable class="flex-1" /><BaseSearchableSelect v-model="status" :options="[{label:'Semua status',value:'all'},{label:'Aktif',value:'active'},{label:'Nonaktif',value:'disabled'}]" :searchable="false" size="sm" class="sm:w-44" /></div>
    <div class="grid grid-cols-2 sm:grid-cols-4 gap-2 shrink-0"><div v-for="stat in stats" :key="stat.label" class="h-9 flex items-center justify-between gap-2 px-2.5 rounded-xl border border-neutral-200 dark:border-[var(--theme-border)] bg-white dark:bg-[var(--theme-surface)]"><span class="text-[10px] uppercase font-bold text-neutral-500 dark:text-neutral-400 truncate">{{ stat.label }}</span><span class="text-xs font-bold text-neutral-900 dark:text-white">{{ stat.value }}</span></div></div>
    <div class="flex-1 min-h-0 flex flex-col rounded-2xl border border-neutral-200 dark:border-[var(--theme-border)] bg-white dark:bg-[var(--theme-surface)] overflow-hidden">
      <div class="flex-1 min-h-0 overflow-auto">
        <div class="hidden md:block"><table class="w-full text-xs text-left"><thead class="sticky top-0 bg-neutral-50 dark:bg-[var(--theme-surface-raised)] border-b border-neutral-200 dark:border-[var(--theme-border)] text-neutral-500 dark:text-neutral-400"><tr><th class="p-3">Nama</th><th class="p-3">Email</th><th class="p-3">Status</th><th class="p-3">Role aktif</th><th class="p-3 text-right">Aksi</th></tr></thead><tbody><tr v-for="item in items" :key="item.id" class="border-b border-neutral-100 dark:border-[var(--theme-border)]"><td class="p-3 font-semibold text-neutral-900 dark:text-white">{{ item.name }}</td><td class="p-3">{{ item.email }}</td><td class="p-3"><BaseBadge :variant="item.status === 'active' ? 'success' : 'neutral'">{{ item.status }}</BaseBadge></td><td class="p-3">{{ item.roleCount }}</td><td class="p-3 text-right"><BaseButton variant="outline" size="sm" @click="openUser(item.id)">Lihat akses</BaseButton></td></tr></tbody></table></div>
        <div class="md:hidden p-3 space-y-2"><div v-for="item in items" :key="item.id" class="rounded-xl border border-neutral-200 dark:border-[var(--theme-border)] p-3"><div class="flex items-start justify-between gap-2"><div class="min-w-0"><p class="text-xs font-semibold text-neutral-900 dark:text-white">{{ item.name }}</p><p class="text-[11px] text-neutral-500 dark:text-neutral-400 break-all">{{ item.email }}</p></div><BaseBadge variant="info">{{ item.roleCount }} role</BaseBadge></div><BaseButton variant="outline" size="sm" class="mt-3" @click="openUser(item.id)">Lihat akses</BaseButton></div></div>
        <BaseEmptyState v-if="!loading && !items.length" title="Pengguna tidak ditemukan" :description="search ? 'Coba kata kunci lain.' : 'Belum ada anggota perusahaan.'" compact />
      </div>
      <div class="shrink-0 border-t border-neutral-200 dark:border-[var(--theme-border)] p-2"><BasePagination v-model:current-page="page" v-model:items-per-page="pageSize" :total-items="total" :per-page-options="[15, 30, 50, 100]" /></div>
    </div>
    <BaseModal :model-value="Boolean(selected) || detailLoading" title="Hak Akses Pengguna" size="xl" @update:model-value="(open) => { if (!open) closeUser() }">
      <div v-if="detailLoading" class="py-8 text-center text-sm text-neutral-500">Memuat akses...</div>
      <div v-else-if="selected" class="space-y-5">
        <div class="flex flex-wrap items-start justify-between gap-3"><div><h2 class="text-base font-bold text-neutral-900 dark:text-white">{{ selected.user.name }}</h2><p class="text-xs text-neutral-500 dark:text-neutral-400">{{ selected.user.email }} · Versi akses {{ selected.user.accessVersion }}</p></div><BaseButton v-if="canManage && !isSelf" size="sm" @click="openAssign"><Plus class="w-4 h-4" />Tambah role</BaseButton></div>
        <section><h3 class="text-sm font-bold mb-2">Role dan masa berlaku</h3><div v-if="selected.roles.length" class="space-y-2"><div v-for="role in selected.roles" :key="role.assignmentId" class="flex flex-wrap items-center justify-between gap-2 rounded-xl border border-neutral-200 dark:border-[var(--theme-border)] p-3"><div><p class="text-xs font-semibold">{{ role.name }} <span class="text-neutral-500 font-normal">({{ role.code }})</span></p><p class="text-[11px] text-neutral-500 dark:text-neutral-400">{{ role.validFrom }} – {{ role.validUntil || 'Tanpa akhir' }}</p></div><div class="flex items-center gap-2"><BaseBadge :variant="roleVariant(role)">{{ roleStatus(role) }}</BaseBadge><BaseButton v-if="canManage && !isSelf && ['Aktif', 'Terjadwal'].includes(roleStatus(role))" variant="outline" size="sm" @click="openRevoke(role)">Cabut</BaseButton></div></div></div><p v-else class="text-xs text-neutral-500">Belum ada role.</p></section>
        <section><h3 class="text-sm font-bold mb-2">Matriks permission dan scope</h3><div class="rounded-xl border border-neutral-200 dark:border-[var(--theme-border)] overflow-hidden max-h-80 overflow-y-auto"><div v-for="permission in selected.permissions" :key="permission.code" class="grid grid-cols-[minmax(0,1fr)_auto] gap-2 p-3 border-b last:border-b-0 border-neutral-200 dark:border-[var(--theme-border)] text-xs"><div class="min-w-0"><p class="font-semibold text-neutral-900 dark:text-white">{{ permission.code }}</p><p class="text-[11px] text-neutral-500 dark:text-neutral-400 mt-1">{{ sourceText(permission.sources) }} · {{ scopeText(permission.scopes) }}</p><p v-if="permission.override !== 'inherit' || permission.scopeOverride" class="text-[11px] text-[var(--theme-primary)] mt-1">{{ permission.override !== 'inherit' ? `Override: ${permission.override === 'deny' ? 'Tolak' : 'Izinkan'}` : '' }} {{ permission.scopeOverride ? '· Scope khusus' : '' }}</p></div><div class="flex flex-wrap items-center justify-end gap-1.5"><BaseBadge :variant="permission.allowed ? 'success' : 'neutral'">{{ permission.allowed ? 'Diizinkan' : 'Ditolak' }}</BaseBadge><BaseButton v-if="canManage && !isSelf" variant="outline" size="sm" @click="openPermission(permission)">Izin</BaseButton><BaseButton v-if="canManage && !isSelf && permission.requiresScope" variant="outline" size="sm" @click="openScope(permission)">Scope</BaseButton><BaseButton v-if="canManage && !isSelf && permission.scopeOverride" variant="outline" size="sm" @click="restoreScope(permission)">Pulihkan</BaseButton></div></div></div></section>
        <section><h3 class="text-sm font-bold mb-2">Riwayat perubahan akses</h3><div v-if="selected.history?.length" class="space-y-2 max-h-40 overflow-y-auto"><div v-for="entry in selected.history" :key="`${entry.at}-${entry.action}`" class="rounded-xl border border-neutral-200 dark:border-[var(--theme-border)] p-2.5 text-xs"><p class="font-semibold">{{ entry.action.replaceAll('_', ' ') }}</p><p class="text-[11px] text-neutral-500 dark:text-neutral-400">{{ entry.actor }} · {{ entry.at }}</p></div></div><p v-else class="text-xs text-neutral-500 dark:text-neutral-400">Belum ada perubahan akses yang tercatat.</p></section>
        <p v-if="isSelf" class="text-xs text-neutral-500 dark:text-neutral-400">Hak akses akun sendiri hanya dapat diubah oleh Admin lain yang berwenang.</p>
      </div>
    </BaseModal>
    <BaseModal v-model="actionOpen" :title="action === 'assign' ? 'Berikan Role' : action === 'revoke' ? 'Cabut Role' : action === 'scope' ? 'Atur Data Scope' : action === 'restore-scope' ? 'Pulihkan Scope Role' : 'Atur Izin Pengguna'" :subtitle="action === 'assign' ? 'Role berlaku pada tanggal yang dipilih.' : 'Perubahan berlaku pada request berikutnya dan tercatat di audit.'" size="lg">
      <div class="space-y-3">
        <template v-if="action === 'assign'"><div><p class="text-xs font-semibold mb-1">Role</p><BaseSearchableSelect v-model="form.roleId" :options="roleOptions" placeholder="Pilih role" size="md" /></div><BaseInput v-model="form.validFrom" type="date" label="Tanggal mulai" required /><BaseInput v-model="form.validUntil" type="date" label="Tanggal akhir (opsional)" /></template>
        <p v-else-if="action === 'revoke'" class="text-sm">Cabut role <strong>{{ revokeRole?.name }}</strong> dari {{ selected?.user?.name }}?</p>
        <p v-else-if="action === 'restore-scope'" class="text-sm">Hapus scope khusus untuk {{ selectedPermission?.code }} dan kembali mengikuti role?</p>
        <div v-else-if="action === 'scope'"><p class="text-xs font-semibold mb-2">{{ selectedPermission?.code }}</p><ScopePicker v-model="form.rules" :options="scopeOptions" /></div>
        <div v-else><p class="text-xs font-semibold mb-1">{{ selectedPermission?.code }}</p><BaseSearchableSelect v-model="form.decision" :options="decisionOptions" :searchable="false" size="md" /><p v-if="selectedPermission?.requiresScope && !selectedPermission.scopeOverride" class="text-[11px] text-neutral-500 dark:text-neutral-400 mt-2">Atur scope khusus terlebih dahulu untuk memberikan izin langsung.</p></div>
        <div v-if="action === 'scope' || action === 'permission'" class="rounded-xl border border-neutral-200 dark:border-[var(--theme-border)] p-3"><BaseButton variant="outline" size="sm" @click="previewAction">Pratinjau akses efektif</BaseButton><p v-if="preview" class="text-xs mt-2">{{ preview.allowed ? 'Diizinkan' : 'Ditolak' }} · {{ preview.reason }} · {{ scopeText(preview.scopes) }} · {{ sourceText(preview.sources) }}</p></div>
        <BasePasswordInput v-model="form.currentPassword" label="Kata sandi Admin untuk konfirmasi" required />
        <p class="text-[11px] text-neutral-500 dark:text-neutral-400">Perubahan hanya berhasil jika berada dalam hak delegasi Anda.</p>
      </div>
      <template #footer><BaseButton variant="outline" size="sm" @click="actionOpen = false">Batal</BaseButton><BaseButton size="sm" :disabled="saving || !form.currentPassword || (action === 'assign' && (!form.roleId || !form.validFrom)) || (action === 'scope' && !form.rules?.length)" @click="saveAction"><KeyRound class="w-4 h-4" />{{ saving ? 'Menyimpan...' : action === 'assign' ? 'Berikan role' : action === 'revoke' ? 'Cabut role' : 'Simpan perubahan' }}</BaseButton></template>
    </BaseModal>
  </div>
</template>
