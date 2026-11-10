# Verifikasi perhitungan PPh 21

Status: **komponen hitung terisolasi; belum dipakai oleh payroll run**. Validasi draft tetap mengeluarkan blocker `TAX_ENGINE_UNVERIFIED` dan tidak menghasilkan nominal.

## Acuan resmi dan hasil tes

- [PP 58/2023 di DJP, penjelasan Pasal 2, contoh Tuan R](https://www.pajak.go.id/id/peraturan/tarif-pemotongan-pajak-penghasiian-pasal-21-atas-penghasilan-sehubungan-dengan-pekerjaan): gaji Rp10.000.000 per bulan, PTKP K/0, iuran pensiun Rp100.000 per bulan. TER kategori A 2% menghasilkan Rp200.000 pada tiap bulan Januari–November. Penghasilan kena pajak setahun Rp54.300.000, PPh setahun Rp2.715.000, dan pemotongan Desember Rp515.000. `TestOfficialTuanRFullYear` memeriksa semua angka tersebut.
- [PMK 168/2023 di JDIH Kemenkeu](https://jdih.kemenkeu.go.id/api/download/e60a82e0-b218-40f5-9d18-b924aa1e11ce/2023pmkeuangan168.pdf), Lampiran bagian I: masa selain masa terakhir memakai TER atas bruto bulanan; masa terakhir memakai pajak tahunan dikurangi pemotongan terdahulu. Penghasilan kena pajak dibulatkan **ke bawah** ke ribuan rupiah penuh. Pasal 10 menetapkan biaya jabatan 5% dengan batas Rp500.000 per bulan atau Rp6.000.000 per tahun. Tes tambahan memeriksa batas bracket, pembulatan, batas biaya jabatan, tarif progresif, dan kelebihan pemotongan.

`backend/internal/payrollcalc` menerima angka dalam **perseratus rupiah**, memakai tarif dalam *parts per million*, dan menghitung tanpa `float`. Input bracket dan jumlah uang harus sudah diambil dari versi aturan tenant yang berlaku. Saat ini komponen hanya mencakup TER bulanan pegawai tetap dan penyesuaian masa terakhir untuk pegawai tetap yang bekerja penuh satu tahun kalender pada pemberi kerja yang sama.

## Syarat sebelum mengaktifkan hitung payroll

1. Tambahkan parameter aturan pengurang dan pemetaan komponen gaji ke bruto pajak, BPJS, dan potongan yang berlaku pada tanggal periode. Jangan memakai angka contoh PP sebagai aturan global; aturan tetap dikelola per tenant.
2. Verifikasi kasus pegawai masuk/keluar tengah tahun, perubahan pemberi kerja, status PTKP, penghasilan tidak teratur, iuran JHT/JP, zakat, serta koreksi lebih potong memakai contoh resmi tambahan. Untuk periode 2026, periksa juga kelayakan insentif PPh 21 ditanggung pemerintah menurut [PMK 105/2025 di JDIH Kemenkeu](https://jdih.kemenkeu.go.id/dok/pmk-105-tahun-2025/overview); komponen ini belum menghitung insentif tersebut.
3. Cocokkan hasil *end-to-end* sumber data → PPh 21 → BPJS → kasbon → gaji bersih, termasuk pembulatan uang menurut pengaturan tenant dan snapshot aturan. Baru setelah itu hapus blocker dan buka submit/finalisasi, slip, serta ekspor bank.
