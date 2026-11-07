# Rencana implementasi penuh backend API digaji.id

Status: roadmap aktif, 6 November 2026. Acuan utama: `full-docs/Spesifikasi API digaji.id — REST + WebSocket.md` (draft 4 November 2026), PRD v2.3, Matriks Menu/Role/Fungsional, Desain Database, dan keputusan Product Owner dalam percakapan. Proyek Go/Chi, Vue, dan MariaDB staging `192.168.11.85` dilanjutkan; migrasi `00001`–`00023` tidak ditulis ulang.

**Status implementasi:** migrasi aditif `00024`–`00032` sudah diterapkan pada staging. Inbox REST/WebSocket awal, CSRF untuk mutasi bersesi, `X-Request-Id`, versi akses/ETag, health/readiness, reauth 5 menit, sesi perangkat, preferensi, TOTP staf platform, dan token reset kata sandi sudah tersedia. Endpoint organisasi dan akses Admin baru memakai `{data,meta}` dan `snake_case` sambil mempertahankan alias lama. CRUD role kustom, penugasan role, Allow/Deny massal, scope, pengguna tenant, sesi, dan undangan staf platform sudah ditambahkan. SMTP lokal telah dikonfigurasi, tetapi host `digaji.id:587` tidak dapat dijangkau dan sertifikat host MX belum valid; pengiriman email menunggu perbaikan server mail. OpenAPI 0.10.0 mencakup master organisasi, geofence, lookup berscope, shift, pola, hari libur, roster draft/publikasi, shift instance, baca jadwal dan presensi ESS, serta izin kantor tambahan karyawan. Cakupan pasangan method/path tersedia di `API_ENDPOINT_COVERAGE.md`. Penulisan presensi, protokol WebSocket lengkap, worker outbox, dan modul domain lain masih mengikuti tahap di bawah.

**Pembaruan 7 November 2026:** CRUD keluarga dan kontak darurat, riwayat penugasan, kontrak draft/aktivasi/perpanjangan, endpoint baca pengajuan, serta editor alur approval (CRUD, tahap, default, simulasi) telah ditambahkan dan diuji di staging. Migrasi `00031` menyiapkan status `returned`, nomor percobaan, dan versi untuk kirim ulang pada request yang sama. Migrasi `00032` melengkapi izin approval untuk jadwal dan perubahan profil. Aksi keputusan approval, koreksi domain, kirim ulang, dan notifikasi atomik masih harus diimplementasikan. Perpanjangan menutup kontrak lama tepat sehari sebelum kontrak baru mulai.

## 1. Hasil pencocokan spesifikasi dengan kode

| Area | Kondisi sekarang | Target dan cara transisi |
| --- | --- | --- |
| Cakupan | Route yang aktif terutama auth, akses, organisasi, karyawan, daftar tenant, dan staf platform. Tabel domain sudah ada, tetapi sebagian besar API domain, worker, file storage, dan WebSocket belum aktif. | Implementasi endpoint per irisan vertikal; halaman pratinjau tetap berlabel **Dalam pengembangan** sampai operasi server dan tesnya siap. |
| Path | Org memakai `/admin/organization/{kind}`; akses `/admin/access/*`; auth `/select-tenant`, `/sessions`, `/change-password`; ESS `/ess/me/employee`; health `/api/v1/health` dan `/api/v1/ready`. | Kontrak baru memakai `/admin/regions` dan empat master lain, `/admin/users`/`roles`, `/auth/switch-tenant`, `/me/sessions`, `/auth/password/change`, `/ess/profile`, `/healthz` dan `/readyz`. Pertahankan alias lama sementara frontend dimigrasikan, lalu hapus setelah tes kontrak dan rilis transisi. |
| JSON dan daftar | DTO sekarang banyak memakai `camelCase`, respons langsung/`{items,total,page,pageSize}`, query `pageSize`, `sort` + `order`, error string. | Endpoint baru memakai `snake_case`, `{data,meta}`/`{error:{code,message,details,request_id}}`, `per_page`, dan `sort=-field`; maksimal 100. Adapter pada endpoint lama atau versi transisi agar UI yang ada tetap bekerja. Jika kontrak lama tidak bisa dipertahankan, gunakan `/v2` sesuai aturan spesifikasi. |
| Sesi dan keamanan | Cookie `digaji_session`, `SameSite=Strict`; mutasi memakai pemeriksaan Origin. Reautentikasi pada editor hak masih mengirim kata sandi dalam body aksi. | Finalkan cookie `dgj_session` + `SameSite=Lax` dengan pembacaan cookie lama selama masa transisi; CSRF header untuk mutasi bersesi, endpoint `/auth/csrf` dan `/auth/reauth` dengan bukti 5 menit; sesi platform tanpa Simpan Sesi dan 2FA. Cookie lama tidak boleh terputus tanpa rencana migrasi sesi. |
| Akses | Registry tenant/entitlement/platform dan evaluasi batch ada. `/me/access` belum menyajikan `access_version` pada respons dan header; permission platform kini berformat `platform.perusahaan.lihat`, sedangkan spesifikasi menulis `platform_tenant.lihat`. | Satu peta kode permission kanonik dan migrasi aditif yang menjaga grant lama; `X-Access-Version`, ETag/304 untuk `/me/access`, 403 tanpa permission dan 404 untuk objek di luar scope. Seluruh daftar, detail, mutasi, lookup, job, dan file harus memakai evaluator serta filter scope yang sama. |
| Data dan transaksi | Migrasi `00011`–`00023` sudah mencakup assignment, approval, shift/pola, presensi, cuti, payroll, notifikasi, billing, audit, dan enkripsi. `import_jobs`/`export_jobs` terpisah. | Tambah tabel pendukung spesifikasi secara aditif, bukan mengganti tabel yang sudah terpakai. Worker dan handler domain masih harus dibangun. |
| Shift dan pola | Tabel shift memuat jendela clock-in/out, selfie, face, pin, warna; pola mingguan/bulanan dan satu shift per `work_date` sudah dimodelkan. | Tambahkan kontrak API `schedule-patterns` dan `slots`, pratinjau penerapan, serta `dry_run` roster. Pola bulanan W1–W5 Senin–Minggu; minggu kalender ke-6 mengulang W5. Face recognition memakai provider adapter dan tidak menghalangi fase presensi dasar. |
| Pajak/BPJS | `tax_rules` dan `bpjs_rules` berversi per tenant di migrasi `00020`. | Koreksi Bagian 11.1 spesifikasi: baca **dan kelola aturan per tenant**, dengan permission sensitif, reautentikasi, tanggal berlaku, dan audit. Jangan membuat admin platform sebagai penulis aturan global. |
| Identitas ID | PK saat ini ULID 26 karakter yang juga dipakai API. | Pertahankan untuk API penuh tahap ini. Klaim spesifikasi bahwa ada ID internal terpisah belum sesuai proyek; kajian BIGINT tetap dokumen/pekerjaan terpisah. |

## 2. Kesepakatan kontrak sebelum coding domain

1. Bekukan OpenAPI **spec-first** di `docs/openapi.yaml`: operasi, schema request/response, kode error, permission, scope, entitlement, reauth, idempotensi, dan contoh. Jangan memperluas file lama dengan endpoint kosong yang diklaim aktif; beri status per operasi (`planned`, `implemented`) dalam matriks terpisah.
2. Buat inventaris endpoint spesifikasi versus route nyata dengan `method + path` sebagai kunci. Setiap tiket implementasi menunjuk operasi OpenAPI, tabel, permission, halaman pemakai, dan tes penerimaan.
3. Pilih satu gaya kontrak baru (`snake_case`, `data/meta`, error berkode), pertahankan adapter route/DTO lama selama migrasi frontend. Contract test memastikan alias dan route baru memberi keputusan akses yang sama. Ubah frontend ke kontrak baru per fitur, baru deprekasi alias.
4. Tetapkan aturan umum: tenant dari sesi, `X-Request-Id`, `X-Access-Version`, cursor untuk log, offset untuk master, batas 100, waktu UTC, uang string desimal, `version` untuk edit serentak, `Idempotency-Key` untuk aksi bertanda I, audit untuk A, job untuk J, entitlement untuk E. Whitelist kolom sort/filter; validasi body dan batas ukuran per operasi.

## 3. Migrasi aditif lanjutan yang diusulkan

Semua perubahan mulai `00024`, setelah kontrak field dan relasi disetujui; uji pada database salinan atau transaksi isolasi sebelum staging. Jangan mengisi atau mereset ulang data demo.

| Kebutuhan | Perubahan yang diusulkan |
| --- | --- |
| Idempotensi | `idempotency_keys`: tenant/user/route/key, hash request, status, response yang aman disimpan, expiry, unique key. Presensi tetap memiliki `client_event_id` sebagai proteksi domain tambahan. |
| Job generik | `jobs` untuk status, progres, owner, payload terbatas, `scope_snapshot`, output file, error, expiry. `import_jobs`/`export_jobs` lama ditautkan atau dibaca melalui adapter; tidak langsung dihapus. |
| Profil ESS | `profile_change_requests` terikat tenant/karyawan/request approval, dengan payload terenkripsi untuk field sensitif dan status versi. |
| Platform 2FA | Penyimpanan credential MFA, recovery code hash, challenge dengan expiry dan audit. Pilih TOTP/WebAuthn pada keputusan produk; jangan simpan secret mentah. |
| Konkurensi | Kolom `version` hanya pada resource yang benar-benar diedit serentak (role/permission, roster, flow, broadcast, payroll draft, settings). Approval instance dan work schedule sudah mempunyai versi. |
| Pengaturan pajak tenant | Indeks/constraint untuk rentang versi aturan yang tidak tumpang tindih, audit perubahan, dan snapshot aturan pada payroll run; tabel pajak/BPJS per tenant sudah ada. |
| Permission spesifikasi | Tambah kode yang dipakai endpoint tetapi belum ada, misalnya `kasbon.kelola`, `disiplin.hapus`, `terminasi.ubah`, `aset.hapus`, `agenda.hapus`; selaraskan kode platform melalui alias atau migrasi grant. Pemeriksaan final dilakukan otomatis terhadap OpenAPI dan registry. |

## 4. Urutan implementasi backend

Setiap irisan mencakup handler Chi, service/domain, query DB tenant-aware, OpenAPI, audit/outbox bila berlaku, tes unit dan integrasi, lalu pengkabelan UI. Endpoint yang belum berfungsi tidak dipasang sebagai stub sukses.

| Tahap | API/worker | Gerbang selesai |
| --- | --- | --- |
| **0A. Kontrak dan kompatibilitas** | Katalog operasi, schema OpenAPI, error berkode, pagination/filter, header, alias route/DTO lama, health/readiness. | CRUD organisasi/karyawan serta login lama lulus; bentuk kontrak baru lulus tes; tidak ada perubahan data staging. |
| **0B. Identitas dan akses** | CSRF, reauth, password reset, switch tenant, sesi perangkat, preferensi, `me/access` version/ETag, role/override/scope versi baru, 2FA platform, rate limit login. | Empat role platform dan preset tenant diuji; Deny menang, larangan eskalasi diri/delegasi, tenant assignment, entitlement, 403/404, pencabutan sesi pada request berikutnya. |
| **0C. Primitif bersama** | File S3-compatible dengan verifikasi/scanning, audit, idempotensi, jobs/worker, outbox, lookup terikat scope, kunci enkripsi per tenant dari master key persisten. | Retry tidak membuat duplikat; file dan job tidak bisa dibaca lintas tenant/scope; rotasi kunci diuji tanpa kehilangan data. |
| **1A. SDM inti** | Master organisasi + geofence, karyawan dan data pendukung, kontrak, dokumen, direktori, pengaturan tenant, import/export. | Daftar/detail/mutasi memakai scope sama; field sensitif tersamar; impor per baris dan ekspor memakai scope snapshot; data lama tetap terbaca. |
| **1B. Approval inti** | Flow, simulate, request/inbox/task/aksi atomik, cancel/resubmit, audit dan outbox. Profil change dapat memakai mesin ini. | Self-approval tertolak, keputusan paralel hanya satu, handler domain tepat sekali. Mesin ini diselesaikan sebelum mutasi/terminasi diaktifkan. |
| **1C. SDM beralur** | Mutasi/promosi/demosi, SP, aset, resign/terminasi bertanggal dan worker status. | Tanggal efektif menulis assignment; terminasi mencabut role/sesi/WebSocket dan memicu payroll akhir sesuai aturan produk. |
| **2. Kehadiran** | Shift template dan pola, roster `dry_run`/publish, instance snapshot, libur, allowed office, geofence, presensi ESS, koreksi, monitor/rekap admin. | Satu shift per hari, shift lintas malam, jendela presensi, GPS/selfie, waktu server, idempotensi, scope monitor, event asli tetap; face adapter opsional. |
| **3. Pengajuan lain** | Cuti + ledger/akrual, lembur, kasbon, reimbursement, approval handler masing-masing. | Saldo dan pencairan tepat sekali; jam lembur diusulkan/disetujui/dibayar terpisah; kuitansi aman. |
| **4. Payroll dan platform** | Komponen dan aturan pajak/BPJS **per tenant**, run draft/recalculate/validation/approval/finalize/snapshot, slip, bank export; tenant lifecycle, paket, langganan, invoice, payment webhook, refund, support session/tiket, analitik agregat. | Final tidak dapat diubah, nominal desimal, slip dicek setiap unduh, support read-only terbatasi, webhook bertanda tangan dan idempoten. |
| **5. Komunikasi dan lanjutan** | Notifikasi REST, outbox delivery, reminder, broadcast, Web Push, WebSocket, dashboard/laporan/search, agenda, integrasi/API key dan talenta sesuai spesifikasi tambahan. | REST adalah sumber kebenaran; reconnect menyinkronkan event; recipient event memenuhi tenant/permission/scope; tidak ada data sensitif dalam frame. |

**Ketergantungan yang mengubah urutan dokumen API:** approval inti dikerjakan lebih awal daripada mutasi/terminasi dan sebelum transaksi cuti/lembur/kasbon. Outbox, job, dan file disiapkan sebelum broadcast, payroll PDF, atau import/export. WebSocket ditambahkan setelah inbox REST stabil; pekerjaan notifikasi REST dapat dimulai pada 0C/1B meskipun tahapan komunikasi lain berada di fase 5.

## 5. Aturan WebSocket dan pekerja latar

- `GET /realtime/ws` memakai sesi cookie dan allowlist Origin. Koneksi diikat ke user, membership/tenant aktif, `access_version`, dan sesi perangkat; klien tidak memilih channel bebas.
- Dispatcher mengambil event dari outbox yang sudah commit, memeriksa ulang tenant, permission, scope, dan status sesi saat mengirim. Payload hanya ID, kategori, status, waktu, tautan yang akan dicek ulang oleh REST; tidak ada NIK/rekening/nominal/isi dokumen.
- `hello`/`resume` tidak dianggap jaminan penyimpanan event. `GET /notifications/sync?after_id=...` dan cursor inbox adalah jalur pemulihan; event boleh diterima lebih dari sekali dan UI menduplikasi berdasarkan ID.
- Revoke/logout/password change/terminasi/tenant suspend memutus koneksi terkait. Pembaruan akses menutup `4408`; sesi tidak sah `4401`; batas koneksi `4429`. Ping dan backoff mengikuti spesifikasi.
- Worker memakai claim/lease, retry dengan backoff, dead-letter yang dapat diaudit, dan idempotensi domain. Job ekspor membaca `scope_snapshot`; jika akses dicabut sebelum hasil diunduh, unduhan tetap diperiksa ulang.

## 6. Verifikasi dan rilis per tahap

- Tes kontrak OpenAPI untuk semua route aktif; tes integrasi MariaDB dua tenant, dua user role sama beda scope, Allow/Deny, entitlement, 403/404, ID lintas tenant, aksi serentak, replay idempotensi, dan perubahan akses saat sesi hidup.
- Tes domain untuk approval paralel, saldo cuti, presensi retry/shift lintas malam, payroll final, webhook replay, serta worker yang mati di tengah proses. Tes keamanan CSRF, Origin WebSocket, file URL, audit redaction, dan kunci tenant.
- Tes kinerja pada akses batch/login, daftar berscope, dashboard agregat, broadcast, dan koneksi WebSocket; angka batas laju/NFR final ditetapkan setelah beban nyata. Pantau p95, query lambat, antrean job/outbox, dan kegagalan delivery.
- Staging memakai migrasi aditif dengan backup dan uji restore. Rilis lewat perubahan backend yang kompatibel, migrasi frontend per fitur, baru penghentian alias lama. Modul preview berubah menjadi aktif hanya setelah endpoint, permission, tes, dan UI end-to-end selesai.

## 7. Keputusan produk yang perlu ditutup

| Keputusan | Rekomendasi saat ini | Dampak |
| --- | --- | --- |
| A-01–A-06, A-10 | Ikuti usulan spesifikasi: OpenAPI spec-first, 404 di luar scope, offset master/cursor log, job impor/ekspor generik, respons massal per item, `version`, pesan Indonesia + kode stabil. | Menetapkan kontrak fondasi. |
| A-08 | Diputuskan: TOTP aplikasi authenticator. | Migrasi `00026`, aktivasi, verifikasi, dan kode pemulihan sudah aktif; tombol demo khusus staging melewati MFA. |
| A-09 | Support session baca-saja, tanpa payroll dan dokumen sensitif; durasi pendek, alasan, indikator UI, audit. | Menentukan evaluator khusus support. |
| Spesifikasi 11.1 vs keputusan Product Owner | **Aturan PPh 21/BPJS sepenuhnya per tenant**. Perbaiki spesifikasi sebelum OpenAPI final. | Endpoint CRUD aturan, hak kelola, snapshot run. |
| Pola roster | Tambah endpoint pola mingguan/bulanan, slot, duplicate, preview; data model sudah ada. | Tanpa ini UI pola tidak dapat memakai backend. |
| A-07, geofence, aturan cuti, lifecycle tenant, retensi | Tutup saat irisan terkait; batas laju final dari uji beban. | Tidak menghambat kontrak fondasi yang tidak bergantung padanya. |

**Belum dinyatakan siap produksi:** migrasi struktural sudah ada, tetapi mayoritas domain service, worker, object storage, pembayaran, dan kontrak API baru masih pekerjaan implementasi. WebSocket notifikasi yang sudah aktif adalah irisan awal; dispatcher outbox, event lain, dan skala banyak koneksi belum tersedia. Penyusunan rencana tidak mengubah staging; migrasi aditif `00024` diterapkan saat irisan pertama dikerjakan.

## 8. Irisan implementasi pertama setelah rencana disepakati

1. Tambah matriks `method + path` dari spesifikasi API ke dokumen kontrak, tandai `active`, `alias`, atau `planned`; koreksi Bagian 11.1 untuk pajak/BPJS per tenant dan tambah operasi pola roster yang belum ada di spesifikasi.
2. Finalkan OpenAPI fondasi (auth, `/me/access`, organisasi, karyawan, platform tenant/staf) dengan schema JSON, error, permission, contoh, dan hasil `204`/`304`. Pertahankan frontend berjalan lewat alias hingga migrasi DTO selesai.
3. Implementasikan middleware request ID, error berkode, CSRF, versi akses, dan kontrak pagination. Pindahkan satu resource organisasi ke kontrak baru sebagai percobaan menyeluruh dari DB sampai Vue.
4. Uji role/tenant/scope/entitlement pada kontrak lama dan baru; ukur jumlah query serta p95 akses/login; jalankan `go test ./...` dan `npm run build`. Setelah irisan ini lulus, lanjutkan reauth/MFA dan endpoint akses admin sebelum domain transaksional.
