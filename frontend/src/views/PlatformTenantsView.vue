<script setup>
import { onMounted, onUnmounted, ref, watch } from 'vue'
import { Building2, RefreshCw } from 'lucide-vue-next'
import { apiRequest } from '../composables/useApi'
import { useToast } from '../composables/useToast'
import BaseButton from '../components/ui/BaseButton.vue'
import BaseBadge from '../components/ui/BaseBadge.vue'
import BaseEmptyState from '../components/ui/BaseEmptyState.vue'
import BaseInput from '../components/ui/BaseInput.vue'
import BasePagination from '../components/ui/BasePagination.vue'

const toast = useToast()
const tenants = ref([])
const total = ref(0)
const page = ref(1)
const pageSize = ref(15)
const search = ref('')
const loading = ref(false)
async function load() {
  loading.value = true
  try {
    const query = new URLSearchParams({ page: String(page.value), pageSize: String(pageSize.value), q: search.value })
    const result = await apiRequest(`/platform/tenants?${query}`)
    tenants.value = result.items
    total.value = result.total
  } catch (error) { tenants.value = []; total.value = 0; toast.error(error.message, 'Gagal memuat perusahaan') }
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
</script>

<template>
  <div class="h-full flex flex-col min-h-0 overflow-hidden gap-2.5 w-full max-w-7xl mx-auto select-none relative">
    <div class="shrink-0 flex items-center justify-between gap-3"><div class="flex items-center gap-2.5"><div class="w-8 h-8 rounded-xl bg-[var(--theme-primary-soft)] text-[var(--theme-primary)] flex items-center justify-center"><Building2 class="w-4 h-4" /></div><div><h1 class="text-lg sm:text-xl font-bold text-neutral-900 dark:text-white">Perusahaan</h1><p class="text-xs text-neutral-500 dark:text-neutral-400">Tenant platform</p></div><BaseBadge variant="neutral">{{ total }}</BaseBadge></div><BaseButton variant="outline" size="sm" title="Muat ulang" aria-label="Muat ulang" :disabled="loading" @click="load"><RefreshCw class="w-4 h-4" /></BaseButton></div>
    <div class="shrink-0 rounded-2xl border border-neutral-200 dark:border-[var(--theme-border)] bg-white dark:bg-[var(--theme-surface)] p-3"><BaseInput v-model="search" size="sm" placeholder="Cari perusahaan atau slug" clearable /></div>
    <div class="hidden md:flex flex-1 min-h-0 flex-col rounded-2xl border border-neutral-200 dark:border-[var(--theme-border)] bg-white dark:bg-[var(--theme-surface)] overflow-hidden"><div class="flex-1 min-h-0 overflow-auto"><table class="w-full text-xs text-left"><thead class="sticky top-0 bg-neutral-50 dark:bg-[var(--theme-surface-raised)] border-b border-neutral-200 dark:border-[var(--theme-border)] text-neutral-500"><tr><th class="p-3">Perusahaan</th><th class="p-3">Slug</th><th class="p-3">Status</th></tr></thead><tbody><tr v-for="tenant in tenants" :key="tenant.id" class="border-b border-neutral-100 dark:border-[var(--theme-border)]"><td class="p-3 font-semibold text-neutral-900 dark:text-white">{{ tenant.name }}</td><td class="p-3 font-mono">{{ tenant.slug }}</td><td class="p-3"><BaseBadge :variant="tenant.status === 'active' ? 'success' : 'warning'">{{ tenant.status }}</BaseBadge></td></tr></tbody></table><BaseEmptyState v-if="!loading && !tenants.length" title="Belum ada perusahaan" compact /></div><div class="shrink-0 border-t border-neutral-200 dark:border-[var(--theme-border)] p-2"><BasePagination v-model:current-page="page" v-model:items-per-page="pageSize" :total-items="total" :per-page-options="[15, 30, 50, 100]" /></div></div>
    <div class="md:hidden flex-1 min-h-0 flex flex-col overflow-hidden"><div class="flex-1 min-h-0 overflow-y-auto space-y-2.5"><div v-for="tenant in tenants" :key="tenant.id" class="rounded-2xl border border-neutral-200 dark:border-[var(--theme-border)] bg-white dark:bg-[var(--theme-surface)] p-3"><p class="text-xs font-semibold text-neutral-900 dark:text-white">{{ tenant.name }}</p><div class="flex items-center justify-between gap-2 mt-2"><span class="text-[11px] text-neutral-500">{{ tenant.slug }}</span><BaseBadge :variant="tenant.status === 'active' ? 'success' : 'warning'">{{ tenant.status }}</BaseBadge></div></div><BaseEmptyState v-if="!loading && !tenants.length" title="Belum ada perusahaan" compact /></div><div class="shrink-0 pt-2"><BasePagination v-model:current-page="page" v-model:items-per-page="pageSize" :total-items="total" :per-page-options="[15, 30, 50, 100]" compact /></div></div>
  </div>
</template>
