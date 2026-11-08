# Cakupan endpoint API

`API_ENDPOINT_COVERAGE.csv` adalah daftar kerja dari **Spesifikasi API digaji.id — REST + WebSocket.md**. Jalankan `python scripts/api-coverage.py` setelah menambah operasi OpenAPI. Status `active` berarti pasangan method dan path yang sama persis telah dicatat di OpenAPI aktif; `planned` berarti belum. Alias lama dan endpoint tambahan dihitung terpisah, sehingga angka ini adalah alat pelacak, bukan klaim bahwa seluruh alur bisnis telah selesai.

Per 8 November 2026: spesifikasi menghasilkan **506 operasi** setelah metode gabungan dan lima master organisasi diuraikan. **136** pasangan method/path cocok dengan OpenAPI aktif, **370** masih direncanakan. OpenAPI aktif berisi **186 operasi**, termasuk alias kompatibilitas dan tambahan pola jadwal yang belum tertulis dalam tabel spesifikasi.

Irisan terbaru menambah pengajuan lembur ESS, daftar/detail lembur ESS dan Admin, serta keputusan `approve`/`reject` dengan handler atomik untuk lembur. Koreksi menit dan alasan lembur saat kirim ulang serta pembatalan pengajuan sendiri juga tersedia. Jenis pengajuan lain, penulisan presensi, payroll, file, job, dan modul lain tetap dilanjutkan menurut `BACKEND_API_IMPLEMENTATION_PLAN.md`. Rute yang belum memiliki perilaku server tidak dimasukkan ke OpenAPI aktif.
