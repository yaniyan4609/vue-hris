# Dokumentasi API interaktif

Swagger UI menyajikan endpoint yang **sudah aktif** pada backend saat ini:

- Lewat Vite saat pengembangan: `http://127.0.0.1:5176/api/v1/docs/` (atau port Vite yang sedang dipakai).
- Langsung dari backend: `http://127.0.0.1:8080/api-docs/`.
- YAML yang dilayani backend: `/api/v1/docs/openapi.yaml` atau `/api-docs/openapi.yaml`.
- Sumber spesifikasi untuk diedit: `docs/openapi.yaml` di akar repositori.

Swagger UI dan asetnya dibundel di server, sehingga tampilan docs tidak memerlukan CDN. Dokumentasi dikelompokkan menjadi Health, Auth, Access, Organization, Employees, Platform, Notifications, dan Realtime. Gunakan kotak filter untuk mencari operasi.

## Mencoba endpoint

1. Masuk melalui aplikasi pada origin yang sama, atau jalankan operasi `POST /api/v1/auth/login` dari Swagger UI. `demo-login` tersedia hanya pada staging yang mengaktifkan fitur tersebut.
2. Buka operasi, klik **Try it out**, lalu **Execute**. Cookie sesi HttpOnly dikirim browser. Untuk mutasi, halaman Swagger mengambil token dari `/api/v1/auth/csrf` dan mengisi `X-CSRF-Token` otomatis.
3. Tenant aktif mengikuti sesi. Pilih tenant melalui aplikasi atau operasi `select-tenant` sebelum mencoba endpoint tenant.

Swagger UI tidak membuka koneksi WebSocket dari operasi GET. Bagian Realtime menjelaskan handshake dan pesan; gunakan klien WebSocket untuk pengujian aliran event. Aksi dalam **Try it out** berjalan terhadap database yang terhubung ke backend, jadi gunakan payload uji dengan sengaja.

## Memperbarui spesifikasi

Setelah mengedit `docs/openapi.yaml`, jalankan dari akar repositori:

```powershell
./backend/scripts/sync-openapi.ps1
cd backend
go test ./internal/httpapi -run 'TestAPIDocsRoutes|TestEmbeddedOpenAPIMatchesSource'
```

Tes gagal jika spesifikasi sumber dan salinan yang dibundel tidak identik. Jalankan ulang backend agar perubahan docs yang dibundel terlihat. Swagger UI memakai `swagger-ui-dist` versi 5.34.1; lisensinya disertakan pada aset backend.
