# vue-hris — digaji.id SaaS HRIS

Repositori produk berdasarkan empat dokumen di `full-docs/`: PRD v2.3, Matriks Menu/Role/Fungsional, Desain Database, dan Spesifikasi API REST + WebSocket. Frontend memakai template Vue 3 milik pemilik proyek dari [ui-ux-vue-v1.0](https://github.com/yaniyan4609/ui-ux-vue-v1.0), commit `31411c9`. Panduan UI berada di `frontend/aturan-ui.md` dan `frontend/desain-ui.md`.

## Struktur

- `frontend/` — template Vue/Vite dan komponen Base dengan login morphing, panel Platform/Admin, serta ESS mobile. View demonstrasi tersedia melalui menu Template khusus Superadmin.
- `backend/` — API Go/Chi, sesi login, seed staging, evaluasi permission/scope, organisasi/karyawan, inbox notifikasi, dan WebSocket notifikasi awal.
- `docs/IMPLEMENTATION_PLAN.md` — urutan fase, keputusan, dan isu produk yang masih terbuka.
- `full-docs/` — kebutuhan produk dan rancangan lengkap dari Product Owner.
- `docs/BACKEND_API_IMPLEMENTATION_PLAN.md` — peta kesenjangan API dan urutan implementasi penuh backend.

## Menjalankan lokal

Di Windows, klik dua kali `run-hris.bat` untuk menjalankan frontend, backend, WebSocket, dan Swagger UI pada IP LAN serta pasangan port yang kosong. Kata sandi MariaDB diminta hanya pada jalankan pertama dan disimpan terenkripsi untuk akun Windows ini. Launcher juga membuat kunci TOTP platform acak dan menyimpannya terenkripsi untuk akun Windows yang sama. Alamat, cara menghentikan proses, dan log dijelaskan di `docs/LOCAL_LAUNCHER.md`.

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
$env:PLATFORM_MFA_KEY = '<kunci-acak-32-byte-dalam-base64>' # wajib untuk login staf platform biasa
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

Migrasi 00001–00029 sudah diterapkan ke staging. Migrasi 00025–00029 menambahkan bukti verifikasi ulang, preferensi akun, TOTP platform, evaluasi MFA saat role berubah, token reset kata sandi, serta versi role tenant. Login staf platform biasa memerlukan TOTP; tombol demo staging tetap melewati TOTP. Launcher staging mengatur SMTP `mail.digaji.id:587` dengan STARTTLS dan mengambil kata sandi dari credential Windows terenkripsi di `work/launcher/smtp-credential.xml`. Sertifikat SMTP saat ini tidak dipercaya dan tidak memuat nama `mail.digaji.id`; pengiriman reset/undangan menunggu perbaikan sertifikat server, sebagaimana dijelaskan di `docs/LOCAL_LAUNCHER.md`. Jika server email gagal setelah akun dibuat, API mengembalikan invitation_status=failed; admin dapat mengirim ulang tautan dari endpoint reset. Inbox REST `/api/v1/notifications`, token `/api/v1/auth/csrf`, dan WebSocket `/api/v1/realtime/ws` sudah aktif. Sebagian besar API domain lain masih direncanakan dalam `docs/BACKEND_API_IMPLEMENTATION_PLAN.md`. Jangan menyimpan `DB_PASSWORD`, `PLATFORM_MFA_KEY`, atau kredensial SMTP di file proyek atau commit.
