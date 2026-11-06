# vue-hris — digaji.id SaaS HRIS

Repositori produk berdasarkan empat dokumen di `full-docs/`: PRD v2.3, Matriks Menu/Role/Fungsional, Desain Database, dan Spesifikasi API REST + WebSocket. Frontend memakai template Vue 3 milik pemilik proyek dari [ui-ux-vue-v1.0](https://github.com/yaniyan4609/ui-ux-vue-v1.0), commit `31411c9`. Panduan UI berada di `frontend/aturan-ui.md` dan `frontend/desain-ui.md`.

## Struktur

- `frontend/` — template Vue/Vite dan komponen Base dengan login morphing, panel Platform/Admin, serta ESS mobile. View demonstrasi tersedia melalui menu Template khusus Superadmin.
- `backend/` — API Go/Chi, sesi login, seed staging, evaluasi permission/scope, organisasi/karyawan, inbox notifikasi, dan WebSocket notifikasi awal.
- `docs/IMPLEMENTATION_PLAN.md` — urutan fase, keputusan, dan isu produk yang masih terbuka.
- `full-docs/` — kebutuhan produk dan rancangan lengkap dari Product Owner.
- `docs/BACKEND_API_IMPLEMENTATION_PLAN.md` — peta kesenjangan API dan urutan implementasi penuh backend.

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

Frontend default di `http://localhost:5173` dengan proxy `/api` ke API `http://127.0.0.1:8080`. `GET /api/v1/health` memeriksa proses API dan `GET /api/v1/ready` memeriksa koneksi database. Dokumentasi Swagger UI tersedia di `/api/v1/docs/` pada port frontend atau `/api-docs/` pada port backend; sumber kontrak berada di `docs/openapi.yaml` dan petunjuknya di `docs/API_DOCS.md`. Endpoint daftar awal memakai `page`, `pageSize` (1–100), dan `q`, lalu mengembalikan `{ items, total, page, pageSize }`.

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

Migrasi 00001–00024 sudah diterapkan ke staging. Migrasi 00011–00023 menambahkan skema domain secara aditif, termasuk assignment karyawan, approval, shift/roster, presensi, cuti, payroll, billing, dan penyimpanan ciphertext per tenant. Migrasi 00024 memberi akses inbox milik sendiri kepada role tenant, tetap tunduk pada Deny dan entitlement. Inbox REST `/api/v1/notifications`, token `/api/v1/auth/csrf`, dan WebSocket `/api/v1/realtime/ws` sudah aktif; WebSocket saat ini mengirim `notification.created` serta jumlah belum dibaca, dengan pemulihan melalui REST `/notifications/sync`. Skema baru belum berarti seluruh API/worker/UI domain aktif; status rinci dan keputusan tersisa ada di `docs/DATABASE_SCHEMA_MIGRATION_DRAFT.md`, `docs/IMPLEMENTATION_PLAN.md`, dan `docs/BACKEND_API_IMPLEMENTATION_PLAN.md`. Admin dapat membuka `/admin/settings/users` untuk mengelola akses; Admin dan ESS dapat membuka inbox notifikasi nyata. Struktur inti dijelaskan di `docs/SCHEMA_CORE.md`. Jangan menyimpan `DB_PASSWORD` atau master key di file proyek atau commit.
