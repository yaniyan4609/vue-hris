import { ref } from 'vue'

const toasts = ref([])
let nextId = 1
const timerMap = new Map()

export function useToast() {
  const add = ({
    title = '',
    message = '',
    type = 'info', // 'loading' | 'success' | 'error' | 'warning' | 'info'
    duration = 4000,
    action = null,
  }) => {
    const id = nextId++
    const toast = {
      id,
      title,
      message,
      type,
      duration,
      action,
      createdAt: Date.now(),
    }

    toasts.value.push(toast)

    if (duration > 0) {
      const timer = setTimeout(() => {
        remove(id)
      }, duration)
      timerMap.set(id, timer)
    }

    return id
  }

  const update = (id, options = {}) => {
    const toast = toasts.value.find((t) => t.id === id)
    if (!toast) return null

    // Clear existing timer if any
    if (timerMap.has(id)) {
      clearTimeout(timerMap.get(id))
      timerMap.delete(id)
    }

    // Update reactive fields
    if (options.message !== undefined) toast.message = options.message
    if (options.title !== undefined) toast.title = options.title
    if (options.type !== undefined) toast.type = options.type
    if (options.action !== undefined) toast.action = options.action

    const duration =
      options.duration !== undefined
        ? options.duration
        : options.type === 'loading'
        ? 0
        : 3500

    toast.duration = duration

    if (duration > 0) {
      const timer = setTimeout(() => {
        remove(id)
      }, duration)
      timerMap.set(id, timer)
    }

    return id
  }

  const remove = (id) => {
    if (timerMap.has(id)) {
      clearTimeout(timerMap.get(id))
      timerMap.delete(id)
    }
    const index = toasts.value.findIndex((t) => t.id === id)
    if (index !== -1) {
      toasts.value.splice(index, 1)
    }
  }

  const clear = () => {
    timerMap.forEach((timer) => clearTimeout(timer))
    timerMap.clear()
    toasts.value = []
  }

  // Specialized helpers with Indonesian default messages
  const loading = (message = 'Menyimpan...', options = {}) => {
    return add({ message, title: '', type: 'loading', duration: 0, ...options })
  }

  const success = (message = 'Perubahan disimpan', title = '', options = {}) => {
    return add({ message, title, type: 'success', duration: 3500, ...options })
  }

  const error = (message = 'Gagal menyimpan perubahan', title = '', options = {}) => {
    return add({ message, title, type: 'error', duration: 4000, ...options })
  }

  const warning = (message = 'Peringatan', title = '', options = {}) => {
    return add({ message, title, type: 'warning', duration: 3500, ...options })
  }

  const info = (message = 'Informasi', title = '', options = {}) => {
    return add({ message, title, type: 'info', duration: 3500, ...options })
  }

  /**
   * Promise toast:
   * Starts with a loading state toast, and when the promise resolves or rejects,
   * updates the EXACT SAME toast to success or error with smooth animation.
   */
  const promise = async (
    promiseOrFn,
    {
      loading: loadingMsg = 'Menyimpan...',
      success: successMsg = 'Perubahan disimpan',
      error: errorMsg = 'Gagal menyimpan perubahan',
    } = {}
  ) => {
    const id = loading(loadingMsg)
    try {
      const result =
        typeof promiseOrFn === 'function' ? await promiseOrFn() : await promiseOrFn
      const finalMsg =
        typeof successMsg === 'function' ? successMsg(result) : successMsg
      update(id, { type: 'success', message: finalMsg, duration: 3500 })
      return result
    } catch (err) {
      const finalMsg =
        typeof errorMsg === 'function' ? errorMsg(err) : (err?.message || errorMsg)
      update(id, { type: 'error', message: finalMsg, duration: 4000 })
      throw err
    }
  }

  return {
    toasts,
    add,
    update,
    remove,
    clear,
    loading,
    success,
    error,
    warning,
    info,
    promise,
  }
}
