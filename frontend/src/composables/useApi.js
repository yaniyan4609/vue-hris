export async function apiRequest(path, options = {}) {
  const response = await fetch(`/api/v1${path}`, {
    credentials: 'include',
    headers: { 'Content-Type': 'application/json' },
    ...options,
  })
  if (response.status === 204) return null
  const payload = await response.json().catch(() => ({}))
  if (!response.ok) throw new Error(payload.error || 'Permintaan gagal. Coba lagi.')
  return payload
}
