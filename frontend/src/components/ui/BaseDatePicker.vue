<script setup>
import { ref, computed, watch, onMounted, onUnmounted } from 'vue'
import {
  Calendar,
  ChevronLeft,
  ChevronRight,
  X,
  Check,
  Clock,
} from 'lucide-vue-next'

const props = defineProps({
  modelValue: {
    type: [String, Object, Array],
    default: '',
  },
  range: {
    type: Boolean,
    default: false,
  },
  label: {
    type: String,
    default: '',
  },
  placeholder: {
    type: String,
    default: 'Pilih tanggal...',
  },
  presets: {
    type: Boolean,
    default: true,
  },
  clearable: {
    type: Boolean,
    default: true,
  },
  size: {
    type: String,
    default: 'md', // 'sm' | 'md'
  },
  placement: {
    type: String,
    default: 'auto', // 'auto' | 'left' | 'right'
  },
})

const emit = defineEmits(['update:modelValue', 'change'])

const isOpen = ref(false)
const containerRef = ref(null)
const alignRight = ref(false)

// Current View Year & Month
const today = new Date()
const viewYear = ref(today.getFullYear())
const viewMonth = ref(today.getMonth()) // 0 - 11

// Date Selection State
const selectedSingle = ref('')
const selectedRangeStart = ref('')
const selectedRangeEnd = ref('')
const hoverDate = ref('')

const monthNames = [
  'Januari', 'Februari', 'Maret', 'April', 'Mei', 'Juni',
  'Juli', 'Agustus', 'September', 'Oktober', 'November', 'Desember',
]

const dayNames = ['Min', 'Sen', 'Sel', 'Rab', 'Kam', 'Jum', 'Sab']

// Format Helper YYYY-MM-DD
const formatDateStr = (d) => {
  if (!d) return ''
  const year = d.getFullYear()
  const month = String(d.getMonth() + 1).padStart(2, '0')
  const date = String(d.getDate()).padStart(2, '0')
  return `${year}-${month}-${date}`
}

// Format Helper Display ID (e.g., 27 Jan 2025)
const formatDisplay = (str) => {
  if (!str) return ''
  const parts = str.split('-')
  if (parts.length !== 3) return str
  const year = parts[0]
  const monthIdx = parseInt(parts[1], 10) - 1
  const day = parseInt(parts[2], 10)
  return `${day} ${monthNames[monthIdx]?.slice(0, 3)} ${year}`
}

// Display Value in Input Field
const displayValue = computed(() => {
  if (props.range) {
    if (selectedRangeStart.value && selectedRangeEnd.value) {
      const s = selectedRangeStart.value.split('-')
      const e = selectedRangeEnd.value.split('-')
      if (s.length === 3 && e.length === 3) {
        const sYear = s[0]
        const sMonth = monthNames[parseInt(s[1], 10) - 1]?.slice(0, 3)
        const sDay = parseInt(s[2], 10)
        const eYear = e[0]
        const eMonth = monthNames[parseInt(e[1], 10) - 1]?.slice(0, 3)
        const eDay = parseInt(e[2], 10)

        if (sYear === eYear && sMonth === eMonth) {
          return `${sDay} – ${eDay} ${eMonth} ${eYear}`
        } else if (sYear === eYear) {
          return `${sDay} ${sMonth} – ${eDay} ${eMonth} ${eYear}`
        }
      }
      return `${formatDisplay(selectedRangeStart.value)} — ${formatDisplay(selectedRangeEnd.value)}`
    }
    if (selectedRangeStart.value) {
      return `${formatDisplay(selectedRangeStart.value)} — ...`
    }
    return ''
  }
  return formatDisplay(selectedSingle.value)
})

// Initialize from prop
const initFromProp = () => {
  if (props.range) {
    if (typeof props.modelValue === 'object' && props.modelValue) {
      selectedRangeStart.value = props.modelValue.start || ''
      selectedRangeEnd.value = props.modelValue.end || ''
    }
  } else {
    selectedSingle.value = typeof props.modelValue === 'string' ? props.modelValue : ''
  }
}

watch(() => props.modelValue, initFromProp, { immediate: true })

// Calendar Grid Days Generation
const calendarDays = computed(() => {
  const year = viewYear.value
  const month = viewMonth.value

  const firstDay = new Date(year, month, 1)
  const lastDay = new Date(year, month + 1, 0)

  const startingDayOfWeek = firstDay.getDay() // 0 is Sunday
  const totalDays = lastDay.getDate()

  // Previous month filler days
  const prevMonthLastDay = new Date(year, month, 0).getDate()
  const days = []

  for (let i = startingDayOfWeek - 1; i >= 0; i--) {
    const d = new Date(year, month - 1, prevMonthLastDay - i)
    days.push({
      dateStr: formatDateStr(d),
      dayNumber: d.getDate(),
      isCurrentMonth: false,
    })
  }

  // Current month days
  for (let i = 1; i <= totalDays; i++) {
    const d = new Date(year, month, i)
    days.push({
      dateStr: formatDateStr(d),
      dayNumber: i,
      isCurrentMonth: true,
      isToday: formatDateStr(d) === formatDateStr(today),
    })
  }

  // Next month filler days to complete grid (42 cells: 6 weeks)
  const remaining = 42 - days.length
  for (let i = 1; i <= remaining; i++) {
    const d = new Date(year, month + 1, i)
    days.push({
      dateStr: formatDateStr(d),
      dayNumber: i,
      isCurrentMonth: false,
    })
  }

  return days
})

// Navigation
const prevMonth = () => {
  if (viewMonth.value === 0) {
    viewMonth.value = 11
    viewYear.value--
  } else {
    viewMonth.value--
  }
}

const nextMonth = () => {
  if (viewMonth.value === 11) {
    viewMonth.value = 0
    viewYear.value++
  } else {
    viewMonth.value++
  }
}

// Day Selection Click
const handleDayClick = (day) => {
  const dateStr = day.dateStr

  if (!props.range) {
    selectedSingle.value = dateStr
    emit('update:modelValue', dateStr)
    emit('change', dateStr)
    isOpen.value = false
    return
  }

  // Range logic
  if (!selectedRangeStart.value || (selectedRangeStart.value && selectedRangeEnd.value)) {
    selectedRangeStart.value = dateStr
    selectedRangeEnd.value = ''
  } else {
    if (new Date(dateStr) < new Date(selectedRangeStart.value)) {
      selectedRangeEnd.value = selectedRangeStart.value
      selectedRangeStart.value = dateStr
    } else {
      selectedRangeEnd.value = dateStr
    }

    const res = { start: selectedRangeStart.value, end: selectedRangeEnd.value }
    emit('update:modelValue', res)
    emit('change', res)
    isOpen.value = false
  }
}

// Check Day State (Selected, In Range)
const isDaySelected = (dateStr) => {
  if (!props.range) {
    return selectedSingle.value === dateStr
  }
  return selectedRangeStart.value === dateStr || selectedRangeEnd.value === dateStr
}

const isDayInRange = (dateStr) => {
  if (!props.range || !selectedRangeStart.value) return false

  const start = new Date(selectedRangeStart.value).getTime()
  const end = selectedRangeEnd.value
    ? new Date(selectedRangeEnd.value).getTime()
    : hoverDate.value && new Date(hoverDate.value).getTime() > start
    ? new Date(hoverDate.value).getTime()
    : null

  if (!end) return false
  const current = new Date(dateStr).getTime()
  return current > start && current < end
}

// Day State Classes dengan Pemisahan Mutlak (Mencegah Konflik Spesifisitas Tailwind di Dark & Light Mode)
const getDayClass = (day) => {
  const isSelected = isDaySelected(day.dateStr)
  const inRange = isDayInRange(day.dateStr)

  // 1. Tanggal Terpilih (Prioritas Utama: Kontras 100% Maksimal Mengikuti Token Tema)
  if (isSelected) {
    let rounded = 'rounded-lg'
    if (props.range) {
      if (selectedRangeStart.value === day.dateStr && selectedRangeEnd.value) {
        rounded = 'rounded-l-lg rounded-r-none'
      } else if (selectedRangeEnd.value === day.dateStr) {
        rounded = 'rounded-r-lg rounded-l-none'
      }
    }
    return `bg-[var(--theme-primary)] !text-[var(--theme-primary-text)] font-bold shadow-sm z-10 scale-105 ${rounded}`
  }

  // 2. Rentang Tanggal (Date Range di antara start & end)
  if (inRange) {
    return 'bg-[var(--theme-primary-soft)] !text-[var(--theme-primary)] font-bold rounded-none'
  }

  // 3. Hari Ini (Today) belum dipilih
  if (day.isToday) {
    return 'ring-1.5 ring-inset ring-[var(--theme-primary)] !text-[var(--theme-primary)] font-bold bg-[var(--theme-primary-soft)]/30 hover:bg-[var(--theme-primary-soft)]/60 rounded-lg'
  }

  // 4. Hari Biasa Bulan Ini (Teks Tegas & Sangat Kontras: Hitam Pekat di Light, Putih Terang di Dark)
  if (day.isCurrentMonth) {
    return 'text-neutral-900 dark:text-[var(--theme-text-primary)] font-semibold hover:bg-neutral-100 dark:hover:bg-[var(--theme-surface-muted)] rounded-lg'
  }

  // 5. Tanggal Luar Bulan (Filler Days - Muted Halus)
  return 'text-neutral-400 dark:text-neutral-600 font-normal hover:bg-neutral-50 dark:hover:bg-[var(--theme-surface-muted)] rounded-lg'
}

// Quick Presets
const applyPreset = (daysOffset) => {
  const end = new Date()
  const start = new Date()

  if (daysOffset === 0) {
    // Hari ini
    start.setDate(end.getDate())
  } else if (daysOffset === -1) {
    // Kemarin
    start.setDate(end.getDate() - 1)
    end.setDate(end.getDate() - 1)
  } else if (daysOffset === 'month') {
    // Bulan ini
    start.setDate(1)
  } else {
    start.setDate(end.getDate() - (daysOffset - 1))
  }

  const startStr = formatDateStr(start)
  const endStr = formatDateStr(end)

  if (props.range) {
    selectedRangeStart.value = startStr
    selectedRangeEnd.value = endStr
    const res = { start: startStr, end: endStr }
    emit('update:modelValue', res)
    emit('change', res)
  } else {
    selectedSingle.value = startStr
    emit('update:modelValue', startStr)
    emit('change', startStr)
  }

  viewYear.value = start.getFullYear()
  viewMonth.value = start.getMonth()
  isOpen.value = false
}

// Clear Value
const clear = () => {
  selectedSingle.value = ''
  selectedRangeStart.value = ''
  selectedRangeEnd.value = ''
  if (props.range) {
    emit('update:modelValue', { start: '', end: '' })
  } else {
    emit('update:modelValue', '')
  }
}

// Dynamic Auto-Placement Check: Melebar ke Kiri bila di Kanan, Melebar ke Kanan bila di Kiri
const checkPlacement = () => {
  if (props.placement === 'right') {
    alignRight.value = true
    return
  }
  if (props.placement === 'left') {
    alignRight.value = false
    return
  }
  if (!containerRef.value) return

  const rect = containerRef.value.getBoundingClientRect()
  // Lebar popover: ~415px jika presets aktif, atau ~290px jika tanpa presets
  const popoverWidth = props.presets ? 415 : 290
  const spaceOnRight = window.innerWidth - rect.left

  // Jika ruang kanan tidak cukup menampung popover,
  // ATAU posisi trigger berada di separuh kanan layar: melebar ke kiri (alignRight = true)
  if (spaceOnRight < popoverWidth || (rect.left + rect.width / 2 > window.innerWidth / 2)) {
    alignRight.value = true
  } else {
    alignRight.value = false
  }
}

const toggleOpen = () => {
  if (!isOpen.value) {
    checkPlacement()
  }
  isOpen.value = !isOpen.value
}

// Click outside handling
const handleClickOutside = (e) => {
  if (containerRef.value && !containerRef.value.contains(e.target)) {
    isOpen.value = false
  }
}

watch(isOpen, (open) => {
  if (open) {
    checkPlacement()
  }
})

onMounted(() => {
  document.addEventListener('click', handleClickOutside)
  window.addEventListener('resize', checkPlacement)
})

onUnmounted(() => {
  document.removeEventListener('click', handleClickOutside)
  window.removeEventListener('resize', checkPlacement)
})
</script>

<template>
  <div ref="containerRef" class="relative inline-block w-full select-none text-left">
    <!-- Label -->
    <label
      v-if="label"
      class="block text-xs font-semibold text-neutral-700 dark:text-neutral-300 mb-1"
    >
      {{ label }}
    </label>

    <!-- Trigger Input Box -->
    <div
      @click="toggleOpen"
      class="w-full flex items-center justify-between rounded-xl border border-neutral-300 dark:border-[var(--theme-border)] bg-white dark:bg-[var(--theme-surface)] text-xs text-neutral-900 dark:text-neutral-100 transition-all duration-150 cursor-pointer shadow-2xs hover:border-neutral-400 dark:hover:border-neutral-600 focus-within:ring-2 focus-within:ring-[var(--theme-primary)]"
      :class="[
        size === 'sm' ? 'h-8.5 px-2.5' : 'h-9.5 px-3',
        isOpen ? 'ring-2 ring-[var(--theme-primary)] border-[var(--theme-primary)]' : '',
      ]"
    >
      <div class="flex items-center gap-2 truncate">
        <Calendar class="w-4 h-4 text-neutral-500 dark:text-neutral-400 shrink-0" />
        <span v-if="displayValue" class="truncate font-semibold font-mono text-[11.5px] text-neutral-900 dark:text-neutral-100">
          {{ displayValue }}
        </span>
        <span v-else class="text-neutral-400 dark:text-neutral-500 truncate">
          {{ placeholder }}
        </span>
      </div>

      <div class="flex items-center gap-1 shrink-0 ml-1">
        <button
          v-if="clearable && displayValue"
          type="button"
          @click.stop="clear"
          class="p-0.5 rounded-md text-neutral-400 hover:text-neutral-700 dark:hover:text-neutral-200 hover:bg-neutral-100 dark:hover:bg-[var(--theme-surface-muted)] transition cursor-pointer"
          title="Reset Tanggal"
        >
          <X class="w-3.5 h-3.5" />
        </button>
      </div>
    </div>

    <!-- Dropdown Calendar Popover (Auto Alignment: right-0 jika di kanan, left-0 jika di kiri) -->
    <Transition
      enter-active-class="transition duration-150 ease-out"
      enter-from-class="opacity-0 scale-95 -translate-y-1"
      enter-to-class="opacity-100 scale-100 translate-y-0"
      leave-active-class="transition duration-100 ease-in"
      leave-from-class="opacity-100 scale-100 translate-y-0"
      leave-to-class="opacity-0 scale-95 -translate-y-1"
    >
      <div
        v-if="isOpen"
        class="absolute top-full mt-1.5 z-50 rounded-2xl bg-white dark:bg-[var(--theme-surface-raised)] border border-neutral-200 dark:border-[var(--theme-border)] shadow-2xl p-3 flex flex-col sm:flex-row gap-3 will-change-transform max-w-[calc(100vw-32px)]"
        :class="alignRight ? 'right-0' : 'left-0'"
      >
        <!-- Quick Presets Sidebar -->
        <div
          v-if="presets"
          class="sm:w-32 border-b sm:border-b-0 sm:border-r border-neutral-100 dark:border-[var(--theme-border)] pb-2 sm:pb-0 sm:pr-2.5 flex sm:flex-col gap-1 overflow-x-auto sm:overflow-visible shrink-0 text-xs"
        >
          <span class="text-[10px] font-bold uppercase tracking-wider text-neutral-500 dark:text-neutral-400 px-2 py-0.5 hidden sm:block">
            Preset
          </span>
          <button
            type="button"
            @click="applyPreset(0)"
            class="text-left px-2.5 py-1.5 rounded-lg text-neutral-700 dark:text-neutral-200 hover:text-neutral-900 dark:hover:text-white hover:bg-neutral-100 dark:hover:bg-[var(--theme-surface-muted)] transition whitespace-nowrap cursor-pointer text-xs font-medium"
          >
            Hari Ini
          </button>
          <button
            type="button"
            @click="applyPreset(7)"
            class="text-left px-2.5 py-1.5 rounded-lg text-neutral-700 dark:text-neutral-200 hover:text-neutral-900 dark:hover:text-white hover:bg-neutral-100 dark:hover:bg-[var(--theme-surface-muted)] transition whitespace-nowrap cursor-pointer text-xs font-medium"
          >
            7 Hari Terakhir
          </button>
          <button
            type="button"
            @click="applyPreset(30)"
            class="text-left px-2.5 py-1.5 rounded-lg text-neutral-700 dark:text-neutral-200 hover:text-neutral-900 dark:hover:text-white hover:bg-neutral-100 dark:hover:bg-[var(--theme-surface-muted)] transition whitespace-nowrap cursor-pointer text-xs font-medium"
          >
            30 Hari Terakhir
          </button>
          <button
            type="button"
            @click="applyPreset('month')"
            class="text-left px-2.5 py-1.5 rounded-lg text-neutral-700 dark:text-neutral-200 hover:text-neutral-900 dark:hover:text-white hover:bg-neutral-100 dark:hover:bg-[var(--theme-surface-muted)] transition whitespace-nowrap cursor-pointer text-xs font-medium"
          >
            Bulan Ini
          </button>
        </div>

        <!-- Main Calendar Matrix -->
        <div class="w-64 sm:w-68">
          <!-- Calendar Header: Month + Year Navigator -->
          <div class="flex items-center justify-between pb-2 mb-2 border-b border-neutral-100 dark:border-[var(--theme-border)]">
            <span class="font-bold text-xs sm:text-sm text-neutral-900 dark:text-white">
              {{ monthNames[viewMonth] }} {{ viewYear }}
            </span>

            <div class="flex items-center gap-1">
              <button
                type="button"
                @click="prevMonth"
                class="p-1 rounded-lg text-neutral-500 hover:text-neutral-900 dark:text-neutral-400 dark:hover:text-white hover:bg-neutral-100 dark:hover:bg-[var(--theme-surface-muted)] transition cursor-pointer"
                title="Bulan Sebelumnya"
              >
                <ChevronLeft class="w-4 h-4" />
              </button>
              <button
                type="button"
                @click="nextMonth"
                class="p-1 rounded-lg text-neutral-500 hover:text-neutral-900 dark:text-neutral-400 dark:hover:text-white hover:bg-neutral-100 dark:hover:bg-[var(--theme-surface-muted)] transition cursor-pointer"
                title="Bulan Berikutnya"
              >
                <ChevronRight class="w-4 h-4" />
              </button>
            </div>
          </div>

          <!-- Day Names Row (Teks Jelas di Light & Dark Mode) -->
          <div class="grid grid-cols-7 gap-1 text-center mb-1.5">
            <span
              v-for="d in dayNames"
              :key="d"
              class="text-[11px] font-bold text-neutral-600 dark:text-neutral-400 uppercase py-1 select-none"
            >
              {{ d }}
            </span>
          </div>

          <!-- Days Grid (42 Cells) dengan Kontras Tinggi & Huruf Jelas Bebas Konflik Class -->
          <div class="grid grid-cols-7 gap-1 text-center text-xs" @mouseleave="hoverDate = null">
            <button
              v-for="day in calendarDays"
              :key="day.dateStr"
              type="button"
              @click="handleDayClick(day)"
              @mouseenter="hoverDate = day.dateStr"
              class="h-8 w-8 mx-auto flex items-center justify-center font-mono text-xs transition-all cursor-pointer relative"
              :class="getDayClass(day)"
              :style="{
                color: isDaySelected(day.dateStr) ? 'var(--theme-primary-text)' : undefined
              }"
            >
              {{ day.dayNumber }}
            </button>
          </div>
        </div>
      </div>
    </Transition>
  </div>
</template>
