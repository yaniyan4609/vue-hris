import { createRouter, createWebHistory } from 'vue-router'
import PanelHomeView from '../views/PanelHomeView.vue'
import TenantSelectView from '../views/TenantSelectView.vue'

const router = createRouter({
  history: createWebHistory(),
  routes: [
    { path: '/', name: 'Masuk', component: PanelHomeView },
    { path: '/platform', name: 'Platform', component: PanelHomeView },
    { path: '/admin', name: 'AdminTenant', component: PanelHomeView },
    { path: '/ess', name: 'KaryawanESS', component: PanelHomeView },
    { path: '/pilih-perusahaan', name: 'PilihPerusahaan', component: TenantSelectView },
    { path: '/:pathMatch(.*)*', redirect: '/' },
  ],
})

export default router
