<script setup>
import { ref, computed, watch, onMounted, onUnmounted, nextTick } from 'vue'
import { useRouter, useRoute } from 'vue-router'
import { ChevronLeft, ChevronRight } from 'lucide-vue-next'

const props = defineProps({
  submenus: {
    type: Array,
    default: () => [],
  },
  menuTitle: {
    type: String,
    default: '',
  },
  collapsed: {
    type: Boolean,
    default: false,
  },
  drawerMode: {
    type: Boolean,
    default: false,
  },
})

const emit = defineEmits(['toggle-collapse', 'navigate'])
const router = useRouter()
const route = useRoute()

const isSubActive = (path) => {
  return route.path === path
}

const handleNavigate = (path) => {
  emit('navigate', path)
  router.push(path)
}

// ========================================================
// Sliding Indicator Logic (Animasi Geser Halus & Elegan)
// ========================================================
const itemRefs = ref([])
const containerRef = ref(null)
const isReady = ref(false)

const pillStyle = ref({
  transform: 'translate3d(0, 0, 0)',
  height: '0px',
  opacity: 0,
})

const setItemRef = (el, idx) => {
  if (el) {
    itemRefs.value[idx] = el
  }
}

const updatePill = () => {
  if (!props.submenus || props.submenus.length === 0) {
    pillStyle.value.opacity = 0
    return
  }

  const activeIdx = props.submenus.findIndex((sub) => isSubActive(sub.route))
  // Jika sedang di root /template tanpa sub-path spesifik, arahkan ke index 0
  const targetIdx =
    activeIdx !== -1
      ? activeIdx
      : route.path.startsWith('/template')
      ? 0
      : -1

  if (targetIdx !== -1 && itemRefs.value[targetIdx]) {
    const el = itemRefs.value[targetIdx]
    pillStyle.value = {
      transform: `translate3d(0, ${el.offsetTop}px, 0)`,
      height: `${el.offsetHeight}px`,
      opacity: 1,
    }
  } else {
    pillStyle.value.opacity = 0
  }
}

// Pantau perubahan URL rute
watch(
  () => route.path,
  () => {
    nextTick(updatePill)
  }
)

// Pantau jika menu berubah
watch(
  () => props.submenus,
  () => {
    nextTick(updatePill)
  },
  { deep: true }
)

// Pantau saat sidebar dibuka kembali dari collapsed
watch(
  () => props.collapsed,
  (val) => {
    if (!val) {
      setTimeout(updatePill, 60)
    }
  }
)

let resizeObserver = null

onMounted(() => {
  nextTick(() => {
    updatePill()
    // Aktifkan animasi transisi setelah posisi awal terpasang tanpa loncatan awal
    setTimeout(() => {
      isReady.value = true
      updatePill()
    }, 60)
  })

  window.addEventListener('resize', updatePill)

  if (window.ResizeObserver && containerRef.value) {
    resizeObserver = new ResizeObserver(() => {
      updatePill()
    })
    resizeObserver.observe(containerRef.value)
  }
})

onUnmounted(() => {
  window.removeEventListener('resize', updatePill)
  if (resizeObserver) {
    resizeObserver.disconnect()
  }
})
</script>

<template>
  <!-- Wireframe "SUBMENU" Column (Pure Neutral Dark) -->
  <aside
    v-if="submenus && submenus.length > 0"
    class="shrink-0 flex flex-col transition-all duration-200 select-none overflow-hidden z-10"
    :class="[
      drawerMode
        ? 'w-52 sm:w-56 bg-transparent border-0'
        : (collapsed ? 'w-0 opacity-0 pointer-events-none' : 'w-56 sm:w-60 opacity-100 bg-neutral-50/90 dark:bg-[var(--theme-surface)] border-r border-neutral-200 dark:border-[var(--theme-border)]'),
    ]"
  >
    <!-- Wireframe "SUBMENU" Header -->
    <div class="p-3.5 border-b border-neutral-200 dark:border-[var(--theme-border)] flex items-center justify-between">
      <div>
        <span class="text-[10px] font-bold tracking-widest text-neutral-400 dark:text-neutral-500 uppercase block">
          SUBMENU
        </span>
        <h3 class="text-xs font-bold text-neutral-800 dark:text-neutral-200">
          {{ menuTitle }}
        </h3>
      </div>

      <!-- Collapse toggle button -->
      <button
        type="button"
        @click="emit('toggle-collapse')"
        class="p-1 rounded-lg text-neutral-400 hover:text-neutral-700 hover:bg-neutral-200/60 dark:hover:bg-[var(--theme-surface-muted)] dark:hover:text-neutral-200 transition cursor-pointer"
        title="Tutup Submenu"
      >
        <ChevronLeft class="w-4 h-4" />
      </button>
    </div>

    <!-- Submenu List Container dengan Sliding Active Pill -->
    <div ref="containerRef" class="flex-1 overflow-y-auto p-2.5">
      <div class="relative flex flex-col gap-1">
        <!-- Floating Active Card Pill (Geser ke Atas/Bawah dengan Kurva Halus & Elegan) -->
        <div
          class="absolute left-0 right-0 rounded-xl pointer-events-none z-0 bg-white dark:bg-[var(--theme-surface-muted)] border border-neutral-200/90 dark:border-[var(--theme-border-strong)] shadow-xs will-change-transform"
          :class="
            isReady
              ? 'transition-all duration-300 ease-[cubic-bezier(0.16,1,0.3,1)]'
              : ''
          "
          :style="pillStyle"
        />

        <!-- Submenu Item Buttons -->
        <button
          v-for="(sub, idx) in submenus"
          :key="sub.id"
          :ref="(el) => setItemRef(el, idx)"
          type="button"
          :title="sub.status === 'preview' ? `${sub.label} · Dalam pengembangan` : sub.label"
          @click="handleNavigate(sub.route)"
          class="relative z-10 w-full flex items-center justify-between px-3 py-2.5 rounded-xl text-xs transition-colors duration-200 text-left cursor-pointer"
          :class="[
            isSubActive(sub.route)
              ? 'text-[var(--theme-primary)] font-semibold'
              : 'text-neutral-600 dark:text-neutral-400 hover:text-neutral-900 dark:hover:text-white hover:bg-neutral-200/35 dark:hover:bg-neutral-800/35',
          ]"
        >
          <div class="flex items-center gap-2.5 truncate">
            <component
              :is="sub.icon"
              class="w-4 h-4 shrink-0 transition-transform duration-200"
              :class="
                isSubActive(sub.route)
                  ? 'text-[var(--theme-primary)] scale-105'
                  : 'text-neutral-400 dark:text-neutral-500'
              "
            />
            <span class="truncate">{{ sub.label }}</span>
          </div>

          <span v-if="sub.status === 'preview'" class="w-1.5 h-1.5 rounded-full bg-amber-400 shrink-0" aria-label="Dalam pengembangan" />
          <ChevronRight
            class="w-3.5 h-3.5 transition-all duration-200"
            :class="
              isSubActive(sub.route)
                ? 'text-[var(--theme-primary)] translate-x-0.5 opacity-100'
                : 'text-neutral-300 dark:text-neutral-600 opacity-60'
            "
          />
        </button>
      </div>
    </div>
  </aside>
</template>
