# Fase penyelesaian API per modul

Status awal: 9 November 2026. Sumber operasi: `full-docs/Spesifikasi API digaji.id — REST + WebSocket.md`; daftar method dan path ada di `API_ENDPOINT_COVERAGE.csv`. Proyek, migrasi, akun demo, dan data staging yang sudah ada dilanjutkan.

**Fase 1 selesai di staging:** lima master organisasi mempunyai GET daftar/detail, POST, PATCH, DELETE arsip, filter status/induk, pohon, dan lookup. Arsip menolak anak serta penugasan aktif; izin `organisasi.hapus` hanya diberikan awal kepada Admin Tenant. Migrasi `00046` diterapkan dan fixture QA dibersihkan setelah uji integrasi.

**Fase 2 selesai di staging:** daftar karyawan kini mendukung seluruh filter spesifikasi dan arsip, kuota paket diperiksa saat membuat, profil pribadi Admin dapat dibaca dan diubah, NIK dienkripsi dengan kunci data per tenant serta blind index terpisah, rekening bank mempunyai daftar/tambah/ubah/nonaktif dengan versi dan penyamaran menurut izin, koreksi assignment bertanggal memakai transaksi dan memeriksa roster/payroll terkunci, akun ESS dapat dibuat atau ditautkan dengan role dan undangan, serta summary, timeline, nomor NIP berikutnya, arsip dan pulih tersedia. Akun tidak diaktifkan otomatis saat pemulihan. Migrasi `00047` diterapkan; seluruh tes integrasi staging dan unit lulus. Endpoint dokumen karyawan tetap termasuk Fase 12. Fase berikutnya: approval inti.

## Aturan selesai satu modul

Satu modul baru dinyatakan selesai setelah **semua method/path modul dalam spesifikasi** memiliki perilaku server nyata, bukan respons sukses kosong. Kontrak OpenAPI harus mencatat request, respons, error, permission, scope, dan status. Operasi tulis memakai validasi, transaksi, audit, dan idempotensi bila diwajibkan. Uji mencakup hak akses, lintas tenant, scope, konflik, serta alur bisnis utama pada MariaDB staging menggunakan fixture sementara yang dibersihkan. Alias lama tetap diuji agar frontend yang ada tidak putus. Setelah modul lulus, baru pindah ke modul berikutnya. Fitur bertanda `[TERBUKA]` menunggu keputusan produk yang terkait sebelum modulnya dinyatakan selesai.

## Urutan modul

| Fase | Modul yang diselesaikan penuh | Ketergantungan / hasil akhir |
| --- | --- | --- |
| 1 | **Master organisasi**: wilayah, kantor, divisi, departemen, jabatan; pohon, filter, lookup, arsip | Lengkapi `organisasi.hapus`, arsip aman, status, dan pemeriksaan penugasan aktif; pertahankan API lama. |
| 2 | **Karyawan dan data pendukung**: profil Admin, assignment, rekening bank, akun, timeline, arsip/pulih | Gunakan kunci tenant yang sudah ada; data sensitif tidak masuk audit/log; assignment memakai approval bila perlu. |
| 3 | **Approval inti**: delegasi, reassign, aksi massal | Tuntaskan keputusan atomik, versi, audit, outbox, dan batas delegasi sebelum domain berikutnya. |
| 4 | **Mutasi, promosi, demosi** | Tanggal efektif konsisten dengan assignment dan approval. |
| 5 | **Disiplin dan konseling** | SP, tahapan, dan riwayat per karyawan. |
| 6 | **Resign dan terminasi** | Checklist dan tanggal efektif mencabut role/sesi serta mengakhiri assignment. |
| 7 | **Aset karyawan** | Penyerahan dan pengembalian tidak menghilangkan riwayat. |
| 8 | **Jadwal** | Roster, instance, generate/cancel, dan tukar shift; satu shift per hari dan shift lintas malam. |
| 9 | **Presensi** | Clock-in/out, GPS, jendela shift, selfie lokal staging, koreksi, monitor, rekap, dan retry idempotent. Face recognition menunggu kontrak provider. |
| 10 | **Cuti dan izin** | Accrual, saldo per tahun, hari kerja roster terbit, approval, dan pengembalian request. |
| 11 | **Payroll** | Pengaturan, draft run, kalkulasi, validasi, adjustment, approval, finalisasi, ekspor bank, slip. Perlu keputusan program BPJS wajib dan uji pajak resmi sebelum menghitung uang. |
| 12 | **Dokumen** | Tipe, template, dokumen karyawan/perusahaan, verifikasi, publikasi, akses ESS, serta file lokal staging. |
| 13 | **Agenda** | CRUD acara, peserta, dan kalender sesuai scope. |
| 14 | **Direktori** | Hanya profil publik sesuai izin dan scope. |
| 15 | **Notifikasi dan WebSocket** | Preferensi, aturan, template, delivery, reminder, Push, worker outbox, retry, dan reconnect. |
| 16 | **Broadcast** | Target, lampiran, jadwal, kirim, batalkan, statistik, dan bacaan ESS. |
| 17 | **Impor, ekspor, dan job** | Template, preview, error, commit, unduh, cancel; job idempotent dan file tenant-scope aman. |
| 18 | **Dashboard** | Agregat Admin/ESS dari data nyata, tanpa angka contoh. |
| 19 | **Laporan** | Katalog, parameter, dan ekspor berscope. |
| 20 | **Audit dan keamanan** | Log audit/login, ekspor, ringkasan keamanan. |
| 21 | **Pencarian global** | Hanya hasil yang diizinkan dalam scope pengguna. |
| 22 | **Pengaturan tenant** | Profil perusahaan, setting, langganan/usage, API key, webhook keluar. |
| 23 | **Dukungan tenant** | Tiket dan pesan dengan batas tenant. |
| 24 | **Platform: tenant** | Dashboard, lifecycle tenant, penugasan staf, dan support session. |
| 25 | **Platform: paket dan billing** | Paket, entitlement, subscription, invoice, payment/refund; perlu kontrak gateway. |
| 26 | **Platform: operasi** | Staf, tiket, analitik, audit, pengumuman, dan pengaturan platform. |
| 27 | **Google OAuth** | Perlu kredensial dan aturan callback yang disepakati. |
| 28 | **Webhook pembayaran masuk** | Perlu kontrak signature/replay gateway; endpoint versi layanan dituntaskan bersama fase ini. |

API health, auth lokal, akses inti, dan fondasi lain yang sudah aktif dipertahankan serta diuji regresinya pada setiap fase.
