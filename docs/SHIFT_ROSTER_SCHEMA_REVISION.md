# Revisi rancangan shift, pola jadwal, dan roster

**Keputusan Product Owner, 5 November 2026.** Dokumen ini memperinci fase Jadwal dan Kehadiran pada `DATABASE_SCHEMA_MIGRATION_DRAFT.md`. Struktur SQL sudah diterapkan melalui migrasi `00014`, `00022`, dan `00023`; fungsi pola serta materialisasi waktu ada di `backend/internal/schedule`. API presensi dan integrasi endpoint face recognition belum diaktifkan.

## 1. Aturan yang disepakati

- Shift mempunyai jam kerja terjadwal (`start_time`, `end_time`) **dan** empat batas jendela presensi: mulai/akhir boleh masuk, mulai/akhir boleh pulang. Keduanya berbeda dari `late_tolerance_min` untuk menentukan terlambat.
- `require_selfie` sudah ada pada desain awal. Tambahkan `require_face_recognition` sebagai aturan shift untuk integrasi tingkat lanjut. Keduanya dapat dipilih terpisah; bila face recognition aktif, foto verifikasi dapat diproses sementara tanpa wajib menyimpan selfie, sesuai kebijakan yang nanti dipilih.
- Shift memiliki `is_pinned`, `pin_order`, dan `color_hex`. Shift pin tampil lebih dulu menurut `pin_order` terkecil. Warna shift dipakai pada pilihan shift dan sel roster; tidak memengaruhi hitungan jam kerja.
- Ada template pola **mingguan** (Senin–Minggu) dan **bulanan** (Minggu 1–5 × Senin–Minggu). Template pola juga memiliki `is_pinned` dan `pin_order`.
- **Satu karyawan hanya boleh memiliki satu shift pada satu `work_date`**, baik hasil pola maupun pengisian manual.
- Pada kalender yang memiliki baris minggu ke-6, **ulang pola minggu ke-5**. Minggu ke-1 adalah baris Senin–Minggu yang memuat tanggal 1, meskipun sebagian hari berada di bulan sebelumnya. Hanya tanggal yang termasuk bulan target dibuatkan roster.

Contoh: Agustus 2026 memiliki enam baris kalender Senin–Minggu. **Senin, 31 Agustus** berada pada baris ke-6, sehingga memakai slot **W5–Senin** dari pola bulanan.

## 2. Tabel dan field yang direvisi

| Tabel | Field inti baru/yang dipertegas | Relasi dan constraint |
| --- | --- | --- |
| `shift_templates` | `tenant_id`, `id`, `code`, `name`, `start_time`, `end_time`, `crosses_midnight`, `clock_in_window_start_time`, `clock_in_window_start_day_offset`, `clock_in_window_end_time`, `clock_in_window_end_day_offset`, `clock_out_window_start_time`, `clock_out_window_start_day_offset`, `clock_out_window_end_time`, `clock_out_window_end_day_offset`, `late_tolerance_min`, `break_minutes`, `target_work_minutes`, `flex_time`, `flex_location`, `require_location`, `require_selfie`, `require_face_recognition`, `is_pinned`, `pin_order`, `color_hex`, `is_active`, timestamp audit | `code` unik per tenant. Waktu lokal mengikuti zona kantor; offset hari menentukan -1/0/1/2 terhadap `work_date` untuk shift lintas hari. `pin_order` hanya terisi bila pin aktif dan unik per tenant. `color_hex` format `#RRGGBB`. |
| `schedule_patterns` **(baru)** | `tenant_id`, `id`, `code`, `name`, `pattern_type` (`weekly`/`monthly`), `version`, `is_pinned`, `pin_order`, `is_active`, timestamp audit | `code` unik per tenant. Urutan pin terpisah per `pattern_type`; perubahan slot menaikkan `version`. |
| `schedule_pattern_slots` **(baru)** | `tenant_id`, `id`, `pattern_id`, `week_no`, `weekday`, `day_type` (`shift`/`off`), `shift_template_id` nullable | FK komposit ke pola dan shift. `weekday` 1=Senin sampai 7=Minggu; mingguan hanya `week_no=1`, bulanan `week_no=1..5`. Unik `(tenant_id,pattern_id,week_no,weekday)`. Bila `day_type=off`, `shift_template_id` kosong. |
| `work_schedules` **(roster)** | `tenant_id`, `id`, `employee_id`, `work_date`, `shift_template_id` nullable untuk hari off, `day_type`, `office_id`, `source_pattern_id` nullable, `source_pattern_version` nullable, `status` (`draft`/`published`/`cancelled`), `created_by`, `published_at`, timestamp audit | FK komposit ke karyawan/shift/kantor/pola. Berisi hasil penerapan template atau edit manual per tanggal. UNIQUE `(tenant_id,employee_id,work_date)`; perubahan baris terbit harus diaudit dan menyelaraskan instance. |
| `shift_instances` | Field desain awal ditambah `clock_in_window_start_at`, `clock_in_window_end_at`, `clock_out_window_start_at`, `clock_out_window_end_at` dalam UTC serta `rule_snapshot` | Dibuat dari roster terbit. UNIQUE `(tenant_id,employee_id,work_date)`. Snapshot memuat jendela, toleransi, kebutuhan selfie/face/lokasi, warna, dan versi pola yang dipakai. Perubahan master tidak mengubah instance lama. |
| `attendance_events` | Field desain awal ditambah `face_verification_attempt_id` nullable saat fase face recognition aktif | Event sah memakai waktu server dan `client_event_id` idempotent. Percobaan face gagal tidak membuat event presensi sah. |
| `attendance_verification_attempts` **(usulan tingkat lanjut)** | `tenant_id`, `id`, `employee_id`, `shift_instance_id`, `event_type`, `client_event_id`, `result` (`passed`/`failed`/`unavailable`), `provider_ref`, `error_code`, `checked_at` | Jejak hasil verifikasi tanpa menyimpan template biometrik mentah di database. Kontrak endpoint, autentikasi, threshold, retry, dan kebijakan saat layanan gagal ditentukan sebelum integrasi. |

`clock_in_before_min`, `clock_in_after_min`, dan `clock_out_after_min` pada desain awal digantikan oleh batas jendela eksplisit sebagai **sumber aturan tunggal**. Jika API lama kelak memakai offset menit, nilainya dihitung dari jendela, bukan disimpan lagi sebagai aturan yang berpotensi berbeda.

## 3. Contoh perilaku jendela

Shift terjadwal **09:00–17:00**, jendela masuk **08:00–09:30**, jendela pulang **16:30–19:00**, dan toleransi terlambat **15 menit**. Masuk pukul 09:20 **boleh dicatat tetapi terlambat**; masuk 09:31 berada di luar jendela. Pulang 16:45 boleh dicatat, tetapi aturan pulang cepat tetap dinilai terhadap jam selesai terjadwal 17:00. Semua batas diuji terhadap waktu server dan zona kantor yang tersimpan pada instance.

Shift lintas hari **22:00–06:00** memakai `work_date` hari masuk. Contoh jendela masuk 21:30–22:30 pada offset hari 0 dan jendela pulang 05:30–07:00 pada offset hari 1. Perhitungan `DATETIME` UTC dilakukan saat instance dibuat, sehingga presensi pukul 06:00 esok hari tetap milik `work_date` sebelumnya.

## 4. Penerapan template ke roster

1. Pengelola memilih pola, karyawan/scope, dan rentang tanggal. Backend menampilkan pratinjau hasil beserta benturan dengan roster yang sudah ada, hari libur, dan penugasan kantor.
2. Pola mingguan membaca satu baris untuk setiap Senin–Minggu. Pola bulanan memakai baris kalender Senin–Minggu: baris 1→W1, ... baris 5→W5, baris 6→**W5 kembali**. Hanya tanggal dalam bulan yang dimasukkan.
3. Penerapan menulis `work_schedules` berstatus `draft`, menyimpan `source_pattern_id` dan `source_pattern_version`. Edit pola setelah itu tidak mengubah roster yang sudah dibuat.
4. Publikasi mengunci/menandai roster dan membuat atau memperbarui `shift_instances` pada jendela materialisasi yang ditetapkan. Roster terbit tidak ditimpa diam-diam ketika template diterapkan ulang; perubahan membuat revisi yang diaudit.
5. UI mengurutkan shift dan pola pin lebih dulu; warna sel roster berasal dari shift yang ditugaskan. Kontras teks dihitung UI agar tetap terbaca dalam empat tema.

## 5. Urutan implementasi dan tes

Migrasi aditif Jadwal telah membuat master shift → pola/slot → roster → instance → event/attempt dengan ID 26 karakter yang kompatibel dengan tabel lama. D-01 sudah diputuskan satu shift per karyawan per `work_date`. Field face recognition dibuat dengan default `false`; pemanggilan endpoint tetap modul tahap lanjut sampai kontrak provider diberikan. Tidak ada data biometrik mentah dalam migrasi awal.

Tes wajib: jendela masuk/pulang tepat pada batas dan di luarnya; terlambat masih dalam jendela; shift lintas hari dan zona waktu kantor; pin dan urutannya; warna valid; pola mingguan 7 hari; pola bulanan untuk bulan 4/5/6 baris kalender termasuk pengulangan W5; hari off; edit pola setelah roster terbit; tenant FK; retry presensi; selfie wajib; face wajib/gagal/provider tidak tersedia setelah integrasi.
