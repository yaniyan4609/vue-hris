# Aturan UI Digaji.id (Vue 3 Base UI)

Dokumen ini berisi **aturan** untuk siapa pun (manusia atau AI agent) yang membuat atau mengubah UI di repo ini.
Template kode lengkap ada di [`desain-ui.md`](./desain-ui.md). Dokumen ini menjelaskan *kapan* dan *bagaimana* memakainya.

Seluruh isi dokumen ini sudah dicocokkan dengan kode di `src/` (props dibaca dari `defineProps`, token dari `style.css`, pola dari `views/` dan `AppLayout.vue`).

> **Kebenaran tertinggi adalah kode.** Jika dokumen ini atau `desain-ui.md` berbeda dengan file `.vue` / `.css` / `.js`, ikuti kode, lalu laporkan perbedaannya.

---

## 0. Prinsip kerja

1. **Sebelum memakai komponen, buka file `.vue`-nya dan baca `defineProps` serta `defineEmits`.** Tabel di bagian 6 adalah ringkasan, bukan pengganti.
2. **Salin pola dari view yang sudah ada.** Pilih view contoh berdasarkan jenis halaman (bagian 3). Jangan menulis tata letak dari nol.
3. **Tidak yakin = tanya.** Jangan mengarang prop, komponen, warna, atau gaya.

---

## 1. Stack & konvensi (terverifikasi)

| Hal | Nilai |
|---|---|
| Framework | Vue 3 (`<script setup>`), Vite, SPA, `vue-router` (history mode) |
| Styling | **Tailwind CSS v4** (`@import "tailwindcss"` di `src/style.css`) |
| Dark mode | Class `dark` pada `<html>` (`@custom-variant dark`). Gunakan `dark:` seperti biasa |
| Tema warna | Atribut `data-theme-color` pada `<html>`: `default`, `pink`, `blue`, `terracotta` |
| Ikon | **`lucide-vue-next`** (satu-satunya library selain `vue` dan `vue-router`) |
| Gaya impor | **Relatif**, mis. `import BaseButton from '../components/ui/BaseButton.vue'`. Contoh di `desain-ui.md` memakai alias `@/`, tetapi kode yang ada tidak memakainya. Gunakan impor relatif kecuali `vite.config.js` jelas mengatur alias `@` |
| Komponen dasar | `src/components/ui/*.vue`, prefix `Base`, tanpa library UI luar |
| Layout | `src/components/layout/`: `AppLayout`, `Header`, `PrimarySidebar`, `SubmenuSidebar` |
| Composable | `src/composables/`: `useAuth`, `useBackground`, `useIsMobile`, `useTheme`, `useThemeColor`, `useToast` |
| Data contoh | `src/data/` (`karyawanData`, `dashboardData`, `gunungJawaData`, `menuData`) |
| Menu & rute | Menu di `src/data/menuData.js` (saat ini: **Beranda** dan **Template** dengan sub: Contoh Kanban, Contoh Data 1, Contoh Data 2, dan 5 sub pengaturan), rute di `src/router/index.js` (`/template/*`; rute lama `/pesanan`, `/data`, `/setting` hanya redirect). Halaman baru perlu didaftarkan di **keduanya**. Halaman data/tabel wajib `meta: { fixedHeight: true }` |
| Helper format | `src/utils/format.js` mengekspor `formatRupiah` (Intl `id-ID`, IDR, tanpa desimal). Impor relatif, mis. `../utils/format.js`. Lihat bagian 8 (temuan) |
| Bahasa UI | Indonesia (label, placeholder, pesan, tooltip) |
| Perintah | `npm run dev`, `npm run build` |

---

## 2. Aturan emas

1. Pakai komponen `Base*` untuk semua fungsi yang sudah punya komponen. Jangan memakai `<button>`, `<select>`, `<input>`, atau modal mentah untuk fungsi tersebut. (Beberapa kode lama masih melanggar ini, lihat bagian 8. Jangan meniru pelanggarannya.)
2. Tidak ada library UI atau chart baru (Element Plus, Vuetify, PrimeVue, Bootstrap, Chart.js, ApexCharts, dan sejenisnya). Grafik dibuat dengan SVG murni.
3. Pilih pola container view sesuai jenis halaman (bagian 3 dan 4). Halaman data/tabel wajib memakai formula container.
4. Warna aksen **wajib** lewat variabel tema (`var(--theme-primary)` dan kawan-kawan, bagian 5). Jangan hardcode warna aksen.
5. Status selalu lewat `BaseBadge` dengan varian standar (bagian 5.3).
6. Konfirmasi aksi berbahaya lewat `ConfirmationModal`. Jangan memakai `alert()` atau `confirm()`.
7. Umpan balik hasil aksi (berhasil/gagal/info) lewat `useToast()`. `ToastContainer` sudah dipasang di `AppLayout`, jangan dipasang lagi.
8. Setiap fitur tabel punya versi desktop (`hidden md:flex`) **dan** mobile (`md:hidden`).
9. Nominal uang di formulir wajib `BaseCurrencyInput`. Pada teks statis pakai `.toLocaleString('id-ID')` atau `<BaseCountUp prefix="Rp " />`.
10. Jangan membuat modal terpisah untuk desktop dan mobile. Cukup `BaseModal` (otomatis menjadi bottom sheet di mobile).
11. Data contoh di dokumen dan di `src/data/` hanyalah ilustrasi. Jangan di-hardcode ke fitur baru.

---

## 3. Pilih pola halaman

AppLayout memiliki dua perilaku untuk area konten (`<main>`):
- Rute dengan `meta: { fixedHeight: true }` (halaman data/tabel, mis. Contoh Data 1 dan 2): `overflow-hidden`. Halaman harus mengunci tinggi sendiri dan scroll di dalam (formula container, bagian 4).
- Rute lain (Beranda, Contoh Kanban, halaman pengaturan): `overflow-y-auto`. Halaman boleh mengalir ke bawah dan di-scroll oleh `<main>`.

| Kebutuhan | Pola | View contoh (salin dari sini) |
|---|---|---|
| Daftar data (CRUD, filter, paginasi, pilih banyak) | **Data View** dengan formula container (bagian 4) | `src/views/template/DataKaryawanView.vue`, `src/views/template/DataView.vue` |
| Katalog komponen / playground master-detail | **Master-Detail View** (`meta.fixedHeight`): dua kolom scroll mandiri | `src/views/template/KomponenView.vue` |
| Dashboard / KPI / grafik SVG | Halaman mengalir: `space-y-4 sm:space-y-5 w-full max-w-7xl mx-auto select-none pb-8` | `src/views/template/DashboardView.vue` |
| Papan Kanban | Drag & drop HTML5 native | `src/views/template/KanbanView.vue` |
| Pengaturan | Halaman mengalir: `space-y-6 w-full max-w-4xl mx-auto` | `src/views/template/SettingView.vue` |
| Form tambah/ubah | `BaseModal` berisi komponen input Base | `DataKaryawanView.vue` (modal tambah/edit) |
| Detail data tanpa meninggalkan halaman | `BaseSlideOver` | `DataKaryawanView.vue`, `KanbanView.vue` |
| Konfirmasi hapus / simpan | `ConfirmationModal` | semua view data |
| Impor CSV | `BaseCsvImporter` | `DataKaryawanView.vue` |
| Unggah file | `BaseFileUploader` | `DataKaryawanView.vue` |
| Riwayat / audit log | `BaseTimeline` | `DataKaryawanView.vue`, `DashboardView.vue` |
| Faktur / cetak | `BaseInvoiceModal` | `KanbanView.vue` |
| Notifikasi hasil aksi | `useToast()` | semua view |
| Data kosong setelah filter | `BaseEmptyState` | `DataKaryawanView.vue` |
| Placeholder saat memuat | `BaseSkeleton` | `DashboardView.vue` |

Halaman baru di luar tabel ini: ikuti bagian 7 (jika pola tidak ada).

> Template kode: halaman data/tabel ada di `desain-ui.md` bagian 11 (formula) dan 8.3 (bulk bar). Halaman mengalir (non-Data) ada di bagian 8.2. Contoh modal di bagian 9, contoh komponen di bagian 4.

---

## 4. Formula container Data View

Dipakai oleh halaman data/tabel (rute dengan `meta.fixedHeight`). Pola ini sudah dipakai `DataKaryawanView.vue` dan `DataView.vue`.

```vue
<div class="h-full flex flex-col min-h-0 overflow-hidden gap-2.5 w-full max-w-7xl mx-auto select-none relative">
  <!-- 1. Header Modul (shrink-0) -->
  <!-- 2. Toolbar Pencarian & Filter (shrink-0) -->
  <!-- 3. Quick Stat Pills (shrink-0) -->
  <!-- 4. Tabel desktop: hidden md:flex flex-1 min-h-0 (scroll mandiri + thead sticky + paginasi shrink-0) -->
  <!-- 5. Kartu mobile: md:hidden flex-1 min-h-0 (scroll mandiri + paginasi compact shrink-0) -->
  <!-- 6. Modal (BaseModal, ConfirmationModal, dst.) -->
</div>
```

Aturan:
- Jarak antar strip memakai `gap-2.5`. Jangan memakai `space-y-*` di container ini.
- Semua strip atas `shrink-0`. Area tabel dan daftar kartu `flex-1 min-h-0` dengan scroll mandiri.
- Header modul: ikon 32x32 `rounded-xl`, judul, `BaseBadge` counter, tombol aksi di kanan (`outline sm` untuk ekspor/reset, `primary sm` untuk tambah). Label tombol disembunyikan di mobile (`hidden sm:inline`).
- Toolbar: input cari dengan tombol clear, lalu `BaseSearchableSelect` untuk filter. Filter paling kanan wajib `align="right"`.
- Stat pills: `grid grid-cols-2 sm:grid-cols-4 gap-2`, tinggi `h-9`. Warna ikon: netral (total), emerald (uang/sukses), sky (info), amber (peringatan).
- Tabel desktop: `thead` `sticky top-0 z-20` dengan `backdrop-blur-md`. Kolom berurutan: checkbox select-all, No, entitas utama, kolom data, status (`BaseBadge`), Aksi (tombol edit + `BaseDropdownMenu`). Nominal uang `font-mono`. Penomoran mengikuti paginasi.
- Kartu mobile: checkbox + identitas + badge status, sub-card detail, dua tombol aksi. Kartu terpilih memakai `ring-2 ring-[var(--theme-primary)]`.
- Floating Bulk Action Bar muncul saat `selectedIds.length > 0`: `fixed bottom-6 left-1/2 -translate-x-1/2 z-40`, dengan `<Transition>`.
- Kode lengkap tiap bagian: `desain-ui.md` bagian 8.3 dan 11, serta `DataKaryawanView.vue`.

Halaman non-data (Dashboard, Kanban, Setting) **tidak** memakai formula ini. Ikuti view contoh pada bagian 3.

---

## 5. Warna & tema

### 5.1. Lapisan & netral (default theme)

| Peran | Light | Dark |
|---|---|---|
| Dasar Pertama (backdrop) | `var(--theme-bg)` (`#f8fafc`) + pola grid/dots | `var(--theme-bg)` (`#09090b`) |
| Dasar Kedua (card utama) | `#ffffff`, border `border-2 border-neutral-300`, `rounded-2xl lg:rounded-3xl`, `shadow-xl` | `var(--theme-layer-2)` (Default `#121214`), border `var(--theme-border)` (`#27272a`) |
| Area konten `<main>` | `bg-neutral-50/60` | `var(--theme-layer-main)` (Default `#0c0c0e`) |
| Card / baris tabel | `#ffffff` | `var(--theme-surface)` (Default `#141417`), hover `var(--theme-surface-raised)` |
| Header tabel | `neutral-100/95` | `var(--theme-surface-raised)/95` (Default `#18181b`) |
| Teks utama / sekunder | `neutral-900` / `neutral-500` | `var(--theme-text-primary)` / `var(--theme-text-secondary)` |

Di dark mode, permukaan (Dasar Kedua, card, header, border, teks) **wajib memakai token tema**, bukan hex zinc. Nilai Default tetap zinc netral (tanpa semburat biru); tema pink, blue, dan terracotta mengikuti hue-nya sendiri. Dilarang: slate/gray biru gelap (`#0f172a`, `#111827`) dan gradient ungu-cyan.

### 5.2. Variabel tema (`src/style.css`)

Dipakai sebagai `var(--nama)`. Nilainya berubah otomatis mengikuti tema dan mode terang/gelap:

`--theme-primary`, `--theme-primary-brand`, `--theme-primary-hover`, `--theme-primary-pressed`, `--theme-primary-text`, `--theme-primary-soft`, `--theme-primary-soft-text`, `--theme-bg`, `--theme-layer-main`, `--theme-layer-2`, `--theme-surface`, `--theme-surface-raised`, `--theme-surface-muted`, `--theme-border`, `--theme-border-strong`, `--theme-text-primary`, `--theme-text-secondary`, `--theme-text-muted`.

| Tema | Primary (light) | Primary (dark) | Latar (light / dark) |
|---|---|---|---|
| default | `#18181b` | `#ffffff` | `#f8fafc` / `#09090b` |
| pink | `#C83D6A` (brand `#E85D88`) | `#E85D88` | `#FFF8FB` / `#140810` |
| blue | `#007AFF` | `#007AFF` | `#F8FAFC` / `#090e17` |
| terracotta | `#c03a18` | `#E05D3A` | `#F3F1EC` / `#15120f` |

Pakai variabel ini untuk aksen (tombol, link aktif, ring fokus, SVG, item terpilih). Contoh: `bg-[var(--theme-primary)] text-[var(--theme-primary-text)]`, `bg-[var(--theme-primary-soft)]`.

Kelas bantu di `style.css`: `bg-grid-pattern`, `bg-dots-pattern`, `anim-fade-up` (+ `anim-delay-1` sampai `5`), `skeleton-shimmer`.

### 5.3. Varian status badge

| Varian | Warna | Contoh |
|---|---|---|
| `success` | Emerald | Tetap, Aktif, Disetujui, Lunas |
| `warning` | Amber | Kontrak, Pending, Menunggu Persetujuan |
| `danger` | Rose | Ditolak, Suspended, Kritis |
| `info` | Sky | Probasi, Dalam Proses |
| `purple` | Violet | Enterprise, Pro |
| `neutral` | Zinc | Draft, Arsip |

---

## 6. Katalog komponen (terverifikasi dari `defineProps`)

Nilai default dicetak tebal. Semua `v-model` memakai `modelValue` / `update:modelValue` kecuali dicatat lain.

### Tombol, label, input

| Komponen | Props | Emit / slot |
|---|---|---|
| `BaseButton` | `variant`: **`primary`**, `secondary`, `outline`, `danger`, `ghost`. `size`: `sm`, **`md`**, `lg`. `disabled`. `type` (**`button`**). **Tidak ada prop `loading`** | slot default |
| `BaseBadge` | `variant`: **`neutral`**, `success`, `danger`, `warning`, `info`, `purple`. `size`: **`sm`**, `md` (**tidak ada `xs`**). `dot`, `pulse`, `removable` | emit `remove`. Slot default |
| `BaseFormField` | `label`, `required` (**false**), `error`, `hint`, `forId`, `helpText` (ikon bantuan dengan `BaseTooltip`) | slot default, `#label-extra` |
| `BaseInput` | `modelValue`, `label`, `required`, `type` (**`text`**), `placeholder`, `disabled`, `readonly`, `error`, `hint`, `leadingIcon`, `trailingIcon`, `prefix`, `suffix`, `clearable`, `size`: `sm`, **`md`**, `lg`, `currency` / `type="currency"` (mode lama, jangan dipakai: pakai `BaseCurrencyInput`), `inputClass` | `update:modelValue`, `clear`, `focus`, `blur`, `enter` |
| `BaseCurrencyInput` | `modelValue` (Number), `label`, `required`, `placeholder`, `prefix` (**`Rp`**), `suffix`, `disabled`, `readonly`, `error`, `hint`, `clearable` (**true**), `size`, `leadingIcon`, `terbilang`, `emitFormatted` | `update:modelValue`, `clear`, `focus`, `blur`, `enter` |
| `BaseNumberInput` | `modelValue` (Number, **0**), `label`, `required`, `min`, `max`, `step` (**1**), `prefix`, `suffix`, `disabled`, `readonly`, `error`, `hint`, `size`: `sm`, **`md`**, `lg`, `controls` (**true**) | `update:modelValue`, `change`, `focus`, `blur` |
| `BasePasswordInput` | `modelValue`, `label`, `required`, `placeholder`, `showStrength` (**false**), `disabled`, `error`, `hint`, `size`: `sm`, **`md`**, `lg` | `update:modelValue`, `focus`, `blur`, `enter` |
| `BaseMaskedInput` | `modelValue`, `preset`: **`nik`**, `npwp`, `phone`, `rekening`, `custom`. `mask`, `emitRaw` (**true**), `label`, `required`, `placeholder`, `leadingIcon`, `disabled`, `error`, `hint`, `size`: `sm`, **`md`**, `lg` | `update:modelValue`, `change`, `focus`, `blur` |
| `BaseTextarea` | `modelValue`, `label`, `required`, `placeholder`, `rows` (**3**), `maxlength`, `autoExpand` (**true**), `disabled`, `readonly`, `error`, `hint` | `update:modelValue`, `focus`, `blur` |
| `BaseSwitch` | `modelValue` (Boolean), `label`, `description`, `disabled`, `size`: `sm`, **`md`**, `lg` | `update:modelValue`, `change` |
| `BaseCheckbox` | `modelValue` (**`false`**, Boolean atau Array), `value`, `options` (Array opsi grup `{ value, label, description?, disabled? }`), `direction`: **`vertical`**, `horizontal`. `indeterminate`, `label`, `description`, `disabled`, `size`: `sm`, **`md`**, `lg` | `update:modelValue`, `change`. Slot default |
| `BaseRadio` | `modelValue`, `value`, `options` (Array opsi grup `{ value, label, description?, disabled? }`), `name`, `label`, `description`, `disabled`, `size`: `sm`, **`md`**, `lg`, `direction`: **`vertical`**, `horizontal` | `update:modelValue`, `change`. Slot default |
| `BaseMultiSelect` | `modelValue` (Array), `options` (primitif atau `{ value, label, badge?, icon?, disabled? }`), `asyncSearch` (Function async), `debounceMs` (**300**), `loading` (**false**), `cacheOptions`, `label`, `required`, `placeholder`, `searchPlaceholder`, `searchable` (**true**), `clearable` (**true**), `selectAll` (**true**), `maxChips` (**2**), `disabled`, `error`, `hint`, `size`: `sm`, **`md`**, `lg`, `align`: **`left`**, `right`, `leadingIcon`, `emptyText`, `widthClass` | `update:modelValue`, `change`, `clear`, `search` |
| `BaseSearchableSelect` | `modelValue`, `options` (primitif atau `{ value, label, badge? }`), `placeholder`, `searchPlaceholder`, `searchable` (**true**), `clearable`, `clearValue`, `disabled`, `size`: **`sm`**, `md`, `lg`, `align`: **`left`**, `right`, `leadingIcon`, `emptyText`, `widthClass` | `update:modelValue`, `change`, `clear` |
| `BaseDatePicker` | `modelValue` (ISO `YYYY-MM-DD`), `range`, `label`, `placeholder`, `presets` (**true**), `clearable` (**true**), `size`: `sm`, **`md`**, `placement`: **`auto`**, `left`, `right` (**bukan `align`**) | `update:modelValue`, `change` |
| `BaseTimePicker` | `modelValue` (String `HH:mm` atau `{ start, end }`), `range` (**false**), `label`, `required`, `placeholder`, `minuteStep` (**5**), `presets`, `clearable` (**true**), `disabled`, `error`, `hint`, `size`: `sm`, **`md`**, `placement`: **`auto`**, `left`, `right` | `update:modelValue`, `change`, `clear` |
| `BaseSegmentedControl` | `modelValue`, `options` (string atau `{ label, value, icon?, badge? }`), `size`: `sm`, **`md`**, `fullWidth` | `update:modelValue`, `change` |
| `BaseFileUploader` | `modelValue` (Array), `label`, `hint`, `accept` (**`*`**), `maxSizeMB` (**5**), `multiple` (**true**), `disabled` | `update:modelValue`, `change`, `error` |

### Dialog & panel

| Komponen | Props | Emit / slot |
|---|---|---|
| `BaseModal` | `modelValue`, `title`, `subtitle`, `size`: `sm`, **`md`**, `lg`, `xl`, `full`, `closeOnBackdrop`, `closeOnEsc`, `adaptive` (**true**), `breakpoint` (**768**) | `update:modelValue`, `close`. Slot: default, `header`, `footer` (`{ close }`) |
| `MobileModal` | `modelValue`, `title`, `subtitle`, `closeOnBackdrop` | Dipakai otomatis oleh `BaseModal`. Jangan dipanggil langsung di view |
| `ConfirmationModal` | `modelValue`, `type`: **`save`**, `delete`, `warning`, `success`. `badgeText`, `title`, `message`, `confirmText`, `cancelText` (**`Batal`**) | `update:modelValue`, `confirm`, `cancel`, `close` |
| `BaseSlideOver` | `modelValue`, `title`, `subtitle`, `size`: `sm`, **`md`**, `lg`, `xl`, `closeOnEsc`, `closeOnClickOutside` | `update:modelValue`, `close`. Slot: default, `badge`, `header`, `footer` (`{ close }`) |
| `BasePopover` | `modelValue`, `placement`: **`bottom-end`**, `bottom-start`, `top-end`, `top-start`. `widthClass` (**`w-80`**), `trigger`: **`click`**, `hover`. `closeOnEsc` (**true**), `closeOnClickOutside` (**true**) | `update:modelValue`, `open`, `close`. Slot: `#trigger`, default, `#header`, `#footer` |
| `BaseTooltip` | `text`, `placement`: **`top`**, `bottom`, `left`, `right`. `delay` (**120**), `maxWidthClass` (**`max-w-xs`**) | Slot: default, `#content` |
| `BaseInvoiceModal` | `modelValue`, `invoiceData` (Object) | `update:modelValue` |
| `BaseCsvImporter` | `modelValue`, `title`, `targetFields` (`[{ key, label, required }]`) | `update:modelValue`, `imported` |
| `BaseCommandPalette` | `modelValue` | `update:modelValue`, `action`. Sudah dipasang global di `AppLayout` |
| `ToastContainer` | props: `[]` | Kontainer toast global, dipasang di `AppLayout` |

### Data & status

| Komponen | Props | Emit / slot |
|---|---|---|
| `BaseDataTable` | `columns` (wajib, `[{ key, label, sortable?, align?, width?, headerClass?, cellClass? }]`), `rows` (wajib), `totalItems`, `rowKey` (**`id`**), `selectable`, `selectedIds` (`v-model:selected-ids`), `showIndex` (**true**), `sortKey` (`v-model:sort-key`), `sortOrder` (`v-model:sort-order`, **`asc`**), `currentPage` (`v-model:current-page`, **1**), `itemsPerPage` (`v-model:items-per-page`, **15**), `perPageOptions`, `paginated` (**true**), `serverSide` (**false**), `loading`, `error`, `errorTitle`, `emptyTitle`, `emptyDescription`, `emptyActionText`, `emptyIcon`, `selectedLabel`, `compactMobilePagination` | emit `sort`, `change`, `retry`, `row-click`, `empty-action`. Slot: `#cell-{key}`, `#header-{key}`, `#actions`, `#mobile-card`, `#bulk-actions` |
| `BaseAlert` | `variant`: **`info`**, `success`, `warning`, `danger`, `neutral`. `title`, `description`, `icon`, `dismissible` (**false**), `banner` (**false**), `actionText` | emit `dismiss`, `action`. Slot: default, `#actions` |
| `BaseAvatar` | `src`, `name`, `initials`, `size`: `xs`, `sm`, **`md`**, `lg`, `xl`. `shape`: **`rounded`**, `circle`. `status`: `''`, `online`, `offline`, `busy`, `away`. `tone`: `primary`, **`neutral`**, `emerald`, `sky`, `amber`, `rose`, `purple` | n/a |
| `BaseStatCard` | `title` (wajib), `value`, `secondaryValue`, `prefix`, `suffix`, `unit`, `decimals`, `icon`, `tone`: **`sky`**, `primary`, `emerald`, `amber`, `indigo`, `cyan`, `rose`, `neutral`. `trend`, `trendDirection`: **`up`**, `down`, `neutral`. `trendLabel`, `subtitle`, `loading` (**false**) | n/a |
| `BaseTabs` | `modelValue` (wajib), `tabs` (wajib, `[{ value, label, icon?, badge?, badgeVariant?, disabled? }]`), `variant`: **`underline`**, `pills`, `cards`. `size`: `sm`, **`md`**. `fullWidth` (**false**) | `update:modelValue`, `change`. Slot: default, `#panel-{value}`, `#actions` |
| `BaseStepper` | `steps` (wajib, `[{ id, title, subtitle?, status?: 'completed'|'current'|'upcoming'|'rejected', approver?, timestamp? }]`), `currentStep` (**0**), `clickable` (**false**), `size`: `sm`, **`md`** | `update:currentStep`, `step-click` |
| `BaseProgress` | `value` (**0**), `max` (**100**), `label`, `showValue` (**true**), `valueFormat`: **`percent`**, `fraction`. `variant`: **`primary`**, `success`, `warning`, `danger`, `info`. `size`: `sm`, **`md`**, `lg`. `striped`, `indeterminate` | n/a |
| `BaseAccordion` | `items` (wajib, `[{ id, title, subtitle?, content?, icon?, badge?, disabled? }]`), `modelValue`, `multiple` (**false**), `defaultOpen` | `update:modelValue`, `change`. Slot: `#item-{id}`, `#content` |
| `BaseBreadcrumb` | `items` (wajib, `[{ label, to?, icon?, active? }]`), `showHome` (**true**) | emit `navigate` |
| `BaseSelectableCard` | `modelValue`, `value`, `multiple` (**false**), `title`, `description`, `badge`, `badgeVariant`: **`neutral`**, `icon`, `disabled` | `update:modelValue`, `change`. Slot: default, `#footer`, `#right` |
| `BaseDescriptionList` | `items` (wajib, `[{ key?, label, value, mono?, badge?, badgeVariant?, span? }]`), `columns`: `1`, **`2`**, `3`. `variant`: **`cards`**, `divided` | Slot: `#value-{key}` |
| `BaseCalendar` | `modelValue` (ISO `YYYY-MM-DD`), `events` (`[{ id, date, endDate?, title, variant?, badge? }]`), `holidays` (`[{ date, label }]`), `selectable` (**true**) | `update:modelValue`, `date-click`, `event-click`, `month-change`. Slot: `#event`, `#day-extra` |
| `BaseCameraCapture` | `modelValue` (Base64 data URL), `facingMode`: **`user`**, `environment`. `aspectRatio`: `square`, **`portrait`**, `landscape`. `watermarkText`, `mirrored` (**true**), `label` | `update:modelValue`, `capture`, `error`, `permission-denied` |
| `BaseMapPolygonEditor` | `modelValue` (Array multi-polygon), `center`, `zoom` (**16**), `editable` (**true**), `height` (**`360px`**), `label` | `update:modelValue`, `change` |
| `BaseTree` | `nodes`, `modelValue`, `selectable` (**false**), `multiple` (**false**), `cascade` (**true**), `searchQuery`, `defaultExpandAll` (**true**) | `update:modelValue`, `select`, `toggle` |
| `BaseTreeSelect` | `modelValue`, `nodes`, `multiple` (**false**), `cascade` (**true**), `label`, `required` (**false**), `placeholder`, `searchable` (**true**), `clearable` (**true**), `size`: `sm`, **`md`**, `lg`. `error`, `hint` | `update:modelValue`, `change`, `clear` |
| `BasePagination` | `currentPage` (wajib), `totalItems` (wajib), `itemsPerPage` (**15**, angka atau `'all'`), `perPageOptions` (**`[15, 30, 50, 'all']`**), `compact`. **Tidak ada prop `totalPages`** | `update:currentPage`, `update:itemsPerPage`, `change` |
| `BaseDropdownMenu` | `items` (`{ label, icon?, action?, danger?, disabled?, divider? }`), `align`: `left`, **`right`**, `triggerClass` | emit `select`. Slot: `trigger`, default |
| `BaseEmptyState` | `title`, `description`, `icon`, `actionText`, `actionIcon`, `compact` | emit `action`. Slot: `icon`, `title`, `description`, `action` |
| `BaseSkeleton` | `variant`: **`text`**, `circular`, `rectangular`, `card`. `width`, `height` (string dengan satuan), `lines`, `shimmer` | n/a |
| `BaseCountUp` | `to` (wajib), `from`, `duration` (**1200**), `decimals`, `prefix`, `suffix`, `formatNumber` (**true**), `autoplay` | n/a |
| `BaseTimeline` | `items` (`{ id, title, description, time, author, variant, icon, badge }`), `compact`. Varian node: `primary`, `success`, `warning`, `danger`, `info`, `neutral` | slot default |
| `BaseToast` | `toast` (Object) | emit `close`. Dipakai internal oleh `ToastContainer` |

### Composable

| Composable | Kegunaan |
|---|---|
| `useToast()` | `success(message, title?, options?)`, `error(...)`, `warning(...)`, `info(...)`, `loading(message)`, `promise(promiseOrFn, { loading, success, error })`, `update`, `remove`, `clear` |
| `useTheme()` | Mode gelap/terang (`isDark`, `toggleTheme`) dengan transisi lingkaran |
| `useThemeColor()` | Palet aksen (`themeColor`, `setThemeColor`: `default`, `pink`, `blue`, `terracotta`) |
| `useBackground()` | Dasar Pertama (`bgType`: `grid`, `dots`, `solid`, `image`) |
| `useIsMobile(breakpoint = 768)` | Deteksi layar mobile reaktif |
| `useAuth()` | Autentikasi in-place (login morphing di `AppLayout`) |

---

## 7. Jika komponen atau pola yang dibutuhkan tidak ada

Urutan wajib:

1. Cari lagi di `src/components/ui/`, view contoh (bagian 3), dan `desain-ui.md`.
2. Susun dari komponen `Base*` yang ada, dengan token tema.
3. Jika benar-benar perlu komponen baru: buat di `src/components/ui/` dengan prefix `Base`, tiru komponen terdekat (struktur props, dark mode, variabel tema), dan catat di laporan akhir.
4. Jika masih ragu: **berhenti dan tanyakan ke pengguna.**

---

## 8. Hal yang diketahui belum konsisten di kode (jangan ditiru)

Kode sudah berjalan dan tidak diubah oleh dokumen ini. Tujuannya agar agent tidak menyalin kelemahan yang ada:

1. ~~`<select>` mentah~~ **sudah diganti** dengan `BaseSearchableSelect` di `DataKaryawanView.vue`, `DataView.vue`, dan `KanbanView.vue`. Pola di form modal: `size="md"`, daftar pendek (<= 6 opsi) `:searchable="false"`, kolom paling kanan `align="right"`. Jangan kembali memakai `<select>`.
2. **Hex hardcode** untuk permukaan dark sudah diganti token tema di layout, `components/ui/`, dan `views/`. Jangan menulis hex zinc baru (`dark:bg-[#141417]` dan sejenisnya); pakai `dark:bg-[var(--theme-surface)]`. Hex yang sah hanya swatch palet di `SettingView`, warna logo, dan CSS cetak invoice.
3. **`formatRupiah` sebagian dibagi.** `DataKaryawanView` sudah memakai `src/utils/format.js`. `BaseInvoiceModal` (`formatRp`) dan `BaseInput` (`formatCurrencyString`) sengaja **belum** diganti karena outputnya berbeda (spasi biasa vs NBSP, format angka negatif, desimal; `BaseInput` hanya mengembalikan digit bertitik tanpa `Rp`). Jangan menambah salinan baru. Tanyakan dulu sebelum menyeragamkannya.
4. **Tema non-default di dark mode sudah memakai token.** Layer: `--theme-bg` (Dasar Pertama) < `--theme-layer-main` (`<main>`) < `--theme-layer-2` (Dasar Kedua) < `--theme-surface` (card) < `--theme-surface-raised` < `--theme-surface-muted` (hover). Nilai dark pink/blue/terracotta di `style.css` adalah turunan yang perlu dinilai visual; ubah di `style.css`, bukan di komponen. Mode terang non-default masih memakai kelas netral Tailwind (belum ditokenkan).
5. **`EssLayout.vue` tidak ada.** Bagian ESS di dokumen lama belum punya implementasi. Jika tugas menyangkut ESS, konfirmasi dulu.

---

## 9. Dilarang → gunakan ini

| Dilarang | Gunakan |
|---|---|
| Menebak prop komponen | Baca `defineProps` di file `.vue` |
| `:loading` pada `BaseButton`, `size="xs"` pada `BaseBadge`, `align` pada `BaseDatePicker`, `:total-pages` pada `BasePagination`, `endVal` pada `BaseCountUp`, `type`/`count` pada `BaseSkeleton` | Prop yang benar di bagian 6 |
| Memakai `BaseTag` | Komponen ini **tidak ada**. Pakai `BaseBadge` |
| `space-y-*` di container Data View | `gap-2.5` |
| Scroll di Dasar Kedua pada halaman Data | Scroll mandiri di area tabel / daftar kartu |
| `<button>`, `<select>`, modal mentah untuk fungsi yang sudah ada komponennya | Komponen `Base*` |
| `alert()` / `confirm()` | `ConfirmationModal`, `useToast()` |
| Library UI, chart, atau ikon baru | Komponen Base, SVG murni, `lucide-vue-next` |
| Warna aksen hardcode | `var(--theme-primary)` dan kawan-kawan |
| Gradient ungu-cyan, glow neon | Warna solid dari token |
| Badge dengan warna improvisasi | `BaseBadge` varian standar |
| Dropdown filter paling kanan tanpa `align="right"` | `align="right"` |
| Hanya membuat versi desktop | Desktop + kartu mobile |
| Membuat modal terpisah desktop/mobile | `BaseModal` (adaptif) |
| Impor `@/...` tanpa memastikan alias | Impor relatif |
| Teks UI berbahasa Inggris | Bahasa Indonesia |
| Menyalin data contoh ke fitur baru | Data dari state atau API |

---

## 10. Checklist verifikasi (sebelum menyatakan selesai)

**Tata letak**
- [ ] Pola container sesuai jenis halaman (bagian 3). Halaman data memakai formula bagian 4 tanpa modifikasi.
- [ ] Tidak ada scrollbar ganda. Semua strip atas `shrink-0`, area tabel `flex-1 min-h-0`.
- [ ] Paginasi desktop dan mobile terkunci di dasar (`shrink-0`).
- [ ] Halaman baru terdaftar di `src/router/index.js` dan `src/data/menuData.js`.

**Tabel & mobile**
- [ ] Ada versi desktop (`hidden md:flex`) dan mobile (`md:hidden`).
- [ ] Select-all, checkbox baris, dan checkbox kartu mobile berfungsi. Kartu terpilih memiliki ring tema.
- [ ] Bulk Action Bar muncul saat ada item terpilih. `thead` sticky. Penomoran mengikuti paginasi.

**Komponen & gaya**
- [ ] Hanya komponen `Base*` yang ada, dengan prop yang sudah dicek di file `.vue`-nya.
- [ ] Tidak ada library baru di `package.json`.
- [ ] Aksen memakai `var(--theme-*)`. Tidak ada hex permukaan baru; pakai token tabel 5.1/5.2. Dark mode Default tetap zinc netral.
- [ ] Status memakai varian `BaseBadge` standar. Filter paling kanan memakai `align="right"`.
- [ ] Konfirmasi hapus memakai `ConfirmationModal`. Hasil aksi lewat `useToast()`.
- [ ] Tampilan benar di **keempat tema** dan mode terang/gelap.

**Konten & aksesibilitas**
- [ ] Semua teks UI berbahasa Indonesia. Nominal di form memakai `BaseCurrencyInput`.
- [ ] Tombol ikon punya `title` atau `aria-label`. Elemen klik punya `cursor-pointer` dan focus ring.
- [ ] Tidak ada data contoh yang ter-hardcode.

**Build**
- [ ] `npm run build` lolos dengan exit code 0.

**Laporan akhir**
- [ ] Menyebut komponen yang dipakai, penyimpangan (jika ada) beserta alasannya, dan hal yang perlu ditanyakan.