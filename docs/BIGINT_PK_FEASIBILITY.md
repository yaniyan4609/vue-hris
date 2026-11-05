# Kajian terpisah: PK `BIGINT` vs ID 26 karakter

**Status: kajian, bukan keputusan migrasi.** Product Owner memilih kajian `BIGINT` terpisah pada 5 November 2026. Dokumen ini tidak mengubah primary key, foreign key, API, maupun data staging.

## Fakta proyek yang terverifikasi

- Server staging: MariaDB `10.11.14`; 35 tabel aplikasi dan 51 foreign key. Tabel inti memakai banyak PK/FK `CHAR(26) CHARACTER SET ascii COLLATE ascii_bin`.
- `backend/internal/ids/ids.go` membuat 16 byte acak dengan `crypto/rand`, lalu meng-encode menjadi 26 karakter Crockford Base32. **Ini bukan ULID dengan timestamp**, walaupun bentuk teksnya serupa. Dokumen desain menyebut ULID; terminologi dan harapan urutan insert perlu diselaraskan.
- ID 26 karakter dipakai pada FK tenant, user, karyawan, role, sesi, scope, dan organisasi, serta dikembalikan oleh API. Setidaknya 12 file Go pada `backend/internal` dan `backend/cmd` berisi pembuatan atau validasi panjang ID. Migrasi penuh berarti perubahan skema, query, serialisasi, dan jalur backfill, bukan sekadar `ALTER COLUMN`.
- Data staging masih kecil: 1 tenant, 3 user, 2 membership, 1 karyawan, 9 role, dan sekitar 582 grant role saat pemeriksaan. Ukuran/index tabel staging **tidak cukup** untuk menyimpulkan perbedaan performa pada volume produksi.

## Perbandingan awal

| Opsi | Keuntungan potensial | Biaya/risiko |
| --- | --- | --- |
| A. Pertahankan ID acak `CHAR(26)` | Tidak mengubah FK/API/data lama; pengembangan domain bisa lanjut. | Kunci lebih panjang; insert acak perlu diuji pada volume tinggi. |
| B. Pertahankan kolom `CHAR(26)`, ubah generator **ID baru** menjadi ULID yang benar dan berurutan waktu | Tanpa rewrite FK lama; berpotensi memperbaiki lokalitas insert untuk baris baru. | ID lama tetap acak; generator, konkurensi, serta urutan leksikografis harus diuji. Jangan mengubah arti ID tanpa tes kontrak. |
| C. `BIGINT` untuk **tabel transaksi baru ber-volume tinggi**, `public_id CHAR(26)` untuk API, FK ke tenant/employee legacy tetap `CHAR(26)` | PK lokal lebih pendek tanpa memigrasikan seluruh 35 tabel. | Model kunci campuran, mapping internal/publik, query dan FK lebih kompleks; tenant FK panjang masih ada. |
| D. Migrasi seluruh tabel ke PK/FK `BIGINT` + `public_id CHAR(26)` | Kunci internal pendek dan model seragam menurut dokumen desain. | Menulis ulang 51 FK dan data terkait, memetakan seluruh ID, menjaga API/sesi/audit, potensi downtime serta rollback rumit. |

Perbandingan ukuran **nilai kunci**, bukan estimasi ukuran tabel: `BIGINT` = 8 byte; `CHAR(26)` dengan charset ASCII = 26 byte. Pasangan `(tenant_id,id)` saat ini memuat 52 byte nilai teks, dibanding 16 byte bila keduanya `BIGINT`. Ukuran indeks nyata juga mencakup struktur halaman, nullability, dan kolom lain. Dokumentasi MariaDB menyatakan PK InnoDB adalah clustered index dan nilainya ikut tersimpan di secondary index, sehingga panjang PK memengaruhi ukuran indeks. [Ukuran tipe data](https://mariadb.com/docs/server/reference/data-types/data-type-storage-requirements), [BIGINT](https://mariadb.com/docs/server/reference/data-types/numeric-data-types/bigint), [primary key InnoDB](https://mariadb.com/docs/server/architecture/server-constraints/primary-key-constraints).

## Pengujian yang perlu dilakukan sebelum memilih

1. Siapkan database benchmark **terpisah dari staging** dengan struktur dan indeks setara untuk tabel kecil (`employees`) dan besar (`attendance_events`, `notifications`, `payroll_item_lines`). Tidak memakai data pribadi nyata.
2. Isi volume representatif dan pola tenant realistis. Bandingkan A, B, C, D dengan jumlah baris dan indeks sama; ukur ukuran data/index, throughput insert, p95 daftar/detail dalam scope tenant, join, waktu backup/restore, dan `EXPLAIN`.
3. Uji keunikan serta urutan generator B pada proses Go paralel dan lintas instance. MariaDB 10.11 tidak menyediakan `UUID_v7()` bawaan; fungsi itu baru tersedia dari 11.7, sehingga ID berurut harus dibuat aplikasi atau pustaka yang diverifikasi. [MariaDB UUID_v7](https://mariadb.com/docs/server/reference/sql-functions/secondary-functions/miscellaneous-functions/uuid_v7).
4. Untuk opsi C/D, buat peta ID lama→baru pada salinan database. Uji FK lintas tenant, seluruh endpoint, sesi aktif, audit, entitlement, data scope, migrasi maju, dan prosedur kembali. Jangan memakai `CAST` ID acak ke angka.
5. Tetapkan ambang keputusan sebelum melihat hasil: manfaat ukuran/latensi harus cukup besar untuk menutup biaya perubahan dan risiko migrasi. Jika tidak, pertahankan A atau uji B untuk baris baru.

## Rekomendasi sementara

**Tahan perubahan PK/FK legacy.** Untuk menjalankan migrasi aditif `00011`–`00023` tanpa mengubah kontrak API, tabel baru memakai ID acak 26 karakter yang sama dengan tabel lama. Ini keputusan kompatibilitas tahap ini, bukan keputusan akhir menolak `BIGINT`. Opsi B dan C tetap layak diuji sebelum mempertimbangkan konversi penuh D; hasil benchmark diperlukan sebelum merancang migrasi PK/FK lain.

Keputusan akhir masih milik Product Owner setelah hasil benchmark, estimasi migrasi, dan batas downtime ditinjau bersama.
