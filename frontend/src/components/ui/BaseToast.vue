<script setup>
import { computed } from 'vue'
import { AlertTriangle, Info, X } from 'lucide-vue-next'

const props = defineProps({
  toast: {
    type: Object,
    required: true,
  },
})

const emit = defineEmits(['close'])
</script>

<template>
  <div
    class="pointer-events-auto relative flex items-center justify-between gap-3 w-full max-w-sm px-4 py-3 sm:px-4.5 sm:py-3.5 rounded-2xl bg-white dark:bg-[var(--theme-surface-raised)] border border-neutral-200/90 dark:border-[var(--theme-border)] shadow-[0_8px_30px_rgba(0,0,0,0.08)] dark:shadow-[0_8px_30px_rgba(0,0,0,0.5)] text-neutral-900 dark:text-[var(--theme-text-primary)] transition-all select-none"
    role="status"
    aria-live="polite"
  >
    <!-- Left Icon Area with Smooth Out-In Transition -->
    <div class="shrink-0 flex items-center justify-center">
      <Transition name="icon-fade" mode="out-in">
        <!-- 1. LOADING SPINNER (Sesuai Gambar Referensi: Saving...) -->
        <div
          v-if="toast.type === 'loading'"
          key="loading"
          class="flex items-center justify-center"
        >
          <svg
            class="w-4.5 h-4.5 text-neutral-400 dark:text-neutral-500 animate-spin"
            viewBox="0 0 24 24"
            fill="none"
            xmlns="http://www.w3.org/2000/svg"
          >
            <!-- Background faint track -->
            <circle
              cx="12"
              cy="12"
              r="9"
              stroke="currentColor"
              stroke-width="2.4"
              stroke-opacity="0.2"
            />
            <!-- Rotating partial arc -->
            <path
              d="M12 3C7.02944 3 3 7.02944 3 12C3 14.2822 3.85044 16.3654 5.25367 17.95"
              stroke="currentColor"
              stroke-width="2.4"
              stroke-linecap="round"
            />
          </svg>
        </div>

        <!-- 2. SUCCESS ICON (Sesuai Gambar Referensi: Green Circle + Checkmark) -->
        <div
          v-else-if="toast.type === 'success'"
          key="success"
          class="flex items-center justify-center text-emerald-600 dark:text-emerald-400"
        >
          <svg
            class="w-4.5 h-4.5"
            viewBox="0 0 24 24"
            fill="none"
            stroke="currentColor"
            stroke-width="2.2"
            stroke-linecap="round"
            stroke-linejoin="round"
            xmlns="http://www.w3.org/2000/svg"
          >
            <circle cx="12" cy="12" r="9.5" />
            <path d="M8.5 12.2L10.8 14.5L15.5 9.8" />
          </svg>
        </div>

        <!-- 3. ERROR ICON: Red Circle + X -->
        <div
          v-else-if="toast.type === 'error'"
          key="error"
          class="flex items-center justify-center text-rose-500 dark:text-rose-400"
        >
          <svg
            class="w-4.5 h-4.5"
            viewBox="0 0 24 24"
            fill="none"
            stroke="currentColor"
            stroke-width="2.2"
            stroke-linecap="round"
            stroke-linejoin="round"
            xmlns="http://www.w3.org/2000/svg"
          >
            <circle cx="12" cy="12" r="9.5" />
            <line x1="14.5" y1="9.5" x2="9.5" y2="14.5" />
            <line x1="9.5" y1="9.5" x2="14.5" y2="14.5" />
          </svg>
        </div>

        <!-- 4. WARNING ICON -->
        <div
          v-else-if="toast.type === 'warning'"
          key="warning"
          class="flex items-center justify-center text-amber-500 dark:text-amber-400"
        >
          <AlertTriangle class="w-4.5 h-4.5" />
        </div>

        <!-- 5. INFO / DEFAULT ICON -->
        <div
          v-else
          key="info"
          class="flex items-center justify-center text-blue-500 dark:text-blue-400"
        >
          <Info class="w-4.5 h-4.5" />
        </div>
      </Transition>
    </div>

    <!-- Center Text Area with Smooth Out-In Transition -->
    <div class="flex-1 min-w-0 pr-1">
      <Transition name="text-fade" mode="out-in">
        <div :key="toast.message" class="flex flex-col justify-center">
          <p
            v-if="toast.title"
            class="text-[11px] font-bold text-neutral-900 dark:text-neutral-100 leading-tight mb-0.5"
          >
            {{ toast.title }}
          </p>
          <p
            class="text-xs sm:text-[13px] font-medium text-neutral-800 dark:text-neutral-100 leading-normal"
          >
            {{ toast.message }}
          </p>
        </div>
      </Transition>

      <!-- Optional Action Link/Button -->
      <div v-if="toast.action" class="mt-1">
        <button
          type="button"
          @click="toast.action.onClick?.()"
          class="text-[11px] font-bold text-neutral-900 dark:text-white hover:underline cursor-pointer"
        >
          {{ toast.action.label }}
        </button>
      </div>
    </div>

    <!-- Right Close 'X' Button (Sesuai Gambar Referensi) -->
    <button
      type="button"
      @click="emit('close', toast.id)"
      class="shrink-0 p-1 -mr-1 rounded-lg text-neutral-400 hover:text-neutral-700 dark:hover:text-neutral-200 hover:bg-neutral-100 dark:hover:bg-[var(--theme-surface-muted)] transition cursor-pointer"
      aria-label="Tutup"
    >
      <X class="w-3.5 h-3.5" />
    </button>
  </div>
</template>

<style scoped>
/* Transisi halus saat icon berubah dari loading spinner ke centang */
.icon-fade-enter-active,
.icon-fade-leave-active {
  transition: all 0.22s cubic-bezier(0.16, 1, 0.3, 1);
}
.icon-fade-enter-from {
  opacity: 0;
  transform: scale(0.65);
}
.icon-fade-leave-to {
  opacity: 0;
  transform: scale(0.65);
}

/* Transisi halus saat teks berubah dari 'Menyimpan...' ke 'Perubahan disimpan' */
.text-fade-enter-active,
.text-fade-leave-active {
  transition: opacity 0.18s ease-out, transform 0.18s ease-out;
}
.text-fade-enter-from {
  opacity: 0;
  transform: translateY(2px);
}
.text-fade-leave-to {
  opacity: 0;
  transform: translateY(-2px);
}
</style>
