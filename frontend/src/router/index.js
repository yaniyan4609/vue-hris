import { createRouter, createWebHistory } from 'vue-router'
import PanelHomeView from '../views/PanelHomeView.vue'
import TenantSelectView from '../views/TenantSelectView.vue'
import PlatformTenantsView from '../views/PlatformTenantsView.vue'
import OrganizationView from '../views/OrganizationView.vue'
import EmployeesView from '../views/EmployeesView.vue'
import AccessUsersView from '../views/AccessUsersView.vue'
import EssHomeView from '../views/EssHomeView.vue'
import EssFeatureView from '../views/EssFeatureView.vue'
import NotificationsView from '../views/NotificationsView.vue'
import EssMoreView from '../views/EssMoreView.vue'
import ModulePreviewView from '../views/ModulePreviewView.vue'
import PlatformStaffView from '../views/PlatformStaffView.vue'
import { allPages } from '../data/menuData'

const templateViews = {
  'template-dashboard': () => import('../views/template/DashboardView.vue'),
  'template-kanban': () => import('../views/template/KanbanView.vue'),
  'template-data-1': () => import('../views/template/DataView.vue'),
  'template-data-2': () => import('../views/template/DataKaryawanView.vue'),
  'template-components': () => import('../views/template/KomponenView.vue'),
  'template-general': () => import('../views/template/SettingView.vue'),
  'template-background': () => import('../views/template/SettingView.vue'),
  'template-modal': () => import('../views/template/SettingView.vue'),
  'template-notifications': () => import('../views/template/SettingView.vue'),
  'template-security': () => import('../views/template/SettingView.vue'),
}

const router = createRouter({
  history: createWebHistory(),
  routes: [
    { path: '/', name: 'Masuk', component: PanelHomeView },
    { path: '/platform', name: 'Platform', component: PanelHomeView },
    { path: '/platform/tenants', name: 'PlatformTenants', component: PlatformTenantsView, meta: { fixedHeight: true } },
    { path: '/platform/staff', name: 'PlatformStaff', component: PlatformStaffView, meta: { fixedHeight: true } },
    { path: '/admin', name: 'AdminTenant', component: PanelHomeView },
    { path: '/admin/employees', name: 'Employees', component: EmployeesView, meta: { fixedHeight: true } },
    { path: '/admin/settings/users', name: 'AccessUsers', component: AccessUsersView, meta: { fixedHeight: true } },
    { path: '/admin/organization/:kind', name: 'Organization', component: OrganizationView, meta: { fixedHeight: true } },
    { path: '/ess', name: 'KaryawanESS', component: EssHomeView },
    { path: '/ess/absensi', name: 'EssAbsensi', component: EssFeatureView },
    { path: '/ess/pengajuan', name: 'EssPengajuan', component: EssFeatureView },
    { path: '/ess/notifikasi', name: 'EssNotifikasi', component: NotificationsView },
    { path: '/ess/lainnya', name: 'EssLainnya', component: EssMoreView },
    { path: '/ess/fitur/:feature', name: 'EssFeature', component: EssFeatureView },
    { path: '/pilih-perusahaan', name: 'PilihPerusahaan', component: TenantSelectView },
    ...allPages.filter((item) => !['/admin', '/platform', '/platform/tenants', '/platform/staff', '/admin/employees', '/admin/settings/users'].includes(item.route) && !item.route.startsWith('/admin/organization/')).map((item) => ({ path: item.route, name: `Module_${item.id}`, component: item.route === '/admin/communication/notifications' ? NotificationsView : templateViews[item.id] || ModulePreviewView, meta: { fixedHeight: item.kind === 'data', permission: item.permission, entitlement: item.entitlement, superadminOnly: item.superadminOnly } })),
    { path: '/:pathMatch(.*)*', redirect: '/' },
  ],
})

export default router
