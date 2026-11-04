<script setup>
import { computed } from 'vue'
import { useRoute } from 'vue-router'
import { Building2, ShieldCheck, UserRound } from 'lucide-vue-next'
import { useAuth } from '../composables/useAuth'

const route = useRoute()
const { state, selectedTenant } = useAuth()
const title = computed(() => route.path === '/platform' ? 'Panel Superadmin' : route.path === '/admin' ? 'Panel Admin Tenant' : 'Portal Karyawan')
const description = computed(() => route.path === '/platform'
  ? 'Kelola perusahaan dan akses platform dari sini.'
  : route.path === '/admin'
    ? 'Kelola data perusahaan dan karyawan dari sini.'
    : 'Lihat informasi kepegawaian Anda di sini.')
const roleIcon = computed(() => route.path === '/platform' ? ShieldCheck : route.path === '/admin' ? Building2 : UserRound)
</script>

<template>
  <div v-if="state" class="space-y-4 sm:space-y-5 w-full max-w-7xl mx-auto select-none p-4 sm:p-6 pb-8">
    <div>
      <h1 class="text-lg sm:text-xl font-bold text-neutral-900 dark:text-white">{{ title }}</h1>
      <p class="text-xs text-neutral-500 dark:text-neutral-400 mt-1">{{ description }}</p>
    </div>
    <div class="rounded-2xl border border-neutral-200 dark:border-[var(--theme-border)] bg-white dark:bg-[var(--theme-surface)] p-4 sm:p-5 shadow-2xs">
      <div class="flex items-start gap-3">
        <div class="rounded-xl bg-[var(--theme-primary-soft)] text-[var(--theme-primary)] p-2.5"><component :is="roleIcon" class="w-5 h-5" /></div>
        <div class="min-w-0">
          <p class="text-sm font-semibold text-neutral-900 dark:text-white">{{ state.user.name }}</p>
          <p class="text-xs text-neutral-500 dark:text-neutral-400 break-all">{{ state.user.email }}</p>
          <p class="text-xs text-neutral-600 dark:text-neutral-300 mt-2">{{ selectedTenant?.name || 'Platform Digaji.id' }}</p>
        </div>
      </div>
    </div>
  </div>
</template>
