// Helper format bersama. Impor dengan path relatif, mis. '../utils/format.js'.

// Format mata uang Rupiah (IDR), tanpa desimal. Nilai kosong/NaN ditampilkan sebagai 0.
export const formatRupiah = (val) => {
  return new Intl.NumberFormat('id-ID', {
    style: 'currency',
    currency: 'IDR',
    maximumFractionDigits: 0,
  }).format(val || 0)
}

