<script setup>
import { ref, computed, watch, onMounted, onUnmounted } from 'vue'

const props = defineProps({
  modelValue: {
    type: Boolean,
    default: undefined,
  },
  placement: {
    type: String,
    default: 'bottom-end', // 'bottom-end' | 'bottom-start' | 'top-end' | 'top-start'
  },
  widthClass: {
    type: String,
    default: 'w-80',
  },
  trigger: {
    type: String,
    default: 'click', // 'click' | 'hover'
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

const emit = defineEmits(['update:modelValue', 'open', 'close'])

const internalOpen = ref(false)
const containerRef = ref(null)

const isControlled = computed(() => props.modelValue !== undefined)

const isOpen = computed({
  get() {
    return isControlled.value ? props.modelValue : internalOpen.value
  },
  set(val) {
    if (!isControlled.value) {
      internalOpen.value = val
    }
    emit('update:modelValue', val)
    if (val) emit('open')
    else emit('close')
  },
})

const open = () => {
  isOpen.value = true
}

const close = () => {
  isOpen.value = false
}

const toggle = () => {
  isOpen.value = !isOpen.value
}

watch(
  () => props.modelValue,
  (val) => {
    if (val !== undefined) {
      internalOpen.value = val
    }
  }
)

const handleClickOutside = (e) => {
  if (!props.closeOnClickOutside || !isOpen.value) return
  if (containerRef.value && !containerRef.value.contains(e.target)) {
    close()
  }
}

const handleKeydown = (e) => {
  if (!props.closeOnEsc || !isOpen.value) return
  if (e.key === 'Escape') {
    e.stopPropagation()
    close()
  }
}

onMounted(() => {
  document.addEventListener('click', handleClickOutside)
  document.addEventListener('keydown', handleKeydown)
})

onUnmounted(() => {
  document.removeEventListener('click', handleClickOutside)
  document.removeEventListener('keydown', handleKeydown)
})

const placementClasses = computed(() => {
  const map = {
    'bottom-end': 'top-full right-0 mt-2',
    'bottom-start': 'top-full left-0 mt-2',
    'top-end': 'bottom-full right-0 mb-2',
    'top-start': 'bottom-full left-0 mb-2',
  }
  return map[props.placement] || map['bottom-end']
})
</script>

<template>
  <div
    ref="containerRef"
    class="relative inline-block"
    @mouseenter="trigger === 'hover' ? open() : null"
    @mouseleave="trigger === 'hover' ? close() : null"
  >
    <!-- Trigger Area -->
    <div @click="trigger === 'click' ? toggle() : null">
      <slot name="trigger" :is-open="isOpen" :toggle="toggle" :open="open" :close="close" />
    </div>

    <!-- Floating Popover Card -->
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
        class="absolute z-50 rounded-2xl bg-white/95 dark:bg-[var(--theme-surface-raised)]/95 backdrop-blur-md border border-neutral-200/90 dark:border-[var(--theme-border)] shadow-2xl overflow-hidden max-w-[calc(100vw-24px)]"
        :class="[placementClasses, widthClass]"
      >
        <!-- Optional Header Slot -->
        <div
          v-if="$slots.header"
          class="px-4 py-2.5 border-b border-neutral-200/80 dark:border-[var(--theme-border)] bg-neutral-50/70 dark:bg-[var(--theme-layer-2)]"
        >
          <slot name="header" :close="close" />
        </div>

        <!-- Body Slot -->
        <div class="p-3">
          <slot :close="close" />
        </div>

        <!-- Optional Footer Slot -->
        <div
          v-if="$slots.footer"
          class="px-4 py-2 border-t border-neutral-200/80 dark:border-[var(--theme-border)] bg-neutral-50/70 dark:bg-[var(--theme-layer-2)]"
        >
          <slot name="footer" :close="close" />
        </div>
      </div>
    </Transition>
  </div>
</template>

