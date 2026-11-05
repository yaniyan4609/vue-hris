# Rencana implementasi digaji.id — revisi setelah PRD lengkap

Status: rencana kelanjutan proyek, 5 November 2026. Acuan produk adalah tiga dokumen di `full-docs/` (PRD v2.3, Matriks Menu/Role/Fungsional, dan Desain Database), serta `frontend/aturan-ui.md` dan `frontend/desain-ui.md`. Database tetap memakai server milik Product Owner di `192.168.11.85`, database `staging-saas-hris` (MariaDB 10.11, protokol MySQL); tidak memindahkannya ke server lain. Rancangan database berlabel **[USULAN]**; bagian **[TERBUKA]** belum menjadi keputusan implementasi. Saat dokumentasi dan kode berbeda, inventarisasi kondisi kode terlebih dahulu, lalu putuskan migrasi tambahan. Migrasi yang sudah diterapkan tidak diubah atau dijalankan ulang dari nol.

## Tujuan dan batas produk

Satu platform SaaS melayani banyak tenant tanpa kebocoran data. Akun global dapat menjadi anggota beberapa tenant; satu sesi yang sama dapat membuka ESS dan Admin sesuai permission efektif. Superadmin memakai panel platform terpisah. Hak akses merupakan gabungan role aktif bertanggal, override Allow/Deny per user, scope per permission, dan entitlement paket; backend memutuskan akses akhir. Organisasi memisahkan Wilayah → Kantor dari Divisi → Departemen → Jabatan. ESS adalah PWA mobile-first; Admin dan Platform desktop-first. UI memakai template Vue yang telah diintegrasikan, termasuk dua layer, dual rail, morphing login/logout, komponen Base, dan empat palet terang/gelap.

## Baseline proyek yang dipertahankan

| Area | Sudah tersedia di proyek | Celah terhadap PRD lengkap |
| --- | --- | --- |
| Infrastruktur | Monorepo `frontend/` Vue 3/Vite/Tailwind v4 dan `backend/` Go/Chi; staging MariaDB 10.11 `staging-saas-hris`; migrasi Goose 00001–00003 (00003 sudah diterapkan di staging, masih berada di working tree). | PRD menyebut MySQL 8, Pinia, Redis, S3, Docker/Linux, observability dan CI; belum semua ada. Sintaks/fitur MySQL 8 dalam rancangan DB perlu diuji kompatibilitas MariaDB sebelum migrasi baru. |
| Identitas | Login dan logout dengan cookie sesi opaque, hash token di DB, toggle Simpan Sesi, idle 60 menit saat OFF, pemilihan tenant setelah login, tiga akun demo staging. | Daftar/cabut semua sesi, ganti sandi dan pencabutan sesi lain, re-autentikasi aksi kritis, pengaturan sesi per tenant, audit login, dan siklus resign belum lengkap. |
| Akses | Evaluator Go untuk multi-role/override/scope; `Store.Decide` membaca DB per request; filter SQL karyawan; capability API Admin; beberapa uji unit dan integrasi staging. | Registry belum lengkap, entitlement belum aktif, UI Pengguna & Hak Akses belum ada, mutasi akses belum punya versi/audit/delegasi, dan cakupan enforcement baru pada endpoint yang sudah dibuat. |
| Data & API | Master Wilayah/Kantor/Divisi/Departemen/Jabatan serta karyawan punya endpoint daftar/buat/ubah; audit write awal dan riwayat penugasan awal; platform dapat membaca daftar tenant. | Struktur `employees` saat ini menyimpan penugasan aktif dan tabel history tambahan; desain baru mengusulkan `employee_assignments` bertanggal dengan snapshot dimensi. Belum ada pagination/filter/sort server, kontrak OpenAPI lengkap, impor/ekspor, arsip, dan field sensitif. |
| UI | Template Admin/Platform, morphing login/logout, ESS mobile, form demo staging, halaman organisasi/karyawan, dark mode dengan token permukaan. Build frontend lulus. | View data saat ini masih irisan awal dan harus diselaraskan dengan `BaseDataTable`/formula Data View, route/menu permission + entitlement, panel switcher, PWA, empat tema di seluruh layar, dan alur domain riil. ESS untuk kehadiran/pengajuan masih empty state. |

`PRD.MD` lama terlihat terhapus secara lokal dan `full-docs/` baru belum terlacak Git. Perubahan dokumen Product Owner tersebut dipertahankan apa adanya; pemindahan/commit dokumen dilakukan terpisah dari perubahan kode agar riwayatnya jelas.

## Urutan kerja yang baru

Setiap fase menghasilkan migrasi **aditif**, kontrak API, UI yang terhubung ke data nyata, dan tes batas tenant/permission/scope. Menu fitur yang belum hidup tidak dipresentasikan sebagai fitur siap pakai.

| Tahap | Implementasi lanjutan | Gerbang selesai |
| --- | --- | --- |
| **A. Stabilkan baseline** | Review dan selesaikan working tree sebelumnya; uji migrasi 00003 dan integrasi staging; sinkronkan README, skema, OpenAPI, dan referensi PRD; verifikasi dark mode empat tema, mobile Admin/ESS, morphing dan reduced motion. | `go test ./...`, `go vet ./...`, `npm run build`, tes integrasi tenant dan audit, pemeriksaan visual; perubahan kode ter-commit tanpa mengikutsertakan perpindahan dokumen secara tidak sengaja. |
| **B. Lengkapi fondasi Fase 0** | Registry permission final dari PRD + lima modul tambahan pada Matriks; preset role yang sudah disetujui; entitlement paket dan kuota awal; access-version/invalidation; endpoint akses efektif dan panel switcher; audit login/akses; sesi aktif, cabut sesi, ganti sandi; platform tenant/paket minimal; OpenAPI error/pagination; CI tenant-isolation. | AC-01–11, AC-45, AC-47–53 untuk perilaku fondasi. Deny menang, OR/AND scope benar, perubahan hak berlaku pada request berikutnya, tenant lain tidak terbaca, paket membatasi UI **dan** API. |
| **C. SDM inti Fase 1** | Migrasi aditif dari `employees`/history kini ke penugasan efektif bertanggal; base office tunggal + allowed offices; CRUD organisasi/karyawan dengan detail, status/arsip, filter/sort/pagination server, riwayat; kontrak, dokumen dan penyimpanan objek; impor CSV dengan preview/validasi dan ekspor sebagai job; inbox REST, outbox, broadcast dasar. | AC-12–16, AC-38–39, AC-54–55 dan AC-57. FK tenant komposit, riwayat penugasan tidak tertimpa, impor/ekspor memakai scope dan audit, field sensitif hanya bagi pemegang izin. |
| **D. Kehadiran Fase 2** | Shift master, roster, shift instance snapshot, libur dan allowed offices; geofence multi-polygon; presensi ESS dengan waktu server, selfie sesuai kebijakan, idempotency; rekap, koreksi; ESS PWA, reminder, WebSocket replay dan Web Push opt-in. | AC-17–23, AC-35–43 yang terkait notifikasi. Shift lintas hari memakai work date yang benar, retry tidak menggandakan event, reminder berhenti saat kondisi berubah, event asli tidak diedit. |
| **E. Pengajuan Fase 3** | Mesin approval terpusat, task approver berdasar permission + scope, transaksi atomik, anti self-approve; cuti/izin dan ledger saldo, lembur/SPKL, kasbon, reimbursement; notifikasi tiap transisi. Jika D-06 memilih menunda mutasi/terminasi, kedua alur itu masuk setelah mesin approval siap. | AC-24–31, AC-35, AC-56 dan AC-58 ketika terminasi diaktifkan. Keputusan serentak konsisten, saldo/pencairan hanya sekali, jam lembur diusulkan/disetujui/dibayar terpisah. |
| **F. Payroll dan platform Fase 4** | Aturan PPh 21/BPJS berversi setelah verifikasi regulasi, payroll run/snapshot/adjustment, slip dan ekspor bank; lifecycle tenant, langganan, billing, metrik platform dan audit pendukung. | AC-32–34 serta lifecycle yang sudah diputuskan. Hasil final tidak berubah; akses slip dicek saat unduh; nominal tidak memakai floating point. |
| **G. Fase 5** | Talenta, integrasi/API, email/WhatsApp, analitik lanjutan dan support session sesuai prioritas produk. | AC modul terkait dan keputusan produk baru. |

### Sprint berikutnya yang konkret

1. **Tutup pekerjaan yang sedang terbuka:** jalankan ulang tes integrasi migrasi 00003 setelah perubahan transaksi terakhir; koreksi bila gagal; cek tampilan dark Default/Blue/Pink/Terracotta pada Admin, Platform, ESS dan login; rapikan dokumentasi implementasi. Ini melanjutkan kode yang sudah dibuat, bukan membuat proyek baru.
2. **Tetapkan kontrak fondasi:** registry permission per modul dan entitlement; hasil `GET /me/access` berisi izin, scope/sumber, panel tersedia, dan entitlement; definisi menu/frontend membaca hasil tersebut. Panel switcher satu sesi ditambahkan dengan guard backend.
3. **Perkuat keamanan sesi dan akses:** audit perubahan role/override/scope, access-version, larangan eskalasi diri/delegasi berlebih, pencabutan sesi, serta tes lintas tenant dan status karyawan. Buat UI `PermissionMatrix`, `ScopePicker`, dan pratinjau akses dari komponen Base.
4. **Selaraskan model karyawan secara aditif:** tetapkan pemetaan data dari `employees`/`employee_assignment_history` ke model penugasan bertanggal, validasi node organisasi, dan indeks scope. Data demo/migrasi lama tetap terbaca selama transisi.
5. **Selesaikan daftar SDM yang siap dipakai:** pagination/filter/sort di server, detail/riwayat, form validasi, loading/error/empty state, data mobile card, serta OpenAPI dan tes kontrak. Setelah itu baru impor/ekspor dan modul SDM lain.

## Kontrak lintas modul

- **Tenant:** identitas tenant berasal dari sesi dan membership yang masih aktif. Semua query, join, worker, file, cache, ekspor, dan event dibatasi tenant. FK komposit menjadi lapisan kedua. ID dari body/URL tidak mengganti tenant sesi.
- **Akses:** permission harus ada di registry. Evaluasi `tenant ∩ entitlement ∩ permission ∩ scope`; Deny per user menang. Beberapa nilai satu dimensi = OR, antardimensi dalam aturan = AND, antaraturan/role = OR. Untuk data terikat karyawan, filter diterjemahkan ke SQL; detail/ubah/impor/ekspor memakai batas yang sama. Aksi berisiko meminta re-autentikasi.
- **Riwayat:** perubahan penempatan/status berlaku pada tanggal efektif, ditulis atomik, dan tidak menimpa rekaman lama. Audit append-only tidak menyimpan kata sandi, token, atau isi field sensitif mentah.
- **UI:** sebelum memakai Base, baca `defineProps`/`defineEmits` aktual. Data View mengikuti container fixed-height, tabel desktop + kartu mobile dan pagination server; modal lewat `BaseModal`, aksi berbahaya lewat `ConfirmationModal`, hasil lewat `useToast`. Dark mode memakai `--theme-bg`, `--theme-layer-main`, `--theme-layer-2`, `--theme-surface`, `--theme-border`, dan teks tema; Default harus gelap netral seperti template. Komponen demo hanya referensi pola.
- **Mutu:** setiap slice punya state loading/error/empty/tidak berhak, akses keyboard/fokus, `prefers-reduced-motion`, pengujian empat tema, serta tes API lintas tenant. Tidak menampilkan angka kehadiran/payroll yang tidak berasal dari API.

## Keputusan yang harus ditutup pada waktunya

**Sudah diputuskan Product Owner:** Go/Chi (T-01), memilih tenant setelah kata sandi benar bila satu email memiliki beberapa membership (T-02), idle 60 menit untuk Simpan Sesi OFF (T-03), serta staging MariaDB 10.11. Keputusan ini berlaku meskipun tabel keputusan PRD v2.3 belum diperbarui.

| Sebelum tahap | Keputusan terbuka dari dokumen | Dampak pada rencana |
| --- | --- | --- |
| Fondasi/SDM | **D-03:** DB baru mengusulkan BIGINT internal + ULID publik, sedangkan migrasi berjalan memakai ULID `CHAR(26)` sebagai PK. | Pilih mempertahankan identitas yang sudah berjalan atau migrasi aditif terukur; jangan mengganti PK migrasi 00001–00003 secara diam-diam. |
| SDM | **T-04** saran Jabatan → role; **T-05/D-09** legal entity; **T-06** trial/grace/suspended/retensi; **D-04** enkripsi dan kunci field sensitif; **D-06** approval sejak Fase 1 atau mutasi/terminasi ditunda. | Tidak mengaktifkan alur atau menyimpan data sensitif sebelum keputusan terkait. Pilihan produk yang aman untuk sementara: mutasi/terminasi tetap belum aktif. |
| Geofence/kehadiran | **D-08** MySQL spatial vs JSON + Go pada MariaDB; **D-01** satu atau beberapa shift per hari; **D-02** scope historis; **T-07–T-10** perangkat/PWA, GPS, work date fleksibel, interval reminder. | Pilih dan uji sebelum migrasi/algoritma presensi. `BaseMapPolygonEditor` sudah ada sebagai UI; penyimpanan dan validasi server tetap diputuskan tersendiri. |
| Pengajuan/payroll | **T-11–T-13**, **D-05** aturan pajak/BPJS global atau per tenant. | Jangan membakukan kebijakan cuti, delegasi, formula, atau pajak saat membuat tabel awal. |
| Pra-produksi | **T-14**, **T-15/D-07** target NFR, retensi dan kebijakan UU PDP. | Wajib ditutup sebelum klaim siap produksi; keputusan hukum memerlukan peninjauan yang tepat. |

Preset role dalam Matriks dan rancangan DDL adalah **[USULAN]**, sehingga ditinjau per bagian sebelum di-seed. Perbedaan lain yang sudah diketahui: dokumen DB memakai collation dan spatial syntax MySQL 8, sementara target nyata MariaDB 10.11; dokumen UI masih menyebut contoh menu/template lama dan menyatakan `EssLayout` belum ada, padahal kode produk sudah menambahkannya. Implementasi baru memakai kondisi kode terbaru dan melaporkan setiap penyelarasan.
