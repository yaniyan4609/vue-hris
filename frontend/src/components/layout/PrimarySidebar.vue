<script setup>
import { ref, computed } from 'vue'
import { useRouter, useRoute } from 'vue-router'
import { ChevronRight } from 'lucide-vue-next'

const props = defineProps({
  items: {
    type: Array,
    required: true,
  },
  activeId: {
    type: String,
    required: true,
  },
  capsule: {
    type: Boolean,
    default: false, // Mode capsule pill rounded top & bottom (Wireframe 1 mobile)
  },
  submenuCollapsed: {
    type: Boolean,
    default: false, // Status apakah submenu sedang ditutup / collapsed
  },
})

const emit = defineEmits(['select', 'open-submenu'])
const router = useRouter()
const route = useRoute()

// State Hover Flyout Submenu Card
const hoveredMenuId = ref(null)
let leaveTimer = null

const handleMouseEnter = (item) => {
  if (props.capsule) return
  if (!item.submenus || item.submenus.length === 0) return
  if (leaveTimer) {
    clearTimeout(leaveTimer)
    leaveTimer = null
  }
  hoveredMenuId.value = item.id
}

const handleMouseLeave = () => {
  if (props.capsule) return
  leaveTimer = setTimeout(() => {
    hoveredMenuId.value = null
  }, 160)
}

const clearLeaveTimer = () => {
  if (leaveTimer) {
    clearTimeout(leaveTimer)
    leaveTimer = null
  }
}

// Cek apakah submenu sedang tidak terbuka di samping rail
const isSubmenuClosed = computed(() => {
  const currentItem = props.items.find((m) => m.id === props.activeId)
  return props.submenuCollapsed || !currentItem?.submenus || currentItem.submenus.length === 0
})

const shouldShowFlyout = (item) => {
  if (props.capsule) return false
  if (!item.submenus || item.submenus.length === 0) return false
  if (hoveredMenuId.value !== item.id) return false
  // Tampilkan jika submenu tertutup, ATAU jika kursor diarahkan ke menu selain menu yang sedang aktif terbuka
  return isSubmenuClosed.value || item.id !== props.activeId
}

const isSubActive = (path) => {
  return route.path === path || route.path.startsWith(path + '/')
}

const handleItemClick = (item) => {
  hoveredMenuId.value = null
  emit('select', item.id)
  router.push(item.route)
}

const handleSubmenuClick = (item, sub) => {
  hoveredMenuId.value = null
  emit('select', item.id)
  router.push(sub.route)
}

const handlePinSubmenu = (item) => {
  hoveredMenuId.value = null
  emit('select', item.id)
  emit('open-submenu')
}
</script>

<template>
  <!-- Wireframe "MENU" Column -->
  <aside
    class="shrink-0 flex flex-col items-center select-none transition-all duration-200 z-30"
    :class="[
      capsule
        ? 'w-18 sm:w-20 my-3 ml-3 mr-1 py-4 px-1 rounded-[26px] border-2 border-neutral-300 dark:border-[var(--theme-border-strong)] bg-neutral-100/90 dark:bg-[var(--theme-surface-raised)] shadow-2xs h-[calc(100%-24px)]'
        : 'w-20 sm:w-22 bg-white dark:bg-[var(--theme-layer-2)] border-r border-neutral-200 dark:border-[var(--theme-border)] py-5'
    ]"
  >
    <!-- Wireframe "MENU" Text Indicator -->
    <div class="mb-5 px-2 text-center">
      <span class="text-[10px] font-bold tracking-widest text-neutral-600 dark:text-neutral-400 uppercase px-2 py-0.5 rounded-md border border-neutral-300 dark:border-[var(--theme-border)] bg-white dark:bg-[var(--theme-surface-muted)] shadow-2xs">
        MENU
      </span>
    </div>

    <!-- Menu Items: Beranda, Template -->
    <nav class="w-full flex flex-col items-center gap-2 px-1">
      <div
        v-for="item in items"
        :key="item.id"
        class="relative w-full"
        @mouseenter="handleMouseEnter(item)"
        @mouseleave="handleMouseLeave"
      >
        <button
          type="button"
          @click="handleItemClick(item)"
          class="group w-full py-2.5 px-1 flex flex-col items-center justify-center rounded-2xl transition-all duration-150 cursor-pointer text-center relative"
          :class="[
            activeId === item.id
              ? 'bg-[var(--theme-primary)] text-[var(--theme-primary-text)] shadow-sm font-semibold'
              : 'text-neutral-600 dark:text-neutral-400 hover:bg-neutral-200/60 dark:hover:bg-[var(--theme-surface-muted)] hover:text-neutral-900 dark:hover:text-white',
          ]"
        >
          <component
            :is="item.icon"
            class="w-5 h-5 transition-transform duration-150 group-hover:scale-105"
          />

          <span class="text-[10px] sm:text-[10.5px] tracking-tight mt-1 font-medium leading-none">
            {{ item.label }}
          </span>

          <!-- Active side indicator bar (hanya di mode desktop rail) -->
          <span
            v-if="activeId === item.id && !capsule"
            class="absolute -left-2 top-1/2 -translate-y-1/2 w-1.5 h-6 rounded-r-full bg-[var(--theme-primary)]"
          />
        </button>

        <!-- ======================================================== -->
        <!-- FLYOUT SUBMENU CARD ON HOVER (Muncul saat Submenu Ditutup)-->
        <!-- ======================================================== -->
        <Transition
          enter-active-class="transition duration-150 ease-out"
          enter-from-class="opacity-0 translate-x-1.5 scale-95"
          enter-to-class="opacity-100 translate-x-0 scale-100"
          leave-active-class="transition duration-100 ease-in"
          leave-from-class="opacity-100 translate-x-0 scale-100"
          leave-to-class="opacity-0 translate-x-1.5 scale-95"
        >
          <div
            v-if="shouldShowFlyout(item)"
            class="absolute left-full top-0 pl-3 z-50 pointer-events-auto"
            @mouseenter="clearLeaveTimer"
            @mouseleave="handleMouseLeave"
          >
            <div
              class="w-56 sm:w-60 rounded-2xl bg-white/95 dark:bg-[var(--theme-surface-raised)]/95 backdrop-blur-md border border-neutral-200 dark:border-[var(--theme-border-strong)] shadow-2xl overflow-hidden"
            >
              <!-- Card Header -->
              <div class="px-3.5 py-2.5 bg-neutral-50/80 dark:bg-[var(--theme-surface)]/80 border-b border-neutral-200/80 dark:border-[var(--theme-border)] flex items-center justify-between">
                <div class="flex items-center gap-2">
                  <div class="w-6 h-6 rounded-lg bg-[var(--theme-primary-soft)] text-[var(--theme-primary)] flex items-center justify-center shrink-0">
                    <component :is="item.icon" class="w-3.5 h-3.5" />
                  </div>
                  <div>
                    <span class="text-xs font-bold text-neutral-900 dark:text-white leading-none block">
                      {{ item.label }}
                    </span>
                    <span class="text-[10px] text-neutral-400 dark:text-neutral-500 leading-none">
                      {{ item.submenus.length }} Menu Tersedia
                    </span>
                  </div>
                </div>

                <!-- Tombol Pin / Buka Submenu Permanen -->
                <button
                  v-if="submenuCollapsed"
                  type="button"
                  @click.stop="handlePinSubmenu(item)"
                  class="px-1.5 py-1 rounded-md text-[10px] font-medium text-neutral-500 hover:text-neutral-800 dark:text-neutral-400 dark:hover:text-neutral-200 hover:bg-neutral-200/60 dark:hover:bg-[var(--theme-surface-muted)] transition flex items-center gap-0.5 cursor-pointer"
                  title="Sematkan / Buka Kolom Submenu Permanen"
                >
                  <span>Buka</span>
                  <ChevronRight class="w-3 h-3" />
                </button>
              </div>

              <!-- List Submenu Items -->
              <div class="p-1.5 space-y-0.5">
                <button
                  v-for="sub in item.submenus"
                  :key="sub.id"
                  type="button"
                  @click.stop="handleSubmenuClick(item, sub)"
                  class="w-full flex items-center justify-between px-2.5 py-2 rounded-xl text-xs font-medium transition cursor-pointer text-left group/sub"
                  :class="[
                    isSubActive(sub.route)
                      ? 'bg-[var(--theme-primary)] text-[var(--theme-primary-text)] shadow-xs font-semibold'
                      : 'text-neutral-600 dark:text-neutral-400 hover:bg-neutral-100 dark:hover:bg-[var(--theme-surface-muted)] hover:text-neutral-900 dark:hover:text-white',
                  ]"
                >
                  <div class="flex items-center gap-2.5 truncate">
                    <component
                      v-if="sub.icon"
                      :is="sub.icon"
                      class="w-4 h-4 shrink-0 transition-transform group-hover/sub:scale-110"
                      :class="isSubActive(sub.route) ? 'text-[var(--theme-primary-text)]' : 'text-neutral-400 dark:text-neutral-500 group-hover/sub:text-neutral-600 dark:group-hover/sub:text-neutral-300'"
                    />
                    <span class="truncate">{{ sub.label }}</span>
                  </div>

                  <!-- Dot Aktif -->
                  <span
                    v-if="isSubActive(sub.route)"
                    class="w-1.5 h-1.5 rounded-full bg-white dark:bg-white shrink-0"
                  />
                  <ChevronRight
                    v-else
                    class="w-3 h-3 opacity-0 -translate-x-1 group-hover/sub:opacity-50 group-hover/sub:translate-x-0 transition-all shrink-0"
                  />
                </button>
              </div>
            </div>
          </div>
        </Transition>
      </div>
    </nav>
  </aside>
</template>
