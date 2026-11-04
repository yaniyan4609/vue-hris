# Rencana implementasi digaji.id

Status: rancangan kerja berdasarkan `PRD.MD` v2.3 (4 November 2026) dan template `frontend/` pada commit `31411c9`. Dokumen ini dapat bertambah saat paruh PRD yang belum ditulis selesai. Label `[WAJIB]`, `[BERTAHAP]`, `[USULAN]`, dan `[TERBUKA]` di PRD tetap menjadi sumber keputusan produk.

PRD saat ini berhenti di Bagian 13. Bagian 17 dan AC-01 sampai AC-58 disebut di pembuka, tetapi belum ada dalam file. Kriteria terukur untuk fase setelah fondasi perlu dilengkapi sebelum modulnya dinyatakan selesai.

## Pemahaman produk

digaji.id adalah SaaS HRIS multi-tenant dengan tiga area: platform untuk operator SaaS, Admin Panel untuk operasi SDM tenant, dan ESS untuk karyawan. Satu identitas dan sesi tenant dapat membuka ESS serta Admin sesuai permission efektif. Hak akses merupakan gabungan banyak role yang berlaku pada tanggal tertentu, override per user, scope data per permission, dan entitlement paket. Backend Go wajib menegakkan semuanya pada setiap jalur data; UI hanya mencerminkan hasilnya.

Organisasi memisahkan wilayah/kantor dari divisi → departemen → jabatan. Kehadiran mengacu pada shift instance dan waktu server, dengan geofence multi-polygon serta idempotency. Approval dan notifikasi adalah layanan lintas modul. Payroll Indonesia masuk fase akhir dan aturan regulasi diverifikasi lagi saat implementasi.

## Keputusan teknis yang sudah ada

- Vue 3 + Vite + Tailwind v4 + Vue Router; komponen `Base*` dan pola layout dari template.
- Go, MySQL 8.x, Redis, storage S3-compatible, OpenAPI, Docker/Linux.
- Router backend: **Chi**, dipilih Product Owner pada 4 November 2026.
- Database `staging-saas-hris` dibuat pada server yang diberikan Product Owner. Server tersebut terverifikasi MariaDB 10.11, sementara PRD mewajibkan MySQL 8.x. Migrasi dan skema fisik menunggu penyelarasan lingkungan.
- Mulai sebagai modular monolith Go. Ini keputusan implementasi yang dapat ditinjau kembali, sesuai usulan PRD.
- Repo template di-clone ke `frontend/`. Data dan halaman demonstrasi di dalamnya adalah referensi, bukan fitur produk.

## Urutan pengerjaan

| Fase | Hasil yang dapat diuji | Syarat selesai |
|---|---|---|
| 0. Fondasi | Struktur backend Go, API `/api/v1/health`, kontrak error/versi, evaluasi akses murni, draft ERD dan OpenAPI, setup MySQL/Redis/S3 lokal; template Vue tetap dapat dijalankan | `go test ./...`, `npm run build`, health merespons; keputusan terbuka yang menyentuh fase berikut dicatat |
| 1. Identitas dan organisasi | Login/logout dengan cookie sesi opaque, tenant membership/switcher, satu sesi lintas panel, permission registry, role bertanggal, override, scope SQL, entitlement, audit; wilayah/kantor dan divisi/departemen/jabatan, karyawan serta riwayat | Uji lintas tenant, deny dan override, scope AND/OR, perubahan hak berlaku pada request berikutnya, audit dan re-autentikasi aksi berisiko |
| 2. Operasi harian | ESS mobile PWA, shift master/roster/instance, geofence, clock-in/out idempotent, koreksi, cuti/izin, lembur, approval terpusat | Uji shift lintas hari, flexible, lokasi, retry, saldo dan approval atomik, larangan approve sendiri |
| 3. Komunikasi dan keuangan HR | Notification outbox, inbox REST + WebSocket, Web Push, reminder, broadcast bertarget; kasbon dan reimbursement; laporan | Uji penerima berdasarkan tenant/scope, dedupe, reconnect/replay, reminder recheck, pembayaran sekali |
| 4. Payroll dan platform SaaS | Payroll run/snapshot/slip/ekspor bank; paket, langganan, billing, tenant lifecycle, dashboard platform | Uji finalisasi dan adjustment, field sensitif, audit; verifikasi pajak/BPJS dan kebijakan tenant sebelum rilis |

## Irisan implementasi untuk setiap modul

1. Finalkan aturan bisnis dan acceptance criteria yang terkait, terutama setiap `[TERBUKA]`.
2. Definisikan entitas, constraint/index berawalan tenant, migrasi, dan kontrak OpenAPI.
3. Implementasikan penyaringan tenant + entitlement + permission + scope di backend, termasuk ekspor, file, worker, dan event.
4. Hubungkan frontend ke API. Menu difilter dari akses efektif API. Halaman menggunakan `Base*`, token tema, dan pola desktop/mobile dari dokumen UI.
5. Tambahkan pengujian yang membuktikan batas tenant, otorisasi, transaksi/idempotency, serta alur pengguna. Jalankan build dan pemeriksaan visual empat tema.

## Keputusan produk yang masih terbuka

Butir berikut tidak akan diberi aturan bisnis tersirat saat modulnya dikerjakan:

- Alur login ketika satu email ada di beberapa tenant; idle timeout sesi non-persisten.
- Masa trial/grace, perilaku tenant suspended, dan retensi setelah berhenti.
- Perlu tidaknya legal entity di MVP; tanggal kerja shift fleksibel tanpa jam baku.
- Ambang akurasi GPS, toleransi tepi geofence, dan alur kegagalan lokasi.
- Carry-over/prorata/cuti bersama/saldo negatif; delegasi/escalation/SLA approval.
- Default frekuensi reminder dan jam tenang; detail payroll yang bertanda `[TERBUKA]`.
- Kebijakan pelindungan data pribadi dan pemeriksaan hukum.

## Temuan template yang perlu ditangani saat integrasi

- `useAuth.js` dan login di `AppLayout.vue` masih simulasi: pengguna contoh, sandi contoh, `sessionStorage`, dan animasi yang menganggap login berhasil. Ganti dengan API sesi tanpa menyimpan token sensitif di browser storage.
- Menu/rute demo (`/template/*`) serta data Gunung/Pesanan belum sesuai produk. Simpan sebagai referensi pola selama transisi; hapus dari navigasi produk setelah menu berbasis permission siap.
- `EssLayout` belum ada. Admin/Superadmin dapat memakai dua layer dan dual rail; ESS membutuhkan layout mobile dengan bottom navigation maksimal lima item.
- Build awal berhasil tetapi satu bundle sekitar 640 kB; route lazy loading dilakukan saat rute produk dibangun.
- Dokumen desain berisi beberapa contoh `<input>`/`<button>` mentah, sedangkan `aturan-ui.md` melarangnya bila sudah ada komponen Base. Saat berbeda, ikuti kode dan aturan UI terbaru.

## Pekerjaan awal pada sesi ini

Fondasi Go/Chi dan evaluasi akses murni dibuat terlebih dahulu. Evaluator ini belum menjadi otorisasi API sampai identitas, membership, registry, entitlement, dan filter SQL tersedia. Endpoint health tidak mengakses data tenant.
