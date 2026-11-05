<script setup>
import { inject, onMounted, ref, watch } from 'vue'
import { useRouter } from 'vue-router'
import { Building2, KeyRound, LogOut, Moon, ShieldCheck, Sun, UserRound } from 'lucide-vue-next'
import { apiRequest } from '../composables/useApi'
import { useAuth } from '../composables/useAuth'
import { useTheme } from '../composables/useTheme'
import { useToast } from '../composables/useToast'
import { useThemeColor } from '../composables/useThemeColor'
import BaseButton from '../components/ui/BaseButton.vue'
import BaseBadge from '../components/ui/BaseBadge.vue'
import BaseModal from '../components/ui/BaseModal.vue'
import BasePasswordInput from '../components/ui/BasePasswordInput.vue'
import ConfirmationModal from '../components/ui/ConfirmationModal.vue'

const router = useRouter()
const toast = useToast()
const { state, selectedTenant } = useAuth()
const { isDark, toggleTheme } = useTheme()
const { themeColor, setThemeColor } = useThemeColor()
const requestLogout = inject('requestLogout')
const profile = ref(null)
const sessions = ref([])
const changeOpen = ref(false)
const logoutAllOpen = ref(false)
const saving = ref(false)
const passwords = ref({ current: '', next: '', confirm: '' })
watch(changeOpen, (open) => {
  if (!open) passwords.value = { current: '', next: '', confirm: '' }
})
const dateTime = (value) => new Intl.DateTimeFormat('id-ID', { dateStyle: 'medium', timeStyle: 'short' }).format(new Date(value))
async function loadSessions() {
  try { sessions.value = await apiRequest('/auth/sessions') }
  catch (error) { toast.error(error.message, 'Gagal memuat sesi') }
}
onMounted(async () => {
  await Promise.all([
    apiRequest('/ess/me/employee').then((value) => { profile.value = value }).catch((error) => toast.error(error.message, 'Gagal memuat profil')),
    loadSessions(),
  ])
})
async function changePassword() {
  if (saving.value || !passwords.value.current || passwords.value.next.length < 8) return
  if (passwords.value.next !== passwords.value.confirm) {
    toast.error('Konfirmasi kata sandi baru tidak cocok.', 'Periksa formulir')
    return
  }
  saving.value = true
  try {
    await apiRequest('/auth/change-password', { method: 'POST', body: JSON.stringify({ currentPassword: passwords.value.current, newPassword: passwords.value.next }) })
    passwords.value = { current: '', next: '', confirm: '' }
    changeOpen.value = false
    toast.success('Kata sandi berubah. Sesi di perangkat lain telah dicabut.')
    await loadSessions()
  } catch (error) { toast.error(error.message, 'Gagal mengganti kata sandi') }
  finally { saving.value = false }
}
async function logoutAll() {
  if (saving.value) return
  saving.value = true
  try {
    await apiRequest('/auth/logout-all', { method: 'POST', body: '{}' })
    logoutAllOpen.value = false
    await requestLogout()
  } catch (error) { toast.error(error.message, 'Gagal keluar dari semua perangkat') }
  finally { saving.value = false }
}
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
      <div class="pt-1"><h2 class="text-xs font-bold text-neutral-900 dark:text-white">Keamanan akun</h2><p class="text-[11px] text-neutral-500 dark:text-neutral-400 mt-1">Perangkat yang masih memiliki sesi aktif.</p></div>
      <div v-for="session in sessions" :key="session.id" class="rounded-xl border border-neutral-200 dark:border-[var(--theme-border)] p-3 text-xs">
        <div class="flex items-center justify-between gap-2"><span class="font-semibold text-neutral-900 dark:text-white">{{ session.deviceLabel }}</span><BaseBadge v-if="session.current" variant="success">Perangkat ini</BaseBadge></div>
        <p class="text-[11px] text-neutral-500 dark:text-neutral-400 mt-1">Aktif {{ dateTime(session.lastSeenAt) }} · {{ session.remember ? 'Simpan sesi' : 'Sesi sementara' }}</p>
      </div>
      <BaseButton variant="outline" size="sm" class="w-full" @click="changeOpen = true"><KeyRound class="w-4 h-4" />Ganti kata sandi</BaseButton>
      <BaseButton variant="outline" size="sm" class="w-full" @click="logoutAllOpen = true"><ShieldCheck class="w-4 h-4" />Keluar dari semua perangkat</BaseButton>
      <BaseButton variant="outline" size="sm" class="w-full" @click="requestLogout()"><LogOut class="w-4 h-4" />Keluar sesi</BaseButton>
    </section>
    <BaseModal v-model="changeOpen" title="Ganti Kata Sandi" subtitle="Sesi di perangkat lain akan dicabut setelah perubahan berhasil.">
      <div class="space-y-3">
        <BasePasswordInput v-model="passwords.current" label="Kata sandi saat ini" required />
        <BasePasswordInput v-model="passwords.next" label="Kata sandi baru" hint="Minimal 8 karakter" required />
        <BasePasswordInput v-model="passwords.confirm" label="Ulangi kata sandi baru" required />
      </div>
      <template #footer><BaseButton variant="outline" size="sm" @click="changeOpen = false">Batal</BaseButton><BaseButton size="sm" :disabled="saving || !passwords.current || passwords.next.length < 8 || !passwords.confirm" @click="changePassword">{{ saving ? 'Menyimpan...' : 'Ganti kata sandi' }}</BaseButton></template>
    </BaseModal>
    <ConfirmationModal v-model="logoutAllOpen" type="warning" title="Keluar dari Semua Perangkat" message="Semua sesi akun ini, termasuk perangkat ini, akan berakhir." confirm-text="Ya, keluar semua" @confirm="logoutAll" />
  </div>
</template>
