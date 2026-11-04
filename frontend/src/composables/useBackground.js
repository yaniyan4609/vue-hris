import { ref, onMounted } from 'vue'

export const bgType = ref('grid') // 'grid' | 'dots' | 'solid' | 'image'
export const solidColor = ref('#f8fafc')
export const imageUrl = ref('https://images.unsplash.com/photo-1618005182384-a83a8bd57fbe?auto=format&fit=crop&w=1920&q=80')

export function useBackground() {
  const setBgType = (type) => {
    bgType.value = type
    localStorage.setItem('template_bg_type', type)
  }

  const setSolidColor = (color) => {
    solidColor.value = color
    localStorage.setItem('template_solid_color', color)
  }

  const setImageUrl = (url) => {
    imageUrl.value = url
    localStorage.setItem('template_image_url', url)
  }

  const initBackground = () => {
    const savedType = localStorage.getItem('template_bg_type')
    const savedColor = localStorage.getItem('template_solid_color')
    const savedImage = localStorage.getItem('template_image_url')

    if (savedType) bgType.value = savedType
    if (savedColor) solidColor.value = savedColor
    if (savedImage) imageUrl.value = savedImage
  }

  return {
    bgType,
    solidColor,
    imageUrl,
    setBgType,
    setSolidColor,
    setImageUrl,
    initBackground,
  }
}

