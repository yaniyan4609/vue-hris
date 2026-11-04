<script setup>
import { ref, computed } from 'vue'
import { ChevronLeft, ChevronRight, Calendar as CalendarIcon } from 'lucide-vue-next'
import BaseButton from './BaseButton.vue'

const props = defineProps({
  modelValue: {
    type: String,
    default: '', // 'YYYY-MM-DD'
  },
  events: {
    type: Array,
    default: () => [],
    // Each event: { id, date: 'YYYY-MM-DD', endDate?: 'YYYY-MM-DD', title: string, variant?: 'primary'|'success'|'warning'|'danger'|'info'|'purple', badge?: string }
  },
  holidays: {
    type: Array,
    default: () => [],
    // Each holiday: { date: 'YYYY-MM-DD', label: string }
  },
  selectable: {
    type: Boolean,
    default: true,
  },
})

const emit = defineEmits(['update:modelValue', 'date-click', 'event-click', 'month-change'])

const today = new Date()
const currentYear = ref(today.getFullYear())
const currentMonth = ref(today.getMonth()) // 0 - 11

const monthNames = [
  'Januari', 'Februari', 'Maret', 'April', 'Mei', 'Juni',
  'Juli', 'Agustus', 'September', 'Oktober', 'November', 'Desember',
]

const dayNames = ['Sen', 'Sel', 'Rab', 'Kam', 'Jum', 'Sab', 'Min']

const currentMonthLabel = computed(() => {
  return `${monthNames[currentMonth.value]} ${currentYear.value}`
})

const formatYMD = (year, month, day) => {
  const m = String(month + 1).padStart(2, '0')
  const d = String(day).padStart(2, '0')
  return `${year}-${m}-${d}`
}

const todayString = formatYMD(today.getFullYear(), today.getMonth(), today.getDate())

const holidayMap = computed(() => {
  const map = new Map()
  for (const h of props.holidays) {
    map.set(h.date, h.label)
  }
  return map
})

const eventsByDate = computed(() => {
  const map = new Map()
  for (const ev of props.events) {
    if (!map.has(ev.date)) {
      map.set(ev.date, [])
    }
    map.get(ev.date).push(ev)
  }
  return map
})

// Build the 35 or 42 grid cells
const calendarDays = computed(() => {
  const year = currentYear.value
  const month = currentMonth.value

  const firstDayOfWeek = (new Date(year, month, 1).getDay() + 6) % 7 // Monday = 0
  const daysInCurrentMonth = new Date(year, month + 1, 0).getDate()
  const daysInPrevMonth = new Date(year, month, 0).getDate()

  const days = []

  // Previous month trailing days
  for (let i = firstDayOfWeek - 1; i >= 0; i--) {
    const dayNum = daysInPrevMonth - i
    const prevMonth = month === 0 ? 11 : month - 1
    const prevYear = month === 0 ? year - 1 : year
    const dateStr = formatYMD(prevYear, prevMonth, dayNum)
    days.push({
      day: dayNum,
      date: dateStr,
      isCurrentMonth: false,
      isToday: dateStr === todayString,
      isHoliday: holidayMap.value.has(dateStr),
      holidayLabel: holidayMap.value.get(dateStr),
      events: eventsByDate.value.get(dateStr) || [],
    })
  }

  // Current month days
  for (let d = 1; d <= daysInCurrentMonth; d++) {
    const dateStr = formatYMD(year, month, d)
    days.push({
      day: d,
      date: dateStr,
      isCurrentMonth: true,
      isToday: dateStr === todayString,
      isHoliday: holidayMap.value.has(dateStr),
      holidayLabel: holidayMap.value.get(dateStr),
      events: eventsByDate.value.get(dateStr) || [],
    })
  }

  // Next month leading days to complete full weeks
  const totalCells = days.length <= 35 ? 35 : 42
  const remaining = totalCells - days.length
  for (let d = 1; d <= remaining; d++) {
    const nextMonth = month === 11 ? 0 : month + 1
    const nextYear = month === 11 ? year + 1 : year
    const dateStr = formatYMD(nextYear, nextMonth, d)
    days.push({
      day: d,
      date: dateStr,
      isCurrentMonth: false,
      isToday: dateStr === todayString,
      isHoliday: holidayMap.value.has(dateStr),
      holidayLabel: holidayMap.value.get(dateStr),
      events: eventsByDate.value.get(dateStr) || [],
    })
  }

  return days
})

const prevMonth = () => {
  if (currentMonth.value === 0) {
    currentMonth.value = 11
    currentYear.value--
  } else {
    currentMonth.value--
  }
  emit('month-change', { year: currentYear.value, month: currentMonth.value + 1 })
}

const nextMonth = () => {
  if (currentMonth.value === 11) {
    currentMonth.value = 0
    currentYear.value++
  } else {
    currentMonth.value++
  }
  emit('month-change', { year: currentYear.value, month: currentMonth.value + 1 })
}

const goToToday = () => {
  currentYear.value = today.getFullYear()
  currentMonth.value = today.getMonth()
  emit('update:modelValue', todayString)
  emit('month-change', { year: currentYear.value, month: currentMonth.value + 1 })
}

const handleDateClick = (dayObj) => {
  if (!props.selectable) return
  emit('update:modelValue', dayObj.date)
  emit('date-click', dayObj)
}

const handleEventClick = (ev, e) => {
  e.stopPropagation()
  emit('event-click', ev)
}

const eventVariantClasses = (variant = 'primary') => {
  const map = {
    primary: 'bg-[var(--theme-primary)] text-[var(--theme-primary-text)]',
    success: 'bg-emerald-500 text-white',
    warning: 'bg-amber-500 text-white',
    danger: 'bg-rose-500 text-white',
    info: 'bg-sky-500 text-white',
    purple: 'bg-purple-500 text-white',
  }
  return map[variant] || map.primary
}
</script>

<template>
  <div class="w-full rounded-2xl border border-neutral-200 dark:border-[var(--theme-border)] bg-white dark:bg-[var(--theme-surface)] shadow-2xs overflow-hidden select-none">
    <!-- Header: Bulan, Tahun & Navigasi -->
    <div class="px-4 py-3 border-b border-neutral-200/80 dark:border-[var(--theme-border)] bg-neutral-50/70 dark:bg-[var(--theme-layer-2)] flex items-center justify-between gap-3">
      <div class="flex items-center gap-2">
        <div class="w-8 h-8 rounded-xl bg-white dark:bg-[var(--theme-surface-raised)] border border-neutral-200/80 dark:border-[var(--theme-border-strong)] flex items-center justify-center text-[var(--theme-primary)] shadow-2xs">
          <CalendarIcon class="w-4 h-4" />
        </div>
        <div>
          <h3 class="text-sm font-bold text-neutral-900 dark:text-white leading-tight">
            {{ currentMonthLabel }}
          </h3>
          <p class="text-[11px] text-neutral-400 dark:text-neutral-500">
            Kalender Agenda, Roster & Cuti
          </p>
        </div>
      </div>

      <div class="flex items-center gap-1.5">
        <BaseButton size="sm" variant="ghost" @click="goToToday">
          Hari Ini
        </BaseButton>

        <button
          type="button"
          @click="prevMonth"
          class="p-1.5 rounded-xl border border-neutral-200/80 dark:border-[var(--theme-border)] bg-white dark:bg-[var(--theme-surface-raised)] text-neutral-600 dark:text-neutral-300 hover:text-neutral-900 dark:hover:text-white hover:border-neutral-300 dark:hover:border-[var(--theme-border-strong)] transition cursor-pointer"
          title="Bulan sebelumnya"
        >
          <ChevronLeft class="w-4 h-4" />
        </button>

        <button
          type="button"
          @click="nextMonth"
          class="p-1.5 rounded-xl border border-neutral-200/80 dark:border-[var(--theme-border)] bg-white dark:bg-[var(--theme-surface-raised)] text-neutral-600 dark:text-neutral-300 hover:text-neutral-900 dark:hover:text-white hover:border-neutral-300 dark:hover:border-[var(--theme-border-strong)] transition cursor-pointer"
          title="Bulan berikutnya"
        >
          <ChevronRight class="w-4 h-4" />
        </button>
      </div>
    </div>

    <!-- Day Header Row (Sen - Min) -->
    <div class="grid grid-cols-7 border-b border-neutral-200/80 dark:border-[var(--theme-border)] bg-neutral-100/60 dark:bg-[var(--theme-layer-2)]/60 text-center text-[11px] font-bold py-2">
      <div
        v-for="(dayName, idx) in dayNames"
        :key="dayName"
        :class="idx >= 5 ? 'text-rose-500' : 'text-neutral-500 dark:text-neutral-400'"
      >
        {{ dayName }}
      </div>
    </div>

    <!-- Days Grid -->
    <div class="grid grid-cols-7 divide-x divide-y divide-neutral-200/70 dark:divide-[var(--theme-border)]">
      <div
        v-for="cell in calendarDays"
        :key="cell.date"
        @click="handleDateClick(cell)"
        class="min-h-[88px] sm:min-h-[100px] p-1.5 transition-colors flex flex-col justify-between"
        :class="[
          selectable ? 'cursor-pointer' : '',
          cell.isCurrentMonth
            ? 'bg-white dark:bg-[var(--theme-surface)] hover:bg-neutral-50/80 dark:hover:bg-[var(--theme-surface-raised)]'
            : 'bg-neutral-50/50 dark:bg-[var(--theme-layer-main)]/30 opacity-40',
          modelValue === cell.date
            ? 'ring-2 ring-inset ring-[var(--theme-primary)] bg-[var(--theme-primary-soft)]/20 dark:bg-[var(--theme-primary-soft)]/10'
            : '',
        ]"
      >
        <!-- Top bar: Tanggal & Indikator Libur -->
        <div class="flex items-center justify-between gap-1">
          <span
            class="w-6 h-6 rounded-lg text-xs font-bold font-mono inline-flex items-center justify-center shrink-0"
            :class="[
              cell.isToday
                ? 'bg-[var(--theme-primary)] text-[var(--theme-primary-text)] shadow-2xs'
                : cell.isHoliday
                ? 'text-rose-600 dark:text-rose-400 font-extrabold'
                : 'text-neutral-800 dark:text-neutral-200',
            ]"
          >
            {{ cell.day }}
          </span>

          <span
            v-if="cell.holidayLabel"
            class="text-[9.5px] font-semibold text-rose-600 dark:text-rose-400 truncate max-w-[80px]"
            :title="cell.holidayLabel"
          >
            {{ cell.holidayLabel }}
          </span>
        </div>

        <!-- Middle: Event Pills -->
        <div class="space-y-1 mt-1 flex-1 overflow-hidden">
          <div
            v-for="ev in cell.events.slice(0, 2)"
            :key="ev.id"
            @click="handleEventClick(ev, $event)"
            class="px-1.5 py-0.5 rounded-md text-[10px] font-semibold truncate transition cursor-pointer hover:opacity-90 flex items-center justify-between gap-1 shadow-2xs"
            :class="eventVariantClasses(ev.variant)"
            :title="ev.title"
          >
            <span class="truncate">{{ ev.title }}</span>
            <span v-if="ev.badge" class="text-[9px] opacity-80 shrink-0">{{ ev.badge }}</span>
          </div>

          <span
            v-if="cell.events.length > 2"
            class="text-[9.5px] font-bold text-neutral-400 dark:text-neutral-500 pl-1 block"
          >
            +{{ cell.events.length - 2 }} lainnya
          </span>
        </div>

        <!-- Slot Day Extra -->
        <div v-if="$slots['day-extra']">
          <slot name="day-extra" :date="cell.date" :events="cell.events" />
        </div>
      </div>
    </div>
  </div>
</template>

