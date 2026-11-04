<script setup>
import { ref, computed, watch, onMounted } from 'vue'
import { useRoute } from 'vue-router'
import { menuItems } from '../../data/menuData'
import { useBackground } from '../../composables/useBackground'
import { useAuth } from '../../composables/useAuth'
import { useToast } from '../../composables/useToast'
import Header from './Header.vue'
import PrimarySidebar from './PrimarySidebar.vue'
import SubmenuSidebar from './SubmenuSidebar.vue'
import ToastContainer from '../ui/ToastContainer.vue'
import BaseCommandPalette from '../ui/BaseCommandPalette.vue'
import BaseCheckbox from '../ui/BaseCheckbox.vue'
import {
  Mail,
  Lock,
  Eye,
  EyeOff,
  ChevronRight,
} from 'lucide-vue-next'

const route = useRoute()
const toast = useToast()
const { bgType, solidColor, imageUrl, initBackground } = useBackground()
const { isAuthenticated, isLoggingOut, login, finishLogout } = useAuth()

// Default desktop: sub menu tertutup (collapsed) sesuai preferensi pengguna
const submenuCollapsed = ref(true)
const mobileDrawerOpen = ref(false)
const isCommandPaletteOpen = ref(false)

// Auth Morphing Card State
// 'form' (Frame 1) | 'loading' (Frames 2-4) | 'expanding' | 'authenticated' | 'collapsing'
const authStage = ref(isAuthenticated.value ? 'authenticated' : 'form')
const isExpanded = ref(isAuthenticated.value)
const isTimelineActive = ref(false)
const isDashboardMounted = ref(isAuthenticated.value)
const showPassword = ref(false)

// Form State
const loginForm = ref({
  identifier: 'admin@digaji.id',
  password: 'password123',
  rememberMe: true,
})

onMounted(() => {
  initBackground()
  if (isAuthenticated.value) {
    authStage.value = 'authenticated'
    isExpanded.value = true
    isDashboardMounted.value = true
  }
})

// Listen for logout triggers from Header or Command Palette
watch(isLoggingOut, (loggingOut) => {
  if (loggingOut) {
    collapseToLogin()
  }
})

// Trigger Hardware-Accelerated Timeline Animation (Frames 2, 3, 4)
// Menggunakan CSS Transition murni untuk 0% CPU loop (super ringan di device low-end)
const startLoginFlow = (provider = 'Standard') => {
  if (authStage.value === 'loading' || authStage.value === 'expanding') return

  authStage.value = 'loading'
  isTimelineActive.value = false

  // Trigger CSS Hardware-Accelerated width transition pada frame berikutnya
  requestAnimationFrame(() => {
    requestAnimationFrame(() => {
      isTimelineActive.value = true
    })
  })

  // Setelah 1.6s (Frame 4 100% tercapai): jeda 200ms lalu lakukan morphing expansion
  setTimeout(() => {
    expandToDashboard(provider)
  }, 1800)
}

// Morphing Card Expansion dari card kecil ke full Dasar Kedua
const expandToDashboard = async (provider) => {
  authStage.value = 'expanding'
  isExpanded.value = true

  // OPTIMASI KRUSIAL LOW-END:
  // Tunda mounting konten dashboard berat (tabel & chart) sampai kartu mencapai 75% ekspansi (~480ms)
  // Ini mencegah browser melakukan Layout Thrashing / Reflow berulang di setiap frame pada CPU lemah!
  setTimeout(() => {
    isDashboardMounted.value = true
  }, 480)

  // Card transition selesai pada 700ms
  setTimeout(async () => {
    await login(loginForm.value.identifier)
    authStage.value = 'authenticated'
    toast.success(`Selamat datang kembali di Digaji.id! (${provider === 'Google' ? 'Google SSO' : 'Kredensial Sesi'})`)
  }, 700)
}

// Reverse morphing on Logout
const collapseToLogin = () => {
  authStage.value = 'collapsing'
  isExpanded.value = false
  isDashboardMounted.value = false // Unmount seketika agar saat mengecil tidak ada kalkulasi reflow
  isTimelineActive.value = false

  setTimeout(() => {
    finishLogout()
    authStage.value = 'form'
  }, 700)
}

const activeMenuId = computed(() => {
  if (route.path.startsWith('/template')) {
    return 'template'
  }
  return 'beranda'
})

const currentMenuItem = computed(() => {
  return menuItems.find((m) => m.id === activeMenuId.value) || menuItems[0]
})

const toggleSubmenu = () => {
  submenuCollapsed.value = !submenuCollapsed.value
}

// Mobile Menu Selection Logic
const handleMobileMenuSelect = (menuId) => {
  const item = menuItems.find((m) => m.id === menuId)
  if (!item?.submenus || item.submenus.length === 0) {
    mobileDrawerOpen.value = false
  }
}

// Background Style for "Dasar Pertama"
const baseLayerStyle = computed(() => {
  if (bgType.value === 'solid') {
    return { backgroundColor: solidColor.value }
  }
  if (bgType.value === 'image') {
    return {
      backgroundImage: `url(${imageUrl.value})`,
      backgroundSize: 'cover',
      backgroundPosition: 'center',
    }
  }
  return {}
})
</script>

<template>
  <!-- ======================================================== -->
  <!-- 1. DASAR PERTAMA (Base Layer 1 - Backdrop Luar)          -->
  <!-- Switchable: grid, dots, solid, image                      -->
  <!-- Pure Neutral Dark: #09090b (Zero Blue Tint)               -->
  <!-- ======================================================== -->
  <div
    class="min-h-screen h-screen overflow-hidden flex flex-col relative select-none"
    :class="[
      bgType === 'grid' ? 'bg-grid-pattern' : '',
      bgType === 'dots' ? 'bg-dots-pattern' : '',
      bgType !== 'solid' && bgType !== 'image' ? 'bg-neutral-100 dark:bg-[var(--theme-bg)] text-neutral-800 dark:text-[var(--theme-text-primary)]' : 'text-neutral-800 dark:text-[var(--theme-text-primary)]',
    ]"
    :style="baseLayerStyle"
  >
    <!-- Topbar: Logo | Digaji.id di kiri, Theme Icon | Profile di kanan -->
    <Header
      :mobile-drawer-open="mobileDrawerOpen"
      @toggle-sidebar="mobileDrawerOpen = !mobileDrawerOpen"
      @open-search="isCommandPaletteOpen = true"
    />

    <!-- ======================================================== -->
    <!-- 2. DASAR KEDUA (Layer 2 - Floating Card Morphing)        -->
    <!-- Keadaan 1 (Login Frame 1): Card Polished Centered         -->
    <!-- Keadaan 2 (Loading Frames 2-4): Circle + Timeline Btn     -->
    <!-- Keadaan 3 (Dashboard): Card Melebar Membesar Full Layout  -->
    <!-- ======================================================== -->
    <div
      class="flex-1 min-h-0 flex items-center justify-center overflow-hidden relative transition-all duration-700"
      :class="isExpanded ? 'p-0 sm:px-1.5 md:px-2 lg:px-2.5 sm:pb-2' : 'p-3 sm:p-4'"
    >
      <!-- THE DASAR KEDUA CARD (Hardware Layer Promoted with will-change & adaptive shadow) -->
      <div
        class="relative bg-white dark:bg-[var(--theme-layer-2)] transition-all duration-700 ease-[cubic-bezier(0.16,1,0.3,1)] overflow-hidden flex will-change-[width,height] transform-gpu"
        :class="[
          isExpanded
            ? 'w-full h-full max-w-full max-h-full rounded-t-[32px] sm:rounded-2xl lg:rounded-3xl border-t-2 sm:border-2 border-neutral-300 dark:border-[var(--theme-border)] border-x-0 sm:border-x-2 border-b-0 sm:border-b-2'
            : 'w-[calc(100vw-32px)] max-w-[390px] sm:max-w-[410px] h-[490px] sm:h-[510px] max-h-[92vh] rounded-3xl border-2 border-neutral-200 dark:border-[var(--theme-border)]',
          authStage === 'expanding' ? 'shadow-md' : 'shadow-2xl'
        ]"
      >

        <!-- ==================================================== -->
        <!-- VIEW A: WIREFRAME AUTH (FRAME 1 S/D 4)               -->
        <!-- Bersih, Rapi, Elegan, Kelas Enterprise (No AI Slop)   -->
        <!-- Diposisikan Absolute Inset-0 agar transisi morphing   -->
        <!-- tidak mengganggu layout dashboard di baliknya         -->
        <!-- ==================================================== -->
        <div
          v-if="authStage !== 'authenticated'"
          class="absolute inset-0 z-20 w-full h-full flex flex-col justify-between p-6 sm:p-7 bg-white dark:bg-[var(--theme-layer-2)] transition-all duration-400 ease-out select-none will-change-[opacity]"
          :class="authStage === 'expanding' ? 'opacity-0 scale-95 pointer-events-none' : 'opacity-100 scale-100'"
        >
          <!-- -------------------------------------------------- -->
          <!-- BAGIAN ATAS: FORM INPUT (FRAME 1) ATAU             -->
          <!-- CIRCLE LOADING ENTERPRISE (FRAMES 2, 3, 4)         -->
          <!-- -------------------------------------------------- -->
          <div class="flex-1 flex flex-col justify-center">
            <!-- STATE A: FORM INPUT (FRAME 1) -->
            <div
              v-if="authStage === 'form' || authStage === 'collapsing'"
              class="space-y-4 my-auto"
            >
              <!-- Header Title & Subtitle dengan Logo Brand -->
              <div class="text-center space-y-1">
                <div class="flex justify-center mb-1.5">
                  <img
                    src="/logo-digaji.png"
                    alt="Digaji.id"
                    class="w-10 h-10 rounded-2xl shadow-sm hover:scale-105 transition-transform"
                  />
                </div>
                <h1 class="text-xl font-bold tracking-tight text-neutral-900 dark:text-white">
                  Masuk ke Akun Anda
                </h1>
                <p class="text-xs text-neutral-500 dark:text-neutral-400">
                  Silakan masukkan kredensial akun Anda untuk melanjutkan
                </p>
              </div>

              <!-- Form Input Group -->
              <form @submit.prevent="startLoginFlow('Standard')" class="space-y-3.5 pt-1">
                <!-- Field 1: Username / email / no hp -->
                <div>
                  <label class="block text-xs font-semibold text-neutral-700 dark:text-neutral-300 mb-1.5">
                    Username / email / no hp
                  </label>
                  <div class="relative">
                    <div class="absolute inset-y-0 left-0 pl-3.5 flex items-center pointer-events-none text-neutral-400">
                      <Mail class="w-4 h-4" />
                    </div>
                    <input
                      type="text"
                      v-model="loginForm.identifier"
                      placeholder="admin@digaji.id / 08123456789"
                      class="w-full pl-10 pr-3.5 py-2.5 rounded-xl text-xs sm:text-sm border border-neutral-300 dark:border-[var(--theme-border)] bg-neutral-50/50 dark:bg-[var(--theme-surface-raised)] text-neutral-900 dark:text-neutral-100 placeholder-neutral-400 focus:outline-none focus:ring-2 focus:ring-[var(--theme-primary)] transition"
                      required
                    />
                  </div>
                </div>

                <!-- Field 2: Kata Sandi -->
                <div>
                  <div class="flex items-center justify-between mb-1.5">
                    <label class="block text-xs font-semibold text-neutral-700 dark:text-neutral-300">
                      Kata Sandi
                    </label>
                    <button
                      type="button"
                      @click="toast.info('Gunakan password default: password123')"
                      class="text-[11px] font-medium text-[var(--theme-primary)] hover:underline"
                      tabindex="-1"
                    >
                      Lupa kata sandi?
                    </button>
                  </div>
                  <div class="relative">
                    <div class="absolute inset-y-0 left-0 pl-3.5 flex items-center pointer-events-none text-neutral-400">
                      <Lock class="w-4 h-4" />
                    </div>
                    <input
                      :type="showPassword ? 'text' : 'password'"
                      v-model="loginForm.password"
                      placeholder="••••••••"
                      class="w-full pl-10 pr-10 py-2.5 rounded-xl text-xs sm:text-sm border border-neutral-300 dark:border-[var(--theme-border)] bg-neutral-50/50 dark:bg-[var(--theme-surface-raised)] text-neutral-900 dark:text-neutral-100 placeholder-neutral-400 focus:outline-none focus:ring-2 focus:ring-[var(--theme-primary)] transition"
                      required
                    />
                    <button
                      type="button"
                      @click="showPassword = !showPassword"
                      class="absolute inset-y-0 right-0 pr-3 flex items-center text-neutral-400 hover:text-neutral-600 dark:hover:text-neutral-200 transition cursor-pointer"
                      tabindex="-1"
                    >
                      <EyeOff v-if="showPassword" class="w-4 h-4" />
                      <Eye v-else class="w-4 h-4" />
                    </button>
                  </div>
                </div>

                <!-- Remember Me Checkbox -->
                <div class="flex items-center justify-between text-xs pt-0.5">
                  <BaseCheckbox
                    v-model="loginForm.rememberMe"
                    size="sm"
                  >
                    <span class="text-neutral-600 dark:text-neutral-400 text-[11px] font-normal">Ingat sesi saya di perangkat ini</span>
                  </BaseCheckbox>
                </div>
              </form>
            </div>

            <!-- STATE B: CIRCLE LOADING ENTERPRISE (FRAMES 2, 3, 4) -->
            <div
              v-else
              class="flex flex-col items-center justify-center my-auto transition-all duration-300 will-change-[transform,opacity]"
            >
              <!-- Geometris Presisi SVG Circular Loader (Rapi, Elegan, Kelas Enterprise) -->
              <div class="relative w-36 h-36 sm:w-40 sm:h-40 flex items-center justify-center">
                <svg class="w-full h-full -rotate-90 transform-gpu" viewBox="0 0 120 120">
                  <!-- Jalur Lingkaran Latar Belakang -->
                  <circle
                    cx="60"
                    cy="60"
                    r="50"
                    fill="none"
                    stroke="currentColor"
                    class="text-neutral-200 dark:text-[var(--theme-surface-muted)]"
                    stroke-width="2"
                  />
                  <!-- Busur Berputar Mengikuti Warna Theme Aktif -->
                  <circle
                    cx="60"
                    cy="60"
                    r="50"
                    fill="none"
                    stroke="var(--theme-primary)"
                    stroke-width="2.5"
                    stroke-linecap="round"
                    stroke-dasharray="314.16"
                    stroke-dashoffset="220"
                    class="animate-spin origin-center will-change-transform"
                    style="animation-duration: 1.4s;"
                  />
                </svg>

                <!-- Tipografi Monospace Centered "LOADING" Sesuai Wireframe -->
                <div class="absolute inset-0 flex flex-col items-center justify-center select-none pointer-events-none">
                  <span class="text-xs sm:text-sm font-bold tracking-[0.28em] text-neutral-800 dark:text-neutral-200 font-mono">
                    LOADING
                  </span>
                </div>
              </div>
            </div>
          </div>

          <!-- -------------------------------------------------- -->
          <!-- BAGIAN BAWAH: TOMBOL "MASUK" / TIMELINE            -->
          <!-- Letak sama persis, cuma dianimasikan dari kiri ke  -->
          <!-- kanan dengan warna mengikuti Theme Sistem!         -->
          <!-- -------------------------------------------------- -->
          <div class="pt-3 space-y-2.5">
            <!-- TOMBOL "MASUK" & TIMELINE PROGRESS (POSISI SAMA PERSIS) -->
            <div
              class="w-full h-11 rounded-xl relative overflow-hidden flex items-center justify-center select-none transition-all duration-150 shadow-xs"
              :class="[
                authStage === 'form' || authStage === 'collapsing'
                  ? 'bg-[var(--theme-primary)] text-[var(--theme-primary-text)] hover:opacity-95 cursor-pointer active:scale-[0.99]'
                  : 'border border-neutral-300 dark:border-[var(--theme-border-strong)] bg-neutral-100 dark:bg-[var(--theme-surface-raised)]'
              ]"
              @click="authStage === 'form' && startLoginFlow('Standard')"
            >
              <!-- Timeline fill bar bergerak dari kiri ke kanan (CSS Hardware-Accelerated: 0% CPU overhead) -->
              <div
                v-if="authStage !== 'form' && authStage !== 'collapsing'"
                class="absolute left-0 top-0 bottom-0 bg-[var(--theme-primary)] will-change-[width] transition-[width] ease-[cubic-bezier(0.25,1,0.5,1)]"
                :class="isTimelineActive ? 'w-full duration-[1600ms]' : 'w-0 duration-0'"
              />

              <!-- Label "Masuk" di tengah tombol persis wireframe -->
              <span
                class="relative z-10 font-semibold text-sm transition-colors duration-300"
                :class="[
                  authStage === 'form' || authStage === 'collapsing'
                    ? 'text-[var(--theme-primary-text)]'
                    : (isTimelineActive ? 'text-[var(--theme-primary-text)] delay-300' : 'text-neutral-800 dark:text-neutral-200')
                ]"
              >
                Masuk
              </span>
            </div>

            <!-- TOMBOL "GOOGLE" (Tampil di Frame 1, Sembunyi Saat Loading) -->
            <button
              v-if="authStage === 'form' || authStage === 'collapsing'"
              type="button"
              @click="startLoginFlow('Google')"
              class="w-full h-10 flex items-center justify-center gap-2.5 rounded-xl border border-neutral-300 dark:border-[var(--theme-border)] bg-white dark:bg-[var(--theme-surface-raised)] hover:bg-neutral-50 dark:hover:bg-[var(--theme-surface-muted)] text-neutral-700 dark:text-neutral-300 text-xs sm:text-sm font-medium transition cursor-pointer shadow-2xs active:scale-[0.99]"
            >
              <!-- Lingkaran icon persis wireframe -->
              <div class="w-4.5 h-4.5 rounded-full border border-neutral-200 dark:border-neutral-700 flex items-center justify-center bg-white dark:bg-[var(--theme-layer-2)] shrink-0">
                <svg class="w-3 h-3" viewBox="0 0 24 24">
                  <path fill="#4285F4" d="M22.56 12.25c0-.78-.07-1.53-.2-2.25H12v4.26h5.92c-.26 1.37-1.04 2.53-2.21 3.31v2.77h3.57c2.08-1.92 3.28-4.74 3.28-8.09z"/>
                  <path fill="#34A853" d="M12 23c2.97 0 5.46-.98 7.28-2.66l-3.57-2.77c-.98.66-2.23 1.06-3.71 1.06-2.86 0-5.29-1.93-6.16-4.53H2.18v2.84C3.99 20.53 7.7 23 12 23z"/>
                  <path fill="#FBBC05" d="M5.84 14.09c-.22-.66-.35-1.36-.35-2.09s.13-1.43.35-2.09V7.06H2.18C1.43 8.55 1 10.22 1 12s.43 3.45 1.18 4.94l2.85-2.22.81-.63z"/>
                  <path fill="#EA4335" d="M12 5.38c1.62 0 3.06.56 4.21 1.64l3.15-3.15C17.45 2.09 14.97 1 12 1 7.7 1 3.99 3.47 2.18 7.06l3.66 2.84c.87-2.6 3.3-4.52 6.16-4.52z"/>
                </svg>
              </div>
              <span>Google</span>
            </button>
          </div>
        </div>

        <!-- ==================================================== -->
        <!-- VIEW B: DASHBOARD CONTAINER (EXPANDED DASAR KEDUA)   -->
        <!-- Tampil saat isDashboardMounted === true               -->
        <!-- Lazy mounted untuk mencegah reflow storm di low-end  -->
        <!-- ==================================================== -->
        <div
          v-if="isDashboardMounted"
          class="flex-1 min-h-0 flex overflow-hidden w-full relative transition-opacity duration-300"
          :class="authStage === 'authenticated' ? 'opacity-100' : 'opacity-0'"
        >
          <!-- Kolom MENU (Primary Rail: Beranda & Template) -->
          <PrimarySidebar
            :items="menuItems"
            :active-id="activeMenuId"
            :submenu-collapsed="submenuCollapsed"
            @open-submenu="submenuCollapsed = false"
            class="hidden lg:flex"
          />

          <!-- Kolom SUBMENU (Muncul bila menu memiliki sub-items, e.g. Template) -->
          <SubmenuSidebar
            v-if="currentMenuItem.submenus && currentMenuItem.submenus.length > 0"
            :submenus="currentMenuItem.submenus"
            :menu-title="currentMenuItem.label"
            :collapsed="submenuCollapsed"
            @toggle-collapse="toggleSubmenu"
            class="hidden lg:flex"
          />

          <!-- Floating button icon ">" di antara garis pembatas menu dan sub menu -->
          <button
            v-if="submenuCollapsed && currentMenuItem.submenus && currentMenuItem.submenus.length > 0"
            type="button"
            @click="toggleSubmenu"
            class="hidden lg:flex absolute left-20 sm:left-22 top-7 -translate-x-1/2 z-30 w-6 h-6 rounded-full items-center justify-center bg-white dark:bg-[var(--theme-surface-raised)] border border-neutral-300 dark:border-[var(--theme-border)] shadow-sm text-neutral-600 dark:text-neutral-300 hover:text-neutral-900 dark:hover:text-white hover:bg-neutral-100 dark:hover:bg-[var(--theme-surface-muted)] transition-all hover:scale-110 cursor-pointer"
            title="Buka Submenu"
            aria-label="Buka Submenu"
          >
            <ChevronRight class="w-3.5 h-3.5" />
          </button>

          <!-- Area Content (Main Class Content Area - var(--theme-layer-main); Default dark: #0c0c0e) -->
          <main
            class="flex-1 min-h-0 flex flex-col p-3.5 sm:p-5 lg:p-6 bg-neutral-50/60 dark:bg-[var(--theme-layer-main)] text-neutral-800 dark:text-[var(--theme-text-primary)]"
            :class="route.meta.fixedHeight ? 'overflow-hidden' : 'overflow-y-auto'"
          >
            <slot />
          </main>
        </div>

      </div>
    </div>

    <!-- ======================================================== -->
    <!-- 1. MOBILE BACKDROP OVERLAY (FADE TRANSITION)             -->
    <!-- ======================================================== -->
    <Transition
      enter-active-class="transition-opacity duration-300 ease-out"
      enter-from-class="opacity-0"
      enter-to-class="opacity-100"
      leave-active-class="transition-opacity duration-250 ease-in"
      leave-from-class="opacity-100"
      leave-to-class="opacity-0"
    >
      <div
        v-if="mobileDrawerOpen && isAuthenticated"
        class="lg:hidden fixed inset-0 z-40 bg-black/60 backdrop-blur-xs"
        @click="mobileDrawerOpen = false"
      />
    </Transition>

    <!-- ======================================================== -->
    <!-- 2. MOBILE DRAWER PANEL (PERSIS WIREFRAME 1)              -->
    <!-- Mepet Atas-Bawah (inset-y-0 h-full) + Slide X Animasi    -->
    <!-- Geser ke Kanan untuk Buka, Geser ke Kiri untuk Menutup   -->
    <!-- ======================================================== -->
    <Transition
      enter-active-class="transition-transform duration-300 ease-[cubic-bezier(0.16,1,0.3,1)]"
      enter-from-class="-translate-x-full"
      enter-to-class="translate-x-0"
      leave-active-class="transition-transform duration-250 ease-[cubic-bezier(0.16,1,0.3,1)]"
      leave-from-class="translate-x-0"
      leave-to-class="-translate-x-full"
    >
      <div
        v-if="mobileDrawerOpen && isAuthenticated"
        class="lg:hidden fixed inset-y-0 left-0 z-50 flex shadow-2xl h-full rounded-r-[32px] sm:rounded-r-[36px] border-r-2 border-neutral-300 dark:border-[var(--theme-border-strong)] bg-white dark:bg-[var(--theme-layer-2)] select-none overflow-hidden max-w-[calc(100vw-24px)]"
      >
        <!-- Left Column: Capsule "MENU" Pill (Rounded Top & Bottom persis Wireframe 1) -->
        <PrimarySidebar
          :items="menuItems"
          :active-id="activeMenuId"
          capsule
          @select="handleMobileMenuSelect"
        />

        <!-- Right Column: "SUBMENU" Inside the Same Outer Rounded Card -->
        <SubmenuSidebar
          v-if="currentMenuItem.submenus && currentMenuItem.submenus.length > 0"
          :submenus="currentMenuItem.submenus"
          :menu-title="currentMenuItem.label"
          :collapsed="false"
          drawer-mode
          @toggle-collapse="mobileDrawerOpen = false"
          @navigate="mobileDrawerOpen = false"
        />
      </div>
    </Transition>

    <!-- Global Toast Component -->
    <ToastContainer />

    <!-- Global Command Palette (Ctrl + K) -->
    <BaseCommandPalette v-model="isCommandPaletteOpen" />
  </div>
</template>
