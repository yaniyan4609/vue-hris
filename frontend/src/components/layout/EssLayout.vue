<script setup>
import { computed } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { Bell, Building2, Fingerprint, UserRound } from 'lucide-vue-next'
import { useAuth } from '../../composables/useAuth'
import { essMenus } from '../../data/menuData'
import { unreadNotifications } from '../../composables/useNotifications'
import BaseButton from '../ui/BaseButton.vue'

defineEmits(['logout'])
const route = useRoute()
const router = useRouter()
const { state, selectedTenant } = useAuth()
const firstName = computed(() => state.value?.user?.name?.split(' ')[0] || 'Karyawan')
function active(item) { return item.route === '/ess' ? route.path === '/ess' : route.path.startsWith(item.route) }
</script>

<template>
  <div class="h-full min-h-0 w-full max-w-3xl mx-auto flex flex-col bg-neutral-50 dark:bg-[var(--theme-layer-main)] shadow-xl overflow-hidden sm:rounded-3xl">
    <header class="shrink-0 min-h-32 bg-[var(--theme-primary)] text-[var(--theme-primary-text)] dark:bg-[var(--theme-layer-2)] dark:text-[var(--theme-text-primary)] px-5 pt-[max(env(safe-area-inset-top),1.25rem)] pb-10 relative overflow-hidden">
      <div class="absolute -right-12 -top-16 w-48 h-48 rounded-full border border-white/10 dark:border-[var(--theme-border)] pointer-events-none" />
      <div class="absolute right-5 -top-10 w-28 h-28 rounded-full border border-white/10 dark:border-[var(--theme-border)] pointer-events-none" />
      <div class="relative z-10 flex items-start justify-between gap-3">
        <div><p class="text-xs opacity-80">Selamat datang,</p><h1 class="text-lg font-bold leading-tight">{{ firstName }}</h1><p class="text-[11px] opacity-75 mt-1">{{ state?.tenants?.find((tenant) => tenant.id === state.selectedTenantId)?.name || 'Digaji.id' }}</p></div>
        <div class="flex gap-2">
          <BaseButton v-if="selectedTenant?.admin" variant="ghost" size="sm" class="!rounded-full !p-2 !text-[var(--theme-primary-text)] dark:!text-[var(--theme-text-primary)] !bg-white/15 dark:!bg-[var(--theme-surface-raised)]" title="Buka Admin Panel" aria-label="Buka Admin Panel" @click="router.push('/admin')"><Building2 class="w-4 h-4" /></BaseButton>
          <BaseButton variant="ghost" size="sm" class="relative !rounded-full !p-2 !text-[var(--theme-primary-text)] dark:!text-[var(--theme-text-primary)] !bg-white/15 dark:!bg-[var(--theme-surface-raised)]" title="Notifikasi" aria-label="Notifikasi" @click="router.push('/ess/notifikasi')"><Bell class="w-4 h-4" /><span v-if="unreadNotifications" class="absolute -top-1 -right-1 min-w-4 h-4 px-0.5 rounded-full bg-white text-[var(--theme-primary)] dark:bg-[var(--theme-primary)] dark:text-[var(--theme-primary-text)] text-[9px] font-bold flex items-center justify-center">{{ Math.min(unreadNotifications, 99) }}</span></BaseButton>
          <BaseButton variant="ghost" size="sm" class="!rounded-full !p-2 !text-[var(--theme-primary-text)] dark:!text-[var(--theme-text-primary)] !bg-white/15 dark:!bg-[var(--theme-surface-raised)]" title="Akun" aria-label="Akun" @click="router.push('/ess/lainnya')"><UserRound class="w-4 h-4" /></BaseButton>
        </div>
      </div>
    </header>

    <main class="flex-1 min-h-0 overflow-y-auto px-3 sm:px-5 pb-24 -mt-7 relative z-10"><slot /></main>
    <nav class="shrink-0 h-17 pb-[env(safe-area-inset-bottom)] bg-white dark:bg-[var(--theme-layer-2)] border-t border-neutral-200 dark:border-[var(--theme-border)] grid grid-cols-5 items-center px-1 relative z-20" aria-label="Navigasi ESS">
      <BaseButton v-for="item in essMenus" :key="item.id" variant="ghost" size="sm" class="!px-0 !py-1 !rounded-xl flex flex-col gap-0.5 min-w-0" :class="active(item) ? '!text-[var(--theme-primary)]' : ''" :aria-label="item.label" @click="router.push(item.route)">
        <span v-if="item.id === 'absence'" class="-mt-7 w-14 h-14 flex items-center justify-center rounded-full bg-[var(--theme-primary)] text-[var(--theme-primary-text)] dark:bg-[var(--theme-surface-raised)] dark:text-[var(--theme-primary)] border-4 border-white dark:border-[var(--theme-layer-2)] shadow-lg"><Fingerprint class="w-7 h-7" /></span>
        <component v-else :is="item.icon" class="w-4 h-4" />
        <span class="text-[10px] truncate">{{ item.label }}</span>
      </BaseButton>
    </nav>
  </div>
</template>
