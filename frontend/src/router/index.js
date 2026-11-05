import { createRouter, createWebHistory } from 'vue-router'
import PanelHomeView from '../views/PanelHomeView.vue'
import TenantSelectView from '../views/TenantSelectView.vue'
import PlatformTenantsView from '../views/PlatformTenantsView.vue'
import OrganizationView from '../views/OrganizationView.vue'
import EmployeesView from '../views/EmployeesView.vue'
import EssHomeView from '../views/EssHomeView.vue'
import EssFeatureView from '../views/EssFeatureView.vue'
import EssMoreView from '../views/EssMoreView.vue'

const router = createRouter({
  history: createWebHistory(),
  routes: [
    { path: '/', name: 'Masuk', component: PanelHomeView },
    { path: '/platform', name: 'Platform', component: PanelHomeView },
    { path: '/platform/tenants', name: 'PlatformTenants', component: PlatformTenantsView, meta: { fixedHeight: true } },
    { path: '/admin', name: 'AdminTenant', component: PanelHomeView },
    { path: '/admin/employees', name: 'Employees', component: EmployeesView, meta: { fixedHeight: true } },
    { path: '/admin/organization/:kind', name: 'Organization', component: OrganizationView, meta: { fixedHeight: true } },
    { path: '/ess', name: 'KaryawanESS', component: EssHomeView },
    { path: '/ess/absensi', name: 'EssAbsensi', component: EssFeatureView },
    { path: '/ess/pengajuan', name: 'EssPengajuan', component: EssFeatureView },
    { path: '/ess/notifikasi', name: 'EssNotifikasi', component: EssFeatureView },
    { path: '/ess/lainnya', name: 'EssLainnya', component: EssMoreView },
    { path: '/ess/fitur/:feature', name: 'EssFeature', component: EssFeatureView },
    { path: '/pilih-perusahaan', name: 'PilihPerusahaan', component: TenantSelectView },
    { path: '/:pathMatch(.*)*', redirect: '/' },
  ],
})

export default router
