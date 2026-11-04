<script setup>
import { computed, watch, onMounted, onUnmounted } from 'vue'
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
  size: {
    type: String,
    default: 'md', // 'sm' (max-w-sm) | 'md' (max-w-lg) | 'lg' (max-w-2xl) | 'xl' (max-w-3xl)
  },
  closeOnEsc: {
    type: Boolean,
    default: true,
  },
  closeOnClickOutside: {
    type: Boolean,
    default: true,
  },
})

const emit = defineEmits(['update:modelValue', 'close'])

const close = () => {
  emit('update:modelValue', false)
  emit('close')
}

const handleKeydown = (e) => {
  if (props.closeOnEsc && e.key === 'Escape' && props.modelValue) {
    close()
  }
}

watch(
  () => props.modelValue,
  (val) => {
    if (val) {
      document.body.style.overflow = 'hidden'
    } else {
      document.body.style.overflow = ''
    }
  }
)

onMounted(() => {
  document.addEventListener('keydown', handleKeydown)
})

onUnmounted(() => {
  document.removeEventListener('keydown', handleKeydown)
  document.body.style.overflow = ''
})

const sizeClasses = computed(() => {
  switch (props.size) {
    case 'sm':
      return 'max-w-sm'
    case 'lg':
      return 'max-w-2xl'
    case 'xl':
      return 'max-w-3xl'
    case 'md':
    default:
      return 'max-w-lg'
  }
})
</script>

<template>
  <Teleport to="body">
    <div
      v-if="modelValue"
      class="fixed inset-0 z-50 overflow-hidden"
      aria-labelledby="slide-over-title"
      role="dialog"
      aria-modal="true"
    >
      <!-- Backdrop with subtle blur -->
      <Transition
        appear
        enter-active-class="ease-out duration-300 transition-opacity"
        enter-from-class="opacity-0"
        enter-to-class="opacity-100"
        leave-active-class="ease-in duration-200 transition-opacity"
        leave-from-class="opacity-100"
        leave-to-class="opacity-0"
      >
        <div
          v-if="modelValue"
          class="fixed inset-0 bg-neutral-950/40 dark:bg-black/60 backdrop-blur-xs transition-opacity"
          @click="closeOnClickOutside && close()"
        />
      </Transition>

      <div class="fixed inset-y-0 right-0 flex max-w-full pl-6 sm:pl-10 pointer-events-none">
        <!-- Sliding Content Drawer -->
        <Transition
          appear
          enter-active-class="transform transition ease-out duration-300"
          enter-from-class="translate-x-full"
          enter-to-class="translate-x-0"
          leave-active-class="transform transition ease-in duration-200"
          leave-from-class="translate-x-0"
          leave-to-class="translate-x-full"
        >
          <div
            v-if="modelValue"
            class="w-screen pointer-events-auto flex flex-col bg-white dark:bg-[var(--theme-layer-2)] border-l border-neutral-200 dark:border-[var(--theme-border)] shadow-2xl select-none"
            :class="sizeClasses"
          >
            <!-- Drawer Header (shrink-0) -->
            <div class="py-4 px-5 sm:px-6 border-b border-neutral-200 dark:border-[var(--theme-border)] flex items-center justify-between gap-3 bg-neutral-50/50 dark:bg-[var(--theme-surface)]/80 shrink-0">
              <slot name="header">
                <div class="min-w-0">
                  <div class="flex items-center gap-2">
                    <h3
                      id="slide-over-title"
                      class="text-sm sm:text-base font-bold text-neutral-900 dark:text-white truncate"
                    >
                      {{ title }}
                    </h3>
                    <slot name="badge" />
                  </div>
                  <p
                    v-if="subtitle"
                    class="text-xs text-neutral-500 dark:text-neutral-400 mt-0.5 truncate"
                  >
                    {{ subtitle }}
                  </p>
                </div>
              </slot>

              <!-- Close Button -->
              <button
                type="button"
                @click="close"
                class="p-1.5 rounded-xl text-neutral-400 hover:text-neutral-700 dark:hover:text-neutral-200 hover:bg-neutral-100 dark:hover:bg-[var(--theme-surface-muted)] transition cursor-pointer shrink-0"
                title="Tutup Panel (Esc)"
              >
                <X class="w-5 h-5" />
              </button>
            </div>

            <!-- Drawer Scrollable Body -->
            <div class="flex-1 min-h-0 overflow-y-auto p-5 sm:p-6 space-y-5">
              <slot />
            </div>

            <!-- Drawer Footer (Optional, shrink-0) -->
            <div
              v-if="$slots.footer"
              class="py-3 px-5 sm:px-6 border-t border-neutral-200 dark:border-[var(--theme-border)] bg-neutral-50/70 dark:bg-[var(--theme-surface)] flex items-center justify-end gap-2.5 shrink-0"
            >
              <slot name="footer" :close="close" />
            </div>
          </div>
        </Transition>
      </div>
    </div>
  </Teleport>
</template>

