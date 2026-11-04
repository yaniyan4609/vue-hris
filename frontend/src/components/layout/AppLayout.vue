<script setup>
import { computed, onMounted, ref, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { Building2, LogOut, Mail, Moon, Sun } from 'lucide-vue-next'
import { useAuth, defaultPanelRoute } from '../../composables/useAuth'
import { useBackground } from '../../composables/useBackground'
import { useTheme } from '../../composables/useTheme'
import { useToast } from '../../composables/useToast'
import BaseButton from '../ui/BaseButton.vue'
import BaseInput from '../ui/BaseInput.vue'
import BasePasswordInput from '../ui/BasePasswordInput.vue'
import BaseCheckbox from '../ui/BaseCheckbox.vue'
import ToastContainer from '../ui/ToastContainer.vue'

const route = useRoute()
const router = useRouter()
const toast = useToast()
const { bgType, solidColor, imageUrl, initBackground } = useBackground()
const { isDark, toggleTheme } = useTheme()
const { state, selectedTenant, isAuthenticated, demoAccounts, restore, loadDemoAccounts, login, demoLogin, logout } = useAuth()

const ready = ref(false)
const busy = ref(false)
const loginForm = ref({ email: '', password: '', remember: false })
const panelRoute = computed(() => defaultPanelRoute(state.value))
const baseLayerStyle = computed(() => {
  if (bgType.value === 'solid') return { backgroundColor: solidColor.value }
  if (bgType.value === 'image') return { backgroundImage: `url(${imageUrl.value})`, backgroundSize: 'cover', backgroundPosition: 'center' }
  return {}
})

function allowedPath(path) {
  if (!state.value) return false
  if (state.value.platform) return path === '/platform'
  if (!selectedTenant.value) return path === '/pilih-perusahaan'
  if (path === '/admin') return selectedTenant.value.admin
  if (path === '/ess') return selectedTenant.value.ess
  return false
}

watch([state, () => route.path], () => {
  if (!ready.value) return
  if (isAuthenticated.value && !allowedPath(route.path)) router.replace(panelRoute.value)
  if (!isAuthenticated.value && route.path !== '/') router.replace('/')
})

onMounted(async () => {
  initBackground()
  try {
    await restore()
  } catch {
    toast.error('Tidak dapat menghubungi server. Coba muat ulang halaman.', 'Koneksi gagal')
  } finally {
    ready.value = true
  }
  if (isAuthenticated.value && !allowedPath(route.path)) await router.replace(panelRoute.value)
  await loadDemoAccounts()
})

async function submitLogin() {
  if (busy.value || !loginForm.value.email.trim() || !loginForm.value.password) return
  busy.value = true
  try {
    await login(loginForm.value.email, loginForm.value.password, loginForm.value.remember)
    loginForm.value.password = ''
    await router.replace(panelRoute.value)
    toast.success('Berhasil masuk.')
  } catch (error) {
    toast.error(error.message, 'Gagal masuk')
  } finally {
    busy.value = false
  }
}

async function submitDemo(accountId) {
  if (busy.value) return
  busy.value = true
  try {
    await demoLogin(accountId)
    loginForm.value.password = ''
    await router.replace(panelRoute.value)
    toast.success('Berhasil masuk ke akun demo.')
  } catch (error) {
    toast.error(error.message, 'Gagal masuk')
  } finally {
    busy.value = false
  }
}

async function submitLogout() {
  if (busy.value) return
  busy.value = true
  try {
    await logout()
    await router.replace('/')
    toast.info('Sesi telah berakhir.')
  } catch (error) {
    toast.error(error.message, 'Gagal keluar')
  } finally {
    busy.value = false
  }
}
</script>

<template>
  <div
    class="min-h-screen flex flex-col"
    :class="[
      bgType === 'grid' ? 'bg-grid-pattern' : '',
      bgType === 'dots' ? 'bg-dots-pattern' : '',
      bgType !== 'solid' && bgType !== 'image' ? 'bg-neutral-100 dark:bg-[var(--theme-bg)]' : '',
      'text-neutral-800 dark:text-[var(--theme-text-primary)]',
    ]"
    :style="baseLayerStyle"
  >
    <header class="shrink-0 flex items-center justify-between gap-3 px-4 sm:px-6 py-3">
      <img src="/logo-tulisan-digaji.png" alt="Digaji.id" class="h-7 w-auto dark:hidden" />
      <img src="/logo-tulisan-digaji-dark.png" alt="Digaji.id" class="h-7 w-auto hidden dark:block" />
      <div class="flex items-center gap-2">
        <span v-if="isAuthenticated" class="hidden sm:block text-xs text-neutral-500 dark:text-neutral-400 truncate max-w-56">{{ state.user.name }}</span>
        <BaseButton variant="outline" size="sm" :title="isDark ? 'Mode terang' : 'Mode gelap'" :aria-label="isDark ? 'Mode terang' : 'Mode gelap'" @click="toggleTheme($event)">
          <Sun v-if="isDark" class="w-4 h-4" />
          <Moon v-else class="w-4 h-4" />
        </BaseButton>
        <BaseButton v-if="isAuthenticated" variant="outline" size="sm" :disabled="busy" @click="submitLogout">
          <LogOut class="w-4 h-4" /> <span class="hidden sm:inline">Keluar</span>
        </BaseButton>
      </div>
    </header>

    <div v-if="!ready" class="flex-1 flex items-center justify-center text-sm text-neutral-500 dark:text-neutral-400">Memuat sesi...</div>

    <main v-else-if="!isAuthenticated" class="flex-1 flex items-center justify-center px-4 py-6 sm:py-10">
      <div class="w-full max-w-md rounded-3xl border border-neutral-200 dark:border-[var(--theme-border)] bg-white dark:bg-[var(--theme-layer-2)] shadow-xl p-6 sm:p-8">
        <div class="text-center mb-6">
          <img src="/logo-digaji.png" alt="" class="w-11 h-11 rounded-2xl mx-auto mb-3" />
          <h1 class="text-xl font-bold text-neutral-900 dark:text-white">Masuk ke Digaji.id</h1>
          <p class="text-xs text-neutral-500 dark:text-neutral-400 mt-1">Masukkan email dan kata sandi akun Anda.</p>
        </div>
        <form class="space-y-4" @submit.prevent="submitLogin">
          <BaseInput v-model="loginForm.email" type="email" label="Email" placeholder="nama@perusahaan.com" :leading-icon="Mail" required />
          <BasePasswordInput v-model="loginForm.password" label="Kata sandi" required />
          <BaseCheckbox v-model="loginForm.remember" size="sm">Simpan sesi di perangkat ini</BaseCheckbox>
          <BaseButton type="submit" size="lg" class="w-full" :disabled="busy || !loginForm.email || !loginForm.password">
            {{ busy ? 'Memproses...' : 'Masuk' }}
          </BaseButton>
        </form>
        <div v-if="demoAccounts.length" class="mt-6 pt-5 border-t border-neutral-200 dark:border-[var(--theme-border)]">
          <p class="text-xs font-semibold text-neutral-700 dark:text-neutral-300 mb-1">Masuk cepat — staging</p>
          <p class="text-[11px] text-neutral-500 dark:text-neutral-400 mb-3">Pilih akun demo untuk masuk tanpa mengetik kata sandi.</p>
          <div class="grid gap-2">
            <BaseButton v-for="account in demoAccounts" :key="account.id" variant="outline" size="sm" class="w-full justify-start" :disabled="busy" @click="submitDemo(account.id)">
              <Building2 class="w-4 h-4 shrink-0" />
              <span class="text-left truncate">{{ account.label }} · {{ account.email }}</span>
            </BaseButton>
          </div>
        </div>
      </div>
    </main>

    <main v-else class="flex-1 min-h-0 px-3 pb-3 sm:px-5 sm:pb-5">
      <div class="h-full min-h-[calc(100vh-5rem)] rounded-3xl border border-neutral-200 dark:border-[var(--theme-border)] bg-white dark:bg-[var(--theme-layer-2)] shadow-sm overflow-auto">
        <nav v-if="selectedTenant?.admin && selectedTenant?.ess" class="flex gap-2 px-4 sm:px-6 pt-4 text-xs">
          <BaseButton :variant="route.path === '/admin' ? 'primary' : 'outline'" size="sm" @click="router.push('/admin')">Admin</BaseButton>
          <BaseButton :variant="route.path === '/ess' ? 'primary' : 'outline'" size="sm" @click="router.push('/ess')">ESS</BaseButton>
        </nav>
        <slot />
      </div>
    </main>
    <ToastContainer />
  </div>
</template>
