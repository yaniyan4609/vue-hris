# vue-hris — digaji.id SaaS HRIS

Repositori produk berdasarkan `PRD.MD`. Frontend memakai template Vue 3 milik pemilik proyek dari [ui-ux-vue-v1.0](https://github.com/yaniyan4609/ui-ux-vue-v1.0), commit `31411c9`. Panduan UI berada di `frontend/aturan-ui.md` dan `frontend/desain-ui.md`.

## Struktur

- `frontend/` — template Vue/Vite dan komponen Base dengan login serta panel awal untuk tiga peran. View demonstrasi template tetap menjadi referensi kode dan tidak masuk navigasi produk.
- `backend/` — API Go/Chi, sesi login, seed staging, dan evaluator permission/scope awal.
- `docs/IMPLEMENTATION_PLAN.md` — urutan fase, keputusan, dan isu produk yang masih terbuka.
- `PRD.MD` — kebutuhan produk dari Product Owner.

## Menjalankan lokal

```powershell
cd frontend
npm ci
npm run dev
```

```powershell
cd backend
$env:DB_ADDR = '<host-database>:3306'
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

Frontend default di `http://localhost:5173` dengan proxy `/api` ke API `http://127.0.0.1:8080`. `GET /api/v1/health` memeriksa proses API dan `GET /api/v1/ready` memeriksa koneksi database. Kontrak ada di `docs/openapi.yaml`.

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

Database staging bernama `staging-saas-hris` pada MariaDB 10.11 sesuai keputusan Product Owner. Kredensial hanya diberikan melalui konfigurasi lokal yang tidak di-commit.

Migrasi 00001 dan 00002 sudah diterapkan ke staging. Struktur inti dan batas implementasinya dijelaskan di `docs/SCHEMA_CORE.md`. Jangan menyimpan `DB_PASSWORD` di file proyek atau commit.
