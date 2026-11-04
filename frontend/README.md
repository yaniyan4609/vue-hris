# Digaji.id — Modular Vue 3 UI/UX Template & Dashboard

Template antarmuka web modern berbasis **Vue 3 (Vite + SPA)** dengan arsitektur **2 Layer Layout** (Dasar Pertama & Dasar Kedua), sistem tema dinamis dengan *Pure Neutral Dark (Zero Blue Tint)*, komponen dasar (*Base UI*) mandiri tanpa library luar, serta visualisasi operasional SVG yang responsif.

Panduan arsitektur dan pedoman implementasi lengkap tersedia di [**`ui-desain.md`**](ui-desain.md).

---

## 🌟 Fitur Utama

- **Filosofi 2-Layer Architecture**:
  - **Dasar Pertama (Base Layer 1 - Backdrop)**: Pilihan latar interaktif (*Grid*, *Dots*, *Solid Color*, atau *Custom Image*).
  - **Dasar Kedua (Base Layer 2 - Floating Card)**: Kartu utama melayang dengan sudut lengkung halus (`rounded-2xl` / `rounded-3xl` di desktop, `rounded-t-[32px]` di mobile).
- **Dual-Sidebar Navigation Rail**:
  - **Primary Rail (`PrimarySidebar.vue`)**: Kolom navigasi ramping dengan indikator menu aktif.
  - **Submenu Rail (`SubmenuSidebar.vue`)**: Kolom submenu collapsible dengan *Sliding Active Pill Indicator* (`cubic-bezier(0.16, 1, 0.3, 1)`).
- **Mobile Drawer 100% Wireframe Compliant**:
  - Panel drawer membentang penuh atas-bawah (`inset-y-0 h-full`).
  - Animasi geser kanan untuk membuka, geser kiri untuk menutup.
  - Floating toggle button di pojok kiri bawah (`bottom-7 left-6 z-60`) dengan ikon squircle SVG kustom.
- **Sistem 4 Tema Warna Dinamis**:
  - **Default**: Pure Neutral Dark `#09090b` *(0% AI Blue Tint)*.
  - **Pink**: Identitas cerah `#E85D88` (CTA `#C83D6A`).
  - **Blue**: Modern SaaS `#007AFF`.
  - **Terracotta**: Warm Sand & Charcoal `#C03A18`.
- **Katalog Komponen Base UI Lengkap (`src/components/ui/`)**:
  - `BaseModal.vue` (**Auto-Adaptive**): Otomatis bertransformasi menjadi *Bottom Sheet iOS-style* di HP (`MobileModal.vue`) dan *Centered Dialog* di Desktop.
  - `BaseCurrencyInput.vue`: Auto format mata uang Rupiah Indonesia (1000 $\rightarrow$ 1.000) dengan konversi kata terbilang otomatis.
  - `BaseSearchableSelect.vue`: Dropdown pencarian lembut (*soft-color*) dengan navigasi keyboard.
  - `BaseSkeleton.vue`: Placeholder pemuatan data dengan animasi *shimmer sweep wave*.
  - `BaseCountUp.vue`: Animasi angka bergulir (*count-up ticker*) dengan kurva *ease-out cubic*.
  - `BaseInput.vue`, `BaseTextarea.vue`, `BaseSwitch.vue`, `BaseBadge.vue`, `BasePagination.vue`, `BaseDropdownMenu.vue`.
- **Dashboard Operasional Real-Time (Zero-Library SVG)**:
  - Kurva tren Bezier dengan riak gelombang harmonis naik-turun (*undulating wave*), otomatis jeda saat hover.
  - Bar chart volume layanan animasi horizontal.
  - Donut chart distribusi status order 360°.
  - Kartu utilisasi mesin radial gauge, stok inventaris peringatan, dan ringkasan keuangan.
  - Tabel order terbaru dengan *Quick Detail Modal*.

---

## 🚀 Menjalankan Proyek

```bash
# 1. Pasang dependensi
npm install

# 2. Jalankan development server
npm run dev

# 3. Build untuk production
npm run build
```

---

## 📖 Dokumentasi Desain
Silakan baca panduan lengkap pada [**`ui-desain.md`**](ui-desain.md) untuk detail properti komponen, slot, dan aturan rekayasa UI.
