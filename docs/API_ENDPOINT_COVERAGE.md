# Cakupan endpoint API

`API_ENDPOINT_COVERAGE.csv` adalah daftar kerja dari **Spesifikasi API digaji.id — REST + WebSocket.md**. Jalankan `python scripts/api-coverage.py` setelah menambah operasi OpenAPI. Status `active` berarti pasangan method dan path yang sama persis telah dicatat di OpenAPI aktif; `planned` berarti belum. Alias lama dan endpoint tambahan dihitung terpisah, sehingga angka ini adalah alat pelacak, bukan klaim bahwa seluruh alur bisnis telah selesai.

Per 7 November 2026: spesifikasi menghasilkan **506 operasi** setelah metode gabungan dan lima master organisasi diuraikan. **128** pasangan method/path cocok dengan OpenAPI aktif, **378** masih direncanakan. OpenAPI aktif berisi **178 operasi**, termasuk alias kompatibilitas dan tambahan pola jadwal yang belum tertulis dalam tabel spesifikasi.

Irisan terbaru menambah perpanjangan kontrak dan editor alur approval (CRUD, tahap, default, dan simulasi approver), selain operasi keluarga, kontak darurat, kontrak, dan pembacaan pengajuan yang telah tersedia. Keputusan approval, perbaikan dan kirim ulang pengajuan, penulisan presensi, payroll, file, job, dan modul lain tetap dilanjutkan menurut `BACKEND_API_IMPLEMENTATION_PLAN.md`. Rute yang belum memiliki perilaku server tidak dimasukkan ke OpenAPI aktif.
