<script setup>
import { useToast } from '../../composables/useToast'
import BaseToast from './BaseToast.vue'

const { toasts, remove } = useToast()
</script>

<template>
  <div
    class="pointer-events-none fixed inset-0 z-50 flex flex-col items-center sm:items-end gap-2.5 p-4 sm:p-6 sm:bottom-auto bottom-0 sm:top-0 justify-end sm:justify-start"
    aria-live="polite"
  >
    <TransitionGroup
      enter-active-class="transition duration-250 ease-out"
      enter-from-class="opacity-0 translate-y-3 sm:-translate-y-2 scale-95"
      enter-to-class="opacity-100 translate-y-0 scale-100"
      leave-active-class="transition duration-150 ease-in"
      leave-from-class="opacity-100 translate-y-0 scale-100"
      leave-to-class="opacity-0 scale-95"
      move-class="transition duration-200"
    >
      <BaseToast
        v-for="toast in toasts"
        :key="toast.id"
        :toast="toast"
        @close="remove"
      />
    </TransitionGroup>
  </div>
</template>
