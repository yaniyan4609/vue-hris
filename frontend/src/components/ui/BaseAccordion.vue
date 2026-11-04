<script setup>
import { ref, computed, watch } from 'vue'
import { ChevronDown } from 'lucide-vue-next'
import BaseBadge from './BaseBadge.vue'

const props = defineProps({
  items: {
    type: Array,
    required: true,
    // Each item: { id, title, subtitle?, content?, icon?, badge?, disabled? }
  },
  modelValue: {
    type: [Array, String, Number],
    default: undefined,
  },
  multiple: {
    type: Boolean,
    default: false,
  },
  defaultOpen: {
    type: [Array, String, Number],
    default: () => [],
  },
})

const emit = defineEmits(['update:modelValue', 'change'])

const normalizeToSet = (val) => {
  if (Array.isArray(val)) return new Set(val)
  if (val !== undefined && val !== null && val !== '') return new Set([val])
  return new Set()
}

const internalOpenSet = ref(
  props.modelValue !== undefined ? normalizeToSet(props.modelValue) : normalizeToSet(props.defaultOpen)
)

watch(
  () => props.modelValue,
  (val) => {
    if (val !== undefined) {
      internalOpenSet.value = normalizeToSet(val)
    }
  }
)

const isOpen = (id) => internalOpenSet.value.has(id)

const toggleItem = (item) => {
  if (item.disabled) return
  const next = new Set(internalOpenSet.value)
  if (next.has(item.id)) {
    next.delete(item.id)
  } else {
    if (!props.multiple) next.clear()
    next.add(item.id)
  }
  internalOpenSet.value = next
  const out = props.multiple ? Array.from(next) : (Array.from(next)[0] ?? '')
  emit('update:modelValue', out)
  emit('change', out)
}
</script>

<template>
  <div class="space-y-2 w-full">
    <div
      v-for="item in items"
      :key="String(item.id)"
      class="rounded-2xl border border-neutral-200 dark:border-[var(--theme-border)] bg-white dark:bg-[var(--theme-surface)] shadow-2xs overflow-hidden transition-colors"
    >
      <!-- Header Button -->
      <button
        type="button"
        :disabled="item.disabled"
        :aria-expanded="isOpen(item.id)"
        @click="toggleItem(item)"
        class="w-full px-4 py-3 flex items-center justify-between gap-3 text-left transition select-none"
        :class="[
          item.disabled
            ? 'opacity-40 cursor-not-allowed'
            : 'cursor-pointer hover:bg-neutral-50/80 dark:hover:bg-[var(--theme-surface-raised)]',
        ]"
      >
        <div class="flex items-center gap-2.5 min-w-0 flex-1">
          <div
            v-if="item.icon"
            class="w-7 h-7 rounded-xl bg-neutral-100 dark:bg-[var(--theme-surface-muted)] flex items-center justify-center shrink-0 text-neutral-600 dark:text-neutral-300"
          >
            <component :is="item.icon" class="w-3.5 h-3.5" />
          </div>
          <div class="min-w-0 flex-1">
            <div class="flex items-center gap-2">
              <span class="text-xs font-bold text-neutral-900 dark:text-white truncate">
                {{ item.title }}
              </span>
              <BaseBadge v-if="item.badge" variant="neutral">{{ item.badge }}</BaseBadge>
            </div>
            <p v-if="item.subtitle" class="text-[11px] text-neutral-500 dark:text-neutral-400 truncate mt-0.5">
              {{ item.subtitle }}
            </p>
          </div>
        </div>

        <ChevronDown
          class="w-4 h-4 text-neutral-400 transition-transform duration-200 shrink-0"
          :class="isOpen(item.id) ? 'rotate-180 text-neutral-900 dark:text-white' : ''"
        />
      </button>

      <!-- Collapsible Body -->
      <div
        v-if="isOpen(item.id)"
        class="px-4 pb-3.5 pt-2 border-t border-neutral-200/70 dark:border-[var(--theme-border)] text-xs text-neutral-600 dark:text-neutral-300 leading-relaxed bg-neutral-50/40 dark:bg-[var(--theme-layer-2)]/40"
      >
        <slot :name="`item-${item.id}`" :item="item">
          <slot name="content" :item="item">
            {{ item.content }}
          </slot>
        </slot>
      </div>
    </div>
  </div>
</template>

