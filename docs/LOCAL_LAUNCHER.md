# Menjalankan HRIS staging sekali klik di Windows

Klik dua kali **`run-hris.bat`** di akar proyek. Launcher otomatis:

1. Memilih IPv4 LAN dari adaptor dengan gateway aktif. `192.168.11.85` tetap alamat **MariaDB**, bukan alamat komputer yang menjalankan aplikasi.
2. Memilih pasangan port pertama yang kosong: frontend/backend `38473/38474`, lalu `39473/39474`, `40473/40474`, atau `41473/41474`.
3. Membangun backend Go, lalu menjalankan backend pada semua interface jaringan dan Vite pada semua interface jaringan. WebSocket dan Swagger UI berada di backend yang sama; frontend memproksikan `/api`, `/healthz`, dan `/readyz` ke backend.
4. Memeriksa `/readyz` serta halaman docs, menampilkan URL, dan membuka frontend serta docs di browser.

Saat pertama kali dijalankan, masukkan kata sandi MariaDB staging di jendela launcher. Setelah koneksi berhasil, credential disimpan dengan enkripsi Windows yang terikat pada akun Windows ini di `work/launcher/db-credential.xml`. File `work/` diabaikan Git. Launcher tidak menaruh kata sandi dalam `.bat`, argumen command line, atau log. Jalankan berikutnya cukup klik `.bat` lagi. Jika `DB_PASSWORD` sudah ada di environment proses, launcher memakai nilai itu tanpa membuat file credential.

Contoh pada komputer dengan IP LAN `192.168.200.112` dan pasangan port pertama:

| Layanan | Alamat |
| --- | --- |
| Frontend | `http://192.168.200.112:38473/` |
| Backend | `http://192.168.200.112:38474/` |
| WebSocket | `ws://192.168.200.112:38473/api/v1/realtime/ws` |
| Swagger UI | `http://192.168.200.112:38473/api/v1/docs/` |

Alamat dan port aktual selalu tercetak di jendela launcher. Biarkan jendela itu terbuka. Tekan **Enter** untuk menghentikan kedua proses, atau klik dua kali **`stop-hris.bat`**. Launcher yang diklik lagi saat proses masih aktif akan menampilkan URL yang sama tanpa membuat instance ganda. Log proses ada di `work/launcher/<waktu-instance>/`.

Untuk memilih adaptor lain, isi environment `HRIS_LAN_IP` dengan salah satu IPv4 yang benar-benar terpasang pada komputer sebelum menjalankan launcher. Jika kata sandi database berubah, hentikan launcher lalu hapus hanya file `work/launcher/db-credential.xml`; jalankan ulang dan masukkan kata sandi baru. Akses dari ponsel/komputer lain memerlukan kedua perangkat dapat saling menjangkau lewat LAN dan aturan firewall Windows mengizinkan koneksi pada port yang dipilih. Launcher tidak mengubah aturan firewall.
