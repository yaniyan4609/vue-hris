import { createRouter, createWebHistory } from 'vue-router'
import DashboardView from '../views/template/DashboardView.vue'
import KanbanView from '../views/template/KanbanView.vue'
import DataView from '../views/template/DataView.vue'
import DataKaryawanView from '../views/template/DataKaryawanView.vue'
import SettingView from '../views/template/SettingView.vue'
import KomponenView from '../views/template/KomponenView.vue'

// meta.fixedHeight: halaman data/tabel mengunci tinggi sendiri (AppLayout memakai overflow-hidden)
const routes = [
  {
    path: '/',
    name: 'Beranda',
    component: DashboardView,
  },

  // ===== Menu Template =====
  {
    path: '/template',
    redirect: '/template/komponen',
  },
  {
    path: '/template/komponen',
    name: 'TemplateKomponen',
    component: KomponenView,
    meta: { fixedHeight: true },
  },
  {
    path: '/template/kanban',
    name: 'TemplateKanban',
    component: KanbanView,
  },
  {
    path: '/template/data-1',
    name: 'TemplateData1',
    component: DataView,
    meta: { fixedHeight: true },
  },
  {
    path: '/template/data-2',
    name: 'TemplateData2',
    component: DataKaryawanView,
    meta: { fixedHeight: true },
  },
  {
    path: '/template/umum',
    name: 'TemplateUmum',
    component: SettingView,
  },
  {
    path: '/template/dasar-pertama',
    name: 'TemplateDasarPertama',
    component: SettingView,
  },
  {
    path: '/template/modal',
    name: 'TemplateModal',
    component: SettingView,
  },
  {
    path: '/template/notifikasi',
    name: 'TemplateNotifikasi',
    component: SettingView,
  },
  {
    path: '/template/keamanan',
    name: 'TemplateKeamanan',
    component: SettingView,
  },

  // ===== Rute lama (dialihkan ke menu Template) =====
  { path: '/pesanan', redirect: '/template/kanban' },
  { path: '/pesanan/kanban', redirect: '/template/kanban' },
  { path: '/data', redirect: '/template/data-1' },
  { path: '/data/gunung', redirect: '/template/data-1' },
  { path: '/data/karyawan', redirect: '/template/data-2' },
  { path: '/setting', redirect: '/template/umum' },
  { path: '/setting/umum', redirect: '/template/umum' },
  { path: '/setting/dasar-pertama', redirect: '/template/dasar-pertama' },
  { path: '/setting/modal', redirect: '/template/modal' },
  { path: '/setting/notifikasi', redirect: '/template/notifikasi' },
  { path: '/setting/keamanan', redirect: '/template/keamanan' },

  {
    path: '/login',
    redirect: '/',
  },
  {
    path: '/register',
    redirect: '/',
  },
  {
    path: '/:pathMatch(.*)*',
    redirect: '/',
  },
]

const router = createRouter({
  history: createWebHistory(),
  routes,
})

export default router
