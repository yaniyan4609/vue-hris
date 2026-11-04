<script setup>
import { ref, computed, onMounted, onUnmounted } from 'vue'
import { Camera, RefreshCw, SwitchCamera, AlertTriangle } from 'lucide-vue-next'
import BaseButton from './BaseButton.vue'
import BaseAlert from './BaseAlert.vue'

const props = defineProps({
  modelValue: {
    type: String,
    default: '', // Base64 data URL
  },
  facingMode: {
    type: String,
    default: 'user', // 'user' | 'environment'
  },
  aspectRatio: {
    type: String,
    default: 'portrait', // 'square' | 'portrait' | 'landscape'
  },
  watermarkText: {
    type: String,
    default: '',
  },
  mirrored: {
    type: Boolean,
    default: true,
  },
  label: {
    type: String,
    default: 'Foto Selfie Presensi',
  },
})

const emit = defineEmits(['update:modelValue', 'capture', 'error', 'permission-denied'])

const videoRef = ref(null)
const canvasRef = ref(null)
const mediaStream = ref(null)
const currentFacingMode = ref(props.facingMode)
const isCameraActive = ref(false)
const permissionDenied = ref(false)
const errorMessage = ref('')

const aspectClasses = computed(() => {
  const map = {
    square: 'aspect-square',
    portrait: 'aspect-[3/4]',
    landscape: 'aspect-[4/3]',
  }
  return map[props.aspectRatio] || map.portrait
})

const stopStream = () => {
  if (mediaStream.value) {
    mediaStream.value.getTracks().forEach((track) => track.stop())
    mediaStream.value = null
  }
  isCameraActive.value = false
}

const startCamera = async () => {
  stopStream()
  permissionDenied.value = false
  errorMessage.value = ''

  if (!navigator?.mediaDevices?.getUserMedia) {
    permissionDenied.value = true
    errorMessage.value = 'Perangkat atau browser Anda tidak mendukung akses kamera.'
    emit('error', errorMessage.value)
    return
  }

  try {
    const stream = await navigator.mediaDevices.getUserMedia({
      video: {
        facingMode: currentFacingMode.value,
        width: { ideal: 1280 },
        height: { ideal: 720 },
      },
      audio: false,
    })
    mediaStream.value = stream
    if (videoRef.value) {
      videoRef.value.srcObject = stream
      await videoRef.value.play().catch(() => {})
    }
    isCameraActive.value = true
  } catch (err) {
    permissionDenied.value = true
    if (err.name === 'NotAllowedError' || err.name === 'PermissionDeniedError') {
      errorMessage.value = 'Izin kamera ditolak. Silakan izinkan akses kamera di pengaturan browser Anda.'
      emit('permission-denied', err)
    } else if (err.name === 'NotFoundError' || err.name === 'DevicesNotFoundError') {
      errorMessage.value = 'Kamera tidak ditemukan pada perangkat Anda.'
      emit('error', err)
    } else {
      errorMessage.value = 'Gagal menghubungkan ke kamera: ' + (err.message || err.name)
      emit('error', err)
    }
  }
}

const toggleCamera = () => {
  currentFacingMode.value = currentFacingMode.value === 'user' ? 'environment' : 'user'
  startCamera()
}

const capture = () => {
  if (!videoRef.value || !isCameraActive.value) return

  const video = videoRef.value
  const canvas = canvasRef.value || document.createElement('canvas')
  const width = video.videoWidth || 640
  const height = video.videoHeight || 480

  canvas.width = width
  canvas.height = height
  const ctx = canvas.getContext('2d')

  // Handle mirror when user facing
  if (props.mirrored && currentFacingMode.value === 'user') {
    ctx.translate(width, 0)
    ctx.scale(-1, 1)
  }

  ctx.drawImage(video, 0, 0, width, height)

  // Reset transform for watermark overlay
  ctx.setTransform(1, 0, 0, 1, 0, 0)

  // Draw Watermark (Timestamp + WatermarkText)
  const now = new Date()
  const timeStr = now.toLocaleDateString('id-ID', {
    day: '2-digit',
    month: 'short',
    year: 'numeric',
  }) + ' ' + now.toLocaleTimeString('id-ID', { hour: '2-digit', minute: '2-digit', second: '2-digit' })
  
  const textLines = [timeStr]
  if (props.watermarkText) {
    textLines.push(props.watermarkText)
  }

  // Watermark background box
  ctx.fillStyle = 'rgba(0, 0, 0, 0.55)'
  const boxHeight = textLines.length * 24 + 16
  ctx.fillRect(0, height - boxHeight, width, boxHeight)

  ctx.font = 'bold 15px -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, sans-serif'
  ctx.fillStyle = '#ffffff'
  ctx.textBaseline = 'middle'

  textLines.forEach((line, idx) => {
    ctx.fillText(line, 16, height - boxHeight + 16 + idx * 24)
  })

  const dataUrl = canvas.toDataURL('image/jpeg', 0.9)
  emit('update:modelValue', dataUrl)
  emit('capture', dataUrl)
  stopStream()
}

const retake = () => {
  emit('update:modelValue', '')
  startCamera()
}

onMounted(() => {
  if (!props.modelValue) {
    startCamera()
  }
})

onUnmounted(() => {
  stopStream()
})
</script>

<template>
  <div class="w-full space-y-2 select-none">
    <div v-if="label" class="flex items-center justify-between text-xs font-semibold text-neutral-700 dark:text-neutral-300">
      <span>{{ label }}</span>
      <span v-if="isCameraActive" class="inline-flex items-center gap-1.5 text-[11px] text-emerald-600 dark:text-emerald-400 font-bold">
        <span class="w-2 h-2 rounded-full bg-emerald-500 animate-pulse" />
        Kamera Siap
      </span>
    </div>

    <!-- Hidden Canvas for processing -->
    <canvas ref="canvasRef" class="hidden" />

    <!-- Permission Denied Alert Banner -->
    <BaseAlert
      v-if="permissionDenied"
      variant="danger"
      title="Izin Kamera Ditolak"
      :description="errorMessage"
      action-text="Coba Hubungkan Lagi"
      @action="startCamera"
    />

    <!-- Main Camera Frame -->
    <div
      v-else
      class="relative w-full rounded-2xl overflow-hidden bg-neutral-900 border border-neutral-300 dark:border-[var(--theme-border)] shadow-md flex items-center justify-center max-w-sm mx-auto"
      :class="aspectClasses"
    >
      <!-- Mode 1: Captured Image Display -->
      <template v-if="modelValue">
        <img
          :src="modelValue"
          alt="Hasil Foto"
          class="w-full h-full object-cover"
        />
        <div class="absolute bottom-3 left-1/2 -translate-x-1/2">
          <BaseButton variant="secondary" size="sm" @click="retake">
            <RefreshCw class="w-3.5 h-3.5" />
            <span>Foto Ulang</span>
          </BaseButton>
        </div>
      </template>

      <!-- Mode 2: Live Video Feed -->
      <template v-else>
        <video
          ref="videoRef"
          autoplay
          playsinline
          muted
          class="w-full h-full object-cover"
          :class="mirrored && currentFacingMode === 'user' ? '-scale-x-100' : ''"
        />

        <!-- Face Guide SVG Overlay -->
        <div class="absolute inset-0 pointer-events-none flex items-center justify-center">
          <svg
            viewBox="0 0 200 240"
            class="w-48 h-56 stroke-white/60 dark:stroke-white/70 fill-none"
            stroke-width="2"
            stroke-dasharray="6 4"
          >
            <ellipse cx="100" cy="115" rx="65" ry="85" />
          </svg>
          <span class="absolute top-4 px-2.5 py-1 rounded-full bg-black/50 backdrop-blur-xs text-[10.5px] font-semibold text-white">
            Posisikan Wajah di Dalam Garis
          </span>
        </div>

        <!-- Controls Bar -->
        <div class="absolute bottom-3 inset-x-3 flex items-center justify-between gap-2">
          <!-- Flip Camera Button -->
          <button
            type="button"
            @click="toggleCamera"
            class="p-2 rounded-xl bg-black/50 hover:bg-black/70 backdrop-blur-md text-white transition cursor-pointer"
            title="Ganti Kamera Depan/Belakang"
          >
            <SwitchCamera class="w-4 h-4" />
          </button>

          <!-- Shutter Button -->
          <button
            type="button"
            @click="capture"
            class="px-4 py-2 rounded-xl bg-[var(--theme-primary)] hover:opacity-95 text-[var(--theme-primary-text)] font-bold text-xs flex items-center gap-2 shadow-lg transition active:scale-95 cursor-pointer"
          >
            <Camera class="w-4 h-4" />
            <span>Ambil Foto</span>
          </button>

          <div class="w-8" />
        </div>
      </template>
    </div>
  </div>
</template>

