-- +goose NO TRANSACTION
-- +goose Up
-- Matriks memberi HR Staff akses lihat pada kebijakan cuti, notifikasi, dan kebijakan HR.
DELETE v FROM role_scope_values v JOIN role_scope_rules sr ON sr.tenant_id=v.tenant_id AND sr.id=v.rule_id JOIN roles r ON r.tenant_id=sr.tenant_id AND r.id=sr.role_id WHERE r.code='HR_STAFF' AND sr.permission_code='cuti.kelola';
DELETE sr FROM role_scope_rules sr JOIN roles r ON r.tenant_id=sr.tenant_id AND r.id=sr.role_id WHERE r.code='HR_STAFF' AND sr.permission_code='cuti.kelola';
DELETE rp FROM role_permissions rp JOIN roles r ON r.tenant_id=rp.tenant_id AND r.id=rp.role_id WHERE r.code='HR_STAFF' AND rp.permission_code IN ('cuti.kelola','notifikasi.kelola','pengaturan.kelola');

-- +goose Down
-- Preset tidak dipulihkan otomatis agar perubahan hak setelah migrasi tidak tertimpa.
