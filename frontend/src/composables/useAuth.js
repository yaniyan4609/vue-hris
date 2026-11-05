import { computed, ref } from 'vue'

const state = ref(null)
const isAuthenticated = computed(() => Boolean(state.value?.user))
const demoAccounts = ref([])
const selectedTenant = computed(() => state.value?.tenants?.find((tenant) => tenant.id === state.value.selectedTenantId) || null)
const currentUser = computed(() => ({
  name: state.value?.user?.name || '',
  email: state.value?.user?.email || '',
  role: state.value?.platform ? 'Superadmin' : selectedTenant.value?.admin ? 'Admin Tenant' : 'Karyawan',
  company: selectedTenant.value?.name || 'Digaji.id',
  avatar: '',
}))

async function request(path, options = {}) {
  const response = await fetch(`/api/v1/auth${path}`, {
    credentials: 'include',
    headers: { 'Content-Type': 'application/json' },
    ...options,
  })
  if (response.status === 204) return null
  const payload = await response.json().catch(() => ({}))
  if (!response.ok) throw new Error(payload.error || 'Permintaan gagal. Coba lagi.')
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
    return state.value
  }

  const demoLogin = async (account) => {
    state.value = await request('/demo-login', {
      method: 'POST',
      body: JSON.stringify({ account }),
    })
    return state.value
  }

  const selectTenant = async (tenantId) => {
    state.value = await request('/select-tenant', {
      method: 'POST',
      body: JSON.stringify({ tenantId }),
    })
    return state.value
  }

  const logout = async () => {
    await request('/logout', { method: 'POST', body: '{}' })
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
    selectTenant,
    logout,
    finishLogout,
  }
}
