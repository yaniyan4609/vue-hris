-- +goose Up
ALTER TABLE employee_allowed_offices
  ADD COLUMN revoked_at DATETIME(6) NULL,
  ADD COLUMN revoked_by CHAR(26) CHARACTER SET ascii COLLATE ascii_bin NULL,
  ADD KEY idx_allowed_office_active (tenant_id,employee_id,office_id,revoked_at,valid_from,valid_to),
  ADD CONSTRAINT fk_allowed_office_revoker FOREIGN KEY (revoked_by) REFERENCES users (id);

-- +goose Down
-- Riwayat cabut izin kantor dipertahankan. Rollback memerlukan migrasi kompensasi yang ditinjau.
SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='Use a reviewed compensating migration for 00030';
