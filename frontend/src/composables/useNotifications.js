import { ref } from 'vue'
import { apiRequest } from './useApi'
import { refreshAdminCapabilities } from './useCapabilities'
import { useAuth } from './useAuth'

export const notifications = ref([])
export const unreadNotifications = ref(0)
export const notificationsReady = ref(false)
export const notificationsError = ref('')
export const realtimeConnected = ref(false)
export const nextNotificationCursor = ref(null)

let socket
let reconnectTimer
let activeKey = ''
let lastNotificationId = ''
let generation = 0
let retry = 0

export async function loadNotifications() {
  if (!activeKey) return
  const current = generation
  const [inbox, count] = await Promise.all([
    apiRequest('/notifications?limit=30'),
    apiRequest('/notifications/unread-count'),
  ])
  if (current !== generation) return
  notificationsError.value = ''
  notifications.value = inbox.data || []
  nextNotificationCursor.value = inbox.meta?.next_cursor || null
  unreadNotifications.value = count.data?.count || 0
  if (!lastNotificationId) lastNotificationId = notifications.value[0]?.id || ''
  notificationsReady.value = true
}

async function recoverNotifications() {
  if (!lastNotificationId) return
  try {
    await apiRequest(`/notifications/sync?after_id=${encodeURIComponent(lastNotificationId)}&limit=100`)
  } catch {
    // The inbox reload below is authoritative even if its old cursor expired.
  }
}

function connect() {
  if (!activeKey || socket || !('WebSocket' in window)) return
  const current = generation
  const protocol = location.protocol === 'https:' ? 'wss:' : 'ws:'
  socket = new WebSocket(`${protocol}//${location.host}/api/v1/realtime/ws`)
  socket.onopen = async () => {
    if (current !== generation) return
    realtimeConnected.value = true
    retry = 0
    socket?.send(JSON.stringify({ type: 'hello', last_notification_id: lastNotificationId }))
    await recoverNotifications()
    await loadNotifications().catch(() => {})
  }
  socket.onmessage = (message) => {
    if (current !== generation) return
    let frame
    try { frame = JSON.parse(message.data) } catch { return }
    if (frame.event === 'notification.created') {
      lastNotificationId = frame.data?.id || lastNotificationId
      loadNotifications().catch(() => {})
    }
    if (frame.event === 'notification.unread_count') unreadNotifications.value = frame.data?.count || 0
    if (frame.event === 'access.changed') refreshAdminCapabilities().catch(() => {})
  }
  socket.onclose = async (event) => {
    if (current !== generation) return
    socket = null
    realtimeConnected.value = false
    if (event.code === 4401) { useAuth().restore().catch(() => {}); return }
    if (event.code === 4408) await refreshAdminCapabilities().catch(() => {})
    if (!activeKey) return
    const delay = Math.min(30000, 1000 * 2 ** Math.min(retry++, 5)) + Math.random() * 400
    reconnectTimer = setTimeout(() => { reconnectTimer = null; connect() }, delay)
  }
  socket.onerror = () => { /* onclose handles reconnection */ }
}

export async function startNotifications(key) {
  if (!key || key === activeKey) return
  stopNotifications()
  activeKey = key
  const current = generation
  try {
    await loadNotifications()
    if (current === generation) connect()
  } catch (error) {
    if (current === generation) {
      notificationsError.value = error.message || 'Notifikasi tidak tersedia.'
      notificationsReady.value = true
      if (!error.status || error.status >= 500) {
        const delay = Math.min(30000, 1000 * 2 ** Math.min(retry++, 5)) + Math.random() * 400
        reconnectTimer = setTimeout(() => { reconnectTimer = null; startNotifications(key) }, delay)
        activeKey = ''
      }
    }
  }
}

export function stopNotifications() {
  generation++
  activeKey = ''
  lastNotificationId = ''
  retry = 0
  if (reconnectTimer) clearTimeout(reconnectTimer)
  reconnectTimer = null
  if (socket) socket.close(1000, 'session changed')
  socket = null
  realtimeConnected.value = false
  notificationsReady.value = false
  notificationsError.value = ''
  notifications.value = []
  nextNotificationCursor.value = null
  unreadNotifications.value = 0
}

export async function markNotificationRead(id) {
  await apiRequest(`/notifications/${encodeURIComponent(id)}/read`, { method: 'POST' })
  await loadNotifications()
}

export async function markAllNotificationsRead() {
  await apiRequest('/notifications/read-all', { method: 'POST' })
  await loadNotifications()
}

export async function loadMoreNotifications() {
  if (!activeKey || !nextNotificationCursor.value) return
  const current = generation
  const page = await apiRequest(`/notifications?limit=30&cursor=${encodeURIComponent(nextNotificationCursor.value)}`)
  if (current !== generation) return
  const seen = new Set(notifications.value.map((item) => item.id))
  notifications.value = [...notifications.value, ...(page.data || []).filter((item) => !seen.has(item.id))]
  nextNotificationCursor.value = page.meta?.next_cursor || null
}
