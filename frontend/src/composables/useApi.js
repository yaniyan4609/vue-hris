import { getCsrfToken } from './csrf'

export async function apiRequest(path, options = {}) {
  const method = (options.method || 'GET').toUpperCase()
  const headers = new Headers(options.headers || {})
  if (!headers.has('Content-Type')) headers.set('Content-Type', 'application/json')
  if (['POST', 'PUT', 'PATCH', 'DELETE'].includes(method)) headers.set('X-CSRF-Token', await getCsrfToken())
  const response = await fetch(`/api/v1${path}`, {
    credentials: 'include',
    ...options,
    headers,
  })
  if (response.status === 204) return null
  const payload = await response.json().catch(() => ({}))
  if (!response.ok) {
    const error = new Error(payload.error?.message || payload.error || 'Permintaan gagal. Coba lagi.')
    error.status = response.status
    error.code = payload.error?.code
    throw error
  }
  return payload
}
