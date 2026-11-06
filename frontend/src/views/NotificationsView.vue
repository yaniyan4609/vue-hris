<script setup>
import { ref, onMounted } from 'vue'
import { useRouter } from 'vue-router'
import { Bell, CheckCheck, RefreshCw } from 'lucide-vue-next'
import BaseButton from '../components/ui/BaseButton.vue'
import BaseEmptyState from '../components/ui/BaseEmptyState.vue'
import { useToast } from '../composables/useToast'
import {
  notifications, notificationsReady, notificationsError, unreadNotifications, realtimeConnected,
  nextNotificationCursor, loadNotifications, loadMoreNotifications,
  markNotificationRead, markAllNotificationsRead,
} from '../composables/useNotifications'

const router = useRouter()
const toast = useToast()
const busy = ref(false)

onMounted(() => { loadNotifications().catch(() => {}) })

function displayTime(value) {
  return new Intl.DateTimeFormat('id-ID', { dateStyle: 'medium', timeStyle: 'short' }).format(new Date(value))
}

async function openItem(item) {
  if (busy.value) return
  busy.value = true
  try {
    if (!item.read_at) await markNotificationRead(item.id)
    if (item.deep_link?.startsWith('/') && !item.deep_link.startsWith('//')) await router.push(item.deep_link)
  } catch (error) { toast.error(error.message, 'Gagal membuka notifikasi') }
  finally { busy.value = false }
}

async function markAll() {
  busy.value = true
  try { await markAllNotificationsRead(); toast.success('Semua notifikasi ditandai dibaca.') }
  catch (error) { toast.error(error.message, 'Gagal mengubah notifikasi') }
  finally { busy.value = false }
}

async function refresh() {
  busy.value = true
  try { await loadNotifications() }
  catch (error) { toast.error(error.message, 'Gagal memuat notifikasi') }
  finally { busy.value = false }
}

async function more() {
  busy.value = true
  try { await loadMoreNotifications() }
  catch (error) { toast.error(error.message, 'Gagal memuat notifikasi') }
  finally { busy.value = false }
}
</script>

<template>
  <section class="w-full max-w-4xl mx-auto space-y-4 text-neutral-800 dark:text-[var(--theme-text-primary)]">
    <div class="flex flex-wrap items-center justify-between gap-3">
      <div><h1 class="text-xl font-bold">Notifikasi</h1><p class="text-xs text-neutral-500 dark:text-[var(--theme-text-muted)] mt-1">{{ unreadNotifications }} belum dibaca · {{ realtimeConnected ? 'Terhubung langsung' : 'Memuat pembaruan' }}</p></div>
      <div class="flex items-center gap-2"><BaseButton variant="outline" size="sm" :disabled="busy" @click="refresh"><RefreshCw class="w-4 h-4" /> Segarkan</BaseButton><BaseButton variant="outline" size="sm" :disabled="busy || !unreadNotifications" @click="markAll"><CheckCheck class="w-4 h-4" /> Tandai dibaca</BaseButton></div>
    </div>
    <div class="rounded-2xl border border-neutral-200 dark:border-[var(--theme-border)] bg-white dark:bg-[var(--theme-surface)] overflow-hidden shadow-sm">
      <div v-if="!notificationsReady" class="p-8 text-sm text-neutral-500 dark:text-[var(--theme-text-muted)]">Memuat notifikasi...</div>
      <div v-else-if="notificationsError" class="p-8 text-sm text-neutral-500 dark:text-[var(--theme-text-muted)]">{{ notificationsError }}</div>
      <BaseEmptyState v-else-if="!notifications.length" :icon="Bell" title="Belum ada notifikasi" description="Pembaruan pekerjaan dan pengumuman akan muncul di sini." class="m-5" />
      <div v-else class="divide-y divide-neutral-100 dark:divide-[var(--theme-border)]">
        <button v-for="item in notifications" :key="item.id" type="button" class="w-full text-left p-4 sm:p-5 flex items-start gap-3 hover:bg-neutral-50 dark:hover:bg-[var(--theme-surface-raised)] focus-visible:outline-2 focus-visible:outline-[var(--theme-primary)]" :class="!item.read_at ? 'bg-[var(--theme-primary-soft)]/20' : ''" :disabled="busy" @click="openItem(item)">
          <span class="w-9 h-9 shrink-0 rounded-xl bg-[var(--theme-primary-soft)] text-[var(--theme-primary)] flex items-center justify-center"><Bell class="w-4 h-4" /></span>
          <span class="min-w-0 flex-1"><span class="flex items-start justify-between gap-2"><span class="text-sm font-semibold">{{ item.title }}</span><span v-if="!item.read_at" class="w-2 h-2 rounded-full bg-[var(--theme-primary)] shrink-0 mt-1.5" /></span><span class="block text-xs text-neutral-500 dark:text-[var(--theme-text-muted)] mt-1">{{ item.body }}</span><span class="block text-[11px] text-neutral-400 dark:text-[var(--theme-text-muted)] mt-2">{{ displayTime(item.created_at) }}</span></span>
        </button>
      </div>
      <div v-if="nextNotificationCursor" class="p-3 text-center border-t border-neutral-100 dark:border-[var(--theme-border)]"><BaseButton variant="ghost" size="sm" :disabled="busy" @click="more">Muat lebih banyak</BaseButton></div>
    </div>
  </section>
</template>
