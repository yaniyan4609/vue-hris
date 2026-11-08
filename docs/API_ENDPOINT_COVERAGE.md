# Cakupan endpoint API

`API_ENDPOINT_COVERAGE.csv` adalah daftar kerja dari **Spesifikasi API digaji.id — REST + WebSocket.md**. Jalankan `python scripts/api-coverage.py` setelah menambah operasi OpenAPI. Status `active` berarti pasangan method dan path yang sama persis telah dicatat di OpenAPI aktif; `planned` berarti belum. Alias lama dan endpoint tambahan dihitung terpisah, sehingga angka ini adalah alat pelacak, bukan klaim bahwa seluruh alur bisnis telah selesai.

Per 8 November 2026: spesifikasi menghasilkan **506 operasi** setelah metode gabungan dan lima master organisasi diuraikan. **171** pasangan method/path cocok dengan OpenAPI aktif, **335** masih direncanakan. OpenAPI aktif berisi **221 operasi**, termasuk alias kompatibilitas dan tambahan pola jadwal yang belum tertulis dalam tabel spesifikasi.

Irisan terbaru menambah CRUD kategori reimbursement dan batas bulanan, termasuk nonaktif tanpa menghapus klaim lama, serta daftar/detail klaim ESS dan Admin yang mengikuti scope. Pembuatan klaim, kuitansi, pembayaran, jenis pengajuan lain, penulisan presensi, payroll, file, job, dan modul lain tetap dilanjutkan menurut `BACKEND_API_IMPLEMENTATION_PLAN.md`. Rute yang belum memiliki perilaku server tidak dimasukkan ke OpenAPI aktif.
