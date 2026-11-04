<script setup>
import { ref, computed } from 'vue'
import BaseModal from './BaseModal.vue'
import BaseButton from './BaseButton.vue'
import BaseBadge from './BaseBadge.vue'
import {
  FileSpreadsheet,
  UploadCloud,
  ArrowRight,
  ArrowLeft,
  Check,
  AlertCircle,
  FileCheck2,
  Trash2,
  Table,
} from 'lucide-vue-next'

const props = defineProps({
  modelValue: {
    type: Boolean,
    default: false,
  },
  title: {
    type: String,
    default: 'Impor Data dari Berkas CSV',
  },
  targetFields: {
    type: Array,
    default: () => [
      { key: 'nama', label: 'Nama Lengkap', required: true },
      { key: 'jabatan', label: 'Jabatan', required: true },
      { key: 'divisi', label: 'Divisi', required: false },
      { key: 'email', label: 'Email', required: false },
      { key: 'gaji', label: 'Gaji Pokok', required: false },
      { key: 'status', label: 'Status', required: false },
    ],
  },
})

const emit = defineEmits(['update:modelValue', 'imported'])

// Wizard Step: 1 (Upload), 2 (Mapping & Preview), 3 (Confirm & Finish)
const currentStep = ref(1)
const isDragging = ref(false)
const selectedFile = ref(null)
const csvHeaders = ref([])
const csvRawRows = ref([])
const columnMapping = ref({}) // { targetKey: csvHeaderName }
const errorMessage = ref('')
const isProcessing = ref(false)

// Reset state
const resetState = () => {
  currentStep.value = 1
  selectedFile.value = null
  csvHeaders.value = []
  csvRawRows.value = []
  columnMapping.value = {}
  errorMessage.value = ''
  isProcessing.value = false
}

// Robust CSV Parser
const parseCSVText = (text) => {
  // Strip BOM if present
  let cleanText = text
  if (cleanText.charCodeAt(0) === 0xfeff) {
    cleanText = cleanText.slice(1)
  }

  // Detect delimiter (, or ;)
  const firstLine = cleanText.split(/\r\n|\n|\r/)[0] || ''
  const delimiter = (firstLine.match(/;/g) || []).length > (firstLine.match(/,/g) || []).length ? ';' : ','

  const rows = []
  let currentRow = []
  let currentField = ''
  let insideQuotes = false

  for (let i = 0; i < cleanText.length; i++) {
    const char = cleanText[i]
    const nextChar = cleanText[i + 1]

    if (char === '"') {
      if (insideQuotes && nextChar === '"') {
        currentField += '"'
        i++ // Skip escaped quote
      } else {
        insideQuotes = !insideQuotes
      }
    } else if (char === delimiter && !insideQuotes) {
      currentRow.push(currentField.trim())
      currentField = ''
    } else if ((char === '\r' || char === '\n') && !insideQuotes) {
      if (char === '\r' && nextChar === '\n') {
        i++ // Skip \r\n
      }
      currentRow.push(currentField.trim())
      if (currentRow.some((val) => val !== '')) {
        rows.push(currentRow)
      }
      currentRow = []
      currentField = ''
    } else {
      currentField += char
    }
  }

  // Add remaining field if any
  if (currentField || currentRow.length > 0) {
    currentRow.push(currentField.trim())
    if (currentRow.some((val) => val !== '')) {
      rows.push(currentRow)
    }
  }

  return rows
}

// Handle File Selection
const handleFile = (file) => {
  errorMessage.value = ''
  if (!file) return

  if (!file.name.toLowerCase().endsWith('.csv') && file.type !== 'text/csv') {
    errorMessage.value = 'Hanya berkas format .CSV yang didukung saat ini.'
    return
  }

  selectedFile.value = file

  const reader = new FileReader()
  reader.onload = (e) => {
    try {
      const content = e.target.result
      const parsed = parseCSVText(content)

      if (parsed.length < 2) {
        errorMessage.value = 'Berkas CSV harus memiliki minimal 1 baris header dan 1 baris data.'
        return
      }

      csvHeaders.value = parsed[0]
      csvRawRows.value = parsed.slice(1)

      // Auto-mapping heuristics
      const mapping = {}
      props.targetFields.forEach((target) => {
        const targetClean = target.label.toLowerCase().replace(/[^a-z0-9]/g, '')
        const targetKeyClean = target.key.toLowerCase().replace(/[^a-z0-9]/g, '')

        const matchedHeader = csvHeaders.value.find((h) => {
          const hClean = h.toLowerCase().replace(/[^a-z0-9]/g, '')
          return hClean.includes(targetClean) || targetClean.includes(hClean) || hClean === targetKeyClean
        })

        mapping[target.key] = matchedHeader || ''
      })

      columnMapping.value = mapping
      currentStep.value = 2
    } catch (err) {
      errorMessage.value = 'Gagal memproses file CSV: ' + err.message
    }
  }

  reader.onerror = () => {
    errorMessage.value = 'Terjadi kesalahan saat membaca file.'
  }

  reader.readAsText(file)
}

// Drag & Drop
const onDrop = (e) => {
  isDragging.value = false
  const file = e.dataTransfer.files?.[0]
  if (file) handleFile(file)
}

const onFileInputChange = (e) => {
  const file = e.target.files?.[0]
  if (file) handleFile(file)
}

// Preview Rows (first 4 data rows)
const previewRows = computed(() => {
  return csvRawRows.value.slice(0, 4)
})

// Check if all required fields are mapped
const isMappingValid = computed(() => {
  return props.targetFields
    .filter((f) => f.required)
    .every((f) => !!columnMapping.value[f.key])
})

// Execute Import
const confirmImport = () => {
  isProcessing.value = true

  // Transform raw rows into structured objects
  const importedData = csvRawRows.value.map((row) => {
    const item = {}
    props.targetFields.forEach((field) => {
      const mappedHeader = columnMapping.value[field.key]
      if (mappedHeader) {
        const headerIndex = csvHeaders.value.indexOf(mappedHeader)
        if (headerIndex !== -1) {
          item[field.key] = row[headerIndex] || ''
        }
      }
    })
    return item
  })

  setTimeout(() => {
    emit('imported', importedData)
    isProcessing.value = false
    emit('update:modelValue', false)
    resetState()
  }, 400)
}
</script>

<template>
  <BaseModal
    :model-value="modelValue"
    @update:model-value="emit('update:modelValue', $event); if (!$event) resetState();"
    size="lg"
  >
    <template #header>
      <div class="flex items-center gap-2">
        <FileSpreadsheet class="w-4 h-4 text-emerald-600 dark:text-emerald-400" />
        <h3 class="text-sm font-bold text-neutral-900 dark:text-white">
          {{ title }}
        </h3>
        <BaseBadge variant="neutral" size="sm">
          Langkah {{ currentStep }} dari 3
        </BaseBadge>
      </div>
    </template>

    <div class="space-y-5 text-xs select-none">
      <!-- Step Indicator Bar -->
      <div class="grid grid-cols-3 gap-2 pb-1 border-b border-neutral-200 dark:border-[var(--theme-border)]">
        <div
          class="flex items-center gap-2 py-1 border-b-2 text-[11px] font-semibold transition"
          :class="currentStep === 1 ? 'border-[var(--theme-primary)] text-[var(--theme-primary)]' : 'border-transparent text-neutral-400'"
        >
          <span class="w-5 h-5 rounded-full flex items-center justify-center text-[10px]" :class="currentStep >= 1 ? 'bg-[var(--theme-primary)] text-white' : 'bg-neutral-200 text-neutral-600'">1</span>
          <span>Unggah CSV</span>
        </div>

        <div
          class="flex items-center gap-2 py-1 border-b-2 text-[11px] font-semibold transition"
          :class="currentStep === 2 ? 'border-[var(--theme-primary)] text-[var(--theme-primary)]' : 'border-transparent text-neutral-400'"
        >
          <span class="w-5 h-5 rounded-full flex items-center justify-center text-[10px]" :class="currentStep >= 2 ? 'bg-[var(--theme-primary)] text-white' : 'bg-neutral-200 text-neutral-600'">2</span>
          <span>Pemetaan Kolom</span>
        </div>

        <div
          class="flex items-center gap-2 py-1 border-b-2 text-[11px] font-semibold transition"
          :class="currentStep === 3 ? 'border-[var(--theme-primary)] text-[var(--theme-primary)]' : 'border-transparent text-neutral-400'"
        >
          <span class="w-5 h-5 rounded-full flex items-center justify-center text-[10px]" :class="currentStep === 3 ? 'bg-[var(--theme-primary)] text-white' : 'bg-neutral-200 text-neutral-600'">3</span>
          <span>Pratinjau & Selesai</span>
        </div>
      </div>

      <!-- ======================================================== -->
      <!-- STEP 1: UPLOAD CSV FILE                                  -->
      <!-- ======================================================== -->
      <div v-if="currentStep === 1" class="space-y-4">
        <div
          @dragover.prevent="isDragging = true"
          @dragleave.prevent="isDragging = false"
          @drop.prevent="onDrop"
          class="border-2 border-dashed rounded-2xl p-8 text-center flex flex-col items-center justify-center gap-3 transition cursor-pointer"
          :class="isDragging ? 'border-[var(--theme-primary)] bg-[var(--theme-primary-soft)]' : 'border-neutral-300 dark:border-[var(--theme-border-strong)] bg-neutral-50/50 dark:bg-[var(--theme-surface-raised)] hover:border-neutral-400 dark:hover:border-neutral-600'"
          @click="$refs.fileInput?.click()"
        >
          <input
            ref="fileInput"
            type="file"
            accept=".csv,text/csv"
            class="hidden"
            @change="onFileInputChange"
          />

          <div class="w-12 h-12 rounded-2xl bg-emerald-50 dark:bg-emerald-950/40 text-emerald-600 dark:text-emerald-400 flex items-center justify-center shadow-xs">
            <UploadCloud class="w-6 h-6 stroke-[1.8]" />
          </div>

          <div class="space-y-1">
            <p class="font-bold text-sm text-neutral-800 dark:text-neutral-200">
              Pilih berkas CSV atau seret ke area ini
            </p>
            <p class="text-neutral-400 text-[11px]">
              Mendukung delimiter koma (,) dan titik koma (;). Encoding UTF-8 direkomendasikan.
            </p>
          </div>

          <BaseButton variant="outline" size="sm" type="button">
            Pilih Berkas CSV
          </BaseButton>
        </div>

        <!-- Error Alert -->
        <div
          v-if="errorMessage"
          class="p-3 rounded-xl bg-rose-50 dark:bg-rose-950/40 border border-rose-200 dark:border-rose-900/50 text-rose-600 dark:text-rose-400 flex items-center gap-2"
        >
          <AlertCircle class="w-4 h-4 shrink-0" />
          <span>{{ errorMessage }}</span>
        </div>
      </div>

      <!-- ======================================================== -->
      <!-- STEP 2: COLUMN MAPPING                                   -->
      <!-- ======================================================== -->
      <div v-else-if="currentStep === 2" class="space-y-4">
        <div class="p-3 rounded-xl bg-neutral-50 dark:bg-[var(--theme-surface-raised)] border border-neutral-200 dark:border-[var(--theme-border)] flex items-center justify-between">
          <div class="flex items-center gap-2 min-w-0">
            <FileSpreadsheet class="w-4 h-4 text-emerald-500 shrink-0" />
            <span class="font-bold text-neutral-900 dark:text-white truncate">{{ selectedFile?.name }}</span>
            <span class="text-neutral-400 font-mono text-[11px]">({{ csvRawRows.length }} baris data)</span>
          </div>
          <BaseBadge variant="success" size="sm">File Terbaca</BaseBadge>
        </div>

        <p class="text-neutral-500 dark:text-neutral-400">
          Cocokkan kolom pada berkas CSV Anda dengan field tujuan di sistem:
        </p>

        <!-- Mapping Grid -->
        <div class="space-y-2.5 max-h-64 overflow-y-auto pr-1">
          <div
            v-for="field in targetFields"
            :key="field.key"
            class="p-2.5 rounded-xl bg-white dark:bg-[var(--theme-surface-raised)] border border-neutral-200 dark:border-[var(--theme-border)] flex items-center justify-between gap-3"
          >
            <div class="w-1/2 min-w-0">
              <span class="font-bold text-neutral-900 dark:text-white block truncate">
                {{ field.label }}
                <span v-if="field.required" class="text-rose-500">*</span>
              </span>
              <span class="text-[10px] text-neutral-400 font-mono block">Field: {{ field.key }}</span>
            </div>

            <div class="w-1/2">
              <select
                v-model="columnMapping[field.key]"
                class="w-full text-xs py-1.5 px-2.5 rounded-lg border border-neutral-300 dark:border-[var(--theme-border-strong)] bg-neutral-50 dark:bg-[var(--theme-surface-muted)] text-neutral-800 dark:text-neutral-200 focus:outline-none focus:ring-1 focus:ring-[var(--theme-primary)]"
              >
                <option value="">-- Lewati Kolom --</option>
                <option v-for="header in csvHeaders" :key="header" :value="header">
                  Kolom CSV: {{ header }}
                </option>
              </select>
            </div>
          </div>
        </div>

        <div v-if="!isMappingValid" class="text-[11px] text-amber-600 dark:text-amber-400 flex items-center gap-1.5">
          <AlertCircle class="w-3.5 h-3.5 shrink-0" />
          <span>Kolom bertanda (*) wajib dipetakan sebelum melanjutkan.</span>
        </div>
      </div>

      <!-- ======================================================== -->
      <!-- STEP 3: PREVIEW & CONFIRMATION                           -->
      <!-- ======================================================== -->
      <div v-else-if="currentStep === 3" class="space-y-4">
        <div class="p-3.5 rounded-xl bg-emerald-50 dark:bg-emerald-950/30 border border-emerald-200 dark:border-emerald-900/40 text-emerald-800 dark:text-emerald-300 flex items-center gap-3">
          <FileCheck2 class="w-5 h-5 text-emerald-600 dark:text-emerald-400 shrink-0" />
          <div>
            <div class="font-bold">Semua data siap diimpor!</div>
            <div class="text-[11px] text-emerald-700 dark:text-emerald-400 mt-0.5">
              Sebanyak <strong>{{ csvRawRows.length }} baris</strong> data akan ditambahkan ke sistem.
            </div>
          </div>
        </div>

        <!-- Sample Preview Table -->
        <div class="space-y-1.5">
          <span class="font-bold text-neutral-700 dark:text-neutral-300 block">
            Pratinjau 4 Baris Pertama Hasil Pemetaan:
          </span>

          <div class="overflow-x-auto rounded-xl border border-neutral-200 dark:border-[var(--theme-border)]">
            <table class="w-full text-left text-xs">
              <thead class="bg-neutral-100 dark:bg-[var(--theme-surface-raised)] font-bold text-neutral-600 dark:text-neutral-400 border-b border-neutral-200 dark:border-[var(--theme-border)]">
                <tr>
                  <th v-for="field in targetFields.filter(f => !!columnMapping[f.key])" :key="field.key" class="py-2 px-3">
                    {{ field.label }}
                  </th>
                </tr>
              </thead>
              <tbody class="divide-y divide-neutral-100 dark:divide-[var(--theme-border)]">
                <tr v-for="(row, rIdx) in previewRows" :key="rIdx" class="hover:bg-neutral-50/50 dark:hover:bg-[var(--theme-surface-raised)]">
                  <td
                    v-for="field in targetFields.filter(f => !!columnMapping[f.key])"
                    :key="field.key"
                    class="py-2 px-3 text-neutral-800 dark:text-neutral-200 truncate max-w-xs"
                  >
                    {{ row[csvHeaders.indexOf(columnMapping[field.key])] || '-' }}
                  </td>
                </tr>
              </tbody>
            </table>
          </div>
        </div>
      </div>
    </div>

    <!-- Modal Footer Controls -->
    <template #footer>
      <div class="flex items-center justify-between w-full">
        <!-- Back Button -->
        <div>
          <BaseButton
            v-if="currentStep > 1"
            variant="outline"
            size="sm"
            @click="currentStep--"
          >
            <ArrowLeft class="w-3.5 h-3.5" />
            <span>Kembali</span>
          </BaseButton>
        </div>

        <!-- Next / Confirm Button -->
        <div class="flex items-center gap-2">
          <BaseButton
            variant="outline"
            size="sm"
            @click="emit('update:modelValue', false); resetState();"
          >
            Batal
          </BaseButton>

          <BaseButton
            v-if="currentStep === 2"
            variant="primary"
            size="sm"
            :disabled="!isMappingValid"
            @click="currentStep = 3"
          >
            <span>Lanjut Pratinjau</span>
            <ArrowRight class="w-3.5 h-3.5" />
          </BaseButton>

          <BaseButton
            v-if="currentStep === 3"
            variant="primary"
            size="sm"
            :loading="isProcessing"
            @click="confirmImport"
          >
            <Check class="w-3.5 h-3.5" />
            <span>Mulai Impor ({{ csvRawRows.length }} Data)</span>
          </BaseButton>
        </div>
      </div>
    </template>
  </BaseModal>
</template>

