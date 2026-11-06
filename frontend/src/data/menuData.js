import { Activity, BarChart3, Bell, BookOpen, BriefcaseBusiness, Building2, CalendarDays, ClipboardCheck, Clock3, CreditCard, FileBarChart, FileText, GraduationCap, HandCoins, HeartHandshake, Home, Layers, MapPinned, Megaphone, Receipt, Settings, UserRound, Users, Wallet } from 'lucide-vue-next'

// Katalog tunggal untuk sidebar, rute, izin, dan status kesiapan modul.
const activePages = new Set(['dashboard-saas', 'tenants', 'staff', 'dashboard', 'employees', 'regions', 'offices', 'divisions', 'departments', 'positions', 'users-access', 'notifications'])
const page = (id, label, route, permission, entitlement = '', kind = 'data') => ({ id, label, route, permission, menuPermission: route.startsWith('/admin') ? `menu.${id}.lihat` : '', entitlement, kind, status: activePages.has(id) ? 'active' : 'preview' })
const templatePage = (id, label, route, kind = 'settings') => ({ ...page(id, label, route, 'platform.dashboard.lihat', '', kind), status: 'active', superadminOnly: true })
const group = (id, label, icon, submenus) => ({ id, label, icon, route: submenus[0].route, submenus: submenus.map((item) => ({ ...item, icon })) })

export const platformMenus = [
  group('beranda', 'Beranda', Home, [page('dashboard-saas', 'Dashboard SaaS', '/platform', 'platform.dashboard.lihat', '', 'dashboard')]),
  group('perusahaan', 'Perusahaan', Building2, [page('tenants', 'Perusahaan (Tenant)', '/platform/tenants', 'platform.perusahaan.lihat')]),
  group('paket', 'Paket', Layers, [page('plans', 'Paket & Langganan', '/platform/plans', 'platform.paket.lihat')]),
  group('billing', 'Billing', CreditCard, [page('billing', 'Billing & Pembayaran', '/platform/billing', 'platform.billing.lihat')]),
  group('pengguna', 'Pengguna', Users, [page('staff', 'Pengguna Platform', '/platform/staff', 'platform.pengguna.lihat')]),
  group('dukungan', 'Dukungan', HeartHandshake, [page('support', 'Dukungan & Tiket', '/platform/support', 'platform.dukungan.lihat')]),
  group('analitik', 'Analitik', BarChart3, [page('analytics', 'Analitik SaaS', '/platform/analytics', 'platform.analitik.lihat', '', 'dashboard')]),
  group('pengaturan', 'Pengaturan', Settings, [page('settings', 'Pengaturan Platform', '/platform/settings', 'platform.pengaturan.lihat', '', 'settings')]),
  group('template', 'Template', BookOpen, [
    templatePage('template-dashboard', 'Dashboard Contoh', '/platform/template/dashboard', 'dashboard'),
    templatePage('template-kanban', 'Contoh Kanban', '/platform/template/kanban', 'dashboard'),
    templatePage('template-data-1', 'Contoh Data 1', '/platform/template/data-1', 'data'),
    templatePage('template-data-2', 'Contoh Data 2', '/platform/template/data-2', 'data'),
    templatePage('template-components', 'Katalog Komponen', '/platform/template/komponen', 'data'),
    templatePage('template-general', 'Pengaturan Umum', '/platform/template/umum'),
    templatePage('template-background', 'Dasar Pertama', '/platform/template/dasar-pertama'),
    templatePage('template-modal', 'Modal', '/platform/template/modal'),
    templatePage('template-notifications', 'Notifikasi', '/platform/template/notifikasi'),
    templatePage('template-security', 'Keamanan', '/platform/template/keamanan'),
  ]),
]

export const adminMenus = [
  group('beranda', 'Beranda', Home, [page('dashboard', 'Dashboard', '/admin', 'dashboard.lihat', '', 'dashboard')]),
  group('sdm', 'SDM', Users, [
    page('employees', 'Direktori Karyawan', '/admin/employees', 'karyawan.lihat', 'core_hr'),
    page('contracts', 'Kontrak', '/admin/sdm/contracts', 'kontrak.lihat', 'core_hr'),
    page('mutations', 'Mutasi & Promosi', '/admin/sdm/mutations', 'mutasi.lihat', 'core_hr'),
    page('discipline', 'SP & Konseling', '/admin/sdm/discipline', 'disiplin.lihat', 'core_hr'),
    page('terminations', 'Resign & Terminasi', '/admin/sdm/terminations', 'terminasi.lihat', 'core_hr'),
    page('assets', 'Aset Karyawan', '/admin/sdm/assets', 'aset.lihat', 'core_hr'),
    page('regions', 'Struktur: Wilayah', '/admin/organization/regions', 'organisasi.lihat', 'core_hr'),
    page('offices', 'Struktur: Kantor & Geofence', '/admin/organization/offices', 'organisasi.lihat', 'core_hr'),
    page('divisions', 'Struktur: Divisi', '/admin/organization/divisions', 'organisasi.lihat', 'core_hr'),
    page('departments', 'Struktur: Departemen', '/admin/organization/departments', 'organisasi.lihat', 'core_hr'),
    page('positions', 'Struktur: Jabatan', '/admin/organization/positions', 'organisasi.lihat', 'core_hr'),
  ]),
  group('kehadiran', 'Kehadiran', Clock3, [
    page('attendance-today', 'Kehadiran Hari Ini', '/admin/attendance/today', 'absensi.lihat', 'attendance'),
    page('attendance-history', 'Riwayat & Rekap', '/admin/attendance/history', 'absensi.lihat', 'attendance'),
    page('attendance-correction', 'Koreksi Absensi', '/admin/attendance/corrections', 'absensi.lihat', 'attendance'),
    page('attendance-monitor', 'Monitor Belum Presensi', '/admin/attendance/monitor', 'absensi.monitor_bawahan', 'attendance'),
    page('shifts', 'Master Shift', '/admin/schedule/shifts', 'jadwal.lihat', 'attendance'),
    page('roster', 'Roster', '/admin/schedule/roster', 'jadwal.lihat', 'attendance'),
    page('holidays', 'Hari Libur', '/admin/schedule/holidays', 'jadwal.lihat', 'attendance'),
    page('shift-swaps', 'Tukar Shift', '/admin/schedule/swaps', 'jadwal.lihat', 'attendance'),
  ]),
  group('cuti', 'Cuti', CalendarDays, [
    page('leave-requests', 'Pengajuan Cuti/Izin', '/admin/leave/requests', 'cuti.lihat', 'leave'),
    page('leave-policy', 'Saldo & Kebijakan', '/admin/leave/policy', 'cuti.lihat', 'leave'),
    page('leave-calendar', 'Kalender Cuti', '/admin/leave/calendar', 'cuti.lihat', 'leave'),
    page('overtime', 'Lembur (SPKL)', '/admin/leave/overtime', 'lembur.lihat', 'leave'),
  ]),
  group('pengajuan', 'Pengajuan', ClipboardCheck, [
    page('approvals', 'Pusat Persetujuan', '/admin/requests/approvals', 'persetujuan.lihat', 'requests'),
    page('loans', 'Kasbon', '/admin/requests/loans', 'kasbon.lihat', 'requests'),
    page('reimbursements', 'Penggantian Biaya', '/admin/requests/reimbursements', 'reimbursement.lihat', 'requests'),
  ]),
  group('payroll', 'Payroll', Wallet, [
    page('payroll-runs', 'Periode & Payroll Run', '/admin/payroll/runs', 'penggajian.lihat', 'payroll'),
    page('pay-components', 'Komponen Gaji', '/admin/payroll/components', 'penggajian.lihat', 'payroll'),
    page('tax-bpjs', 'PPh 21 & BPJS', '/admin/payroll/tax-bpjs', 'penggajian.lihat', 'payroll'),
    page('payslips', 'Slip Gaji', '/admin/payroll/payslips', 'penggajian.lihat', 'payroll'),
    page('bank-export', 'Ekspor Bank', '/admin/payroll/bank-export', 'penggajian.ekspor_bank', 'payroll'),
  ]),
  group('talenta', 'Talenta', GraduationCap, [
    page('performance', 'Kinerja', '/admin/talent/performance', 'talenta.lihat', 'talent'),
    page('recruitment', 'Rekrutmen', '/admin/talent/recruitment', 'talenta.lihat', 'talent'),
    page('training', 'Pelatihan', '/admin/talent/training', 'talenta.lihat', 'talent'),
  ]),
  group('komunikasi', 'Komunikasi', Megaphone, [
    page('broadcasts', 'Pengumuman / Broadcast', '/admin/communication/broadcasts', 'broadcast.lihat', 'communication'),
    page('notifications', 'Notifikasi', '/admin/communication/notifications', 'notifikasi.lihat_sendiri', 'communication'),
    page('agenda', 'Agenda', '/admin/communication/agenda', 'agenda.lihat', 'communication'),
    page('directory', 'Teman Kantor', '/admin/communication/directory', 'direktori.lihat', 'communication'),
  ]),
  group('dokumen', 'Dokumen', FileText, [
    page('employee-documents', 'Dokumen Karyawan', '/admin/documents/employees', 'dokumen.lihat', 'documents'),
    page('company-documents', 'Dokumen Perusahaan', '/admin/documents/company', 'dokumen.lihat', 'documents'),
    page('document-templates', 'Template', '/admin/documents/templates', 'dokumen.lihat', 'documents'),
  ]),
  group('laporan', 'Laporan', FileBarChart, [page('reports', 'Laporan', '/admin/reports', 'laporan.lihat', 'reports')]),
  group('pengaturan', 'Pengaturan', Settings, [
    page('company-profile', 'Profil Perusahaan', '/admin/settings/company', 'pengaturan.lihat', 'core_hr', 'settings'),
    page('users-access', 'Pengguna & Hak Akses', '/admin/settings/users', 'pengguna.lihat', 'core_hr'),
    page('approval-flow', 'Alur Approval', '/admin/settings/approval-flow', 'pengaturan.lihat', 'requests', 'settings'),
    page('reminders', 'Notifikasi & Reminder', '/admin/settings/reminders', 'notifikasi.kelola', 'communication', 'settings'),
    page('hr-policy', 'Kebijakan HR', '/admin/settings/hr-policy', 'pengaturan.lihat', 'core_hr', 'settings'),
    page('theme', 'Tema', '/admin/settings/theme', 'pengaturan.lihat', 'core_hr', 'settings'),
    page('integrations', 'Integrasi & API', '/admin/settings/integrations', 'pengaturan.lihat', 'integrations', 'settings'),
    page('audit', 'Log Audit & Keamanan', '/admin/settings/audit', 'audit.lihat', 'core_hr'),
  ]),
]

export const allPages = [...platformMenus, ...adminMenus].flatMap((item) => item.submenus)
export const pageByPath = (path) => allPages.find((item) => item.route === path)
export const menuByPath = (path) => [...platformMenus, ...adminMenus].find((item) => item.submenus.some((sub) => sub.route === path))
export const visibleAdminMenus = (capabilities, entitlements = {}) => adminMenus.map((item) => ({ ...item, submenus: item.submenus.filter((sub) => capabilities[sub.menuPermission] && capabilities[sub.permission] && (!sub.entitlement || entitlements[sub.entitlement])) })).filter((item) => item.submenus.length)
export const visiblePlatformMenus = (capabilities, isSuperadmin = false) => platformMenus.map((item) => ({ ...item, submenus: item.submenus.filter((sub) => capabilities[sub.permission] && (!sub.superadminOnly || isSuperadmin)) })).filter((item) => item.submenus.length)

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
  { id: 'riwayat', label: 'Riwayat', icon: Activity, route: '/ess/fitur/riwayat' },
  { id: 'slip-gaji', label: 'Slip Gaji', icon: Wallet, route: '/ess/fitur/slip-gaji' },
  { id: 'kasbon', label: 'Kasbon', icon: HandCoins, route: '/ess/fitur/kasbon' },
  { id: 'klaim', label: 'Klaim', icon: Receipt, route: '/ess/fitur/klaim' },
  { id: 'teman', label: 'Teman', icon: Users, route: '/ess/fitur/teman' },
  { id: 'agenda', label: 'Agenda', icon: BookOpen, route: '/ess/fitur/agenda' },
]
