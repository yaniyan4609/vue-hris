<script setup>
import { computed, ref, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { Activity, Building2, CalendarDays, Clock3, FileBarChart, RefreshCw, Users, Wallet } from 'lucide-vue-next'
import { apiRequest } from '../composables/useApi'
import { useAuth } from '../composables/useAuth'
import { useToast } from '../composables/useToast'
import { adminCapabilities, platformCapabilities } from '../composables/useCapabilities'
import { visibleAdminMenus, visiblePlatformMenus } from '../data/menuData'
import { activeEntitlements } from '../composables/useCapabilities'
import BaseBadge from '../components/ui/BaseBadge.vue'
import BaseButton from '../components/ui/BaseButton.vue'
import BaseEmptyState from '../components/ui/BaseEmptyState.vue'
import BaseStatCard from '../components/ui/BaseStatCard.vue'

const route = useRoute()
const router = useRouter()
const toast = useToast()
const { state, selectedTenant } = useAuth()
const platform = computed(() => route.path.startsWith('/platform'))
const loading = ref(false)
const error = ref(false)
const counts = ref({})
const menus = computed(() => platform.value ? visiblePlatformMenus(platformCapabilities.value) : visibleAdminMenus(adminCapabilities.value, activeEntitlements.value))
const shortcuts = computed(() => menus.value.flatMap(group => group.submenus).filter(page => page.kind !== 'dashboard').slice(0, 6))
const kpis = computed(() => platform.value ? [
  { title: 'Perusahaan', value: counts.value.tenants ?? 0, icon: Building2, tone: 'primary' },
  { title: 'Tenant Aktif', value: counts.value.active ?? 0, icon: Activity, tone: 'emerald' },
  { title: 'Trial', value: counts.value.trial ?? 0, icon: Clock3, tone: 'amber' },
  { title: 'Ditangguhkan', value: counts.value.suspended ?? 0, icon: Building2, tone: 'neutral' },
] : [
  { title: 'Karyawan', value: counts.value.employees ?? 0, unavailable: !adminCapabilities.value['karyawan.lihat'], subtitle: 'Sesuai scope Anda', icon: Users, tone: 'primary' },
  { title: 'Kantor', value: counts.value.offices ?? 0, unavailable: !adminCapabilities.value['organisasi.lihat'], subtitle: 'Sesuai izin Anda', icon: Building2, tone: 'neutral' },
  { title: 'Wilayah', value: counts.value.regions ?? 0, unavailable: !adminCapabilities.value['organisasi.lihat'], subtitle: 'Sesuai izin Anda', icon: Activity, tone: 'neutral' },
  { title: 'Pengajuan Menunggu', value: 0, unavailable: true, subtitle: 'Data belum tersedia', icon: Clock3, tone: 'amber' },
])
let version = 0
async function load() {
  if (!state.value?.user || !['/admin', '/platform'].includes(route.path)) return
  const current = ++version
  loading.value = true; error.value = false
  try {
    if (platform.value) {
      const [all, active, trial, suspended] = await Promise.all([
        apiRequest('/platform/tenants?pageSize=1'),
        apiRequest('/platform/tenants?pageSize=1&status=active'),
        apiRequest('/platform/tenants?pageSize=1&status=trial'),
        apiRequest('/platform/tenants?pageSize=1&status=suspended'),
      ])
      if (current === version) counts.value = { tenants: all.total, active: active.total, trial: trial.total, suspended: suspended.total }
    } else {
      const allowed = adminCapabilities.value
      const calls = [allowed['karyawan.lihat'] ? apiRequest('/admin/employees?pageSize=1') : Promise.resolve(null), allowed['organisasi.lihat'] ? apiRequest('/admin/organization/offices?pageSize=1') : Promise.resolve(null), allowed['organisasi.lihat'] ? apiRequest('/admin/organization/regions?pageSize=1') : Promise.resolve(null)]
      const [employees, offices, regions] = await Promise.all(calls)
      if (current === version) counts.value = { employees: employees?.total ?? 0, offices: offices?.total ?? 0, regions: regions?.total ?? 0 }
    }
  } catch (cause) { if (current === version) { error.value = true; toast.error(cause.message, 'Gagal memuat dashboard') } }
  finally { if (current === version) loading.value = false }
}
watch([() => route.path, () => state.value?.user?.id, adminCapabilities, platformCapabilities], load, { immediate: true })
</script>

<template>
  <div v-if="state" class="space-y-4 sm:space-y-5 w-full max-w-7xl mx-auto select-none pb-8">
    <div class="flex flex-wrap items-start justify-between gap-3"><div><div class="flex items-center gap-2"><h1 class="text-lg sm:text-xl font-bold text-neutral-900 dark:text-white">{{ platform ? 'Dashboard SaaS' : 'Dashboard Admin' }}</h1><BaseBadge variant="info" size="sm">Hari ini</BaseBadge></div><p class="text-xs text-neutral-500 dark:text-neutral-400 mt-1">{{ platform ? 'Ringkasan operasional platform Digaji.id.' : selectedTenant?.name }}</p></div><BaseButton variant="outline" size="sm" :disabled="loading" @click="load"><RefreshCw class="w-4 h-4" />Muat ulang</BaseButton></div>
    <div v-if="!error" class="grid grid-cols-2 lg:grid-cols-4 gap-2.5"><BaseStatCard v-for="kpi in kpis" :key="kpi.title" :title="kpi.title" :value="kpi.value" :icon="kpi.icon" :tone="kpi.tone" :subtitle="kpi.subtitle" :unavailable="kpi.unavailable" :loading="loading" /></div>
    <BaseEmptyState v-else title="Ringkasan belum tersedia" description="Coba muat ulang dashboard." compact />
    <div class="grid grid-cols-1 xl:grid-cols-[minmax(0,1.65fr)_minmax(280px,1fr)] gap-3">
      <section class="rounded-2xl border border-neutral-200 dark:border-[var(--theme-border)] bg-white dark:bg-[var(--theme-surface)] p-4 sm:p-5 min-h-64"><div class="flex items-center justify-between gap-2 border-b border-neutral-200 dark:border-[var(--theme-border)] pb-3"><div class="flex items-center gap-2"><Activity class="w-4 h-4 text-[var(--theme-primary)]" /><h2 class="text-sm font-bold text-neutral-900 dark:text-white">{{ platform ? 'Tren Perusahaan' : 'Tren Kehadiran' }}</h2></div><span class="text-[11px] text-neutral-500">Bulanan</span></div><div class="h-44 flex items-center justify-center"><BaseEmptyState :title="platform ? 'Belum ada tren perusahaan' : 'Belum ada data kehadiran'" description="Grafik akan tampil saat data periode tersedia." compact /></div></section>
      <section class="rounded-2xl border border-neutral-200 dark:border-[var(--theme-border)] bg-white dark:bg-[var(--theme-surface)] p-4 sm:p-5 min-h-64"><div class="flex items-center gap-2 border-b border-neutral-200 dark:border-[var(--theme-border)] pb-3"><FileBarChart class="w-4 h-4 text-[var(--theme-primary)]" /><h2 class="text-sm font-bold text-neutral-900 dark:text-white">{{ platform ? 'Tagihan & Langganan' : 'Pengajuan & Tindakan' }}</h2></div><div class="h-44 flex items-center justify-center"><BaseEmptyState title="Belum ada ringkasan" description="Angka dan daftar akan mengikuti izin serta cakupan data akun Anda." compact /></div></section>
    </div>
    <div class="grid grid-cols-1 xl:grid-cols-2 gap-3"><section class="rounded-2xl border border-neutral-200 dark:border-[var(--theme-border)] bg-white dark:bg-[var(--theme-surface)] p-4 sm:p-5"><div class="flex items-center gap-2 mb-3"><CalendarDays class="w-4 h-4 text-[var(--theme-primary)]" /><h2 class="text-sm font-bold text-neutral-900 dark:text-white">Aktivitas Terkini</h2></div><BaseEmptyState title="Belum ada aktivitas" description="Riwayat terbaru akan muncul di sini." compact /></section><section class="rounded-2xl border border-neutral-200 dark:border-[var(--theme-border)] bg-white dark:bg-[var(--theme-surface)] p-4 sm:p-5"><div class="flex items-center gap-2 mb-3"><Wallet class="w-4 h-4 text-[var(--theme-primary)]" /><h2 class="text-sm font-bold text-neutral-900 dark:text-white">Akses Cepat</h2></div><div class="grid grid-cols-2 sm:grid-cols-3 gap-2"><BaseButton v-for="item in shortcuts" :key="item.route" variant="outline" size="sm" class="justify-start truncate" @click="router.push(item.route)">{{ item.label }}</BaseButton><p v-if="!shortcuts.length" class="col-span-full text-xs text-neutral-500">Belum ada modul yang dapat diakses.</p></div></section></div>
  </div>
</template>
