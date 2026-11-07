-- +goose NO TRANSACTION
-- +goose Up
CREATE TABLE request_revisions (
  tenant_id CHAR(26) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  id CHAR(26) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  request_id CHAR(26) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  attempt_no INT UNSIGNED NOT NULL,
  correction_json JSON NOT NULL,
  submitted_by CHAR(26) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  submitted_at DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
  PRIMARY KEY (tenant_id,id),
  UNIQUE KEY uq_request_revision_attempt (tenant_id,request_id,attempt_no),
  CONSTRAINT fk_request_revision_request FOREIGN KEY (tenant_id,request_id) REFERENCES requests (tenant_id,id),
  CONSTRAINT fk_request_revision_submitter FOREIGN KEY (tenant_id,submitted_by) REFERENCES tenant_memberships (tenant_id,user_id),
  CONSTRAINT ck_request_revision_attempt CHECK (attempt_no > 1)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

-- +goose Down
DROP TABLE request_revisions;
