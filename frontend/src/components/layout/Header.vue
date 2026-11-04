<script setup>
import { ref, onMounted, onUnmounted } from 'vue'
import {
  Menu,
  X,
  Bell,
  User,
  Sun,
  Moon,
  LogOut,
  Sliders,
  Search,
  CheckCheck,
  ShoppingBag,
  Cpu,
  Users,
  AlertTriangle,
  Clock,
  Sparkles,
} from 'lucide-vue-next'
import { useTheme } from '../../composables/useTheme'
import { useToast } from '../../composables/useToast'
import { useAuth } from '../../composables/useAuth'

const props = defineProps({
  mobileDrawerOpen: {
    type: Boolean,
    default: false,
  },
})

const emit = defineEmits(['open-search', 'toggle-sidebar'])

const { isDark, toggleTheme } = useTheme()
const { currentUser, logout, isAuthenticated } = useAuth()
const toast = useToast()

const userMenuOpen = ref(false)
const notificationOpen = ref(false)
const notificationTab = ref('semua') // 'semua' | 'order' | 'sistem'

// Notification Items Data
const notifications = ref([
  {
    id: 1,
    title: 'Order Baru Masuk: ORD-250127-0142',
    category: 'order',
    desc: 'Pelanggan Budi Santoso memesan Cuci Kiloan 5,0 kg.',
    time: '5 mnt lalu',
    unread: true,
    icon: ShoppingBag,
    color: 'emerald',
  },
  {
    id: 2,
    title: 'Peringatan Stok Deterjen Menipis',
    category: 'sistem',
    desc: 'Sisa stok Deterjen tinggal 5,0 kg. Segera buat PO restok.',
    time: '25 mnt lalu',
    unread: true,
    icon: AlertTriangle,
    color: 'amber',
  },
  {
    id: 3,
    title: 'Siklus Mesin M-02 Selesai',
    category: 'sistem',
    desc: 'Pencucian 12,5 kg durasi 48 menit telah tuntas.',
    time: '1 jam lalu',
    unread: true,
    icon: Cpu,
    color: 'blue',
  },
  {
    id: 4,
    title: 'Karyawan Baru Terdaftar',
    category: 'sistem',
    desc: 'Rizky Pratama telah ditambahkan ke sistem payroll.',
    time: '3 jam lalu',
    unread: false,
    icon: Users,
    color: 'purple',
  },
])

const unreadCount = () => notifications.value.filter((n) => n.unread).length

const markAllAsRead = () => {
  notifications.value.forEach((n) => (n.unread = false))
  toast.success('Semua notifikasi telah ditandai dibaca!')
}

const filteredNotifications = () => {
  if (notificationTab.value === 'semua') return notifications.value
  return notifications.value.filter((n) => n.category === notificationTab.value)
}

// Click outside handling for dropdowns
const headerRef = ref(null)
const handleClickOutside = (e) => {
  if (headerRef.value && !headerRef.value.contains(e.target)) {
    userMenuOpen.value = false
    notificationOpen.value = false
  }
}

onMounted(() => {
  document.addEventListener('click', handleClickOutside)
})

onUnmounted(() => {
  document.removeEventListener('click', handleClickOutside)
})
</script>

<template>
  <!-- Topbar on Dasar Pertama -->
  <header
    ref="headerRef"
    class="w-full px-2.5 sm:px-3 lg:px-4 py-2 sm:py-2.5 flex items-center justify-between select-none z-30"
  >
    <!-- Kiri: Mobile Sidebar Toggle + Logo | Digaji.id -->
    <div class="flex items-center gap-2 sm:gap-3">
      <!-- Tombol Toggle Sidebar Mobile (Pojok Kiri Atas) -->
      <button
        v-if="isAuthenticated"
        type="button"
        @click="emit('toggle-sidebar')"
        class="lg:hidden w-8.5 h-8.5 rounded-xl border border-neutral-300 dark:border-[var(--theme-border)] bg-white dark:bg-[var(--theme-surface-raised)] flex items-center justify-center text-neutral-700 dark:text-neutral-200 hover:bg-neutral-100 dark:hover:bg-[var(--theme-surface-muted)] transition cursor-pointer shadow-2xs active:scale-95 shrink-0"
        :class="mobileDrawerOpen ? 'ring-2 ring-[var(--theme-primary)] text-[var(--theme-primary)] border-[var(--theme-primary)]' : ''"
        title="Buka / Tutup Menu Sidebar"
        aria-label="Toggle Sidebar"
      >
        <X v-if="mobileDrawerOpen" class="w-4.5 h-4.5" />
        <Menu v-else class="w-4.5 h-4.5" />
      </button>

      <router-link
        to="/"
        class="group flex items-center transition-transform hover:opacity-90 py-0.5"
        title="Digaji.id"
      >
        <!-- Logo Tulisan Digaji.id (Light Mode) -->
        <img
          src="/logo-tulisan-digaji.png"
          alt="Digaji.id Logo"
          class="h-7 sm:h-7.5 w-auto object-contain dark:hidden transition-transform group-hover:scale-[1.02]"
        />
        <!-- Logo Tulisan Digaji.id (Dark Mode) -->
        <img
          src="/logo-tulisan-digaji-dark.png"
          alt="Digaji.id Logo"
          class="h-7 sm:h-7.5 w-auto object-contain hidden dark:block transition-transform group-hover:scale-[1.02]"
        />
      </router-link>
    </div>

    <!-- Tengah: Quick Command Palette Trigger (Ctrl + K) -->
    <div v-if="isAuthenticated" class="flex-1 max-w-xs md:max-w-sm mx-3 hidden sm:block">
      <button
        type="button"
        @click="emit('open-search')"
        class="w-full h-8 sm:h-8.5 px-3 rounded-xl border border-neutral-300/80 dark:border-[var(--theme-border)] bg-white/80 dark:bg-[var(--theme-surface-raised)]/80 backdrop-blur-xs flex items-center justify-between text-xs text-neutral-400 hover:border-neutral-400 dark:hover:border-neutral-600 hover:text-neutral-600 dark:hover:text-neutral-200 transition shadow-2xs cursor-pointer group"
      >
        <div class="flex items-center gap-2 truncate">
          <Search class="w-3.5 h-3.5 text-neutral-400 group-hover:text-neutral-700 dark:group-hover:text-neutral-200 transition-colors" />
          <span class="truncate">Cari data, menu, atau aksi...</span>
        </div>
        <kbd class="px-1.5 py-0.5 rounded text-[10px] font-mono text-neutral-400 bg-neutral-100 dark:bg-[var(--theme-surface-muted)] border border-neutral-200 dark:border-[var(--theme-border-strong)] shrink-0">
          Ctrl K
        </kbd>
      </button>
    </div>

    <!-- Kanan: Icon Search Mobile | Icon Tema | Icon Lonceng Popover | Profile -->
    <div class="flex items-center gap-2 sm:gap-2.5">
      <!-- Search Mobile Icon Button -->
      <button
        v-if="isAuthenticated"
        type="button"
        @click="emit('open-search')"
        class="sm:hidden w-8 h-8 rounded-xl border border-neutral-300 dark:border-[var(--theme-border)] bg-white dark:bg-[var(--theme-surface-raised)] flex items-center justify-center text-neutral-700 dark:text-neutral-200 hover:bg-neutral-100 dark:hover:bg-[var(--theme-surface-muted)] transition cursor-pointer shadow-2xs"
        title="Buka Command Palette (Cari)"
      >
        <Search class="w-4 h-4" />
      </button>

      <!-- 1. Single Icon Button Theme -->
      <button
        type="button"
        @click="toggleTheme($event)"
        class="w-8 h-8 sm:w-9 sm:h-9 rounded-xl border border-neutral-300 dark:border-[var(--theme-border)] bg-white dark:bg-[var(--theme-surface-raised)] flex items-center justify-center text-neutral-700 dark:text-neutral-200 hover:bg-neutral-100 dark:hover:bg-[var(--theme-surface-muted)] transition cursor-pointer shadow-2xs group"
        :title="isDark ? 'Beralih ke Mode Terang' : 'Beralih ke Mode Gelap'"
        aria-label="Toggle theme"
      >
        <Moon
          v-if="!isDark"
          class="w-4 h-4 text-neutral-700 group-hover:scale-110 transition-transform"
        />
        <Sun
          v-else
          class="w-4 h-4 text-amber-400 group-hover:scale-110 transition-transform"
        />
      </button>

      <!-- 2. Icon Lonceng Popover Notification Center -->
      <div v-if="isAuthenticated" class="relative">
        <button
          type="button"
          @click="notificationOpen = !notificationOpen; userMenuOpen = false"
          class="relative w-8 h-8 sm:w-9 sm:h-9 rounded-xl border border-neutral-300 dark:border-[var(--theme-border)] bg-white dark:bg-[var(--theme-surface-raised)] flex items-center justify-center text-neutral-700 dark:text-neutral-200 hover:bg-neutral-100 dark:hover:bg-[var(--theme-surface-muted)] transition cursor-pointer shadow-2xs"
          :class="notificationOpen ? 'ring-2 ring-[var(--theme-primary)]' : ''"
          title="Pusat Pemberitahuan"
          aria-label="Notifikasi"
        >
          <Bell class="w-4 h-4" />
          <!-- Pulsing dot jika ada notifikasi belum dibaca -->
          <span
            v-if="unreadCount() > 0"
            class="absolute top-1 right-1 w-2 h-2 rounded-full bg-[var(--theme-primary)] ring-2 ring-white dark:ring-[var(--theme-border)] animate-pulse"
          />
        </button>

        <!-- Notification Popover Dropdown Panel -->
        <Transition
          enter-active-class="transition duration-150 ease-out"
          enter-from-class="opacity-0 scale-95 -translate-y-1"
          enter-to-class="opacity-100 scale-100 translate-y-0"
          leave-active-class="transition duration-100 ease-in"
          leave-from-class="opacity-100 scale-100 translate-y-0"
          leave-to-class="opacity-0 scale-95 -translate-y-1"
        >
          <div
            v-if="notificationOpen"
            class="absolute right-0 mt-2 w-80 sm:w-96 rounded-2xl bg-white dark:bg-[var(--theme-surface)] border border-neutral-200 dark:border-[var(--theme-border)] shadow-2xl overflow-hidden z-50 flex flex-col max-h-[85vh]"
          >
            <!-- Header Notification -->
            <div class="p-3.5 border-b border-neutral-100 dark:border-[var(--theme-border)] flex items-center justify-between">
              <div class="flex items-center gap-2">
                <h3 class="text-xs font-bold text-neutral-900 dark:text-white">Pemberitahuan</h3>
                <span
                  v-if="unreadCount() > 0"
                  class="px-1.5 py-0.2 rounded-full text-[10px] font-bold bg-[var(--theme-primary-soft)] text-[var(--theme-primary)]"
                >
                  {{ unreadCount() }} Baru
                </span>
              </div>
              <button
                type="button"
                @click="markAllAsRead"
                class="text-[11px] font-medium text-[var(--theme-primary)] hover:underline flex items-center gap-1 cursor-pointer"
              >
                <CheckCheck class="w-3.5 h-3.5" />
                <span>Tandai Dibaca</span>
              </button>
            </div>

            <!-- Category Tabs Pill -->
            <div class="px-3 pt-2 pb-1.5 flex gap-1 border-b border-neutral-100 dark:border-[var(--theme-border)] bg-neutral-50/50 dark:bg-[var(--theme-surface-raised)] text-xs">
              <button
                v-for="t in [
                  { id: 'semua', label: 'Semua' },
                  { id: 'order', label: 'Order' },
                  { id: 'sistem', label: 'Sistem' }
                ]"
                :key="t.id"
                type="button"
                @click="notificationTab = t.id"
                class="px-2.5 py-1 rounded-lg text-[11px] font-semibold transition cursor-pointer"
                :class="
                  notificationTab === t.id
                    ? 'bg-white dark:bg-[var(--theme-surface-muted)] text-neutral-900 dark:text-white shadow-2xs'
                    : 'text-neutral-500 hover:text-neutral-800 dark:hover:text-neutral-200'
                "
              >
                {{ t.label }}
              </button>
            </div>

            <!-- Notification Items List -->
            <div class="overflow-y-auto divide-y divide-neutral-100 dark:divide-[var(--theme-border)] max-h-72">
              <div
                v-for="notif in filteredNotifications()"
                :key="notif.id"
                class="p-3 flex items-start gap-3 hover:bg-neutral-50 dark:hover:bg-[var(--theme-surface-raised)] transition cursor-pointer relative group"
                :class="notif.unread ? 'bg-neutral-50/30 dark:bg-[var(--theme-surface-raised)]/40' : ''"
              >
                <!-- Icon Box -->
                <div
                  class="w-7 h-7 rounded-xl flex items-center justify-center shrink-0 mt-0.5"
                  :class="
                    notif.color === 'emerald' ? 'bg-emerald-50 dark:bg-emerald-950/40 text-emerald-600' :
                    notif.color === 'amber' ? 'bg-amber-50 dark:bg-amber-950/40 text-amber-600' :
                    notif.color === 'blue' ? 'bg-blue-50 dark:bg-blue-950/40 text-blue-600' :
                    'bg-purple-50 dark:bg-purple-950/40 text-purple-600'
                  "
                >
                  <component :is="notif.icon" class="w-3.5 h-3.5" />
                </div>

                <div class="flex-1 min-w-0 text-xs">
                  <div class="flex items-center justify-between gap-1">
                    <span class="font-bold text-neutral-900 dark:text-white truncate">
                      {{ notif.title }}
                    </span>
                    <span
                      v-if="notif.unread"
                      class="w-2 h-2 rounded-full bg-[var(--theme-primary)] shrink-0"
                    />
                  </div>
                  <p class="text-[11px] text-neutral-500 dark:text-neutral-400 mt-0.5 leading-snug">
                    {{ notif.desc }}
                  </p>
                  <span class="text-[10px] text-neutral-400 font-mono mt-1 block">
                    {{ notif.time }}
                  </span>
                </div>
              </div>
            </div>

            <!-- Footer View All -->
            <div class="p-2 border-t border-neutral-100 dark:border-[var(--theme-border)] bg-neutral-50/70 dark:bg-[var(--theme-surface)] text-center">
              <router-link
                to="/template/notifikasi"
                @click="notificationOpen = false"
                class="text-[11px] font-semibold text-[var(--theme-primary)] hover:underline block py-1"
              >
                Pengaturan Preferensi Notifikasi &rarr;
              </router-link>
            </div>
          </div>
        </Transition>
      </div>

      <!-- 3. Profile Avatar & Dropdown -->
      <div v-if="isAuthenticated" class="relative">
        <button
          type="button"
          @click="userMenuOpen = !userMenuOpen; notificationOpen = false"
          class="w-8 h-8 sm:w-9 sm:h-9 rounded-full border-2 border-neutral-700 dark:border-neutral-300 bg-white dark:bg-[var(--theme-surface-raised)] flex items-center justify-center text-neutral-700 dark:text-neutral-200 shadow-2xs hover:scale-105 transition cursor-pointer"
          aria-label="Profil Pengguna"
        >
          <User class="w-4 h-4 sm:w-4.5 sm:h-4.5" />
        </button>

        <!-- Dropdown Menu Profil -->
        <Transition
          enter-active-class="transition duration-150 ease-out"
          enter-from-class="opacity-0 scale-95 -translate-y-1"
          enter-to-class="opacity-100 scale-100 translate-y-0"
          leave-active-class="transition duration-100 ease-in"
          leave-from-class="opacity-100 scale-100 translate-y-0"
          leave-to-class="opacity-0 scale-95 -translate-y-1"
        >
          <div
            v-if="userMenuOpen"
            @click="userMenuOpen = false"
            class="absolute right-0 mt-2 w-48 rounded-2xl bg-white dark:bg-[var(--theme-surface)] border border-neutral-200 dark:border-[var(--theme-border)] shadow-xl py-2 z-50 text-xs text-neutral-700 dark:text-neutral-300"
          >
            <div class="px-4 py-2 border-b border-neutral-100 dark:border-[var(--theme-border)]">
              <p class="font-bold text-neutral-900 dark:text-white">{{ currentUser.name }}</p>
              <p class="text-neutral-400 text-[11px] truncate">{{ currentUser.email }}</p>
            </div>
            <router-link
              to="/template/umum"
              class="flex items-center gap-2.5 px-4 py-2 hover:bg-neutral-50 dark:hover:bg-[var(--theme-surface-muted)] transition"
            >
              <Sliders class="w-3.5 h-3.5 text-neutral-400" />
              Buka Setting
            </router-link>
            <button
              type="button"
              @click="logout"
              class="w-full flex items-center gap-2.5 px-4 py-2 text-rose-600 dark:text-rose-400 hover:bg-rose-50 dark:hover:bg-rose-950/30 transition text-left cursor-pointer"
            >
              <LogOut class="w-3.5 h-3.5" />
              Keluar Sesi
            </button>
          </div>
        </Transition>
      </div>

      <!-- Badge Mode Tamu jika belum login -->
      <div
        v-if="!isAuthenticated"
        class="hidden sm:flex items-center gap-1.5 px-3 py-1 rounded-xl border border-neutral-300 dark:border-[var(--theme-border)] bg-white/60 dark:bg-[var(--theme-surface-raised)]/60 backdrop-blur-xs text-xs font-medium text-neutral-600 dark:text-neutral-400"
      >
        <div class="w-2 h-2 rounded-full bg-amber-500 animate-pulse" />
        <span>Mode Masuk</span>
      </div>

    </div>

  </header>
</template>
