<script setup>
import { computed, nextTick, onMounted, onUnmounted, provide, ref, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { Bell, Building2, ChevronDown, LogOut, Mail, Menu, Moon, Sun, X } from 'lucide-vue-next'
import { useAuth, defaultPanelRoute } from '../../composables/useAuth'
import { useBackground } from '../../composables/useBackground'
import { useTheme } from '../../composables/useTheme'
import { useToast } from '../../composables/useToast'
import { startNotifications, stopNotifications, unreadNotifications } from '../../composables/useNotifications'
import BaseButton from '../ui/BaseButton.vue'
import BaseInput from '../ui/BaseInput.vue'
import BasePasswordInput from '../ui/BasePasswordInput.vue'
import BaseCheckbox from '../ui/BaseCheckbox.vue'
import ToastContainer from '../ui/ToastContainer.vue'
import AdminLayout from './AdminLayout.vue'
import EssLayout from './EssLayout.vue'

const route = useRoute()
const router = useRouter()
const toast = useToast()
const { bgType, solidColor, imageUrl, initBackground } = useBackground()
const { isDark, toggleTheme } = useTheme()
const { state, selectedTenant, isAuthenticated, demoAccounts, restore, loadDemoAccounts, login, demoLogin, logout, finishLogout } = useAuth()

const ready = ref(false)
const busy = ref(false)
const stage = ref('form') // form | loading | expanding | authenticated | collapsing
const expanded = ref(false)
const dashboardMounted = ref(false)
const timelineActive = ref(false)
const mobileDrawerOpen = ref(false)
const loginForm = ref({ email: '', password: '', remember: false })
const timers = new Set()
let lastSessionRefresh = 0
let sessionRefreshPromise
const isEssPanel = computed(() => route.path.startsWith('/ess'))
const isTenantChoice = computed(() => route.path === '/pilih-perusahaan')
const showTopbar = computed(() => stage.value !== 'authenticated' || !isEssPanel.value)
const panelRoute = computed(() => defaultPanelRoute(state.value))
const baseLayerStyle = computed(() => {
  if (bgType.value === 'solid') return { backgroundColor: solidColor.value }
  if (bgType.value === 'image') return { backgroundImage: `url(${imageUrl.value})`, backgroundSize: 'cover', backgroundPosition: 'center' }
  return {}
})

function later(callback, delay) {
  const timer = setTimeout(() => { timers.delete(timer); callback() }, delay)
  timers.add(timer)
}

function allowedPath(path) {
  if (!state.value) return false
  if (state.value.platform) return path === '/platform' || path.startsWith('/platform/')
  if (path === '/pilih-perusahaan') return true
  if (!selectedTenant.value) return false
  if (path === '/admin' || path.startsWith('/admin/')) return selectedTenant.value.admin
  if (path === '/ess' || path.startsWith('/ess/')) return selectedTenant.value.ess
  return false
}

watch([state, () => route.path], () => {
  if (!ready.value || stage.value !== 'authenticated') return
  if (isAuthenticated.value && !allowedPath(route.path)) router.replace(panelRoute.value)
  if (!isAuthenticated.value) {
    dashboardMounted.value = false
    expanded.value = false
    stage.value = 'form'
    if (route.path !== '/') router.replace('/')
  }
})

watch(() => [state.value?.user?.id, state.value?.selectedTenantId], ([userId, tenantId]) => {
  if (userId && tenantId) startNotifications(`${userId}:${tenantId}`)
  else stopNotifications()
}, { immediate: true })

function refreshSessionOnFocus() {
  if (!ready.value || !isAuthenticated.value || document.visibilityState !== 'visible') return
  if (sessionRefreshPromise || Date.now() - lastSessionRefresh < 3000) return
  lastSessionRefresh = Date.now()
  sessionRefreshPromise = restore().catch(() => {}).finally(() => { sessionRefreshPromise = null })
}

onMounted(async () => {
  window.addEventListener('focus', refreshSessionOnFocus)
  document.addEventListener('visibilitychange', refreshSessionOnFocus)
  initBackground()
  loginForm.value.remember = window.matchMedia('(display-mode: standalone)').matches
  try {
    await restore()
  } catch {
    toast.error('Tidak dapat menghubungi server. Coba muat ulang halaman.', 'Koneksi gagal')
  }
  if (isAuthenticated.value) {
    expanded.value = true
    dashboardMounted.value = true
    stage.value = 'authenticated'
    if (!allowedPath(route.path)) await router.replace(panelRoute.value)
  } else if (route.path !== '/') await router.replace('/')
  ready.value = true
  await loadDemoAccounts()
})
onUnmounted(() => {
  stopNotifications()
  window.removeEventListener('focus', refreshSessionOnFocus)
  document.removeEventListener('visibilitychange', refreshSessionOnFocus)
  for (const timer of timers) clearTimeout(timer)
  timers.clear()
})

async function morphIntoPanel(message) {
  stage.value = 'loading'
  timelineActive.value = false
  await nextTick()
  requestAnimationFrame(() => { timelineActive.value = true })
  later(async () => {
    stage.value = 'expanding'
    expanded.value = true
    await router.replace(panelRoute.value)
    later(() => { dashboardMounted.value = true }, 420)
    later(() => { stage.value = 'authenticated'; busy.value = false; toast.success(message) }, 720)
  }, 900)
}

async function submitLogin() {
  if (busy.value || !loginForm.value.email.trim() || !loginForm.value.password) return
  busy.value = true
  try {
    await login(loginForm.value.email, loginForm.value.password, loginForm.value.remember)
    loginForm.value.password = ''
    await morphIntoPanel('Berhasil masuk.')
  } catch (error) { busy.value = false; toast.error(error.message, 'Gagal masuk') }
}

async function submitDemo(accountId) {
  if (busy.value) return
  busy.value = true
  try {
    await demoLogin(accountId)
    loginForm.value.password = ''
    await morphIntoPanel('Berhasil masuk ke akun demo.')
  } catch (error) { busy.value = false; toast.error(error.message, 'Gagal masuk') }
}

async function submitLogout() {
  if (busy.value) return
  busy.value = true
  try {
    await logout()
    stopNotifications()
    mobileDrawerOpen.value = false
    stage.value = 'collapsing'
    dashboardMounted.value = false
    timelineActive.value = false
    expanded.value = false
    later(async () => {
      finishLogout()
      await router.replace('/')
      stage.value = 'form'
      busy.value = false
      toast.info('Sesi telah berakhir.')
    }, 720)
  } catch (error) { busy.value = false; toast.error(error.message, 'Gagal keluar') }
}
provide('requestLogout', submitLogout)
</script>

<template>
  <div
    class="h-dvh min-h-0 flex flex-col overflow-hidden"
    :class="[
      bgType === 'grid' ? 'bg-grid-pattern' : '',
      bgType === 'dots' ? 'bg-dots-pattern' : '',
      bgType !== 'solid' && bgType !== 'image' ? 'bg-neutral-100 dark:bg-[var(--theme-bg)]' : '',
      'text-neutral-800 dark:text-[var(--theme-text-primary)]',
    ]"
    :style="baseLayerStyle"
  >
    <header v-if="showTopbar" class="shrink-0 h-14 flex items-center justify-between gap-3 px-3 sm:px-5 relative z-30">
      <div class="flex items-center gap-2">
        <BaseButton v-if="stage === 'authenticated' && !isTenantChoice" variant="outline" size="sm" class="lg:hidden !p-2" :aria-label="mobileDrawerOpen ? 'Tutup menu' : 'Buka menu'" @click="mobileDrawerOpen = !mobileDrawerOpen"><X v-if="mobileDrawerOpen" class="w-4 h-4" /><Menu v-else class="w-4 h-4" /></BaseButton>
        <img src="/logo-tulisan-digaji.png" alt="Digaji.id" class="h-7 w-auto dark:hidden" />
        <img src="/logo-tulisan-digaji-dark.png" alt="Digaji.id" class="h-7 w-auto hidden dark:block" />
      </div>
      <div class="flex items-center gap-2 min-w-0">
        <span v-if="stage === 'authenticated'" class="hidden sm:block text-xs text-neutral-500 dark:text-neutral-400 truncate max-w-40">{{ state?.user?.name }}</span>
        <BaseButton v-if="stage === 'authenticated' && state?.tenants?.length > 1 && !state?.platform" variant="outline" size="sm" title="Pilih perusahaan" aria-label="Pilih perusahaan" @click="router.push('/pilih-perusahaan')"><Building2 class="w-4 h-4" /><ChevronDown class="w-3 h-3" /></BaseButton>
        <BaseButton v-if="stage === 'authenticated' && selectedTenant?.admin && selectedTenant?.ess" variant="outline" size="sm" :title="isEssPanel ? 'Buka Admin Panel' : 'Buka ESS'" @click="router.push(isEssPanel ? '/admin' : '/ess')">{{ isEssPanel ? 'Admin' : 'ESS' }}</BaseButton>
        <BaseButton v-if="stage === 'authenticated' && selectedTenant && !isEssPanel" variant="outline" size="sm" class="relative !p-2" title="Notifikasi" aria-label="Notifikasi" @click="router.push('/admin/communication/notifications')"><Bell class="w-4 h-4" /><span v-if="unreadNotifications" class="absolute -top-1 -right-1 min-w-4 h-4 px-0.5 rounded-full bg-[var(--theme-primary)] text-[var(--theme-primary-text)] text-[9px] font-bold flex items-center justify-center">{{ Math.min(unreadNotifications, 99) }}</span></BaseButton>
        <BaseButton variant="outline" size="sm" class="!p-2" :title="isDark ? 'Mode terang' : 'Mode gelap'" :aria-label="isDark ? 'Mode terang' : 'Mode gelap'" @click="toggleTheme($event)"><Sun v-if="isDark" class="w-4 h-4" /><Moon v-else class="w-4 h-4" /></BaseButton>
        <BaseButton v-if="stage === 'authenticated'" variant="outline" size="sm" :disabled="busy" title="Keluar sesi" aria-label="Keluar sesi" @click="submitLogout"><LogOut class="w-4 h-4" /><span class="hidden sm:inline">Keluar</span></BaseButton>
      </div>
    </header>

    <div v-if="!ready" class="flex-1 flex items-center justify-center text-sm text-neutral-500 dark:text-neutral-400">Memuat sesi...</div>
    <div v-else class="flex-1 min-h-0 flex items-center justify-center transition-all duration-700" :class="expanded ? (isEssPanel ? 'p-0' : 'p-0 sm:px-1.5 md:px-2 lg:px-2.5 sm:pb-2') : 'p-3 sm:p-4'">
      <div
        class="relative bg-white dark:bg-[var(--theme-layer-2)] flex overflow-hidden transform-gpu will-change-[width,height] transition-all duration-700 ease-[cubic-bezier(0.16,1,0.3,1)]"
        :class="expanded ? (isEssPanel ? 'w-full h-full rounded-none border-0' : 'w-full h-full rounded-t-[32px] sm:rounded-2xl lg:rounded-3xl border-t-2 sm:border-2 border-neutral-300 dark:border-[var(--theme-border)]') : 'w-full max-w-[410px] h-full max-h-[690px] rounded-3xl border-2 border-neutral-200 dark:border-[var(--theme-border)] shadow-2xl'"
      >
        <div v-if="stage !== 'authenticated'" class="absolute inset-0 z-20 flex flex-col p-6 sm:p-7 bg-white dark:bg-[var(--theme-layer-2)] transition-opacity duration-300" :class="stage === 'expanding' ? 'opacity-0 pointer-events-none' : 'opacity-100'">
          <div v-if="stage === 'form' || stage === 'collapsing'" class="flex-1 min-h-0 overflow-y-auto py-1">
            <div class="text-center mb-5"><img src="/logo-digaji.png" alt="" class="w-11 h-11 rounded-2xl mx-auto mb-2" /><h1 class="text-xl font-bold text-neutral-900 dark:text-white">Masuk ke Digaji.id</h1><p class="text-xs text-neutral-500 dark:text-neutral-400 mt-1">Masukkan email dan kata sandi akun Anda.</p></div>
            <form id="login-form" class="space-y-3.5" @submit.prevent="submitLogin">
              <BaseInput v-model="loginForm.email" type="email" label="Email" placeholder="nama@perusahaan.com" :leading-icon="Mail" required />
              <BasePasswordInput v-model="loginForm.password" label="Kata sandi" required />
              <BaseCheckbox v-model="loginForm.remember" size="sm" label="Simpan sesi di perangkat ini" description="Jangan aktifkan di komputer bersama." />
            </form>
            <div v-if="demoAccounts.length" class="mt-5 pt-4 border-t border-neutral-200 dark:border-[var(--theme-border)]"><p class="text-xs font-semibold text-neutral-700 dark:text-neutral-300">Masuk cepat — staging</p><p class="text-[11px] text-neutral-500 dark:text-neutral-400 mt-1 mb-3">Pilih akun demo untuk masuk tanpa mengetik kata sandi.</p><div class="grid gap-2"><BaseButton v-for="account in demoAccounts" :key="account.id" variant="outline" size="sm" class="w-full justify-start" :disabled="busy" @click="submitDemo(account.id)"><Building2 class="w-4 h-4 shrink-0" /><span class="text-left truncate">{{ account.label }} · {{ account.email }}</span></BaseButton></div></div>
          </div>
          <div v-else class="flex-1 flex flex-col items-center justify-center"><div class="relative w-36 h-36 flex items-center justify-center"><svg class="w-full h-full -rotate-90 animate-spin" style="animation-duration:1.4s" viewBox="0 0 120 120" aria-hidden="true"><circle cx="60" cy="60" r="50" fill="none" stroke="currentColor" class="text-neutral-200 dark:text-[var(--theme-surface-muted)]" stroke-width="2" /><circle cx="60" cy="60" r="50" fill="none" stroke="var(--theme-primary)" stroke-width="2.5" stroke-linecap="round" stroke-dasharray="220 94" /></svg><span class="absolute text-xs font-bold tracking-[0.28em] text-neutral-800 dark:text-neutral-200 font-mono">LOADING</span></div></div>
          <div class="shrink-0 pt-3">
            <BaseButton v-if="stage === 'form' || stage === 'collapsing'" size="lg" class="w-full h-11" :disabled="busy || !loginForm.email || !loginForm.password" @click="submitLogin">Masuk</BaseButton>
            <div v-else class="w-full h-11 rounded-xl relative overflow-hidden flex items-center justify-center border border-neutral-300 dark:border-[var(--theme-border-strong)] bg-neutral-100 dark:bg-[var(--theme-surface-raised)]"><div class="absolute inset-y-0 left-0 bg-[var(--theme-primary)] transition-[width] ease-[cubic-bezier(0.25,1,0.5,1)]" :class="timelineActive ? 'w-full duration-[850ms]' : 'w-0 duration-0'" /><span class="relative z-10 text-sm font-semibold" :class="timelineActive ? 'text-[var(--theme-primary-text)]' : 'text-neutral-700 dark:text-neutral-200'">Masuk</span></div>
          </div>
        </div>

        <div v-if="dashboardMounted" class="flex-1 min-h-0 w-full transition-opacity duration-300" :class="stage === 'authenticated' ? 'opacity-100' : 'opacity-0'">
          <EssLayout v-if="isEssPanel"><slot /></EssLayout>
          <main v-else-if="isTenantChoice" class="h-full overflow-y-auto p-4 sm:p-6 bg-neutral-50/60 dark:bg-[var(--theme-layer-main)]"><slot /></main>
          <AdminLayout v-else :mobile-drawer-open="mobileDrawerOpen" @close-drawer="mobileDrawerOpen = false"><slot /></AdminLayout>
        </div>
      </div>
    </div>
    <ToastContainer />
  </div>
</template>
