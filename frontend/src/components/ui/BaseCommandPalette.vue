<script setup>
import { ref, computed, watch, onMounted, onUnmounted, nextTick } from 'vue'
import { useRouter } from 'vue-router'
import { useTheme } from '../../composables/useTheme'
import { useThemeColor } from '../../composables/useThemeColor'
import { useToast } from '../../composables/useToast'
import {
  Search,
  ArrowRight,
  Sun,
  Moon,
  Palette,
  Users,
  Mountain,
  LayoutDashboard,
  Settings,
  Sparkles,
  Command,
  CornerDownLeft,
  X,
  FileText,
  PlusCircle,
  KanbanSquare,
  LogIn,
  LogOut,
} from 'lucide-vue-next'
import { useAuth } from '../../composables/useAuth'

const props = defineProps({
  modelValue: {
    type: Boolean,
    default: false,
  },
})

const emit = defineEmits(['update:modelValue', 'action'])

const router = useRouter()
const { isDark, toggleTheme } = useTheme()
const { setThemeColor } = useThemeColor()
const { logout, isAuthenticated } = useAuth()
const toast = useToast()

const searchQuery = ref('')
const selectedIndex = ref(0)
const inputRef = ref(null)

const close = () => {
  emit('update:modelValue', false)
  searchQuery.value = ''
  selectedIndex.value = 0
}

// Command Actions Data List
const commands = [
  // Halaman & Navigasi
  {
    id: 'nav-dashboard',
    title: 'Buka Beranda / Dashboard',
    category: 'Navigasi',
    icon: LayoutDashboard,
    badge: 'Halaman',
    action: () => router.push('/'),
  },
  {
    id: 'nav-pesanan-kanban',
    title: 'Buka Kanban Pipeline Pesanan',
    category: 'Navigasi',
    icon: KanbanSquare,
    badge: 'Halaman',
    action: () => router.push('/template/kanban'),
  },
  {
    id: 'nav-data-gunung',
    title: 'Buka Data Gunung Jawa',
    category: 'Navigasi',
    icon: Mountain,
    badge: 'Halaman',
    action: () => router.push('/template/data-1'),
  },
  {
    id: 'nav-data-karyawan',
    title: 'Buka Data Karyawan & Payroll',
    category: 'Navigasi',
    icon: Users,
    badge: 'Halaman',
    action: () => router.push('/template/data-2'),
  },
  {
    id: 'nav-setting',
    title: 'Buka Pengaturan Sistem',
    category: 'Navigasi',
    icon: Settings,
    badge: 'Halaman',
    action: () => router.push('/template/umum'),
  },
  {
    id: 'act-logout',
    title: 'Keluar Sesi / Uji Animasi Login (Morphing Card)',
    category: 'Aksi Cepat',
    icon: LogOut,
    badge: 'Auth',
    action: () => logout(),
  },
  {
    id: 'nav-setting-bg',
    title: 'Pengaturan Dasar Pertama (Background)',
    category: 'Navigasi',
    icon: Palette,
    badge: 'Setting',
    action: () => router.push('/template/dasar-pertama'),
  },

  // Aksi Cepat Sistem
  {
    id: 'act-toggle-theme',
    title: computed(() => (isDark.value ? 'Beralih ke Mode Terang (Light Mode)' : 'Beralih ke Mode Gelap (Dark Mode)')),
    category: 'Aksi Cepat',
    icon: computed(() => (isDark.value ? Sun : Moon)),
    badge: 'Tema',
    action: () => {
      toggleTheme()
      toast.info('Mode tampilan berhasil diubah!')
    },
  },
  {
    id: 'act-theme-terracotta',
    title: 'Terapkan Tema Warna Terracotta (Warm Sand)',
    category: 'Aksi Cepat',
    icon: Sparkles,
    badge: 'Warna',
    action: () => {
      setThemeColor('terracotta')
      toast.success('Tema Terracotta diterapkan!')
    },
  },
  {
    id: 'act-theme-pink',
    title: 'Terapkan Tema Warna Pink (#E85D88)',
    category: 'Aksi Cepat',
    icon: Sparkles,
    badge: 'Warna',
    action: () => {
      setThemeColor('pink')
      toast.success('Tema Pink diterapkan!')
    },
  },
  {
    id: 'act-theme-blue',
    title: 'Terapkan Tema Warna Blue (#007AFF)',
    category: 'Aksi Cepat',
    icon: Sparkles,
    badge: 'Warna',
    action: () => {
      setThemeColor('blue')
      toast.success('Tema Blue diterapkan!')
    },
  },
  {
    id: 'act-theme-default',
    title: 'Terapkan Tema Warna Default (Pure Dark)',
    category: 'Aksi Cepat',
    icon: Sparkles,
    badge: 'Warna',
    action: () => {
      setThemeColor('default')
      toast.success('Tema Default diterapkan!')
    },
  },

  // Data & Record Cepat
  {
    id: 'data-pegawai-1',
    title: 'Rizky Pratama — Lead Frontend Engineer',
    category: 'Data Pegawai',
    icon: Users,
    badge: 'EMP-2024-001',
    action: () => router.push('/template/data-2'),
  },
  {
    id: 'data-pegawai-2',
    title: 'Siti Rahma — Senior UI/UX Designer',
    category: 'Data Pegawai',
    icon: Users,
    badge: 'EMP-2024-002',
    action: () => router.push('/template/data-2'),
  },
  {
    id: 'data-gunung-semeru',
    title: 'Gunung Semeru — 3.676 mdpl (Jawa Timur)',
    category: 'Data Gunung',
    icon: Mountain,
    badge: 'Gunung',
    action: () => router.push('/template/data-1'),
  },
  {
    id: 'data-gunung-slameto',
    title: 'Gunung Slamet — 3.428 mdpl (Jawa Tengah)',
    category: 'Data Gunung',
    icon: Mountain,
    badge: 'Gunung',
    action: () => router.push('/template/data-1'),
  },
  {
    id: 'act-read-docs',
    title: 'Baca Panduan Desain (ui-desain.md)',
    category: 'Bantuan',
    icon: FileText,
    badge: 'Dokumentasi',
    action: () => {
      window.open('https://github.com/yaniyan4609/ui-ux-vue-v1.0/blob/main/ui-desain.md', '_blank')
    },
  },
]

// Filtered commands based on query
const filteredCommands = computed(() => {
  const q = searchQuery.value.trim().toLowerCase()
  if (!q) return commands

  return commands.filter((cmd) => {
    const titleText = typeof cmd.title === 'string' ? cmd.title : cmd.title.value
    const catText = cmd.category || ''
    const badgeText = cmd.badge || ''
    return (
      titleText.toLowerCase().includes(q) ||
      catText.toLowerCase().includes(q) ||
      badgeText.toLowerCase().includes(q)
    )
  })
})

// Grouped by Category for display
const groupedCommands = computed(() => {
  const groups = {}
  filteredCommands.value.forEach((cmd) => {
    if (!groups[cmd.category]) {
      groups[cmd.category] = []
    }
    groups[cmd.category].push(cmd)
  })
  return groups
})

// Flat list for keyboard index selection
const flatList = computed(() => filteredCommands.value)

// Keyboard navigation
const onKeyDown = (e) => {
  if (!props.modelValue) return

  if (e.key === 'ArrowDown') {
    e.preventDefault()
    if (flatList.value.length > 0) {
      selectedIndex.value = (selectedIndex.value + 1) % flatList.value.length
    }
  } else if (e.key === 'ArrowUp') {
    e.preventDefault()
    if (flatList.value.length > 0) {
      selectedIndex.value = (selectedIndex.value - 1 + flatList.value.length) % flatList.value.length
    }
  } else if (e.key === 'Enter') {
    e.preventDefault()
    executeSelected()
  } else if (e.key === 'Escape') {
    e.preventDefault()
    close()
  }
}

const executeSelected = () => {
  const target = flatList.value[selectedIndex.value]
  if (target) {
    target.action()
    close()
  }
}

const selectAndExecute = (idx) => {
  selectedIndex.value = idx
  executeSelected()
}

// Watch query reset index
watch(searchQuery, () => {
  selectedIndex.value = 0
})

// Auto-focus input when opened
watch(
  () => props.modelValue,
  (open) => {
    if (open) {
      selectedIndex.value = 0
      nextTick(() => {
        inputRef.value?.focus()
      })
    }
  }
)

// Global Ctrl+K / Cmd+K listener
const handleGlobalKey = (e) => {
  if ((e.metaKey || e.ctrlKey) && e.key.toLowerCase() === 'k') {
    e.preventDefault()
    emit('update:modelValue', !props.modelValue)
  }
}

onMounted(() => {
  window.addEventListener('keydown', handleGlobalKey)
  window.addEventListener('keydown', onKeyDown)
})

onUnmounted(() => {
  window.removeEventListener('keydown', handleGlobalKey)
  window.removeEventListener('keydown', onKeyDown)
})
</script>

<template>
  <Teleport to="body">
    <!-- Backdrop Overlay -->
    <Transition
      enter-active-class="transition duration-200 ease-out"
      enter-from-class="opacity-0"
      enter-to-class="opacity-100"
      leave-active-class="transition duration-150 ease-in"
      leave-from-class="opacity-100"
      leave-to-class="opacity-0"
    >
      <div
        v-if="modelValue"
        class="fixed inset-0 z-50 bg-black/60 backdrop-blur-xs flex items-start justify-center p-3 sm:p-6 pt-16 sm:pt-24 select-none"
        @click.self="close"
        role="dialog"
        aria-modal="true"
      >
        <!-- Modal Palette Window -->
        <Transition
          enter-active-class="transition duration-200 cubic-bezier(0.16, 1, 0.3, 1)"
          enter-from-class="opacity-0 scale-95 -translate-y-4"
          enter-to-class="opacity-100 scale-100 translate-y-0"
          leave-active-class="transition duration-150 ease-in"
          leave-from-class="opacity-100 scale-100 translate-y-0"
          leave-to-class="opacity-0 scale-95 -translate-y-4"
        >
          <div
            v-if="modelValue"
            class="relative w-full max-w-xl rounded-2xl bg-white dark:bg-[var(--theme-surface)] border border-neutral-200 dark:border-[var(--theme-border)] shadow-2xl overflow-hidden flex flex-col max-h-[75vh]"
          >
            <!-- Search Bar Input Header -->
            <div class="flex items-center gap-3 px-4 py-3.5 border-b border-neutral-200/90 dark:border-[var(--theme-border)] bg-neutral-50/50 dark:bg-[var(--theme-surface-raised)]">
              <Search class="w-5 h-5 text-neutral-400 shrink-0" />
              <input
                ref="inputRef"
                v-model="searchQuery"
                type="text"
                placeholder="Ketik untuk mencari halaman, data, atau aksi cepat..."
                class="w-full bg-transparent text-sm text-neutral-900 dark:text-white placeholder-neutral-400 focus:outline-none"
              />
              <button
                v-if="searchQuery"
                type="button"
                @click="searchQuery = ''"
                class="p-1 text-neutral-400 hover:text-neutral-600 dark:hover:text-neutral-200 rounded-md transition"
              >
                <X class="w-4 h-4" />
              </button>
              <kbd class="hidden sm:inline-block px-1.5 py-0.5 text-[10.5px] font-mono text-neutral-400 bg-neutral-200/60 dark:bg-[var(--theme-surface-muted)] rounded-md border border-neutral-300 dark:border-[var(--theme-border-strong)]">
                ESC
              </kbd>
            </div>

            <!-- List Results -->
            <div class="flex-1 overflow-y-auto p-2 divide-y divide-neutral-100 dark:divide-[var(--theme-border)]">
              <div v-if="flatList.length === 0" class="py-12 text-center text-xs text-neutral-400 space-y-2">
                <Search class="w-8 h-8 mx-auto opacity-30" />
                <p>Tidak ada hasil untuk "<strong>{{ searchQuery }}</strong>"</p>
                <span class="text-[11px] text-neutral-500">Coba kata kunci: beranda, tema, semeru, pegawai, atau setting.</span>
              </div>

              <!-- Group List -->
              <div
                v-for="(items, category) in groupedCommands"
                :key="category"
                class="py-1.5 first:pt-0 last:pb-0"
              >
                <div class="px-2.5 py-1 text-[10px] font-bold uppercase tracking-wider text-neutral-400 dark:text-neutral-500">
                  {{ category }}
                </div>

                <div class="space-y-0.5">
                  <div
                    v-for="cmd in items"
                    :key="cmd.id"
                    @click="selectAndExecute(flatList.indexOf(cmd))"
                    @mouseenter="selectedIndex = flatList.indexOf(cmd)"
                    class="group flex items-center justify-between px-3 py-2.5 rounded-xl text-xs cursor-pointer transition-colors duration-150"
                    :class="[
                      selectedIndex === flatList.indexOf(cmd)
                        ? 'bg-[var(--theme-primary-soft)] text-[var(--theme-primary)] font-semibold'
                        : 'text-neutral-700 dark:text-neutral-300 hover:bg-neutral-100/70 dark:hover:bg-[var(--theme-surface-muted)]',
                    ]"
                  >
                    <div class="flex items-center gap-2.5 truncate">
                      <component
                        :is="cmd.icon"
                        class="w-4 h-4 shrink-0 transition-transform"
                        :class="
                          selectedIndex === flatList.indexOf(cmd)
                            ? 'text-[var(--theme-primary)] scale-110'
                            : 'text-neutral-400'
                        "
                      />
                      <span class="truncate">{{ typeof cmd.title === 'string' ? cmd.title : cmd.title.value }}</span>
                    </div>

                    <div class="flex items-center gap-2 shrink-0">
                      <span
                        v-if="cmd.badge"
                        class="text-[10px] px-2 py-0.5 rounded-md font-mono"
                        :class="
                          selectedIndex === flatList.indexOf(cmd)
                            ? 'bg-white/80 dark:bg-[var(--theme-surface-raised)] text-[var(--theme-primary)] shadow-2xs'
                            : 'bg-neutral-100 dark:bg-[var(--theme-surface-muted)] text-neutral-500 dark:text-neutral-400'
                        "
                      >
                        {{ cmd.badge }}
                      </span>
                      <CornerDownLeft
                        v-if="selectedIndex === flatList.indexOf(cmd)"
                        class="w-3.5 h-3.5 text-[var(--theme-primary)] opacity-70"
                      />
                    </div>
                  </div>
                </div>
              </div>
            </div>

            <!-- Footer Keyboard Helper Navigation -->
            <div class="px-4 py-2.5 border-t border-neutral-100 dark:border-[var(--theme-border)] bg-neutral-50/70 dark:bg-[var(--theme-surface)] flex items-center justify-between text-[11px] text-neutral-400">
              <div class="flex items-center gap-3">
                <span class="flex items-center gap-1">
                  <kbd class="px-1 py-0.2 bg-white dark:bg-[var(--theme-surface-muted)] rounded border border-neutral-200 dark:border-[var(--theme-border-strong)] text-[9.5px]">↑</kbd>
                  <kbd class="px-1 py-0.2 bg-white dark:bg-[var(--theme-surface-muted)] rounded border border-neutral-200 dark:border-[var(--theme-border-strong)] text-[9.5px]">↓</kbd>
                  navigasi
                </span>
                <span class="flex items-center gap-1">
                  <kbd class="px-1 py-0.2 bg-white dark:bg-[var(--theme-surface-muted)] rounded border border-neutral-200 dark:border-[var(--theme-border-strong)] text-[9.5px]">↵</kbd>
                  pilih
                </span>
                <span class="flex items-center gap-1">
                  <kbd class="px-1 py-0.2 bg-white dark:bg-[var(--theme-surface-muted)] rounded border border-neutral-200 dark:border-[var(--theme-border-strong)] text-[9.5px]">esc</kbd>
                  tutup
                </span>
              </div>
              <span class="font-mono text-[10px] text-neutral-500">Digaji Command Palette</span>
            </div>
          </div>
        </Transition>
      </div>
    </Transition>
  </Teleport>
</template>

