<script setup>
import { ref } from 'vue'
import { useRouter } from 'vue-router'
import { Building2 } from 'lucide-vue-next'
import { useAuth, defaultPanelRoute } from '../composables/useAuth'
import { useToast } from '../composables/useToast'
import BaseButton from '../components/ui/BaseButton.vue'

const router = useRouter()
const toast = useToast()
const { state, selectTenant } = useAuth()
const busy = ref(false)

async function choose(tenantId) {
  if (busy.value) return
  busy.value = true
  try {
    const authState = await selectTenant(tenantId)
    await router.replace(defaultPanelRoute(authState))
  } catch (error) {
    toast.error(error.message, 'Gagal memilih perusahaan')
  } finally {
    busy.value = false
  }
}
</script>

<template>
  <div class="space-y-4 sm:space-y-5 w-full max-w-7xl mx-auto select-none p-4 sm:p-6 pb-8">
    <div>
      <h1 class="text-lg sm:text-xl font-bold text-neutral-900 dark:text-white">Pilih perusahaan</h1>
      <p class="text-xs text-neutral-500 dark:text-neutral-400 mt-1">Pilih perusahaan yang ingin Anda akses.</p>
    </div>
    <div v-if="state?.tenants?.length" class="grid gap-3 sm:grid-cols-2 lg:grid-cols-3">
      <div v-for="tenant in state.tenants" :key="tenant.id" class="rounded-2xl border border-neutral-200 dark:border-[var(--theme-border)] bg-white dark:bg-[var(--theme-surface)] p-4 shadow-2xs">
        <div class="flex gap-3 items-start mb-4">
          <Building2 class="w-5 h-5 text-[var(--theme-primary)] shrink-0" />
          <div class="min-w-0">
            <p class="text-sm font-semibold text-neutral-900 dark:text-white">{{ tenant.name }}</p>
            <p class="text-xs text-neutral-500 dark:text-neutral-400">{{ tenant.admin ? 'Admin Tenant' : 'Karyawan ESS' }}</p>
          </div>
        </div>
        <BaseButton size="sm" :disabled="busy" @click="choose(tenant.id)">Pilih perusahaan</BaseButton>
      </div>
    </div>
    <p v-else class="text-sm text-neutral-500 dark:text-neutral-400">Akun ini belum memiliki perusahaan aktif.</p>
  </div>
</template>
