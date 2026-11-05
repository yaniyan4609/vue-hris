# Skema inti identitas dan akses

Migrasi `backend/migrations/00001_core_identity_access.sql`, `00002_platform_org_employee.sql`, dan `00003_admin_access_audit.sql` telah diterapkan pada database staging MariaDB 10.11 di server Product Owner. Versi Goose: 3. Migrasi kedua menambah role platform, struktur organisasi, dan karyawan. Migrasi ketiga menambah permission organisasi/karyawan, audit write, dan riwayat penugasan awal. Seed demo membuat satu tenant, tiga user, role, membership, dan satu data karyawan ESS.

## Relasi utama

```text
users ──< tenant_memberships >── tenants
  │                │                 │
  └──< sessions    ├──< user_roles >── roles
                   │                    └──< role_permissions >── permission_registry
                   ├──< user_permission_overrides      │
                   └──< user_scope_overrides            └──< role_scope_rules ──< role_scope_values
                            └──< user_scope_rules ──< user_scope_values
```

- User bersifat global; satu email unik dapat memiliki banyak `tenant_memberships`.
- Tabel milik tenant memakai `tenant_id` di primary/composite key dan foreign key gabungan sehingga hubungan role, grant, dan scope tidak dapat menunjuk ke tenant lain.
- `sessions` menyimpan hanya hash token 32 byte, bukan token mentah. `selected_tenant_id` boleh kosong sampai user memilih tenant setelah login; foreign key gabungan memastikan tenant yang dipilih merupakan keanggotaan user. Status keanggotaan tetap diperiksa oleh layanan pada setiap request.
- `user_roles` menyimpan periode berlaku. `role_permissions` mengacu ke registry global. Override izin `allow` atau `deny` disimpan terpisah dari penggantian scope.
- Satu baris `user_scope_overrides` berarti scope role diganti. Bila belum ada aturan di bawahnya, akses scoped gagal tertutup; menghapus header mengembalikan scope role.
- Aturan scope dihubungkan dengan OR. Nilai di satu dimensi beroperasi OR, beberapa dimensi dalam satu aturan beroperasi AND. `kind` khusus: seluruh tenant, diri sendiri, bawahan langsung, atau filter organisasi.
- Nilai node organisasi pada `*_scope_values` belum memiliki foreign key ke region/office/division/department/position. Tabel organisasinya sudah tersedia, tetapi layanan penulisan scope tetap wajib memvalidasi node dan tenant; pembacaan daftar wajib menerjemahkan scope ke SQL berfilter tenant.
- Endpoint Admin untuk organisasi dan karyawan memakai evaluator permission saat request, `tenant_id` dari sesi, filter SQL pada daftar/detail karyawan, serta audit dalam transaksi perubahan. Daftar memakai paginasi server dan batas maksimal 100 item per halaman.
- `employee_assignment_history` merekam penempatan awal dan perubahan kantor/jabatan/atasan. Model ini masih lebih sempit dari usulan `employee_assignments` bertanggal dalam dokumen database lengkap; penyelarasan dilakukan melalui migrasi aditif. Status resign/terminasi belum bisa diubah langsung karena belum ada alur tanggal efektif dan pencabutan sesi/role.

## Batas pada tahap ini

Login dan sesi sudah tersedia di API, termasuk pilihan tenant dan logout. `panel.admin` sudah ada di registry dan diperiksa saat membentuk state sesi, sedangkan akses ESS mensyaratkan karyawan aktif. Penegakan permission/scope sudah terhubung untuk endpoint bisnis awal, tetapi registry masih parsial dan entitlement paket belum tersedia. Audit baru mencatat perubahan organisasi/karyawan. Role/permission/scope belum memiliki UI manajemen dan audit perubahan. Fitur bisnis baru harus menambahkan permission registry, tenant context, scope, entitlement, dan audit sesuai PRD sebelum endpoint-nya dibuka.

Kredensial tidak tersimpan dalam repo. `cmd/migrate` menerima konfigurasi `DB_ADDR`, `DB_USER`, `DB_NAME`, opsional `DB_TLS`, dan membaca kata sandi secara tersembunyi dari terminal bila `DB_PASSWORD` belum diberikan.
