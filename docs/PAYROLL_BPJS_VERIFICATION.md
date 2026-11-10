# Verifikasi iuran BPJS payroll

Status: komponen hitung terisolasi; **belum dipakai oleh payroll run**. Preflight periode kini mensyaratkan aturan KESEHATAN, JHT, JKK, JKM, dan JP yang mencakup seluruh tanggal periode.

`backend/internal/payrollcalc.MonthlyBPJS` menerima dasar upah, tarif pekerja/pemberi kerja, dan plafon **per program** dalam perseratus rupiah. Tarif memakai *parts per million*; `0` pada plafon berarti tidak ada batas atas. Komponen menghitung kedua bagian secara terpisah tanpa `float` dan menolak aturan yang tidak lengkap atau duplikat. Nilai tarif dan plafon harus datang dari versi aturan tenant yang tepat.

## Acuan uji

- [Perpres 64/2020, Pasal 30 dan 32 di JDIH Kemenkeu](https://jdih.kemenkeu.go.id/api/download/179608f7-a2a0-41c0-aaec-e7d60eb36054/64TAHUN2020PERPRES.pdf): Kesehatan PPU 1% pekerja, 4% pemberi kerja, dan batas atas dasar upah Rp12.000.000. Aturan juga menyebut batas bawah berdasarkan upah minimum wilayah; komponen ini **belum** menentukan batas bawah.
- [BPJS Ketenagakerjaan, Perhitungan Iuran Penerima Upah](https://www.bpjsketenagakerjaan.go.id/penerima-upah.html): contoh tarif JHT 2% pekerja/3,7% pemberi kerja, JP 1%/2%, JKM 0,3% pemberi kerja, dan JKK sesuai kelas risiko. Tes memakai kelas risiko rendah 0,54% untuk JKK. Batas JP dalam halaman tersebut tidak dijadikan konstanta program karena berubah menurut periode.

Tes dasar upah Rp6.000.000 per program menghasilkan total iuran pekerja Rp240.000 dan pemberi kerja Rp632.400 dari kelima aturan contoh. Tes lain memeriksa plafon Kesehatan, aturan yang hilang/duplikat, tarif tidak valid, serta cakupan lima program lintas versi tanggal.

## Sebelum dihubungkan ke run

Pastikan dasar upah program dihitung dari komponen yang benar, batas bawah Kesehatan mengikuti kantor/wilayah dan tanggal, kelas risiko JKK serta kepesertaan JP berlaku untuk karyawan, plafon JP dipilih dari versi tenant yang tepat, dan kebijakan pembulatan sesuai data pembayaran nyata. Simpan snapshot aturan yang dipakai agar run lama tidak berubah saat tarif baru diterbitkan.
