<script setup>
import { ref, computed } from 'vue'
import {
  UploadCloud,
  File,
  FileText,
  FileSpreadsheet,
  FileArchive,
  ImageIcon,
  X,
  CheckCircle2,
  AlertCircle,
} from 'lucide-vue-next'

const props = defineProps({
  modelValue: {
    type: [Array, Object],
    default: () => [],
  },
  label: {
    type: String,
    default: '',
  },
  hint: {
    type: String,
    default: '',
  },
  accept: {
    type: String,
    default: '*',
  },
  maxSizeMB: {
    type: Number,
    default: 5,
  },
  multiple: {
    type: Boolean,
    default: true,
  },
  disabled: {
    type: Boolean,
    default: false,
  },
})

const emit = defineEmits(['update:modelValue', 'change', 'error'])

const fileInputRef = ref(null)
const isDragging = ref(false)
const errorMessage = ref('')

// Internal file state
const fileList = ref([])

const formatBytes = (bytes) => {
  if (bytes === 0) return '0 B'
  const k = 1024
  const sizes = ['B', 'KB', 'MB', 'GB']
  const i = Math.floor(Math.log(bytes) / Math.log(k))
  return parseFloat((bytes / Math.pow(k, i)).toFixed(1)) + ' ' + sizes[i]
}

const getFileIcon = (file) => {
  const name = file.name.toLowerCase()
  if (file.type.startsWith('image/')) return ImageIcon
  if (name.endsWith('.pdf')) return FileText
  if (name.endsWith('.xlsx') || name.endsWith('.xls') || name.endsWith('.csv')) return FileSpreadsheet
  if (name.endsWith('.zip') || name.endsWith('.rar') || name.endsWith('.7z')) return FileArchive
  return File
}

const getFileIconColor = (file) => {
  const name = file.name.toLowerCase()
  if (file.type.startsWith('image/')) return 'text-purple-500 bg-purple-50 dark:bg-purple-950/40'
  if (name.endsWith('.pdf')) return 'text-rose-500 bg-rose-50 dark:bg-rose-950/40'
  if (name.endsWith('.xlsx') || name.endsWith('.xls') || name.endsWith('.csv')) return 'text-emerald-500 bg-emerald-50 dark:bg-emerald-950/40'
  if (name.endsWith('.zip') || name.endsWith('.rar')) return 'text-amber-500 bg-amber-50 dark:bg-amber-950/40'
  return 'text-neutral-500 bg-neutral-100 dark:bg-neutral-800'
}

const validateFile = (file) => {
  // Max size check
  const maxBytes = props.maxSizeMB * 1024 * 1024
  if (file.size > maxBytes) {
    return `Ukuran file "${file.name}" (${formatBytes(file.size)}) melebihi batas ${props.maxSizeMB} MB.`
  }

  // Accept check (if specified and not '*')
  if (props.accept && props.accept !== '*') {
    const acceptedTypes = props.accept.split(',').map((t) => t.trim().toLowerCase())
    const ext = '.' + file.name.split('.').pop().toLowerCase()
    const mime = file.type.toLowerCase()

    const matches = acceptedTypes.some((type) => {
      if (type.startsWith('.')) return ext === type
      if (type.endsWith('/*')) {
        const prefix = type.replace('/*', '')
        return mime.startsWith(prefix)
      }
      return mime === type
    })

    if (!matches) {
      return `Format file "${file.name}" tidak diizinkan. Diterima: ${props.accept}`
    }
  }

  return null
}

const processFiles = (files) => {
  errorMessage.value = ''
  const validFiles = []

  for (const file of files) {
    const err = validateFile(file)
    if (err) {
      errorMessage.value = err
      emit('error', err)
      return
    }

    const item = {
      id: Math.random().toString(36).substring(2, 9),
      file,
      name: file.name,
      size: formatBytes(file.size),
      rawSize: file.size,
      type: file.type,
      preview: file.type.startsWith('image/') ? URL.createObjectURL(file) : null,
      progress: 100,
    }
    validFiles.push(item)
  }

  if (props.multiple) {
    fileList.value.push(...validFiles)
  } else {
    fileList.value = validFiles.slice(0, 1)
  }

  emit('update:modelValue', fileList.value.map((f) => f.file))
  emit('change', fileList.value)
}

const handleFileSelect = (e) => {
  if (props.disabled) return
  const files = Array.from(e.target.files || [])
  if (files.length > 0) {
    processFiles(files)
  }
  // Reset input value so re-selecting same file triggers change
  if (fileInputRef.value) fileInputRef.value.value = ''
}

const handleDrop = (e) => {
  if (props.disabled) return
  isDragging.value = false
  const files = Array.from(e.dataTransfer.files || [])
  if (files.length > 0) {
    processFiles(files)
  }
}

const removeFile = (idx) => {
  const item = fileList.value[idx]
  if (item?.preview) {
    URL.revokeObjectURL(item.preview)
  }
  fileList.value.splice(idx, 1)
  emit('update:modelValue', fileList.value.map((f) => f.file))
  emit('change', fileList.value)
}

const triggerSelect = () => {
  if (props.disabled) return
  fileInputRef.value?.click()
}
</script>

<template>
  <div class="w-full space-y-2.5 select-none text-left">
    <!-- Label -->
    <div v-if="label" class="flex items-center justify-between">
      <label class="block text-xs font-semibold text-neutral-700 dark:text-neutral-300">
        {{ label }}
      </label>
      <span class="text-[10.5px] text-neutral-400">
        Maks. {{ maxSizeMB }} MB per berkas
      </span>
    </div>

    <!-- Drag & Drop Zone -->
    <div
      @dragover.prevent="!disabled && (isDragging = true)"
      @dragleave.prevent="!disabled && (isDragging = false)"
      @drop.prevent="handleDrop"
      @click="triggerSelect"
      class="relative w-full rounded-2xl border-2 border-dashed p-6 text-center transition-all duration-150 cursor-pointer flex flex-col items-center justify-center gap-2.5 group"
      :class="[
        disabled ? 'opacity-50 cursor-not-allowed border-neutral-300 dark:border-[var(--theme-border)] bg-neutral-100 dark:bg-[var(--theme-surface-raised)]' :
        isDragging
          ? 'border-[var(--theme-primary)] bg-[var(--theme-primary-soft)] scale-[1.01]'
          : 'border-neutral-300 dark:border-[var(--theme-border-strong)] bg-neutral-50/50 dark:bg-[var(--theme-surface)]/60 hover:border-neutral-400 dark:hover:border-neutral-600 hover:bg-neutral-50 dark:hover:bg-[var(--theme-surface-raised)]',
      ]"
    >
      <input
        ref="fileInputRef"
        type="file"
        class="hidden"
        :accept="accept"
        :multiple="multiple"
        :disabled="disabled"
        @change="handleFileSelect"
      />

      <!-- Icon Box -->
      <div
        class="w-12 h-12 rounded-2xl flex items-center justify-center transition-transform group-hover:scale-110 shadow-2xs"
        :class="isDragging ? 'bg-white dark:bg-[var(--theme-surface-muted)] text-[var(--theme-primary)]' : 'bg-white dark:bg-[var(--theme-surface-raised)] border border-neutral-200 dark:border-[var(--theme-border)] text-neutral-500 dark:text-neutral-400'"
      >
        <UploadCloud class="w-6 h-6 stroke-[1.8]" />
      </div>

      <!-- Text Instructions -->
      <div class="space-y-0.5">
        <p class="text-xs sm:text-sm font-bold text-neutral-800 dark:text-neutral-200">
          <span class="text-[var(--theme-primary)] group-hover:underline">Klik untuk unggah</span> atau seret berkas ke sini
        </p>
        <p class="text-[11px] text-neutral-400">
          {{ hint || (accept !== '*' ? `Format diizinkan: ${accept}` : 'Mendukung semua format berkas dokumen dan gambar') }}
        </p>
      </div>
    </div>

    <!-- Error Alert Box -->
    <div
      v-if="errorMessage"
      class="p-2.5 rounded-xl bg-rose-50 dark:bg-rose-950/40 border border-rose-200 dark:border-rose-900/50 text-xs text-rose-600 dark:text-rose-400 flex items-center gap-2"
    >
      <AlertCircle class="w-4 h-4 shrink-0" />
      <span>{{ errorMessage }}</span>
    </div>

    <!-- Uploaded File List Cards -->
    <div v-if="fileList.length > 0" class="space-y-2 pt-1">
      <div
        v-for="(item, idx) in fileList"
        :key="item.id"
        class="p-2.5 rounded-xl bg-white dark:bg-[var(--theme-surface-raised)] border border-neutral-200 dark:border-[var(--theme-border)] shadow-2xs flex items-center justify-between gap-3"
      >
        <!-- File Info Left -->
        <div class="flex items-center gap-2.5 min-w-0">
          <!-- Thumbnail Image or File Icon -->
          <div
            v-if="item.preview"
            class="w-10 h-10 rounded-lg overflow-hidden shrink-0 border border-neutral-200 dark:border-[var(--theme-border)] bg-neutral-100"
          >
            <img :src="item.preview" :alt="item.name" class="w-full h-full object-cover" />
          </div>
          <div
            v-else
            class="w-10 h-10 rounded-lg flex items-center justify-center shrink-0"
            :class="getFileIconColor(item)"
          >
            <component :is="getFileIcon(item)" class="w-5 h-5 stroke-[1.8]" />
          </div>

          <!-- Name & Size -->
          <div class="min-w-0">
            <h5 class="text-xs font-bold text-neutral-900 dark:text-white truncate">
              {{ item.name }}
            </h5>
            <div class="flex items-center gap-2 mt-0.5 text-[10.5px] text-neutral-400 font-mono">
              <span>{{ item.size }}</span>
              <span class="text-neutral-300">&bull;</span>
              <span class="text-emerald-600 dark:text-emerald-400 font-sans flex items-center gap-0.5">
                <CheckCircle2 class="w-3 h-3" />
                Siap
              </span>
            </div>
          </div>
        </div>

        <!-- Remove Action Button -->
        <button
          type="button"
          @click.stop="removeFile(idx)"
          class="p-1 rounded-lg text-neutral-400 hover:text-rose-600 hover:bg-rose-50 dark:hover:bg-rose-950/40 transition cursor-pointer shrink-0"
          title="Hapus Berkas"
        >
          <X class="w-4 h-4" />
        </button>
      </div>
    </div>
  </div>
</template>

