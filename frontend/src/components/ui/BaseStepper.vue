<script setup>
import { Check, X, Clock } from 'lucide-vue-next'

const props = defineProps({
  steps: {
    type: Array,
    required: true,
    // Each step: { id, title, subtitle?, status?: 'completed'|'current'|'upcoming'|'rejected', approver?, timestamp? }
  },
  currentStep: {
    type: Number,
    default: 0,
  },
  clickable: {
    type: Boolean,
    default: false,
  },
  size: {
    type: String,
    default: 'md', // 'sm' | 'md'
  },
})

const emit = defineEmits(['update:currentStep', 'step-click'])

const getStepStatus = (step, idx) => {
  if (step.status) return step.status
  if (idx < props.currentStep) return 'completed'
  if (idx === props.currentStep) return 'current'
  return 'upcoming'
}

const handleStepClick = (step, idx) => {
  if (!props.clickable) return
  emit('update:currentStep', idx)
  emit('step-click', { step, index: idx })
}
</script>

<template>
  <div class="w-full overflow-x-auto no-scrollbar">
    <ol class="flex items-start w-full min-w-[360px]">
      <li
        v-for="(step, idx) in steps"
        :key="step.id ?? idx"
        class="flex-1 flex flex-col relative"
      >
        <div class="flex items-center w-full">
          <!-- Step Indicator Circle -->
          <button
            type="button"
            :disabled="!clickable"
            @click="handleStepClick(step, idx)"
            class="relative z-10 inline-flex items-center justify-center rounded-xl font-bold border transition-all shrink-0"
            :class="[
              size === 'sm' ? 'w-7 h-7 text-[11px]' : 'w-8 h-8 text-xs',
              clickable ? 'cursor-pointer' : 'cursor-default',
              getStepStatus(step, idx) === 'completed'
                ? 'bg-emerald-500 border-emerald-500 text-white shadow-xs'
                : getStepStatus(step, idx) === 'rejected'
                ? 'bg-rose-500 border-rose-500 text-white shadow-xs'
                : getStepStatus(step, idx) === 'current'
                ? 'bg-[var(--theme-primary)] border-[var(--theme-primary)] text-[var(--theme-primary-text)] ring-4 ring-[var(--theme-primary-soft)]'
                : 'bg-white dark:bg-[var(--theme-surface-raised)] border-neutral-300 dark:border-[var(--theme-border-strong)] text-neutral-400',
            ]"
          >
            <Check v-if="getStepStatus(step, idx) === 'completed'" class="w-3.5 h-3.5 stroke-[3]" />
            <X v-else-if="getStepStatus(step, idx) === 'rejected'" class="w-3.5 h-3.5 stroke-[3]" />
            <Clock v-else-if="getStepStatus(step, idx) === 'current'" class="w-3.5 h-3.5" />
            <span v-else>{{ idx + 1 }}</span>
          </button>

          <!-- Connector Line -->
          <div
            v-if="idx < steps.length - 1"
            class="flex-1 h-0.5 mx-2 rounded-full transition-colors"
            :class="
              getStepStatus(step, idx) === 'completed'
                ? 'bg-emerald-500'
                : 'bg-neutral-200 dark:bg-[var(--theme-border-strong)]'
            "
          />
        </div>

        <!-- Step Labels -->
        <div class="mt-2 pr-3">
          <p
            class="text-xs font-bold leading-snug"
            :class="
              getStepStatus(step, idx) === 'upcoming'
                ? 'text-neutral-400 dark:text-neutral-500'
                : getStepStatus(step, idx) === 'rejected'
                ? 'text-rose-600 dark:text-rose-400'
                : 'text-neutral-900 dark:text-white'
            "
          >
            {{ step.title }}
          </p>
          <p v-if="step.subtitle || step.approver" class="text-[11px] text-neutral-500 dark:text-neutral-400 mt-0.5">
            {{ step.approver || step.subtitle }}
          </p>
          <p v-if="step.timestamp" class="text-[10px] font-mono text-neutral-400 dark:text-neutral-500 mt-0.5">
            {{ step.timestamp }}
          </p>
        </div>
      </li>
    </ol>
  </div>
</template>

