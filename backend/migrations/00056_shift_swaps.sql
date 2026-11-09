-- +goose NO TRANSACTION
-- +goose Up
ALTER TABLE shift_swaps
  ADD COLUMN reason VARCHAR(500) NOT NULL DEFAULT '',
  ADD COLUMN version BIGINT UNSIGNED NOT NULL DEFAULT 1,
  ADD COLUMN responded_at DATETIME(6) NULL,
  ADD COLUMN applied_at DATETIME(6) NULL,
  ADD COLUMN created_at DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
  ADD COLUMN updated_at DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6) ON UPDATE CURRENT_TIMESTAMP(6),
  ADD KEY idx_shift_swaps_requester (tenant_id,requester_id,created_at),
  ADD KEY idx_shift_swaps_partner (tenant_id,partner_id,created_at),
  ADD KEY idx_shift_swaps_status (tenant_id,status,created_at),
  ADD CONSTRAINT ck_shift_swaps_status CHECK (status IN ('pending_partner','in_review','returned','approved','rejected','cancelled'));

INSERT IGNORE INTO role_permissions (tenant_id,role_id,permission_code)
SELECT tenant_id,id,'jadwal.ubah' FROM roles WHERE code='KARYAWAN';
INSERT INTO role_scope_rules (tenant_id,id,role_id,permission_code,kind)
SELECT r.tenant_id,LEFT(REPLACE(UUID(),'-',''),26),r.id,'jadwal.ubah','self'
FROM roles r LEFT JOIN role_scope_rules s ON s.tenant_id=r.tenant_id AND s.role_id=r.id AND s.permission_code='jadwal.ubah'
WHERE r.code='KARYAWAN' AND s.id IS NULL;

-- +goose Down
SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='Use a reviewed compensating migration for 00056';
