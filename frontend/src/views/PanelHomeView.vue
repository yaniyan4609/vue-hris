<script setup>
import { computed, ref, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { Building2, MapPinned, Users } from 'lucide-vue-next'
import { apiRequest } from '../composables/useApi'
import { useAuth } from '../composables/useAuth'
import { useToast } from '../composables/useToast'
import BaseStatCard from '../components/ui/BaseStatCard.vue'
import BaseButton from '../components/ui/BaseButton.vue'
import BaseEmptyState from '../components/ui/BaseEmptyState.vue'

const route = useRoute()
const router = useRouter()
const toast = useToast()
const { state, selectedTenant } = useAuth()
const loading = ref(false)
const error = ref(false)
const counts = ref({ tenants: 0, employees: 0, offices: 0, regions: 0 })
const platform = computed(() => route.path.startsWith('/platform'))
async function load() {
  if (!state.value?.user || route.path === '/') return
  loading.value = true; error.value = false
  try {
    if (platform.value) {
      const tenants = await apiRequest('/platform/tenants')
      counts.value.tenants = tenants.total
    } else {
      const [employees, offices, regions] = await Promise.all([
        apiRequest('/admin/employees'), apiRequest('/admin/organization/offices'), apiRequest('/admin/organization/regions'),
      ])
      counts.value = { employees: employees.total, offices: offices.total, regions: regions.total }
    }
  } catch (cause) { error.value = true; toast.error(cause.message, 'Gagal memuat ringkasan') }
  finally { loading.value = false }
}
watch([() => route.path, () => state.value?.user?.id], load, { immediate: true })
</script>

<template>
  <div v-if="state" class="space-y-4 sm:space-y-5 w-full max-w-7xl mx-auto select-none pb-8">
    <div><h1 class="text-lg sm:text-xl font-bold text-neutral-900 dark:text-white">{{ platform ? 'Dashboard SaaS' : 'Dashboard Admin' }}</h1><p class="text-xs text-neutral-500 dark:text-neutral-400 mt-1">{{ platform ? 'Ringkasan perusahaan di platform Digaji.id.' : selectedTenant?.name }}</p></div>
    <div v-if="!error" class="grid grid-cols-2 lg:grid-cols-4 gap-2.5">
      <BaseStatCard v-if="platform" title="Perusahaan" :value="counts.tenants" :icon="Building2" tone="primary" :loading="loading" />
      <template v-else><BaseStatCard title="Karyawan" :value="counts.employees" :icon="Users" tone="primary" :loading="loading" /><BaseStatCard title="Kantor" :value="counts.offices" :icon="Building2" tone="neutral" :loading="loading" /><BaseStatCard title="Wilayah" :value="counts.regions" :icon="MapPinned" tone="neutral" :loading="loading" /></template>
    </div>
    <BaseEmptyState v-else title="Ringkasan belum tersedia" description="Coba muat ulang halaman untuk menampilkan data terbaru." compact />
    <div class="rounded-2xl bg-white dark:bg-[var(--theme-surface)] border border-neutral-200 dark:border-[var(--theme-border)] p-4 sm:p-5 shadow-2xs"><h2 class="text-sm font-semibold text-neutral-900 dark:text-white">Akses cepat</h2><p class="text-xs text-neutral-500 dark:text-neutral-400 mt-1">Buka data yang sudah tersedia.</p><div class="flex flex-wrap gap-2 mt-4"><BaseButton v-if="platform" variant="outline" size="sm" @click="router.push('/platform/tenants')"><Building2 class="w-4 h-4" />Perusahaan</BaseButton><template v-else><BaseButton variant="outline" size="sm" @click="router.push('/admin/employees')"><Users class="w-4 h-4" />Karyawan</BaseButton><BaseButton variant="outline" size="sm" @click="router.push('/admin/organization/regions')"><MapPinned class="w-4 h-4" />Organisasi</BaseButton></template></div></div>
  </div>
</template>
