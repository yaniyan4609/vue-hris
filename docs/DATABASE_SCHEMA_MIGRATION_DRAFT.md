# Inventaris dan rencana migrasi database digaji.id

**Draf diskusi, 5 November 2026. Belum ada migrasi baru yang dijalankan berdasarkan dokumen ini.** Sumber: `full-docs/Desain Database digaji.id.md` (draft [USULAN], 4 November 2026), PRD v2.3, Matriks Menu/Role/Fungsional, migrasi `00001`–`00010`, dan `information_schema` pada MariaDB staging `192.168.11.85/staging-saas-hris`. Daftar pada bagian berikut berisi **seluruh field yang benar-benar ada** di staging. Daftar target setelahnya mengikuti **kolom penting** pada dokumen desain; tipe, nullability, constraint, dan field audit final masih harus ditetapkan per migrasi.

## 1. Skema staging saat ini — 35 tabel aplikasi

Semua ID utama memakai string `CHAR(26)`; generator Go saat ini meng-encode **128 bit acak** dengan alfabet Crockford Base32. Bentuknya mirip ULID, tetapi tidak memiliki prefiks timestamp ULID. Waktu yang sudah ada memakai `DATETIME(6)`. `tenant_id` menjadi bagian dari PK/FK komposit pada tabel bisnis yang sudah ada. Tabel internal Goose tidak dihitung.

| Tabel | Field aktual | Relasi FK aktual |
| --- | --- | --- |
| `tenants` | id, slug, name, status, timezone, idle_timeout_minutes, remember_admin_allowed, created_at, updated_at | — |
| `users` | id, email, email_normalized, display_name, password_hash, status, created_at, updated_at | — |
| `tenant_memberships` | tenant_id, user_id, status, joined_at, ended_at, access_version | tenants, users |
| `sessions` | id, token_hash, user_id, selected_tenant_id, remember, idle_timeout_minutes, expires_at, last_seen_at, rotated_at, revoked_at, created_at, user_agent, ip, device_label, revoked_reason | users, tenant_memberships |
| `login_events` | id, user_id, tenant_id, event, ip, user_agent, created_at | users, tenants |
| `permission_registry` | code, module, action, is_sensitive, requires_scope, entitlement_code, created_at | — |
| `permission_scope_dimensions` | permission_code, dimension | permission_registry |
| `roles` | tenant_id, id, code, name, is_system, created_at, updated_at | tenants |
| `user_roles` | tenant_id, id, user_id, role_id, valid_from, valid_until, assigned_by, assigned_at, revoked_at, revoked_by | tenant_memberships, roles, users |
| `role_permissions` | tenant_id, role_id, permission_code, granted_at | roles, permission_registry |
| `role_scope_rules` | tenant_id, id, role_id, permission_code, kind | role_permissions |
| `role_scope_values` | tenant_id, rule_id, dimension, node_id | role_scope_rules |
| `user_permission_overrides` | tenant_id, user_id, permission_code, decision, updated_at | tenant_memberships, permission_registry |
| `user_scope_overrides` | tenant_id, user_id, permission_code, updated_at | tenant_memberships, permission_registry |
| `user_scope_rules` | tenant_id, id, user_id, permission_code, kind | user_scope_overrides |
| `user_scope_values` | tenant_id, rule_id, dimension, node_id | user_scope_rules |
| `platform_roles` | id, code, name, created_at | — |
| `platform_user_roles` | user_id, role_id, valid_from, valid_until | users, platform_roles |
| `platform_permission_registry` | code, module, action | — |
| `platform_role_permissions` | role_id, permission_code | platform_roles, platform_permission_registry |
| `platform_user_permission_overrides` | user_id, permission_code, decision, updated_at | users, platform_permission_registry |
| `platform_user_tenants` | user_id, tenant_id | users, tenants |
| `platform_audit_logs` | id, actor_user_id, target_user_id, action, detail_json, created_at | users |
| `regions` | tenant_id, id, code, name | tenants |
| `offices` | tenant_id, id, region_id, code, name, timezone | regions |
| `divisions` | tenant_id, id, code, name | tenants |
| `departments` | tenant_id, id, division_id, code, name | divisions |
| `positions` | tenant_id, id, department_id, code, name | departments |
| `employees` | tenant_id, id, user_id, employee_number, display_name, status, base_office_id, position_id, reports_to_id, created_at | tenant_memberships, offices, positions, employees |
| `employee_assignment_history` | tenant_id, id, employee_id, base_office_id, position_id, reports_to_id, valid_from, valid_until, created_at | employees, offices, positions |
| `audit_logs` | id, tenant_id, actor_user_id, action, entity_type, entity_id, before_json, after_json, created_at | tenants, users |
| `entitlements` | code, name | — |
| `plans` | id, code, name, status | — |
| `plan_entitlements` | plan_id, entitlement_code | plans, entitlements |
| `tenant_subscriptions` | tenant_id, plan_id, status | tenants, plans |

**Relasi yang sudah aman:** kantor→wilayah; departemen→divisi; jabatan→departemen; karyawan→kantor/jabatan/atasan; role dan override→tenant. `role_scope_values` dan `user_scope_values` menyimpan `node_id` polimorfik tanpa FK ke lima tabel organisasi; backend saat ini memvalidasi node dan tenant saat penulisan. Ini pengecualian yang perlu diuji pada skema penuh.

## 2. Selisih penting terhadap dokumen desain

| Rancangan | Kondisi proyek | Sikap migrasi yang diusulkan |
| --- | --- | --- |
| PK `BIGINT` + `public_id` ULID, MySQL 8 `utf8mb4_0900_ai_ci`, `DATETIME(3)` | PK ID acak `CHAR(26)`, MariaDB 10.11, `utf8mb4_unicode_ci`/`ascii_bin`, `DATETIME(6)` | D-03 masuk kajian `BIGINT` terpisah. Jangan menulis ulang PK/FK atau migrasi lama sebelum hasil studi dan keputusan baru. |
| `subscriptions` dan `tenants.plan_id` | `tenant_subscriptions` menjadi relasi paket tenant | Kembangkan `tenant_subscriptions` dengan periode, siklus, dan histori; hindari dua sumber status langganan. Nama final dibahas dahulu. |
| Status tenant `pending`/`closed` dan karyawan `inactive` dalam desain | Tenant aktual `pending_verification`/`stopped`; karyawan aktual punya `probation`, `leave`, `resigned`, `terminated` | Buat peta transisi status dan pertahankan nilai yang sudah disimpan. Jangan mengganti `CHECK` sebelum API, seed, dan data diselaraskan. |
| `platform_staff` satu role per user | `platform_roles` + `platform_user_roles` mendukung beberapa role aktif, override, dan tenant assignment | Pertahankan model multi-role yang sudah dipakai. Jangan membuat `platform_staff` duplikat. |
| `access_scope_rules` gabungan | `role_scope_rules/values` dan `user_scope_overrides/rules/values` per permission | Pertahankan evaluator saat ini. Jika disatukan nanti, perlu backfill dan tes ekuivalensi Allow/Deny/OR/AND; bukan migrasi awal. |
| `employee_assignments` efektif bertanggal dan snapshot 5 dimensi | `employees` menyimpan penempatan aktif; `employee_assignment_history` hanya kantor/jabatan/atasan | Tambahkan tabel assignment baru, backfill dari data aktif/history, dual-write sementara, baru alihkan baca/scope. Kolom lama jangan dihapus pada migrasi yang sama. |
| `notifications.id BIGINT` sebagai cursor inbox | Standar API saat ini ID acak 26 karakter | Usulan yang perlu diuji: `id` publik 26 karakter + `sequence_no BIGINT AUTO_INCREMENT UNIQUE` untuk cursor. Perlu uji urutan, indeks, dan retry worker. |
| Beberapa tabel rancangan tidak memuat `tenant_id` (`reimbursement_receipts`, `notification_preferences`) | Aturan isolasi meminta tenant pada tabel bisnis | Tambahkan `tenant_id` serta FK komposit sebelum DDL final. Pengecualian hanya tabel global seperti aturan pajak platform dan webhook gateway. |
| DDL spatial MySQL 8 untuk geofence | Server nyata MariaDB | Uji kompatibilitas, axis order, dan point-in-polygon pada MariaDB sebelum memilih spatial atau JSON + validasi Go (D-08). |

## 3. Daftar tabel/field target dari desain (belum final)

Tipe PK tabel baru ditetapkan setelah kajian D-03; tabel lama tetap memakai ID 26 karakter selama studi. Semua tabel bisnis baru akan mendapat `tenant_id` bila datanya milik tenant, timestamp UTC, FK komposit `(tenant_id, parent_id)`, dan indeks dari query nyata. Daftar di bawah menampilkan **field domain yang tertulis di desain**, sehingga belum menjanjikan semua field teknis/audit. Tabel yang sudah ada pada bagian 1 **diperluas**, bukan dibuat ulang. Istilah `request_id`, `file_id`, dan sejenisnya mengacu ke tabel induknya di domain terkait.

### Platform, paket, dan identitas

| Tabel target | Field domain menurut desain | Relasi/status |
| --- | --- | --- |
| `tenants` (perluas) | name, slug, status, timezone, trial_ends_at | Status lifecycle dan kuota belum lengkap. |
| `plans` (perluas) | code, name, price_monthly, price_yearly, max_employees, max_users, max_offices, max_storage_mb, is_active | Entitlement melalui `plan_entitlements`. |
| `plan_entitlements` (perluas) | plan_id, entitlement_key, limit_value | Existing memakai `entitlement_code`; pertahankan nama itu atau migrasikan terukur. |
| `tenant_subscriptions` (perluas) / `subscriptions` (opsi histori) | tenant_id, plan_id, status, billing_cycle, period_start, period_end, auto_renew, canceled_at | Satu langganan aktif; model histori dan nama final perlu keputusan. |
| `invoices` | tenant_id, subscription_id, invoice_no, amount, tax, total, status, due_at, paid_at | FK tenant→subscription; nomor invoice unik sesuai domain penomoran. |
| `payments` | tenant_id, invoice_id, gateway, gateway_ref, amount, status, paid_at | FK invoice; `(gateway,gateway_ref)` unik untuk idempotensi. |
| `payment_webhook_events` | gateway, event_id, payload, processed_at | Global gateway; event unik. Isi payload sensitif dibatasi/retensi. |
| `tenant_usage` | tenant_id, period, employees_count, users_count, storage_mb | FK tenant, unik per bulan. |
| `platform_roles`, `platform_user_roles`, `platform_user_permission_overrides`, `platform_user_tenants` (ada) | role, masa berlaku, keputusan izin, tenant assignment | Menggantikan usulan tabel tunggal `platform_staff`. |
| `support_tickets` | tenant_id (opsional), subject, status, assignee_id | Assignee staf platform. |
| `support_ticket_messages` | ticket_id, author, body | FK tiket; actor tenant/platform perlu tipe eksplisit. |
| `support_sessions` | staff_id, tenant_id, reason, started_at, expires_at, ended_at | FK staf/tenant; satu-satunya jalan support ke data internal tenant. |
| `platform_settings` | key, value JSON | Global, tanpa tenant. |
| `users` (perluas) | email, display_name/full_name, phone, status, password_changed_at, last_login_at | Akun global. Nama field final mengikuti API berjalan. |
| `tenant_memberships` (perluas) | tenant_id, user_id, employee_id, status, access_version | FK komposit `employee_id` ke karyawan tenant yang sama. |
| `sessions` (perluas) | token_hash, user_id, selected_tenant_id, remember, expires_at, idle_expires_at, last_seen_at, ip, user_agent, device_label, revoked_at, revoked_reason | Sesi saat ini sudah menangani idle; selaraskan nama/constraint tanpa mereset sesi. |
| `roles`, `user_roles`, `permission_registry`, `role_permissions`, `user_permission_overrides` (ada) | kode role/permission, masa berlaku, Allow/Deny, entitlement | Preset dan registry aktif dipertahankan. |
| `role_scope_rules/values`, `user_scope_overrides/rules/values` (ada) | jenis scope dan node organisasi | Setara secara perilaku dengan usulan `access_scope_rules/items`; tidak membuat kedua model sekaligus. |
| `login_events` (ada) | user_id, tenant_id, event, ip, user_agent, created_at | Append-only. |
| `access_reviews` | tenant_id, reviewer_id, period, status, finding JSON | Review berkala akses; fase lanjutan. |

### Organisasi dan kepegawaian

| Tabel target | Field domain menurut desain | Relasi/status |
| --- | --- | --- |
| `regions` (perluas) | code, name, deleted_at | Tenant master. |
| `offices` (perluas) | region_id, code, name, address, timezone, lat, lng, is_active, deleted_at | FK komposit ke wilayah. |
| `office_geofences` | office_id, name, polygon/geo_json, is_active | Banyak area per kantor; representasi menunggu D-08. |
| `divisions` (perluas) | code, name, deleted_at | Tenant master. |
| `departments` (perluas) | division_id, code, name, deleted_at | FK komposit ke divisi. |
| `positions` (perluas) | department_id, code, name, grade, level, deleted_at | FK komposit ke departemen. |
| `employees` (perluas) | employee_number, display_name, nickname, gender, birth_date, birth_place, marital_status, nik_enc, nik_hash, work_email, mobile, join_date, end_date, status, photo_file_id, deleted_at | PK lama dipertahankan; penempatan aktif pindah bertahap ke assignment. Enkripsi menunggu D-04. |
| `employee_assignments` | employee_id, region_id, office_id, division_id, department_id, position_id, reports_to_employee_id, grade, valid_from, valid_to, current_flag, created_by | FK komposit ke karyawan dan semua node; hanya satu current per karyawan. Backfill dari `employees` + `employee_assignment_history`. |
| `employee_assignment_history` (ada) | employee_id, base_office_id, position_id, reports_to_id, valid_from, valid_until | Dipertahankan saat masa transisi; strategi arsip/penyatuan ditetapkan setelah verifikasi backfill. |
| `employee_allowed_offices` | employee_id, office_id, valid_from, valid_to, reason | FK karyawan/kantor; kantor tambahan selain base office. |
| `employee_families` | employee_id, relation, name, birth_date, is_dependent | FK karyawan. |
| `employee_emergency_contacts` | employee_id, name, relation, phone | FK karyawan. |
| `employee_bank_accounts` | employee_id, bank_code, account_no_enc, holder_name, is_primary, valid_from, valid_to | FK karyawan; enkripsi menunggu D-04. |
| `employee_tax_bpjs` | employee_id, npwp_enc, ptkp_status, bpjs_kes_no_enc, bpjs_tk_no_enc, valid_from, valid_to | FK karyawan; data berversi dan terenkripsi. |
| `contracts` | employee_id, contract_type, start_date, end_date, status, file_id | FK karyawan/file; indeks akhir kontrak. |
| `employee_changes` | employee_id, change_type, effective_date, new_office_id, new_division_id, new_department_id, new_position_id, new_reports_to, status, request_id, applied_at | FK karyawan/node/request; mutasi menghasilkan assignment baru. |
| `disciplinary_actions` | employee_id, level, issued_date, valid_until, description, file_id | FK karyawan/file. |
| `terminations` | employee_id, type, notice_date, effective_date, reason, status, request_id, processed_at | FK karyawan/request; pencabutan akses pada tanggal efektif. |
| `assets` | code, name, category, status | Master aset tenant. |
| `employee_assets` | employee_id, asset_id, issued_at, returned_at | FK karyawan/aset; histori peminjaman. |

### Jadwal, kehadiran, cuti, dan approval

| Tabel target | Field domain menurut desain | Relasi/status |
| --- | --- | --- |
| `shift_templates` | code, name, start_time, end_time, crosses_midnight, empat batas jendela masuk/pulang beserta offset hari, flex_time, flex_location, late_tolerance_min, break_minutes, target_work_minutes, require_location, require_selfie, require_face_recognition, is_pinned, pin_order, color_hex, is_active | Revisi Product Owner; jendela presensi berbeda dari toleransi keterlambatan. Rincian di `SHIFT_ROSTER_SCHEMA_REVISION.md`. |
| `schedule_patterns` **(baru)** | code, name, pattern_type (weekly/monthly), version, is_pinned, pin_order, is_active | Template pola mingguan dan bulanan yang dapat dipin. |
| `schedule_pattern_slots` **(baru)** | pattern_id, week_no, weekday, day_type (shift/off), shift_template_id | Weekly: W1 × Senin–Minggu; monthly: W1–W5 × Senin–Minggu, W6 mengulang W5. |
| `work_schedules` | employee_id, work_date, shift_template_id, day_type, office_id, source_pattern_id, source_pattern_version, status, created_by, published_at | Roster hasil penerapan pola atau edit manual; FK karyawan/shift/pola/kantor; unik tenant+karyawan+work_date. |
| `shift_instances` | employee_id, work_schedule_id, shift_template_id, office_id, work_date, expected_start, expected_end, empat batas jendela sebagai DATETIME UTC, rule_snapshot, status | FK karyawan/jadwal/shift/kantor; unik tenant+karyawan+work_date. |
| `attendance_events` | employee_id, shift_instance_id, event_type, occurred_at, device_time, lat, lng, accuracy_m, office_id, geofence_id, location_status, timing_status, selfie_file_id, face_verification_attempt_id, source, client_event_id | FK tenant ke karyawan/instance/kantor/geofence/file/attempt; insert-only dan idempotent. |
| `attendance_verification_attempts` **(usulan tingkat lanjut)** | employee_id, shift_instance_id, event_type, client_event_id, result, provider_ref, error_code, checked_at | Hasil face recognition; tidak menyimpan template biometrik mentah. Integrasi endpoint menyusul. |
| `attendance_corrections` | employee_id, shift_instance_id, original_event_id, requested_type, requested_time, reason, status, request_id, result_event_id | FK event/instance/request; hasilnya event baru. |
| `attendance_daily` | employee_id, work_date, shift_instance_id, first_in, last_out, worked_minutes, late_minutes, status, computed_at | Proyeksi yang bisa dibangun ulang dari event. |
| `holidays` | holiday_date, name, type, region_id | Libur tenant/wilayah. |
| `shift_swaps` | requester_id, partner_id, instance_a_id, instance_b_id, status, request_id | FK karyawan/instance/request. |
| `leave_types` | code, name, is_paid, requires_attachment, max_consecutive_days, is_active | Master jenis cuti/izin. |
| `leave_policies` | leave_type_id, accrual_type, quota_days, carry_over_days, expire_months, eligibility JSON, valid_from, valid_to | FK jenis; aturan akrual/prorata menunggu T-12. |
| `leave_balance_ledger` | employee_id, leave_type_id, entry_type, days, effective_date, ref_type, ref_id, note | FK karyawan/jenis; insert-only; idempotensi ref+entry. |
| `leave_balances` | employee_id, leave_type_id, period_year, balance_days | Cache ledger yang dapat dihitung ulang. |
| `leave_requests` | employee_id, leave_type_id, start_date, end_date, days, reason, attachment_file_id, status, request_id | FK karyawan/jenis/file/request; cek tanggal tumpang tindih di transaksi. |
| `overtime_requests` | employee_id, work_date, shift_instance_id, proposed_minutes, approved_minutes, paid_minutes, reason, status, request_id | FK karyawan/instance/request. |
| `requests` | request_type, subject_employee_id, requested_by, ref_table, ref_id, status, submitted_at, final_at | FK subjek/pengaju; relasi polimorfik ref divalidasi aplikasi. |
| `approval_flows` | request_type, name, is_default, valid_from, valid_to, fallback_role_id | FK role fallback. |
| `approval_flow_steps` | flow_id, level_no, approver_kind, approver_ref_id, required_permission, sla_hours | FK flow; referensi approver bertipe perlu validasi. |
| `approval_instances` | request_id, flow_id, current_level, status, version, started_at, finished_at | Satu instance per request, version untuk konkurensi. |
| `approval_tasks` | instance_id, level_no, assignee_membership_id, status, created_at | FK instance/membership; sumber inbox approver. |
| `approval_actions` | instance_id, level_no, actor_membership_id, action, reason, created_at | FK instance/membership; insert-only. |
| `approval_delegations` | from_membership_id, to_membership_id, request_type, valid_from, valid_to | FK dua membership; aturan delegasi menunggu T-11. |

### Kasbon, reimbursement, dan payroll

| Tabel target | Field domain menurut desain | Relasi/status |
| --- | --- | --- |
| `employee_loans` | employee_id, amount, tenor_months, installment_amount, outstanding, status, disbursed_at, disbursement_ref, request_id | FK karyawan/request; pencairan idempotent. |
| `loan_installments` | loan_id, seq, due_period, amount, status, payroll_item_line_id | FK pinjaman/baris payroll. |
| `reimbursement_categories` | code, name, monthly_limit | Master kategori tenant. |
| `reimbursements` | employee_id, category_id, trx_date, amount, description, duplicate_hash, status, payment_ref, paid_at, request_id | FK karyawan/kategori/request; cek duplikasi dan pembayaran. |
| `reimbursement_receipts` | **tenant_id (disetujui)**, reimbursement_id, file_id | FK komposit ke klaim/file; desain awal belum mencantumkan tenant_id. |
| `pay_components` | code, name, type, taxable, calc_type, is_system | Master komponen gaji tenant. |
| `employee_pay_components` | employee_id, component_id, amount, valid_from, valid_to | FK karyawan/komponen; berversi. |
| `tax_rules` | rule_set, effective_from, effective_to, data JSON | Aturan global/tenant menunggu D-05. |
| `bpjs_rules` | program, effective_from, effective_to, rate_employee, rate_employer, wage_cap | Aturan global/tenant menunggu D-05. |
| `payroll_periods` | year, month, start_date, end_date, status | Unik tenant+tahun+bulan. |
| `payroll_runs` | period_id, run_no, status, rules_snapshot JSON, total_gross, total_net, calculated_at, finalized_at, finalized_by | FK periode; snapshot aturan. |
| `payroll_items` | run_id, employee_id, gross, total_deduction, tax, net, employee_snapshot JSON | FK run/karyawan; unik tenant+run+karyawan. |
| `payroll_item_lines` | item_id, component_code, line_type, amount, source_type, source_id, description | FK item; sumber polimorfik diverifikasi aplikasi. |
| `payroll_adjustments` | run_id, employee_id, reason, amount, request_id, created_by | Koreksi setelah final lewat baris baru. |
| `payslips` | payroll_item_id, file_id, published_at, viewed_at | FK item/file; satu slip per item. |

### Notifikasi, komunikasi, dokumen, audit, dan integrasi

| Tabel target | Field domain menurut desain | Relasi/status |
| --- | --- | --- |
| `notifications` | recipient_membership_id, category, event_type, title, body, deep_link, payload JSON, dedupe_key, priority, read_at, archived_at, obsolete_at | FK membership; inbox sumber kebenaran, cursor perlu keputusan ID publik+sequence. |
| `notification_outbox` | event_type, aggregate_type, aggregate_id, payload JSON, dedupe_key, status, attempts, next_attempt_at, processed_at | Ditulis atomik dengan transaksi bisnis; agregat polimorfik. |
| `notification_schedules` | rule_id, subject_type, subject_id, recipient_membership_id, fire_at, attempt_no, status, dedupe_key | FK aturan/penerima; subjek polimorfik. |
| `notification_events` | event_type, aggregate_type, aggregate_id, created_at | Log event opsional, append-only. |
| `notification_deliveries` | notification_id, channel, status, error, attempted_at | FK notifikasi; terkirim berbeda dari dibaca. |
| `notification_rules` | event_type, enabled, config JSON, updated_by | Aturan reminder per tenant. |
| `notification_templates` | tenant_id (nullable untuk default platform), event_type, channel, title_tpl, body_tpl | Perhatikan UNIQUE dengan tenant_id NULL pada MariaDB; perlu desain kunci default terpisah. |
| `notification_preferences` | **tenant_id (disetujui)**, membership_id, category, channel, enabled | FK komposit membership; notifikasi keamanan tetap wajib. |
| `push_subscriptions` | membership_id, endpoint_hash, endpoint, p256dh, auth, user_agent, last_success_at, revoked_at | FK membership; rahasia push terenkripsi/dibatasi. |
| `broadcasts` | title, body, attachment_file_id, status, send_at, combine_mode, recipients_count, snapshot_at, created_by, sent_by | FK file/aktor; status draft→sent. |
| `broadcast_targets` | broadcast_id, target_type, ref_id | FK broadcast; target polimorfik divalidasi. |
| `broadcast_recipients` | broadcast_id, membership_id, employee_id, notification_id, delivery_status, read_at | Snapshot penerima; unik tenant+broadcast+membership. |
| `files` | storage_key, original_name, mime_type, size_bytes, sha256, purpose, uploaded_by, scan_status, created_at, deleted_at | Metadata objek, bukan isi berkas; FK tenant/aktor. |
| `document_types` | code, name, requires_expiry, is_mandatory | Master dokumen tenant. |
| `employee_documents` | employee_id, document_type_id, file_id, issued_date, expires_at, status | FK karyawan/jenis/file. |
| `company_documents` | title, category, file_id, visibility, published_at | FK file; scope visibility harus didefinisikan. |
| `document_templates` | code, name, body, variables JSON | Template tenant. |
| `agenda_events` | title, description, starts_at, ends_at, all_day, location, visibility, created_by | FK aktor tenant. |
| `audit_logs` (perluas) | actor_type, actor_id, support_session_id, action, entity_type, entity_id, before_json, after_json, ip, user_agent, request_id, created_at | Existing hanya actor_user_id dan field inti; penambahan harus mempertahankan audit lama dan menyaring data sensitif. |
| `import_jobs` | type, file_id, status, total_rows, ok_rows, error_rows, created_by | FK file/aktor; scope tiap baris. |
| `import_job_errors` | **tenant_id (disetujui)**, job_id, row_no, field, message | FK komposit job; desain awal belum mencantumkan tenant_id. |
| `export_jobs` | type, filters JSON, scope_snapshot JSON, status, file_id, row_count, created_by, expires_at | FK file/aktor; selalu audit dan scope. |
| `tenant_settings` | setting_key, value JSON, updated_by | PK tenant+setting; tema, absensi, sesi. |
| `api_keys` [BERTAHAP] | Belum dirinci dalam dokumen | DDL menunggu kontrak integrasi/rotasi/secret hashing. |
| `webhooks` [BERTAHAP] | Belum dirinci dalam dokumen | DDL menunggu kontrak event, signature, dan retry. |
| Tabel talenta [BERTAHAP] | Belum dirinci dalam dokumen | Kinerja, rekrutmen, pelatihan ditentukan di Fase 5; tidak mengarang field final. |

## 4. Urutan migrasi aditif yang diusulkan

Nomor di bawah adalah **kelompok kerja**, bukan file SQL yang sudah dibuat. Tiap kelompok dipecah menjadi beberapa migrasi Goose berurutan mulai `00011`; DDL dibekukan setelah keputusan terkait. Jangan mengubah migrasi `00001`–`00010` atau membuat ulang database staging.

| Urutan | Perubahan | Backfill/cutover | Gerbang verifikasi |
| --- | --- | --- | --- |
| 0. Baseline | Simpan snapshot skema/row count/FK/index, backup dan uji restore di lingkungan terpisah. Jalankan kajian D-03 terpisah; tetapkan D-04, D-08, D-09 pada waktunya. | Tidak mengubah data. | Backup terbaca, checksum dan jumlah baris awal tercatat. |
| 1. Fondasi data | Perluas `plans`, `plan_entitlements`, lifecycle tenant/langganan; tambah `tenant_settings`, `files`, metadata pengguna, constraint tenant yang belum ada. | Field baru nullable/default; API lama tetap membaca. | Migrasi naik pada salinan staging, rollback atau compensating migration teruji, data demo tetap utuh. |
| 2. Organisasi dan assignment | Perluas master; buat `employee_assignments`, `employee_allowed_offices`, profil karyawan bertahap. | Backfill 5 dimensi dari kantor/jabatan existing, periksa hubungan wilayah/divisi/departemen; dual-write dan bandingkan hasil scope; cutover baca setelah sama. | Satu assignment aktif, tidak tumpang tindih, FK lintas tenant gagal, daftar/detail/ubah karyawan tidak berubah salah. |
| 3. Approval dasar + outbox **(disetujui lebih dulu)** | Buat `requests`, flows/steps, instances/tasks/actions, serta `notification_outbox` dan `notifications` minimum. | Approval tidak diaktifkan sebelum evaluator permission+scope dan anti self-approve lulus. | Dua approver serentak menghasilkan satu keputusan; event outbox idempotent. |
| 4. SDM dan dokumen | Buat kontrak, perubahan karyawan, terminasi, aset, family/contact/bank/tax, dokumen, agenda, impor/ekspor. | Mutasi/terminasi aktif setelah mesin approval dasar; field sensitif bergantung D-04. | Tanggal efektif benar, sesi/role dicabut pada terminasi, berkas tetap milik tenant. |
| 5. Jadwal dan kehadiran | Buat geofence, shift dengan empat batas jendela presensi, pola mingguan/bulanan, roster, instance, event, koreksi, rekap, libur, swap, aturan reminder. | D-01 ditutup: satu shift/karyawan/work_date. D-08 menentukan polygon. Face recognition diaktifkan setelah kontrak endpoint, sementara flag shift dapat disiapkan. | Batas jendela/toleransi, pola W1–W5 dengan W6 mengulang W5, pin/warna, shift lintas hari, retry event, dan tenant FK teruji. |
| 6. Cuti, lembur, kasbon, reimbursement | Buat master/policy/ledger/proyeksi, permintaan domain, pinjaman/cicilan, klaim/kuitansi. | T-11/T-12 menentukan delegasi/akrual; approval dari urutan 3 wajib aktif. | Saldo/pencairan/pembayaran hanya sekali; perubahan memakai event reversal. |
| 7. Payroll | Buat komponen, periode/run/item/line/adjustment/slip, aturan pajak/BPJS berversi. | D-05 dan D-09 diselesaikan; snapshot final dibuat dari data yang sudah stabil. | Nominal `DECIMAL`, final tidak berubah, slip dan ekspor bank dicek ulang scope. |
| 8. Platform lanjutan | Billing invoice/payment/webhook, kuota, support tickets/session, notifikasi kanal, broadcast penuh. | Histori langganan dan pemrosesan gateway diputuskan sebelum pembayaran. | Webhook idempotent, staf platform hanya tenant tugasnya, support session berakhir tepat waktu. |
| 9. Fase 5 | Talenta, API key, webhook keluar, access review, optimasi/arsip. | Skema dibuat setelah spesifikasi modul dan kebijakan retensi. | Tes isolasi tenant, audit, dan restore untuk tiap modul. |

Setiap migrasi: (1) tambah tabel/kolom/FK/index tanpa menghapus field yang dipakai API; (2) backfill batch idempotent; (3) dual-read/dual-write bila mengganti sumber data; (4) uji jumlah baris, FK dan query tenant; (5) alihkan API; (6) baru bersihkan kolom lama di migrasi terpisah setelah masa observasi. Untuk operasi DDL MariaDB yang tidak atomik, rollback berarti migrasi kompensasi dan restore yang sudah diuji, bukan mengandalkan `Down` saja.

## 5. Keputusan dan tambahan yang perlu dibahas sebelum SQL baru

| Topik | Rekomendasi awal | Perlu keputusan |
| --- | --- | --- |
| D-03 identitas | **Diputuskan: kajian `BIGINT` terpisah** di `docs/BIGINT_PK_FEASIBILITY.md`. ID 26 karakter yang ada tetap dipakai selama studi; hasil kajian menentukan PK tabel baru dan perlu/tidaknya migrasi legacy. | Belum ada persetujuan untuk mengubah PK/FK. |
| Model langganan | Pakai `tenant_subscriptions` sebagai pointer aktif; tambah histori periode/langganan bila billing dibangun. | Nama dan sumber status utama sebelum invoice/payment. |
| D-06 approval | **Disetujui:** mesin approval dasar sebelum mutasi/terminasi aktif. | Urutan implementasi terkunci; detail flow/fallback masih perlu spesifikasi. |
| D-08 geofence | Uji spatial MariaDB dan JSON+Go pada polygon nyata sebelum DDL. | Pilihan penyimpanan dan validasi final. |
| D-04 data sensitif | Tambah field terenkripsi hanya setelah kunci, rotasi, backup, dan hak baca disepakati. | Pengelolaan kunci per tenant. |
| D-01/D-02 | **D-01 disetujui:** satu shift per karyawan per `work_date`. Scope historis menurut penugasan saat ini masih usulan D-02. | D-02 ditetapkan sebelum query historis attendance. |
| D-05/D-09 | Aturan pajak/BPJS global dan kemungkinan legal entity belum diputuskan. | Tetapkan sebelum payroll. |
| Tambahan tenant ID | **Disetujui:** tambah `tenant_id` pada kuitansi, preferensi notifikasi, dan error impor agar FK komposit bisa ditegakkan. | Masukkan saat migrasi tabel terkait. |
| Cursor notifikasi | ID publik 26 karakter + nomor urut database untuk cursor sinkronisasi (usulan). | Pilih setelah kajian D-03 dan uji konsistensi cursor. |
| Retensi D-07 | Belum menetapkan angka retensi pada tabel event/audit/inbox. | Tentukan sebelum produksi, bukan prasyarat struktur awal. |

**Revisi shift/roster 5 November 2026:** detail field, perilaku jendela, pola, pin, warna, serta persiapan face recognition ada pada `docs/SHIFT_ROSTER_SCHEMA_REVISION.md`. Keputusan pola bulanan W1–W5 dan pengulangan W5 pada baris kalender keenam sudah diberikan Product Owner.

**Belum dikerjakan:** tidak ada SQL/migrasi `00011+` dalam draf ini; tidak ada perubahan data staging. Tabel talenta, API key, dan webhook belum memiliki field final di desain, sehingga bagian itu memerlukan spesifikasi produk sebelum DDL.
