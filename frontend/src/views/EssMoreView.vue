<script setup>
import { inject, onMounted, ref } from 'vue'
import { useRouter } from 'vue-router'
import { Building2, LogOut, Moon, Sun, UserRound } from 'lucide-vue-next'
import { apiRequest } from '../composables/useApi'
import { useAuth } from '../composables/useAuth'
import { useTheme } from '../composables/useTheme'
import { useToast } from '../composables/useToast'
import { useThemeColor } from '../composables/useThemeColor'
import BaseButton from '../components/ui/BaseButton.vue'

const router = useRouter()
const toast = useToast()
const { state, selectedTenant } = useAuth()
const { isDark, toggleTheme } = useTheme()
const { themeColor, setThemeColor } = useThemeColor()
const requestLogout = inject('requestLogout')
const profile = ref(null)
onMounted(async () => { try { profile.value = await apiRequest('/ess/me/employee') } catch (error) { toast.error(error.message, 'Gagal memuat profil') } })
</script>

<template>
  <div class="space-y-3 pb-4">
    <section class="rounded-2xl bg-white dark:bg-[var(--theme-surface)] border border-neutral-200 dark:border-[var(--theme-border)] p-4 shadow-sm">
      <div class="flex items-center gap-3"><div class="w-11 h-11 rounded-2xl bg-[var(--theme-primary-soft)] text-[var(--theme-primary)] flex items-center justify-center"><UserRound class="w-5 h-5" /></div><div class="min-w-0"><h1 class="text-sm font-bold text-neutral-900 dark:text-white">{{ state?.user?.name }}</h1><p class="text-xs text-neutral-500 dark:text-neutral-400 break-all">{{ state?.user?.email }}</p></div></div>
      <dl v-if="profile" class="grid grid-cols-2 gap-3 mt-5 text-xs"><div><dt class="text-neutral-500 dark:text-neutral-400">Nomor karyawan</dt><dd class="font-semibold mt-1">{{ profile.employeeNumber }}</dd></div><div><dt class="text-neutral-500 dark:text-neutral-400">Status</dt><dd class="font-semibold mt-1">{{ profile.status }}</dd></div><div><dt class="text-neutral-500 dark:text-neutral-400">Kantor</dt><dd class="font-semibold mt-1">{{ profile.officeName }}</dd></div><div><dt class="text-neutral-500 dark:text-neutral-400">Jabatan</dt><dd class="font-semibold mt-1">{{ profile.positionName }}</dd></div></dl>
    </section>
    <section class="rounded-2xl bg-white dark:bg-[var(--theme-surface)] border border-neutral-200 dark:border-[var(--theme-border)] p-4 shadow-sm space-y-3">
      <h2 class="text-xs font-bold text-neutral-900 dark:text-white">Tampilan</h2>
      <BaseButton variant="outline" size="sm" class="w-full" @click="toggleTheme($event)"><Sun v-if="isDark" class="w-4 h-4" /><Moon v-else class="w-4 h-4" />{{ isDark ? 'Mode terang' : 'Mode gelap' }}</BaseButton>
      <div class="grid grid-cols-4 gap-2"><BaseButton v-for="color in ['default', 'pink', 'blue', 'terracotta']" :key="color" :variant="themeColor === color ? 'primary' : 'outline'" size="sm" class="!px-1" @click="setThemeColor(color)">{{ color === 'default' ? 'Netral' : color === 'pink' ? 'Pink' : color === 'blue' ? 'Biru' : 'Terra' }}</BaseButton></div>
    </section>
    <section class="rounded-2xl bg-white dark:bg-[var(--theme-surface)] border border-neutral-200 dark:border-[var(--theme-border)] p-4 shadow-sm space-y-2">
      <BaseButton v-if="selectedTenant?.admin" variant="outline" size="sm" class="w-full" @click="router.push('/admin')"><Building2 class="w-4 h-4" />Buka Admin Panel</BaseButton>
      <BaseButton variant="outline" size="sm" class="w-full" @click="requestLogout()"><LogOut class="w-4 h-4" />Keluar sesi</BaseButton>
    </section>
  </div>
</template>
