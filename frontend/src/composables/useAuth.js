import { ref } from 'vue'
import { useToast } from './useToast'

const currentUser = ref({
  name: 'Admin Digaji.id',
  email: 'admin@digaji.id',
  role: 'Super Admin',
  avatar: '',
  company: 'PT Digaji Indonesia',
})

// Default false agar user langsung melihat tampilan wireframe login saat membuka aplikasi
const isAuthenticated = ref(false)
const isLoggingOut = ref(false)

export function useAuth() {
  const toast = useToast()

  const login = async (identifier = 'admin@digaji.id') => {
    if (identifier.includes('hr')) {
      currentUser.value = {
        name: 'Sarah HR Manager',
        email: identifier,
        role: 'HR & Payroll Lead',
        avatar: '',
        company: 'PT Digaji Indonesia',
      }
    } else if (identifier.includes('staf')) {
      currentUser.value = {
        name: 'Budi Santoso',
        email: identifier,
        role: 'Staf Operasional',
        avatar: '',
        company: 'PT Digaji Indonesia',
      }
    } else {
      currentUser.value = {
        name: 'Admin Digaji.id',
        email: identifier || 'admin@digaji.id',
        role: 'Super Admin',
        avatar: '',
        company: 'PT Digaji Indonesia',
      }
    }

    isAuthenticated.value = true
    try {
      sessionStorage.setItem('digaji_auth', 'true')
    } catch {
      // ignore
    }
    return true
  }

  const startLogout = () => {
    isLoggingOut.value = true
  }

  const finishLogout = () => {
    isAuthenticated.value = false
    isLoggingOut.value = false
    try {
      sessionStorage.removeItem('digaji_auth')
    } catch {
      // ignore
    }
    toast.info('Sesi telah berakhir. Kembali ke form masuk.')
  }

  const logout = () => {
    startLogout()
  }

  return {
    currentUser,
    isAuthenticated,
    isLoggingOut,
    login,
    startLogout,
    finishLogout,
    logout,
  }
}
