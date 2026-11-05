<script setup>
import { computed, onMounted, ref } from 'vue'
import { useRouter } from 'vue-router'
import { CalendarDays, Clock3, Fingerprint } from 'lucide-vue-next'
import { apiRequest } from '../composables/useApi'
import { useToast } from '../composables/useToast'
import { essQuickActions } from '../data/menuData'
import BaseButton from '../components/ui/BaseButton.vue'
import BaseBadge from '../components/ui/BaseBadge.vue'

const router = useRouter()
const toast = useToast()
const profile = ref(null)
const today = computed(() => new Intl.DateTimeFormat('id-ID', { weekday: 'long', day: '2-digit', month: 'long', year: 'numeric' }).format(new Date()))
onMounted(async () => { try { profile.value = await apiRequest('/ess/me/employee') } catch (error) { toast.error(error.message, 'Gagal memuat profil') } })
</script>

<template>
  <div class="space-y-3 pb-4">
    <section class="rounded-2xl bg-white dark:bg-[var(--theme-surface)] border border-neutral-200 dark:border-[var(--theme-border)] p-4 shadow-sm">
      <p class="text-xs text-neutral-500 dark:text-neutral-400">{{ today }}</p>
      <div class="flex items-center justify-between gap-3 mt-2">
        <div><BaseBadge variant="neutral" dot>Presensi hari ini</BaseBadge><p class="text-2xl font-bold tabular-nums text-neutral-900 dark:text-white mt-2">--:--</p><p class="text-[11px] text-neutral-500 dark:text-neutral-400">Data presensi belum tersedia</p></div>
        <div class="w-14 h-14 rounded-2xl border border-dashed border-neutral-300 dark:border-[var(--theme-border)] flex items-center justify-center text-neutral-400"><CalendarDays class="w-5 h-5" /></div>
      </div>
      <div class="grid grid-cols-3 mt-4 pt-3 border-t border-neutral-100 dark:border-[var(--theme-border)] text-center text-[11px]">
        <div><p class="text-neutral-500 dark:text-neutral-400">Masuk</p><p class="font-semibold mt-1">--:--</p></div>
        <div class="border-x border-neutral-100 dark:border-[var(--theme-border)]"><p class="text-neutral-500 dark:text-neutral-400">Target kerja</p><p class="font-semibold mt-1">—</p></div>
        <div><p class="text-neutral-500 dark:text-neutral-400">Pulang</p><p class="font-semibold mt-1">--:--</p></div>
      </div>
      <div class="mt-4 rounded-xl bg-[var(--theme-primary-soft)] text-[var(--theme-primary-soft-text)] px-3 py-2.5 flex items-center gap-2 text-[11px]"><Clock3 class="w-4 h-4" /><span>Informasi shift dan presensi akan tampil di sini.</span></div>
    </section>

    <section class="rounded-2xl bg-white dark:bg-[var(--theme-surface)] border border-neutral-200 dark:border-[var(--theme-border)] p-4 shadow-sm">
      <div class="grid grid-cols-4 gap-3">
        <BaseButton v-for="action in essQuickActions" :key="action.id" variant="ghost" size="sm" class="!p-1 flex flex-col gap-1 min-w-0" @click="router.push(action.route)">
          <span class="w-10 h-10 rounded-2xl bg-[var(--theme-primary-soft)] text-[var(--theme-primary)] flex items-center justify-center"><component :is="action.icon" class="w-5 h-5" /></span>
          <span class="text-[10px] truncate w-full">{{ action.label }}</span>
        </BaseButton>
      </div>
    </section>

    <section class="rounded-2xl bg-white dark:bg-[var(--theme-surface)] border border-neutral-200 dark:border-[var(--theme-border)] p-4 shadow-sm">
      <div class="flex items-center justify-between gap-2"><div><p class="text-xs font-semibold text-neutral-900 dark:text-white">Rekap presensi bulan ini</p><p class="text-[11px] text-neutral-500 dark:text-neutral-400 mt-1">Belum ada ringkasan presensi</p></div><Fingerprint class="w-5 h-5 text-[var(--theme-primary)]" /></div>
      <div class="grid grid-cols-4 gap-2 mt-4 text-center"><div v-for="label in ['Hadir', 'Sakit', 'Izin', 'Cuti']" :key="label"><p class="text-sm font-bold text-neutral-400">—</p><p class="text-[10px] text-neutral-500 dark:text-neutral-400">{{ label }}</p></div></div>
    </section>

    <section v-if="profile" class="px-1 text-xs text-neutral-500 dark:text-neutral-400">{{ profile.positionName }} · {{ profile.officeName }}</section>
  </div>
</template>
