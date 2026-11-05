# PRD digaji.id — SaaS HRIS / HRM

**Versi 2.3** · 4 November 2026 · Draft konsolidasi

Dua Panel (Admin & ESS) · Multi-Role · Permission & Data Scope per User · Sesi dengan Toggle “Simpan Sesi” · Real-Time Notification Engine

| Item | Nilai |
| --- | --- |
| Bahasa produk | Indonesia |
| Jenis produk | SaaS HRIS/HRM multi-tenant |
| Platform | Web SPA untuk Superadmin dan Admin Panel (desktop-first); PWA untuk ESS Panel (mobile-first) |
| Stack | Vue 3 + Go (Golang) + MySQL 8.x |
| UI | Template Vue 3 digaji.id. Komponen Base sudah selesai dibuat |

> **Tujuan dokumen:** acuan tunggal bagi Product Owner, desainer, frontend, backend, QA, dan AI coding agent. PRD menjelaskan **apa** yang dibutuhkan dan **bagaimana sistem berperilaku**. Detail kode dan skema database ada di dokumen teknis turunan (Bagian 17).

**Arti label**

| Label | Makna |
| --- | --- |
| **\[WAJIB\]** | Sudah diputuskan Product Owner. Tidak boleh diubah tanpa persetujuan |
| **\[BERTAHAP\]** | Masuk cakupan produk, tetapi tidak harus ada di rilis awal |
| **\[USULAN\]** | Rekomendasi yang perlu ditinjau. Belum final |
| **\[TERBUKA\]** | Butuh keputusan sebelum bagian terkait dikerjakan. **Agent dilarang mengimplementasikannya tanpa bertanya** |

---

## Ringkasan perubahan v2.3

| Topik | v2.2 | v2.3 |
| --- | --- | --- |
| Sesi login | Tidak pernah kedaluwarsa untuk semua | **Toggle “Simpan Sesi”.** ON = tidak expired otomatis. OFF = berakhir saat browser ditutup atau idle |
| Portal | 3 portal terpisah | Superadmin (platform) + **Admin Panel** + **ESS Panel**. Admin dan ESS memakai **satu akun dan satu sesi** |
| Role | Satu role sebagai preset | **Multi-role** per user, masing-masing dengan tanggal berlaku |
| Akses ESS ke Admin | Belum diatur | User ESS yang diberi role berizin (mis. SPV) otomatis mendapat Admin Panel sesuai permission dan scope-nya |
| Permission | Contoh saja | **Permission registry** lengkap: CRUD, impor, ekspor, approve, lihat data sensitif, dan aksi khusus |
| Data scope | Item terbuka | **Final:** Wilayah, Kantor, Divisi, Departemen, Jabatan, plus Seluruh Tenant, Bawahan Langsung, Diri Sendiri. Aturan AND/OR dan hierarki ditetapkan |
| Menu | Pengelompokan usulan | **Struktur menu final** untuk Superadmin, Admin Panel, dan ESS Panel |
| Stack | Gin atau Chi terbuka | Daftar stack lengkap beserta status \[USULAN\] |
| Komponen UI | Mengacu template | Komponen Base selesai. Ditambah komponen domain dan `EssLayout` |
| Acceptance criteria | AC-01 s.d. AC-46 | Ditambah **AC-47 s.d. AC-58** |

---

## Ringkasan produk

**digaji.id** adalah HRIS/HRM berlangganan. Banyak perusahaan (tenant) memakai satu platform, tanpa bisa melihat data tenant lain.

**Tiga area aplikasi:**

1. **Superadmin (platform):** mengelola tenant, paket, billing, dukungan, dan operasional SaaS.
2. **Admin Panel (per tenant):** mengelola karyawan, jadwal, absensi, pengajuan, payroll, dan semua proses SDM, sesuai permission dan scope tiap user.
3. **ESS Panel (per tenant):** absensi, pengajuan, jadwal, slip gaji, notifikasi, dan profil. Tersedia untuk semua karyawan aktif.

### Keputusan tetap \[WAJIB\]

- Frontend **Vue 3**, backend **Go**. Tidak diganti React atau NestJS.
- Memakai **template UI digaji.id**. Tidak membangun ulang desain.
- Dashboard berupa **SPA**: pindah modul tanpa reload penuh.
- ESS adalah **PWA mobile-first**. Admin Panel dan Superadmin **desktop-first** dan tetap responsif.
- Hierarki organisasi **Divisi → Departemen → Jabatan**. Tidak ada unit organisasi “Tim”.
- **Satu akun, banyak role.** Permission dan data scope bisa diatur **per user**.
- **Isolasi tenant**, audit trail, approval, dan notifikasi real-time adalah fondasi lintas modul.
- Sesi memakai **toggle “Simpan Sesi”** (Bagian 10).
- Mendukung multi-cabang: **1 base office + beberapa allowed offices**, geofence **multi-polygon**, shift reguler/fleksibel/lintas hari, dan payroll Indonesia.

### Sasaran produk

- Memudahkan administrasi SDM dari awal bekerja sampai berhenti.
- Membuat operasional harian efisien: absensi, jadwal, approval, penggajian.
- Mendukung perusahaan kecil sampai struktur multi-cabang.
- Menawarkan paket langganan dengan fitur dan kuota yang bisa diatur.
- Menjaga data pelanggan terisolasi, terlacak, dan terlindungi.

### Bukan tujuan rilis awal

Rekrutmen, kinerja, pelatihan, integrasi lanjutan, dan penyaluran gaji otomatis ke bank boleh menyusul. **Fondasi tidak boleh menghalangi modul-modul itu ditambahkan nanti.**

---

## 1. Pengguna, panel, dan akun

### 1.1 Siapa memakai apa

| Pengguna | Panel | Tanggung jawab utama |
| --- | --- | --- |
| Superadmin, Admin Platform, Support, Finance Platform | Platform | Mengelola ekosistem SaaS dan tenant |
| Owner / Direktur | Admin Panel | Ringkasan dan persetujuan strategis |
| Admin Tenant, HR Manager, HR | Admin Panel | Operasional SDM, organisasi, kebijakan |
| Kepala Cabang, Regional Manager | Admin Panel | Data pada kantor atau wilayah yang diizinkan |
| Supervisor / Atasan Langsung | ESS + Admin Panel | Memantau bawahan dan menyetujui pengajuan |
| Finance / Payroll | Admin Panel | Menghitung dan memproses payroll |
| Karyawan | ESS Panel | Absensi, pengajuan, slip gaji, profil |

Nama di atas hanyalah **preset role**. Akses final ditentukan per user (Bagian 3).

### 1.2 Satu akun, dua panel \[WAJIB\]

| Panel | Syarat tampil | Rute |
| --- | --- | --- |
| **ESS Panel** | User terhubung ke karyawan aktif (role dasar `KARYAWAN`, scope Diri Sendiri) | `/ess/*` |
| **Admin Panel** | User memiliki permission gerbang `panel.admin` dari role mana pun atau override user | `/admin/*` |
| **Platform** | Pengguna platform dengan permission platform | `/platform/*` |

Aturan:

1. Panel dipilih dari hak akses efektif, **bukan** dari jenis akun.
2. Jika user punya ESS dan Admin, header menampilkan **panel switcher** (ESS ⇄ Admin). Tanpa login ulang dan tanpa memutus sesi.
3. Role `KARYAWAN` otomatis ada selama status karyawan aktif dan tidak bisa dicabut manual.
4. Akun non-karyawan (mis. owner yang tidak tercatat sebagai karyawan) hanya punya Admin Panel.
5. Permission `panel.admin` dapat diberikan atau ditolak per user. Menolaknya menutup seluruh Admin Panel walau role lain memberi akses.
6. Setiap panel punya route guard, layout, dan navigasi sendiri. Pindah panel tidak me-reload data sensitif panel lain.

### 1.3 Contoh: Iyan dari ESS menjadi SPV

| Langkah | Kondisi |
| --- | --- |
| 1. Awal | Iyan karyawan. Role `KARYAWAN`. Hanya punya ESS Panel |
| 2. Admin menambahkan role `SPV` | Preset SPV berisi `panel.admin`, `absensi.lihat`, `absensi.monitor_bawahan`, `cuti.approve`, dengan scope *Bawahan Langsung* |
| 3. Role Iyan | `KARYAWAN + SPV` |
| 4. Request berikutnya | Panel switcher muncul. Admin Panel hanya menampilkan modul yang boleh ia akses. **Tanpa logout** |
| 5. Penyesuaian | Admin bisa menolak `cuti.approve` khusus Iyan atau mengganti scope-nya ke satu Departemen |
| 6. Role SPV dicabut | Admin Panel tertutup seketika. ESS tetap berjalan |

### 1.4 Siklus role

- Role **ditetapkan oleh admin yang berwenang**. Perubahan Jabatan tidak otomatis memberi role, karena Jabatan dan Role adalah konsep berbeda.
- **\[USULAN\]** Admin dapat mendefinisikan pemetaan *Jabatan → role yang disarankan*. Saat karyawan dipromosikan, sistem menampilkan saran dan admin mengonfirmasi sekali.
- Setiap penugasan role memiliki **tanggal mulai** dan **tanggal berakhir** (opsional), serta riwayat.
- Karyawan **resign atau diberhentikan**: semua role admin dicabut dan semua sesi dihentikan pada tanggal efektif.
- Admin hanya boleh memberi role dan scope yang berada dalam kewenangan delegasinya.

---

## 2. Tenant dan organisasi

### 2.1 Multi-tenant \[WAJIB\]

```text
digaji.id (platform SaaS)
├── Platform: Tenant & Langganan · Billing · Dukungan · Monitoring
└── Tenant Perusahaan
    ├── Organisasi & Kantor
    ├── Pengguna & Karyawan
    ├── Absensi, Cuti, Lembur, Approval
    ├── Payroll & Dokumen
    └── Admin Panel + ESS Panel
```

Semua data bisnis tenant memiliki `tenant_id`. Pembacaan, perubahan, laporan, ekspor, file, cache, dan background job wajib patuh isolasi tenant.

- Tenant ditentukan backend dari sesi dan keanggotaan yang terverifikasi, **bukan** dari `tenant_id` di body request atau URL.
- Modul lintas tenant hanya untuk pengguna platform yang berwenang.
- MySQL tidak punya Row-Level Security. Karena itu isolasi dijaga di aplikasi: `tenant_id` ada di setiap primary/composite index, akses data hanya lewat lapisan data access yang menyuntikkan tenant (termasuk raw query, join, dan worker), serta test otomatis lintas tenant.
- Backup, restore, ekspor, cache, dan objek storage punya strategi pemisahan tenant.
- Satu email dapat tergabung di beberapa tenant lewat `tenant_memberships`. Pergantian tenant lewat tenant switcher. **\[TERBUKA\]** rincian alur login multi-tenant.

### 2.2 Hierarki organisasi \[WAJIB\]

```text
Tenant
├── Wilayah (Region)
│   └── Kantor (Office)
│       └── Area absensi (satu atau lebih polygon)
└── Struktur organisasi
    └── Divisi → Departemen → Jabatan
```

- Satu Divisi punya banyak Departemen. Satu Departemen punya banyak Jabatan. **Tidak ada tingkat organisasi lain.**
- **Lokasi** (Wilayah, Kantor) adalah dimensi terpisah, bukan tingkat di dalam Divisi. Divisi tidak harus berada di satu kantor.
- Karyawan menyimpan penugasan jabatan beserta departemen dan divisi, berikut lokasi kerja, menurut periode berlaku.
- Atasan langsung memakai relasi `reports_to`, bukan unit organisasi.
- **\[USULAN\]** Siapkan konsep `legal_entity` bila satu tenant punya beberapa badan hukum. **\[TERBUKA\]** apakah wajib di MVP.

### 2.3 Penugasan multi-kantor \[WAJIB\]

- Setiap karyawan punya **tepat satu base office aktif**.
- Izin presensi di kantor lain lewat daftar **allowed offices** (dengan tanggal mulai/akhir dan riwayat).
- Presensi wajib lokasi divalidasi terhadap polygon kantor yang diizinkan dan sesuai aturan shift.
- Perubahan penempatan tidak menghapus histori.

---

## 3. Akses: role, permission, dan data scope \[WAJIB\]

Bagian ini adalah inti keamanan produk. Semua evaluasi dilakukan di **backend Go**. UI hanya menyembunyikan tombol dan menu berdasarkan hasil evaluasi backend.

### 3.1 Model

| Konsep | Arti |
| --- | --- |
| **Role** | Kelompok tanggung jawab dan **preset** permission serta scope. Satu user boleh punya **banyak role** |
| **Permission** | Aksi yang boleh dilakukan, format `modul.aksi`, mis. `karyawan.ekspor` |
| **Data scope** | Batas data untuk suatu permission: wilayah, kantor, divisi, departemen, jabatan, dan lainnya |
| **User override** | Pengaturan khusus satu user: izinkan, tolak, atau ganti scope |

```text
User terautentikasi + tenant
        │
  Semua role aktif (bertanggal berlaku)
        │
  Permission = gabungan role + User Allow − User Deny
        │
  Scope per permission (override user, atau gabungan scope role)
        │
  Validasi backend: tenant + permission + scope + entitlement paket
        │
     Allow / Deny
```

### 3.2 Rumus akses efektif

```text
permission_efektif = (gabungan permission semua role aktif ∪ user_allow) − user_deny

scope_efektif(permission) =
    scope override user untuk permission/modul itu, jika ada;
    jika tidak: gabungan (OR) scope dari semua role yang memberi permission itu
```

Aturan:

1. **Deny by default.** Tanpa izin yang sesuai, akses ditolak.
2. **User Deny mengalahkan semuanya**, termasuk izin dari role lain.
3. Scope dihitung **per permission**. Contoh: `absensi.lihat` dari role `KARYAWAN` (diri sendiri) digabung dengan dari role `SPV` (bawahan langsung) menghasilkan diri sendiri + bawahan.
4. Override scope **menggantikan** scope preset untuk modul itu. Menghapus override mengembalikan preset, **bukan** membuka seluruh tenant.
5. Izin tidak bisa melampaui batas tenant, entitlement paket, atau pembatas keamanan platform.
6. User tidak boleh menaikkan hak akses dirinya sendiri. Admin hanya boleh mendelegasikan izin dan scope yang ia kuasai.
7. Tidak ada izin otomatis karena Jabatan.

### 3.3 Data scope \[WAJIB\]

**Dimensi scope yang dapat dipilih:**

| Dimensi | Cakupan | Catatan |
| --- | --- | --- |
| **Wilayah** | Satu atau lebih wilayah | Otomatis mencakup semua kantor di dalamnya |
| **Kantor** | Satu atau lebih kantor | Mengikuti base office karyawan |
| **Divisi** | Satu atau lebih divisi | Otomatis mencakup semua departemen dan jabatan di bawahnya |
| **Departemen** | Satu atau lebih departemen | Mencakup semua jabatan di bawahnya |
| **Jabatan** | Satu atau lebih jabatan | Paling spesifik |
| **Bawahan langsung** | Karyawan dengan `reports_to` = user | Khusus |
| **Diri sendiri** | Data milik user sendiri | Khusus |
| **Seluruh tenant** | Semua data tenant | Hanya bila diberi hak yang sesuai |

**Cara aturan scope digabung:**

| Tingkat | Operasi | Contoh |
| --- | --- | --- |
| Beberapa nilai dalam satu dimensi | **OR** | Kantor A atau Kantor B |
| Beberapa dimensi dalam satu aturan | **AND (irisan)** | Kantor Pusat **dan** Divisi Operasional |
| Beberapa aturan (termasuk dari role berbeda) | **OR (gabungan)** | Aturan 1 atau Aturan 2 |

**Ketentuan:**

- Scope disimpan sebagai **referensi ke node organisasi**, bukan salinan daftar karyawan. Departemen baru di bawah Divisi yang sudah masuk scope otomatis tercakup.
- **Atribut pencocokan \[USULAN\]:** Kantor → base office karyawan. Wilayah → wilayah dari kantor tersebut. Divisi/Departemen/Jabatan → penugasan aktif pada tanggal terkait.
- Modul yang tidak terikat karyawan (mis. pengaturan tenant) hanya mendukung *Seluruh Tenant*.
- Perubahan role, struktur organisasi, status karyawan, dan atasan harus langsung memengaruhi hasil evaluasi.
- Evaluasi scope pada daftar data **diterjemahkan menjadi filter SQL**, bukan cek per baris di aplikasi, supaya performa terjaga.

### 3.4 Contoh: dua user dengan role yang sama

| Pengaturan | Pengguna A | Pengguna B |
| --- | --- | --- |
| Role | `KARYAWAN + SPV` | `KARYAWAN + SPV` |
| `absensi.lihat` | Diizinkan | Diizinkan |
| `cuti.approve` | Diizinkan | **Ditolak khusus user** |
| Scope Absensi | Kantor Pusat | Departemen Keuangan |
| Scope Cuti | Bawahan langsung | Tidak berlaku (izin ditolak) |

Hasilnya berbeda untuk menu, data, ekspor, dan approval, walau role-nya sama.

### 3.5 Permission registry \[WAJIB\]

Semua permission didaftarkan di **satu registry** (seed + tabel). Tiap entri berisi: kode, modul, aksi, penanda *sensitif*, dimensi scope yang didukung, dan dependensi entitlement paket. Permission yang tidak ada di registry tidak boleh dipakai di kode.

**Aksi standar:**

| Aksi | Arti | Ketentuan |
| --- | --- | --- |
| `lihat` | Daftar dan detail | Selalu dalam scope |
| `buat` | Menambah data |  |
| `ubah` | Mengedit data | Mensyaratkan `lihat` |
| `hapus` | Menghapus atau mengarsipkan | Default soft-delete. Hapus massal wajib konfirmasi dan audit |
| `impor` | Impor massal (CSV) | Tiap baris dicek terhadap scope pengimpor. Baris di luar scope ditolak dan masuk laporan error |
| `ekspor` | Ekspor data | Hasil mengikuti scope dan filter. Berjalan sebagai job. Selalu diaudit |
| `unduh` / `cetak` | Dokumen, slip, faktur | Akses file dicek ulang saat diunduh |
| `approve` | Menyetujui, menolak, mengembalikan | Wajib permission **dan** scope mencakup pengajuan |
| `lihat_sensitif` | Membuka field sensitif: gaji, NIK, NPWP, rekening | Level **field**. Tanpa izin ini, kolom disamarkan di layar **dan** ekspor |
| `kelola` | Mengatur konfigurasi modul |  |
| `kelola_akses` | Mengubah permission/scope orang lain | Dibatasi hak delegasi |

**Permission per modul:**

| Modul | Kode | Aksi yang tersedia |
| --- | --- | --- |
| Gerbang panel | `panel` | `admin` |
| Dashboard | `dashboard` | `lihat`, `lihat_keuangan` |
| Karyawan | `karyawan` | `lihat`, `buat`, `ubah`, `hapus`, `impor`, `ekspor`, `lihat_sensitif` |
| Kontrak | `kontrak` | `lihat`, `buat`, `ubah`, `hapus`, `ekspor` |
| Mutasi & Promosi | `mutasi` | `lihat`, `buat`, `ubah`, `approve` |
| Organisasi (wilayah, kantor, geofence, divisi, departemen, jabatan) | `organisasi` | `lihat`, `buat`, `ubah`, `hapus`, `impor`, `ekspor` |
| Absensi | `absensi` | `lihat`, `koreksi`, `approve`, `monitor_bawahan`, `ekspor` |
| Jadwal Kerja | `jadwal` | `lihat`, `buat`, `ubah`, `hapus`, `impor`, `ekspor`, `publikasi` |
| Cuti & Izin | `cuti` | `lihat`, `buat`, `ubah`, `approve`, `kelola`, `ekspor` |
| Lembur | `lembur` | `lihat`, `buat`, `ubah`, `approve`, `ekspor` |
| Penggajian | `penggajian` | `lihat`, `proses`, `approve`, `finalisasi`, `ekspor_bank`, `lihat_slip`, `lihat_sensitif`, `kelola` |
| Kasbon | `kasbon` | `lihat`, `buat`, `approve`, `cairkan`, `ekspor` |
| Penggantian Biaya | `reimbursement` | `lihat`, `buat`, `approve`, `bayar`, `ekspor` |
| Dokumen | `dokumen` | `lihat`, `buat`, `ubah`, `hapus`, `unduh` |
| Broadcast | `broadcast` | `lihat`, `buat`, `ubah`, `kirim`, `target_semua`, `lihat_penerima` |
| Notifikasi | `notifikasi` | `lihat_sendiri`, `kelola` |
| Pusat Persetujuan | `persetujuan` | `lihat`, `delegasikan` |
| Laporan | `laporan` | `lihat`, `ekspor` |
| Pengguna & Akses | `pengguna` | `lihat`, `buat`, `ubah`, `nonaktifkan`, `cabut_sesi`, `kelola_akses` |
| Pengaturan | `pengaturan` | `lihat`, `kelola` |
| Audit | `audit` | `lihat`, `ekspor` |

Nama final disepakati saat menyusun registry. Struktur `modul.aksi` dan daftar aksi di atas mengikat.

### 3.6 Pengaturan akses di UI \[WAJIB\]

Di **Admin → Pengaturan → Pengguna → Detail Pengguna → Hak Akses**:

1. **Role pengguna:** daftar role aktif beserta tanggal berlaku, bisa menambah atau mencabut.
2. **Matriks permission** (`PermissionMatrix`): baris = modul, kolom = aksi. Tiap sel memiliki tiga pilihan: **Ikuti Role / Izinkan / Tolak**.
3. **Scope per modul:** pilih dimensi (Wilayah, Kantor, Divisi, Departemen, Jabatan, Bawahan Langsung, Diri Sendiri, Seluruh Tenant) dan nilainya (multi-select).
4. **Pratinjau akses efektif** sebelum menyimpan, termasuk **sumber** tiap hak (role mana atau override user).
5. **Riwayat perubahan:** siapa, kapan, nilai sebelum dan sesudah, alasan (jika diwajibkan).

### 3.7 Penegakan keamanan \[WAJIB\]

- Backend mengevaluasi tenant + permission + scope + entitlement pada **semua** API, data, ekspor, impor, file, laporan, approval, dan event real-time.
- Cache hak akses memakai **versi hak akses per user**. Perubahan berlaku pada request berikutnya tanpa logout.
- Approver wajib punya `*.approve` dan scope yang mencakup pengajuan itu.
- **User tidak boleh menyetujui pengajuannya sendiri**, walau scope-nya mencakup dirinya.
- Percobaan menaikkan hak sendiri atau mendelegasikan melebihi kewenangan ditolak dan dicatat.
- Semua perubahan role, permission, dan scope tercatat di audit log. **\[USULAN\]** review akses berkala (`access_reviews`) untuk mendeteksi akumulasi hak yang tidak perlu.

---

## 4. Struktur menu dan submenu \[WAJIB\]

### 4.1 Aturan tampil menu

- Menu **tampil** bila user punya permission `lihat` modul tersebut **dan** paket tenant mengizinkan modulnya (entitlement).
- **Grup** tampil bila minimal satu submenu di dalamnya tampil.
- Menu disembunyikan, tetapi **API tetap menolak** akses tanpa izin (AC-03).
- Daftar menu **tidak ditulis statis** di frontend. Frontend memiliki definisi menu (ikon, rute, `permission`, `entitlement`), lalu memfilternya dengan hak akses efektif dari API.
- Menu dan data contoh template (Template, Gunung, Pesanan) **dihapus** dari produk. Hanya dipakai sebagai referensi pola.

### 4.2 Superadmin (`/platform`)

| Kode | Menu | Isi |
| --- | --- | --- |
| S01 | Dashboard SaaS | Tenant aktif/trial/suspended, pertumbuhan, MRR/ARR, invoice, kesehatan layanan |
| S02 | Perusahaan (Tenant) | Daftar, verifikasi, masa percobaan, status, detail, branding, kuota, riwayat |
| S03 | Paket & Langganan | Paket, harga, fitur per paket, batas karyawan/pengguna/cabang/storage, add-on, upgrade/downgrade |
| S04 | Billing & Pembayaran | Tagihan, transaksi, webhook gateway, refund, riwayat |
| S05 | Pengguna Platform | Admin platform, support, finance, role dan permission platform |
| S06 | Dukungan & Tiket | Tiket, catatan, penanggung jawab, basis pengetahuan |
| S07 | Analitik SaaS | Pertumbuhan, konversi trial, churn, pendapatan, penggunaan modul |
| S08 | Pengaturan Platform | Brand, SMTP, kanal notifikasi platform, integrasi, backup/restore, keamanan |

### 4.3 Admin Panel (`/admin`)

20 modul fungsional (A01–A20) dikelompokkan menjadi **11 grup** pada primary rail. Setiap grup membuka submenu di sidebar kedua.

| Grup | Submenu | Modul |
| --- | --- | --- |
| **Beranda** | Dashboard | A01 |
| **SDM & Organisasi** | Direktori Karyawan · Kontrak · Mutasi & Promosi · SP & Konseling · Resign & Terminasi · Aset Karyawan · Struktur Organisasi (Wilayah, Kantor & Geofence, Divisi, Departemen, Jabatan) | A02 |
| **Kehadiran & Jadwal** | Kehadiran Hari Ini · Riwayat & Rekap · Koreksi Absensi · Monitor Belum Presensi · Master Shift · Roster · Hari Libur · Tukar Shift | A03, A04 |
| **Cuti & Lembur** | Pengajuan Cuti/Izin · Saldo & Kebijakan · Kalender Cuti · Lembur (SPKL) | A05, A06 |
| **Pengajuan & Persetujuan** | Pusat Persetujuan · Kasbon · Penggantian Biaya | A08, A09, A18 |
| **Payroll & Kepatuhan** | Periode & Payroll Run · Komponen Gaji · PPh 21 & BPJS · Slip Gaji · Ekspor Bank | A07 |
| **Pengembangan Talenta** \[BERTAHAP\] | Kinerja · Rekrutmen · Pelatihan | A10–A12 |
| **Komunikasi & Agenda** | Pengumuman/Broadcast · Notifikasi · Agenda · Teman Kantor | A13–A15, A19 |
| **Dokumen** | Dokumen Karyawan · Dokumen Perusahaan · Template | A16 |
| **Laporan & Analitik** | Laporan per modul | A17 |
| **Pengaturan** | Profil Perusahaan · Pengguna & Hak Akses · Alur Approval · Notifikasi & Reminder · Kebijakan HR · Tema · Integrasi & API · Log Audit & Keamanan | A20 |

**Dashboard Admin** memakai kartu KPI, grafik, tabel ringkas, dan timeline. Semua angka berasal dari API dan tunduk pada permission serta scope. Data payroll tidak ditampilkan ke user tanpa `dashboard.lihat_keuangan`.

### 4.4 ESS Panel (`/ess`)

ESS tidak memakai dual-sidebar. Navigasi berupa **bottom navigation** yang ramah satu tangan, dengan maksimal 5 item.

| Bottom nav | Isi |
| --- | --- |
| **Beranda** (E01) | Status kehadiran hari ini, jadwal, saldo cuti, status pengajuan, pengumuman, reminder |
| **Absensi** (E02) | Clock in/out, status lokasi, selfie bila diwajibkan, riwayat, kalender, koreksi |
| **Pengajuan** (E03) | Cuti, izin/sakit, lembur, kasbon, reimbursement, riwayat dan status approval |
| **Notifikasi** (E07) | Inbox real-time, belum dibaca, pengajuan, pengingat, broadcast, preferensi |
| **Lainnya** | Jadwal & Agenda (E04) · Teman Kantor (E05) · Dokumen & Slip Gaji (E06) · Profil & Keamanan (E08) · Tombol pindah ke Admin Panel (jika berhak) |

**Aturan ESS:** karyawan pada dasarnya hanya melihat data miliknya. Data tambahan hanya muncul sesuai permission dan scope individualnya. Data pribadi tertentu hanya bisa diubah lewat pengajuan koreksi atau persetujuan HR.

---

## 5. Paket, entitlement, dan lifecycle tenant

### 5.1 Paket dan entitlement \[WAJIB\]

Nama paket (Trial, Starter, Professional, Business, Enterprise) **tidak di-hardcode**. Yang konsisten adalah *entitlement* (modul yang boleh dipakai) dan batas penggunaannya. Menyembunyikan menu tidak cukup, API juga harus menolak fitur di luar paket.

### 5.2 Lifecycle tenant \[WAJIB\]

```text
Didaftarkan → Menunggu Verifikasi → Trial / Aktif
                                       ↓
                           Perpanjang / Upgrade / Downgrade
                                       ↓
                            Grace Period / Suspended
                                       ↓
                             Aktif Kembali / Berhenti
```

Setiap perubahan status memiliki aturan akses, kuota, dampak ke ESS, tanggal efektif, dan audit. **\[TERBUKA\]** masa trial, grace period, perilaku saat suspended (ESS read-only atau diblokir penuh), dan kebijakan penghapusan data setelah berhenti.

### 5.3 Dukungan masuk ke tenant \[BERTAHAP\]

Superadmin dapat membuka *support session* dengan alasan tercatat, masa akses terbatas, indikator UI yang jelas, dan audit yang membedakan aktor platform dari pengguna tenant.

---

## 6. Aturan bisnis inti

### 6.1 Data karyawan dan riwayat \[WAJIB\]

- Identitas karyawan unik **di dalam tenant**.
- Data mencakup identitas, kontak, keluarga, kontak darurat, penempatan, jabatan, kontrak, pajak/BPJS, rekening, dokumen, dan status.
- Perubahan jabatan, grade, gaji, kantor, atasan, dan status menyimpan **tanggal efektif** dan riwayat. Histori tidak ditimpa.
- Karyawan yang berhenti tidak langsung dihapus (audit, arsip, laporan, retensi).
- Impor massal: preview, validasi, laporan error, dan pencegahan duplikasi. Tiap baris dicek terhadap scope pengimpor.

### 6.2 Shift, jadwal, dan shift instance \[WAJIB\]

| Konsep | Arti |
| --- | --- |
| **Shift master** | Aturan jam kerja yang dipakai berulang |
| **Jadwal / roster** | Penugasan shift ke karyawan pada tanggal atau periode |
| **Shift instance** | Kejadian kerja aktual: `work_date`, `expected_start`, `expected_end`, dan snapshot aturan saat berlaku |

Shift mendukung: jam masuk/pulang, jendela clock-in/out, toleransi keterlambatan, istirahat, target jam kerja, wajib lokasi, wajib selfie, fleksibilitas waktu dan lokasi, serta lintas hari.

| Flexible waktu | Flexible lokasi | Perilaku |
| --- | --- | --- |
| Tidak | Tidak | Jam dan area mengikuti shift |
| Ya | Tidak | Jam fleksibel, lokasi tetap wajib |
| Tidak | Ya | Jam ditentukan, lokasi tidak terikat polygon |
| Ya | Ya | Keduanya fleksibel, target durasi dapat tetap berlaku |

**Shift lintas hari:** work date 18 September, masuk 22:00, pulang 06:00 hari berikutnya. Presensi pulang tanggal 19 tetap terkait work date 18.

**Toleransi:** jam masuk 07:00, toleransi 15 menit → sampai 07:15 tepat waktu, mulai 07:16 terlambat. Jendela clock-in dan toleransi adalah konsep berbeda.

Perubahan shift master **tidak** mengubah shift instance atau payroll yang sudah berjalan. **\[USULAN\]** shift instance dibuat (dimaterialisasi) untuk jendela N hari ke depan agar bisa dipakai penjadwal reminder. **\[TERBUKA\]** `work_date` untuk shift fleksibel tanpa jam masuk baku.

### 6.3 Absensi dan geofencing \[WAJIB\]

- Satu kantor boleh punya **banyak polygon**. Validasi memakai Point-in-Polygon (mis. ray casting) pada kantor yang diizinkan.
- Waktu absensi memakai **waktu server**. Waktu perangkat hanya informasi tambahan.
- Setiap event presensi terikat ke karyawan, shift instance, koordinat, akurasi, polygon yang cocok, waktu, status, dan selfie bila diwajibkan.
- Retry, koneksi lambat, atau double tap tidak boleh membuat event ganda (idempotency).
- Perbaikan absensi lewat **alur koreksi dan approval**, bukan mengedit event asli.
- Status minimal: tepat waktu, terlambat, pulang cepat, tidak hadir, di luar area, di luar window, menunggu koreksi, dikoreksi.

**Batas PWA:** GPS browser dan selfie biasa tidak menjamin lokasi asli atau liveness. Sistem memakai validasi akurasi, kewajaran, kamera langsung, dan audit sebagai pengendalian risiko, **tanpa mengklaim bebas fake GPS**. **\[TERBUKA\]** ambang akurasi GPS, toleransi tepi polygon, alur bila lokasi gagal, dan kebutuhan aplikasi native.

### 6.4 Cuti dan izin \[WAJIB\]

- Tenant menentukan jenis, kuota, akrual, lampiran, hari libur, dan aturan approval.
- Pengajuan memiliki periode, alasan, dokumen opsional, approver, status, dan riwayat.
- Pengurangan saldo harus konsisten dan tidak ganda.
- Penolakan, pembatalan, perubahan tanggal, dan pengajuan bertumpang tindih punya aturan eksplisit.
- **\[TERBUKA\]** carry-over, prorata, cuti bersama, saldo negatif.

### 6.5 Lembur \[WAJIB\]

- SPKL dan persetujuan (sebelum atau sesudah) mengikuti kebijakan tenant.
- Pisahkan jam **diusulkan**, **disetujui**, dan **dibayar**.
- Rumus tarif mengikuti aturan yang berlaku pada tanggal periode.
- Lembur yang sudah masuk payroll terhubung ke payroll run dan hanya bisa dikoreksi lewat mekanisme terkontrol.

### 6.6 Approval terpusat \[WAJIB\]

Jenis pengajuan: koreksi absensi, cuti/izin, lembur, kasbon, reimbursement, mutasi, promosi/demosi, payroll, resign, dan jenis baru di masa depan.

```text
Diajukan → Menunggu L1 → Menunggu L2 (jika perlu) → Disetujui
               │              │
               └── Ditolak / Dikembalikan / Dibatalkan
```

- Jalur approval bisa berbeda per jenis pengajuan dan per tenant.
- Approver hanya menerima tugas yang cocok dengan **permission dan scope**.
- Aksi approval **atomik**. Dua approver tidak boleh menghasilkan status akhir yang bertentangan.
- Simpan pelaku, level, keputusan, waktu, alasan, lampiran.
- **Pengaju tidak bisa menyetujui pengajuannya sendiri.**
- **Fallback \[USULAN\]:** bila tidak ada approver yang memenuhi (mis. `reports_to` kosong), pengajuan diarahkan ke HR yang berwenang dan pelaku dicatat. **\[TERBUKA\]** delegasi, eskalasi, dan SLA otomatis di MVP.

### 6.7 Kasbon dan reimbursement

- **Kasbon:** nominal, plafon, approval, pencairan, cicilan, sisa kewajiban, keterkaitan payroll, pelunasan, histori.
- **Reimbursement:** kategori, batas, kuitansi, tanggal transaksi, validasi duplikasi, approval, status bayar.
- Nominal uang **tidak memakai floating point**. Pakai integer rupiah atau decimal.
- Pembayaran tidak boleh terjadi dua kali akibat retry.

### 6.8 Payroll, pajak, dan BPJS \[Fase 4\]

```text
Buka periode → Hitung (Draft) → Validasi → Review & approval
   → Finalisasi & kunci → Ekspor bank → Slip & arsip
```

- Komponen: gaji pokok, tunjangan tetap/variabel, potongan, pinjaman, lembur, pajak, BPJS, dan komponen kustom.
- Mendukung karyawan masuk/keluar di tengah periode dan prorata.
- PPh 21 mendukung skema TER dan perhitungan akhir tahun sesuai aturan yang **berlaku pada periodenya**.
- Tarif, plafon, dan aturan BPJS **berversi dengan tanggal berlaku**, tidak di-hardcode di UI.
- Hasil final adalah **snapshot** yang tidak berubah walau profil atau aturan diedit kemudian. Perbaikan lewat adjustment yang tercatat.
- Slip gaji hanya bisa diakses pihak berwenang. Ekspor Bank CMS berbeda dengan transfer otomatis via API bank (integrasi terpisah).
- **Compliance:** validasi terhadap regulasi Indonesia yang berlaku wajib diulang saat modul diimplementasikan dan saat aturan berubah. Dokumen ini tidak mengesahkan tarif tertentu.
- **\[TERBUKA\]** multi-currency, multi-legal-entity, formula kustom, pembayaran otomatis, format bank.

---

## 7. Real-Time Notification Engine \[WAJIB\]

Satu mesin notifikasi lintas modul, bukan sekadar ikon lonceng. Ketika event bisnis terjadi, sistem menentukan penerima yang sah, menyimpan notifikasi, mengirim pembaruan real-time, dan mencatat hasilnya. Tidak boleh mencampur tenant, dan selalu tunduk pada permission dan scope per user.

### 7.1 Empat kategori

| Kategori | Peristiwa | Penerima utama |
| --- | --- | --- |
| **Pengajuan & persetujuan** | Dibuat, diperbaiki, dibatalkan, diteruskan, disetujui, ditolak, butuh keputusan | Pengaju dan approver berikutnya yang berhak |
| **Pengingat otomatis** | Belum clock-in/out, shift akan mulai, approval tertunda, kontrak mendekati akhir | Karyawan, atasan langsung, HR berwenang |
| **Broadcast / pengumuman** | Pesan langsung atau terjadwal dari pengirim berwenang | Semua user tenant atau subset yang lolos target dan scope |
| **Sistem & layanan** | Billing tenant, keamanan akun, hasil ekspor, pemeliharaan | Pengguna relevan. Pesan platform dipisah dari pesan HR tenant |

Satu peristiwa bisa menghasilkan notifikasi untuk beberapa peran tanpa membuat event bisnis ganda.

### 7.2 Event pengajuan dan penerimanya

| Event | Perilaku minimum |
| --- | --- |
| Pengajuan dibuat | Kirim ke approver tahap pertama yang lolos permission + scope. Konfirmasi di inbox pengaju |
| Satu tahap disetujui | Kirim ke approver tahap berikutnya. Perbarui status di inbox pengaju |
| Disetujui final / ditolak | Beri tahu pengaju beserta alasan yang boleh dilihat dan tautan detail |
| Dikembalikan untuk revisi | Ingatkan pengaju tentang tindakan yang diperlukan |
| Dibatalkan / dialihkan / didelegasikan | Perbarui pihak terkait. Notifikasi actionable lama tidak boleh tetap aktif |
| Menunggu melewati SLA | Kirim reminder atau eskalasi sesuai kebijakan tenant, tanpa mengulang terus-menerus |

- Sumber status final tetap **database transaksi**, bukan notifikasi.
- Approver hanya diberi tahu tentang objek yang boleh ia akses.
- Klik notifikasi membuka detail yang **diperiksa ulang backend**. Bila izin sudah dicabut, akses ditolak.

### 7.3 Reminder absensi untuk karyawan

Dihitung dari `shift_instance`, `work_date`, timezone kantor, window presensi, status absensi aktual, toleransi, izin/cuti, serta aturan shift fleksibel dan lintas hari.

**Contoh ilustratif** (bukan angka baku), shift 08.00–17.00:

| Waktu | Pesan | Kondisi |
| --- | --- | --- |
| 07.45 | “Shift dimulai dalam 15 menit. Jangan lupa absen masuk.” | Belum masuk |
| 08.15 | “Anda belum melakukan absen masuk.” | Belum masuk, masih relevan |
| 17.00 | “Jangan lupa absen pulang.” | Sudah masuk, belum pulang |
| 17.30 | “Absen pulang Anda belum tercatat.” | Pengingat lanjutan diizinkan |

**Aturan wajib:**

1. Jadwal, frekuensi, jeda, dan maksimal pengulangan dapat dikonfigurasi **per tenant**, dengan default yang bisa diedit.
2. Reminder **berhenti** saat clock-in/out sah, shift dibatalkan, status dikecualikan, atau sudah tidak relevan.
3. Shift malam 22.00–06.00 terikat `work_date` yang benar. Jangan kirim reminder pulang pada 06.00 hari sebelumnya.
4. Shift fleksibel punya kebijakan reminder sendiri. Jangan mengasumsikan 08.00–17.00.
5. Clock-in/out yang terlambat masuk atau dikoreksi harus memperbarui reminder berikutnya dan ringkasan atasan.
6. Jangan melaporkan “tidak hadir” sebelum window dan kebijakan status mengizinkan.
7. Bila push tidak diizinkan atau perangkat offline, notifikasi tetap tersimpan di inbox.

### 7.4 Ringkasan untuk atasan dan HR: siapa belum absen?

- Default penerima: **atasan langsung (`reports_to`)** dan **HR berwenang**. Tenant dapat mengubahnya.
- Isi hanya karyawan dalam **permission dan scope penerima** (`absensi.monitor_bawahan`).
- Berupa **ringkasan jumlah** dan tautan ke daftar terfilter. **Tidak** satu push per bawahan (anti-spam).
- Daftar membedakan: belum waktunya, belum absen, izin/cuti disetujui, libur, tidak dijadwalkan, menunggu koreksi. Jangan disamakan menjadi mangkir.
- Karyawan yang sudah bertindak tidak muncul lagi pada ringkasan berikutnya.

### 7.5 Katalog reminder lain

| Jenis | Sasaran | Contoh |
| --- | --- | --- |
| Pengajuan belum diproses | Approver, HR | Mendekati atau melewati SLA |
| Perubahan jadwal / shift berikutnya | Karyawan | Sebelum dimulai atau saat berubah |
| Kontrak hampir berakhir | HR, atasan | H-30, H-14, H-7 (contoh, configurable) |
| Saldo cuti | Karyawan, HR | Hak cuti akan hangus |
| Dokumen wajib/kedaluwarsa | Karyawan, HR | Belum dikirim atau segera habis |
| Payroll / slip gaji | Finance, ESS | Butuh persetujuan atau slip terbit, **tanpa nominal di push** |
| Kasbon / reimbursement | Pengaju, approver, finance | Dokumen kurang, cicilan jatuh tempo |
| Pengumuman terjadwal | Sasaran broadcast | Waktu terbit atau pengingat membaca |
| Keamanan akun | Pemilik akun, admin | Login perangkat baru, perubahan hak akses |

**\[BERTAHAP\]** Jenis reminder baru didaftarkan lewat katalog event dan template tanpa menulis ulang mesin. Reminder tidak berjalan bila entitlement paket tenant tidak mencakupnya.

### 7.6 Broadcast / pengumuman bertarget

Pembuat berwenang membuat **draft** (judul, isi, lampiran), memilih penerima, melihat pratinjau, lalu **Kirim Sekarang** atau **Jadwalkan**.

**Pilihan target:**

1. Seluruh karyawan/pengguna tenant (butuh `broadcast.target_semua`).
2. Filter **Divisi, Departemen, Jabatan, Wilayah, Kantor**. Tiap kategori multi-select.
3. Pengguna pilihan secara manual (cari nama atau ID).
4. Gabungan filter + pengguna manual, dengan operasi gabungan yang dipilih eksplisit.

**Aturan kombinasi:**

| Tingkat | Operasi |
| --- | --- |
| Banyak nilai dalam satu dimensi | **OR** |
| Antardimensi | **AND** (default, lebih sempit) atau **OR** (lebih luas), dipilih per broadcast |
| Filter + pengguna manual | **Gabungan (union)** sebagai default usulan, lalu dedupe |

- **Penerima final = hasil target ∩ scope pengirim ∩ akun yang berhak menerima.** Dihitung dan ditegakkan backend.
- Satu user hanya menerima **satu salinan** per broadcast.
- **Pratinjau** menampilkan jumlah penerima sebenarnya, daftar (bila berwenang), dan berapa yang **terpotong karena scope pengirim**. Mode OR harus diberi peringatan jelas.
- Penerima adalah **user yang terhubung ke karyawan aktif**. Karyawan tanpa akun tidak menerima.
- Broadcast terjadwal: audience dihitung saat eksekusi, lalu **snapshot penerima dan versi filter** disimpan. Perubahan struktur setelahnya tidak mengubah histori.
- Read receipt per pengguna. Tidak membocorkan daftar penerima di luar scope pembuat.
- Lampiran mengikuti kontrol file tenant.

**Contoh:** HR memilih Divisi Operasional + Kantor Kajen dengan AND. Penerima adalah karyawan yang cocok dengan **kedua** filter, ditambah tiga user manual (bila gabungan = OR), tanpa duplikasi.

### 7.7 Kanal dan perilaku real-time

| Kanal | Status | Perilaku |
| --- | --- | --- |
| **In-app real-time (WebSocket)** | \[WAJIB\] | Lonceng dan inbox diperbarui tanpa refresh. Koneksi hanya menerima notifikasi milik user dan tenant sah |
| **Web Push PWA** | \[WAJIB sebagai kemampuan\] | Push saat app di latar belakang, bila browser mendukung dan user mengizinkan. Tidak dijanjikan selalu terkirim |
| **Email** | \[BERTAHAP\] | Kanal tambahan sesuai preferensi |
| **WhatsApp** | \[BERTAHAP\] | Butuh gateway, biaya, consent, template |

- **MySQL adalah sumber kebenaran** notifikasi dan status baca. WebSocket dan Redis bukan penyimpanan utama.
- WebSocket **tidak menggantikan REST**. REST untuk inbox, detail, arsip, preferensi, pencarian, tandai terbaca, dan aksi bisnis.
- Saat koneksi putus, UI reconnect dan menyinkronkan notifikasi yang terlewat dari REST memakai cursor atau ID event.
- **Dilarang** memasukkan NIK, rekening, nominal gaji, isi dokumen medis, atau data rahasia lain ke payload push atau pratinjau layar terkunci.
- Sesi dicabut, user dinonaktifkan, atau permission/scope berubah harus langsung memengaruhi WebSocket dan tautan detail.
- **Catatan iOS:** Web Push di iPhone hanya bekerja bila PWA **dipasang ke Home Screen**. Perlu UX ajakan pasang aplikasi dan matriks browser/OS yang diputuskan sebelum Fase 2.
- Koneksi WebSocket boleh putus dan menyambung ulang. Itu tidak terkait kebijakan sesi.

### 7.8 Pengaturan dan preferensi

**Admin → Pengaturan → Notifikasi & Reminder:**

- Aktif/nonaktif per jenis event dan modul (kecuali pesan keamanan dan operasional wajib).
- Jadwal sebelum/sesudah shift, pengulangan maksimal, jeda, jam tenang, zona waktu, hari kerja/libur, pengecualian.
- Penerima ringkasan monitoring.
- Template judul/isi, kanal tersedia, status Web Push/email/WhatsApp.
- Uji dan pratinjau template, riwayat pengiriman, kesalahan, kirim ulang yang aman, audit perubahan.

**ESS → Profil → Preferensi Notifikasi:** pilih jenis opsional dan kanal pribadi bila diizinkan tenant. Notifikasi keamanan dan kebijakan ketenagakerjaan yang wajib tidak bisa dimatikan.

### 7.9 Tampilan (UI)

- **Topbar:** lonceng dengan badge belum dibaca dan popover beberapa notifikasi terbaru.
- **Notification Center:** tab Semua / Belum Dibaca / Pengajuan / Pengingat / Broadcast / Sistem, filter tanggal dan status, Tandai Terbaca, Arsipkan, deep link.
- **Status notifikasi:** tersimpan, antre, terkirim ke kanal, dibaca, gagal. **Terkirim ≠ dibaca.**
- **Broadcast composer:** `BaseInput`, `BaseTextarea`, `BaseMultiSelect`, `BaseDatePicker`, `BaseModal`, `BaseBadge`, tabel penerima, dan konfirmasi.
- **Reminder atasan:** kartu ringkasan jumlah dan tabel bawahan belum clock-in/out sesuai scope.
- **ESS mobile:** toast dan badge saat aktif, inbox ringkas, CTA relevan, push berbasis izin.
- Wajib: state offline/loading/error, aksesibilitas, `prefers-reduced-motion`.

### 7.10 Permission notifikasi dan perlindungan

| Permission | Kegunaan |
| --- | --- |
| `notifikasi.lihat_sendiri` | Membaca inbox sendiri |
| `notifikasi.kelola` | Mengelola aturan reminder dan kanal |
| `broadcast.buat`, `broadcast.ubah` | Mengelola draft |
| `broadcast.kirim` | Mengirim atau menjadwalkan |
| `broadcast.target_semua` | Menargetkan seluruh tenant (tidak otomatis dimiliki semua HR) |
| `broadcast.lihat_penerima` | Melihat rincian penerima |
| `absensi.monitor_bawahan` | Ringkasan belum presensi dalam scope |

- Tidak ada broadcast lintas tenant dari Admin Panel. Pesan platform ke admin tenant adalah tipe **platform announcement** tersendiri dengan izin platform.
- Audit mencatat pembuat, penyunting, pengirim, waktu, target dan filter, jumlah target, hasil, perubahan status, template, alasan batal.
- Wajib: rate limit, idempotency, pembatalan broadcast terjadwal, dan deduplikasi.

### 7.11 Arsitektur pengiriman

```text
[Event bisnis: Approval / Absensi / Broadcast / lainnya]
                     │
                     ▼
          [Go Notification Engine]
        aturan + tenant + scope user
                     │
     ┌─────────────┼─────────────┐
     ▼               ▼               ▼
 MySQL            Go Worker       Redis Pub/Sub
 notifications    Scheduler       (distribusi antar
 + outbox         + Retry          instance)
     └─────────────┼─────────────┘
                     ▼
             WebSocket Gateway
                     │
      ┌─────────────┼─────────────┐
      ▼              ▼              ▼
 Admin Panel      ESS Panel      Platform
      └───────── REST API ───────┘
        inbox / baca / preferensi
```

**Kontrak perilaku:**

1. Perubahan bisnis dan pencatatan outbox **atomik** bila dalam transaksi yang sama. Worker memproses dengan retry aman.
2. Event punya ID stabil dan *deduplication key*.
3. WebSocket hanya mendorong notifikasi ke user terautentikasi dan terotorisasi, dengan pengecekan tenant di jalur event. Cek `Origin` saat handshake.
4. Redis untuk distribusi antar instance. **Pub/sub bukan antrean durable.**
5. Scheduler menilai ulang kondisi reminder **tepat sebelum kirim**.
6. **\[USULAN\]** Jadwal disimpan di tabel MySQL (`next_fire_at`) dan diambil worker dengan `SELECT … FOR UPDATE SKIP LOCKED`.
7. Broadcast besar (mis. 10.000 penerima) diproses **bertahap**, dengan pencatatan kegagalan parsial dan riwayat yang dapat diaudit.

**Model data konseptual:** `notification_events`, `notifications`, `notification_deliveries`, `notification_read_states`, `notification_templates`, `notification_preferences`, `notification_rules`, `notification_schedules`, `notification_outbox`, `broadcasts`, `broadcast_targets`, `broadcast_recipients`, `broadcast_deliveries`, `push_subscriptions`. Bentuk fisik final ditentukan di ERD.

### 7.12 Default konfigurasi

| Topik | Default | Bisa diubah? |
| --- | --- | --- |
| Jadwal reminder absensi | Mengikuti shift instance, bukan jam global | Ya, per tenant |
| Penerima “belum absen” | Atasan langsung + HR sesuai scope | Ya, per tenant |
| Gabungan antarfilter broadcast | AND (opsi OR tersedia) | Ya, per broadcast |
| Kanal awal | In-app real-time + kemampuan Web Push | Sesuai izin user/perangkat |
| Kanal lanjutan | Email dan WhatsApp | Bila integrasi aktif |
| Frekuensi, jeda, jam tenang, prioritas | Belum ada angka baku | **\[TERBUKA\]** |
| Penerima broadcast terjadwal | Dihitung saat eksekusi, disimpan snapshot | **\[USULAN\]** |

---

## 8. UI/UX berbasis template digaji.id \[WAJIB\]

### 8.1 Prinsip

Template Vue 3 adalah **sumber desain**. Jangan membangun ulang UI, mengganti framework, atau memasang UI kit besar. Yang dipertahankan:

- **2-Layer Layout:** backdrop luar (Dasar Pertama) dan floating surface (Dasar Kedua).
- **Desktop dual-sidebar:** primary rail, submenu yang bisa dilipat, hover flyout, sliding active pill.
- **Mobile:** drawer sesuai template untuk Admin Panel.
- **Adaptive modal:** `BaseModal` sebagai pintu utama (dialog di desktop, bottom sheet di mobile).
- **Theme tokens:** warna aksen dan permukaan lewat CSS variable. Tidak ada hex tetap per halaman.
- **SPA:** navigasi antarmodul tanpa reload.
- **Login morphing:** boleh dipertahankan sebagai tampilan. Autentikasi dan route protection tetap independen dari animasi.

### 8.2 Tema

| Tema | Primary |
| --- | --- |
| Default | `#18181B` (light) / `#FFFFFF` (dark) |
| Blue | `#007AFF` |
| Pink | `#E85D88` |
| Terracotta | `#C03A18` |

Mendukung mode terang/gelap, preferensi tenant, dan preferensi individu bila diizinkan.

### 8.3 Komponen

**Komponen Base** sudah selesai dibuat dan menjadi dasar semua layar. Komponen yang dibutuhkan fitur ini wajib sudah tersedia sebelum modul terkait dikerjakan: `BaseCheckbox`, `BaseFormField`, `BaseMultiSelect` (termasuk mode async), `BaseDataTable` (server-side), `BaseTabs`, `BasePopover`, `BaseAlert`, `BaseTimePicker`.

**Komponen domain** (bukan `Base`, disimpan di `src/components/domain/`):

| Komponen | Fungsi |
| --- | --- |
| `PermissionMatrix` | Matriks Ikuti Role / Izinkan / Tolak per modul dan aksi |
| `ScopePicker` | Pilih dimensi dan nilai data scope |
| `EffectiveAccessPreview` | Pratinjau akses efektif beserta sumbernya |
| `PanelSwitcher` | Pindah ESS ⇄ Admin |
| `NotificationBell`, `NotificationItem` | Lonceng dan item inbox |
| `AudienceBuilder` | Penyusun target broadcast dan pratinjau penerima |
| `ApprovalChain` | Visual alur approval L1 → L2 |

**Layout:** `AppLayout` (Admin Panel dan Superadmin), `EssLayout` dengan bottom navigation (**baru**).

Komponen demonstrasi template yang tidak relevan (Gunung, Pesanan, Invoice demo) adalah **referensi pola**, bukan modul produk.

### 8.4 Standar UX

- Setiap aksi utama punya state: normal, loading, berhasil, gagal, tidak berhak.
- Form punya validasi dan pesan error yang jelas.
- Tabel besar memakai filter, sort, dan paginasi **di server**. Ekspor mematuhi scope.
- Dialog punya focus management, dukungan Escape, dan pencegahan submit ganda.
- Animasi mendukung `prefers-reduced-motion` dan tidak menghalangi tugas utama.
- Hindari layout shift. Teks penting (NIP, nama, nominal) harus bisa dipilih dan disalin (jangan `select-none` pada konten data).
- Bahasa Indonesia konsisten untuk label, status, tanggal, dan Rupiah.
- Data sensitif tidak muncul di notifikasi tidak aman atau dashboard pengguna tak berwenang.

---

## 9. Stack dan arsitektur

### 9.1 Stack

| Lapisan | Pilihan | Status |
| --- | --- | --- |
| Frontend | Vue 3, Vite, Tailwind CSS v4, Vue Router, **Pinia**, lucide-vue-next | \[WAJIB\] |
| PWA | **vite-plugin-pwa** (service worker, Web Push) | \[USULAN\] |
| Klien API | **openapi-typescript** dari kontrak OpenAPI | \[USULAN\] |
| Backend | Go, REST API. **Satu router: Chi atau Gin** | Go \[WAJIB\]. Router **\[TERBUKA\]**, harus diputuskan sebelum Fase 0 |
| Akses data | GORM v2 untuk CRUD. **sqlc / raw SQL** untuk laporan dan query ber-scope | \[USULAN\] |
| Evaluator akses | **Dibuat sendiri** (bukan Casbin) karena scope berbasis data. Hasil di-cache dengan versi hak akses per user | \[USULAN\] |
| Database | MySQL 8.x. Migrasi dengan golang-migrate atau goose. Uang: integer rupiah atau `shopspring/decimal` | \[WAJIB\] MySQL |
| Cache, antrean, jobs | Redis, Go worker, scheduler berbasis tabel MySQL | \[WAJIB\] |
| Real-time | WebSocket gateway di Go (coder/websocket atau gorilla). REST tetap jalur utama | \[WAJIB\] |
| Penyimpanan file | S3-compatible (MinIO / R2 / AWS S3) | \[WAJIB\] |
| Peta geofence | Library peta untuk editor polygon (mis. Leaflet) sebagai **pengecualian resmi** aturan “tanpa library baru” | \[USULAN\] |
| Deploy | Docker / Linux. Dev, staging, production terpisah | \[WAJIB\] |
| Observability | Structured logs, metrics, tracing, error reporting, audit trail | \[WAJIB\] |
| Test | Go test + testcontainers (uji lintas tenant), Vitest, Playwright | \[USULAN\] |
| Kontrak API | OpenAPI: versi, error standar, pagination | \[WAJIB\] |

**Aturan library frontend:** dilarang menambah library **UI dan chart** (Element Plus, Vuetify, Chart.js, dan sejenisnya). Library **infrastruktur** (PWA, state, peta) boleh dengan persetujuan Product Owner dan dicatat di PRD.

**Arsitektur \[USULAN\]:** mulai dari **modular monolith Go** dengan modul internal yang jelas. Microservices bukan syarat MVP.

### 9.2 Batas tanggung jawab

```text
Vue 3 (Admin Panel, ESS Panel, Platform; design system bersama)
                   │
                REST API
                   │
     Go API / Auth / Tenant Context
                   │
      Permission + Scope + Entitlement
                   │
  ┌─────────────┬────┴──────────┬──────────┐
  Core HR     Attendance    Approval
  Payroll     Subscription  Notification
  └─────────────┴──────────────┴──────────┘
                   │
           MySQL / Redis / S3
                   │
               Go Workers
```

- **Vue:** pengalaman pengguna, presentasi, form, route.
- **Go:** otorisasi final, validasi bisnis, perubahan data, payroll, audit.
- **MySQL:** sumber kebenaran. Cache tidak boleh menyebabkan akses lintas tenant atau hak akses basi.
- **Worker:** ekspor dan impor besar, rekap, notifikasi, dokumen, payroll batch.

### 9.3 Kelompok entitas data (konseptual)

| Domain | Entitas |
| --- | --- |
| SaaS | tenants, plans, plan\_entitlements, subscriptions, invoices, payments, tenant\_usage |
| Identitas & akses | users, tenant\_memberships, **user\_roles** (bertanggal berlaku), roles, **permission\_registry**, role\_permissions, **user\_permission\_overrides**, **role\_scope\_presets**, **user\_scope\_assignments** (per dimensi), **sessions** (dengan `remember`) |
| Organisasi | regions, offices, office\_geofences, divisions, departments, positions |
| Kepegawaian | employees, employee\_assignments, employee\_allowed\_offices, contracts, employee\_changes, employee\_documents |
| Kehadiran | shift\_templates, work\_schedules, shift\_instances, attendance\_events, attendance\_corrections |
| Pengajuan | requests, approval\_flows, approval\_steps, approval\_actions, leave\_balances, overtime\_requests |
| Keuangan HR | payroll\_periods, payroll\_runs, payroll\_items, payroll\_adjustments, employee\_loans, reimbursements |
| Komunikasi | Entitas notifikasi dan broadcast (Bagian 7.11) |
| Keamanan | audit\_logs, login\_events, support\_sessions, access\_reviews |

Semua ini rancangan konseptual. ERD, constraint, indeks, dan migrasi disusun sebelum implementasi fisik. Tidak semua tabel dibuat sejak Fase 0.

### 9.4 Kualitas API

Konsisten pada pagination, filter, sort, format error, idempotency aksi kritis, versioning, dan otorisasi. Perubahan atomik (saldo cuti, approval, finalisasi payroll) memakai transaksi database.

---

## 10. Kebijakan sesi login: toggle “Simpan Sesi” \[WAJIB\]

**Keputusan Product Owner:** tidak ada lagi sesi yang “pasti tidak pernah expired untuk semua orang”. Pengguna memilih lewat toggle **Simpan Sesi** di halaman login. Ini lebih aman karena perangkat bersama tidak otomatis menyimpan sesi.

### 10.1 Dua mode

|  | **Simpan Sesi ON** | **Simpan Sesi OFF** |
| --- | --- | --- |
| Cookie | Persisten | Cookie sesi browser |
| Expiry otomatis | **Tidak ada** | Berakhir saat browser ditutup **dan** idle timeout di server |
| Cocok untuk | Perangkat pribadi, PWA terpasang | Komputer bersama, kantor |
| Berakhir karena | Logout, dicabut admin, akun nonaktif, ganti kata sandi, resign | Semua di kiri, ditambah idle timeout |

**Mengapa OFF tetap punya idle timeout:** beberapa browser memulihkan cookie sesi saat dibuka kembali. Nilai idle timeout **\[TERBUKA\]** dan dibuat configurable (per tenant).

### 10.2 Aturan

1. Label toggle: “Simpan sesi di perangkat ini. Jangan aktifkan di komputer bersama.”
2. **Default:** OFF untuk Admin Panel. ON bila ESS berjalan sebagai PWA terpasang (`display-mode: standalone`). Pengguna tetap bisa mengubahnya.
3. Tenant boleh **menonaktifkan opsi Simpan Sesi** untuk Admin Panel.
4. Menutup tab atau browser pada mode ON tidak berarti logout. Refresh dan deep link memulihkan sesi sah.
5. Tombol **Keluar** mencabut sesi di perangkat itu. Tersedia juga **Keluar dari semua perangkat** dan daftar perangkat aktif.
6. Sesi dicabut bila akun dinonaktifkan, karyawan resign, admin mencabut sesi, atau **kata sandi diganti** (sesi lain dicabut).
7. Permission dan scope terbaru berlaku pada sesi lama tanpa logout.
8. Pencabutan efektif di **API, file, dan WebSocket**, termasuk menolak reconnect.
9. **Aksi berisiko** (finalisasi payroll, ubah hak akses, ekspor data sensitif) tetap meminta **re-autentikasi**, walau Simpan Sesi ON.
10. Koneksi WebSocket boleh putus dan menyambung ulang. Itu tidak mengubah kebijakan sesi.

### 10.3 Pendekatan teknis \[USULAN\]

- Identitas sesi **opaque** yang dikelola server. Hanya **hash** yang disimpan di tabel `sessions`.
- Cookie `HttpOnly`, `Secure`, `SameSite` sesuai kebutuhan, plus perlindungan CSRF. Token sensitif tidak disimpan di `localStorage`.
- Tabel `sessions`: `remember` (bool), `expires_at` (NULL bila remember), `last_seen_at`, perangkat, IP, waktu dibuat. Rotasi identitas sesi secara berkala.
- Handshake WebSocket memeriksa sesi dan `Origin`.
- Login ESS dan Admin memakai sesi yang sama (satu akun, satu sesi). Perpindahan panel tidak membuat sesi baru.

**Batas teknis:** browser atau OS dapat menghapus cookie atau data situs, sehingga login ulang tetap mungkin. Sasaran produk adalah **tidak ada expiry otomatis dari aplikasi pada mode ON**, bukan jaminan sesi abadi dalam segala keadaan.

---

## 11. Keamanan, privasi, dan audit \[WAJIB\]

- NIK, NPWP, rekening, penghasilan, dokumen, geolokasi, dan selfie diperlakukan sebagai **data sensitif**.
- Enkripsi transport, akses file aman (cek izin + URL sementara), penyimpanan sesuai klasifikasi data.
- **Audit** operasi kritis: login, akses berisiko, perubahan role/permission/scope, penempatan, geofence, approval, payroll, **ekspor dan impor**, support access.
- Audit menyimpan metadata dan perubahan yang aman. **Jangan** menyimpan password, token sesi, atau rahasia mentah dalam JSON before/after.
- **Field sensitif** (`lihat_sensitif`) disamarkan di layar, ekspor, dan notifikasi bila tidak berizin.
- Backup terjadwal dengan **proses restore yang diuji**.
- Admin platform tidak otomatis boleh membaca data gaji dan dokumen tenant tanpa konteks dukungan yang sah.
- Target broadcast, ringkasan belum absen, riwayat pengiriman, dan koneksi WebSocket mengikuti tenant + permission + scope.
- **Perlindungan data pribadi:** produk harus selaras dengan **UU No. 27 Tahun 2022 tentang Pelindungan Data Pribadi (UU PDP)**. Posisi umum: tenant sebagai pengendali data, digaji.id sebagai prosesor. Perlu perjanjian pemrosesan data, kebijakan retensi dan penghapusan, mekanisme permintaan koreksi, dan prosedur respons insiden. **\[TERBUKA\]** detail kebijakan, perlu ditinjau penasihat hukum.

---

## 12. Laporan, dashboard, dan metrik

| Portal | KPI utama |
| --- | --- |
| **Superadmin** | Total tenant, trial, aktif, suspended, churn, pertumbuhan, MRR/ARR, invoice jatuh tempo, utilisasi sistem |
| **Admin Panel** | Karyawan aktif, hadir/terlambat/izin/tidak hadir, pengajuan menunggu, saldo cuti, kontrak mendekati akhir, biaya payroll (sesuai izin), statistik shift, tren bulanan |
| **ESS** | Jadwal hari ini, status absensi, saldo cuti, pengajuan terakhir, lembur, slip gaji. Hindari layar penuh grafik karena aksi utama adalah clock in/out |
| **Notifikasi & broadcast** | Total diproses/terkirim/gagal, tren belum dibaca, reminder dieksekusi atau dibatalkan, jumlah penerima broadcast, ketepatan jadwal. Superadmin hanya melihat agregat, bukan isi pesan tenant |

**Aturan metrik:** setiap KPI punya sumber data, filter tenant, periode, scope, definisi perhitungan, kebijakan refresh, dan empty state. Hasil ekspor tidak boleh melampaui data yang bisa dilihat pengguna. Definisi revenue (pajak, refund, diskon) harus konsisten.

---

## 13. Kebutuhan non-fungsional

| Area | Persyaratan |
| --- | --- |
| Responsivitas | Superadmin dan Admin desktop-first. ESS mobile-first. Tablet tetap layak |
| Performa | Route lazy loading, bundle splitting, caching aman, query dan indeks terukur, skeleton |
| Keandalan | Retry aman, idempotency, transaksi atomik, outbox dan scheduler tahan retry dan dedupe |
| Isolasi | Uji lintas tenant di setiap layanan, query, file, cache, ekspor, dan worker |
| Keamanan | Otorisasi di backend, perlindungan sesi, audit, masking, penanganan error yang aman |
| Aksesibilitas | Keyboard, fokus modal, kontras, label, pengurangan animasi |
| Operasional | Dev/staging/prod terpisah, logging, metrics, alerting, uji backup dan restore |
| Skalabilitas | Pagination server, worker batch, tidak mengunduh seluruh data tenant, fan-out broadcast bertahap |
| Kompatibilitas | Daftar browser dan perangkat target ditetapkan sebelum QA rilis. Web Push opt-in dan mengikuti dukungan OS |
| Real-time | Event live tanpa refresh. Reconnect dan replay dari REST. Heartbeat tidak mengubah kebijakan sesi |
| Reminder | Menghormati shift instance, timezone, cuti/izin, recheck sebelum kirim, batas frekuensi |
| Broadcast | Dedupe penerima, preview dan audit target, eksekusi terjadwal, aman untuk volume tinggi |
| Compliance | Pajak, BPJS, dan pelindungan data dievaluasi terhadap aturan yang berlaku saat implementasi |

**\[TERBUKA\]** target numerik: latensi API p95, batas delay event real-time dan toleransi keterlambatan reminder, waktu muat dashboard, jumlah tenant dan karyawan awal, SLO, RPO/RTO, kuota storage, dan matriks browser. Angka ditetapkan dari target bisnis dan hasil pengujian.

---

## 14. Kriteria penerimaan (Acceptance Criteria)

> **Catatan:** AC-01 sampai AC-46 disusun ulang dan dikelompokkan per topik pada v2.3. Nomornya bisa berbeda dari v2.2, jadi cocokkan dengan test plan sebelum digabungkan. **AC-47 sampai AC-58 adalah tambahan baru** untuk sesi, dua panel, multi-role, scope, dan impor/ekspor.

### 14.1 Tenant dan akses

| ID | Skenario | Hasil yang diharapkan |
| --- | --- | --- |
| AC-01 | Pengguna Tenant A meminta data Tenant B lewat API, file, ekspor, atau cache | Ditolak. Tidak ada data yang bocor |
| AC-02 | Request membawa `tenant_id` di body atau URL | Diabaikan. Tenant ditentukan dari sesi dan keanggotaan |
| AC-03 | Menu disembunyikan, tetapi API dipanggil langsung tanpa izin | API menolak (deny by default) |
| AC-04 | User tanpa `panel.admin` membuka `/admin/*` | Ditolak. ESS tetap berfungsi |
| AC-05 | Tenant memanggil fitur di luar paket | API menolak, UI menampilkan info upgrade |
| AC-06 | Kode memakai permission yang tidak ada di registry | Build atau test gagal |
| AC-07 | Scope Kantor dipasang pada `karyawan.lihat` | Daftar, detail, dan ekspor hanya berisi karyawan kantor itu |
| AC-08 | Scope Wilayah dipilih | Semua kantor di wilayah itu tercakup |
| AC-09 | Scope Kantor **dan** Divisi dalam satu aturan | Hanya karyawan yang cocok dengan keduanya |
| AC-10 | Role, struktur organisasi, atau atasan diubah | Berlaku pada request berikutnya tanpa logout |
| AC-11 | Karyawan membuka ESS | Hanya melihat data miliknya |
| AC-12 | User tanpa `lihat_sensitif` membuka karyawan atau ekspor | Gaji, NIK, NPWP, rekening disamarkan |

### 14.2 Organisasi dan karyawan

| ID | Skenario | Hasil yang diharapkan |
| --- | --- | --- |
| AC-13 | Membuat struktur organisasi | Hanya Divisi → Departemen → Jabatan. Tidak ada “Tim” |
| AC-14 | Karyawan memiliki dua base office aktif | Ditolak. Tepat satu base office |
| AC-15 | Jabatan, gaji, kantor, atau atasan diubah | Riwayat dan tanggal efektif tersimpan |
| AC-16 | Impor CSV karyawan | Ada preview, validasi, laporan error, dan pencegahan duplikasi |

### 14.3 Jadwal dan absensi

| ID | Skenario | Hasil yang diharapkan |
| --- | --- | --- |
| AC-17 | Presensi di dalam salah satu polygon kantor yang diizinkan | Lokasi valid |
| AC-18 | Perangkat mengirim waktu yang berbeda dari server | Waktu server dipakai |
| AC-19 | Shift lintas hari (22.00–06.00) | Presensi pulang terikat ke `work_date` shift masuk |
| AC-20 | Datang di dalam jendela clock-in tapi melewati toleransi | Tercatat terlambat. Jendela dan toleransi dibedakan |
| AC-21 | Double tap atau retry koneksi | Tidak ada event presensi ganda |
| AC-22 | Karyawan salah presensi | Perbaikan lewat koreksi + approval. Event asli tidak diubah |
| AC-23 | Shift master diubah | Shift instance dan payroll yang berjalan tidak berubah |

### 14.4 Approval, cuti, lembur, keuangan

| ID | Skenario | Hasil yang diharapkan |
| --- | --- | --- |
| AC-24 | Pengajuan masuk ke approver | Hanya yang punya permission **dan** scope yang menerima tugas |
| AC-25 | Dua approver memutuskan bersamaan | Satu hasil akhir yang konsisten (atomik) |
| AC-26 | Admin mendelegasikan akses | Hanya yang ia kuasai. Melebihi kewenangan ditolak dan dicatat |
| AC-27 | Keputusan approval dibuat | Pelaku, level, waktu, alasan, lampiran tersimpan |
| AC-28 | Cuti disetujui lalu diproses ulang | Saldo tidak berkurang dua kali |
| AC-29 | Lembur diusulkan, disetujui, dibayar | Tiga jam tersebut tercatat terpisah |
| AC-30 | Perhitungan nominal | Tidak memakai floating point |
| AC-31 | Pencairan kasbon atau reimbursement di-retry | Tidak terbayar dua kali |

### 14.5 Payroll

| ID | Skenario | Hasil yang diharapkan |
| --- | --- | --- |
| AC-32 | Payroll difinalisasi, lalu profil karyawan diedit | Hasil final tidak berubah. Koreksi lewat adjustment |
| AC-33 | Tarif pajak atau BPJS berubah | Periode lama tetap memakai aturan lama (berversi tanggal berlaku) |
| AC-34 | Mengakses slip gaji | Hanya pemilik dan pihak berwenang. Akses dicek ulang saat unduh |

### 14.6 Notifikasi dan broadcast

| ID | Skenario | Hasil yang diharapkan |
| --- | --- | --- |
| AC-35 | Pengajuan baru dibuat | Approver tahap pertama yang berhak menerima notifikasi real-time tanpa refresh |
| AC-36 | Karyawan sudah clock-in sah | Reminder masuk berhenti |
| AC-37 | Shift malam 22.00–06.00 | Reminder pulang mengikuti `work_date` yang benar |
| AC-38 | Broadcast terjadwal dieksekusi | Audience dihitung saat eksekusi. Snapshot penerima dan filter disimpan |
| AC-39 | Broadcast atau event yang sama diproses ulang | Satu user hanya menerima satu salinan (dedupe) |
| AC-40 | Ringkasan “belum absen” dikirim ke atasan | Berupa ringkasan jumlah, hanya karyawan dalam scope penerima |
| AC-41 | Push dikirim | Payload tidak memuat NIK, rekening, nominal, atau data medis |
| AC-42 | WebSocket terputus lalu tersambung lagi | Notifikasi yang terlewat disinkronkan dari REST |
| AC-43 | Sesi dicabut atau permission/scope berubah | WebSocket tertutup atau terfilter. Tautan detail dicek ulang |

### 14.7 Umum

| ID | Skenario | Hasil yang diharapkan |
| --- | --- | --- |
| AC-44 | Tenant ditangguhkan (suspended) | Perilaku mengikuti kebijakan yang diputuskan, dan tercatat di audit |
| AC-45 | Aksi kritis dilakukan | Audit tercatat. Tanpa password, token, atau rahasia mentah |
| AC-46 | Dialog dibuka | Fokus terkelola, mendukung Escape, tanpa submit ganda, menghormati `prefers-reduced-motion` |

### 14.8 Tambahan v2.3: sesi, dua panel, multi-role, scope, impor/ekspor

| ID | Skenario | Hasil yang diharapkan |
| --- | --- | --- |
| AC-47 | Simpan Sesi ON, browser ditutup lalu dibuka | Sesi tetap valid |
| AC-48 | Simpan Sesi OFF | Sesi berakhir saat browser ditutup atau idle timeout |
| AC-49 | Kata sandi diganti atau admin mencabut sesi | Semua sesi terkait dan WebSocket tertutup |
| AC-50 | User ESS diberi role SPV | Panel switcher dan menu admin muncul sesuai permission, **tanpa logout** |
| AC-51 | Role SPV dicabut | Admin Panel tertutup. ESS tetap berfungsi |
| AC-52 | Dua role dengan scope berbeda untuk permission yang sama | Hasilnya gabungan (OR) |
| AC-53 | User Deny dipasang pada permission yang diberi role lain | Deny menang |
| AC-54 | Impor berisi baris di luar scope pengimpor | Baris itu ditolak, baris valid diproses, laporan error tersedia |
| AC-55 | Ekspor oleh user dengan scope terbatas | Mengikuti scope, kolom sensitif disamarkan tanpa `lihat_sensitif`, tercatat di audit |
| AC-56 | User menyetujui pengajuannya sendiri | Ditolak, walau scope mencakup dirinya |
| AC-57 | Departemen baru dibuat di bawah Divisi yang sudah masuk scope | Otomatis tercakup |
| AC-58 | Karyawan resign | Semua role admin dan sesi berakhir pada tanggal efektif |

---

## 15. Roadmap bertahap

| Fase | Fokus | Isi utama |
| --- | --- | --- |
| **0. Fondasi** | Tenant, identitas, dan akses | Tenant dan membership, login + **toggle Simpan Sesi**, **multi-role**, **permission registry**, **evaluator scope**, panel switcher, audit, `AppLayout` + `EssLayout`, komponen domain dasar, CI uji lintas tenant. Superadmin minimal (tenant, paket) |
| **1. SDM inti** | Organisasi dan karyawan | Struktur organisasi, direktori karyawan, kontrak, impor CSV, dokumen. **Notifikasi in-app** (REST inbox) + broadcast dasar. WebSocket dan outbox masuk di fase ini bila event nyata sudah ada |
| **2. Kehadiran** | Jadwal dan absensi | Shift, roster, geofence multi-polygon, presensi ESS (PWA), koreksi, reminder, monitor belum presensi, Web Push |
| **3. Pengajuan** | Cuti, lembur, approval | Cuti/izin, lembur (SPKL), Pusat Persetujuan, kasbon, reimbursement, delegasi dan eskalasi |
| **4. Payroll** | Penggajian | Komponen gaji, PPh 21, BPJS, payroll run, slip gaji, ekspor bank |
| **5. Lanjutan** | Talenta dan integrasi | Kinerja, rekrutmen, pelatihan, integrasi/API, email dan WhatsApp, analitik lanjutan |

**Prinsip:** tiap fase selesai bila AC fase itu lulus, komponen UI yang dibutuhkan tersedia, dan bagian \[TERBUKA\] terkait sudah diputuskan. Modul lintas fase (approval, notifikasi, audit) dirancang dari awal walau diisi bertahap.

---

## 16. Keputusan: sudah ditutup dan yang masih terbuka

### 16.1 Ditutup di v2.3

| Topik | Keputusan |
| --- | --- |
| Kebijakan sesi | Toggle Simpan Sesi (Bagian 10) |
| Scope lintas dimensi | AND antardimensi, OR antarnilai dan antaraturan (Bagian 3.3) |
| Multi-role | Ya, dengan tanggal berlaku (Bagian 3.1) |
| Dua panel | Admin Panel + ESS Panel, satu akun, satu sesi (Bagian 1.2) |
| Permission per user | Allow, Deny, dan ganti scope per user (Bagian 3) |
| Daftar dimensi scope | Wilayah, Kantor, Divisi, Departemen, Jabatan, plus Bawahan Langsung, Diri Sendiri, Seluruh Tenant |
| Struktur menu | Final untuk Superadmin, Admin, dan ESS (Bagian 4) |

### 16.2 Masih terbuka

| ID | Topik | Dibutuhkan sebelum |
| --- | --- | --- |
| T-01 | **Router Go:** Chi atau Gin | Fase 0 |
| T-02 | Alur login satu email di banyak tenant | Fase 0 |
| T-03 | Nilai idle timeout untuk Simpan Sesi OFF | Fase 0 |
| T-04 | Pemetaan Jabatan → role yang disarankan: dipakai atau tidak | Fase 1 |
| T-05 | `legal_entity`: wajib di MVP atau tidak | Fase 1 |
| T-06 | Perilaku tenant suspended (ESS read-only atau blok penuh), masa trial, grace period, retensi data | Fase 1 |
| T-07 | Matriks browser/OS, dukungan Web Push di iOS, dan ajakan pasang PWA | Fase 2 |
| T-08 | Ambang akurasi GPS, toleransi tepi polygon, alur bila lokasi gagal, kebutuhan app native | Fase 2 |
| T-09 | `work_date` dan reminder untuk shift fleksibel | Fase 2 |
| T-10 | Frekuensi, jeda, jam tenang, dan batas reminder | Fase 2 |
| T-11 | Delegasi, eskalasi, dan SLA approval otomatis di MVP | Fase 3 |
| T-12 | Carry-over, prorata, cuti bersama, saldo negatif | Fase 3 |
| T-13 | Multi-currency, formula payroll kustom, format bank, pembayaran otomatis | Fase 4 |
| T-14 | Target numerik NFR (latensi, SLO, RPO/RTO, kuota) | Sebelum rilis |
| T-15 | Kebijakan UU PDP: DPA, retensi, respons insiden | Sebelum produksi |

---

## 17. Dokumen turunan dan aturan untuk AI agent

### 17.1 Dokumen yang perlu dibuat

| Dokumen | Isi |
| --- | --- |
| ERD dan skema | Entitas fisik, constraint, indeks (selalu dengan `tenant_id`), migrasi |
| Permission registry (seed) | Daftar final kode permission, aksi, dimensi scope, penanda sensitif, entitlement |
| Katalog event notifikasi | Event, penerima, template, kanal, aturan reminder |
| Spesifikasi OpenAPI | Kontrak endpoint, format error, pagination, idempotency |
| Konvensi backend | Struktur proyek Go, lapisan data access, penanganan error, tool migrasi |
| Konvensi UI | `aturan-ui.md` dan `desain-ui.md` yang disinkronkan dengan kode, plus aturan komponen domain, `EssLayout`, dan pengecualian library infrastruktur |
| Test plan | Pemetaan AC ke test otomatis, uji lintas tenant, uji scope |
| `AGENTS.md` / `CLAUDE.md` | Menunjuk ke PRD, konvensi UI, dan konvensi backend |

### 17.2 Aturan untuk AI agent

1. **Jangan mengimplementasikan bagian bertanda \[TERBUKA\]** tanpa bertanya.
2. Bagian \[USULAN\] boleh dikerjakan hanya setelah ditinjau dan dikonfirmasi.
3. Semua otorisasi dilakukan di backend. UI hanya mencerminkannya.
4. Setiap query data bisnis wajib melewati lapisan yang menyuntikkan `tenant_id`.
5. Permission baru harus didaftarkan di registry lebih dulu.
6. Ikuti `aturan-ui.md`: baca `defineProps` sebelum memakai komponen, pakai komponen `Base*`, dan gunakan token tema.
7. Bila PRD, dokumen UI, dan kode berbeda: **kode adalah kebenaran sementara**, lalu laporkan perbedaannya.
8. Jangan menyalin data demo template ke fitur produk.

---

## 18. Kamus istilah

| Istilah | Arti |
| --- | --- |
| **Tenant** | Satu perusahaan pelanggan di platform |
| **Admin Panel** | Antarmuka pengelolaan SDM untuk user yang punya `panel.admin` |
| **ESS (Employee Self Service)** | Panel karyawan untuk absensi, pengajuan, slip gaji, dan profil |
| **Role** | Kelompok tanggung jawab sebagai preset permission dan scope |
| **Permission** | Izin aksi dengan format `modul.aksi` |
| **Data scope** | Batas data yang boleh diakses untuk suatu permission |
| **User override** | Pengaturan Allow, Deny, atau scope khusus satu user |
| **Entitlement** | Modul dan batas yang diizinkan oleh paket langganan |
| **Base office** | Kantor utama karyawan. Tepat satu per karyawan |
| **Geofence** | Area polygon tempat presensi boleh dilakukan |
| **Shift instance** | Kejadian kerja aktual dari sebuah shift pada tanggal tertentu |
| **Work date** | Tanggal kerja yang menjadi acuan shift, termasuk shift lintas hari |
| **SPKL** | Surat Perintah Kerja Lembur |
| **TER** | Tarif Efektif Rata-rata untuk PPh 21 |
| **Outbox** | Pola pencatatan event dalam transaksi yang sama agar pengiriman tidak hilang |
| **Idempotency** | Aksi yang diulang tidak menghasilkan efek ganda |
| **UU PDP** | UU No. 27 Tahun 2022 tentang Pelindungan Data Pribadi |
