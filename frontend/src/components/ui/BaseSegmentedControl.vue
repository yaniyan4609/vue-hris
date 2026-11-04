<script setup>
import { ref, computed, watch, onMounted, onUnmounted, nextTick } from 'vue'

const props = defineProps({
  modelValue: {
    type: [String, Number],
    required: true,
  },
  options: {
    type: Array,
    required: true,
    // Array of strings or { label: string, value: any, icon?: any, badge?: string|number }
  },
  size: {
    type: String,
    default: 'md', // 'sm' | 'md'
  },
  fullWidth: {
    type: Boolean,
    default: false,
  },
})

const emit = defineEmits(['update:modelValue', 'change'])

const containerRef = ref(null)
const itemRefs = ref([])
const indicatorStyle = ref({
  width: '0px',
  height: '0px',
  transform: 'translate3d(0, 0, 0)',
  opacity: 0,
})
const isReady = ref(false)
let resizeObserver = null

const normalizedOptions = computed(() => {
  return props.options.map((opt) => {
    if (typeof opt === 'object' && opt !== null) {
      return {
        label: opt.label ?? String(opt.value),
        value: opt.value ?? opt.label,
        icon: opt.icon ?? null,
        badge: opt.badge ?? null,
      }
    }
    return {
      label: String(opt),
      value: opt,
      icon: null,
      badge: null,
    }
  })
})

const activeIndex = computed(() => {
  return normalizedOptions.value.findIndex((o) => o.value === props.modelValue)
})

const updateIndicator = () => {
  if (activeIndex.value === -1 || !itemRefs.value[activeIndex.value]) {
    indicatorStyle.value.opacity = 0
    return
  }

  const el = itemRefs.value[activeIndex.value]
  const container = containerRef.value
  if (!el || !container) return

  const containerRect = container.getBoundingClientRect()
  const elRect = el.getBoundingClientRect()

  const left = elRect.left - containerRect.left
  const top = elRect.top - containerRect.top

  indicatorStyle.value = {
    width: `${elRect.width}px`,
    height: `${elRect.height}px`,
    transform: `translate3d(${left}px, ${top}px, 0)`,
    opacity: 1,
  }
}

const select = (opt) => {
  emit('update:modelValue', opt.value)
  emit('change', opt.value)
}

watch(
  () => props.modelValue,
  () => {
    nextTick(updateIndicator)
  }
)

onMounted(() => {
  nextTick(() => {
    updateIndicator()
    setTimeout(() => {
      isReady.value = true
    }, 50)

    if (window.ResizeObserver && containerRef.value) {
      resizeObserver = new ResizeObserver(() => {
        updateIndicator()
      })
      resizeObserver.observe(containerRef.value)
    }
  })
})

onUnmounted(() => {
  if (resizeObserver) {
    resizeObserver.disconnect()
  }
})

const sizeClasses = computed(() => {
  const map = {
    sm: 'p-0.5 text-xs rounded-xl',
    md: 'p-1 text-xs rounded-xl',
  }
  return map[props.size] || map.md
})

const itemSizeClasses = computed(() => {
  const map = {
    sm: 'py-1 px-2.5 rounded-lg gap-1.5',
    md: 'py-1.5 px-3 rounded-lg gap-2',
  }
  return map[props.size] || map.md
})
</script>

<template>
  <div
    ref="containerRef"
    class="relative inline-flex items-center bg-neutral-100 dark:bg-[var(--theme-surface-raised)] border border-neutral-200/80 dark:border-[var(--theme-border)] shadow-2xs select-none"
    :class="[sizeClasses, fullWidth ? 'w-full' : '']"
    role="tablist"
  >
    <!-- Fluid Gliding Pill Indicator -->
    <div
      class="absolute left-0 top-0 rounded-lg bg-white dark:bg-[var(--theme-surface-muted)] border border-neutral-300/80 dark:border-[var(--theme-border-strong)] shadow-sm pointer-events-none"
      :class="isReady ? 'transition-all duration-200 cubic-bezier(0.16, 1, 0.3, 1)' : ''"
      :style="indicatorStyle"
    />

    <!-- Option Buttons -->
    <button
      v-for="(opt, idx) in normalizedOptions"
      :key="opt.value"
      :ref="(el) => (itemRefs[idx] = el)"
      type="button"
      role="tab"
      :aria-selected="modelValue === opt.value"
      @click="select(opt)"
      class="relative z-10 font-semibold transition-colors flex items-center justify-center cursor-pointer outline-none"
      :class="[
        itemSizeClasses,
        fullWidth ? 'flex-1' : '',
        modelValue === opt.value
          ? 'text-neutral-900 dark:text-white'
          : 'text-neutral-500 dark:text-neutral-400 hover:text-neutral-800 dark:hover:text-neutral-200',
      ]"
    >
      <component :is="opt.icon" v-if="opt.icon" class="w-3.5 h-3.5 shrink-0" />
      <span class="truncate">{{ opt.label }}</span>
      <span
        v-if="opt.badge !== null"
        class="text-[9.5px] px-1.5 py-0.2 rounded-full font-mono bg-neutral-200/80 dark:bg-[var(--theme-surface-muted)] text-neutral-600 dark:text-neutral-300 ml-1"
      >
        {{ opt.badge }}
      </span>
    </button>
  </div>
</template>

