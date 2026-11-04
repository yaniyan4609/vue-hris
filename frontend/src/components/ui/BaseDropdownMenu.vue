<script setup>
import { ref, onMounted, onUnmounted } from 'vue'
import { MoreHorizontal } from 'lucide-vue-next'

const props = defineProps({
  items: {
    type: Array,
    default: () => [],
    // Each item: { label: string, icon?: any, action?: Function, danger?: boolean, disabled?: boolean, divider?: boolean }
  },
  align: {
    type: String,
    default: 'right', // 'left' | 'right'
  },
  triggerClass: {
    type: String,
    default: '',
  },
})

const emit = defineEmits(['select'])

const isOpen = ref(false)
const containerRef = ref(null)

const toggle = (e) => {
  e.stopPropagation()
  isOpen.value = !isOpen.value
}

const handleClickOutside = (e) => {
  if (containerRef.value && !containerRef.value.contains(e.target)) {
    isOpen.value = false
  }
}

const handleKeydown = (e) => {
  if (e.key === 'Escape' && isOpen.value) {
    isOpen.value = false
  }
}

const handleSelect = (item, e) => {
  e.stopPropagation()
  if (item.disabled) return
  if (item.action) {
    item.action()
  }
  emit('select', item)
  isOpen.value = false
}

onMounted(() => {
  document.addEventListener('click', handleClickOutside)
  document.addEventListener('keydown', handleKeydown)
})

onUnmounted(() => {
  document.removeEventListener('click', handleClickOutside)
  document.removeEventListener('keydown', handleKeydown)
})
</script>

<template>
  <div ref="containerRef" class="relative inline-block text-left select-none">
    <!-- Trigger Slot or Default 3-Dots Button -->
    <div @click="toggle">
      <slot name="trigger" :is-open="isOpen">
        <button
          type="button"
          class="p-1.5 rounded-lg border border-transparent hover:border-neutral-200 dark:hover:border-[var(--theme-border)] text-neutral-500 hover:text-neutral-900 dark:text-neutral-400 dark:hover:text-white hover:bg-neutral-100 dark:hover:bg-[var(--theme-surface-muted)] transition cursor-pointer shadow-2xs"
          :class="[isOpen ? 'bg-neutral-100 dark:bg-[var(--theme-surface-muted)] text-neutral-900 dark:text-white' : '', triggerClass]"
          title="Menu Aksi"
          aria-haspopup="true"
          :aria-expanded="isOpen"
        >
          <MoreHorizontal class="w-4 h-4" />
        </button>
      </slot>
    </div>

    <!-- Floating Dropdown Popover -->
    <Transition
      enter-active-class="transition duration-150 ease-out"
      enter-from-class="opacity-0 scale-95 -translate-y-1"
      enter-to-class="opacity-100 scale-100 translate-y-0"
      leave-active-class="transition duration-100 ease-in"
      leave-from-class="opacity-100 scale-100 translate-y-0"
      leave-to-class="opacity-0 scale-95 -translate-y-1"
    >
      <div
        v-if="isOpen"
        class="absolute top-full mt-1.5 min-w-[160px] z-50 rounded-2xl bg-white/95 dark:bg-[var(--theme-surface-raised)]/95 backdrop-blur-md border border-neutral-200/90 dark:border-[var(--theme-border)] shadow-xl p-1 space-y-0.5"
        :class="align === 'right' ? 'right-0' : 'left-0'"
        role="menu"
      >
        <slot :close="() => (isOpen = false)">
          <template v-for="(item, idx) in items" :key="idx">
            <!-- Divider Line -->
            <div
              v-if="item.divider"
              class="border-b border-neutral-100 dark:border-[var(--theme-border)] my-1 -mx-1"
            />

            <!-- Menu Option Item -->
            <button
              v-else
              type="button"
              @click="handleSelect(item, $event)"
              :disabled="item.disabled"
              class="w-full px-2.5 py-1.5 rounded-xl text-xs flex items-center gap-2 transition-all cursor-pointer text-left"
              :class="[
                item.disabled
                  ? 'opacity-40 cursor-not-allowed'
                  : item.danger
                  ? 'text-rose-600 dark:text-rose-400 hover:bg-rose-50 dark:hover:bg-rose-950/30'
                  : 'text-neutral-700 dark:text-neutral-200 hover:bg-neutral-100 dark:hover:bg-[var(--theme-surface-muted)] hover:text-neutral-900 dark:hover:text-white',
              ]"
              role="menuitem"
            >
              <!-- Icon -->
              <component
                :is="item.icon"
                v-if="item.icon"
                class="w-3.5 h-3.5 shrink-0"
                :class="item.danger ? 'text-rose-500' : 'text-neutral-400 group-hover:text-neutral-600'"
              />

              <!-- Label -->
              <span class="truncate flex-1 font-medium">{{ item.label }}</span>
            </button>
          </template>
        </slot>
      </div>
    </Transition>
  </div>
</template>

