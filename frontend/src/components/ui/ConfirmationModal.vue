<script setup>
import { computed, watch, onMounted, onUnmounted } from 'vue'
import { X } from 'lucide-vue-next'

const props = defineProps({
  modelValue: {
    type: Boolean,
    default: false,
  },
  type: {
    type: String,
    default: 'save', // 'save' | 'delete' | 'warning' | 'success'
    validator: (v) => ['save', 'delete', 'warning', 'success'].includes(v),
  },
  badgeText: {
    type: String,
    default: '',
  },
  title: {
    type: String,
    default: '',
  },
  message: {
    type: String,
    default: '',
  },
  confirmText: {
    type: String,
    default: '',
  },
  cancelText: {
    type: String,
    default: 'Batal',
  },
})

const emit = defineEmits(['update:modelValue', 'confirm', 'cancel', 'close'])

const close = () => {
  emit('update:modelValue', false)
  emit('close')
}

const handleCancel = () => {
  emit('cancel')
  close()
}

const handleConfirm = () => {
  emit('confirm')
  close()
}

const defaultBadge = computed(() => {
  if (props.badgeText) return props.badgeText
  if (props.type === 'save' || props.type === 'success') return 'Confirmation — Simpan'
  if (props.type === 'delete') return 'Confirmation — Delete'
  return 'Alert Confirmation'
})

const defaultTitle = computed(() => {
  if (props.title) return props.title
  if (props.type === 'save' || props.type === 'success') return 'Apakah Anda yakin ingin menyimpan perubahan?'
  if (props.type === 'delete') return 'Apakah Anda yakin ingin menghapus data ini?'
  return 'Konfirmasi Tindakan'
})

const defaultMessage = computed(() => {
  if (props.message) return props.message
  if (props.type === 'save' || props.type === 'success') return 'Pastikan semua perubahan data yang Anda masukkan sudah benar sebelum disimpan ke sistem.'
  if (props.type === 'delete') return 'Data yang dihapus tidak dapat dikembalikan. Pastikan Anda sudah memeriksa kembali.'
  return 'Harap periksa kembali detail sebelum melanjutkan.'
})

const defaultConfirmText = computed(() => {
  if (props.confirmText) return props.confirmText
  if (props.type === 'save' || props.type === 'success') return 'Simpan'
  if (props.type === 'delete') return 'Ya, Hapus'
  return 'Lanjutkan'
})

const iconColorClass = computed(() => {
  if (props.type === 'delete') {
    return 'text-rose-500 dark:text-rose-400'
  }
  if (props.type === 'save') {
    return 'text-blue-600 dark:text-blue-400'
  }
  if (props.type === 'success') {
    return 'text-emerald-500 dark:text-emerald-400'
  }
  return 'text-amber-500 dark:text-amber-400'
})

const confirmButtonClass = computed(() => {
  if (props.type === 'delete') {
    return 'bg-rose-500 hover:bg-rose-600 text-white shadow-sm shadow-rose-500/25 dark:bg-rose-600 dark:hover:bg-rose-500'
  }
  if (props.type === 'save' || props.type === 'success') {
    return 'bg-blue-600 hover:bg-blue-700 text-white shadow-sm shadow-blue-500/25 dark:bg-blue-600 dark:hover:bg-blue-500'
  }
  return 'bg-neutral-900 hover:bg-neutral-800 text-white dark:bg-white dark:text-neutral-900'
})

watch(
  () => props.modelValue,
  (val) => {
    if (val) {
      document.body.style.overflow = 'hidden'
    } else {
      document.body.style.overflow = ''
    }
  }
)

const handleKeydown = (e) => {
  if (e.key === 'Escape' && props.modelValue) {
    close()
  }
}

onMounted(() => {
  window.addEventListener('keydown', handleKeydown)
})

onUnmounted(() => {
  window.removeEventListener('keydown', handleKeydown)
  document.body.style.overflow = ''
})
</script>

<template>
  <Teleport to="body">
    <Transition
      enter-active-class="transition duration-200 ease-out"
      enter-from-class="opacity-0"
      enter-to-class="opacity-100"
      leave-active-class="transition duration-150 ease-in"
      leave-from-class="opacity-100"
      leave-to-class="opacity-0"
    >
      <div
        v-if="modelValue"
        class="fixed inset-0 z-50 flex items-center justify-center p-4 bg-black/60 backdrop-blur-xs select-none"
        @click.self="close"
        role="dialog"
        aria-modal="true"
      >
        <Transition
          enter-active-class="transition duration-250 cubic-bezier(0.16, 1, 0.3, 1)"
          enter-from-class="opacity-0 scale-90 translate-y-3"
          enter-to-class="opacity-100 scale-100 translate-y-0"
          leave-active-class="transition duration-150 ease-in"
          leave-from-class="opacity-100 scale-100 translate-y-0"
          leave-to-class="opacity-0 scale-95 translate-y-2"
        >
          <div
            v-if="modelValue"
            class="relative w-full max-w-sm rounded-3xl bg-white dark:bg-[var(--theme-surface)] border border-neutral-200 dark:border-[var(--theme-border)] shadow-2xl p-6 sm:p-7 flex flex-col items-center text-center overflow-hidden"
          >
            <!-- Top Bar: Badge & Close Button (Sesuai gambar referensi) -->
            <div class="w-full flex items-center justify-between mb-3">
              <span class="text-[11px] font-semibold text-neutral-600 dark:text-neutral-300 bg-neutral-100 dark:bg-[var(--theme-surface-muted)] px-3 py-1 rounded-full border border-neutral-200/80 dark:border-[var(--theme-border-strong)]">
                {{ defaultBadge }}
              </span>

              <button
                type="button"
                @click="close"
                class="rounded-full p-1 text-neutral-400 hover:text-neutral-700 hover:bg-neutral-100 dark:hover:bg-[var(--theme-surface-muted)] dark:hover:text-neutral-200 transition cursor-pointer"
                aria-label="Tutup"
              >
                <X class="w-4 h-4" />
              </button>
            </div>

            <!-- Clean Minimal Icon Badge with Cloud Halo & 3 Radiating Sparkles (Sesuai Gambar Referensi) -->
            <div class="my-3 flex items-center justify-center relative w-32 h-32">
              <svg
                viewBox="0 0 120 120"
                class="w-full h-full select-none"
                :class="iconColorClass"
                fill="none"
                xmlns="http://www.w3.org/2000/svg"
              >
                <!-- Cloud Halo (Latar Belakang Halus Organik) -->
                <g fill="currentColor" fill-opacity="0.06" class="dark:fill-opacity-10">
                  <ellipse cx="32" cy="68" rx="22" ry="17" />
                  <ellipse cx="80" cy="68" rx="24" ry="18" />
                  <ellipse cx="50" cy="74" rx="20" ry="14" />
                </g>

                <!-- Lingkaran Utama Badge -->
                <circle
                  cx="56"
                  cy="62"
                  r="34"
                  fill="currentColor"
                  fill-opacity="0.14"
                  class="dark:fill-opacity-20"
                />

                <!-- 3 Radiating Sparkle Lines di Pojok Kanan Atas (Persis Gambar Referensi) -->
                <g stroke="currentColor" stroke-width="3.2" stroke-linecap="round">
                  <!-- Garis 1 (Paling Atas) -->
                  <line x1="86" y1="28" x2="90" y2="18" />
                  <!-- Garis 2 (Tengah) -->
                  <line x1="91" y1="35" x2="102" y2="26" />
                  <!-- Garis 3 (Bawah) -->
                  <line x1="93" y1="44" x2="104" y2="44" />
                </g>

                <!-- 1. SAVE ICON: Disket Simpan Garis Tebal Membulat Modern (Confirmation Simpan) -->
                <g v-if="type === 'save'" stroke="currentColor" stroke-linecap="round" stroke-linejoin="round">
                  <!-- Badan Disket / Save Frame dengan Sudut Terpotong Halus di Kanan Atas -->
                  <path
                    d="M42 45 H64 L72 53 V72 C72 75.5 69.5 78 66 78 H46 C42.5 78 40 75.5 40 72 V47 C40 45.9 40.9 45 42 45 Z"
                    stroke-width="3.2"
                    fill="none"
                  />
                  <!-- Slider Shutter Atas -->
                  <path
                    d="M47 45 V54 C47 55.5 48.2 56.5 49.8 56.5 H61.2 C62.8 56.5 64 55.5 64 54 V47"
                    stroke-width="2.8"
                    fill="none"
                  />
                  <line x1="57" y1="48" x2="57" y2="52.5" stroke-width="2.4" />
                  <!-- Garis Label Data di Bagian Bawah -->
                  <line x1="47" y1="65" x2="65" y2="65" stroke-width="2.8" />
                  <line x1="47" y1="71" x2="58" y2="71" stroke-width="2.4" opacity="0.7" />
                </g>

                <!-- 2. DELETE ICON: Tempat Sampah Garis Tebal Membulat -->
                <g v-else-if="type === 'delete'" stroke="currentColor" stroke-linecap="round" stroke-linejoin="round">
                  <!-- Handle / Gagang Tutup Atas -->
                  <path
                    d="M50 48 V43 C50 41.5 51.5 40 53 40 H59 C60.5 40 62 41.5 62 43 V48"
                    stroke-width="3.2"
                    fill="none"
                  />
                  <!-- Tutup Garis Horizontal -->
                  <line x1="41" y1="48" x2="71" y2="48" stroke-width="3.2" />
                  <!-- Badan Tempat Sampah (Memipih sedikit ke bawah dengan sudut melengkung) -->
                  <path
                    d="M44 48 L46.8 72.8 C47.2 75.6 49.5 77.8 52.3 77.8 H59.7 C62.5 77.8 64.8 75.6 65.2 72.8 L68 48"
                    stroke-width="3.2"
                    fill="none"
                  />
                  <!-- 2 Garis Vertikal di Dalam -->
                  <line x1="51.5" y1="56" x2="51.5" y2="69" stroke-width="3.2" />
                  <line x1="60.5" y1="56" x2="60.5" y2="69" stroke-width="3.2" />
                </g>

                <!-- 3. SUCCESS ICON: Centang Tebal Membulat Modern -->
                <g v-else-if="type === 'success'" stroke="currentColor" stroke-linecap="round" stroke-linejoin="round">
                  <path
                    d="M43 63 L52.5 72.5 L69 49"
                    stroke-width="4.2"
                    fill="none"
                  />
                </g>

                <!-- 4. WARNING / FALLBACK ICON: Segitiga Peringatan Membulat -->
                <g v-else stroke="currentColor" stroke-linecap="round" stroke-linejoin="round">
                  <path
                    d="M56 43 C57.2 43 58.2 43.8 58.8 45 L73.5 71.5 C74.2 73 73.2 75 71.5 75 H40.5 C38.8 75 37.8 73 38.5 71.5 L53.2 45 C53.8 43.8 54.8 43 56 43 Z"
                    stroke-width="3.2"
                    fill="none"
                  />
                  <line x1="56" y1="53" x2="56" y2="62" stroke-width="3.2" />
                  <circle cx="56" cy="68.5" r="1.6" fill="currentColor" stroke="none" />
                </g>
              </svg>
            </div>

            <!-- Title & Description (Sesuai gambar referensi) -->
            <div class="space-y-1.5 my-2">
              <h3 class="text-base sm:text-lg font-bold text-neutral-900 dark:text-[var(--theme-text-primary)] leading-snug">
                {{ defaultTitle }}
              </h3>
              <p class="text-xs text-neutral-500 dark:text-neutral-400 leading-relaxed px-1">
                {{ defaultMessage }}
              </p>
            </div>

            <!-- Footer Buttons: Batal & Confirm (Simpan / Ya, Hapus) -->
            <div class="w-full grid grid-cols-2 gap-3 mt-5">
              <button
                type="button"
                @click="handleCancel"
                class="w-full py-2.5 px-4 rounded-xl text-xs sm:text-sm font-semibold border border-neutral-200 dark:border-[var(--theme-border)] bg-neutral-50 hover:bg-neutral-100 dark:bg-[var(--theme-surface-raised)] dark:hover:bg-[var(--theme-surface-muted)] text-neutral-700 dark:text-neutral-200 transition cursor-pointer"
              >
                {{ cancelText }}
              </button>

              <button
                type="button"
                @click="handleConfirm"
                :class="[
                  'w-full py-2.5 px-4 rounded-xl text-xs sm:text-sm font-semibold transition cursor-pointer active:scale-[0.98]',
                  confirmButtonClass,
                ]"
              >
                {{ defaultConfirmText }}
              </button>
            </div>

          </div>
        </Transition>
      </div>
    </Transition>
  </Teleport>
</template>

