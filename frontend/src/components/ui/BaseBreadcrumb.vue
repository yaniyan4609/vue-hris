<script setup>
import { ChevronRight, Home } from 'lucide-vue-next'
import { useRouter } from 'vue-router'

const props = defineProps({
  items: {
    type: Array,
    required: true,
    // Each item: { label, to?, icon?, active? }
  },
  showHome: {
    type: Boolean,
    default: true,
  },
})

const emit = defineEmits(['navigate'])
const router = useRouter()

const handleClick = (item, idx) => {
  const isLast = idx === props.items.length - 1
  if (isLast || item.active) return
  emit('navigate', item)
  if (item.to) {
    router.push(item.to)
  }
}
</script>

<template>
  <nav aria-label="Breadcrumb" class="flex items-center overflow-x-auto no-scrollbar select-none">
    <ol class="inline-flex items-center gap-1.5 text-xs whitespace-nowrap">
      <!-- Optional Home Crumb -->
      <li v-if="showHome" class="inline-flex items-center gap-1.5">
        <button
          type="button"
          @click="router.push('/')"
          class="inline-flex items-center gap-1 text-neutral-500 dark:text-neutral-400 hover:text-neutral-900 dark:hover:text-white transition cursor-pointer"
          title="Beranda"
        >
          <Home class="w-3.5 h-3.5" />
        </button>
        <ChevronRight v-if="items.length > 0" class="w-3.5 h-3.5 text-neutral-300 dark:text-neutral-600 shrink-0" />
      </li>

      <li
        v-for="(item, idx) in items"
        :key="idx"
        class="inline-flex items-center gap-1.5"
      >
        <button
          type="button"
          :disabled="idx === items.length - 1 || item.active"
          @click="handleClick(item, idx)"
          class="inline-flex items-center gap-1.5 transition"
          :class="
            idx === items.length - 1 || item.active
              ? 'font-bold text-neutral-900 dark:text-white cursor-default'
              : 'font-medium text-neutral-500 dark:text-neutral-400 hover:text-neutral-900 dark:hover:text-white cursor-pointer'
          "
        >
          <component :is="item.icon" v-if="item.icon" class="w-3.5 h-3.5 shrink-0" />
          <span>{{ item.label }}</span>
        </button>

        <ChevronRight
          v-if="idx < items.length - 1"
          class="w-3.5 h-3.5 text-neutral-300 dark:text-neutral-600 shrink-0"
        />
      </li>
    </ol>
  </nav>
</template>

