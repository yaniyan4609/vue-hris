<script setup>
import { ref, computed, onUnmounted } from 'vue'

const props = defineProps({
  text: {
    type: String,
    default: '',
  },
  placement: {
    type: String,
    default: 'top', // 'top' | 'bottom' | 'left' | 'right'
  },
  delay: {
    type: Number,
    default: 120,
  },
  maxWidthClass: {
    type: String,
    default: 'max-w-xs',
  },
})

const isVisible = ref(false)
let timer = null

const show = () => {
  clearTimeout(timer)
  timer = setTimeout(() => {
    isVisible.value = true
  }, props.delay)
}

const hide = () => {
  clearTimeout(timer)
  isVisible.value = false
}

onUnmounted(() => {
  clearTimeout(timer)
})

const positionClasses = computed(() => {
  const map = {
    top: 'bottom-full left-1/2 -translate-x-1/2 mb-1.5',
    bottom: 'top-full left-1/2 -translate-x-1/2 mt-1.5',
    left: 'right-full top-1/2 -translate-y-1/2 mr-1.5',
    right: 'left-full top-1/2 -translate-y-1/2 ml-1.5',
  }
  return map[props.placement] || map.top
})
</script>

<template>
  <span
    class="relative inline-flex items-center"
    @mouseenter="show"
    @mouseleave="hide"
    @focusin="show"
    @focusout="hide"
  >
    <slot />

    <Transition
      enter-active-class="transition duration-120 ease-out"
      enter-from-class="opacity-0 scale-95"
      enter-to-class="opacity-100 scale-100"
      leave-active-class="transition duration-75 ease-in"
      leave-from-class="opacity-100 scale-100"
      leave-to-class="opacity-0 scale-95"
    >
      <div
        v-if="isVisible && (text || $slots.content)"
        role="tooltip"
        class="absolute z-50 pointer-events-none px-2.5 py-1.5 rounded-xl bg-neutral-900 dark:bg-[var(--theme-surface-raised)] text-white dark:text-neutral-100 border border-neutral-800 dark:border-[var(--theme-border-strong)] shadow-xl text-[11px] font-medium leading-snug w-max"
        :class="[positionClasses, maxWidthClass]"
      >
        <slot name="content">{{ text }}</slot>
      </div>
    </Transition>
  </span>
</template>

