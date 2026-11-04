<script setup>
import { ref, computed, watch, onMounted, onUnmounted } from 'vue'
import { X } from 'lucide-vue-next'

const props = defineProps({
  modelValue: {
    type: Boolean,
    default: false,
  },
  title: {
    type: String,
    default: '',
  },
  subtitle: {
    type: String,
    default: '',
  },
  closeOnBackdrop: {
    type: Boolean,
    default: true,
  },
})

const emit = defineEmits(['update:modelValue', 'close'])

const sheetRef = ref(null)
const isDragging = ref(false)
const dragY = ref(0)
const startY = ref(0)
const sheetHeight = ref(400)

const close = () => {
  emit('update:modelValue', false)
  emit('close')
}

const onBackdropClick = () => {
  if (props.closeOnBackdrop) {
    close()
  }
}

// Opacity backdrop dinamis saat user menggeser sheet ke bawah
const backdropOpacity = computed(() => {
  if (!isDragging.value && dragY.value === 0) return 1
  const progress = Math.min(1, Math.max(0, dragY.value / (sheetHeight.value * 0.7)))
  return Math.max(0, 1 - progress)
})

const onPointerDown = (e) => {
  isDragging.value = true
  startY.value = e.clientY ?? e.touches?.[0]?.clientY ?? 0
  if (sheetRef.value) {
    sheetHeight.value = sheetRef.value.offsetHeight || 400
  }

  window.addEventListener('pointermove', onPointerMove)
  window.addEventListener('pointerup', onPointerUp)
  window.addEventListener('pointercancel', onPointerUp)
}

const onPointerMove = (e) => {
  if (!isDragging.value) return
  const currentY = e.clientY ?? e.touches?.[0]?.clientY ?? startY.value
  const delta = currentY - startY.value

  // Hanya izinkan slide ke bawah, dengan resistansi jika ditarik ke atas
  if (delta > 0) {
    dragY.value = delta
  } else {
    dragY.value = delta * 0.15
  }
}

const onPointerUp = () => {
  if (!isDragging.value) return
  isDragging.value = false

  window.removeEventListener('pointermove', onPointerMove)
  window.removeEventListener('pointerup', onPointerUp)
  window.removeEventListener('pointercancel', onPointerUp)

  // Threshold: jika ditarik ke bawah > 100px atau 25% tinggi sheet, selesaikan penutupan
  if (dragY.value > Math.min(100, sheetHeight.value * 0.25)) {
    // Geser turun penuh ke bawah
    dragY.value = sheetHeight.value
    setTimeout(() => {
      close()
    }, 180)
  } else {
    // Snap back lembut ke posisi normal
    dragY.value = 0
  }
}

// Reset state HANYA setelah transisi leave selesai sepenuhnya (mencegah kedipan)
const onAfterLeave = () => {
  dragY.value = 0
  isDragging.value = false
  document.body.style.overflow = ''
}

watch(
  () => props.modelValue,
  (val) => {
    if (val) {
      dragY.value = 0
      isDragging.value = false
      document.body.style.overflow = 'hidden'
    }
  }
)

const handleKeydown = (e) => {
  if (e.key === 'Escape' && props.modelValue) {
    close()
  }
}

onMounted(() => {
  window.addEventListener('keydown', handleKeydown)
})

onUnmounted(() => {
  window.removeEventListener('keydown', handleKeydown)
  window.removeEventListener('pointermove', onPointerMove)
  window.removeEventListener('pointerup', onPointerUp)
  window.removeEventListener('pointercancel', onPointerUp)
  document.body.style.overflow = ''
})
</script>

<template>
  <Teleport to="body">
    <!-- Outer Transition: 250ms masuk, 180ms keluar (Sinkron & bebas kedipan) -->
    <Transition
      enter-active-class="transition duration-250 ease-out"
      enter-from-class="opacity-0"
      enter-to-class="opacity-100"
      leave-active-class="transition duration-180 ease-in"
      leave-from-class="opacity-100"
      leave-to-class="opacity-0"
      @after-leave="onAfterLeave"
    >
      <div
        v-if="modelValue"
        class="fixed inset-0 z-50 flex items-end justify-center p-0 sm:p-4 select-none"
        role="dialog"
        aria-modal="true"
      >
        <!-- Backdrop: bg-black/50 + backdrop-blur-sm, fade in serempak tanpa jeda warna ke blur -->
        <div
          class="absolute inset-0 bg-black/50 backdrop-blur-sm pointer-events-auto will-change-[opacity]"
          :style="{
            opacity: isDragging || dragY > 0 ? backdropOpacity : 1,
            transition: isDragging ? 'none' : 'opacity 0.2s ease',
          }"
          @click="onBackdropClick"
        />

        <!-- Bottom Sheet Container (Muncul dari bawah ke atas & bisa dislide ke bawah untuk menutup) -->
        <Transition
          enter-active-class="transition duration-250 cubic-bezier(0.16, 1, 0.3, 1)"
          enter-from-class="translate-y-full opacity-70"
          enter-to-class="translate-y-0 opacity-100"
          leave-active-class="transition duration-180 cubic-bezier(0.16, 1, 0.3, 1)"
          leave-from-class="translate-y-0 opacity-100"
          leave-to-class="translate-y-full opacity-70"
        >
          <div
            v-if="modelValue"
            ref="sheetRef"
            class="relative w-full max-w-lg rounded-t-3xl sm:rounded-3xl bg-white dark:bg-[var(--theme-surface)] border-t sm:border border-neutral-200 dark:border-[var(--theme-border)] shadow-2xl flex flex-col max-h-[90vh] overflow-hidden z-10 will-change-transform"
            :style="{
              transform: `translateY(${dragY}px)`,
              transition: isDragging ? 'none' : 'transform 0.22s cubic-bezier(0.16, 1, 0.3, 1)',
            }"
          >
            <!-- Area Drag Handle (Zona tarikan slider ke bawah) -->
            <div
              class="w-full flex flex-col items-center pt-3 pb-2 cursor-grab active:cursor-grabbing touch-none"
              @pointerdown="onPointerDown"
            >
              <div class="h-1.5 w-12 rounded-full bg-neutral-300 dark:bg-neutral-600 transition-colors" />
              <span class="text-[10px] text-neutral-400 mt-1 font-medium select-none">
                Geser ke bawah untuk menutup
              </span>
            </div>

            <!-- Header Modal Mobile -->
            <div
              class="flex items-center justify-between px-6 py-2.5 border-b border-neutral-100 dark:border-[var(--theme-border)] touch-none cursor-grab active:cursor-grabbing"
              @pointerdown="onPointerDown"
            >
              <div>
                <h3 class="text-base font-bold text-neutral-900 dark:text-[var(--theme-text-primary)]">
                  {{ title }}
                </h3>
                <p v-if="subtitle" class="text-xs text-neutral-500 dark:text-neutral-400 mt-0.5">
                  {{ subtitle }}
                </p>
              </div>
              <button
                type="button"
                @click="close"
                class="rounded-full p-1.5 text-neutral-400 hover:bg-neutral-100 hover:text-neutral-600 dark:hover:bg-[var(--theme-surface-muted)] dark:hover:text-neutral-300 transition cursor-pointer"
                aria-label="Tutup"
              >
                <X class="w-5 h-5" />
              </button>
            </div>

            <!-- Scrollable Content Body -->
            <div class="px-5 sm:px-6 py-4 overflow-y-auto flex-1 text-neutral-700 dark:text-neutral-300 text-xs sm:text-sm overscroll-contain touch-auto select-auto">
              <slot />
            </div>

            <!-- Footer Sheet -->
            <div
              v-if="$slots.footer"
              class="border-t border-neutral-100 bg-neutral-50/90 px-5 sm:px-6 py-3.5 dark:border-[var(--theme-border)] dark:bg-[var(--theme-layer-2)] flex items-center justify-end gap-3 shrink-0"
            >
              <slot name="footer" :close="close" />
            </div>
          </div>
        </Transition>
      </div>
    </Transition>
  </Teleport>
</template>
