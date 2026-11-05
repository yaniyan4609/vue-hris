<script setup>
import { computed, onMounted, onUnmounted, ref, watch } from 'vue'
import { Plus, RefreshCw, ShieldCheck } from 'lucide-vue-next'
import { apiRequest } from '../composables/useApi'
import { useAuth } from '../composables/useAuth'
import { useToast } from '../composables/useToast'
import { effectiveAccess, platformCapabilities } from '../composables/useCapabilities'
import BaseBadge from '../components/ui/BaseBadge.vue'
import BaseButton from '../components/ui/BaseButton.vue'
import BaseDataTable from '../components/ui/BaseDataTable.vue'
import BaseInput from '../components/ui/BaseInput.vue'
import BaseModal from '../components/ui/BaseModal.vue'
import BaseMultiSelect from '../components/ui/BaseMultiSelect.vue'
import BasePasswordInput from '../components/ui/BasePasswordInput.vue'
import BaseSearchableSelect from '../components/ui/BaseSearchableSelect.vue'

const toast = useToast()
const { state } = useAuth()
const items = ref([])
const total = ref(0)
const page = ref(1)
const pageSize = ref(15)
const search = ref('')
const status = ref('all')
const loading = ref(false)
const selected = ref(null)
const roles = ref([])
const tenants = ref([])
const actionOpen = ref(false)
const action = ref('role')
const saving = ref(false)
const form = ref({ email: '', roleCode: '', permissionCode: '', decision: 'inherit', tenantIds: [], currentPassword: '' })
const canManage = computed(() => Boolean(effectiveAccess.value?.platformSuperadmin && platformCapabilities.value['platform.pengguna.kelola']))
const isSelf = computed(() => selected.value?.user?.id === state.value?.user?.id)
const roleOptions = computed(() => roles.value.map((r) => ({ label: r.name, value: r.code })))
const tenantOptions = computed(() => tenants.value.map((t) => ({ label: t.name, value: t.id })))
const columns = [{ key: 'name', label: 'Nama', sortable: true }, { key: 'email', label: 'Email' }, { key: 'status', label: 'Status' }, { key: 'roleCount', label: 'Role aktif' }]
let timer

async function load() {
  loading.value = true
  try {
    const query = new URLSearchParams({ page: String(page.value), pageSize: String(pageSize.value), q: search.value })
    if (status.value !== 'all') query.set('status', status.value)
    const data = await apiRequest(`/platform/staff?${query}`)
    items.value = data.items; total.value = data.total
  } catch (error) { items.value = []; total.value = 0; toast.error(error.message, 'Gagal memuat staf') }
  finally { loading.value = false }
}
async function loadChoices() {
  try {
    const [roleData, tenantData] = await Promise.all([apiRequest('/platform/staff/roles'), apiRequest('/platform/tenants?pageSize=100')])
    roles.value = roleData; tenants.value = tenantData.items
  } catch (error) { toast.error(error.message, 'Gagal memuat pilihan') }
}
async function openUser(id) {
  try { selected.value = await apiRequest(`/platform/staff/${id}`) }
  catch (error) { toast.error(error.message, 'Gagal memuat hak akses') }
}
function openAction(type, payload = {}) {
  action.value = type
  form.value = { email: payload.email || '', roleCode: payload.roleCode || '', permissionCode: payload.permissionCode || '', decision: payload.decision || 'inherit', tenantIds: [...(payload.tenantIds || [])], currentPassword: '' }
  actionOpen.value = true
}
async function save() {
  if (saving.value || !form.value.currentPassword) return
  saving.value = true
  try {
    const id = selected.value?.user?.id
    if (action.value === 'role') await apiRequest('/platform/staff/roles', { method: 'POST', body: JSON.stringify({ email: form.value.email, roleCode: form.value.roleCode, currentPassword: form.value.currentPassword }) })
    if (action.value === 'revoke') await apiRequest(`/platform/staff/${id}/roles/${form.value.roleCode}`, { method: 'DELETE', body: JSON.stringify({ currentPassword: form.value.currentPassword }) })
    if (action.value === 'permission') await apiRequest(`/platform/staff/${id}/permissions/${form.value.permissionCode}`, { method: 'PUT', body: JSON.stringify({ decision: form.value.decision, currentPassword: form.value.currentPassword }) })
    if (action.value === 'tenants') await apiRequest(`/platform/staff/${id}/tenants`, { method: 'PUT', body: JSON.stringify({ tenantIds: form.value.tenantIds, currentPassword: form.value.currentPassword }) })
    actionOpen.value = false
    toast.success('Akses staf berhasil diperbarui.')
    await load()
    if (id) await openUser(id)
  } catch (error) { toast.error(error.message, 'Gagal menyimpan akses staf') }
  finally { saving.value = false; form.value.currentPassword = '' }
}
onMounted(() => { load(); loadChoices() })
onUnmounted(() => clearTimeout(timer))
watch([page, pageSize], load)
watch(search, () => { clearTimeout(timer); timer = setTimeout(() => { if (page.value === 1) load(); else page.value = 1 }, 250) })
watch(status, () => { if (page.value === 1) load(); else page.value = 1 })
</script>

<template>
  <div class="h-full flex flex-col min-h-0 overflow-hidden gap-2.5 w-full max-w-7xl mx-auto select-none relative">
    <div class="flex items-center justify-between gap-3 pb-2 border-b border-neutral-200 dark:border-[var(--theme-border)] shrink-0"><div class="flex items-center gap-2"><ShieldCheck class="w-5 h-5 text-[var(--theme-primary)]" /><h1 class="text-lg font-bold text-neutral-900 dark:text-white">Pengguna Platform</h1><BaseBadge variant="neutral">{{ total }}</BaseBadge></div><div class="flex gap-2"><BaseButton variant="outline" size="sm" @click="load"><RefreshCw class="w-4 h-4" /></BaseButton><BaseButton v-if="canManage" size="sm" @click="openAction('role')"><Plus class="w-4 h-4" />Tambah staf</BaseButton></div></div>
    <div class="shrink-0 flex flex-col sm:flex-row gap-2"><BaseInput v-model="search" placeholder="Cari nama atau email staf" clearable class="flex-1" /><BaseSearchableSelect v-model="status" :options="[{label:'Semua status',value:'all'},{label:'Aktif',value:'active'},{label:'Nonaktif',value:'disabled'}]" :searchable="false" class="sm:w-44" /></div>
    <div class="grid grid-cols-2 sm:grid-cols-4 gap-2 shrink-0"><div v-for="label in ['Total staf', 'Role aktif', 'Penugasan tenant', 'Izin khusus']" :key="label" class="h-9 flex items-center justify-between px-2.5 rounded-xl border border-neutral-200 dark:border-[var(--theme-border)] bg-white dark:bg-[var(--theme-surface)]"><span class="text-[10px] uppercase font-bold text-neutral-500">{{ label }}</span><span class="text-xs font-bold">{{ label === 'Total staf' ? total : '—' }}</span></div></div>
    <BaseDataTable :columns="columns" :rows="items" :total-items="total" server-side :loading="loading" :current-page="page" :items-per-page="pageSize" empty-title="Belum ada staf platform" empty-description="Tambahkan staf menggunakan akun yang sudah terdaftar." @update:current-page="page = $event" @update:items-per-page="pageSize = $event" @row-click="openUser($event.id)">
      <template #cell-status="{ row }"><BaseBadge :variant="row.status === 'active' ? 'success' : 'neutral'">{{ row.status }}</BaseBadge></template>
      <template #actions="{ row }"><BaseButton variant="outline" size="sm" @click.stop="openUser(row.id)">Lihat akses</BaseButton></template>
    </BaseDataTable>
    <BaseModal :model-value="Boolean(selected)" title="Hak Akses Staf Platform" size="xl" @update:model-value="(open) => { if (!open) selected = null }">
      <div v-if="selected" class="space-y-5"><div><h2 class="font-bold text-neutral-900 dark:text-white">{{ selected.user.name }}</h2><p class="text-xs text-neutral-500">{{ selected.user.email }}</p></div>
        <section><h3 class="text-sm font-bold mb-2">Role platform</h3><div v-for="role in selected.roles" :key="role.code" class="flex items-center justify-between gap-2 border-b border-neutral-200 dark:border-[var(--theme-border)] py-2 text-xs"><span>{{ role.name }} · {{ role.validFrom }}</span><BaseButton v-if="canManage && !isSelf && role.code !== 'SUPERADMIN'" variant="outline" size="sm" @click="openAction('revoke', { roleCode: role.code })">Cabut</BaseButton></div><BaseButton v-if="canManage && !isSelf" variant="outline" size="sm" class="mt-2" @click="openAction('role', { email: selected.user.email })">Tambah role</BaseButton></section>
        <section><div class="flex justify-between items-center"><h3 class="text-sm font-bold">Tenant yang ditugaskan</h3><BaseButton v-if="canManage && !isSelf && !selected.superadmin" variant="outline" size="sm" @click="openAction('tenants', { tenantIds: selected.tenantIds })">Atur</BaseButton></div><p class="text-xs text-neutral-500 mt-1">{{ selected.superadmin ? 'Seluruh platform' : selected.tenantIds.length ? selected.tenantIds.map(id => tenants.find(t => t.id === id)?.name || id).join(', ') : 'Belum ada tenant yang ditugaskan' }}</p></section>
        <section><h3 class="text-sm font-bold mb-2">Permission per menu dan aksi</h3><div class="max-h-72 overflow-auto rounded-xl border border-neutral-200 dark:border-[var(--theme-border)]"><div v-for="(entry, code) in selected.permissions" :key="code" class="flex items-center justify-between gap-2 p-2.5 border-b border-neutral-200 dark:border-[var(--theme-border)] text-xs"><div><p class="font-semibold">{{ code }}</p><p class="text-[11px] text-neutral-500">{{ entry.override ? `Override: ${entry.override}` : entry.sources?.join(', ') || entry.reason }}</p></div><div class="flex gap-2 items-center"><BaseBadge :variant="entry.allowed ? 'success' : 'neutral'">{{ entry.allowed ? 'Diizinkan' : 'Ditolak' }}</BaseBadge><BaseButton v-if="canManage && !isSelf" variant="outline" size="sm" @click="openAction('permission', { permissionCode: code, decision: entry.override || 'inherit' })">Atur</BaseButton></div></div></div></section>
      </div>
    </BaseModal>
    <BaseModal v-model="actionOpen" :title="action === 'role' ? 'Berikan Role Platform' : action === 'revoke' ? 'Cabut Role' : action === 'tenants' ? 'Penugasan Tenant' : 'Atur Permission'">
      <div class="space-y-3">
        <template v-if="action === 'role'"><BaseInput v-model="form.email" label="Email akun terdaftar" type="email" /><BaseSearchableSelect v-model="form.roleCode" :options="roleOptions" placeholder="Pilih role" size="md" /></template>
        <p v-else-if="action === 'revoke'" class="text-sm">Cabut role {{ form.roleCode }} dari {{ selected?.user?.name }}?</p>
        <BaseMultiSelect v-else-if="action === 'tenants'" v-model="form.tenantIds" :options="tenantOptions" label="Tenant yang boleh diakses" placeholder="Pilih tenant" />
        <template v-else><p class="text-xs font-semibold">{{ form.permissionCode }}</p><BaseSearchableSelect v-model="form.decision" :options="[{label:'Ikuti Role',value:'inherit'},{label:'Izinkan',value:'allow'},{label:'Tolak',value:'deny'}]" :searchable="false" size="md" /></template>
        <BasePasswordInput v-model="form.currentPassword" label="Kata sandi Superadmin" required />
      </div>
      <template #footer><BaseButton variant="outline" size="sm" @click="actionOpen = false">Batal</BaseButton><BaseButton size="sm" :disabled="saving || !form.currentPassword || (action === 'role' && (!form.email || !form.roleCode))" @click="save">{{ saving ? 'Menyimpan...' : 'Simpan' }}</BaseButton></template>
    </BaseModal>
  </div>
</template>
