import { Home, LayoutTemplate, Sliders, Palette, Shield, Bell, Layers, Mountain, Users, KanbanSquare, Component } from 'lucide-vue-next'

export const menuItems = [
  {
    id: 'beranda',
    label: 'Beranda',
    icon: Home,
    route: '/',
    submenus: [],
  },
  {
    id: 'template',
    label: 'Template',
    icon: LayoutTemplate,
    route: '/template',
    submenus: [
      { id: 'komponen', label: 'Komponen', icon: Component, route: '/template/komponen' },
      { id: 'kanban', label: 'Contoh Kanban', icon: KanbanSquare, route: '/template/kanban' },
      { id: 'data-1', label: 'Contoh Data 1', icon: Mountain, route: '/template/data-1' },
      { id: 'data-2', label: 'Contoh Data 2', icon: Users, route: '/template/data-2' },
      { id: 'umum', label: 'Pengaturan Umum', icon: Sliders, route: '/template/umum' },
      { id: 'dasar1', label: 'Dasar Pertama (Background)', icon: Palette, route: '/template/dasar-pertama' },
      { id: 'modal', label: 'Modal & Konfirmasi', icon: Layers, route: '/template/modal' },
      { id: 'notifikasi', label: 'Notifikasi & Toast', icon: Bell, route: '/template/notifikasi' },
      { id: 'keamanan', label: 'Keamanan & Akun', icon: Shield, route: '/template/keamanan' },
    ],
  },
]
