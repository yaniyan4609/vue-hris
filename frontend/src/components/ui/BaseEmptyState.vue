<script setup>
import { computed } from 'vue'
import BaseButton from './BaseButton.vue'
import { Search, FolderOpen, Plus } from 'lucide-vue-next'

const props = defineProps({
  title: {
    type: String,
    default: 'Data Tidak Ditemukan',
  },
  description: {
    type: String,
    default: 'Tidak ada data yang cocok dengan kriteria pencarian atau filter yang Anda terapkan.',
  },
  icon: {
    type: [Object, Function],
    default: null,
  },
  actionText: {
    type: String,
    default: '',
  },
  actionIcon: {
    type: [Object, Function],
    default: null,
  },
  compact: {
    type: Boolean,
    default: false,
  },
})

const emit = defineEmits(['action'])

const effectiveIcon = computed(() => props.icon || Search)
</script>

<template>
  <div
    class="w-full flex flex-col items-center justify-center text-center select-none"
    :class="compact ? 'py-6 px-4' : 'py-12 sm:py-16 px-4'"
  >
    <!-- Soft Decorative Illustrated Badge Icon -->
    <div class="relative flex items-center justify-center mb-3">
      <!-- Outer soft pulse halo ring -->
      <div class="w-14 h-14 sm:w-16 sm:h-16 rounded-3xl bg-[var(--theme-primary-soft)] opacity-70 flex items-center justify-center shadow-xs" />

      <!-- Center Icon Box -->
      <div class="absolute w-10 h-10 sm:w-12 sm:h-12 rounded-2xl bg-white dark:bg-[var(--theme-surface-raised)] border border-neutral-200 dark:border-[var(--theme-border)] shadow-sm flex items-center justify-center text-[var(--theme-primary)]">
        <slot name="icon">
          <component :is="effectiveIcon" class="w-5 h-5 sm:w-6 sm:h-6 stroke-[1.8]" />
        </slot>
      </div>
    </div>

    <!-- Title & Description -->
    <h3 class="text-xs sm:text-sm font-bold text-neutral-800 dark:text-neutral-200 mt-1">
      <slot name="title">{{ title }}</slot>
    </h3>

    <p class="text-xs text-neutral-500 dark:text-neutral-400 mt-1 max-w-sm leading-relaxed">
      <slot name="description">{{ description }}</slot>
    </p>

    <!-- Optional Action Button -->
    <div v-if="actionText || $slots.action" class="mt-4">
      <slot name="action">
        <BaseButton
          variant="outline"
          size="sm"
          @click="emit('action')"
        >
          <component :is="actionIcon || Plus" class="w-3.5 h-3.5 mr-1" />
          <span>{{ actionText }}</span>
        </BaseButton>
      </slot>
    </div>
  </div>
</template>

