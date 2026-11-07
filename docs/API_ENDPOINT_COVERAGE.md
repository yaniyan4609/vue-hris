# Cakupan endpoint API

`API_ENDPOINT_COVERAGE.csv` adalah daftar kerja dari **Spesifikasi API digaji.id — REST + WebSocket.md**. Jalankan `python scripts/api-coverage.py` setelah menambah operasi OpenAPI. Status `active` berarti pasangan method dan path yang sama persis telah dicatat di OpenAPI aktif; `planned` berarti belum. Alias lama dan endpoint tambahan dihitung terpisah, sehingga angka ini adalah alat pelacak, bukan klaim bahwa seluruh alur bisnis telah selesai.

Per 7 November 2026: spesifikasi menghasilkan **506 operasi** setelah metode gabungan dan lima master organisasi diuraikan. **121** pasangan method/path cocok dengan OpenAPI aktif, **385** masih direncanakan. OpenAPI aktif berisi **171 operasi**, termasuk alias kompatibilitas dan tambahan pola jadwal yang belum tertulis dalam tabel spesifikasi.

Irisan terbaru menambah CRUD keluarga dan kontak darurat, baca riwayat penugasan, daftar/detail/buat/ubah/hapus kontrak, serta pembacaan pengajuan dan alur approval. Keputusan approval, perpanjangan kontrak, penulisan presensi, payroll, file, job, dan modul lain tetap dilanjutkan menurut `BACKEND_API_IMPLEMENTATION_PLAN.md`. Rute yang belum memiliki perilaku server tidak dimasukkan ke OpenAPI aktif.
