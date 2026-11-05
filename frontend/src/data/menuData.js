import { Bell, Building2, Home, MapPinned, Network, UserRound, Users, BriefcaseBusiness, CalendarDays, History, Wallet, HandCoins, Receipt, Megaphone, Settings } from 'lucide-vue-next'

// Definisi visual tetap di frontend; izin efektif datang dari API.
export const platformMenus = [
  { id: 'beranda', label: 'Beranda', icon: Home, route: '/platform', submenus: [] },
  { id: 'perusahaan', label: 'Perusahaan', icon: Building2, route: '/platform/tenants', submenus: [] },
]

export const adminMenus = [
  { id: 'beranda', label: 'Beranda', icon: Home, route: '/admin', submenus: [] },
  {
    id: 'sdm', label: 'SDM', icon: Users, route: '/admin/employees',
    submenus: [
      { id: 'employees', label: 'Karyawan', icon: Users, route: '/admin/employees', permission: 'karyawan.lihat' },
      { id: 'regions', label: 'Wilayah', icon: MapPinned, route: '/admin/organization/regions', permission: 'organisasi.lihat' },
      { id: 'offices', label: 'Kantor', icon: Building2, route: '/admin/organization/offices', permission: 'organisasi.lihat' },
      { id: 'divisions', label: 'Divisi', icon: Network, route: '/admin/organization/divisions', permission: 'organisasi.lihat' },
      { id: 'departments', label: 'Departemen', icon: BriefcaseBusiness, route: '/admin/organization/departments', permission: 'organisasi.lihat' },
      { id: 'positions', label: 'Jabatan', icon: BriefcaseBusiness, route: '/admin/organization/positions', permission: 'organisasi.lihat' },
    ],
  },
  {
    id: 'pengaturan', label: 'Pengaturan', icon: Settings, route: '/admin/settings/users',
    submenus: [
      { id: 'users-access', label: 'Pengguna & Hak Akses', icon: Users, route: '/admin/settings/users', permission: 'pengguna.lihat' },
    ],
  },
]

export function visibleAdminMenus(capabilities) {
  return adminMenus.map((item) => ({
    ...item,
    submenus: item.submenus.filter((sub) => capabilities[sub.permission]),
  })).filter((item) => item.id === 'beranda' || item.submenus.length)
}

export const essMenus = [
  { id: 'home', label: 'Beranda', icon: Home, route: '/ess' },
  { id: 'request', label: 'Pengajuan', icon: BriefcaseBusiness, route: '/ess/pengajuan' },
  { id: 'absence', label: 'Absensi', icon: MapPinned, route: '/ess/absensi' },
  { id: 'notifications', label: 'Notifikasi', icon: Bell, route: '/ess/notifikasi' },
  { id: 'more', label: 'Lainnya', icon: UserRound, route: '/ess/lainnya' },
]

export const essQuickActions = [
  { id: 'pengajuan', label: 'Izin/Cuti', icon: BriefcaseBusiness, route: '/ess/pengajuan' },
  { id: 'jadwal', label: 'Jadwal', icon: CalendarDays, route: '/ess/fitur/jadwal' },
  { id: 'riwayat', label: 'Riwayat', icon: History, route: '/ess/fitur/riwayat' },
  { id: 'slip-gaji', label: 'Slip Gaji', icon: Wallet, route: '/ess/fitur/slip-gaji' },
  { id: 'kasbon', label: 'Kasbon', icon: HandCoins, route: '/ess/fitur/kasbon' },
  { id: 'klaim', label: 'Klaim', icon: Receipt, route: '/ess/fitur/klaim' },
  { id: 'teman', label: 'Teman', icon: Users, route: '/ess/fitur/teman' },
  { id: 'agenda', label: 'Agenda', icon: Megaphone, route: '/ess/fitur/agenda' },
]
