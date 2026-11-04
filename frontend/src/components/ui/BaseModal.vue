<script setup>
import { ref, computed, watch, onMounted, onUnmounted } from 'vue'
import { X } from 'lucide-vue-next'
import MobileModal from './MobileModal.vue'

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
  size: {
    type: String,
    default: 'md', // 'sm' | 'md' | 'lg' | 'xl' | 'full'
  },
  closeOnBackdrop: {
    type: Boolean,
    default: true,
  },
  closeOnEsc: {
    type: Boolean,
    default: true,
  },
  adaptive: {
    type: Boolean,
    default: true, // Otomatis beralih ke MobileModal (Bottom Sheet iOS-style) saat dibuka di layar mobile (< 768px)
  },
  breakpoint: {
    type: Number,
    default: 768, // px
  },
})

const emit = defineEmits(['update:modelValue', 'close'])

const isMobile = ref(false)

const checkMobile = () => {
  if (typeof window !== 'undefined') {
    isMobile.value = window.innerWidth < props.breakpoint
  }
}

const close = () => {
  emit('update:modelValue', false)
  emit('close')
}

const onBackdropClick = () => {
  if (props.closeOnBackdrop) {
    close()
  }
}

const handleKeydown = (e) => {
  if (props.closeOnEsc && e.key === 'Escape' && props.modelValue) {
    close()
  }
}

watch(
  () => props.modelValue,
  (val) => {
    // Hanya atur body overflow di sini jika sedang di mode desktop (MobileModal sudah menangani dirinya sendiri)
    if (!props.adaptive || !isMobile.value) {
      if (val) {
        document.body.style.overflow = 'hidden'
      } else {
        document.body.style.overflow = ''
      }
    }
  }
)

onMounted(() => {
  checkMobile()
  window.addEventListener('resize', checkMobile)
  window.addEventListener('keydown', handleKeydown)
})

onUnmounted(() => {
  window.removeEventListener('resize', checkMobile)
  window.removeEventListener('keydown', handleKeydown)
  if (!props.adaptive || !isMobile.value) {
    document.body.style.overflow = ''
  }
})

const sizeClass = computed(() => {
  const map = {
    sm: 'max-w-md',
    md: 'max-w-lg',
    lg: 'max-w-2xl',
    xl: 'max-w-4xl',
    full: 'max-w-[95vw] h-[90vh]',
  }
  return map[props.size] || map.md
})
</script>

<template>
  <!-- ======================================================== -->
  <!-- 1. MODE MOBILE: BOTTOM SHEET MODAL (Gaya iOS / Native)   -->
  <!-- Dilengkapi handle geser ke bawah untuk menutup sheet      -->
  <!-- ======================================================== -->
  <MobileModal
    v-if="adaptive && isMobile"
    :model-value="modelValue"
    :title="title"
    :subtitle="subtitle"
    :close-on-backdrop="closeOnBackdrop"
    @update:model-value="emit('update:modelValue', $event)"
    @close="emit('close')"
  >
    <slot />
    <template v-if="$slots.footer" #footer="slotProps">
      <slot name="footer" v-bind="slotProps" />
    </template>
  </MobileModal>

  <!-- ======================================================== -->
  <!-- 2. MODE DESKTOP: CENTERED DIALOG DENGAN BACKDROP BLUR    -->
  <!-- ======================================================== -->
  <Teleport v-else to="body">
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
        class="fixed inset-0 z-50 overflow-y-auto bg-black/60 backdrop-blur-xs p-4 sm:p-6 flex min-h-full items-center justify-center"
        @click.self="onBackdropClick"
        role="dialog"
        aria-modal="true"
      >
        <Transition
          enter-active-class="transition duration-200 ease-out"
          enter-from-class="opacity-0 scale-95 translate-y-2"
          enter-to-class="opacity-100 scale-100 translate-y-0"
          leave-active-class="transition duration-150 ease-in"
          leave-from-class="opacity-100 scale-100 translate-y-0"
          leave-to-class="opacity-0 scale-95 translate-y-2"
        >
          <div
            v-if="modelValue"
            class="relative w-full rounded-2xl bg-white shadow-2xl border border-neutral-200 dark:bg-[var(--theme-surface)] dark:border-[var(--theme-border)] flex flex-col overflow-hidden max-h-[90vh]"
            :class="sizeClass"
          >
            <!-- Header -->
            <div
              v-if="title || $slots.header"
              class="flex items-center justify-between border-b border-neutral-100 px-6 py-4 dark:border-[var(--theme-border)]"
            >
              <slot name="header">
                <div>
                  <h3 class="text-base font-semibold text-neutral-900 dark:text-[var(--theme-text-primary)] leading-tight">
                    {{ title }}
                  </h3>
                  <p v-if="subtitle" class="text-xs text-neutral-500 dark:text-neutral-400 mt-0.5">
                    {{ subtitle }}
                  </p>
                </div>
              </slot>
              <button
                type="button"
                @click="close"
                class="rounded-xl p-1.5 text-neutral-400 hover:bg-neutral-100 hover:text-neutral-600 dark:hover:bg-[var(--theme-surface-muted)] dark:hover:text-neutral-300 transition cursor-pointer"
                aria-label="Tutup modal"
              >
                <X class="w-5 h-5" />
              </button>
            </div>

            <!-- Content -->
            <div class="px-6 py-5 overflow-y-auto flex-1 text-neutral-700 dark:text-neutral-300 text-sm">
              <slot />
            </div>

            <!-- Footer -->
            <div
              v-if="$slots.footer"
              class="flex items-center justify-end gap-3 border-t border-neutral-100 bg-neutral-50/70 px-6 py-3.5 dark:border-[var(--theme-border)] dark:bg-[var(--theme-layer-2)]"
            >
              <slot name="footer" :close="close" />
            </div>
          </div>
        </Transition>
      </div>
    </Transition>
  </Teleport>
</template>
