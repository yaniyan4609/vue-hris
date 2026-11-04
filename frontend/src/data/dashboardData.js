// ========================================================
// DATASET ERP OPERASIONAL PT DIGAJI SOLUSI INDONESIA
// Modul: Human Capital Management, Payroll & Enterprise Operations
// ========================================================

// 1. KPI Top Summary Cards
export const kpiSummary = {
  totalOrder: {
    value: 142,
    unit: 'transaksi',
    growth: '+12,7%',
    growthType: 'up',
    subtitle: 'batch payroll bulan ini',
  },
  omzet: {
    value: 384500000,
    formatted: 'Rp 384.500.000',
    growth: '+14,2%',
    growthType: 'up',
    subtitle: 'realisasi payroll disalurkan',
  },
  orderDiproses: {
    value: 36,
    unit: 'tiket',
    subtitle: 'Klaim & lembur dalam antrean',
  },
  pelangganAktif: {
    value: 1248,
    unit: 'pegawai',
    growth: '+4,8%',
    growthType: 'up',
    subtitle: 'karyawan aktif di 6 cabang',
  },
  mesinAktif: {
    aktif: 10,
    total: 10,
    utilisasiPct: 99.8,
    subtitle: '99,8% Uptime SLA',
  },
  stokMenipis: {
    count: 4,
    unit: 'pegawai',
    warning: 'Evaluasi PKWT segera',
    isAlert: true,
  },
}

// 2. Data Tren Penggajian & Alokasi Anggaran (Dual Line / Area Chart)
export const trenDataPresets = {
  '7hari': {
    label: '7 Hari Terakhir',
    dates: ['21 Jan', '22 Jan', '23 Jan', '24 Jan', '25 Jan', '26 Jan', '27 Jan'],
    omzet: [42000000, 48000000, 51000000, 56000000, 69000000, 72000000, 84500000],
    order: [82, 94, 98, 110, 132, 136, 142],
    maxOmzet: 100000000,
    maxOrder: 200,
  },
  '14hari': {
    label: '14 Hari Terakhir',
    dates: ['14 Jan', '16 Jan', '18 Jan', '20 Jan', '22 Jan', '24 Jan', '27 Jan'],
    omzet: [38000000, 41000000, 46000000, 49000000, 58000000, 67000000, 84500000],
    order: [74, 80, 88, 96, 114, 128, 142],
    maxOmzet: 100000000,
    maxOrder: 200,
  },
  'bulan': {
    label: 'Bulan Ini',
    dates: ['Minggu 1', 'Minggu 2', 'Minggu 3', 'Minggu 4'],
    omzet: [75000000, 92000000, 110000000, 138000000],
    order: [310, 420, 540, 680],
    maxOmzet: 160000000,
    maxOrder: 800,
  },
}

// 3. Distribusi SDM per Divisi (Horizontal Bar Chart)
export const layananData = [
  { id: 'tech', label: 'Teknologi & Engineering', count: 426, pct: 34.2, color: '#3b82f6' },
  { id: 'ops', label: 'Operasional & Logistik', count: 310, pct: 24.8, color: '#0ea5e9' },
  { id: 'fin', label: 'Finance & Akuntansi', count: 206, pct: 16.5, color: '#10b981' },
  { id: 'sales', label: 'Sales & Bisnis', count: 155, pct: 12.4, color: '#f59e0b' },
  { id: 'hr', label: 'People Ops & HRD', count: 95, pct: 7.6, color: '#8b5cf6' },
  { id: 'legal', label: 'Legal & Kepatuhan', count: 56, pct: 4.5, color: '#ec4899' },
]

// 4. Distribusi Status Pengajuan & Tiket Payroll (Donut Chart)
export const statusDistribusi = [
  { id: 'selesai', label: 'Disetujui & Cair', count: 78, pct: 54.9, color: '#10b981', softBg: '#ecfdf5' },
  { id: 'diproses', label: 'Verifikasi HR', count: 36, pct: 25.4, color: '#3b82f6', softBg: '#eff6ff' },
  { id: 'siap', label: 'Review Finance', count: 20, pct: 14.1, color: '#f59e0b', softBg: '#fffbeb' },
  { id: 'batal', label: 'Revisi / Ditolak', count: 8, pct: 5.6, color: '#f43f5e', softBg: '#fff1f2' },
]

// 5. Ringkasan Finansial ERP
export const ringkasanKeuangan = {
  pemasukan: 512000000, // Pendapatan operasional
  pengeluaran: 384500000, // Beban payroll & tunjangan
  labaKotor: 127500000, // Margin bersih perusahaan
  labaGrowthPct: '+24,9%',
}

// 6. Utilisasi Modul & Infrastruktur ERP Cloud
export const utilisasiMesin = {
  totalUnit: 10,
  aktifUnit: 10,
  utilisasiPct: 99.8,
  totalSiklus: 1420, // Total request / batch run hari ini
  totalKg: 99.9, // 99.9% Sistem Uptime SLA
}

// 7. Peringatan Kepatuhan & Masa Berlaku Dokumen (Compliance Alerts)
export const inventarisWarning = [
  { id: 1, nama: 'Evaluasi PKWT 4 Karyawan', sisa: 'Batas 31 Jan', status: 'Kritis', badgeVariant: 'danger' },
  { id: 2, nama: 'Rekonsiliasi BPJS Ketenagakerjaan', sisa: 'Cabang JKT & SBY', status: 'Verifikasi', badgeVariant: 'warning' },
  { id: 3, nama: 'Pelaporan PPh 21 Masa Pajak', sisa: 'e-Bupot DJP Siap', status: 'Normal', badgeVariant: 'success' },
  { id: 4, nama: 'Lisensi Modul Payroll Cloud', sisa: '180 Hari Aktif', status: 'Aman', badgeVariant: 'neutral' },
]

// 8. Ringkasan Presensi & Distribusi Tenaga Kerja
export const logistikDanKaryawan = {
  pickup: {
    count: 842,
    growth: '+94,2%',
    label: 'WFO (Kantor)',
  },
  delivery: {
    count: 386,
    growth: '+96,5%',
    label: 'WFH (Remote)',
  },
  absensi: {
    hadir: 1208,
    total: 1248,
    pct: 96.8,
  },
}

// 9. Timeline Aktivitas Operasional ERP Real-Time
export const aktivitasOperasional = [
  {
    id: 'act-1',
    waktu: '10:24',
    judul: 'Slip Gaji Jan 2025 Terbit',
    deskripsi: 'Batch slip gaji untuk 426 staf Divisi Engineering berhasil dikirim ke portal pegawai.',
    icon: 'cart',
    color: 'emerald',
  },
  {
    id: 'act-2',
    waktu: '10:18',
    judul: 'Pencairan Klaim Medis Disetujui',
    deskripsi: 'TRX-250127-0842 • Rp 3.450.000 via Mandiri Payroll Transfer.',
    icon: 'wallet',
    color: 'blue',
  },
  {
    id: 'act-3',
    waktu: '10:07',
    judul: 'Presensi Biometrik WFO Tercatat',
    deskripsi: '184 pegawai Cabang Sudirman telah check-in tepat waktu via mobile app.',
    icon: 'truck',
    color: 'sky',
  },
  {
    id: 'act-4',
    waktu: '09:56',
    judul: 'Sinkronisasi e-Bupot PPh 21',
    deskripsi: 'API integrasi ke server perpajakan DJP berhasil sinkron 100% tanpa anomali.',
    icon: 'machine',
    color: 'purple',
  },
  {
    id: 'act-5',
    waktu: '09:41',
    judul: 'Pembaruan Struktur Kontrak PKWT',
    deskripsi: '4 kontrak diperpanjang otomatis menjadi Karyawan Tetap oleh Divisi HRD.',
    icon: 'box',
    color: 'amber',
  },
]

// 10. Tabel Pengajuan & Transaksi Payroll Terbaru
export const orderTerbaruList = [
  {
    noOrder: 'PAY-250127-0842',
    pelanggan: 'Budi Santoso, S.Kom',
    layanan: 'Gaji Pokok & Tunjangan Tetap',
    berat: 'Engineering',
    total: 18500000,
    status: 'Berhasil Cair',
    statusVariant: 'success',
    waktu: '10:24',
  },
  {
    noOrder: 'CLM-250127-0841',
    pelanggan: 'Siti Rahmawati, S.E.',
    layanan: 'Reimburse Perjalanan Dinas',
    berat: 'Finance & Tax',
    total: 2450000,
    status: 'Diproses HR',
    statusVariant: 'info',
    waktu: '10:20',
  },
  {
    noOrder: 'INS-250127-0840',
    pelanggan: 'Andi Pratama, B.B.A.',
    layanan: 'Bonus & Insentif Kinerja Q4',
    berat: 'Sales & Marketing',
    total: 8750000,
    status: 'Disetujui',
    statusVariant: 'success',
    waktu: '10:18',
  },
  {
    noOrder: 'CLM-250127-0839',
    pelanggan: 'Dewi Lestari, S.Psi',
    layanan: 'Klaim Rawat Jalan & Resep',
    berat: 'People Ops',
    total: 1250000,
    status: 'Review Finance',
    statusVariant: 'warning',
    waktu: '10:02',
  },
  {
    noOrder: 'LMB-250127-0838',
    pelanggan: 'Rizky Maulana',
    layanan: 'Upah Lembur Server Maintenance',
    berat: 'DevOps & Infra',
    total: 3200000,
    status: 'Berhasil Cair',
    statusVariant: 'success',
    waktu: '09:56',
  },
]
