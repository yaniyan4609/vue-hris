import { ref } from 'vue'

export const themeColor = ref('default') // 'default' | 'pink' | 'blue' | 'terracotta'

export const themePalettes = {
  default: {
    id: 'default',
    name: 'Hitam / Netral (Default)',
    subtitle: 'Monokrom & Pure Neutral Dark (Zero AI Tint)',
    primaryHex: '#18181b',
    darkPrimaryHex: '#ffffff',
    bgHex: '#f8fafc',
    darkBgHex: '#09090b',
    previewHex: '#18181b',
  },
  pink: {
    id: 'pink',
    name: 'Rose Berry (Pink)',
    subtitle: 'Identitas utama tema Pink feminin & modern',
    primaryHex: '#C83D6A',
    darkPrimaryHex: '#E85D88',
    bgHex: '#FFF8FB',
    darkBgHex: '#140810',
    previewHex: '#E85D88',
  },
  blue: {
    id: 'blue',
    name: 'Apple Slate (Blue)',
    subtitle: 'Identitas tema Slate Blue teknologi & clean',
    primaryHex: '#007AFF',
    darkPrimaryHex: '#007AFF',
    bgHex: '#F8FAFC',
    darkBgHex: '#090e17',
    previewHex: '#007AFF',
  },
  terracotta: {
    id: 'terracotta',
    name: 'Terracotta / Warm Sand',
    subtitle: 'Identitas hangat VisionID: Terracotta Rust & Warm Sand Linen',
    primaryHex: '#c03a18',
    darkPrimaryHex: '#E05D3A',
    bgHex: '#F3F1EC',
    darkBgHex: '#15120f',
    previewHex: '#c03a18',
  },
}

export function useThemeColor() {
  const applyThemeColor = (color) => {
    themeColor.value = color
    if (color === 'default') {
      document.documentElement.removeAttribute('data-theme-color')
    } else {
      document.documentElement.setAttribute('data-theme-color', color)
    }
    localStorage.setItem('template_theme_color', color)
  }

  const setThemeColor = (color) => {
    applyThemeColor(color)
  }

  const initThemeColor = () => {
    const saved = localStorage.getItem('template_theme_color') || 'default'
    applyThemeColor(saved)
  }

  return {
    themeColor,
    themePalettes,
    setThemeColor,
    initThemeColor,
  }
}

