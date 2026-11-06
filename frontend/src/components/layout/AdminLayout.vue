<script setup>
import { computed, onMounted, ref, watch } from 'vue'
import { useRoute } from 'vue-router'
import { ChevronRight } from 'lucide-vue-next'
import { useAuth } from '../../composables/useAuth'
import { useToast } from '../../composables/useToast'
import { adminCapabilities, platformCapabilities, activeEntitlements, effectiveAccess, refreshAdminCapabilities } from '../../composables/useCapabilities'
import { menuByPath, pageByPath, visibleAdminMenus, visiblePlatformMenus } from '../../data/menuData'
import BaseButton from '../ui/BaseButton.vue'
import PrimarySidebar from './PrimarySidebar.vue'
import SubmenuSidebar from './SubmenuSidebar.vue'

const props = defineProps({ mobileDrawerOpen: { type: Boolean, default: false } })
const emit = defineEmits(['close-drawer'])
const route = useRoute()
const toast = useToast()
const { state, selectedTenant } = useAuth()
const submenuCollapsed = ref(true)
const accessReady = ref(false)
const menus = computed(() => state.value?.platform ? visiblePlatformMenus(platformCapabilities.value, Boolean(effectiveAccess.value?.platformSuperadmin)) : visibleAdminMenus(adminCapabilities.value, activeEntitlements.value))
const activeMenuId = computed(() => menuByPath(route.path)?.id || 'beranda')
const currentMenu = computed(() => menus.value.find((item) => item.id === activeMenuId.value) || menus.value[0])
const authorized = computed(() => {
  const item = pageByPath(route.path)
  if (!item) return false
  if (state.value?.platform) return Boolean(platformCapabilities.value[item.permission] && (!item.superadminOnly || effectiveAccess.value?.platformSuperadmin))
  return Boolean(adminCapabilities.value[item.menuPermission] && adminCapabilities.value[item.permission] && (!item.entitlement || activeEntitlements.value[item.entitlement]))
})

async function loadCapabilities() {
  if (!state.value?.platform && !selectedTenant.value?.admin) return
  accessReady.value = false
  try {
    await refreshAdminCapabilities()
  } catch (error) {
    adminCapabilities.value = {}
    platformCapabilities.value = {}
    toast.error(error.message, 'Gagal memuat menu')
  } finally { accessReady.value = true }
}

onMounted(loadCapabilities)
watch([() => selectedTenant.value?.id, () => state.value?.platform], loadCapabilities)
watch(() => route.path, () => emit('close-drawer'))
function selectMenu(id) { submenuCollapsed.value = false; emit('close-drawer') }
</script>

<template>
  <div class="h-full min-h-0 flex overflow-hidden relative">
    <PrimarySidebar :items="menus" :active-id="activeMenuId" :submenu-collapsed="submenuCollapsed" class="hidden lg:flex" @select="selectMenu" @open-submenu="submenuCollapsed = false" />
    <SubmenuSidebar v-if="currentMenu?.submenus?.length" :submenus="currentMenu.submenus" :menu-title="currentMenu.label" :collapsed="submenuCollapsed" class="hidden lg:flex" @toggle-collapse="submenuCollapsed = !submenuCollapsed" />
    <BaseButton v-if="submenuCollapsed && currentMenu?.submenus?.length" variant="outline" size="sm" class="hidden lg:flex absolute left-18 top-6 z-30 !rounded-full !p-1" title="Buka submenu" aria-label="Buka submenu" @click="submenuCollapsed = false"><ChevronRight class="w-4 h-4" /></BaseButton>
    <main class="flex-1 min-h-0 flex flex-col p-3.5 sm:p-5 lg:p-6 bg-neutral-50/60 dark:bg-[var(--theme-layer-main)] text-neutral-800 dark:text-[var(--theme-text-primary)]" :class="route.meta.fixedHeight ? 'overflow-hidden' : 'overflow-y-auto'">
      <slot v-if="accessReady && authorized" />
      <div v-else-if="accessReady" class="w-full max-w-xl mx-auto mt-8 rounded-2xl border border-neutral-200 dark:border-[var(--theme-border)] bg-white dark:bg-[var(--theme-surface)] p-6 text-center"><h1 class="font-semibold text-neutral-900 dark:text-white">Akses tidak tersedia</h1><p class="text-sm text-neutral-500 dark:text-neutral-400 mt-2">Menu ini tidak termasuk izin dan paket akun Anda.</p></div>
    </main>

    <Transition enter-active-class="transition-opacity duration-300" enter-from-class="opacity-0" leave-active-class="transition-opacity duration-200" leave-to-class="opacity-0">
      <div v-if="props.mobileDrawerOpen" class="lg:hidden fixed inset-0 z-40 bg-black/60" @click="emit('close-drawer')" />
    </Transition>
    <Transition enter-active-class="transition-transform duration-300 ease-[cubic-bezier(0.16,1,0.3,1)]" enter-from-class="-translate-x-full" leave-active-class="transition-transform duration-200" leave-to-class="-translate-x-full">
      <div v-if="props.mobileDrawerOpen" class="lg:hidden fixed inset-y-0 left-0 z-50 flex h-full rounded-r-[32px] border-r-2 border-neutral-300 dark:border-[var(--theme-border-strong)] bg-white dark:bg-[var(--theme-layer-2)] shadow-2xl overflow-hidden max-w-[calc(100vw-24px)]">
        <PrimarySidebar :items="menus" :active-id="activeMenuId" capsule @select="selectMenu" />
        <SubmenuSidebar v-if="currentMenu?.submenus?.length" :submenus="currentMenu.submenus" :menu-title="currentMenu.label" drawer-mode @navigate="emit('close-drawer')" @toggle-collapse="emit('close-drawer')" />
      </div>
    </Transition>
  </div>
</template>
