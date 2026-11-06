# Spesifikasi API digaji.id — REST + WebSocket

**Turunan PRD v2.3** · 4 November 2026 · Draft \[USULAN\]

> **Tujuan:** daftar lengkap endpoint API untuk Superadmin, Admin Panel, dan ESS Panel, beserta konvensi, permission tiap endpoint, aturan async/idempotensi, protokol WebSocket, dan katalog error. Mengacu ke dokumen **Matriks Menu, Role & Fungsional** (permission) dan **Desain Database** (tabel). Detail field request/response akhir dibuat sebagai file **OpenAPI** dari dokumen ini.

## 1. Konvensi umum

| Hal | Aturan |
| --- | --- |
| Base URL | `https://api.digaji.id/api/v1` (contoh). Satu versi mayor di path. Perubahan yang merusak kompatibilitas menjadi `/v2` |
| Format | JSON UTF-8. Nama field `snake_case`. Berkas diunggah langsung ke storage lewat URL sementara (Bagian 12) |
| Autentikasi | Cookie sesi `dgj_session` (`HttpOnly`, `Secure`, `SameSite=Lax`). Tidak ada token di `localStorage`. WebSocket memakai cookie yang sama |
| CSRF | Semua `POST`, `PUT`, `PATCH`, `DELETE` wajib header `X-CSRF-Token` |
| Tenant | Ditentukan backend dari sesi (membership aktif). **Tidak ada** `tenant_id` di path, body, atau query |
| ID | `public_id` berupa ULID 26 karakter, mis. `01JC8Z4K9Q3X7M2V5N6B8D0F1G`. ID internal tidak pernah keluar |
| Waktu | Instan: ISO 8601 UTC `2026-11-05T01:23:45.123Z`. Tanggal: `YYYY-MM-DD`. Jam: `HH:mm`. Zona waktu kantor dikirim terpisah (`timezone`) |
| Uang | String desimal, mis. `15000000.00`. Tidak pernah angka floating |
| Header respons | `X-Request-Id`, `X-Access-Version`, `X-RateLimit-Limit`, `X-RateLimit-Remaining` |
| `X-Access-Version` | Naik bila hak akses user berubah. Frontend yang melihat angka berbeda **memuat ulang `GET /me/access`** (AC-10, AC-50) |
| Idempotensi | Header `Idempotency-Key` (UUID) wajib pada aksi bertanda **I**. Ulangan dengan kunci sama mengembalikan hasil pertama (header `Idempotent-Replay: true`) |
| Konkurensi | Resource yang bisa diedit bersamaan memuat `version`. Aksi dan `PATCH` mengirim `version`. Tidak cocok menghasilkan `409 VERSION_CONFLICT` |
| Aksi non-CRUD | `POST /resource/{id}/aksi`, mis. `/finalize`, `/cancel`, `/send` |
| Hapus | `DELETE` pada master berarti arsip (soft delete). Data transaksi final tidak bisa dihapus |
| Bahasa | Pesan error dan label berbahasa Indonesia. Kode error tetap bahasa Inggris dan stabil |

### 1.1 Bentuk respons

```json
// Sukses
{ "data": { }, "meta": { "page": 1, "per_page": 15, "total": 142, "total_pages": 10 } }

// Error
{ "error": {
    "code": "VALIDATION_FAILED",
    "message": "Data tidak valid.",
    "details": [ { "field": "email", "message": "Format email tidak valid" } ],
    "request_id": "01JC8Z..."
} }
```

### 1.2 Daftar, filter, dan paginasi

| Parameter | Arti |
| --- | --- |
| `page`, `per_page` | Halaman (default 1) dan jumlah per halaman (default 15, **maksimal 100**) |
| `q` | Pencarian teks bebas |
| `sort` | Daftar kolom, awalan `-` untuk menurun, mis. `sort=-created_at,full_name` |
| `filter[<field>]` | Filter per field, mis. `filter[status]=active`. Banyak nilai dipisah koma (OR) |
| `cursor`, `limit` | Untuk daftar bertipe log (notifikasi, audit): default 30, maksimal 100. Respons memuat `meta.next_cursor` |
| Tanpa `per_page=all` | Mengambil seluruh data lewat **ekspor** (job), bukan lewat daftar |

Semua daftar otomatis **disaring oleh data scope** user. Total (`meta.total`) hanya menghitung data yang boleh dilihat.

### 1.3 Status HTTP

| Kode | Pemakaian |
| --- | --- |
| `200` | Sukses (baca, ubah, aksi) |
| `201` | Resource dibuat. Header `Location` |
| `202` | Diterima untuk diproses async (job). Isi: `job_id` |
| `204` | Sukses tanpa isi (hapus, tandai) |
| `400` | Permintaan salah bentuk |
| `401` | Belum login atau sesi tidak sah |
| `403` | Tidak punya permission, entitlement, atau perlu re-autentikasi |
| `404` | Tidak ada, **atau di luar scope** \[USULAN A-02\] |
| `409` | Konflik versi, duplikasi, atau status tidak mengizinkan |
| `422` | Validasi gagal atau aturan bisnis menolak |
| `429` | Terlalu banyak permintaan |
| `5xx` | Kesalahan server. Pesan tidak membocorkan detail internal |

**Aturan 403 vs 404:** tanpa permission → `403 PERMISSION_DENIED`. Resource ada tetapi **di luar scope** → `404 NOT_FOUND`, supaya keberadaan data tidak bocor. Daftar tidak pernah menampilkan baris di luar scope.

## 2. Peta area API

| Prefix | Dipakai oleh | Aturan akses |
| --- | --- | --- |
| `/auth`, `/me` | Semua user login | Sesi valid |
| `/ess/*` | ESS Panel | **Selalu diri sendiri.** Tidak ada ID karyawan di path. Karyawan aktif |
| `/admin/*` | Admin Panel | `panel.admin` + permission modul + data scope + entitlement paket |
| `/platform/*` | Superadmin | Role platform |
| `/notifications`, `/push`, `/files`, `/jobs`, `/directory` | ESS dan Admin | Cek permission di tiap endpoint |
| `/realtime/ws` | ESS dan Admin | WebSocket, sesi sah, cek `Origin` |
| `/webhooks/*` | Pihak luar (payment gateway) | Tanpa sesi. Tanda tangan diverifikasi, idempoten |
| `/healthz`, `/readyz` | Monitoring | Publik |

**Kenapa `/ess` dan `/admin` dipisah:** endpoint ESS tidak menerima ID karyawan, sehingga satu kelas bug (mengakses data orang lain) hilang sejak desain. Endpoint admin selalu melewati evaluator permission + scope.

## 3. Legenda tabel endpoint

| Tanda | Arti |
| --- | --- |
| **S** | Hasil disaring oleh data scope. Di luar scope menjadi `404` |
| **R** | Wajib re-autentikasi (`POST /auth/reauth`, berlaku 5 menit) |
| **I** | Wajib header `Idempotency-Key` |
| **J** | Berjalan async. Mengembalikan `202` + `job_id` (lihat `/jobs`) |
| **A** | Diaudit (masuk `audit_logs`) |
| **E** | Dibatasi entitlement paket |
| `(dasar)` | Cukup role dasar `KARYAWAN`. Selalu diri sendiri |

Kolom **Hak** berisi kode permission dari `permission_registry`. Tanda `·` memisahkan lebih dari satu syarat yang semuanya harus terpenuhi.

## 4. Autentikasi, akun, dan sesi

| Method | Path | Fungsi | Hak |
| --- | --- | --- | --- |
| POST | `/auth/login` | Masuk. Body: `email`, `password`, `remember` (bool). Respons: `user`, `memberships`, `active_membership`, `available_panels`, `csrf_token`, `remember_effective` | Publik, rate limit |
| GET | `/auth/csrf` | Ambil token CSRF baru | Sesi |
| POST | `/auth/logout` | Cabut sesi di perangkat ini | Sesi |
| POST | `/auth/logout-all` | Cabut semua sesi milik sendiri. Opsi `except_current` | Sesi · A |
| POST | `/auth/reauth` | Verifikasi ulang kata sandi untuk aksi berisiko | Sesi |
| POST | `/auth/password/forgot` | Kirim tautan reset. Selalu `202` (tidak membocorkan keberadaan email) | Publik, rate limit |
| POST | `/auth/password/reset` | Token + kata sandi baru. Mencabut semua sesi | Publik |
| POST | `/auth/password/change` | Kata sandi lama + baru. **Mencabut sesi lain** (AC-49) | Sesi · A |
| POST | `/auth/switch-tenant` | Ganti membership aktif (satu email, banyak tenant) | Sesi |
| GET | `/auth/oauth/google/start`, `/auth/oauth/google/callback` | Masuk dengan Google \[BERTAHAP\] | Publik |
| GET | `/auth/me` | Profil ringkas, tenant aktif, panel tersedia, status Simpan Sesi | Sesi |
| GET | `/me/access` | **Hak akses efektif:** permission, ringkasan scope per modul, entitlement, panel, `access_version`. Dasar menu dan route guard | Sesi |
| GET | `/me/sessions` | Daftar perangkat dan sesi aktif | Sesi |
| DELETE | `/me/sessions/{id}` | Keluarkan satu perangkat | Sesi · A |
| GET, PATCH | `/me/preferences` | Tema, mode gelap, preferensi tampilan | Sesi |

**Aturan login:**

- `remember = true` → cookie persisten, `expires_at = NULL`. `remember = false` → cookie sesi browser + idle timeout server.
- Bila tenant menonaktifkan Simpan Sesi untuk Admin Panel (`session.remember_allowed_admin = false`), server mengabaikan `remember` dan mengembalikan `remember_effective: false`.
- Percobaan gagal dibatasi (per akun dan per IP) dan dicatat di `login_events`.
- `GET /me/access` mendukung `If-None-Match` dengan `access_version` supaya murah dipanggil ulang.

## 5. Admin: pengguna dan hak akses

Mengimplementasikan PRD Bagian 3. Semua aksi ubah hak akses diaudit dan menaikkan `access_version` user terkait.

### 5.1 Pengguna

| Method | Path | Fungsi | Hak |
| --- | --- | --- | --- |
| GET | `/admin/users` | Daftar pengguna (filter status, role, kantor) | `pengguna.lihat` · S |
| POST | `/admin/users` | Buat akun/membership, opsional hubungkan ke karyawan. Mengirim undangan | `pengguna.buat` · A |
| GET | `/admin/users/{id}` | Detail pengguna | `pengguna.lihat` · S |
| PATCH | `/admin/users/{id}` | Ubah nama, kontak, status | `pengguna.ubah` · A |
| POST | `/admin/users/{id}/disable` | Nonaktifkan akun. Mencabut semua sesi | `pengguna.nonaktifkan` · A |
| POST | `/admin/users/{id}/enable` | Aktifkan kembali | `pengguna.nonaktifkan` · A |
| POST | `/admin/users/{id}/reset-password` | Kirim tautan reset kata sandi | `pengguna.ubah` · A |
| GET | `/admin/users/{id}/sessions` | Sesi aktif pengguna | `pengguna.lihat` |
| DELETE | `/admin/users/{id}/sessions` | Cabut **semua** sesi pengguna | `pengguna.cabut_sesi` · R · A |
| DELETE | `/admin/users/{id}/sessions/{sessionId}` | Cabut satu sesi | `pengguna.cabut_sesi` · A |

### 5.2 Role dan registry permission

| Method | Path | Fungsi | Hak |
| --- | --- | --- | --- |
| GET | `/admin/permissions` | Registry permission (dikelompokkan per modul, hanya yang sesuai entitlement paket) | `pengguna.kelola_akses` |
| GET | `/admin/roles` | Daftar role tenant | `pengguna.lihat` |
| POST | `/admin/roles` | Buat role kustom | `pengguna.kelola_akses` · A |
| GET | `/admin/roles/{id}` | Detail role: permission dan scope preset | `pengguna.lihat` |
| PATCH | `/admin/roles/{id}` | Ubah nama dan deskripsi. Role sistem terbatas | `pengguna.kelola_akses` · A |
| DELETE | `/admin/roles/{id}` | Hapus role kustom yang tidak dipakai | `pengguna.kelola_akses` · A |
| PUT | `/admin/roles/{id}/permissions` | Ganti daftar permission role. Hanya yang dikuasai pemanggil | `pengguna.kelola_akses` · R · A |
| GET, PUT | `/admin/roles/{id}/scopes` | Baca atau ganti aturan scope preset role | `pengguna.kelola_akses` · A |

### 5.3 Hak akses per pengguna

| Method | Path | Fungsi | Hak |
| --- | --- | --- | --- |
| GET | `/admin/users/{id}/roles` | Role pengguna beserta tanggal berlaku | `pengguna.lihat` |
| POST | `/admin/users/{id}/roles` | Tambah role. Body: `role_id`, `valid_from`, `valid_to` | `pengguna.kelola_akses` · A |
| PATCH | `/admin/users/{id}/roles/{userRoleId}` | Ubah masa berlaku | `pengguna.kelola_akses` · A |
| DELETE | `/admin/users/{id}/roles/{userRoleId}` | Cabut role (tanggal pencabutan tercatat) | `pengguna.kelola_akses` · R · A |
| GET | `/admin/users/{id}/permission-overrides` | Override Allow/Deny pengguna | `pengguna.lihat` |
| PUT | `/admin/users/{id}/permission-overrides` | Set massal: tiap permission `allow`, `deny`, atau `inherit` (hapus override) | `pengguna.kelola_akses` · R · A |
| GET | `/admin/users/{id}/scopes` | Scope override per modul | `pengguna.lihat` |
| PUT | `/admin/users/{id}/scopes/{module}` | Ganti scope override untuk satu modul | `pengguna.kelola_akses` · R · A |
| DELETE | `/admin/users/{id}/scopes/{module}` | Hapus override, kembali ke preset role (**bukan** membuka semua) | `pengguna.kelola_akses` · A |
| GET | `/admin/users/{id}/effective-access` | Akses efektif beserta **sumber** tiap hak (role mana atau override) | `pengguna.kelola_akses` |
| POST | `/admin/users/{id}/effective-access/preview` | Pratinjau akses efektif dari perubahan yang belum disimpan | `pengguna.kelola_akses` |
| GET | `/admin/users/{id}/access-history` | Riwayat perubahan hak akses | `pengguna.lihat` · `audit.lihat` |
| GET, PUT | `/admin/position-role-suggestions` | Pemetaan Jabatan → role yang disarankan \[USULAN T-04\] | `pengguna.kelola_akses` |
| GET, POST | `/admin/access-reviews` | Review akses berkala \[BERTAHAP\] | `pengguna.kelola_akses` |

**Aturan keamanan hak akses:**

- Pemanggil hanya boleh memberi permission dan scope yang ia kuasai sendiri. Selebihnya `403 DELEGATION_EXCEEDED` (AC-26).
- Pemanggil tidak bisa mengubah hak akses dirinya sendiri (`403 SELF_ACCESS_CHANGE_FORBIDDEN`).
- Menghapus role `KARYAWAN` dari karyawan aktif ditolak (`409`).
- Setiap perubahan menaikkan `tenant_memberships.access_version` dalam transaksi yang sama.

## 6. Admin: organisasi

### 6.1 Master organisasi

Pola yang sama berlaku untuk lima resource: `regions`, `offices`, `divisions`, `departments`, `positions`. Ganti `{resource}` dengan salah satunya.

| Method | Path | Fungsi | Hak |
| --- | --- | --- | --- |
| GET | `/admin/{resource}` | Daftar. Filter induk (`filter[region_id]`, `filter[division_id]`, `filter[department_id]`), status, `q` | `organisasi.lihat` |
| POST | `/admin/{resource}` | Buat | `organisasi.buat` · A |
| GET | `/admin/{resource}/{id}` | Detail | `organisasi.lihat` |
| PATCH | `/admin/{resource}/{id}` | Ubah | `organisasi.ubah` · A |
| DELETE | `/admin/{resource}/{id}` | Arsip. `409 IN_USE` bila masih dipakai penugasan aktif | `organisasi.hapus` · A |
| GET | `/admin/organization/tree` | Pohon **Divisi → Departemen → Jabatan** | `organisasi.lihat` |
| GET | `/admin/locations/tree` | Pohon **Wilayah → Kantor** | `organisasi.lihat` |

Impor dan ekspor lewat job generik (Bagian 14) dengan `type` `regions`, `offices`, `divisions`, `departments`, atau `positions`.

### 6.2 Geofence kantor

Satu kantor boleh punya banyak polygon (PRD 6.3).

| Method | Path | Fungsi | Hak |
| --- | --- | --- | --- |
| GET | `/admin/offices/{id}/geofences` | Daftar polygon kantor (GeoJSON) | `organisasi.lihat` |
| POST | `/admin/offices/{id}/geofences` | Tambah polygon. Validasi: polygon tertutup, tidak self-intersecting | `organisasi.ubah` · A |
| GET | `/admin/geofences/{id}` | Detail polygon | `organisasi.lihat` |
| PATCH | `/admin/geofences/{id}` | Ubah nama, bentuk, status aktif | `organisasi.ubah` · A |
| DELETE | `/admin/geofences/{id}` | Hapus polygon | `organisasi.ubah` · A |
| POST | `/admin/geofences/test-point` | Uji titik (`lat`, `lng`): polygon mana yang cocok. Untuk editor peta | `organisasi.lihat` |

### 6.3 Lookup untuk dropdown

Endpoint ringan untuk `BaseSearchableSelect` dan `BaseMultiSelect`. Mengembalikan `{ id, label, badge? }` dengan paginasi dan `q`.

| Method | Path | Fungsi | Hak |
| --- | --- | --- | --- |
| GET | `/lookups/{type}` | `type`: `regions`, `offices`, `divisions`, `departments`, `positions`, `roles`, `leave-types`, `shift-templates`, `document-types`, `reimbursement-categories`, `pay-components`, `banks` | Sesuai modul pemakai (minimal `*.lihat` atau dasar untuk ESS) |
| GET | `/lookups/employees` | Pencarian karyawan async. Parameter wajib `for` (mis. `broadcast`, `roster`, `approval`, `leave`) menentukan permission dan scope yang dipakai | Sesuai `for` · S |

> `for` memastikan pencarian karyawan **tidak** menampilkan orang di luar scope modul pemanggil. Tanpa `for` ditolak (`400`).

## 7. Admin: karyawan dan SDM

### 7.1 Karyawan

| Method | Path | Fungsi | Hak |
| --- | --- | --- | --- |
| GET | `/admin/employees` | Daftar. Filter: status, kantor, wilayah, divisi, departemen, jabatan, atasan, rentang tanggal masuk, tipe kontrak | `karyawan.lihat` · S |
| POST | `/admin/employees` | Buat karyawan beserta penugasan awal. `422 QUOTA_EXCEEDED` bila melewati kuota paket | `karyawan.buat` · A · E |
| GET | `/admin/employees/{id}` | Detail. Field sensitif disamarkan tanpa `lihat_sensitif` | `karyawan.lihat` · S |
| GET | `/admin/employees/{id}/summary` | Ringkasan untuk slide-over (identitas, penugasan, kontrak, status) | `karyawan.lihat` · S |
| PATCH | `/admin/employees/{id}` | Ubah data pribadi. Field sensitif butuh `lihat_sensitif` | `karyawan.ubah` · S · A |
| DELETE | `/admin/employees/{id}` | Arsip karyawan nonaktif. Karyawan aktif ditolak (`409`), gunakan terminasi | `karyawan.hapus` · S · A |
| POST | `/admin/employees/{id}/restore` | Pulihkan karyawan terarsip | `karyawan.hapus` · S · A |
| POST | `/admin/employees/bulk-archive` | Arsip massal. Body: `ids[]`. Setiap ID dicek scope | `karyawan.hapus` · R · A |
| GET | `/admin/employees/{id}/timeline` | Riwayat perubahan (penugasan, mutasi, kontrak, status) | `karyawan.lihat` · S |
| GET | `/admin/employees/next-number` | Saran NIP berikutnya | `karyawan.buat` |
| POST | `/admin/employees/{id}/account` | Buat atau hubungkan akun login (ESS) ke karyawan | `pengguna.buat` · `karyawan.ubah` · S · A |

Impor dan ekspor: job generik `type = employees` (Bagian 14). Tiap baris impor dicek terhadap scope pengimpor (AC-54).

### 7.2 Data pendukung karyawan

| Method | Path | Fungsi | Hak |
| --- | --- | --- | --- |
| GET | `/admin/employees/{id}/assignments` | Riwayat penugasan (wilayah, kantor, divisi, departemen, jabatan, atasan) | `karyawan.lihat` · S |
| POST | `/admin/employees/{id}/assignments` | Penugasan baru dengan tanggal efektif (koreksi data). Perubahan reguler lewat **mutasi** (7.4) | `karyawan.ubah` · S · A |
| GET | `/admin/employees/{id}/allowed-offices` | Kantor tambahan untuk presensi | `karyawan.lihat` · S |
| POST | `/admin/employees/{id}/allowed-offices` | Tambah kantor yang diizinkan (dengan masa berlaku) | `karyawan.ubah` · S · A |
| DELETE | `/admin/employees/{id}/allowed-offices/{allowedId}` | Cabut izin kantor | `karyawan.ubah` · S · A |
| GET, POST | `/admin/employees/{id}/families` | Keluarga dan tanggungan | `karyawan.lihat` / `karyawan.ubah` · S |
| PATCH, DELETE | `/admin/employees/{id}/families/{familyId}` | Ubah atau hapus | `karyawan.ubah` · S · A |
| GET, POST | `/admin/employees/{id}/emergency-contacts` | Kontak darurat | `karyawan.lihat` / `karyawan.ubah` · S |
| PATCH, DELETE | `/admin/employees/{id}/emergency-contacts/{contactId}` | Ubah atau hapus | `karyawan.ubah` · S · A |
| GET, POST | `/admin/employees/{id}/bank-accounts` | Rekening bank (nomor disamarkan tanpa `lihat_sensitif`) | `karyawan.lihat_sensitif` / `karyawan.ubah` · S · A |
| PATCH, DELETE | `/admin/employees/{id}/bank-accounts/{accountId}` | Ubah atau nonaktifkan | `karyawan.ubah` · `karyawan.lihat_sensitif` · S · A |
| GET, PUT | `/admin/employees/{id}/tax-bpjs` | NPWP, PTKP, nomor BPJS (berversi tanggal berlaku) | `karyawan.lihat_sensitif` · S · A |

Dokumen karyawan ada di Bagian 12. Komponen gaji per karyawan ada di Bagian 11.

### 7.3 Kontrak

| Method | Path | Fungsi | Hak |
| --- | --- | --- | --- |
| GET | `/admin/contracts` | Daftar kontrak (filter tipe, status, rentang berakhir) | `kontrak.lihat` · S |
| GET | `/admin/contracts/expiring` | Kontrak yang berakhir dalam N hari (`days`, default 30) | `kontrak.lihat` · S |
| POST | `/admin/contracts` | Buat kontrak | `kontrak.buat` · S · A |
| GET | `/admin/contracts/{id}` | Detail | `kontrak.lihat` · S |
| PATCH | `/admin/contracts/{id}` | Ubah | `kontrak.ubah` · S · A |
| DELETE | `/admin/contracts/{id}` | Hapus kontrak yang belum aktif | `kontrak.hapus` · S · A |
| POST | `/admin/contracts/{id}/renew` | Perpanjang. Membuat kontrak baru, menutup yang lama | `kontrak.buat` · S · A |
| GET | `/admin/employees/{id}/contracts` | Kontrak milik satu karyawan | `kontrak.lihat` · S |

### 7.4 Mutasi, promosi, dan demosi

| Method | Path | Fungsi | Hak |
| --- | --- | --- | --- |
| GET | `/admin/employee-changes` | Daftar pengajuan perubahan (filter jenis, status, tanggal efektif) | `mutasi.lihat` · S |
| POST | `/admin/employee-changes` | Buat draft. Body: `employee_id`, `change_type` (`mutasi`, `promosi`, `demosi`), `effective_date`, tujuan baru | `mutasi.buat` · S · A |
| GET | `/admin/employee-changes/{id}` | Detail beserta rantai approval | `mutasi.lihat` · S |
| PATCH | `/admin/employee-changes/{id}` | Ubah draft | `mutasi.ubah` · S · A |
| DELETE | `/admin/employee-changes/{id}` | Hapus draft | `mutasi.ubah` · S · A |
| POST | `/admin/employee-changes/{id}/submit` | Ajukan ke approval | `mutasi.buat` · S · A |

Persetujuan lewat mesin approval (Bagian 10). **Penerapan** ke `employee_assignments` dilakukan job sistem pada tanggal efektif, bukan endpoint publik.

### 7.5 SP dan konseling

| Method | Path | Fungsi | Hak |
| --- | --- | --- | --- |
| GET | `/admin/disciplinary-actions` | Daftar | `disiplin.lihat` · S |
| POST | `/admin/disciplinary-actions` | Terbitkan SP atau catatan konseling | `disiplin.buat` · S · A |
| GET | `/admin/disciplinary-actions/{id}` | Detail | `disiplin.lihat` · S |
| PATCH | `/admin/disciplinary-actions/{id}` | Ubah | `disiplin.ubah` · S · A |
| DELETE | `/admin/disciplinary-actions/{id}` | Batalkan | `disiplin.hapus` · S · A |

### 7.6 Resign dan terminasi

| Method | Path | Fungsi | Hak |
| --- | --- | --- | --- |
| GET | `/admin/terminations` | Daftar | `terminasi.lihat` · S |
| POST | `/admin/terminations` | Buat proses terminasi (resign, pemutusan, pensiun) | `terminasi.buat` · S · A |
| GET | `/admin/terminations/{id}` | Detail dan checklist | `terminasi.lihat` · S |
| PATCH | `/admin/terminations/{id}` | Ubah tanggal efektif atau alasan | `terminasi.ubah` · S · A |
| PUT | `/admin/terminations/{id}/checklist` | Perbarui checklist exit (aset kembali, serah terima) | `terminasi.ubah` · S · A |
| POST | `/admin/terminations/{id}/submit` | Ajukan ke approval | `terminasi.buat` · S · A |
| POST | `/admin/terminations/{id}/cancel` | Batalkan sebelum tanggal efektif | `terminasi.ubah` · S · A |

**Efek pada tanggal efektif** (job sistem, bukan endpoint): status karyawan berubah, semua `user_roles` dicabut, semua sesi dan koneksi WebSocket dihentikan, payroll akhir dipicu (AC-58).

### 7.7 Aset karyawan

| Method | Path | Fungsi | Hak |
| --- | --- | --- | --- |
| GET, POST | `/admin/assets` | Master aset | `aset.lihat` / `aset.buat` · A |
| PATCH, DELETE | `/admin/assets/{id}` | Ubah atau arsip aset | `aset.ubah` / `aset.hapus` · A |
| GET | `/admin/employee-assets` | Aset yang sedang dipinjam (filter karyawan, status) | `aset.lihat` · S |
| POST | `/admin/employee-assets` | Serahkan aset ke karyawan | `aset.buat` · S · A |
| POST | `/admin/employee-assets/{id}/return` | Catat pengembalian | `aset.ubah` · S · A |

### 7.8 ESS: profil sendiri

| Method | Path | Fungsi | Hak |
| --- | --- | --- | --- |
| GET | `/ess/profile` | Data diri sendiri (penugasan, kontrak ringkas, atasan) | (dasar) |
| POST | `/ess/profile/photo` | Ganti foto (`file_id` hasil unggah) | (dasar) |
| POST | `/ess/profile/change-requests` | Ajukan koreksi data pribadi (nama, rekening, NPWP). Menunggu persetujuan HR | (dasar) |
| GET | `/ess/profile/change-requests` | Status pengajuan koreksi | (dasar) |
| DELETE | `/ess/profile/change-requests/{id}` | Batalkan pengajuan | (dasar) |
| GET, POST | `/ess/profile/emergency-contacts` | Kontak darurat sendiri | (dasar) |
| PATCH, DELETE | `/ess/profile/emergency-contacts/{id}` | Ubah atau hapus | (dasar) |
| POST | `/ess/resignation` | Ajukan pengunduran diri | (dasar) |
| GET | `/ess/resignation` | Status pengajuan resign | (dasar) |

Data pribadi tertentu **tidak** bisa diubah langsung oleh karyawan. Hanya lewat `change-requests` yang disetujui HR (PRD 4.4).

### 7.9 Direktori (Teman Kantor)

| Method | Path | Fungsi | Hak |
| --- | --- | --- | --- |
| GET | `/directory/employees` | Direktori ringkas seluruh tenant. **Hanya field publik:** nama, jabatan, divisi, kantor, kontak kerja, foto. Tidak disaring scope, tetapi field dibatasi | `direktori.lihat` |
| GET | `/directory/employees/{id}` | Kartu kontak satu karyawan (field publik) | `direktori.lihat` |

## 8. Kehadiran dan jadwal

### 8.1 ESS: absensi dan jadwal pribadi

| Method | Path | Fungsi | Hak |
| --- | --- | --- | --- |
| GET | `/ess/attendance/today` | Shift hari ini, status presensi, event terakhir, kantor yang diizinkan, ringkasan geofence | (dasar) |
| POST | `/ess/attendance/location-check` | Uji lokasi (`lat`, `lng`, `accuracy_m`) **tanpa mencatat**. Hasil: `inside`, `outside`, `flexible`, `unknown` | (dasar) |
| POST | `/ess/attendance/clock-in` | Presensi masuk. Body: `client_event_id`, `lat`, `lng`, `accuracy_m`, `device_time`, `selfie_file_id` | (dasar) · I |
| POST | `/ess/attendance/clock-out` | Presensi pulang. Body sama | (dasar) · I |
| GET | `/ess/attendance/history` | Riwayat presensi (`from`, `to`), berpaginasi | (dasar) |
| GET | `/ess/attendance/calendar` | Status per hari untuk satu bulan (`month=YYYY-MM`) | (dasar) |
| GET | `/ess/attendance/summary` | Ringkasan bulanan: hadir, terlambat, jam kerja | (dasar) |
| POST | `/ess/attendance/corrections` | Ajukan koreksi (`shift_instance_id`, `requested_type`, `requested_time`, `reason`, lampiran) | (dasar) |
| GET | `/ess/attendance/corrections` | Daftar koreksi sendiri | (dasar) |
| GET | `/ess/schedule` | Jadwal pribadi (`from`, `to`): shift instance | (dasar) |
| GET | `/ess/shift-swaps` | Pertukaran shift sendiri dan yang diusulkan ke saya | (dasar) |
| POST | `/ess/shift-swaps` | Usulkan tukar shift dengan rekan | `jadwal.ubah` (terbatas) |
| POST | `/ess/shift-swaps/{id}/respond` | Rekan menerima atau menolak (`accept`: bool) | (dasar) |

**Aturan clock-in/out:**

- `client_event_id` dibuat klien per percobaan dan menjadi kunci idempotensi (`UNIQUE tenant, employee, client_event_id`). Retry atau double tap menghasilkan **satu event** (AC-21).
- Waktu resmi adalah **waktu server**. `device_time` hanya informasi (AC-18).
- Backend memvalidasi: ada shift instance yang cocok, dalam jendela clock-in/out, lokasi terhadap polygon kantor yang diizinkan (kecuali shift fleksibel lokasi), akurasi GPS, dan selfie bila diwajibkan.
- Respons memuat `timing_status` (`on_time`, `late`, `early_leave`, `out_of_window`) dan `location_status`.
- Presensi sah **membatalkan** jadwal reminder terkait (AC-36) dan memicu pembaruan WebSocket ke atasan.
- Kode error khusus: `LOCATION_OUT_OF_AREA`, `LOCATION_ACCURACY_LOW`, `OUT_OF_WINDOW`, `ALREADY_CLOCKED_IN`, `ALREADY_CLOCKED_OUT`, `SELFIE_REQUIRED`, `NO_SHIFT_FOUND`.
- Pembatalan atau kirim ulang koreksi memakai `/ess/requests` (Bagian 10).

### 8.2 Admin: monitor dan rekap

| Method | Path | Fungsi | Hak |
| --- | --- | --- | --- |
| GET | `/admin/attendance/today` | Status real-time per karyawan (filter kantor, departemen, status). Pembaruan lewat WebSocket | `absensi.lihat` · S |
| GET | `/admin/attendance/not-yet` | Daftar **belum presensi** dengan kategori: belum waktunya, belum absen, izin/cuti, libur, tidak dijadwalkan, menunggu koreksi | `absensi.monitor_bawahan` · S |
| GET | `/admin/attendance/events` | Event presensi mentah (filter karyawan, tanggal, status) | `absensi.lihat` · S |
| GET | `/admin/attendance/events/{id}` | Detail event: lokasi, polygon cocok, selfie (URL sementara) | `absensi.lihat` · S |
| GET | `/admin/attendance/daily` | Rekap harian | `absensi.lihat` · S |
| GET | `/admin/attendance/summary` | Agregat bulanan per karyawan atau departemen | `absensi.lihat` · S |
| POST | `/admin/attendance/recalculate` | Hitung ulang rekap harian untuk rentang tanggal | `absensi.koreksi` · S · A · J |
| GET | `/admin/attendance/corrections` | Daftar pengajuan koreksi | `absensi.lihat` · S |
| GET | `/admin/attendance/corrections/{id}` | Detail koreksi dan rantai approval | `absensi.lihat` · S |
| POST | `/admin/attendance/corrections` | Buat koreksi atas nama karyawan. Tetap melewati approval. **Event asli tidak diubah**, hasilnya event baru bersumber `correction` | `absensi.koreksi` · S · A |

Ekspor: job `type = attendance` (Bagian 14), dengan `absensi.ekspor`. Persetujuan koreksi lewat Bagian 10.

### 8.3 Admin: master shift, roster, dan libur

| Method | Path | Fungsi | Hak |
| --- | --- | --- | --- |
| GET | `/admin/shift-templates` | Daftar master shift | `jadwal.lihat` |
| POST | `/admin/shift-templates` | Buat shift (reguler, fleksibel, lintas hari; jendela clock-in/out; toleransi; wajib lokasi/selfie) | `jadwal.buat` · A |
| GET | `/admin/shift-templates/{id}` | Detail | `jadwal.lihat` |
| PATCH | `/admin/shift-templates/{id}` | Ubah. **Tidak** mengubah shift instance yang sudah ada (AC-23) | `jadwal.ubah` · A |
| DELETE | `/admin/shift-templates/{id}` | Nonaktifkan | `jadwal.hapus` · A |
| POST | `/admin/shift-templates/{id}/duplicate` | Gandakan | `jadwal.buat` · A |
| GET | `/admin/work-schedules` | Daftar penugasan roster (filter karyawan, kantor, rentang) | `jadwal.lihat` · S |
| POST | `/admin/work-schedules` | Tugaskan shift ke satu karyawan | `jadwal.buat` · S · A |
| POST | `/admin/work-schedules/bulk` | Tugaskan banyak karyawan atau pola mingguan. Opsi `dry_run` untuk pratinjau bentrok | `jadwal.buat` · S · A |
| PATCH | `/admin/work-schedules/{id}` | Ubah penugasan (draft) | `jadwal.ubah` · S · A |
| DELETE | `/admin/work-schedules/{id}` | Hapus penugasan draft | `jadwal.hapus` · S · A |
| POST | `/admin/work-schedules/publish` | Publikasikan roster untuk rentang dan filter. Membuat shift instance dan memberi notifikasi | `jadwal.publikasi` · S · A |
| GET | `/admin/roster` | Tampilan grid roster (karyawan × tanggal) untuk satu kantor atau departemen | `jadwal.lihat` · S |
| GET | `/admin/shift-instances` | Daftar kejadian kerja aktual | `jadwal.lihat` · S |
| GET | `/admin/shift-instances/{id}` | Detail, termasuk `rule_snapshot` | `jadwal.lihat` · S |
| POST | `/admin/shift-instances/{id}/cancel` | Batalkan satu instance (alasan wajib). Membatalkan reminder terkait | `jadwal.ubah` · S · A |
| POST | `/admin/shift-instances/generate` | Materialisasi instance untuk rentang tanggal (juga berjalan otomatis N hari ke depan) | `jadwal.publikasi` · A · J |
| GET | `/admin/holidays` | Daftar hari libur | `jadwal.lihat` |
| POST | `/admin/holidays` | Tambah hari libur | `jadwal.ubah` · A |
| PATCH, DELETE | `/admin/holidays/{id}` | Ubah atau hapus | `jadwal.ubah` · A |
| POST | `/admin/holidays/seed-national` | Isi libur nasional untuk satu tahun (`year`) \[USULAN\] | `jadwal.ubah` · A |
| GET | `/holidays` | Libur untuk tampilan kalender (`year`) | Sesi |
| GET | `/admin/shift-swaps` | Pertukaran shift dalam scope | `jadwal.lihat` · S |

Impor roster dan ekspor lewat job generik `type = work_schedules`.

## 9. Cuti, lembur, kasbon, dan reimbursement

Pembatalan dan kirim ulang untuk semua pengajuan ESS memakai `/ess/requests/{requestId}/cancel` dan `/resubmit` (Bagian 10).

### 9.1 ESS: cuti dan izin

| Method | Path | Fungsi | Hak |
| --- | --- | --- | --- |
| GET | `/ess/leave/types` | Jenis cuti yang tersedia untuk saya | (dasar) |
| GET | `/ess/leave/balances` | Saldo cuti per jenis | (dasar) |
| GET | `/ess/leave/balances/{leaveTypeId}/ledger` | Riwayat mutasi saldo | (dasar) |
| POST | `/ess/leave-requests/preview` | Hitung jumlah hari, bentrok, saldo setelah, dan jalur approval, tanpa menyimpan | (dasar) |
| GET | `/ess/leave-requests` | Daftar pengajuan sendiri | (dasar) |
| POST | `/ess/leave-requests` | Ajukan cuti atau izin/sakit (lampiran bila diwajibkan) | `cuti.buat` (diri sendiri) |
| GET | `/ess/leave-requests/{id}` | Detail dan status | (dasar) |

### 9.2 Admin: cuti dan izin

| Method | Path | Fungsi | Hak |
| --- | --- | --- | --- |
| GET | `/admin/leave-requests` | Daftar pengajuan (filter status, jenis, tanggal) | `cuti.lihat` · S |
| GET | `/admin/leave-requests/{id}` | Detail dan rantai approval | `cuti.lihat` · S |
| POST | `/admin/leave-requests` | Buat atas nama karyawan | `cuti.buat` · S · A |
| PATCH | `/admin/leave-requests/{id}` | Ubah pengajuan yang berstatus draft atau dikembalikan | `cuti.ubah` · S · A |
| GET | `/admin/leave-calendar` | Kalender cuti tim (`month`, `office_id`) | `cuti.lihat` · S |
| GET, POST | `/admin/leave-types` | Jenis cuti | `cuti.kelola` · A |
| PATCH, DELETE | `/admin/leave-types/{id}` | Ubah atau nonaktifkan | `cuti.kelola` · A |
| GET, POST | `/admin/leave-policies` | Kebijakan kuota dan akrual (berversi tanggal berlaku) | `cuti.kelola` · A |
| PATCH, DELETE | `/admin/leave-policies/{id}` | Ubah atau akhiri | `cuti.kelola` · A |
| GET | `/admin/leave-balances` | Saldo (filter karyawan, jenis, tahun) | `cuti.lihat` · S |
| GET | `/admin/employees/{id}/leave-balances` | Saldo satu karyawan | `cuti.lihat` · S |
| GET | `/admin/leave-balances/ledger` | Ledger saldo (`employee_id`, `leave_type_id`) | `cuti.lihat` · S |
| POST | `/admin/leave-balances/adjustments` | Penyesuaian saldo manual. Alasan wajib | `cuti.kelola` · S · R · I · A |
| POST | `/admin/leave-balances/accrual-run` | Jalankan akrual untuk periode | `cuti.kelola` · A · J |

Saldo dikurangi oleh handler persetujuan akhir dengan **satu baris ledger** (`UNIQUE ref_type, ref_id, entry_type`). Memproses ulang tidak memotong dua kali (AC-28).

### 9.3 Lembur

| Method | Path | Fungsi | Hak |
| --- | --- | --- | --- |
| GET | `/ess/overtime-requests` | Daftar lembur sendiri | (dasar) |
| POST | `/ess/overtime-requests` | Ajukan SPKL. Body: `work_date`, `proposed_minutes`, `reason` | `lembur.buat` (diri sendiri) |
| GET | `/ess/overtime-requests/{id}` | Detail dan status | (dasar) |
| GET | `/admin/overtime-requests` | Daftar lembur (filter status, tanggal) | `lembur.lihat` · S |
| GET | `/admin/overtime-requests/{id}` | Detail: jam diusulkan, disetujui, dibayar | `lembur.lihat` · S |
| POST | `/admin/overtime-requests` | Buat atas nama karyawan | `lembur.buat` · S · A |
| PATCH | `/admin/overtime-requests/{id}` | Ubah draft atau yang dikembalikan | `lembur.ubah` · S · A |
| GET | `/admin/overtime-summary` | Rekap per periode | `lembur.lihat` · S |

Approver mengisi `approved_minutes` pada aksi approval (`data.approved_minutes`). Tiga angka (diusulkan, disetujui, dibayar) disimpan terpisah (AC-29).

### 9.4 Kasbon

| Method | Path | Fungsi | Hak |
| --- | --- | --- | --- |
| POST | `/ess/loans/simulate` | Simulasi cicilan (nominal, tenor) dan cek plafon | (dasar) |
| GET | `/ess/loans` | Daftar kasbon sendiri | (dasar) |
| POST | `/ess/loans` | Ajukan kasbon | `kasbon.buat` (diri sendiri) |
| GET | `/ess/loans/{id}` | Detail beserta jadwal cicilan | (dasar) |
| GET | `/admin/loans` | Daftar kasbon | `kasbon.lihat` · S |
| GET | `/admin/loans/{id}` | Detail dan cicilan | `kasbon.lihat` · S |
| POST | `/admin/loans` | Buat atas nama karyawan | `kasbon.buat` · S · A |
| POST | `/admin/loans/{id}/disburse` | Cairkan kasbon yang sudah disetujui. **Tidak boleh ganda** (AC-31) | `kasbon.cairkan` · S · R · I · A |
| POST | `/admin/loans/{id}/installments/{seq}/waive` | Bebaskan satu cicilan | `kasbon.kelola` · S · R · A |
| POST | `/admin/loans/{id}/settle` | Pelunasan dipercepat | `kasbon.kelola` · S · R · A |
| GET, PUT | `/admin/loan-policies` | Plafon dan tenor maksimum | `kasbon.kelola` · A |

> Aksi `kelola` pada modul `kasbon` belum ada di registry PRD 3.5. \[USULAN\] tambahkan.

### 9.5 Reimbursement

| Method | Path | Fungsi | Hak |
| --- | --- | --- | --- |
| GET | `/ess/reimbursements` | Daftar klaim sendiri | (dasar) |
| POST | `/ess/reimbursements` | Ajukan klaim (kategori, tanggal, nominal, kuitansi). `409 DUPLICATE_SUSPECTED` bila terindikasi duplikat, kecuali `confirm_duplicate: true` | `reimbursement.buat` (diri sendiri) |
| GET | `/ess/reimbursements/{id}` | Detail dan status | (dasar) |
| POST | `/ess/reimbursements/{id}/receipts` | Tambah kuitansi (`file_ids[]`) pada klaim yang dikembalikan | (dasar) |
| GET | `/admin/reimbursements` | Daftar klaim | `reimbursement.lihat` · S |
| GET | `/admin/reimbursements/{id}` | Detail dan kuitansi | `reimbursement.lihat` · S |
| POST | `/admin/reimbursements/{id}/pay` | Tandai dibayar (`payment_ref`). Tidak boleh ganda | `reimbursement.bayar` · S · R · I · A |
| GET, POST | `/admin/reimbursement-categories` | Kategori dan batas bulanan | `pengaturan.kelola` · A |
| PATCH, DELETE | `/admin/reimbursement-categories/{id}` | Ubah atau nonaktifkan | `pengaturan.kelola` · A |

## 10. Approval terpusat

Semua jenis pengajuan memakai satu mesin (`requests` → `approval_instances` → `approval_tasks`). Endpoint di bawah berlaku untuk **semua** `request_type`: `leave`, `overtime`, `attendance_correction`, `loan`, `reimbursement`, `mutation`, `promotion`, `payroll`, `termination`, `shift_swap`, `profile_change`.

### 10.1 Admin: inbox dan keputusan

| Method | Path | Fungsi | Hak |
| --- | --- | --- | --- |
| GET | `/admin/approvals/inbox` | Tugas yang menunggu **saya** (filter jenis, status, terlambat) | `persetujuan.lihat` |
| GET | `/admin/approvals/summary` | Jumlah menunggu per jenis (untuk badge) | `persetujuan.lihat` |
| GET | `/admin/approvals` | Semua pengajuan dalam scope (filter jenis, status, tanggal) | `persetujuan.lihat` · S |
| GET | `/admin/approvals/{requestId}` | Detail: data pengajuan, rantai approval, riwayat aksi | `persetujuan.lihat` · S |
| POST | `/admin/approvals/{requestId}/actions` | **Keputusan.** Body: `action` (`approve`, `reject`, `return`), `reason` (wajib untuk `reject` dan `return`), `level_no`, `version`, `data` (mis. `approved_minutes`) | `{modul}.approve` · S · A |
| POST | `/admin/approvals/bulk-actions` | Keputusan massal. Tiap item dievaluasi sendiri. Respons per item | `{modul}.approve` · S · I · A |
| POST | `/admin/approvals/{requestId}/reassign` | Alihkan tahap ke approver lain yang berhak \[TERBUKA T-11\] | `persetujuan.delegasikan` · S · A |

**Aturan keputusan:**

- Pemanggil harus punya task `open` pada tahap itu. Task dibuat saat tahap diaktifkan dari evaluator (permission `*.approve` **dan** scope mencakup pengaju). Pemanggil lain mendapat `403 NOT_ASSIGNED`.
- **Pengaju tidak pernah masuk daftar approver.** Menyetujui pengajuan sendiri ditolak `403 SELF_APPROVAL_FORBIDDEN` (AC-56).
- Aksi atomik: `approval_instances` dikunci (`FOR UPDATE`) dan `version` dicek. Dua approver bersamaan hanya menghasilkan satu hasil (`409 ALREADY_DECIDED`) (AC-25).
- Persetujuan akhir menjalankan **handler domain** dalam transaksi yang sama (potong saldo cuti, set jam lembur disetujui, buat event koreksi, jadwalkan mutasi).
- Setiap perubahan status mencatat `approval_actions` dan menulis ke `notification_outbox`.

### 10.2 ESS: pengajuan saya

| Method | Path | Fungsi | Hak |
| --- | --- | --- | --- |
| GET | `/ess/requests` | Semua pengajuan saya lintas jenis (filter jenis, status) | (dasar) |
| GET | `/ess/requests/{requestId}` | Status dan posisi approval | (dasar) |
| POST | `/ess/requests/{requestId}/cancel` | Batalkan (hanya `pending` atau `returned`) | (dasar) |
| POST | `/ess/requests/{requestId}/resubmit` | Kirim ulang setelah `returned`, dengan perbaikan | (dasar) |

### 10.3 Pengaturan alur approval

| Method | Path | Fungsi | Hak |
| --- | --- | --- | --- |
| GET | `/admin/approval-flows` | Daftar alur (per jenis pengajuan) | `pengaturan.lihat` |
| POST | `/admin/approval-flows` | Buat alur | `pengaturan.kelola` · A |
| GET, PATCH, DELETE | `/admin/approval-flows/{id}` | Detail, ubah, nonaktifkan | `pengaturan.lihat` / `pengaturan.kelola` · A |
| PUT | `/admin/approval-flows/{id}/steps` | Ganti tahap: `level_no`, `approver_kind` (`reports_to`, `role`, `position`, `membership`), `approver_ref`, `sla_hours` | `pengaturan.kelola` · A |
| POST | `/admin/approval-flows/{id}/set-default` | Jadikan default untuk jenisnya | `pengaturan.kelola` · A |
| POST | `/admin/approval-flows/simulate` | Simulasi: untuk `request_type` dan `employee_id`, siapa approver tiap tahap (termasuk fallback HR) | `pengaturan.kelola` · S |
| GET, POST | `/admin/approval-delegations` | Delegasi approval \[TERBUKA T-11\] | `persetujuan.delegasikan` · A |
| DELETE | `/admin/approval-delegations/{id}` | Cabut delegasi | `persetujuan.delegasikan` · A |

## 11. Payroll

Prinsip (PRD 6.8): hasil final adalah **snapshot** yang tidak bisa diubah. Perbaikan pasca-final hanya lewat `adjustments`. Tidak ada endpoint untuk membuka kembali run yang sudah final.

### 11.1 Komponen dan aturan

| Method | Path | Fungsi | Hak |
| --- | --- | --- | --- |
| GET | `/admin/payroll/components` | Daftar komponen gaji | `penggajian.lihat` |
| POST | `/admin/payroll/components` | Buat komponen (tunjangan, potongan, kustom) | `penggajian.kelola` · A |
| GET, PATCH, DELETE | `/admin/payroll/components/{id}` | Detail, ubah, nonaktifkan | `penggajian.lihat` / `penggajian.kelola` · A |
| GET | `/admin/employees/{id}/pay-components` | Komponen gaji karyawan (nominal disamarkan tanpa `lihat_sensitif`) | `penggajian.lihat` · `penggajian.lihat_sensitif` · S |
| PUT | `/admin/employees/{id}/pay-components` | Ganti komponen karyawan mulai `valid_from` | `penggajian.kelola` · `penggajian.lihat_sensitif` · S · R · A |
| GET | `/admin/payroll/rules/tax` | Aturan PPh 21 (TER, PTKP) yang berlaku pada `date`. Hanya baca, dikelola platform \[D-05\] | `penggajian.lihat` |
| GET | `/admin/payroll/rules/bpjs` | Aturan BPJS yang berlaku pada `date`. Hanya baca | `penggajian.lihat` |
| GET, PUT | `/admin/payroll/settings` | Tanggal gajian, pembulatan, pengaturan umum | `penggajian.kelola` · A |

### 11.2 Periode dan payroll run

| Method | Path | Fungsi | Hak |
| --- | --- | --- | --- |
| GET | `/admin/payroll/periods` | Daftar periode | `penggajian.lihat` |
| POST | `/admin/payroll/periods` | Buka periode (`year`, `month`) | `penggajian.proses` · A |
| GET | `/admin/payroll/periods/{id}` | Detail periode dan run-nya | `penggajian.lihat` |
| POST | `/admin/payroll/periods/{id}/close` | Tutup periode setelah semua run final | `penggajian.finalisasi` · R · A |
| POST | `/admin/payroll/periods/{id}/runs` | Buat run dan **hitung draft** dari absensi, cuti, lembur, kasbon, komponen | `penggajian.proses` · A · J |
| GET | `/admin/payroll/runs` | Daftar run (filter periode, status) | `penggajian.lihat` |
| GET | `/admin/payroll/runs/{id}` | Detail run: status, total, `rules_snapshot` | `penggajian.lihat` |
| GET | `/admin/payroll/runs/{id}/summary` | Ringkasan: total bruto, potongan, pajak, neto, jumlah peringatan | `penggajian.lihat` |
| GET | `/admin/payroll/runs/{id}/validation` | Daftar isu sebelum finalisasi (absensi belum final, rekening kosong, PTKP kosong) | `penggajian.lihat` |
| GET | `/admin/payroll/runs/{id}/items` | Baris per karyawan, berpaginasi | `penggajian.lihat` · `penggajian.lihat_sensitif` · S |
| GET | `/admin/payroll/runs/{id}/items/{itemId}` | Rincian komponen satu karyawan | `penggajian.lihat` · `penggajian.lihat_sensitif` · S · A |
| POST | `/admin/payroll/runs/{id}/recalculate` | Hitung ulang (hanya run `draft`) | `penggajian.proses` · A · J |
| POST | `/admin/payroll/runs/{id}/items/{itemId}/lines` | Tambah baris manual (hanya draft) | `penggajian.proses` · S · A |
| DELETE | `/admin/payroll/runs/{id}/items/{itemId}/lines/{lineId}` | Hapus baris manual (hanya draft) | `penggajian.proses` · S · A |
| POST | `/admin/payroll/runs/{id}/submit` | Ajukan ke approval (`request_type = payroll`) | `penggajian.proses` · A |
| POST | `/admin/payroll/runs/{id}/finalize` | **Finalisasi dan kunci.** Syarat: sudah disetujui. Setelah ini item dan baris tidak bisa diubah (AC-32) | `penggajian.finalisasi` · R · I · A |
| GET, POST | `/admin/payroll/runs/{id}/adjustments` | Penyesuaian **pasca-final** (`employee_id`, `component`, `amount`, `reason`) | `penggajian.proses` · S · R · A |
| POST | `/admin/payroll/runs/{id}/bank-export` | Buat file transfer bank (`format`) | `penggajian.ekspor_bank` · R · A · J |
| GET | `/admin/payroll/bank-formats` | Format bank yang didukung | `penggajian.ekspor_bank` |

Persetujuan run lewat Bagian 10 dengan `penggajian.approve`. Finalisasi hanya sah bila approval selesai dan `validation` tidak memuat isu berat.

### 11.3 Slip gaji

| Method | Path | Fungsi | Hak |
| --- | --- | --- | --- |
| POST | `/admin/payroll/runs/{id}/payslips/publish` | Terbitkan slip untuk run final. Notifikasi ke karyawan **tanpa nominal** | `penggajian.finalisasi` · A · J |
| GET | `/admin/payslips` | Daftar slip (filter run, karyawan) | `penggajian.lihat_slip` · S |
| GET | `/admin/payslips/{id}` | Detail slip | `penggajian.lihat_slip` · `penggajian.lihat_sensitif` · S · A |
| GET | `/admin/payslips/{id}/download` | Unduh PDF. Akses dicek ulang, `302` ke URL sementara | `penggajian.lihat_slip` · S · A |
| GET | `/ess/payslips` | Daftar slip sendiri | (dasar) |
| GET | `/ess/payslips/{id}` | Detail slip sendiri | (dasar) |
| GET | `/ess/payslips/{id}/download` | Unduh slip sendiri | (dasar) · A |

## 12. File, dokumen, dan agenda

### 12.1 Berkas

Berkas tidak lewat API. API hanya mengeluarkan **URL unggah sementara** ke storage S3-compatible, lalu memverifikasi hasilnya.

| Method | Path | Fungsi | Hak |
| --- | --- | --- | --- |
| POST | `/files/uploads` | Minta unggah. Body: `purpose`, `original_name`, `mime_type`, `size_bytes`, `sha256`. Respons: `file_id`, `upload_url`, `method`, `headers`, `expires_at` | Sesuai `purpose` |
| POST | `/files/{id}/complete` | Konfirmasi selesai unggah. Server memverifikasi ukuran dan hash, lalu memindai | Pengunggah |
| GET | `/files/{id}` | Metadata berkas | Sesuai entitas terikat |
| GET | `/files/{id}/download` | `302` ke URL sementara (sekitar 5 menit). Akses dicek ulang tiap kali. Untuk `purpose` sensitif diaudit | Sesuai entitas terikat · A |
| DELETE | `/files/{id}` | Hapus berkas yang belum terikat, atau berkas yang berhak dihapus pemanggil | Pengunggah / `dokumen.hapus` |

**Aturan:** `purpose` menentukan batas ukuran, jenis MIME yang diizinkan, dan siapa boleh mengunduh. Nilai: `selfie`, `contract`, `receipt`, `payslip`, `document`, `avatar`, `attachment`. Selfie presensi hanya bisa diunduh pemiliknya dan pengguna dengan `absensi.lihat` yang scope-nya mencakup pemilik. Kuota storage tenant dicek saat `uploads` (`422 STORAGE_QUOTA_EXCEEDED`).

### 12.2 Dokumen

| Method | Path | Fungsi | Hak |
| --- | --- | --- | --- |
| GET | `/ess/documents` | Dokumen pribadi sendiri | (dasar) |
| POST | `/ess/documents` | Unggah dokumen pribadi (status menunggu verifikasi HR) | (dasar) |
| GET | `/ess/documents/{id}` | Detail | (dasar) |
| GET | `/ess/company-documents` | Dokumen perusahaan yang boleh saya lihat | `dokumen.lihat` |
| GET | `/ess/company-documents/{id}` | Detail dokumen perusahaan | `dokumen.lihat` |
| GET | `/admin/employee-documents` | Daftar dokumen karyawan (filter jenis, status, akan kedaluwarsa) | `dokumen.lihat` · S |
| GET | `/admin/employees/{id}/documents` | Dokumen satu karyawan | `dokumen.lihat` · S |
| POST | `/admin/employee-documents` | Tambah dokumen untuk karyawan | `dokumen.buat` · S · A |
| GET, PATCH, DELETE | `/admin/employee-documents/{id}` | Detail, ubah, hapus | `dokumen.lihat` / `dokumen.ubah` / `dokumen.hapus` · S · A |
| POST | `/admin/employee-documents/{id}/verify` | Verifikasi dokumen yang diunggah karyawan | `dokumen.ubah` · S · A |
| GET, POST | `/admin/document-types` | Jenis dokumen (wajib, perlu kedaluwarsa) | `dokumen.lihat` / `dokumen.ubah` · A |
| PATCH, DELETE | `/admin/document-types/{id}` | Ubah atau nonaktifkan | `dokumen.ubah` · A |
| GET, POST | `/admin/company-documents` | Dokumen perusahaan | `dokumen.lihat` / `dokumen.buat` · A |
| GET, PATCH, DELETE | `/admin/company-documents/{id}` | Detail, ubah, hapus | `dokumen.lihat` / `dokumen.ubah` / `dokumen.hapus` · A |
| POST | `/admin/company-documents/{id}/publish` | Terbitkan (dengan `visibility` `all` atau `scoped`) | `dokumen.ubah` · A |
| GET, POST | `/admin/document-templates` | Template surat | `dokumen.lihat` / `dokumen.ubah` · A |
| GET, PATCH, DELETE | `/admin/document-templates/{id}` | Detail, ubah, hapus | `dokumen.ubah` · A |
| POST | `/admin/document-templates/{id}/render` | Hasilkan surat untuk `employee_id` dengan variabel. Mengembalikan `file_id` | `dokumen.buat` · S · A |

### 12.3 Agenda

| Method | Path | Fungsi | Hak |
| --- | --- | --- | --- |
| GET | `/agenda` | Agenda yang boleh saya lihat (`from`, `to`) | Sesi |
| GET | `/admin/agenda-events` | Daftar agenda untuk dikelola | `agenda.lihat` |
| POST | `/admin/agenda-events` | Buat agenda | `agenda.buat` · A |
| PATCH | `/admin/agenda-events/{id}` | Ubah | `agenda.ubah` · A |
| DELETE | `/admin/agenda-events/{id}` | Hapus | `agenda.hapus` · A |

## 13. Notifikasi, broadcast, dan real-time

MySQL adalah sumber kebenaran notifikasi. REST untuk membaca dan bertindak. WebSocket hanya mendorong pembaruan (PRD 7.7).

### 13.1 Inbox dan preferensi (semua pengguna)

| Method | Path | Fungsi | Hak |
| --- | --- | --- | --- |
| GET | `/notifications` | Inbox, pagination `cursor`. Filter: `category` (`request`, `reminder`, `broadcast`, `system`), `unread`, `archived` | `notifikasi.lihat_sendiri` |
| GET | `/notifications/unread-count` | Jumlah belum dibaca (untuk badge) | `notifikasi.lihat_sendiri` |
| GET | `/notifications/sync` | Notifikasi yang terlewat setelah koneksi putus (`after_id`) (AC-42) | `notifikasi.lihat_sendiri` |
| GET | `/notifications/{id}` | Detail. `deep_link` dicek ulang. Hak sudah dicabut → `403`/`404` (AC-43) | `notifikasi.lihat_sendiri` |
| POST | `/notifications/{id}/read` | Tandai terbaca | `notifikasi.lihat_sendiri` |
| POST | `/notifications/read-all` | Tandai semua terbaca (opsi `category`) | `notifikasi.lihat_sendiri` |
| POST | `/notifications/{id}/archive` | Arsipkan | `notifikasi.lihat_sendiri` |
| GET, PUT | `/notifications/preferences` | Preferensi jenis dan kanal. Notifikasi wajib (keamanan, kebijakan) tidak bisa dimatikan (`422 MANDATORY_NOTIFICATION`) | `notifikasi.lihat_sendiri` |
| GET | `/push/vapid-public-key` | Kunci publik VAPID untuk Web Push | Sesi |
| POST | `/push/subscriptions` | Daftarkan perangkat (`endpoint`, `p256dh`, `auth`) | Sesi |
| GET | `/push/subscriptions` | Perangkat terdaftar | Sesi |
| DELETE | `/push/subscriptions/{id}` | Hapus perangkat | Sesi |
| POST | `/push/test` | Kirim notifikasi uji ke perangkat ini | Sesi |

### 13.2 Admin: aturan, template, dan log

| Method | Path | Fungsi | Hak |
| --- | --- | --- | --- |
| GET | `/admin/notification-event-types` | Katalog event dan reminder yang bisa diatur | `notifikasi.kelola` |
| GET | `/admin/notification-rules` | Semua aturan tenant | `notifikasi.kelola` |
| GET, PUT | `/admin/notification-rules/{eventType}` | Baca atau ganti aturan: aktif, offset, pengulangan, jeda, batas ulang, jam tenang, penerima ringkasan | `notifikasi.kelola` · A |
| GET | `/admin/notification-templates` | Template judul dan isi | `notifikasi.kelola` |
| PUT | `/admin/notification-templates/{id}` | Ubah template | `notifikasi.kelola` · A |
| POST | `/admin/notification-templates/{id}/preview` | Pratinjau dengan data contoh | `notifikasi.kelola` |
| GET | `/admin/notification-deliveries` | Log pengiriman (filter kanal, status, tanggal). Terkirim ≠ dibaca | `notifikasi.kelola` |
| POST | `/admin/notification-deliveries/{id}/retry` | Kirim ulang yang gagal (aman diulang) | `notifikasi.kelola` · A |
| GET | `/admin/notification-stats` | Agregat terkirim, gagal, belum dibaca | `notifikasi.kelola` |
| GET | `/admin/reminders/summary` | Ringkasan untuk kartu atasan: jumlah belum clock-in/out dalam scope | `absensi.monitor_bawahan` · S |

### 13.3 Broadcast / pengumuman

| Method | Path | Fungsi | Hak |
| --- | --- | --- | --- |
| GET | `/admin/broadcasts` | Daftar broadcast (filter status) | `broadcast.lihat` |
| POST | `/admin/broadcasts` | Buat **draft**: `title`, `body`, `targets[]`, `combine_mode` (`and`/`or`) | `broadcast.buat` · A |
| GET | `/admin/broadcasts/{id}` | Detail | `broadcast.lihat` |
| PATCH | `/admin/broadcasts/{id}` | Ubah draft atau yang terjadwal | `broadcast.ubah` · A |
| DELETE | `/admin/broadcasts/{id}` | Hapus draft | `broadcast.ubah` · A |
| POST | `/admin/broadcasts/audience-preview` | Pratinjau penerima dari target yang belum disimpan | `broadcast.buat` · S |
| POST | `/admin/broadcasts/{id}/audience-preview` | Pratinjau dari draft tersimpan | `broadcast.buat` · S |
| POST | `/admin/broadcasts/{id}/attachments` | Lampirkan berkas (`file_id`) | `broadcast.ubah` · A |
| POST | `/admin/broadcasts/{id}/send` | Kirim sekarang. Audience dihitung saat eksekusi dan di-snapshot | `broadcast.kirim` · I · A · J |
| POST | `/admin/broadcasts/{id}/schedule` | Jadwalkan (`send_at`). Audience dihitung **saat eksekusi** (AC-38) | `broadcast.kirim` · A |
| POST | `/admin/broadcasts/{id}/cancel` | Batalkan yang terjadwal atau hentikan pengiriman yang sedang berjalan | `broadcast.kirim` · A |
| GET | `/admin/broadcasts/{id}/recipients` | Daftar penerima dan status baca (berpaginasi) | `broadcast.lihat_penerima` · S |
| GET | `/admin/broadcasts/{id}/stats` | Terkirim, dibaca, gagal | `broadcast.lihat` |
| GET | `/ess/announcements` | Pengumuman yang saya terima (`cursor`) | (dasar) |
| GET | `/ess/announcements/{id}` | Detail. Membukanya menandai terbaca | (dasar) |

**Aturan broadcast:**

- **Penerima final = target ∩ scope pengirim ∩ akun yang berhak menerima.** Dihitung backend, bukan klien.
- Respons `audience-preview`: `recipients_count`, `excluded_by_scope_count`, rincian per target, dan daftar contoh. Daftar lengkap hanya bila `broadcast.lihat_penerima`.
- Target `all` butuh `broadcast.target_semua`, selain itu `403 TARGET_ALL_FORBIDDEN`.
- Satu pengguna hanya mendapat satu salinan (`UNIQUE broadcast_id, membership_id`) (AC-39). Audience kosong → `422 AUDIENCE_EMPTY`.
- `send` bersifat async dan bertahap untuk volume besar. Progres lewat event `broadcast.progress` dan `GET /jobs/{id}`.
- Tidak ada endpoint broadcast lintas tenant. Pesan platform ada di Bagian 15.

### 13.4 WebSocket

`GET /realtime/ws` (upgrade). Autentikasi dengan cookie sesi. Server memeriksa `Origin` dan menolak asal yang tidak sah. Tenant dan user ditentukan dari sesi. **Klien tidak memilih channel**: server mendaftarkan koneksi ke channel milik user dan hanya event yang lolos pengecekan tenant + permission + scope.

| Pesan klien | Fungsi |
| --- | --- |
| `hello` | Perkenalan. Memuat `client_version` dan `last_notification_id` |
| `resume` | Lanjutkan setelah putus. Memuat `after_id` |
| `ping` | Detak jantung |
| `ack` | Konfirmasi event diterima (`event_id`) |

| Event server | Penerima | Isi ringkas | Tindakan klien |
| --- | --- | --- | --- |
| `notification.created` | Pemilik | `id`, `category`, `title`, `deep_link`, `unread_count` | Tambah ke inbox, naikkan badge, toast bila aktif |
| `notification.updated` | Pemilik | `id`, `read_at` / `archived` / `obsolete` | Perbarui item (mis. tindakan approval yang sudah tidak relevan) |
| `notification.unread_count` | Pemilik | `count` | Perbarui badge |
| `approval.task.created`, `approval.task.closed` | Approver | `request_id`, `request_type` | Muat ulang inbox dan `summary` |
| `approval.request.updated` | Pengaju | `request_id`, `status` | Perbarui status pengajuan |
| `attendance.event.created` | Pengawas yang scope-nya mencakup | referensi karyawan, status, waktu | Perbarui tabel Kehadiran Hari Ini (hanya baris dalam scope) |
| `reminder.summary.updated` | Atasan, HR dalam scope | `not_yet_count` | Perbarui kartu ringkasan |
| `broadcast.progress` | Pembuat | `id`, `sent`, `total`, `failed` | Perbarui progres |
| `job.updated` | Pemilik job | `job_id`, `status`, `progress` | Perbarui status job |
| `access.changed` | User terkait | `access_version` | Muat ulang `GET /me/access` |
| `session.revoked` | User terkait | (kosong) | Tutup dan arahkan ke login |
| `tenant.status.changed` | User tenant | `status` | Tampilkan banner status |

**Aturan WebSocket:**

- Payload **tidak memuat** NIK, rekening, nominal gaji, atau isi dokumen sensitif.
- Detak jantung `ping` sekitar tiap 25 detik. Klien menyambung ulang dengan *exponential backoff* + jitter, lalu memanggil `GET /notifications/sync?after_id=...` dan memuat ulang penghitung.
- Kode penutupan: `4401` sesi dicabut atau tidak sah (jangan sambung ulang, arahkan login), `4408` hak akses berubah (muat `/me/access` lalu sambung ulang), `4429` terlalu banyak koneksi.
- Sesi dicabut, akun dinonaktifkan, atau tenant ditangguhkan memutus koneksi (AC-43, AC-49).
- WebSocket **tidak menggantikan REST**. Kehilangan koneksi tidak boleh menghilangkan notifikasi.

## 14. Dashboard, laporan, impor/ekspor, audit, dan pengaturan

### 14.1 Dashboard

Semua angka berasal dari API dan tunduk pada permission serta scope. Data payroll tidak muncul tanpa `dashboard.lihat_keuangan`.

| Method | Path | Fungsi | Hak |
| --- | --- | --- | --- |
| GET | `/admin/dashboard/summary` | KPI: karyawan aktif, hadir hari ini, terlambat, pengajuan menunggu, kontrak mendekati akhir | `dashboard.lihat` · S |
| GET | `/admin/dashboard/attendance-trend` | Tren kehadiran (`from`, `to`) | `dashboard.lihat` · S |
| GET | `/admin/dashboard/headcount` | Distribusi karyawan per divisi, kantor, status | `dashboard.lihat` · S |
| GET | `/admin/dashboard/expiring` | Kontrak dan dokumen yang akan kedaluwarsa | `dashboard.lihat` · S |
| GET | `/admin/dashboard/payroll-cost` | Biaya payroll per periode | `dashboard.lihat_keuangan` · S |
| GET | `/ess/home` | Ringkasan Beranda ESS: status kehadiran, jadwal hari ini, saldo cuti, pengajuan terakhir, pengumuman, reminder | (dasar) |

### 14.2 Laporan

| Method | Path | Fungsi | Hak |
| --- | --- | --- | --- |
| GET | `/admin/reports` | Katalog laporan yang boleh diakses user | `laporan.lihat` |
| GET | `/admin/reports/{key}` | Jalankan laporan dengan filter, berpaginasi. Hasil mengikuti scope | `laporan.lihat` · S |
| POST | `/admin/reports/{key}/export` | Ekspor hasil laporan | `laporan.ekspor` · S · A · J |

Contoh `key`: `headcount`, `turnover`, `attendance-summary`, `lateness`, `leave-usage`, `overtime`, `loans`, `reimbursements`, `payroll-summary`. Laporan keuangan juga mensyaratkan `penggajian.lihat_sensitif`.

### 14.3 Ekspor dan impor (job generik)

Ekspor dan impor besar **selalu async**. Semua modul memakai endpoint ini dengan `type` yang berbeda.

**Ekspor**

| Method | Path | Fungsi | Hak |
| --- | --- | --- | --- |
| POST | `/admin/exports` | Buat ekspor. Body: `type`, `filters`, `columns`, `format` (`csv` atau `xlsx`). Respons `202` + `job_id` | `{modul}.ekspor` · S · A · J |
| GET | `/jobs` | Job milik saya (ekspor, impor, hitung ulang) | Sesi |
| GET | `/jobs/{id}` | Status, progres, error | Pemilik job |
| GET | `/jobs/{id}/download` | Unduh hasil. `302` ke URL sementara | Pemilik job · A |
| DELETE | `/jobs/{id}` | Batalkan job yang berjalan atau hapus hasilnya | Pemilik job |

**Aturan ekspor** (AC-55): hasil mengikuti scope dan filter yang berlaku saat job dibuat (`scope_snapshot` disimpan). Kolom sensitif disamarkan tanpa `lihat_sensitif`. CSV memakai UTF-8 BOM dan escape RFC 4180. Selalu masuk audit. Nilai `type`: `employees`, `contracts`, `attendance`, `work_schedules`, `leave_requests`, `overtime`, `loans`, `reimbursements`, `audit_logs`, `regions`, `offices`, `divisions`, `departments`, `positions`, `payroll_bank` (lewat endpoint payroll).

**Impor**

| Method | Path | Fungsi | Hak |
| --- | --- | --- | --- |
| GET | `/admin/imports/templates/{type}` | Unduh template CSV | `{modul}.impor` |
| POST | `/admin/imports` | Buat impor. Body: `type`, `file_id`, `mapping` (kolom CSV → field), `dry_run`. Langkah **pratinjau** | `{modul}.impor` · S · A · J |
| GET | `/admin/imports/{id}` | Status dan ringkasan (total, valid, error) | Pemilik |
| GET | `/admin/imports/{id}/preview` | Baris teratas hasil pemetaan | Pemilik |
| GET | `/admin/imports/{id}/errors` | Laporan error per baris (nomor baris, field, pesan) | Pemilik |
| POST | `/admin/imports/{id}/commit` | Jalankan impor untuk baris yang valid | `{modul}.impor` · S · I · A · J |

**Aturan impor** (AC-54): tiap baris dicek terhadap **scope pengimpor**. Baris di luar scope ditolak dan masuk laporan error, baris valid tetap diproses. Duplikasi dicegah (mis. NIP, NIK). Kuota paket dicek (`QUOTA_EXCEEDED`). Nilai `type`: `employees`, `contracts`, `work_schedules`, `holidays`, `regions`, `offices`, `divisions`, `departments`, `positions`.

### 14.4 Audit dan keamanan

| Method | Path | Fungsi | Hak |
| --- | --- | --- | --- |
| GET | `/admin/audit-logs` | Audit (`cursor`). Filter: pelaku, aksi, entitas, tanggal | `audit.lihat` |
| GET | `/admin/audit-logs/{id}` | Detail perubahan (before/after, tanpa rahasia) | `audit.lihat` |
| GET | `/admin/login-events` | Percobaan login, login perangkat baru, kegagalan | `audit.lihat` |
| GET | `/admin/security/overview` | Ringkasan: sesi aktif, login gagal, perubahan hak akses terbaru | `audit.lihat` |
| POST | `/admin/audit-logs/export` | Ekspor audit | `audit.ekspor` · R · A · J |

### 14.5 Pengaturan tenant dan langganan

| Method | Path | Fungsi | Hak |
| --- | --- | --- | --- |
| GET | `/admin/company-profile` | Identitas, alamat, NPWP perusahaan | `pengaturan.lihat` |
| PATCH | `/admin/company-profile` | Ubah profil | `pengaturan.kelola` · A |
| POST | `/admin/company-profile/logo` | Ganti logo (`file_id`) | `pengaturan.kelola` · A |
| GET | `/admin/settings/{group}` | Baca pengaturan. `group`: `general`, `hr-policy`, `session`, `theme`, `notifications` | `pengaturan.lihat` |
| PUT | `/admin/settings/{group}` | Ganti pengaturan. Grup `session` (`remember_allowed_admin`, `idle_timeout_min`) wajib re-auth | `pengaturan.kelola` · A (`session`: R) |
| GET | `/admin/subscription` | Paket aktif, entitlement, batas | `pengaturan.lihat` |
| GET | `/admin/subscription/usage` | Pemakaian: karyawan, pengguna, kantor, storage | `pengaturan.lihat` |
| GET, POST | `/admin/support/tickets` | Tiket ke tim digaji.id | `pengaturan.kelola` |
| GET | `/admin/support/tickets/{id}` | Detail tiket | `pengaturan.kelola` |
| POST | `/admin/support/tickets/{id}/messages` | Balas tiket | `pengaturan.kelola` |
| GET, POST | `/admin/api-keys` | Kunci API \[BERTAHAP\] | `pengaturan.kelola` · R · A |
| DELETE | `/admin/api-keys/{id}` | Cabut kunci API | `pengaturan.kelola` · R · A |
| GET, POST | `/admin/webhooks` | Webhook keluar \[BERTAHAP\] | `pengaturan.kelola` · A |
| PATCH, DELETE | `/admin/webhooks/{id}` | Ubah atau hapus | `pengaturan.kelola` · A |
| GET | `/admin/webhooks/{id}/deliveries` | Riwayat pengiriman webhook | `pengaturan.kelola` |

### 14.6 Pencarian global (Command Palette)

| Method | Path | Fungsi | Hak |
| --- | --- | --- | --- |
| GET | `/search` | Pencarian untuk `Ctrl + K` (`q`, `types`). Hasil dikelompokkan: **menu** (dari hak akses), **karyawan** (dalam scope), **halaman**. Tiap jenis hasil hanya muncul bila user berhak | Sesi + hak per jenis · S |

## 15. Platform / Superadmin (`/platform`)

Platform memakai role dan permission terpisah dari tenant. Login memakai `POST /auth/login` yang sama, dan `available_panels` memuat `platform` untuk staf platform.

**Permission platform** (matriks role ada di dokumen Matriks Menu, Bagian 5): `platform_tenant.lihat/kelola`, `platform_paket.lihat/kelola`, `platform_billing.lihat/kelola`, `platform_staff.kelola`, `platform_tiket.lihat/kelola`, `platform_analitik.lihat`, `platform_pengaturan.kelola`, `platform_support.akses`, `platform_pengumuman.kirim`, `platform_audit.lihat`.

**Pengamanan tambahan \[USULAN\]:** Simpan Sesi dinonaktifkan untuk akun platform, autentikasi dua faktor wajib, dan semua aksi bertanda **R** meminta re-autentikasi.

### 15.1 Dashboard, tenant, dan dukungan

| Method | Path | Fungsi | Hak |
| --- | --- | --- | --- |
| GET | `/platform/dashboard` | KPI SaaS: tenant per status, MRR/ARR, invoice jatuh tempo, kesehatan layanan | `platform_analitik.lihat` |
| GET | `/platform/tenants` | Daftar tenant (filter status, paket, `q`) | `platform_tenant.lihat` |
| POST | `/platform/tenants` | Daftarkan tenant baru + undangan owner | `platform_tenant.kelola` · A |
| GET | `/platform/tenants/{id}` | Detail tenant | `platform_tenant.lihat` |
| PATCH | `/platform/tenants/{id}` | Ubah data tenant | `platform_tenant.kelola` · A |
| POST | `/platform/tenants/{id}/verify` | Verifikasi tenant | `platform_tenant.kelola` · A |
| POST | `/platform/tenants/{id}/suspend` | Tangguhkan (alasan wajib). Memutus sesi sesuai kebijakan \[T-06\] | `platform_tenant.kelola` · R · A |
| POST | `/platform/tenants/{id}/reactivate` | Aktifkan kembali | `platform_tenant.kelola` · R · A |
| POST | `/platform/tenants/{id}/close` | Tutup layanan tenant | `platform_tenant.kelola` · R · A |
| GET | `/platform/tenants/{id}/timeline` | Riwayat status tenant | `platform_tenant.lihat` |
| GET | `/platform/tenants/{id}/usage` | Pemakaian dan kuota | `platform_tenant.lihat` |
| PUT | `/platform/tenants/{id}/branding` | Logo dan tema tenant | `platform_tenant.kelola` · A |
| POST | `/platform/tenants/{id}/support-sessions` | Buka **support session** (`reason` wajib, durasi terbatas) | `platform_support.akses` · R · A |
| GET | `/platform/support-sessions` | Daftar support session aktif dan riwayat | `platform_support.akses` |
| DELETE | `/platform/support-sessions/{id}` | Akhiri support session | `platform_support.akses` · A |

**Support session:** akses staf platform ke data tenant hanya lewat sesi ini. Permintaan membawa header `X-Support-Session`. Dievaluasi sebagai peran dukungan **baca-saja** yang tidak mencakup payroll dan dokumen sensitif \[USULAN\]. Semua aksi diaudit dengan `support_session_id`, UI menampilkan indikator jelas. Admin platform **tidak** otomatis boleh membaca data gaji tenant.

### 15.2 Paket, langganan, dan billing

| Method | Path | Fungsi | Hak |
| --- | --- | --- | --- |
| GET | `/platform/plans` | Daftar paket | `platform_paket.lihat` |
| POST | `/platform/plans` | Buat paket | `platform_paket.kelola` · A |
| GET, PATCH | `/platform/plans/{id}` | Detail atau ubah | `platform_paket.lihat` / `kelola` · A |
| DELETE | `/platform/plans/{id}` | Nonaktifkan paket | `platform_paket.kelola` · A |
| PUT | `/platform/plans/{id}/entitlements` | Ganti entitlement dan batas paket | `platform_paket.kelola` · A |
| GET | `/platform/subscriptions` | Daftar langganan | `platform_billing.lihat` |
| POST | `/platform/tenants/{id}/subscription` | Tetapkan langganan | `platform_billing.kelola` · R · A |
| POST | `/platform/subscriptions/{id}/change-plan` | Upgrade atau downgrade (dengan tanggal efektif) | `platform_billing.kelola` · R · A |
| POST | `/platform/subscriptions/{id}/renew` | Perpanjang | `platform_billing.kelola` · A |
| POST | `/platform/subscriptions/{id}/cancel` | Batalkan | `platform_billing.kelola` · R · A |
| GET | `/platform/invoices` | Daftar tagihan | `platform_billing.lihat` |
| GET | `/platform/invoices/{id}` | Detail tagihan | `platform_billing.lihat` |
| POST | `/platform/invoices/{id}/send` | Kirim ulang tagihan | `platform_billing.kelola` · A |
| POST | `/platform/invoices/{id}/void` | Batalkan tagihan | `platform_billing.kelola` · R · A |
| GET | `/platform/payments` | Daftar transaksi | `platform_billing.lihat` |
| GET | `/platform/payments/{id}` | Detail transaksi | `platform_billing.lihat` |
| POST | `/platform/payments/{id}/refund` | Refund | `platform_billing.kelola` · R · I · A |

### 15.3 Staf, tiket, analitik, pengaturan

| Method | Path | Fungsi | Hak |
| --- | --- | --- | --- |
| GET, POST | `/platform/staff` | Staf platform | `platform_staff.kelola` · R · A |
| GET, PATCH, DELETE | `/platform/staff/{id}` | Detail, ubah role, nonaktifkan | `platform_staff.kelola` · R · A |
| GET, POST | `/platform/tickets` | Daftar dan buat tiket | `platform_tiket.lihat` / `kelola` |
| GET, PATCH | `/platform/tickets/{id}` | Detail atau ubah status | `platform_tiket.lihat` / `kelola` |
| POST | `/platform/tickets/{id}/messages` | Balas tiket | `platform_tiket.kelola` |
| POST | `/platform/tickets/{id}/assign` | Tetapkan penanggung jawab | `platform_tiket.kelola` |
| POST | `/platform/tickets/{id}/close` | Tutup tiket | `platform_tiket.kelola` |
| GET | `/platform/analytics/{metric}` | `metric`: `growth`, `conversion`, `churn`, `revenue`, `module-usage`. Hanya agregat | `platform_analitik.lihat` |
| GET | `/platform/settings` | Daftar pengaturan platform | `platform_pengaturan.kelola` |
| GET, PUT | `/platform/settings/{key}` | Brand, SMTP, kanal notifikasi, integrasi, backup | `platform_pengaturan.kelola` · R · A |
| POST | `/platform/settings/smtp/test` | Uji pengiriman email | `platform_pengaturan.kelola` · A |
| GET, POST | `/platform/announcements` | Pesan platform ke admin tenant (kategori `system`). **Terpisah** dari broadcast HR tenant | `platform_pengumuman.kirim` · A |
| GET | `/platform/audit-logs` | Audit aksi staf platform dan support session | `platform_audit.lihat` |

Analitik notifikasi bagi platform hanya agregat, **tidak memuat isi pesan tenant**.

## 16. Webhook dan kesehatan layanan

| Method | Path | Fungsi | Pengamanan |
| --- | --- | --- | --- |
| POST | `/webhooks/payments/{gateway}` | Notifikasi pembayaran dari payment gateway | Tanda tangan diverifikasi. Idempoten lewat `(gateway, event_id)` di `payment_webhook_events`. Respons `200` cepat, proses lanjutan async |
| GET | `/healthz` | Liveness | Publik |
| GET | `/readyz` | Readiness (database, Redis, storage) | Internal / jaringan terbatas |
| GET | `/version` | Versi API dan build | Publik |

**CORS:** hanya origin frontend yang terdaftar, dengan `credentials` diizinkan. Tidak memakai wildcard.

## 17. Contoh request dan response

Contoh ilustratif. Skema final ditetapkan di file OpenAPI.

### 17.1 Login dan hak akses

```json
// POST /auth/login
{ "email": "iyan@contoh.co.id", "password": "********", "remember": true }

// 200
{ "data": {
    "user": { "id": "01JC8Z4K9Q3X7M2V5N6B8D0F1G", "full_name": "Iyan" },
    "memberships": [ { "id": "01JC8Z5A1B2C3D4E5F6G7H8J9K", "tenant_name": "PT Contoh Abadi" } ],
    "active_membership": "01JC8Z5A1B2C3D4E5F6G7H8J9K",
    "available_panels": ["ess", "admin"],
    "remember_effective": true,
    "csrf_token": "b1f0..."
} }

// GET /me/access  (Iyan = KARYAWAN + SPV, cuti.approve ditolak khusus user)
{ "data": {
    "access_version": 12,
    "panels": ["ess", "admin"],
    "permissions": ["panel.admin", "absensi.lihat", "absensi.monitor_bawahan", "cuti.lihat"],
    "scopes": { "absensi": [ { "kind": "self" }, { "kind": "direct_reports" } ] },
    "entitlements": ["core_hr", "attendance", "leave"]
} }
```

### 17.2 Presensi

```json
// POST /ess/attendance/clock-in
{ "client_event_id": "01JC8Z4K9Q3X7M2V5N6B8D0F1G",
  "lat": -6.2000, "lng": 106.8166, "accuracy_m": 12.5,
  "device_time": "2026-11-05T01:00:02.000Z",
  "selfie_file_id": "01JC8Z6M7N8P9Q0R1S2T3V4W5X" }

// 201
{ "data": { "event_id": "01JC8Z7Y...", "event_type": "clock_in",
    "occurred_at": "2026-11-05T01:00:03.120Z",
    "timing_status": "on_time", "location_status": "inside",
    "office": { "id": "01JC8Z...", "name": "Kantor Pusat" } } }

// 422 (di luar area)
{ "error": { "code": "LOCATION_OUT_OF_AREA",
    "message": "Anda berada di luar area presensi.",
    "request_id": "01JC8Z..." } }
```

### 17.3 Keputusan approval

```json
// POST /admin/approvals/01JD1A2B3C4D5E6F7G8H9J0K1L/actions
{ "action": "approve", "level_no": 1, "version": 3,
  "data": { "approved_minutes": 120 } }

// 200 (lanjut ke tahap 2)
{ "data": { "request_id": "01JD1A2B3C4D5E6F7G8H9J0K1L",
    "status": "pending", "current_level": 2, "version": 4 } }

// 403 (menyetujui pengajuan sendiri)
{ "error": { "code": "SELF_APPROVAL_FORBIDDEN",
    "message": "Anda tidak dapat menyetujui pengajuan Anda sendiri." } }
```

### 17.4 Pratinjau penerima broadcast

```json
// POST /admin/broadcasts/audience-preview
{ "targets": [ { "type": "division", "ids": ["01JE..."] },
                { "type": "office",   "ids": ["01JF..."] } ],
  "combine_mode": "and", "manual_user_ids": [] }

// 200
{ "data": { "recipients_count": 48, "excluded_by_scope_count": 6,
    "breakdown": [ { "type": "division", "matched": 120 },
                   { "type": "office",   "matched": 75 } ],
    "list_visible": false } }
```

### 17.5 Ekspor async

```json
// POST /admin/exports   (header Idempotency-Key opsional untuk ekspor)
{ "type": "employees", "filters": { "status": "active" }, "format": "csv" }

// 202
{ "data": { "job_id": "01JG3H4J5K6M7N8P9Q0R1S2T3V", "status": "queued" } }

// GET /jobs/01JG3H4J5K6M7N8P9Q0R1S2T3V
{ "data": { "id": "01JG3H4J5K6M7N8P9Q0R1S2T3V", "type": "export",
    "status": "running", "progress": 42 } }
```

### 17.6 Frame WebSocket

```json
// Klien -> server setelah tersambung ulang
{ "type": "resume", "after_id": "01JC8Z7Y0A1B2C3D4E5F6G7H8J" }

// Server -> klien
{ "type": "event", "id": "evt_01JC8Z8K...", "event": "notification.created",
  "ts": "2026-11-05T01:00:03.120Z",
  "data": { "id": "01JC8Z8K...", "category": "request",
            "title": "Pengajuan cuti baru",
            "deep_link": "/admin/approvals/01JD1A2B3C4D5E6F7G8H9J0K1L",
            "unread_count": 4 } }
```

## 18. Katalog kode error

Kode stabil dalam bahasa Inggris. Pesan untuk pengguna berbahasa Indonesia.

| Kode | HTTP | Arti |
| --- | --- | --- |
| `AUTH_INVALID_CREDENTIALS` | 401 | Email atau kata sandi salah |
| `AUTH_SESSION_INVALID` | 401 | Sesi tidak ada, dicabut, atau kedaluwarsa (mode Simpan Sesi OFF) |
| `AUTH_ACCOUNT_DISABLED` | 403 | Akun atau membership nonaktif |
| `REAUTH_REQUIRED` | 403 | Aksi berisiko butuh `POST /auth/reauth` |
| `CSRF_INVALID` | 403 | Header CSRF hilang atau salah |
| `PERMISSION_DENIED` | 403 | Tidak punya permission |
| `ENTITLEMENT_REQUIRED` | 403 | Fitur di luar paket langganan |
| `TENANT_SUSPENDED` | 403 | Tenant ditangguhkan. Perilaku mengikuti T-06 |
| `DELEGATION_EXCEEDED` | 403 | Memberi hak melebihi kewenangan |
| `SELF_ACCESS_CHANGE_FORBIDDEN` | 403 | Mengubah hak akses diri sendiri |
| `SELF_APPROVAL_FORBIDDEN` | 403 | Menyetujui pengajuan sendiri |
| `NOT_ASSIGNED` | 403 | Bukan approver yang ditugaskan pada tahap itu |
| `TARGET_ALL_FORBIDDEN` | 403 | Broadcast ke seluruh tenant tanpa `broadcast.target_semua` |
| `NOT_FOUND` | 404 | Tidak ada, atau di luar scope |
| `IDEMPOTENCY_KEY_REQUIRED` | 400 | Aksi bertanda **I** tanpa `Idempotency-Key` |
| `IDEMPOTENCY_KEY_REUSED` | 422 | Kunci sama dengan isi berbeda |
| `VALIDATION_FAILED` | 422 | Validasi field gagal. Rincian di `details` |
| `QUOTA_EXCEEDED` | 422 | Melewati batas paket (karyawan, pengguna, kantor) |
| `STORAGE_QUOTA_EXCEEDED` | 422 | Kuota storage habis |
| `AUDIENCE_EMPTY` | 422 | Penerima broadcast kosong setelah dipotong scope |
| `MANDATORY_NOTIFICATION` | 422 | Notifikasi wajib tidak bisa dimatikan |
| `LOCATION_OUT_OF_AREA` | 422 | Presensi di luar polygon yang diizinkan |
| `LOCATION_ACCURACY_LOW` | 422 | Akurasi GPS di bawah ambang |
| `OUT_OF_WINDOW` | 422 | Di luar jendela clock-in/out |
| `SELFIE_REQUIRED` | 422 | Shift mewajibkan selfie |
| `NO_SHIFT_FOUND` | 422 | Tidak ada shift instance yang cocok |
| `ALREADY_CLOCKED_IN`, `ALREADY_CLOCKED_OUT` | 409 | Presensi sudah tercatat |
| `VERSION_CONFLICT` | 409 | `version` tidak cocok |
| `ALREADY_DECIDED` | 409 | Pengajuan sudah diputuskan |
| `DUPLICATE` | 409 | Data ganda (mis. NIP, NIK) |
| `DUPLICATE_SUSPECTED` | 409 | Klaim terindikasi duplikat. Konfirmasi dengan `confirm_duplicate` |
| `IN_USE` | 409 | Masih dipakai data lain |
| `INVALID_STATE` | 409 | Status resource tidak mengizinkan aksi |
| `PAYROLL_LOCKED` | 409 | Run payroll sudah final |
| `JOB_NOT_READY` | 409 | Hasil job belum tersedia |
| `RATE_LIMITED` | 429 | Terlalu banyak permintaan |
| `INTERNAL_ERROR` | 500 | Kesalahan server. Hanya `request_id` yang ditampilkan |

## 19. Ringkasan, fase, dan keputusan terbuka

### 19.1 Endpoint per fase

| Fase | Kelompok endpoint |
| --- | --- |
| **0. Fondasi** | Bagian 1–5 (auth, sesi, akses), `/me/access`, `/lookups`, `/files`, `/jobs`, `/admin/audit-logs`, `/admin/settings`, `/admin/company-profile`, Platform dasar (tenant, paket, staf), `/healthz` |
| **1. SDM inti** | Bagian 6–7 (organisasi, karyawan, kontrak, dokumen, direktori), `/notifications` + `/push`, broadcast (13.3), WebSocket (13.4), `/admin/imports`, `/admin/exports`, `/agenda`, mutasi dan terminasi (dengan approval, lihat D-06) |
| **2. Kehadiran** | Bagian 8, aturan dan template reminder (13.2), `/admin/reminders/summary`, event `attendance.*` |
| **3. Pengajuan** | Bagian 9–10 (cuti, lembur, kasbon, reimbursement, approval) |
| **4. Payroll** | Bagian 11 |
| **5. Lanjutan** | Integrasi (`api-keys`, `webhooks`), modul talenta, analitik lanjutan |

### 19.2 Batas laju awal \[USULAN\]

| Endpoint | Batas | Kunci |
| --- | --- | --- |
| `POST /auth/login` | 5 per menit | Akun dan IP |
| `POST /auth/password/forgot` | 3 per jam | Email dan IP |
| `POST /ess/attendance/clock-in`, `clock-out` | 10 per menit | Pengguna |
| `POST /admin/exports`, `/admin/imports` | 5 per jam | Pengguna |
| `POST /admin/broadcasts/{id}/send` | 10 per jam | Tenant |
| Lainnya | 120 per menit | Pengguna |

Angka final \[TERBUKA A-07\], ditetapkan setelah uji beban.

### 19.3 Dampak ke dokumen lain

API ini membutuhkan tambahan yang belum ada di dokumen turunan lain:

- **Registry permission:** modul `disiplin`, `terminasi`, `aset`, `agenda`, `direktori`, aksi `kasbon.kelola`, serta permission `platform_*`.
- **Database:** tabel `jobs` generik (menyatukan `import_jobs`, `export_jobs`, dan job hitung), `idempotency_keys` (kunci, hash isi, respons tersimpan, kedaluwarsa), `profile_change_requests` (koreksi data pribadi ESS), dan penyimpanan 2FA untuk akun platform.
- **Approval:** `request_type` ditambah `profile_change`.
- **PRD:** endpoint ini mengasumsikan pemisahan `/ess` dan `/admin`, aturan 404 untuk data di luar scope, dan support session baca-saja. Tiga hal itu perlu ditulis eksplisit di PRD v2.3.

### 19.4 Keputusan terbuka untuk API

| ID | Topik | Usulan | Dibutuhkan sebelum |
| --- | --- | --- | --- |
| A-01 | OpenAPI dibuat dari kode atau **spec-first** | Spec-first, lalu generate klien TypeScript dan server stub | Fase 0 |
| A-02 | Data di luar scope: `404` atau `403` | `404`, supaya keberadaan data tidak bocor | Fase 0 |
| A-03 | Paginasi offset atau cursor untuk daftar besar | Offset untuk master, cursor untuk log | Fase 0 |
| A-04 | Impor dan ekspor generik atau per modul | Generik (`/admin/imports`, `/admin/exports`) | Fase 1 |
| A-05 | Bentuk aksi massal (arsip, setuju massal) | `POST .../bulk-*` dengan hasil per item | Fase 1 |
| A-06 | Kontrol konkurensi: field `version` atau `ETag`/`If-Match` | Field `version` | Fase 0 |
| A-07 | Angka batas laju | Lihat 19.2, finalkan setelah uji beban | Sebelum produksi |
| A-08 | Autentikasi dua faktor untuk staf platform (dan opsional admin tenant) | Wajib untuk platform | Fase 0 |
| A-09 | Definisi role baca-saja untuk support session | Tanpa payroll dan dokumen sensitif | Fase 1 |
| A-10 | Pesan error: hanya Indonesia atau siap multi-bahasa | Indonesia, kode stabil agar siap i18n | Fase 0 |
