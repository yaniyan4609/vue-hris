# AGENTS.md — Digaji.id (Vue 3 UI/UX Template & Dashboard)

Panduan untuk AI coding agent (Codex, OpenCode, Claude Code, Antigravity, dan lainnya).
Baca file ini sampai habis sebelum mengubah apa pun.

## Proyek

- **Digaji.id**: SaaS HRIS & Payroll multi-tenant. Repo ini adalah template UI/UX dan dashboard berbasis **Vue 3 (Vite, SPA)**.
- Komponen dasar buatan sendiri ada di `src/components/ui/` (prefix `Base`, tanpa library UI luar). Folder itu adalah **sumber kebenaran desain**.
- Bahasa antarmuka: **Bahasa Indonesia**.
- Stack: Vue 3 (`<script setup>`), Vite, `vue-router`, **Tailwind CSS v4**, ikon **`lucide-vue-next`**. Dark mode lewat class `dark` di `<html>`, tema warna lewat atribut `data-theme-color`.
- Impor memakai **path relatif** (mis. `../components/ui/BaseButton.vue`). Jangan memakai alias `@/` kecuali `vite.config.js` mengaturnya.

## Perintah

```bash
npm install      # pasang dependensi
npm run dev      # development server
npm run build    # build production (WAJIB lolos, exit code 0)
```

## Peta kode `src/`

- `components/ui/`: komponen Base (sumber kebenaran desain). Baca `defineProps` sebelum memakai komponen.
- `components/layout/`: `AppLayout`, `Header`, `PrimarySidebar`, `SubmenuSidebar`.
- `composables/`: `useAuth`, `useBackground`, `useIsMobile`, `useTheme`, `useThemeColor`, `useToast`. Pakai yang ada.
- `views/template/`: `DashboardView`, `DataKaryawanView`, `DataView`, `KanbanView`, `SettingView`, `KomponenView`. Contoh hidup: halaman data/tabel salin dari `DataKaryawanView`/`DataView`; dashboard dari `DashboardView`; katalog komponen di `KomponenView`.
- Halaman baru wajib didaftarkan di `router/index.js` **dan** `data/menuData.js`.
- `data/`, `router/`, `style.css`.

## Dokumen UI (wajib dibaca sebelum menyentuh UI)

1. **`aturan-ui.md`**: aturan, token warna, katalog komponen, larangan, alur keputusan, checklist.
2. **`desain-ui.md`**: katalog template dan contoh kode. Bagian 11 (Data View), 8.2 (halaman mengalir non-Data), 8.3 (pilih banyak + bulk bar), 9 (modal), 4.x (contoh tiap komponen). Salin pola dari sini, jangan menulis ulang dari nol.

Untuk tugas UI apa pun, urutannya:

1. Baca `aturan-ui.md`.
2. Cari pola yang cocok di `desain-ui.md`.
3. Buka file komponen di `src/components/ui/` yang akan dipakai dan baca `defineProps` / `defineEmits`-nya. Jangan menebak prop.
4. Tulis kode dengan pola dan komponen yang sudah ada.
5. Jalankan `npm run build`, lalu periksa checklist di akhir `aturan-ui.md`.

## Aturan inti (ringkasan)

- Pakai komponen `Base*` yang sudah ada. Jangan membuat komponen, varian, atau gaya baru kalau yang ada sudah cukup.
- Jangan menambah library UI, chart, atau ikon baru. Grafik dibuat dengan SVG murni.
- Halaman data/tabel (rute dengan `meta: { fixedHeight: true }`, mis. Contoh Data 1 dan 2) memakai container wajib: `h-full flex flex-col min-h-0 overflow-hidden gap-2.5 w-full max-w-7xl mx-auto select-none relative`. Halaman lain (Dashboard, Kanban, Setting) mengikuti view contohnya, lihat `aturan-ui.md` bagian 3.
- Warna aksen lewat `var(--theme-primary)` dan variabel tema lain (`src/style.css`). Dark mode tema Default memakai zinc netral (tanpa semburat biru). Uji hasil di 4 tema.
- Teks antarmuka berbahasa Indonesia. Nominal di form pakai `BaseCurrencyInput`. Notifikasi lewat `useToast()`, konfirmasi lewat `ConfirmationModal`.
- Jangan memakai `<select>` mentah atau hex zinc permukaan (`dark:bg-[#141417]` dan sejenisnya): pakai `BaseSearchableSelect` dan token tema (`var(--theme-surface)`, `var(--theme-border)`, dan lainnya). Kelemahan lain yang diketahui ada di `aturan-ui.md` bagian 8.
- Data contoh di dokumen (nama, angka, warna) hanya ilustrasi. Jangan di-hardcode ke aplikasi.

## Jika komponen atau pola yang dibutuhkan tidak ada

Jangan mengarang. Lakukan berurutan:

1. Cari lagi di `src/components/ui/` dan `desain-ui.md`.
2. Susun dari komponen `Base*` yang ada.
3. Jika benar-benar perlu komponen baru: buat di `src/components/ui/` dengan prefix `Base`, tiru struktur komponen yang paling mirip, dan pakai token yang sama.
4. Jika masih ragu, **berhenti dan tanyakan ke pengguna** daripada menebak.

## Definisi selesai

- `npm run build` lolos.
- Semua item di "Checklist Verifikasi" pada `aturan-ui.md` terpenuhi.
- Di laporan akhir, sebutkan komponen yang dipakai dan jelaskan setiap penyimpangan dari aturan (kalau ada) beserta alasannya.