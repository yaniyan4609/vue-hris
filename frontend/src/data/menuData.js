import { Building2, LayoutDashboard, UserRound, Users } from 'lucide-vue-next'

export const menuItems = [
  { id: 'platform', label: 'Panel Superadmin', icon: LayoutDashboard, route: '/platform', submenus: [] },
  { id: 'admin', label: 'Panel Admin Tenant', icon: Building2, route: '/admin', submenus: [] },
  { id: 'ess', label: 'Portal Karyawan', icon: UserRound, route: '/ess', submenus: [] },
  { id: 'pilih-perusahaan', label: 'Pilih Perusahaan', icon: Users, route: '/pilih-perusahaan', submenus: [] },
]
