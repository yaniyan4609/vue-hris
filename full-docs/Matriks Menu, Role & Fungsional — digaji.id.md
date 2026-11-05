# Matriks Menu, Role & Fungsional — digaji.id

**Turunan PRD v2.3** · 4 November 2026 · Draft

> **Tujuan:** daftar menu dan submenu untuk setiap panel dan role, fungsi tiap submenu, permission yang dipakai, serta relasi datanya. Dipakai sebagai acuan definisi menu di frontend, seed `permission_registry`, dan desain API. Nama tabel mengacu ke dokumen **Desain Database digaji.id**.

## 1. Cara membaca

**Singkatan role preset (Admin Panel):** **OW** Owner · **AT** Admin Tenant · **HM** HR Manager · **HR** HR Staff · **KC** Kepala Cabang · **RM** Regional Manager · **SP** Supervisor · **FN** Finance/Payroll. Karyawan biasa tidak punya Admin Panel. Mereka memakai ESS (Bagian 4).

**Simbol akses default** (berurutan **OW · AT · HM · HR · KC · RM · SP · FN**):

| Simbol | Arti |
| --- | --- |
| ✔ | Akses penuh sesuai fungsi menu, dalam scope role |
| ◐ | Akses terbatas: hanya lihat, hanya aksi tertentu (mis. approve atau ajukan), atau scope sempit |
| — | Menu tidak muncul secara default |

> **Penting:** semua nilai di bawah adalah **preset default** \[USULAN\]. Admin dapat mengubahnya **per user** (Allow, Deny, scope) sesuai PRD Bagian 3. Menu muncul bila user punya `*.lihat` modulnya **dan** paket mengizinkan. Scope default tiap role ada di Bagian 2.

## 2. Preset role

| Kode | Nama | Panel | Scope default | Karakter |
| --- | --- | --- | --- | --- |
| `KARYAWAN` | Karyawan | ESS | Diri sendiri | Dasar. Otomatis ada selama karyawan aktif. Tidak bisa dicabut manual |
| `SPV` | Supervisor | ESS + Admin | Bawahan langsung | Memantau dan menyetujui pengajuan bawahan |
| `KEPALA_CABANG` (KC) | Kepala Cabang | ESS + Admin | Kantor yang ditentukan | Data satu atau beberapa kantor |
| `REGIONAL_MANAGER` (RM) | Regional Manager | ESS + Admin | Wilayah yang ditentukan | Data semua kantor di wilayah |
| `HR_STAFF` (HR) | HR Staff | ESS + Admin | Seluruh tenant (atau dibatasi) | Operasional SDM harian |
| `HR_MANAGER` (HM) | HR Manager | ESS + Admin | Seluruh tenant | Kebijakan, approval SDM, pengelolaan data |
| `FINANCE_PAYROLL` (FN) | Finance / Payroll | ESS + Admin | Seluruh tenant (modul keuangan) | Payroll, kasbon, reimbursement |
| `ADMIN_TENANT` (AT) | Admin Tenant | ESS + Admin | Seluruh tenant | Pengaturan tenant dan hak akses |
| `OWNER` (OW) | Owner / Direktur | Admin (+ ESS bila karyawan) | Seluruh tenant | Ringkasan dan persetujuan strategis |

User dapat memegang **beberapa role sekaligus**. Hak efektif adalah gabungan semua role, dikurangi Deny (PRD 3.2).

**Role platform (Superadmin):** `SUPERADMIN`, `ADMIN_PLATFORM`, `SUPPORT`, `FINANCE_PLATFORM`. Lihat Bagian 5.

**Modul yang perlu ditambahkan ke `permission_registry`** (belum ada di PRD 3.5): `disiplin` (SP & konseling), `terminasi` (resign & terminasi), `aset` (aset karyawan), `agenda`, dan `direktori` (Teman Kantor). Format aksinya sama dengan modul lain.

## 3. Admin Panel (`/admin`)

Kolom **Role** berurutan OW · AT · HM · HR · KC · RM · SP · FN.

### 3.1 Beranda

| Submenu | Fungsi | Permission | Relasi data | Role |
| --- | --- | --- | --- | --- |
| **Dashboard** | KPI karyawan aktif, kehadiran hari ini, pengajuan menunggu, kontrak mendekati akhir, tren bulanan. Biaya payroll hanya untuk yang berhak | `dashboard.lihat`, `dashboard.lihat_keuangan` (hanya OW dan FN) | `employees`, `attendance_events`, `requests`, `contracts`, `payroll_runs` | ✔ ✔ ✔ ✔ ◐ ◐ ◐ ◐ |

### 3.2 SDM & Organisasi

| Submenu | Fungsi | Permission | Relasi data | Role |
| --- | --- | --- | --- | --- |
| **Direktori Karyawan** | Daftar, filter, detail (slide-over), tambah/ubah, arsip, impor CSV, ekspor, riwayat perubahan | `karyawan.*` | `employees` → `employee_assignments` → `positions` → `departments` → `divisions`; `offices`; `memberships`; `files` | ◐ ✔ ✔ ✔ ◐ ◐ ◐ ◐ |
| **Kontrak** | Kontrak PKWT/PKWTT, perpanjangan, peringatan kontrak berakhir | `kontrak.*` | `contracts` → `employees`; `notification_schedules` | ◐ ✔ ✔ ✔ — — — ◐ |
| **Mutasi & Promosi** | Ajukan mutasi, promosi, demosi dengan tanggal efektif. Diterapkan otomatis saat tanggal efektif | `mutasi.*` | `employee_changes` → `employee_assignments` (baris baru); `approval_instances` | ◐ ✔ ✔ ✔ ◐ ◐ — — |
| **SP & Konseling** | Surat peringatan, catatan konseling, riwayat | `disiplin.*` (baru) | `disciplinary_actions` → `employees`; `files` | ◐ ✔ ✔ ✔ ◐ ◐ ◐ — |
| **Resign & Terminasi** | Pengajuan berhenti, exit checklist, tanggal efektif. Saat efektif: cabut role dan sesi, ubah status, picu payroll akhir | `terminasi.*` (baru) | `terminations` → `employees.status`; `user_roles`, `sessions` (dicabut); `payroll_runs` | ◐ ✔ ✔ ✔ ◐ ◐ ◐ ◐ |
| **Aset Karyawan** | Inventaris yang dipinjamkan dan pengembalian | `aset.*` (baru) | `assets`, `employee_assets` → `employees` | — ✔ ✔ ✔ ◐ — — — |
| **Struktur: Wilayah** | Master wilayah | `organisasi.*` | `regions` → `offices` | ◐ ✔ ✔ ◐ — — — — |
| **Struktur: Kantor & Geofence** | Kantor, zona waktu, polygon area presensi (multi-polygon), editor peta | `organisasi.*` | `offices` → `office_geofences`; `employee_allowed_offices` | ◐ ✔ ✔ ◐ — — — — |
| **Struktur: Divisi** | Master divisi | `organisasi.*` | `divisions` → `departments` | ◐ ✔ ✔ ◐ — — — — |
| **Struktur: Departemen** | Master departemen di bawah divisi | `organisasi.*` | `departments` → `positions` | ◐ ✔ ✔ ◐ — — — — |
| **Struktur: Jabatan** | Master jabatan di bawah departemen, grade | `organisasi.*` | `positions` → `employee_assignments` | ◐ ✔ ✔ ◐ — — — — |

### 3.3 Kehadiran & Jadwal

| Submenu | Fungsi | Permission | Relasi data | Role |
| --- | --- | --- | --- | --- |
| **Kehadiran Hari Ini** | Status real-time per karyawan (hadir, terlambat, belum, izin) dengan pembaruan WebSocket | `absensi.lihat`, `absensi.monitor_bawahan` | `shift_instances` → `attendance_events`; `leave_requests` (pengecualian) | ◐ ✔ ✔ ✔ ◐ ◐ ◐ — |
| **Riwayat & Rekap** | Rekap harian dan bulanan, filter, ekspor | `absensi.lihat`, `absensi.ekspor` | `attendance_events`, `attendance_daily`, `shift_instances` | ◐ ✔ ✔ ✔ ◐ ◐ ◐ ◐ |
| **Koreksi Absensi** | Memproses pengajuan koreksi. Event asli tidak diubah | `absensi.koreksi`, `absensi.approve` | `attendance_corrections` → `attendance_events`; `approval_instances` | — ✔ ✔ ✔ ◐ ◐ ◐ — |
| **Monitor Belum Presensi** | Daftar terfilter dari ringkasan notifikasi atasan | `absensi.monitor_bawahan` | `shift_instances`, `attendance_events`, `leave_requests`, `notifications` | — ✔ ✔ ✔ ◐ ◐ ◐ — |
| **Master Shift** | Shift reguler/fleksibel/lintas hari, jendela clock-in/out, toleransi, wajib lokasi dan selfie | `jadwal.*` | `shift_templates` → `work_schedules` | — ✔ ✔ ✔ — — — — |
| **Roster** | Penugasan shift ke karyawan per tanggal atau periode, impor, publikasi | `jadwal.*` | `work_schedules` → `shift_instances` | — ✔ ✔ ✔ ◐ ◐ ◐ — |
| **Hari Libur** | Libur nasional dan libur perusahaan | `jadwal.ubah` | `holidays` → `shift_instances`, `leave_requests` | — ✔ ✔ ✔ — — — — |
| **Tukar Shift** | Pertukaran shift antar karyawan dengan persetujuan | `jadwal.ubah` + alur approval | `shift_swaps` → `work_schedules`; `approval_instances` | — ✔ ✔ ✔ ◐ ◐ ◐ — |

### 3.4 Cuti & Lembur

| Submenu | Fungsi | Permission | Relasi data | Role |
| --- | --- | --- | --- | --- |
| **Pengajuan Cuti/Izin** | Daftar pengajuan, buat atas nama karyawan, setujui/tolak | `cuti.lihat`, `cuti.buat`, `cuti.approve` | `leave_requests` → `leave_balance_ledger`; `approval_instances` | ◐ ✔ ✔ ✔ ◐ ◐ ◐ — |
| **Saldo & Kebijakan** | Jenis cuti, kuota, akrual, saldo per karyawan | `cuti.kelola` | `leave_types`, `leave_policies`, `leave_balances` | — ✔ ✔ ◐ — — — — |
| **Kalender Cuti** | Tampilan kalender cuti tim | `cuti.lihat` | `leave_requests`, `holidays` | ◐ ✔ ✔ ✔ ◐ ◐ ◐ — |
| **Lembur (SPKL)** | Pengajuan lembur, jam diusulkan/disetujui/dibayar | `lembur.*` | `overtime_requests` → `payroll_item_lines`; `attendance_events` | ◐ ✔ ✔ ✔ ◐ ◐ ◐ ◐ |

### 3.5 Pengajuan & Persetujuan

| Submenu | Fungsi | Permission | Relasi data | Role |
| --- | --- | --- | --- | --- |
| **Pusat Persetujuan** | Inbox tugas approval lintas jenis. Hanya menampilkan yang cocok dengan permission + scope approver | `persetujuan.lihat` + `*.approve` | `approval_instances` → `approval_actions`; `requests` | ◐ ✔ ✔ ✔ ◐ ◐ ◐ ◐ |
| **Kasbon** | Pengajuan, plafon, pencairan, cicilan, pelunasan | `kasbon.*` | `employee_loans` → `loan_installments` → `payroll_item_lines` | ◐ ✔ ◐ — — — — ✔ |
| **Penggantian Biaya** | Klaim, kuitansi, validasi duplikasi, status bayar | `reimbursement.*` | `reimbursements`, `reimbursement_categories`, `files` | ◐ ✔ ◐ — ◐ ◐ ◐ ✔ |

### 3.6 Payroll & Kepatuhan

| Submenu | Fungsi | Permission | Relasi data | Role |
| --- | --- | --- | --- | --- |
| **Periode & Payroll Run** | Buka periode, hitung draft, validasi, approval, finalisasi (kunci). Hasil final adalah snapshot | `penggajian.proses`, `.approve`, `.finalisasi`, `.lihat` | `payroll_periods` → `payroll_runs` → `payroll_items` → `payroll_item_lines`. Sumber: `attendance_daily`, `leave_requests`, `overtime_requests`, `employee_loans` | ◐ ◐ ◐ — — — — ✔ |
| **Komponen Gaji** | Tunjangan, potongan, komponen kustom per karyawan | `penggajian.kelola` | `pay_components`, `employee_pay_components` | — ◐ ◐ — — — — ✔ |
| **PPh 21 & BPJS** | Aturan pajak dan BPJS berversi tanggal berlaku | `penggajian.kelola` | `tax_rules`, `bpjs_rules`, `employee_tax_bpjs` | — ◐ ◐ — — — — ✔ |
| **Slip Gaji** | Terbitkan dan lihat slip, unduh PDF | `penggajian.lihat_slip` | `payslips` → `payroll_items`; `files` | ◐ ◐ ◐ — — — — ✔ |
| **Ekspor Bank** | File transfer massal format bank | `penggajian.ekspor_bank` | `payroll_runs`, `employee_bank_accounts`, `export_jobs` | — — — — — — — ✔ |

### 3.7 Pengembangan Talenta \[BERTAHAP\]

| Submenu | Fungsi | Status |
| --- | --- | --- |
| Kinerja | Siklus penilaian dan KPI | Permission dan tabel ditentukan di Fase 5 |
| Rekrutmen | Lowongan, kandidat, proses seleksi | Fase 5 |
| Pelatihan | Program, peserta, sertifikasi | Fase 5 |

### 3.8 Komunikasi & Agenda

| Submenu | Fungsi | Permission | Relasi data | Role |
| --- | --- | --- | --- | --- |
| **Pengumuman / Broadcast** | Draft, pilih target (filter organisasi, pengguna manual), pratinjau penerima, kirim atau jadwalkan | `broadcast.*` (`target_semua`: OW, AT, HM saja) | `broadcasts` → `broadcast_targets` → `broadcast_recipients` → `notifications` | ✔ ✔ ✔ ✔ ◐ ◐ — — |
| **Notifikasi** | Log dan status pengiriman, kesalahan, kirim ulang aman | `notifikasi.kelola` | `notification_deliveries`, `notification_events` | — ✔ ✔ ◐ — — — — |
| **Agenda** | Agenda perusahaan dan acara | `agenda.*` (baru) | `agenda_events` | ◐ ✔ ✔ ✔ ◐ ◐ ◐ — |
| **Teman Kantor** | Direktori ringkas (nama, jabatan, kontak kerja). Hanya field publik | `direktori.lihat` (baru) | `employees` (view terbatas) | ✔ ✔ ✔ ✔ ✔ ✔ ✔ ✔ |

### 3.9 Dokumen, Laporan, Pengaturan

| Submenu | Fungsi | Permission | Relasi data | Role |
| --- | --- | --- | --- | --- |
| **Dokumen Karyawan** | Berkas per karyawan: KTP, ijazah, kontrak, tanggal kedaluwarsa | `dokumen.*` | `employee_documents` → `files`; `document_types` | ◐ ✔ ✔ ✔ ◐ ◐ — ◐ |
| **Dokumen Perusahaan** | Kebijakan, SOP, formulir | `dokumen.*` | `company_documents` → `files` | ◐ ✔ ✔ ✔ ◐ ◐ ◐ ◐ |
| **Template** | Template surat dan dokumen | `dokumen.ubah` | `document_templates` | — ✔ ✔ ◐ — — — — |
| **Laporan** | Laporan per modul, filter, ekspor (mengikuti scope) | `laporan.lihat`, `laporan.ekspor` | View/query baca dari modul terkait; `export_jobs` | ◐ ✔ ✔ ✔ ◐ ◐ ◐ ◐ |
| **Profil Perusahaan** | Identitas, logo, alamat, NPWP perusahaan | `pengaturan.kelola` | `tenants`, `tenant_settings` | — ✔ ◐ — — — — — |
| **Pengguna & Hak Akses** | Akun, role, matriks permission, scope per user, sesi aktif, cabut sesi | `pengguna.*` | `memberships`, `user_roles`, `user_permission_overrides`, `access_scope_rules`, `sessions` | ◐ ✔ ◐ — — — — — |
| **Alur Approval** | Jalur L1/L2 per jenis pengajuan, fallback | `pengaturan.kelola` | `approval_flows` → `approval_flow_steps` | — ✔ ✔ — — — — — |
| **Notifikasi & Reminder** | Aturan, jadwal, jam tenang, template, kanal | `notifikasi.kelola` | `notification_rules`, `notification_templates` | — ✔ ✔ — — — — — |
| **Kebijakan HR** | Toleransi terlambat, aturan lembur, kebijakan absensi | `pengaturan.kelola` | `tenant_settings` | — ✔ ✔ ◐ — — — — |
| **Tema** | Warna, mode, latar | `pengaturan.kelola` | `tenant_settings` | — ✔ ✔ — — — — — |
| **Integrasi & API** \[BERTAHAP\] | Kunci API, webhook | `pengaturan.kelola` | `api_keys`, `webhooks` | — ✔ — — — — — — |
| **Log Audit & Keamanan** | Audit perubahan, login, ekspor/impor, sesi | `audit.lihat`, `audit.ekspor` | `audit_logs`, `login_events`, `sessions` | ◐ ✔ ◐ — — — — — |

## 4. ESS Panel (`/ess`)

Navigasi berupa **bottom navigation** maksimal 5 item. Semua karyawan aktif memiliki ESS lewat role `KARYAWAN` (scope **Diri Sendiri**). Role tambahan (mis. SPV) **tidak** mengubah isi ESS, tetapi menambah akses Admin Panel dan tombol pindah panel.

| Bottom nav | Submenu / layar | Fungsi | Permission | Relasi data |
| --- | --- | --- | --- | --- |
| **Beranda** (E01) | Ringkasan | Status kehadiran hari ini, jadwal, saldo cuti, status pengajuan, pengumuman, reminder | (dasar `KARYAWAN`) | `shift_instances`, `attendance_events`, `leave_balances`, `requests`, `notifications` |
| **Absensi** (E02) | Clock in/out | Presensi dengan validasi lokasi, selfie bila diwajibkan, status lokasi | `absensi.lihat` (diri sendiri) | `attendance_events` → `shift_instances`; `office_geofences`; `files` (selfie) |
|  | Riwayat dan kalender | Riwayat presensi, rekap bulanan | `absensi.lihat` | `attendance_events`, `attendance_daily` |
|  | Koreksi | Ajukan koreksi presensi | `absensi.koreksi` (ajukan) | `attendance_corrections` → `approval_instances` |
| **Pengajuan** (E03) | Cuti dan izin/sakit | Ajukan, batalkan, lampiran, lihat saldo | `cuti.buat` (diri sendiri) | `leave_requests`, `leave_balances` |
|  | Lembur | Ajukan SPKL | `lembur.buat` | `overtime_requests` |
|  | Kasbon | Ajukan dan pantau cicilan | `kasbon.buat` | `employee_loans`, `loan_installments` |
|  | Reimbursement | Klaim dengan kuitansi | `reimbursement.buat` | `reimbursements`, `files` |
|  | Tukar shift | Usul tukar dengan rekan | `jadwal.ubah` (terbatas) | `shift_swaps` |
|  | Riwayat dan status | Semua pengajuan dan posisi approval | (dasar) | `requests`, `approval_instances` |
| **Notifikasi** (E07) | Inbox | Belum dibaca, pengajuan, pengingat, broadcast, sistem. Real-time | `notifikasi.lihat_sendiri` | `notifications`, `notification_read_states` |
|  | Preferensi | Kanal dan jenis opsional | `notifikasi.lihat_sendiri` | `notification_preferences`, `push_subscriptions` |
| **Lainnya** | Jadwal dan Agenda (E04) | Jadwal pribadi, agenda perusahaan | (dasar) | `work_schedules`, `shift_instances`, `agenda_events` |
|  | Teman Kantor (E05) | Direktori ringkas | `direktori.lihat` | `employees` (view terbatas) |
|  | Dokumen dan Slip Gaji (E06) | Dokumen pribadi dan perusahaan, slip gaji | `dokumen.lihat`, `penggajian.lihat_slip` (diri sendiri) | `employee_documents`, `company_documents`, `payslips` |
|  | Profil dan Keamanan (E08) | Data diri, ajukan perubahan data, ganti kata sandi, daftar perangkat, Simpan Sesi, keluar | (dasar) | `employees`, `sessions`, `login_events` |
|  | **Pindah ke Admin Panel** | Tombol muncul bila user punya `panel.admin` | `panel.admin` | `user_roles`, `user_permission_overrides` |

**Aturan ESS:** data pribadi tertentu (nama, rekening, NPWP) hanya berubah lewat pengajuan koreksi yang disetujui HR. Slip gaji hanya bisa dibuka pemiliknya. Karyawan yang juga SPV tetap **tidak bisa menyetujui pengajuannya sendiri**.

## 5. Superadmin / Platform (`/platform`)

Platform memakai role terpisah dan tidak berbagi permission dengan tenant. Kolom role: **SA** Superadmin · **AP** Admin Platform · **SU** Support · **FP** Finance Platform.

| Kode | Submenu | Fungsi | Relasi data | SA · AP · SU · FP |
| --- | --- | --- | --- | --- |
| S01 | **Dashboard SaaS** | Tenant aktif/trial/suspended, pertumbuhan, MRR/ARR, invoice jatuh tempo, kesehatan layanan | `tenants`, `subscriptions`, `invoices`, `tenant_usage` | ✔ ✔ ◐ ◐ |
| S02 | **Perusahaan (Tenant)** | Daftar, verifikasi, trial, status, detail, branding, kuota, riwayat. Membuka support session | `tenants` → `subscriptions`; `support_sessions` | ✔ ✔ ◐ ◐ |
| S03 | **Paket & Langganan** | Paket, harga, entitlement per paket, batas, add-on, upgrade/downgrade | `plans` → `plan_entitlements`; `subscriptions` | ✔ ✔ — ◐ |
| S04 | **Billing & Pembayaran** | Tagihan, transaksi, webhook gateway, refund | `invoices` → `payments`; `payment_webhook_events` | ✔ ◐ — ✔ |
| S05 | **Pengguna Platform** | Akun dan role staf platform | `platform_staff` → `users` | ✔ — — — |
| S06 | **Dukungan & Tiket** | Tiket, catatan, penanggung jawab, basis pengetahuan | `support_tickets` → `support_ticket_messages` | ✔ ✔ ✔ — |
| S07 | **Analitik SaaS** | Pertumbuhan, konversi trial, churn, pendapatan, penggunaan modul | `tenants`, `subscriptions`, `tenant_usage` | ✔ ✔ — ◐ |
| S08 | **Pengaturan Platform** | Brand, SMTP, kanal notifikasi platform, integrasi, backup/restore, keamanan | `platform_settings` | ✔ ◐ — — |

**Batas penting:** admin platform **tidak** otomatis boleh membaca data gaji atau dokumen tenant. Akses ke data tenant hanya lewat **support session** (alasan, masa terbatas, indikator UI, audit terpisah). Analitik notifikasi hanya agregat, bukan isi pesan.

## 6. Relasi antar modul

### 6.1 Rantai data utama

```text
tenants
  └─ regions ──▸ offices ──▸ office_geofences
  └─ divisions ──▸ departments ──▸ positions
  └─ employees ──▸ employee_assignments (region, office, division, dept, position, atasan)
        │
        ├─ contracts · employee_changes · documents · bank · tax_bpjs
        ├─ work_schedules ──▸ shift_instances ──▸ attendance_events ──▸ attendance_daily
        ├─ leave_requests · overtime_requests · loans · reimbursements
        └─ tenant_memberships ──▸ user_roles · overrides · access_scope_rules · sessions

requests ──▸ approval_instances ──▸ approval_actions
attendance_daily + leave + overtime + loans ──▸ payroll_items ──▸ payslips
events ──▸ notification_outbox ──▸ notifications ──▸ WebSocket / Web Push
```

### 6.2 Ketergantungan fungsional

| Dari | Ke | Bentuk relasi |
| --- | --- | --- |
| Struktur organisasi | Karyawan, scope, broadcast | Penugasan karyawan menyimpan wilayah, kantor, divisi, departemen, jabatan. Scope dan target broadcast membaca nilai ini |
| Karyawan | Akun login | `tenant_memberships.employee_id` menghubungkan akun ke karyawan. Karyawan tanpa akun tidak punya ESS dan tidak menerima notifikasi |
| Shift master | Roster → shift instance | Roster memakai shift. Shift instance menjadi acuan presensi, reminder, dan payroll. Mengubah master tidak mengubah instance berjalan |
| Absensi | Reminder, cuti, lembur, payroll | Reminder membaca status presensi. Cuti disetujui menjadi pengecualian. Lembur dan presensi masuk payroll |
| Pengajuan (cuti, lembur, kasbon, dst.) | Pusat Persetujuan | Semua jenis memakai satu mesin approval (`requests` → `approval_instances`) |
| Approval | Notifikasi | Setiap perubahan status memicu event ke penerima berhak |
| Cuti | Saldo, payroll | Persetujuan membuat entri ledger saldo. Cuti tak berbayar memengaruhi payroll |
| Kasbon | Payroll | Cicilan menjadi potongan di `payroll_item_lines` |
| Payroll | Slip gaji, ekspor bank | Hasil final menjadi snapshot. Slip dan ekspor bank dibuat dari snapshot |
| Role dan scope | Semua modul | Evaluator akses dipanggil di setiap API, ekspor, impor, WebSocket |
| Terminasi | Akses, payroll | Saat tanggal efektif: status karyawan berubah, role dan sesi dicabut, payroll akhir dipicu |
| Paket langganan | Menu dan API | Entitlement menentukan modul yang boleh tampil dan dipanggil |

## 7. Contoh alur: Iyan (ESS → SPV)

| Langkah | Yang terjadi di data | Yang terlihat |
| --- | --- | --- |
| 1 | `user_roles`: Iyan = `KARYAWAN`. `access_versions` v1 | ESS saja |
| 2 | Admin menambah baris `user_roles` (`SPV`). Versi hak akses naik ke v2 | Pada request berikutnya, panel switcher muncul |
| 3 | Evaluator: permission = `KARYAWAN` ∪ `SPV`. Scope `absensi.lihat` = diri sendiri ∪ bawahan langsung | Admin Panel hanya memuat: Dashboard, Kehadiran Hari Ini, Monitor Belum Presensi, Pusat Persetujuan, Kalender Cuti |
| 4 | Admin menambah `user_permission_overrides`: `cuti.approve` = deny | Menu Pengajuan Cuti tetap tampil (lihat), tombol Setujui hilang. API menolak approve |
| 5 | Role `SPV` dicabut (`revoked_at`). Versi v3 | Admin Panel tertutup. WebSocket mengikuti hak baru. ESS tetap jalan |

## 8. Definisi menu di frontend

Menu **tidak ditulis statis** per role. Frontend menyimpan definisi dan memfilternya dengan hak akses efektif dari API (`GET /me/access`).

```js
// src/data/menuData.js (contoh bentuk)
export const adminMenu = [
  {
    key: 'sdm', label: 'SDM & Organisasi', icon: 'Users', panel: 'admin',
    children: [
      {
        key: 'karyawan', label: 'Direktori Karyawan',
        route: '/admin/sdm/karyawan',
        permission: 'karyawan.lihat',   // menu tampil bila ada
        entitlement: 'core_hr',         // dan paket mengizinkan
      },
      {
        key: 'kontrak', label: 'Kontrak',
        route: '/admin/sdm/kontrak',
        permission: 'kontrak.lihat',
        entitlement: 'core_hr',
      },
    ],
  },
]
```

**Aturan filter:** submenu tampil bila `permission` ada di hak efektif **dan** `entitlement` aktif. Grup tampil bila minimal satu submenu tampil. Router guard memakai definisi yang sama, tetapi keputusan final tetap di backend.

## 9. Catatan untuk keputusan berikutnya

- Tabel role default di Bagian 3 adalah **usulan**. Tinjau per baris sebelum dipakai sebagai seed.
- Lima modul baru (`disiplin`, `terminasi`, `aset`, `agenda`, `direktori`) perlu masuk ke registry di PRD 3.5.
- Pengembangan Talenta belum punya permission dan tabel. Ditentukan di Fase 5.
- Karyawan tanpa akun login (mis. operasional lapangan) perlu keputusan: apakah diberi akun ESS massal atau tetap tanpa akun. Ini memengaruhi absensi, notifikasi, dan slip gaji.
