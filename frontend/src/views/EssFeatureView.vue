<script setup>
import { computed } from 'vue'
import { useRoute } from 'vue-router'
import { Fingerprint, CalendarDays } from 'lucide-vue-next'
import BaseEmptyState from '../components/ui/BaseEmptyState.vue'

const route = useRoute()
const names = { jadwal: 'Jadwal', riwayat: 'Riwayat Presensi', 'slip-gaji': 'Slip Gaji', kasbon: 'Kasbon', klaim: 'Klaim', teman: 'Teman Kantor', agenda: 'Agenda' }
const title = computed(() => route.path === '/ess/absensi' ? 'Absensi' : route.path === '/ess/pengajuan' ? 'Pengajuan' : route.path === '/ess/notifikasi' ? 'Notifikasi' : names[String(route.params.feature)] || 'Informasi')
const icon = computed(() => route.path === '/ess/absensi' ? Fingerprint : CalendarDays)
</script>

<template>
  <div class="rounded-2xl bg-white dark:bg-[var(--theme-surface)] border border-neutral-200 dark:border-[var(--theme-border)] shadow-sm p-4 min-h-80">
    <h1 class="text-base font-bold text-neutral-900 dark:text-white">{{ title }}</h1>
    <BaseEmptyState :icon="icon" :title="`Belum ada data ${title.toLowerCase()}`" description="Informasi untuk bagian ini akan tampil setelah data tersedia." />
  </div>
</template>
