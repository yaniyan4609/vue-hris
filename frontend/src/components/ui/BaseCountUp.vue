<script setup>
import { ref, computed, watch, onMounted, onUnmounted } from 'vue'

const props = defineProps({
  to: {
    type: Number,
    required: true,
  },
  from: {
    type: Number,
    default: 0,
  },
  duration: {
    type: Number,
    default: 1200, // ms
  },
  decimals: {
    type: Number,
    default: 0,
  },
  prefix: {
    type: String,
    default: '',
  },
  suffix: {
    type: String,
    default: '',
  },
  formatNumber: {
    type: Boolean,
    default: true,
  },
  autoplay: {
    type: Boolean,
    default: true,
  },
})

const currentValue = ref(props.from)
let animationFrameId = null
let startTime = null

// Ease Out Cubic function (start fast, settle smoothly)
const easeOutCubic = (t) => 1 - Math.pow(1 - t, 3)

const formattedValue = computed(() => {
  const val = currentValue.value
  let formatted = ''

  if (props.formatNumber) {
    if (props.decimals > 0) {
      formatted = val.toLocaleString('id-ID', {
        minimumFractionDigits: props.decimals,
        maximumFractionDigits: props.decimals,
      })
    } else {
      formatted = Math.round(val).toLocaleString('id-ID')
    }
  } else {
    formatted = props.decimals > 0 ? val.toFixed(props.decimals) : Math.round(val).toString()
  }

  return `${props.prefix}${formatted}${props.suffix}`
})

const startAnimation = (startVal = props.from, endVal = props.to) => {
  if (animationFrameId) {
    cancelAnimationFrame(animationFrameId)
  }

  const duration = props.duration
  if (duration <= 0) {
    currentValue.value = endVal
    return
  }

  startTime = null

  const step = (timestamp) => {
    if (!startTime) startTime = timestamp
    const progress = Math.min((timestamp - startTime) / duration, 1)
    const easedProgress = easeOutCubic(progress)

    currentValue.value = startVal + (endVal - startVal) * easedProgress

    if (progress < 1) {
      animationFrameId = requestAnimationFrame(step)
    } else {
      currentValue.value = endVal
      animationFrameId = null
    }
  }

  animationFrameId = requestAnimationFrame(step)
}

const restart = () => {
  startAnimation(props.from, props.to)
}

watch(
  () => props.to,
  (newVal, oldVal) => {
    startAnimation(oldVal ?? props.from, newVal)
  }
)

onMounted(() => {
  if (props.autoplay) {
    startAnimation()
  } else {
    currentValue.value = props.to
  }
})

onUnmounted(() => {
  if (animationFrameId) {
    cancelAnimationFrame(animationFrameId)
  }
})

defineExpose({
  restart,
})
</script>

<template>
  <span class="inline-block tabular-nums">{{ formattedValue }}</span>
</template>

