# Skema inti identitas dan akses

Migrasi `backend/migrations/00001_core_identity_access.sql` dan `00002_platform_org_employee.sql` telah diterapkan pada database staging MariaDB 10.11 pada 4 November 2026. Versi Goose: 2. Migrasi kedua menambah role platform, struktur organisasi, dan karyawan. Seed demo membuat satu tenant, tiga user, role, membership, dan satu data karyawan ESS.

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

## Batas pada tahap ini

Login dan sesi sudah tersedia di API, termasuk pilihan tenant dan logout. `panel.admin` sudah ada di registry dan diperiksa saat membentuk state sesi, sedangkan akses ESS mensyaratkan karyawan aktif. Belum tersedia penegakan permission/scope umum di seluruh HTTP API, entitlement paket, audit log, atau filter SQL untuk endpoint bisnis. Evaluator akses di Go sudah memiliki pengujian unit, tetapi belum dihubungkan ke data MariaDB. Endpoint bisnis tidak boleh dibuka sebelum lapisan tersebut siap.

Kredensial tidak tersimpan dalam repo. `cmd/migrate` menerima konfigurasi `DB_ADDR`, `DB_USER`, `DB_NAME`, opsional `DB_TLS`, dan membaca kata sandi secara tersembunyi dari terminal bila `DB_PASSWORD` belum diberikan.
