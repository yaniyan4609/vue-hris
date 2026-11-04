<script setup>
import { ref, computed, watch } from 'vue'
import { ChevronRight, Folder, FileText } from 'lucide-vue-next'
import BaseCheckbox from './BaseCheckbox.vue'

const props = defineProps({
  nodes: {
    type: Array,
    default: () => [],
    // Each node: { id, label, badge?, icon?, children?: [] }
  },
  modelValue: {
    type: [Array, String, Number],
    default: () => [],
  },
  selectable: {
    type: Boolean,
    default: false,
  },
  multiple: {
    type: Boolean,
    default: false,
  },
  cascade: {
    type: Boolean,
    default: true,
  },
  searchQuery: {
    type: String,
    default: '',
  },
  defaultExpandAll: {
    type: Boolean,
    default: true,
  },
})

const emit = defineEmits(['update:modelValue', 'select', 'toggle'])

const collectAllParentIds = (nodes, acc = []) => {
  for (const n of nodes) {
    if (n.children && n.children.length > 0) {
      acc.push(n.id)
      collectAllParentIds(n.children, acc)
    }
  }
  return acc
}

const expandedIds = ref(new Set(props.defaultExpandAll ? collectAllParentIds(props.nodes) : []))

watch(
  () => props.nodes,
  (newNodes) => {
    if (props.defaultExpandAll) {
      expandedIds.value = new Set(collectAllParentIds(newNodes))
    }
  }
)

const toggleExpand = (node) => {
  if (!node.children || node.children.length === 0) return
  const next = new Set(expandedIds.value)
  if (next.has(node.id)) {
    next.delete(node.id)
  } else {
    next.add(node.id)
  }
  expandedIds.value = next
  emit('toggle', node)
}

const getDescendantIds = (node, acc = []) => {
  acc.push(node.id)
  if (node.children && node.children.length > 0) {
    for (const child of node.children) {
      getDescendantIds(child, acc)
    }
  }
  return acc
}

const selectedSet = computed(() => {
  if (Array.isArray(props.modelValue)) return new Set(props.modelValue)
  if (props.modelValue !== null && props.modelValue !== undefined && props.modelValue !== '') {
    return new Set([props.modelValue])
  }
  return new Set()
})

const isNodeChecked = (node) => {
  if (!props.multiple || !props.cascade || !node.children || node.children.length === 0) {
    return selectedSet.value.has(node.id)
  }
  const allIds = getDescendantIds(node)
  return allIds.every((id) => selectedSet.value.has(id))
}

const isNodeIndeterminate = (node) => {
  if (!props.multiple || !props.cascade || !node.children || node.children.length === 0) {
    return false
  }
  const allIds = getDescendantIds(node)
  const count = allIds.filter((id) => selectedSet.value.has(id)).length
  return count > 0 && count < allIds.length
}

const selectNode = (node) => {
  if (!props.selectable) {
    toggleExpand(node)
    return
  }

  if (props.multiple) {
    const next = new Set(selectedSet.value)
    const idsToToggle = props.cascade ? getDescendantIds(node) : [node.id]
    const shouldCheck = !isNodeChecked(node)

    for (const id of idsToToggle) {
      if (shouldCheck) next.add(id)
      else next.delete(id)
    }

    const arr = Array.from(next)
    emit('update:modelValue', arr)
    emit('select', node)
  } else {
    emit('update:modelValue', node.id)
    emit('select', node)
  }
}

// Flatten visible nodes with depth & search filtering
const nodeMatchesQuery = (node, q) => {
  if (!q) return true
  if (String(node.label || '').toLowerCase().includes(q)) return true
  if (node.children && node.children.length > 0) {
    return node.children.some((c) => nodeMatchesQuery(c, q))
  }
  return false
}

const visibleRows = computed(() => {
  const q = props.searchQuery.trim().toLowerCase()
  const rows = []

  const walk = (list, depth) => {
    for (const node of list) {
      if (!nodeMatchesQuery(node, q)) continue
      const hasChildren = Boolean(node.children && node.children.length > 0)
      const isExpanded = q ? true : expandedIds.value.has(node.id)
      rows.push({
        node,
        depth,
        hasChildren,
        isExpanded,
      })
      if (hasChildren && isExpanded) {
        walk(node.children, depth + 1)
      }
    }
  }

  walk(props.nodes, 0)
  return rows
})
</script>

<template>
  <div role="tree" class="space-y-0.5 select-none">
    <div
      v-for="row in visibleRows"
      :key="String(row.node.id)"
      role="treeitem"
      :aria-expanded="row.hasChildren ? row.isExpanded : undefined"
      :style="{ paddingLeft: `${row.depth * 18 + 6}px` }"
      class="flex items-center justify-between gap-2 py-1.5 pr-2.5 rounded-xl text-xs transition cursor-pointer"
      :class="[
        !multiple && selectable && selectedSet.has(row.node.id)
          ? 'bg-[var(--theme-primary-soft)] text-[var(--theme-primary-soft-text)] font-bold'
          : 'text-neutral-700 dark:text-neutral-300 hover:bg-neutral-100/80 dark:hover:bg-[var(--theme-surface-muted)] hover:text-neutral-900 dark:hover:text-white',
      ]"
      @click="selectNode(row.node)"
    >
      <div class="flex items-center gap-2 min-w-0 flex-1">
        <!-- Expand Chevron -->
        <button
          v-if="row.hasChildren"
          type="button"
          @click.stop="toggleExpand(row.node)"
          class="w-4 h-4 rounded-md flex items-center justify-center text-neutral-400 hover:text-neutral-700 dark:hover:text-neutral-200 hover:bg-neutral-200/60 dark:hover:bg-[var(--theme-surface-raised)] transition cursor-pointer shrink-0"
        >
          <ChevronRight
            class="w-3.5 h-3.5 transition-transform duration-150"
            :class="row.isExpanded ? 'rotate-90 text-neutral-800 dark:text-white' : ''"
          />
        </button>
        <span v-else class="w-4 shrink-0" />

        <!-- Checkbox when multiple selectable -->
        <div v-if="selectable && multiple" class="pointer-events-none shrink-0">
          <BaseCheckbox
            :model-value="isNodeChecked(row.node)"
            :indeterminate="isNodeIndeterminate(row.node)"
            size="sm"
          />
        </div>

        <!-- Node Icon -->
        <component
          :is="row.node.icon || (row.hasChildren ? Folder : FileText)"
          class="w-3.5 h-3.5 shrink-0"
          :class="row.hasChildren ? 'text-amber-500 dark:text-amber-400' : 'text-neutral-400'"
        />

        <!-- Label -->
        <span class="truncate font-medium">{{ row.node.label }}</span>
      </div>

      <!-- Optional Badge -->
      <span
        v-if="row.node.badge !== undefined && row.node.badge !== null"
        class="text-[10px] font-semibold px-1.5 py-0.5 rounded-full bg-neutral-100 dark:bg-[var(--theme-surface-muted)] text-neutral-500 dark:text-neutral-400 border border-neutral-200/60 dark:border-[var(--theme-border-strong)] shrink-0"
      >
        {{ row.node.badge }}
      </span>
    </div>

    <div
      v-if="visibleRows.length === 0"
      class="py-4 text-center text-xs text-neutral-400 dark:text-neutral-500"
    >
      Tidak ada struktur yang cocok.
    </div>
  </div>
</template>

