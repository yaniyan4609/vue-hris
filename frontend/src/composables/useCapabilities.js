import { ref } from 'vue'
import { apiRequest } from './useApi'

export const adminCapabilities = ref({})
export const effectiveAccess = ref(null)

export async function refreshAdminCapabilities() {
  effectiveAccess.value = await apiRequest('/me/access')
  adminCapabilities.value = Object.fromEntries(
    Object.entries(effectiveAccess.value.permissions || {}).map(([code, entry]) => [code, entry.allowed])
  )
  return adminCapabilities.value
}
