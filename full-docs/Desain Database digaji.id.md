# Desain Database digaji.id — MySQL 8

**Turunan PRD v2.3** · 4 November 2026 · Draft \[USULAN\]

> **Tujuan:** rancangan skema database untuk SaaS HRIS multi-tenant: konvensi, daftar tabel per domain, kolom penting, key dan index, relasi, serta DDL lengkap untuk inti akses (user, role, permission, scope, sesi). Ini **rancangan awal** untuk ditinjau. ERD fisik, constraint, dan migrasi final dibuat per fase. Menu dan fungsi mengacu ke dokumen **Matriks Menu, Role & Fungsional**.

## 1. Prinsip dan konvensi

| Hal | Aturan |
| --- | --- |
| Engine, charset | InnoDB, `utf8mb4`, collation `utf8mb4_0900_ai_ci` |
| Primary key | `id BIGINT UNSIGNED AUTO_INCREMENT`. ID internal tidak diekspos ke API |
| ID publik | `public_id CHAR(26)` (ULID) untuk URL dan API, `UNIQUE` |
| Tenant | `tenant_id BIGINT UNSIGNED NOT NULL` pada **semua tabel bisnis**. Selalu kolom **pertama** pada index komposit |
| FK aman tenant | Tabel induk punya `UNIQUE (tenant_id, id)`. Tabel anak memakai **FK komposit** `(tenant_id, parent_id)`. Database sendiri menolak relasi lintas tenant |
| Waktu | `DATETIME(3)` dalam **UTC**. `DATE` untuk tanggal kerja dan tanggal efektif. Zona waktu kantor di `offices.timezone` (nama IANA) |
| Uang | `DECIMAL(18,2)`. Tarif `DECIMAL(9,6)`. **Dilarang** `FLOAT`/`DOUBLE` |
| Boolean | `TINYINT(1)` |
| Status/enum | `VARCHAR(20-30)` + `CHECK`. Lebih mudah diubah daripada `ENUM` |
| Soft delete | `deleted_at` hanya pada master (karyawan, kantor, jabatan). Tabel transaksi final tidak dihapus |
| Effective dating | `valid_from DATE`, `valid_to DATE NULL` (NULL = masih berlaku). Tumpang tindih dicegah aplikasi dalam transaksi dengan lock |
| Kolom audit | `created_at`, `updated_at`, `created_by`, `updated_by` (id membership) |
| Tabel immutable | `attendance_events`, `audit_logs`, `approval_actions`, snapshot payroll: **INSERT-only**. Perbaikan lewat baris baru |
| Penamaan | `snake_case`, tabel jamak, FK bernama `<entitas>_id` |
| Migrasi | golang-migrate atau goose. Satu perubahan per file, ada rollback |
| Partisi | Tidak di awal. MySQL **tidak mendukung FK pada tabel terpartisi**. Untuk `audit_logs` dan `notifications` pertimbangkan arsip bulanan tanpa FK |

### Pola FK komposit (isolasi tenant)

```sql
-- Induk
CREATE TABLE departments (
  id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  tenant_id BIGINT UNSIGNED NOT NULL,
  division_id BIGINT UNSIGNED NOT NULL,
  name VARCHAR(120) NOT NULL,
  PRIMARY KEY (id),
  UNIQUE KEY uq_dept_tenant_id (tenant_id, id),
  -- anak menunjuk induk lewat (tenant_id, id), sehingga lintas tenant mustahil
  CONSTRAINT fk_dept_division FOREIGN KEY (tenant_id, division_id)
    REFERENCES divisions (tenant_id, id)
);
```

Pola ini **tidak menggantikan** penyuntikan `tenant_id` di lapisan data access (PRD 2.1). Ia adalah lapisan pengaman kedua bila ada query yang lolos.

## 2. Peta domain

| Bagian | Domain | Tabel utama |
| --- | --- | --- |
| 3 | Platform & SaaS | `tenants`, `plans`, `subscriptions`, `invoices`, `payments`, `support_*` |
| 4 | Identitas & akses | `users`, `tenant_memberships`, `sessions`, `roles`, `permission_registry`, `user_roles`, `access_scope_rules` |
| 5 | Organisasi | `regions`, `offices`, `office_geofences`, `divisions`, `departments`, `positions` |
| 6 | Kepegawaian | `employees`, `employee_assignments`, `contracts`, `employee_changes`, `terminations` |
| 7 | Kehadiran & jadwal | `shift_templates`, `work_schedules`, `shift_instances`, `attendance_events`, `holidays` |
| 8 | Cuti & lembur | `leave_types`, `leave_requests`, `leave_balance_ledger`, `overtime_requests` |
| 9 | Approval | `requests`, `approval_flows`, `approval_instances`, `approval_tasks`, `approval_actions` |
| 10 | Kasbon & reimbursement | `employee_loans`, `loan_installments`, `reimbursements` |
| 11 | Payroll | `payroll_periods`, `payroll_runs`, `payroll_items`, `payslips`, `tax_rules` |
| 12 | Notifikasi & broadcast | `notifications`, `notification_outbox`, `broadcasts`, `broadcast_recipients` |
| 13 | File & dokumen | `files`, `employee_documents`, `company_documents` |
| 14 | Audit, impor/ekspor, pengaturan | `audit_logs`, `import_jobs`, `export_jobs`, `tenant_settings` |

### Relasi inti (ringkas)

```text
tenants ──< tenant_memberships >── users
                 │ └── employee_id ──▸ employees
                 ├──< user_roles >── roles >──< role_permissions >── permission_registry
                 ├──< user_permission_overrides >── permission_registry
                 └──< access_scope_rules ──< access_scope_rule_items   (juga milik roles)

regions ──< offices ──< office_geofences
divisions ──< departments ──< positions
employees ──< employee_assignments >── (region, office, division, department, position)
employees ──< work_schedules >── shift_templates ; work_schedules ──< shift_instances ──< attendance_events
```

## 3. Platform dan SaaS

| Tabel | Kolom penting | Key / index | Catatan |
| --- | --- | --- | --- |
| `tenants` | id, public\_id, name, slug, status, plan\_id, timezone, trial\_ends\_at, created\_at | PK id. UNIQUE public\_id, slug. CHECK status | DDL di bawah. Status: `pending, trial, active, grace, suspended, closed` |
| `plans` | id, code, name, price\_monthly, price\_yearly, max\_employees, max\_users, max\_offices, max\_storage\_mb, is\_active | UNIQUE code | Nama paket tidak di-hardcode |
| `plan_entitlements` | plan\_id, entitlement\_key, limit\_value NULL | PK (plan\_id, entitlement\_key) | Contoh key: `core_hr`, `attendance`, `payroll`, `broadcast` |
| `subscriptions` | id, tenant\_id, plan\_id, status, billing\_cycle, period\_start, period\_end, auto\_renew, canceled\_at | idx (tenant\_id, status) | Satu langganan aktif per tenant |
| `invoices` | id, tenant\_id, subscription\_id, invoice\_no, amount, tax, total, status, due\_at, paid\_at | UNIQUE invoice\_no. idx (tenant\_id, status, due\_at) |  |
| `payments` | id, tenant\_id, invoice\_id, gateway, gateway\_ref, amount, status, paid\_at | UNIQUE (gateway, gateway\_ref) | Idempotensi pembayaran |
| `payment_webhook_events` | id, gateway, event\_id, payload JSON, processed\_at | UNIQUE (gateway, event\_id) | Cegah webhook diproses dua kali |
| `tenant_usage` | tenant\_id, period (YYYY-MM), employees\_count, users\_count, storage\_mb | PK (tenant\_id, period) | Dasar penegakan kuota |
| `platform_staff` | id, user\_id, platform\_role, status | UNIQUE user\_id | Role: `SUPERADMIN, ADMIN_PLATFORM, SUPPORT, FINANCE_PLATFORM` |
| `support_tickets`, `support_ticket_messages` | id, tenant\_id NULL, subject, status, assignee\_id; message: ticket\_id, author, body | idx (status, assignee\_id) |  |
| `support_sessions` | id, staff\_id, tenant\_id, reason, started\_at, expires\_at, ended\_at | idx (tenant\_id, started\_at) | Akses platform ke data tenant. Wajib alasan dan masa terbatas |
| `platform_settings` | key, value JSON | PK key | Brand, SMTP, integrasi |

## 4. Identitas dan akses (inti keamanan)

Tabel ini mengimplementasikan PRD Bagian 3 dan 10. Perbedaan penamaan dari PRD 9.3: `role_scope_presets` dan `user_scope_assignments` digabung menjadi satu tabel **`access_scope_rules`** (pemilik: role atau membership). `access_versions` dilebur menjadi kolom `tenant_memberships.access_version`.

### 4.1 DDL inti

```sql
CREATE TABLE tenants (
  id            BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  public_id     CHAR(26)        NOT NULL,
  name          VARCHAR(150)    NOT NULL,
  slug          VARCHAR(63)     NOT NULL,
  status        VARCHAR(20)     NOT NULL DEFAULT 'pending',
  plan_id       BIGINT UNSIGNED NULL,
  timezone      VARCHAR(40)     NOT NULL DEFAULT 'Asia/Jakarta',
  trial_ends_at DATETIME(3)     NULL,
  created_at    DATETIME(3)     NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  updated_at    DATETIME(3)     NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
  PRIMARY KEY (id),
  UNIQUE KEY uq_tenants_public (public_id),
  UNIQUE KEY uq_tenants_slug (slug),
  CONSTRAINT ck_tenants_status CHECK (status IN ('pending','trial','active','grace','suspended','closed'))
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

CREATE TABLE users (
  id                  BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  public_id           CHAR(26)        NOT NULL,
  email               VARCHAR(190)    NOT NULL,
  password_hash       VARCHAR(255)    NOT NULL,
  full_name           VARCHAR(150)    NOT NULL,
  phone               VARCHAR(30)     NULL,
  status              VARCHAR(20)     NOT NULL DEFAULT 'active',
  password_changed_at DATETIME(3)     NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  last_login_at       DATETIME(3)     NULL,
  created_at          DATETIME(3)     NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  updated_at          DATETIME(3)     NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
  PRIMARY KEY (id),
  UNIQUE KEY uq_users_public (public_id),
  UNIQUE KEY uq_users_email (email),
  CONSTRAINT ck_users_status CHECK (status IN ('active','disabled'))
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

-- Satu akun bisa menjadi anggota banyak tenant. Role dan scope melekat ke membership.
CREATE TABLE tenant_memberships (
  id             BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  tenant_id      BIGINT UNSIGNED NOT NULL,
  user_id        BIGINT UNSIGNED NOT NULL,
  employee_id    BIGINT UNSIGNED NULL,          -- NULL untuk akun non-karyawan (mis. owner)
  status         VARCHAR(20)     NOT NULL DEFAULT 'active',
  access_version BIGINT UNSIGNED NOT NULL DEFAULT 1,  -- naik setiap hak akses berubah
  created_at     DATETIME(3)     NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  updated_at     DATETIME(3)     NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
  PRIMARY KEY (id),
  UNIQUE KEY uq_tm_tenant_id   (tenant_id, id),
  UNIQUE KEY uq_tm_tenant_user (tenant_id, user_id),
  UNIQUE KEY uq_tm_tenant_emp  (tenant_id, employee_id),
  KEY ix_tm_user (user_id),
  CONSTRAINT fk_tm_tenant FOREIGN KEY (tenant_id) REFERENCES tenants (id),
  CONSTRAINT fk_tm_user   FOREIGN KEY (user_id)   REFERENCES users (id),
  CONSTRAINT ck_tm_status CHECK (status IN ('active','disabled','ended'))
  -- FK komposit (tenant_id, employee_id) -> employees(tenant_id, id) ditambah setelah tabel employees dibuat
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

-- Sesi dengan toggle Simpan Sesi (PRD Bagian 10). Hanya HASH token yang disimpan.
CREATE TABLE sessions (
  id                   BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  token_hash           BINARY(32)      NOT NULL,
  user_id              BIGINT UNSIGNED NOT NULL,
  active_membership_id BIGINT UNSIGNED NULL,   -- tenant yang sedang aktif
  remember             TINYINT(1)      NOT NULL DEFAULT 0,
  expires_at           DATETIME(3)     NULL,   -- NULL bila remember = 1 (tanpa expiry otomatis)
  idle_expires_at      DATETIME(3)     NULL,   -- hanya untuk remember = 0
  last_seen_at         DATETIME(3)     NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  ip                   VARBINARY(16)   NULL,
  user_agent           VARCHAR(300)    NULL,
  device_label         VARCHAR(100)    NULL,
  revoked_at           DATETIME(3)     NULL,
  revoked_reason       VARCHAR(40)     NULL,   -- logout, admin, password_changed, resign, disabled
  created_at           DATETIME(3)     NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  PRIMARY KEY (id),
  UNIQUE KEY uq_sessions_token (token_hash),
  KEY ix_sessions_user (user_id, revoked_at),
  CONSTRAINT fk_sessions_user FOREIGN KEY (user_id) REFERENCES users (id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

-- Role per tenant. Preset sistem disalin dari template saat tenant dibuat.
CREATE TABLE roles (
  id          BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  tenant_id   BIGINT UNSIGNED NOT NULL,
  code        VARCHAR(40)     NOT NULL,
  name        VARCHAR(100)    NOT NULL,
  description VARCHAR(255)    NULL,
  is_system   TINYINT(1)      NOT NULL DEFAULT 0,
  created_at  DATETIME(3)     NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  PRIMARY KEY (id),
  UNIQUE KEY uq_roles_tenant_id   (tenant_id, id),
  UNIQUE KEY uq_roles_tenant_code (tenant_id, code),
  CONSTRAINT fk_roles_tenant FOREIGN KEY (tenant_id) REFERENCES tenants (id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

-- Registry global (bukan per tenant). Diisi lewat seed dan migrasi.
CREATE TABLE permission_registry (
  id               SMALLINT UNSIGNED NOT NULL AUTO_INCREMENT,
  code             VARCHAR(60)  NOT NULL,        -- modul.aksi, mis. karyawan.ekspor
  module           VARCHAR(30)  NOT NULL,
  action           VARCHAR(30)  NOT NULL,
  is_sensitive     TINYINT(1)   NOT NULL DEFAULT 0,
  scope_dimensions JSON         NULL,            -- dimensi yang didukung, mis. ["office","division"]
  entitlement_key  VARCHAR(40)  NULL,
  description      VARCHAR(255) NULL,
  PRIMARY KEY (id),
  UNIQUE KEY uq_perm_code (code),
  UNIQUE KEY uq_perm_module_action (module, action)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

CREATE TABLE role_permissions (
  tenant_id     BIGINT UNSIGNED   NOT NULL,
  role_id       BIGINT UNSIGNED   NOT NULL,
  permission_id SMALLINT UNSIGNED NOT NULL,
  PRIMARY KEY (role_id, permission_id),
  KEY ix_rp_tenant (tenant_id),
  CONSTRAINT fk_rp_role FOREIGN KEY (tenant_id, role_id) REFERENCES roles (tenant_id, id),
  CONSTRAINT fk_rp_perm FOREIGN KEY (permission_id) REFERENCES permission_registry (id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

-- Multi-role: satu membership boleh punya banyak baris aktif, masing-masing dengan masa berlaku.
CREATE TABLE user_roles (
  id            BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  tenant_id     BIGINT UNSIGNED NOT NULL,
  membership_id BIGINT UNSIGNED NOT NULL,
  role_id       BIGINT UNSIGNED NOT NULL,
  valid_from    DATE            NOT NULL,
  valid_to      DATE            NULL,
  assigned_by   BIGINT UNSIGNED NULL,
  revoked_at    DATETIME(3)     NULL,
  created_at    DATETIME(3)     NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  PRIMARY KEY (id),
  KEY ix_ur_member (tenant_id, membership_id, revoked_at),
  KEY ix_ur_role (tenant_id, role_id),
  CONSTRAINT fk_ur_member FOREIGN KEY (tenant_id, membership_id) REFERENCES tenant_memberships (tenant_id, id),
  CONSTRAINT fk_ur_role   FOREIGN KEY (tenant_id, role_id)       REFERENCES roles (tenant_id, id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

-- Allow / Deny per user. Deny mengalahkan semua sumber lain.
CREATE TABLE user_permission_overrides (
  id            BIGINT UNSIGNED   NOT NULL AUTO_INCREMENT,
  tenant_id     BIGINT UNSIGNED   NOT NULL,
  membership_id BIGINT UNSIGNED   NOT NULL,
  permission_id SMALLINT UNSIGNED NOT NULL,
  effect        VARCHAR(5)        NOT NULL,
  reason        VARCHAR(255)      NULL,
  created_by    BIGINT UNSIGNED   NULL,
  created_at    DATETIME(3)       NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  PRIMARY KEY (id),
  UNIQUE KEY uq_upo (tenant_id, membership_id, permission_id),
  CONSTRAINT ck_upo_effect CHECK (effect IN ('allow','deny')),
  CONSTRAINT fk_upo_member FOREIGN KEY (tenant_id, membership_id) REFERENCES tenant_memberships (tenant_id, id),
  CONSTRAINT fk_upo_perm   FOREIGN KEY (permission_id) REFERENCES permission_registry (id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

-- Satu aturan scope. Pemilik: role (preset) ATAU membership (override). Tepat satu terisi.
CREATE TABLE access_scope_rules (
  id            BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  tenant_id     BIGINT UNSIGNED NOT NULL,
  role_id       BIGINT UNSIGNED NULL,
  membership_id BIGINT UNSIGNED NULL,
  module_code   VARCHAR(30)     NOT NULL,          -- '*' = semua modul
  kind          VARCHAR(20)     NOT NULL,          -- all, self, direct_reports, dimensions
  created_at    DATETIME(3)     NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  PRIMARY KEY (id),
  UNIQUE KEY uq_asr_tenant_id (tenant_id, id),
  KEY ix_asr_role   (tenant_id, role_id, module_code),
  KEY ix_asr_member (tenant_id, membership_id, module_code),
  CONSTRAINT ck_asr_owner CHECK ((role_id IS NULL) <> (membership_id IS NULL)),
  CONSTRAINT ck_asr_kind  CHECK (kind IN ('all','self','direct_reports','dimensions')),
  CONSTRAINT fk_asr_role   FOREIGN KEY (tenant_id, role_id)       REFERENCES roles (tenant_id, id),
  CONSTRAINT fk_asr_member FOREIGN KEY (tenant_id, membership_id) REFERENCES tenant_memberships (tenant_id, id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

-- Isi aturan bertipe 'dimensions'. Antardimensi = AND. Beberapa nilai satu dimensi = OR.
CREATE TABLE access_scope_rule_items (
  id        BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  tenant_id BIGINT UNSIGNED NOT NULL,
  rule_id   BIGINT UNSIGNED NOT NULL,
  dimension VARCHAR(12)     NOT NULL,   -- region, office, division, department, position
  ref_id    BIGINT UNSIGNED NOT NULL,   -- id node organisasi (bukan salinan daftar karyawan)
  PRIMARY KEY (id),
  UNIQUE KEY uq_asri (rule_id, dimension, ref_id),
  KEY ix_asri_ref (tenant_id, dimension, ref_id),
  CONSTRAINT ck_asri_dim CHECK (dimension IN ('region','office','division','department','position')),
  CONSTRAINT fk_asri_rule FOREIGN KEY (tenant_id, rule_id) REFERENCES access_scope_rules (tenant_id, id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;
```

### 4.2 Catatan desain

- **Rumus akses efektif** (PRD 3.2) dihitung di Go dari `user_roles` aktif, `role_permissions`, `user_permission_overrides`, dan `access_scope_rules`. Hasilnya di-cache dengan kunci `(membership_id, access_version)`. Setiap perubahan hak akses menaikkan `access_version`, sehingga cache lama otomatis tidak terpakai.
- **Aturan scope:** semua aturan dengan pemilik dan modul yang sama digabung **OR**. Dalam satu aturan bertipe `dimensions`, antardimensi **AND** dan nilai dalam satu dimensi **OR**. Override membership untuk suatu modul **menggantikan** aturan role untuk modul itu. Menghapus override mengembalikan aturan role.
- **Role `KARYAWAN`** memiliki aturan `kind = 'self'` untuk modul ESS. Role `SPV` memiliki `kind = 'direct_reports'`.
- **`sessions.expires_at = NULL`** hanya sah bila `remember = 1`. Mode `remember = 0` wajib punya `idle_expires_at`. Aturan ini divalidasi di aplikasi dan bisa ditambah `CHECK ((remember = 1) OR (idle_expires_at IS NOT NULL))`.
- **Pencabutan sesi massal:** `UPDATE sessions SET revoked_at = NOW(3), revoked_reason = 'password_changed' WHERE user_id = ? AND id <> ? AND revoked_at IS NULL`. Gateway WebSocket memeriksa `revoked_at` dan menutup koneksi terkait.
- **`login_events`** (id, user\_id, tenant\_id NULL, event, ip, user\_agent, created\_at) dan **`access_reviews`** (id, tenant\_id, reviewer\_id, period, status, finding JSON) melengkapi domain ini. Keduanya append-only.

## 5. Organisasi

Hierarki PRD 2.2: **Divisi → Departemen → Jabatan** (struktur) dan **Wilayah → Kantor** (lokasi). Dua dimensi ini terpisah. Tidak ada tabel “tim”.

| Tabel | Kolom penting | Key / index | Catatan |
| --- | --- | --- | --- |
| `regions` | id, tenant\_id, code, name, deleted\_at | UNIQUE (tenant\_id, id). UNIQUE (tenant\_id, code) | Wilayah |
| `offices` | id, tenant\_id, region\_id, code, name, address, timezone, lat, lng, is\_active, deleted\_at | UNIQUE (tenant\_id, id), (tenant\_id, code). FK komposit ke `regions` | `timezone` = nama IANA (mis. `Asia/Jakarta`, `Asia/Makassar`) |
| `office_geofences` | id, tenant\_id, office\_id, name, polygon, is\_active | SPATIAL INDEX polygon. FK komposit ke `offices` | **Banyak polygon per kantor** (DDL di bawah) |
| `divisions` | id, tenant\_id, code, name, deleted\_at | UNIQUE (tenant\_id, id), (tenant\_id, code) |  |
| `departments` | id, tenant\_id, division\_id, code, name, deleted\_at | FK komposit ke `divisions` |  |
| `positions` | id, tenant\_id, department\_id, code, name, grade, level, deleted\_at | FK komposit ke `departments` | Jabatan hidup di bawah departemen |

```sql
CREATE TABLE office_geofences (
  id         BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  tenant_id  BIGINT UNSIGNED NOT NULL,
  office_id  BIGINT UNSIGNED NOT NULL,
  name       VARCHAR(100)    NOT NULL,
  polygon    POLYGON SRID 4326 NOT NULL,
  is_active  TINYINT(1)      NOT NULL DEFAULT 1,
  PRIMARY KEY (id),
  UNIQUE KEY uq_geo_tenant_id (tenant_id, id),
  KEY ix_geo_office (tenant_id, office_id, is_active),
  SPATIAL KEY sx_geo_polygon (polygon),
  CONSTRAINT fk_geo_office FOREIGN KEY (tenant_id, office_id) REFERENCES offices (tenant_id, id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;
```

> **Perhatian sumbu koordinat:** untuk SRID 4326 MySQL 8 memakai urutan **lintang, bujur** (lat, lon). Uji dengan titik nyata sebelum dipakai, atau gunakan opsi `axis-order=long-lat` pada `ST_GeomFromText`. **Alternatif \[USULAN\]:** simpan polygon sebagai JSON dan lakukan point-in-polygon (ray casting) di Go, dengan `office_geofences` hanya sebagai penyimpan. Spatial index MySQL cocok sebagai pra-filter.

## 6. Kepegawaian

### 6.1 Inti: `employees` dan `employee_assignments`

Penugasan memakai **effective dating**. Setiap perubahan jabatan, kantor, atau atasan membuat baris baru. Kolom **wilayah, kantor, divisi, departemen, dan jabatan semuanya disimpan** pada baris penugasan (denormalisasi yang dijaga saat menulis). Tujuannya: filter scope menjadi satu `WHERE` sederhana tanpa join berantai, dan scope Divisi otomatis mencakup departemen baru.

```sql
CREATE TABLE employees (
  id            BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  public_id     CHAR(26)        NOT NULL,
  tenant_id     BIGINT UNSIGNED NOT NULL,
  employee_no   VARCHAR(30)     NOT NULL,            -- NIP
  full_name     VARCHAR(150)    NOT NULL,
  nickname      VARCHAR(60)     NULL,
  gender        CHAR(1)         NULL,
  birth_date    DATE            NULL,
  birth_place   VARCHAR(80)     NULL,
  marital_status VARCHAR(15)    NULL,
  nik_enc       VARBINARY(255)  NULL,                -- NIK terenkripsi di aplikasi
  nik_hash      BINARY(32)      NULL,                -- hash untuk cek duplikasi, bukan untuk dibaca
  work_email    VARCHAR(190)    NULL,
  mobile        VARCHAR(30)     NULL,
  join_date     DATE            NOT NULL,
  end_date      DATE            NULL,
  status        VARCHAR(20)     NOT NULL DEFAULT 'active',
  photo_file_id BIGINT UNSIGNED NULL,
  created_at    DATETIME(3)     NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  updated_at    DATETIME(3)     NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
  deleted_at    DATETIME(3)     NULL,
  PRIMARY KEY (id),
  UNIQUE KEY uq_emp_public (public_id),
  UNIQUE KEY uq_emp_tenant_id (tenant_id, id),
  UNIQUE KEY uq_emp_no (tenant_id, employee_no),
  UNIQUE KEY uq_emp_nik (tenant_id, nik_hash),
  KEY ix_emp_status (tenant_id, status),
  KEY ix_emp_name (tenant_id, full_name),
  CONSTRAINT fk_emp_tenant FOREIGN KEY (tenant_id) REFERENCES tenants (id),
  CONSTRAINT ck_emp_status CHECK (status IN ('active','inactive','terminated'))
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

-- Setelah employees ada, hubungkan membership ke karyawan (aman tenant)
ALTER TABLE tenant_memberships
  ADD CONSTRAINT fk_tm_employee FOREIGN KEY (tenant_id, employee_id) REFERENCES employees (tenant_id, id);

CREATE TABLE employee_assignments (
  id                      BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  tenant_id               BIGINT UNSIGNED NOT NULL,
  employee_id             BIGINT UNSIGNED NOT NULL,
  region_id               BIGINT UNSIGNED NOT NULL,
  office_id               BIGINT UNSIGNED NOT NULL,   -- base office (tepat satu aktif)
  division_id             BIGINT UNSIGNED NOT NULL,
  department_id           BIGINT UNSIGNED NOT NULL,
  position_id             BIGINT UNSIGNED NOT NULL,
  reports_to_employee_id  BIGINT UNSIGNED NULL,       -- atasan langsung
  grade                   VARCHAR(20)     NULL,
  valid_from              DATE            NOT NULL,
  valid_to                DATE            NULL,       -- NULL = penugasan saat ini
  current_flag            TINYINT AS (IF(valid_to IS NULL, 1, NULL)) STORED,
  created_by              BIGINT UNSIGNED NULL,
  created_at              DATETIME(3)     NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  PRIMARY KEY (id),
  -- hanya satu baris current per karyawan (NULL tidak dihitung duplikat)
  UNIQUE KEY uq_ea_current (tenant_id, employee_id, current_flag),
  KEY ix_ea_office   (tenant_id, current_flag, office_id),
  KEY ix_ea_region   (tenant_id, current_flag, region_id),
  KEY ix_ea_division (tenant_id, current_flag, division_id),
  KEY ix_ea_dept     (tenant_id, current_flag, department_id),
  KEY ix_ea_position (tenant_id, current_flag, position_id),
  KEY ix_ea_reports  (tenant_id, current_flag, reports_to_employee_id),
  CONSTRAINT fk_ea_emp      FOREIGN KEY (tenant_id, employee_id) REFERENCES employees (tenant_id, id),
  CONSTRAINT fk_ea_region   FOREIGN KEY (tenant_id, region_id)   REFERENCES regions (tenant_id, id),
  CONSTRAINT fk_ea_office   FOREIGN KEY (tenant_id, office_id)   REFERENCES offices (tenant_id, id),
  CONSTRAINT fk_ea_div      FOREIGN KEY (tenant_id, division_id) REFERENCES divisions (tenant_id, id),
  CONSTRAINT fk_ea_dept     FOREIGN KEY (tenant_id, department_id) REFERENCES departments (tenant_id, id),
  CONSTRAINT fk_ea_pos      FOREIGN KEY (tenant_id, position_id) REFERENCES positions (tenant_id, id),
  CONSTRAINT fk_ea_reports  FOREIGN KEY (tenant_id, reports_to_employee_id) REFERENCES employees (tenant_id, id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;
```

**Catatan:** aplikasi harus memastikan `office_id` milik `region_id` yang sama, dan `position_id` milik `department_id` yang sama (validasi saat menulis atau trigger). Menutup penugasan lama dan membuat yang baru dilakukan dalam **satu transaksi**.

### 6.2 Tabel kepegawaian lain

| Tabel | Kolom penting | Key / index | Catatan |
| --- | --- | --- | --- |
| `employee_allowed_offices` | id, tenant\_id, employee\_id, office\_id, valid\_from, valid\_to, reason | idx (tenant\_id, employee\_id, valid\_to) | Kantor tambahan untuk presensi (base office tidak termasuk) |
| `employee_families` | id, tenant\_id, employee\_id, relation, name, birth\_date, is\_dependent | idx (tenant\_id, employee\_id) | Tanggungan untuk PTKP |
| `employee_emergency_contacts` | id, tenant\_id, employee\_id, name, relation, phone | idx (tenant\_id, employee\_id) |  |
| `employee_bank_accounts` | id, tenant\_id, employee\_id, bank\_code, account\_no\_enc, holder\_name, is\_primary, valid\_from, valid\_to | idx (tenant\_id, employee\_id, is\_primary) | Nomor rekening terenkripsi |
| `employee_tax_bpjs` | id, tenant\_id, employee\_id, npwp\_enc, ptkp\_status, bpjs\_kes\_no\_enc, bpjs\_tk\_no\_enc, valid\_from, valid\_to | idx (tenant\_id, employee\_id, valid\_to) | Berversi tanggal berlaku |
| `contracts` | id, tenant\_id, employee\_id, contract\_type, start\_date, end\_date, status, file\_id | idx (tenant\_id, employee\_id, end\_date), (tenant\_id, status, end\_date) | Tipe: `PKWT, PKWTT, PROBASI`. Indeks `end_date` dipakai reminder kontrak |
| `employee_changes` | id, tenant\_id, employee\_id, change\_type, effective\_date, new\_office\_id, new\_division\_id, new\_department\_id, new\_position\_id, new\_reports\_to, status, request\_id, applied\_at | idx (tenant\_id, status, effective\_date) | Mutasi, promosi, demosi. Job menerapkan perubahan menjadi baris baru di `employee_assignments` pada tanggal efektif |
| `disciplinary_actions` | id, tenant\_id, employee\_id, level, issued\_date, valid\_until, description, file\_id | idx (tenant\_id, employee\_id) | SP dan konseling |
| `terminations` | id, tenant\_id, employee\_id, type, notice\_date, effective\_date, reason, status, request\_id, processed\_at | idx (tenant\_id, status, effective\_date) | Saat efektif: status karyawan, cabut `user_roles` dan `sessions`, picu payroll akhir |
| `assets`, `employee_assets` | assets: code, name, category, status. employee\_assets: employee\_id, asset\_id, issued\_at, returned\_at | idx (tenant\_id, employee\_id) |  |

## 7. Kehadiran dan jadwal

### 7.1 Tabel

| Tabel | Kolom penting | Key / index | Catatan |
| --- | --- | --- | --- |
| `shift_templates` | id, tenant\_id, code, name, start\_time, end\_time, crosses\_midnight, flex\_time, flex\_location, clock\_in\_before\_min, clock\_in\_after\_min, clock\_out\_after\_min, late\_tolerance\_min, break\_minutes, target\_work\_minutes, require\_location, require\_selfie, is\_active | UNIQUE (tenant\_id, code) | Master aturan shift. Jendela clock-in dan toleransi adalah dua kolom berbeda |
| `work_schedules` | id, tenant\_id, employee\_id, shift\_template\_id, pattern (`date`/`weekly`), start\_date, end\_date, days\_mask, status (`draft`/`published`) | idx (tenant\_id, employee\_id, start\_date) | Roster (penugasan). Bukan kejadian aktual |
| `shift_instances` | DDL di bawah | UNIQUE (tenant\_id, employee\_id, work\_date) | Kejadian kerja aktual. Dibuat untuk jendela N hari ke depan |
| `attendance_events` | DDL di bawah | UNIQUE (tenant\_id, employee\_id, client\_event\_id) | **INSERT-only** |
| `attendance_corrections` | id, tenant\_id, employee\_id, shift\_instance\_id, original\_event\_id, requested\_type, requested\_time, reason, status, request\_id, result\_event\_id | idx (tenant\_id, status) | Hasil koreksi = event baru bersumber `correction`. Event asli tidak diubah |
| `attendance_daily` | tenant\_id, employee\_id, work\_date, shift\_instance\_id, first\_in, last\_out, worked\_minutes, late\_minutes, status, computed\_at | PK (tenant\_id, employee\_id, work\_date) | **Turunan** untuk rekap dan payroll. Bukan sumber kebenaran, bisa dihitung ulang |
| `holidays` | id, tenant\_id, holiday\_date, name, type, region\_id NULL | UNIQUE (tenant\_id, holiday\_date, name) | Libur nasional dan perusahaan |
| `shift_swaps` | id, tenant\_id, requester\_id, partner\_id, instance\_a\_id, instance\_b\_id, status, request\_id | idx (tenant\_id, status) |  |

### 7.2 DDL inti kehadiran

```sql
CREATE TABLE shift_instances (
  id               BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  tenant_id        BIGINT UNSIGNED NOT NULL,
  employee_id      BIGINT UNSIGNED NOT NULL,
  work_schedule_id BIGINT UNSIGNED NULL,
  shift_template_id BIGINT UNSIGNED NULL,
  office_id        BIGINT UNSIGNED NULL,
  work_date        DATE            NOT NULL,       -- acuan tanggal, termasuk shift lintas hari
  expected_start   DATETIME(3)     NULL,           -- UTC. NULL untuk shift fleksibel tanpa jam baku
  expected_end     DATETIME(3)     NULL,
  rule_snapshot    JSON            NOT NULL,       -- jendela, toleransi, flag fleksibel/selfie/lokasi saat dibuat
  status           VARCHAR(20)     NOT NULL DEFAULT 'scheduled',
  created_at       DATETIME(3)     NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  PRIMARY KEY (id),
  UNIQUE KEY uq_si_tenant_id (tenant_id, id),
  UNIQUE KEY uq_si_emp_date  (tenant_id, employee_id, work_date),   -- satu shift per hari [TERBUKA]
  KEY ix_si_date  (tenant_id, work_date, status),
  KEY ix_si_start (tenant_id, expected_start),
  CONSTRAINT fk_si_emp FOREIGN KEY (tenant_id, employee_id) REFERENCES employees (tenant_id, id),
  CONSTRAINT ck_si_status CHECK (status IN ('scheduled','cancelled','completed'))
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

-- INSERT-only. Waktu resmi = waktu server (occurred_at).
CREATE TABLE attendance_events (
  id                BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  tenant_id         BIGINT UNSIGNED NOT NULL,
  employee_id       BIGINT UNSIGNED NOT NULL,
  shift_instance_id BIGINT UNSIGNED NULL,
  event_type        VARCHAR(12)     NOT NULL,      -- clock_in, clock_out
  occurred_at       DATETIME(3)     NOT NULL,      -- waktu server, UTC
  device_time       DATETIME(3)     NULL,          -- hanya informasi
  lat               DECIMAL(9,6)    NULL,
  lng               DECIMAL(9,6)    NULL,
  accuracy_m        DECIMAL(7,2)    NULL,
  office_id         BIGINT UNSIGNED NULL,          -- kantor yang cocok
  geofence_id       BIGINT UNSIGNED NULL,          -- polygon yang cocok
  location_status   VARCHAR(12)     NOT NULL,      -- inside, outside, flexible, unknown
  timing_status     VARCHAR(16)     NOT NULL,      -- on_time, late, early_leave, out_of_window
  selfie_file_id    BIGINT UNSIGNED NULL,
  source            VARCHAR(12)     NOT NULL DEFAULT 'ess',   -- ess, admin, correction
  client_event_id   VARCHAR(64)     NOT NULL,      -- kunci idempotensi dari klien
  created_at        DATETIME(3)     NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  PRIMARY KEY (id),
  UNIQUE KEY uq_ae_idem (tenant_id, employee_id, client_event_id),   -- retry tidak membuat event ganda
  KEY ix_ae_emp_time (tenant_id, employee_id, occurred_at),
  KEY ix_ae_instance (tenant_id, shift_instance_id),
  CONSTRAINT fk_ae_emp FOREIGN KEY (tenant_id, employee_id) REFERENCES employees (tenant_id, id),
  CONSTRAINT fk_ae_si  FOREIGN KEY (tenant_id, shift_instance_id) REFERENCES shift_instances (tenant_id, id),
  CONSTRAINT ck_ae_type CHECK (event_type IN ('clock_in','clock_out'))
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;
```

**Catatan desain kehadiran:**

- `rule_snapshot` menyimpan aturan shift pada saat instance dibuat, sehingga mengubah `shift_templates` **tidak** mengubah instance berjalan atau payroll (AC-23).
- `uq_ae_idem` menjamin double tap atau retry koneksi tidak menghasilkan event ganda (AC-21).
- Reminder membaca `shift_instances` (waktu mulai/selesai) dan `attendance_events` (sudah presensi atau belum), dibatasi cuti dari `leave_requests` yang disetujui.
- `attendance_events` akan menjadi tabel terbesar. Index dibuat seperlunya dan ada kebijakan arsip. Jangan memakai partisi bila FK dipertahankan.

## 8. Cuti dan lembur

| Tabel | Kolom penting | Key / index | Catatan |
| --- | --- | --- | --- |
| `leave_types` | id, tenant\_id, code, name, is\_paid, requires\_attachment, max\_consecutive\_days, is\_active | UNIQUE (tenant\_id, code) | Jenis cuti dan izin |
| `leave_policies` | id, tenant\_id, leave\_type\_id, accrual\_type, quota\_days, carry\_over\_days, expire\_months, eligibility JSON, valid\_from, valid\_to | idx (tenant\_id, leave\_type\_id, valid\_from) | Berversi tanggal berlaku. Aturan carry-over dan prorata masih \[TERBUKA\] T-12 |
| `leave_balance_ledger` | id, tenant\_id, employee\_id, leave\_type\_id, entry\_type, days (+/−), effective\_date, ref\_type, ref\_id, note | **UNIQUE (tenant\_id, ref\_type, ref\_id, entry\_type)**. idx (tenant\_id, employee\_id, leave\_type\_id, effective\_date) | **INSERT-only.** Sumber kebenaran saldo. Unique key mencegah potong saldo dua kali (AC-28). `entry_type`: `accrual, usage, adjustment, expire, carry_over, reversal` |
| `leave_balances` | tenant\_id, employee\_id, leave\_type\_id, period\_year, balance\_days | PK (tenant\_id, employee\_id, leave\_type\_id, period\_year) | Cache hasil ledger untuk tampilan cepat. Bisa dibangun ulang |
| `leave_requests` | id, tenant\_id, employee\_id, leave\_type\_id, start\_date, end\_date, days, reason, attachment\_file\_id, status, request\_id | idx (tenant\_id, employee\_id, start\_date, end\_date), (tenant\_id, status) | Tumpang tindih tanggal dicek di aplikasi dalam transaksi |
| `overtime_requests` | id, tenant\_id, employee\_id, work\_date, shift\_instance\_id NULL, proposed\_minutes, approved\_minutes, paid\_minutes, reason, status, request\_id | idx (tenant\_id, employee\_id, work\_date), (tenant\_id, status) | Tiga angka dipisah: diusulkan, disetujui, dibayar (AC-29) |

**Alur saldo:** persetujuan akhir cuti menulis satu baris `usage` di ledger (dalam transaksi yang sama dengan perubahan status), lalu memperbarui `leave_balances`. Pembatalan menulis baris `reversal`, bukan menghapus.

## 9. Approval terpusat

Semua jenis pengajuan memakai **satu mesin**: `requests` → `approval_instances` → `approval_tasks` / `approval_actions`. Tabel spesifik (cuti, lembur, kasbon, dst.) menyimpan data domainnya dan menunjuk ke `request_id`.

| Tabel | Kolom penting | Key / index | Catatan |
| --- | --- | --- | --- |
| `requests` | id, tenant\_id, request\_type, subject\_employee\_id, requested\_by (membership), ref\_table, ref\_id, status, submitted\_at, final\_at | UNIQUE (tenant\_id, request\_type, ref\_id). idx (tenant\_id, status, submitted\_at) | `request_type`: `leave, overtime, attendance_correction, loan, reimbursement, mutation, promotion, payroll, termination, shift_swap` |
| `approval_flows` | id, tenant\_id, request\_type, name, is\_default, valid\_from, valid\_to, fallback\_role\_id | idx (tenant\_id, request\_type, is\_default) | Jalur per jenis pengajuan. Fallback bila tidak ada approver memenuhi |
| `approval_flow_steps` | id, tenant\_id, flow\_id, level\_no, approver\_kind, approver\_ref\_id, required\_permission, sla\_hours | UNIQUE (flow\_id, level\_no) | `approver_kind`: `reports_to, role, position, membership` |
| `approval_instances` | id, tenant\_id, request\_id, flow\_id, current\_level, status, version, started\_at, finished\_at | UNIQUE (tenant\_id, request\_id) | `version` untuk penguncian optimistik. Status: `pending, approved, rejected, returned, cancelled` |
| `approval_tasks` | id, tenant\_id, instance\_id, level\_no, assignee\_membership\_id, status, created\_at | idx (tenant\_id, assignee\_membership\_id, status) | **Siapa yang boleh bertindak saat ini.** Diisi saat tahap diaktifkan. Menjadi sumber inbox Pusat Persetujuan |
| `approval_actions` | id, tenant\_id, instance\_id, level\_no, actor\_membership\_id, action, reason, created\_at | idx (tenant\_id, instance\_id) | **INSERT-only.** `action`: `approve, reject, return, cancel, delegate` |
| `approval_delegations` | id, tenant\_id, from\_membership\_id, to\_membership\_id, request\_type NULL, valid\_from, valid\_to | idx (tenant\_id, from\_membership\_id) | Delegasi dan eskalasi masih \[TERBUKA\] T-11 |

**Aturan penting:**

- Saat tahap diaktifkan, backend memanggil **evaluator akses** untuk mencari anggota yang punya `*.approve` **dan** scope yang mencakup `subject_employee_id`. Hasilnya ditulis ke `approval_tasks`.
- **Pengaju dikeluarkan** dari daftar task. Saat aksi, backend memeriksa ulang bahwa `actor <> requested_by` (AC-56).
- Aksi approval memakai transaksi: `SELECT ... FROM approval_instances WHERE id = ? FOR UPDATE`, cek `version` dan task masih `open`, tulis `approval_actions`, ubah instance dan request. Dua approver bersamaan hanya menghasilkan satu hasil akhir (AC-25).
- Bila tidak ada approver yang memenuhi, task dibuat untuk role `fallback_role_id` (HR) dan fallback dicatat di `approval_actions`.

## 10. Kasbon dan reimbursement

| Tabel | Kolom penting | Key / index | Catatan |
| --- | --- | --- | --- |
| `employee_loans` | id, tenant\_id, employee\_id, amount, tenor\_months, installment\_amount, outstanding, status, disbursed\_at, disbursement\_ref, request\_id | **UNIQUE (tenant\_id, disbursement\_ref)**. idx (tenant\_id, employee\_id, status) | Kunci pencairan mencegah bayar ganda saat retry (AC-31) |
| `loan_installments` | id, tenant\_id, loan\_id, seq, due\_period, amount, status, payroll\_item\_line\_id | UNIQUE (loan\_id, seq). idx (tenant\_id, due\_period, status) | Status: `scheduled, deducted, paid, waived`. Dipotong lewat payroll |
| `reimbursement_categories` | id, tenant\_id, code, name, monthly\_limit | UNIQUE (tenant\_id, code) |  |
| `reimbursements` | id, tenant\_id, employee\_id, category\_id, trx\_date, amount, description, duplicate\_hash, status, payment\_ref, paid\_at, request\_id | UNIQUE (tenant\_id, payment\_ref). idx (tenant\_id, employee\_id, trx\_date), (tenant\_id, duplicate\_hash) | `duplicate_hash` dari karyawan + tanggal + nominal + berkas, untuk validasi duplikasi |
| `reimbursement_receipts` | reimbursement\_id, file\_id | PK (reimbursement\_id, file\_id) | Kuitansi |

## 11. Payroll

Prinsip: **hasil final adalah snapshot yang tidak berubah** (AC-32). Perhitungan membaca data hidup, tetapi saat difinalisasi seluruh nilai dan aturan yang dipakai disalin ke tabel snapshot. Perbaikan setelah final lewat `payroll_adjustments`.

| Tabel | Kolom penting | Key / index | Catatan |
| --- | --- | --- | --- |
| `pay_components` | id, tenant\_id, code, name, type, taxable, calc\_type, is\_system | UNIQUE (tenant\_id, code) | `type`: `earning, deduction, benefit`. `calc_type`: `fixed, percent, formula` |
| `employee_pay_components` | id, tenant\_id, employee\_id, component\_id, amount, valid\_from, valid\_to | idx (tenant\_id, employee\_id, valid\_to) | Komponen per karyawan, berversi tanggal berlaku |
| `tax_rules` | id, rule\_set, effective\_from, effective\_to, data JSON | idx (rule\_set, effective\_from) | **Global**, dikelola platform \[USULAN\]. Tarif PPh 21 dan PTKP. Tidak di-hardcode |
| `bpjs_rules` | id, program, effective\_from, effective\_to, rate\_employee, rate\_employer, wage\_cap | idx (program, effective\_from) | Global, berversi |
| `payroll_periods` | id, tenant\_id, year, month, start\_date, end\_date, status | UNIQUE (tenant\_id, year, month) | Status: `open, calculating, review, finalized, closed` |
| `payroll_runs` | id, tenant\_id, period\_id, run\_no, status, rules\_snapshot JSON, total\_gross, total\_net, calculated\_at, finalized\_at, finalized\_by | UNIQUE (tenant\_id, period\_id, run\_no) | `rules_snapshot` = versi `tax_rules` dan `bpjs_rules` yang dipakai |
| `payroll_items` | id, tenant\_id, run\_id, employee\_id, gross, total\_deduction, tax, net, employee\_snapshot JSON | UNIQUE (run\_id, employee\_id) | `employee_snapshot`: jabatan, bank, PTKP saat run. Tidak berubah walau profil diedit |
| `payroll_item_lines` | id, tenant\_id, item\_id, component\_code, line\_type, amount, source\_type, source\_id, description | idx (tenant\_id, item\_id), (source\_type, source\_id) | Rincian. `source_type`: `attendance, overtime, loan, reimbursement, manual` |
| `payroll_adjustments` | id, tenant\_id, run\_id, employee\_id, reason, amount, request\_id, created\_by | idx (tenant\_id, run\_id) | Koreksi pasca-final |
| `payslips` | id, tenant\_id, payroll\_item\_id, file\_id, published\_at, viewed\_at | UNIQUE (payroll\_item\_id) | Hanya pemilik dan pihak berwenang. Akses dicek ulang saat unduh |

**Penjagaan data final:** saat `payroll_runs.status = 'finalized'`, tabel `payroll_items` dan `payroll_item_lines` run itu tidak boleh diubah. Terapkan di aplikasi **dan** di database (trigger `BEFORE UPDATE/DELETE` yang menolak) atau hak akses akun aplikasi.

## 12. Notifikasi dan broadcast

### 12.1 Tabel

| Tabel | Kolom penting | Key / index | Catatan |
| --- | --- | --- | --- |
| `notifications` | DDL di bawah | UNIQUE (tenant\_id, recipient\_membership\_id, dedupe\_key) | Inbox. **Sumber kebenaran**. `id` naik monoton dipakai sebagai cursor sinkronisasi |
| `notification_outbox` | DDL di bawah | UNIQUE (tenant\_id, dedupe\_key) | Ditulis dalam transaksi yang sama dengan perubahan bisnis |
| `notification_schedules` | DDL di bawah | UNIQUE (tenant\_id, dedupe\_key) | Jadwal reminder. Diambil worker dengan `SKIP LOCKED` |
| `notification_events` | id, tenant\_id, event\_type, aggregate\_type, aggregate\_id, created\_at | idx (tenant\_id, event\_type, created\_at) | Log event yang telah diproses (opsional, append-only) |
| `notification_deliveries` | id, tenant\_id, notification\_id, channel, status, error, attempted\_at | UNIQUE (notification\_id, channel) | `channel`: `inapp, push, email, whatsapp`. Terkirim ≠ dibaca |
| `notification_rules` | id, tenant\_id, event\_type, enabled, config JSON, updated\_by | UNIQUE (tenant\_id, event\_type) | Offset reminder, pengulangan, jam tenang, penerima ringkasan |
| `notification_templates` | id, tenant\_id NULL, event\_type, channel, title\_tpl, body\_tpl | UNIQUE (tenant\_id, event\_type, channel) | `tenant_id` NULL = default platform |
| `notification_preferences` | membership\_id, category, channel, enabled | PK (membership\_id, category, channel) | Notifikasi keamanan tidak bisa dimatikan |
| `push_subscriptions` | id, tenant\_id, membership\_id, endpoint\_hash, endpoint, p256dh, auth, user\_agent, last\_success\_at, revoked\_at | UNIQUE endpoint\_hash | Web Push |
| `broadcasts` | id, tenant\_id, title, body, attachment\_file\_id, status, send\_at, combine\_mode, recipients\_count, snapshot\_at, created\_by, sent\_by | idx (tenant\_id, status, send\_at) | `combine_mode`: `and` (default) atau `or`. Status: `draft, scheduled, sending, sent, cancelled` |
| `broadcast_targets` | id, tenant\_id, broadcast\_id, target\_type, ref\_id | idx (broadcast\_id) | `target_type`: `all, region, office, division, department, position, user` |
| `broadcast_recipients` | id, tenant\_id, broadcast\_id, membership\_id, employee\_id, notification\_id, delivery\_status, read\_at | **UNIQUE (broadcast\_id, membership\_id)** | **Snapshot penerima** saat eksekusi (AC-38). Unique key = satu salinan per user (AC-39) |

### 12.2 DDL inti notifikasi

```sql
CREATE TABLE notifications (
  id                      BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  tenant_id               BIGINT UNSIGNED NOT NULL,
  recipient_membership_id BIGINT UNSIGNED NOT NULL,
  category                VARCHAR(20)     NOT NULL,   -- request, reminder, broadcast, system
  event_type              VARCHAR(50)     NOT NULL,
  title                   VARCHAR(150)    NOT NULL,
  body                    VARCHAR(500)    NULL,
  deep_link               VARCHAR(255)    NULL,       -- rute internal. Hak akses dicek ulang saat dibuka
  payload                 JSON            NULL,       -- dilarang memuat NIK, rekening, nominal gaji
  dedupe_key              VARCHAR(120)    NOT NULL,
  priority                TINYINT         NOT NULL DEFAULT 0,
  read_at                 DATETIME(3)     NULL,
  archived_at             DATETIME(3)     NULL,
  obsolete_at             DATETIME(3)     NULL,       -- tindakan lama yang tidak relevan lagi
  created_at              DATETIME(3)     NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  PRIMARY KEY (id),
  UNIQUE KEY uq_n_dedupe (tenant_id, recipient_membership_id, dedupe_key),
  KEY ix_n_inbox (tenant_id, recipient_membership_id, archived_at, read_at, id),
  CONSTRAINT fk_n_member FOREIGN KEY (tenant_id, recipient_membership_id) REFERENCES tenant_memberships (tenant_id, id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

CREATE TABLE notification_outbox (
  id              BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  tenant_id       BIGINT UNSIGNED NOT NULL,
  event_type      VARCHAR(50)     NOT NULL,
  aggregate_type  VARCHAR(40)     NOT NULL,
  aggregate_id    BIGINT UNSIGNED NOT NULL,
  payload         JSON            NULL,
  dedupe_key      VARCHAR(120)    NOT NULL,
  status          VARCHAR(12)     NOT NULL DEFAULT 'pending',   -- pending, processing, done, failed
  attempts        SMALLINT        NOT NULL DEFAULT 0,
  next_attempt_at DATETIME(3)     NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  created_at      DATETIME(3)     NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  processed_at    DATETIME(3)     NULL,
  PRIMARY KEY (id),
  UNIQUE KEY uq_ob_dedupe (tenant_id, dedupe_key),
  KEY ix_ob_pick (status, next_attempt_at)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

CREATE TABLE notification_schedules (
  id                      BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  tenant_id               BIGINT UNSIGNED NOT NULL,
  rule_id                 BIGINT UNSIGNED NULL,
  subject_type            VARCHAR(30)     NOT NULL,   -- shift_instance, contract, request, broadcast
  subject_id              BIGINT UNSIGNED NOT NULL,
  recipient_membership_id BIGINT UNSIGNED NULL,
  fire_at                 DATETIME(3)     NOT NULL,   -- UTC
  attempt_no              TINYINT         NOT NULL DEFAULT 1,
  status                  VARCHAR(12)     NOT NULL DEFAULT 'scheduled',  -- scheduled, fired, cancelled
  dedupe_key              VARCHAR(120)    NOT NULL,
  created_at              DATETIME(3)     NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  PRIMARY KEY (id),
  UNIQUE KEY uq_ns_dedupe (tenant_id, dedupe_key),
  KEY ix_ns_due (status, fire_at)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;
```

**Query worker (aman untuk banyak instance):**

```sql
SELECT id FROM notification_schedules
WHERE status = 'scheduled' AND fire_at <= UTC_TIMESTAMP(3)
ORDER BY fire_at
LIMIT 100
FOR UPDATE SKIP LOCKED;
```

Sebelum mengirim, worker **menilai ulang kondisi** (sudah presensi? shift dibatalkan? sedang cuti?). Bila tidak relevan, jadwal ditandai `cancelled`. Presensi sah yang masuk juga membatalkan jadwal reminder terkait (AC-36).

## 13. File dan dokumen

| Tabel | Kolom penting | Key / index | Catatan |
| --- | --- | --- | --- |
| `files` | id, public\_id, tenant\_id, storage\_key, original\_name, mime\_type, size\_bytes, sha256, purpose, uploaded\_by, scan\_status, created\_at, deleted\_at | UNIQUE storage\_key. idx (tenant\_id, purpose) | Metadata objek di S3-compatible. Berkas **tidak** disimpan di database. Diunduh lewat URL sementara setelah cek permission. `purpose`: `selfie, contract, receipt, payslip, document, avatar` |
| `document_types` | id, tenant\_id, code, name, requires\_expiry, is\_mandatory | UNIQUE (tenant\_id, code) | KTP, ijazah, NPWP, dan seterusnya |
| `employee_documents` | id, tenant\_id, employee\_id, document\_type\_id, file\_id, issued\_date, expires\_at, status | idx (tenant\_id, employee\_id), (tenant\_id, expires\_at) | Indeks `expires_at` dipakai reminder dokumen kedaluwarsa |
| `company_documents` | id, tenant\_id, title, category, file\_id, visibility, published\_at | idx (tenant\_id, category) | `visibility`: `all` atau `scoped` |
| `document_templates` | id, tenant\_id, code, name, body, variables JSON | UNIQUE (tenant\_id, code) | Template surat |
| `agenda_events` | id, tenant\_id, title, description, starts\_at, ends\_at, all\_day, location, visibility, created\_by | idx (tenant\_id, starts\_at) | Agenda perusahaan |

## 14. Audit, impor/ekspor, dan pengaturan

| Tabel | Kolom penting | Key / index | Catatan |
| --- | --- | --- | --- |
| `audit_logs` | id, tenant\_id NULL, actor\_type, actor\_id, support\_session\_id NULL, action, entity\_type, entity\_id, before\_json, after\_json, ip, user\_agent, request\_id, created\_at | idx (tenant\_id, created\_at), (tenant\_id, entity\_type, entity\_id), (tenant\_id, actor\_id, created\_at) | **INSERT-only.** `actor_type`: `membership, platform, system`. **Dilarang** menyimpan password, token sesi, atau rahasia mentah di JSON |
| `import_jobs` | id, tenant\_id, type, file\_id, status, total\_rows, ok\_rows, error\_rows, created\_by | idx (tenant\_id, created\_at) | Tiap baris dicek terhadap scope pengimpor (AC-54) |
| `import_job_errors` | id, job\_id, row\_no, field, message | idx (job\_id) | Laporan error per baris |
| `export_jobs` | id, tenant\_id, type, filters JSON, scope\_snapshot JSON, status, file\_id, row\_count, created\_by, expires\_at | idx (tenant\_id, created\_by, created\_at) | Ekspor berjalan sebagai job. `scope_snapshot` mencatat scope yang berlaku. Selalu diaudit (AC-55) |
| `tenant_settings` | tenant\_id, setting\_key, value JSON, updated\_by | PK (tenant\_id, setting\_key) | Tema, kebijakan absensi, `session.remember_allowed_admin`, `session.idle_timeout_min`, dan seterusnya |
| `api_keys`, `webhooks` \[BERTAHAP\] | Fase 5 |  | Integrasi dan API |

## 15. Dari scope menjadi SQL

### 15.1 Menentukan permission efektif

```sql
-- Permission dari semua role aktif (hari ini)
SELECT DISTINCT p.code
FROM user_roles ur
JOIN role_permissions rp   ON rp.tenant_id = ur.tenant_id AND rp.role_id = ur.role_id
JOIN permission_registry p ON p.id = rp.permission_id
WHERE ur.tenant_id = :tenant_id
  AND ur.membership_id = :membership_id
  AND ur.revoked_at IS NULL
  AND ur.valid_from <= CURRENT_DATE
  AND (ur.valid_to IS NULL OR ur.valid_to >= CURRENT_DATE);
```

Lalu di Go: **tambahkan** override `allow`, **kurangi** override `deny` (Deny menang), saring dengan entitlement paket. Hasilnya di-cache dengan kunci `(membership_id, access_version)`.

### 15.2 Filter scope pada daftar data

Evaluator menghasilkan kumpulan aturan untuk `(membership, modul)`. Query builder mengubahnya menjadi satu klausa `WHERE`. Contoh: user punya dua aturan dimensi, satu aturan bawahan langsung, dan data diri sendiri.

```sql
SELECT e.id, e.full_name, a.office_id, a.division_id
FROM employees e
JOIN employee_assignments a
  ON a.tenant_id = e.tenant_id AND a.employee_id = e.id AND a.current_flag = 1
WHERE e.tenant_id = :tenant_id
  AND e.deleted_at IS NULL
  AND (
        (a.office_id IN (1, 2) AND a.division_id IN (5))   -- aturan 1: Kantor A/B DAN Divisi X
     OR (a.region_id IN (3))                               -- aturan 2: Wilayah 3 (semua kantornya)
     OR (a.reports_to_employee_id = :me_employee_id)       -- aturan 3: bawahan langsung
     OR (e.id = :me_employee_id)                           -- aturan 4: diri sendiri
      );
```

**Aturan penerjemahan:**

| Kasus | Hasil `WHERE` |
| --- | --- |
| Ada aturan `kind = 'all'` | Tanpa batasan tambahan (hanya `tenant_id`) |
| Tidak ada aturan sama sekali | `1 = 0` (deny by default) |
| Dimensi `division` | `a.division_id IN (...)`. Departemen dan jabatan baru di bawahnya otomatis tercakup, karena baris penugasan menyimpan `division_id` |
| Dimensi `region` | `a.region_id IN (...)`. Kantor baru di wilayah itu otomatis tercakup |
| Beberapa aturan | Digabung `OR` |
| Dalam satu aturan | Antardimensi `AND`, antarnilai `IN (...)` |

**Modul tanpa kolom karyawan langsung** (cuti, lembur, absensi, kasbon, dan lainnya) memakai `subject_employee_id` yang di-join ke `employee_assignments` dengan pola yang sama. Ekspor dan impor memakai filter yang sama.

> **\[TERBUKA\] D-02:** untuk data historis (mis. absensi bulan lalu), apakah scope memakai penugasan **saat ini** atau penugasan **pada tanggal kejadian**? Versi kedua menambah `a.valid_from <= :tanggal AND (a.valid_to IS NULL OR a.valid_to >= :tanggal)` dan menghilangkan `current_flag`. Default usulan: **saat ini**, karena lebih sederhana dan aman.

## 16. Index, performa, dan keamanan data

- **Aturan index:** `tenant_id` selalu kolom pertama. Index dibuat dari pola query nyata (daftar + filter + sort), bukan menebak. Kolom JSON tidak diindeks langsung. Bila perlu, pakai *generated column* lalu indeks.
- **Enkripsi field sensitif** \[USULAN\]: `nik`, nomor rekening, NPWP, dan nomor BPJS dienkripsi di aplikasi (AES-256-GCM, kunci per tenant lewat envelope key/KMS). Pencarian duplikasi memakai `HMAC` dengan *pepper* (kolom `*_hash`), bukan dekripsi massal.
- **Penyamaran:** kolom sensitif dikembalikan API hanya jika user punya `lihat_sensitif`. Ekspor mengikuti aturan yang sama (AC-12, AC-55).
- **Hak akses akun database:** akun aplikasi tidak punya `DROP` atau `ALTER`. Migrasi memakai akun terpisah. Laporan berat dan ekspor dibaca dari **read replica**.
- **Tabel terbesar:** `attendance_events`, `audit_logs`, `notifications`, `payroll_item_lines`. Rencanakan arsip berkala. Kebijakan retensi mengikuti T-15.
- **Backup dan restore:** backup penuh terjadwal dengan uji restore. Ekspor per tenant berdasarkan `tenant_id` untuk kebutuhan penghentian layanan.
- **Tes wajib di CI:**
  1. Menyisipkan baris anak dengan `tenant_id` berbeda dari induknya **harus gagal** (FK komposit).
  2. Setiap query repository harus menyertakan `tenant_id` (lint atau tes statis).
  3. Retry pembuatan event presensi, pencairan kasbon, dan potong saldo cuti tidak menghasilkan baris ganda.
  4. Finalisasi payroll lalu mengubah profil karyawan tidak mengubah `payroll_items`.

## 17. Urutan migrasi per fase

| Fase | Tabel yang dibuat |
| --- | --- |
| **0. Fondasi** | `tenants`, `plans`, `plan_entitlements`, `subscriptions` (minimal), `platform_staff`, `users`, `tenant_memberships`, `sessions`, `login_events`, `roles`, `permission_registry`, `role_permissions`, `user_roles`, `user_permission_overrides`, `access_scope_rules`, `access_scope_rule_items`, `audit_logs`, `files`, `tenant_settings` |
| **1. SDM inti** | `regions`, `offices`, `office_geofences`, `divisions`, `departments`, `positions`, `employees`, `employee_assignments`, `employee_allowed_offices`, `employee_families`, `employee_emergency_contacts`, `employee_bank_accounts`, `employee_tax_bpjs`, `contracts`, `document_types`, `employee_documents`, `company_documents`, `import_jobs`, `export_jobs`, `notifications`, `notification_preferences`, `notification_outbox`, `notification_deliveries`, `broadcasts`, `broadcast_targets`, `broadcast_recipients`, `push_subscriptions`, `agenda_events` |
| **2. Kehadiran** | `shift_templates`, `work_schedules`, `shift_instances`, `attendance_events`, `attendance_corrections`, `attendance_daily`, `holidays`, `shift_swaps`, `notification_rules`, `notification_templates`, `notification_schedules`, `notification_events` |
| **3. Pengajuan** | `requests`, `approval_flows`, `approval_flow_steps`, `approval_instances`, `approval_tasks`, `approval_actions`, `approval_delegations`, `leave_types`, `leave_policies`, `leave_balance_ledger`, `leave_balances`, `leave_requests`, `overtime_requests`, `employee_loans`, `loan_installments`, `reimbursement_categories`, `reimbursements`, `reimbursement_receipts` |
| **4. Payroll** | `pay_components`, `employee_pay_components`, `tax_rules`, `bpjs_rules`, `payroll_periods`, `payroll_runs`, `payroll_items`, `payroll_item_lines`, `payroll_adjustments`, `payslips` |
| **5. Lanjutan** | Tabel talenta (kinerja, rekrutmen, pelatihan), `api_keys`, `webhooks`, `support_*` lanjutan, `access_reviews` |

> **Konflik urutan yang perlu diputuskan (D-06):** menu **Mutasi & Promosi** dan **Resign & Terminasi** memakai approval, padahal mesin approval baru masuk Fase 3. Pilihannya: (a) pindahkan tabel approval ke Fase 1, atau (b) tunda kedua menu itu ke Fase 3. Saran saya (a), karena approval adalah fondasi lintas modul.

## 18. Keputusan terbuka untuk database

| ID | Topik | Usulan | Dibutuhkan sebelum |
| --- | --- | --- | --- |
| D-01 | Satu shift per karyawan per hari (unique `work_date`) atau boleh lebih dari satu | Satu per hari untuk MVP | Fase 2 |
| D-02 | Scope data historis memakai penugasan saat ini atau pada tanggal kejadian | Saat ini | Fase 2 |
| D-03 | PK `BIGINT` auto-increment atau UUID v7 | `BIGINT` + `public_id` ULID | Fase 0 |
| D-04 | Enkripsi field sensitif dan manajemen kunci (KMS) | AES-GCM per tenant, envelope key | Fase 1 |
| D-05 | `tax_rules` dan `bpjs_rules`: global atau bisa di-override per tenant | Global, dikelola platform | Fase 4 |
| D-06 | Mesin approval masuk Fase 1 atau Mutasi/Terminasi ditunda | Pindahkan approval ke Fase 1 | Fase 1 |
| D-07 | Retensi dan arsip (`attendance_events`, `audit_logs`, `notifications`) | Ikuti T-15 | Sebelum produksi |
| D-08 | Polygon: MySQL spatial atau JSON + ray casting di Go | MySQL spatial untuk pra-filter | Fase 1 |
| D-09 | `legal_entity_id` pada karyawan dan payroll (terkait T-05) | Putuskan bersama T-05 | Fase 1 |
| D-10 | Skema modul talenta | Ditentukan di Fase 5 | Fase 5 |
