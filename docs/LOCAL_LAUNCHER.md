# Menjalankan HRIS staging sekali klik di Windows

Klik dua kali **`run-hris.bat`** di akar proyek. Launcher otomatis:

1. Memilih IPv4 LAN dari adaptor dengan gateway aktif. `192.168.11.85` tetap alamat **MariaDB**, bukan alamat komputer yang menjalankan aplikasi.
2. Memilih pasangan port pertama yang kosong: frontend/backend `38473/38474`, lalu `39473/39474`, `40473/40474`, atau `41473/41474`.
3. Membangun backend Go, lalu menjalankan backend pada semua interface jaringan dan Vite pada semua interface jaringan. WebSocket dan Swagger UI berada di backend yang sama; frontend memproksikan `/api`, `/healthz`, dan `/readyz` ke backend.
4. Memeriksa `/readyz` serta halaman docs, menampilkan URL, dan membuka frontend serta docs di browser.

Saat pertama kali dijalankan, masukkan kata sandi MariaDB staging di jendela launcher. Setelah koneksi berhasil, credential disimpan dengan enkripsi Windows yang terikat pada akun Windows ini di `work/launcher/db-credential.xml`. Launcher membuat kunci TOTP platform acak dan menyimpannya dengan enkripsi Windows di `work/launcher/platform-mfa-key.xml`; pertahankan file ini agar authenticator yang sudah terdaftar tetap dapat digunakan. SMTP staging memakai `digaji.id:587` dengan STARTTLS dan akun `admin@digaji.id`. Kredensialnya disimpan dengan enkripsi Windows di `work/launcher/smtp-credential.xml`; bila file belum ada, launcher meminta kata sandi sekali. File `work/` diabaikan Git. Launcher tidak menaruh rahasia dalam `.bat`, argumen command line, atau log. Jalankan berikutnya cukup klik `.bat` lagi. Jika `DB_PASSWORD`, `PLATFORM_MFA_KEY`, atau `SMTP_PASSWORD` sudah ada di environment proses, launcher memakai nilai tersebut.

Pada pemeriksaan 6 November 2026, DNS `digaji.id` mengarah ke dua IP yang tidak menerima koneksi port 587, sementara MX mengarah ke `mail.digaji.id`. Server `mail.digaji.id:587` dapat dijangkau tetapi menyajikan sertifikat mandiri bernama `etc` tanpa DNS name; verifikasi TLS gagal. Alur reset/undangan tetap mengembalikan kegagalan pengiriman sampai host SMTP yang dipilih bisa dijangkau dan menyajikan sertifikat tepercaya dengan SAN sesuai hostname serta rantai lengkap. Aplikasi tidak menonaktifkan verifikasi TLS dan tidak mengirim kredensial pada koneksi yang gagal diverifikasi.

Contoh pada komputer dengan IP LAN `192.168.200.112` dan pasangan port pertama:

| Layanan | Alamat |
| --- | --- |
| Frontend | `http://192.168.200.112:38473/` |
| Backend | `http://192.168.200.112:38474/` |
| WebSocket | `ws://192.168.200.112:38473/api/v1/realtime/ws` |
| Swagger UI | `http://192.168.200.112:38473/api/v1/docs/` |

Alamat dan port aktual selalu tercetak di jendela launcher. Biarkan jendela itu terbuka. Tekan **Enter** untuk menghentikan kedua proses, atau klik dua kali **`stop-hris.bat`**. Launcher yang diklik lagi saat proses masih aktif akan menampilkan URL yang sama tanpa membuat instance ganda. Log proses ada di `work/launcher/<waktu-instance>/`.

Untuk memilih adaptor lain, isi environment `HRIS_LAN_IP` dengan salah satu IPv4 yang benar-benar terpasang pada komputer sebelum menjalankan launcher. Jika kata sandi database atau SMTP berubah, hentikan launcher lalu hapus hanya file credential terkait di `work/launcher/`; jalankan ulang dan masukkan kata sandi baru. Akses dari ponsel/komputer lain memerlukan kedua perangkat dapat saling menjangkau lewat LAN dan aturan firewall Windows mengizinkan koneksi pada port yang dipilih. Launcher tidak mengubah aturan firewall.
