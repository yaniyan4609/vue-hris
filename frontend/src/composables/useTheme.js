import { ref, nextTick } from 'vue'

const isDark = ref(false)

// Jumlah pergantian tema yang sedang berjalan (klik cepat bisa menumpuk)
let pendingSwitches = 0

export function useTheme() {
  const applyTheme = (dark) => {
    if (dark) {
      document.documentElement.classList.add('dark')
      localStorage.setItem('template_theme', 'dark')
    } else {
      document.documentElement.classList.remove('dark')
      localStorage.setItem('template_theme', 'light')
    }
  }

  // Deteksi perangkat spesifikasi rendah (Low-End Hardware)
  // Menghindari stuttering clip-path pada GPU dengan bandwidth sempit
  const isLowEndDevice = () => {
    if (typeof window === 'undefined') return false
    const prefersReducedMotion = window.matchMedia && window.matchMedia('(prefers-reduced-motion: reduce)').matches
    if (prefersReducedMotion) return true

    const cores = navigator.hardwareConcurrency || 4
    const memory = navigator.deviceMemory || 4 // dalam GB (Chromium/Android)
    return cores < 4 || memory < 4
  }

  const toggleTheme = async (event) => {
    const canAnimate =
      typeof document !== 'undefined' &&
      document.startViewTransition &&
      !window.matchMedia('(prefers-reduced-motion: reduce)').matches

    const flip = () => {
      isDark.value = !isDark.value
      applyTheme(isDark.value)
    }

    if (!canAnimate) {
      flip()
      return
    }

    const root = document.documentElement

    // Matikan transisi CSS per elemen selama pergantian tema, supaya warna di dalam
    // lingkaran tidak memudar pelan dari warna lama (lihat .theme-switching di style.css)
    pendingSwitches++
    root.classList.add('theme-switching')

    const transition = document.startViewTransition(async () => {
      flip()
      await nextTick()
    })

    try {
      await transition.ready

      if (isLowEndDevice()) {
        // Perangkat low-end: crossfade pendek, tanpa kalkulasi clip-path
        await root.animate(
          { opacity: [0, 1] },
          { duration: 150, easing: 'ease-out', pseudoElement: '::view-transition-new(root)' }
        ).finished
      } else {
        // Pusat lingkaran: koordinat tengah tombol icon tema yang diklik
        const rect = event?.currentTarget?.getBoundingClientRect?.()
        const x = rect ? rect.left + rect.width / 2 : (event?.clientX ?? window.innerWidth - 40)
        const y = rect ? rect.top + rect.height / 2 : (event?.clientY ?? 20)

        // Jarak terjauh ke sudut layar
        const endRadius = Math.hypot(
          Math.max(x, window.innerWidth - x),
          Math.max(y, window.innerHeight - y)
        )

        // Animasi Circular Reveal 650ms dengan kurva GPU-friendly
        await root.animate(
          {
            clipPath: [
              `circle(0px at ${x}px ${y}px)`,
              `circle(${endRadius}px at ${x}px ${y}px)`,
            ],
          },
          {
            duration: 650,
            easing: 'cubic-bezier(0.2, 0.8, 0.2, 1)',
            pseudoElement: '::view-transition-new(root)',
          }
        ).finished
      }
    } catch {
      // Transisi dibatalkan (mis. klik cepat dua kali): state tema tetap sudah diterapkan
    } finally {
      await transition.finished.catch(() => {})
      pendingSwitches = Math.max(0, pendingSwitches - 1)
      if (pendingSwitches === 0) root.classList.remove('theme-switching')
    }
  }

  const initTheme = () => {
    const saved = localStorage.getItem('template_theme')
    const prefersDark = window.matchMedia && window.matchMedia('(prefers-color-scheme: dark)').matches
    if (saved === 'dark' || (!saved && prefersDark)) {
      isDark.value = true
      applyTheme(true)
    } else {
      isDark.value = false
      applyTheme(false)
    }
  }

  return {
    isDark,
    toggleTheme,
    initTheme,
  }
}
