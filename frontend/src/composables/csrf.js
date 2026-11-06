let tokenPromise

export function clearCsrfToken() {
  tokenPromise = null
}

export async function getCsrfToken() {
  if (!tokenPromise) {
    tokenPromise = fetch('/api/v1/auth/csrf', { credentials: 'include' })
      .then(async (response) => {
        const payload = await response.json().catch(() => ({}))
        if (!response.ok || !payload.data?.csrf_token) throw new Error(payload.error?.message || 'Sesi tidak sah.')
        return payload.data.csrf_token
      })
      .catch((error) => { tokenPromise = null; throw error })
  }
  return tokenPromise
}
