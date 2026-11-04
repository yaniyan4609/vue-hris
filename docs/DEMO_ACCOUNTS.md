# Akun demo staging

Database: `staging-saas-hris`. Tenant: **PT CLIENT PERTAMA**.

| Email | Akses | Tenant |
| --- | --- | --- |
| `super@digaji.id` | Superadmin platform | Tanpa tenant |
| `admin@digaji.id` | Admin Tenant | PT CLIENT PERTAMA |
| `ess@digaji.id` | Karyawan ESS | PT CLIENT PERTAMA |

Kata sandi awal ketiga akun diberikan langsung kepada pemilik proyek dan dimasukkan melalui terminal saat seed. Dokumen publik ini tidak menyimpan kata sandi. Pada API staging yang mengaktifkan `ENABLE_DEMO_LOGIN=true`, form login menampilkan tiga tombol masuk cepat. Endpoint demo hanya tersedia jika `APP_ENV=staging` dan `DB_NAME=staging-saas-hris`; jangan aktifkan pada production.

Superadmin masuk ke `/platform`, admin tenant ke `/admin`, dan karyawan ke `/ess`. Jika seorang pengguna kelak memiliki lebih dari satu tenant, setelah sandi benar ia memilih perusahaan di `/pilih-perusahaan`. Sesi biasa tanpa **Simpan sesi** berakhir setelah 60 menit tanpa aktivitas; kebijakan ini disimpan per tenant.
