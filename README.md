# vue-hris — digaji.id SaaS HRIS

Repositori produk berdasarkan tiga dokumen lengkap di `full-docs/`: PRD v2.3, Matriks Menu/Role/Fungsional, dan Desain Database. Frontend memakai template Vue 3 milik pemilik proyek dari [ui-ux-vue-v1.0](https://github.com/yaniyan4609/ui-ux-vue-v1.0), commit `31411c9`. Panduan UI berada di `frontend/aturan-ui.md` dan `frontend/desain-ui.md`.

## Struktur

- `frontend/` — template Vue/Vite dan komponen Base dengan login morphing, panel Platform/Admin, serta ESS mobile. View demonstrasi template tetap menjadi referensi kode dan tidak masuk navigasi produk.
- `backend/` — API Go/Chi, sesi login, seed staging, evaluasi permission/scope dan endpoint organisasi/karyawan awal.
- `docs/IMPLEMENTATION_PLAN.md` — urutan fase, keputusan, dan isu produk yang masih terbuka.
- `full-docs/` — kebutuhan produk dan rancangan lengkap dari Product Owner.

## Menjalankan lokal

```powershell
cd frontend
npm ci
npm run dev
```

```powershell
cd backend
$env:DB_ADDR = '192.168.11.85:3306'
$env:DB_USER = '<pengguna-database>'
$env:DB_NAME = 'staging-saas-hris'
go run ./cmd/migrate -action up  # meminta kata sandi lewat terminal
$env:DB_PASSWORD = ConvertFrom-SecureString -SecureString (Read-Host 'Kata sandi database' -AsSecureString) -AsPlainText
$env:APP_ENV = 'staging'
$env:ENABLE_DEMO_LOGIN = 'true'
$env:COOKIE_SECURE = 'false' # hanya untuk HTTP localhost
$env:APP_ORIGIN = 'http://localhost:5173'
go run ./cmd/api
```

Frontend default di `http://localhost:5173` dengan proxy `/api` ke API `http://127.0.0.1:8080`. `GET /api/v1/health` memeriksa proses API dan `GET /api/v1/ready` memeriksa koneksi database. Kontrak ada di `docs/openapi.yaml`. Endpoint daftar awal memakai `page`, `pageSize` (1–100), dan `q`, lalu mengembalikan `{ items, total, page, pageSize }`.

Untuk menyiapkan ulang akun demo, jalankan `go run ./cmd/seed-demo` dari terminal backend dengan `APP_ENV=staging` dan `DB_NAME=staging-saas-hris`. Perintah meminta kata sandi database dan kata sandi akun demo secara interaktif; akun yang sudah ada dengan kata sandi berbeda tidak ditimpa. Tombol masuk cepat muncul hanya jika API berjalan dengan `APP_ENV=staging`, `ENABLE_DEMO_LOGIN=true`, dan nama database staging yang tepat. Daftar akun dan cara akses ada di `docs/DEMO_ACCOUNTS.md`.

## Verifikasi

```powershell
cd frontend
npm run build
```

```powershell
cd backend
go test ./...
go vet ./...
```

Database staging bernama `staging-saas-hris` pada server MariaDB 10.11 milik Product Owner di `192.168.11.85`. Kredensial hanya diberikan melalui konfigurasi lokal yang tidak di-commit.

Migrasi 00001–00004 sudah diterapkan ke staging. Migrasi keempat menambah metadata perangkat sesi untuk daftar sesi aktif dan pencabutan. Struktur inti dan batas implementasinya dijelaskan di `docs/SCHEMA_CORE.md`. Jangan menyimpan `DB_PASSWORD` di file proyek atau commit.
