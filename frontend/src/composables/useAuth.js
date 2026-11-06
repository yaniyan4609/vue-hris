import { computed, ref } from 'vue'
import { effectiveAccess } from './useCapabilities'
import { clearCsrfToken, getCsrfToken } from './csrf'

const state = ref(null)
const isAuthenticated = computed(() => Boolean(state.value?.user))
const demoAccounts = ref([])
const selectedTenant = computed(() => state.value?.tenants?.find((tenant) => tenant.id === state.value.selectedTenantId) || null)
const currentUser = computed(() => ({
  name: state.value?.user?.name || '',
  email: state.value?.user?.email || '',
  role: state.value?.platform ? (effectiveAccess.value?.platformSuperadmin ? 'Superadmin' : 'Staf Platform') : selectedTenant.value?.admin ? 'Admin Tenant' : 'Karyawan',
  company: selectedTenant.value?.name || 'Digaji.id',
  avatar: '',
}))

async function request(path, options = {}) {
  const method = (options.method || 'GET').toUpperCase()
  const headers = { 'Content-Type': 'application/json', ...options.headers }
  if (['POST', 'PUT', 'PATCH', 'DELETE'].includes(method) && !['/login', '/demo-login', '/password/forgot', '/password/reset'].includes(path)) {
    headers['X-CSRF-Token'] = await getCsrfToken()
  }
  const response = await fetch(`/api/v1/auth${path}`, {
    credentials: 'include',
    ...options,
    headers,
  })
  if (response.status === 204) return null
  const payload = await response.json().catch(() => ({}))
  if (!response.ok) throw new Error(typeof payload.error === 'string' ? payload.error : payload.error?.message || 'Permintaan gagal. Coba lagi.')
  return payload
}

export function defaultPanelRoute(authState = state.value) {
  if (authState?.platform) return '/platform'
  const selected = authState?.tenants?.find((tenant) => tenant.id === authState.selectedTenantId)
  if (selected?.admin) return '/admin'
  if (selected?.ess) return '/ess'
  return '/pilih-perusahaan'
}

export function useAuth() {
  const restore = async () => {
    try {
      state.value = await request('/me')
    } catch (error) {
      if (error.message.startsWith('Sesi ')) state.value = null
      else throw error
    }
    return state.value
  }

  const loadDemoAccounts = async () => {
    try {
      demoAccounts.value = await request('/demo-accounts')
    } catch {
      demoAccounts.value = []
    }
  }

  const login = async (email, password, remember = false) => {
    state.value = await request('/login', {
      method: 'POST',
      body: JSON.stringify({ email, password, remember }),
    })
    clearCsrfToken()
    return state.value
  }

  const demoLogin = async (account) => {
    state.value = await request('/demo-login', {
      method: 'POST',
      body: JSON.stringify({ account }),
    })
    clearCsrfToken()
    return state.value
  }

  const mfaStatus = async () => (await request('/mfa/status')).data
  const mfaSetup = async () => (await request('/mfa/setup', { method: 'POST', body: '{}' })).data
  const mfaConfirm = async (code) => {
    const result = (await request('/mfa/confirm', { method: 'POST', body: JSON.stringify({ code }) })).data
    state.value = result.state
    return result.recovery_codes
  }
  const mfaVerify = async (code) => {
    state.value = (await request('/mfa/verify', { method: 'POST', body: JSON.stringify({ code }) })).data
    return state.value
  }
  const forgotPassword = async (email) => (await request('/password/forgot', { method: 'POST', body: JSON.stringify({ email }) })).data
  const resetPassword = async (token, newPassword) => request('/password/reset', { method: 'POST', body: JSON.stringify({ token, new_password: newPassword }) })

  const selectTenant = async (tenantId) => {
    state.value = await request('/select-tenant', {
      method: 'POST',
      body: JSON.stringify({ tenantId }),
    })
    clearCsrfToken()
    return state.value
  }

  const logout = async () => {
    await request('/logout', { method: 'POST', body: '{}' })
    clearCsrfToken()
  }

  const finishLogout = () => {
    state.value = null
  }

  return {
    state,
    currentUser,
    selectedTenant,
    isAuthenticated,
    demoAccounts,
    restore,
    loadDemoAccounts,
    login,
    demoLogin,
    mfaStatus,
    mfaSetup,
    mfaConfirm,
    mfaVerify,
    forgotPassword,
    resetPassword,
    selectTenant,
    logout,
    finishLogout,
  }
}
