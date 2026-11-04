<script setup>
import { onMounted } from 'vue'
import AppLayout from './components/layout/AppLayout.vue'
import { useTheme } from './composables/useTheme'
import { useThemeColor } from './composables/useThemeColor'

const { initTheme } = useTheme()
const { initThemeColor } = useThemeColor()

onMounted(() => {
  initTheme()
  initThemeColor()
})
</script>

<template>
  <AppLayout>
    <router-view v-slot="{ Component }">
      <Transition
        enter-active-class="transition duration-200 ease-out"
        enter-from-class="opacity-0 translate-y-1"
        enter-to-class="opacity-100 translate-y-0"
        leave-active-class="transition duration-150 ease-in"
        leave-from-class="opacity-100 translate-y-0"
        leave-to-class="opacity-0 -translate-y-1"
        mode="out-in"
      >
        <component :is="Component" />
      </Transition>
    </router-view>
  </AppLayout>
</template>
