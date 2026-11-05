-- +goose NO TRANSACTION
-- +goose Up
CREATE TABLE subscription_periods (
  tenant_id CHAR(26) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  id CHAR(26) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  plan_id CHAR(26) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  billing_cycle VARCHAR(16) NOT NULL,
  period_start DATETIME(6) NOT NULL,
  period_end DATETIME(6) NOT NULL,
  status VARCHAR(24) NOT NULL,
  created_at DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
  PRIMARY KEY (tenant_id,id),
  KEY idx_subscription_history (tenant_id,period_start,period_end),
  FOREIGN KEY (tenant_id) REFERENCES tenants (id),
  FOREIGN KEY (plan_id) REFERENCES plans (id),
  CONSTRAINT ck_subscription_period_dates CHECK (period_end > period_start),
  CONSTRAINT ck_subscription_period_cycle CHECK (billing_cycle IN ('monthly','yearly'))
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

ALTER TABLE tenant_subscriptions
  ADD COLUMN current_period_id CHAR(26) CHARACTER SET ascii COLLATE ascii_bin NULL,
  ADD CONSTRAINT fk_subscription_current_period FOREIGN KEY (tenant_id,current_period_id) REFERENCES subscription_periods (tenant_id,id);

CREATE TABLE invoices (
  tenant_id CHAR(26) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  id CHAR(26) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  subscription_period_id CHAR(26) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  invoice_no VARCHAR(80) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  amount DECIMAL(18,2) NOT NULL,
  tax DECIMAL(18,2) NOT NULL DEFAULT 0,
  total DECIMAL(18,2) NOT NULL,
  status VARCHAR(24) NOT NULL DEFAULT 'draft',
  due_at DATETIME(6) NOT NULL,
  paid_at DATETIME(6) NULL,
  created_at DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
  PRIMARY KEY (tenant_id,id),
  UNIQUE KEY uq_invoice_number (invoice_no),
  KEY idx_invoice_status_due (status,due_at),
  FOREIGN KEY (tenant_id,subscription_period_id) REFERENCES subscription_periods (tenant_id,id),
  CONSTRAINT ck_invoice_amount CHECK (amount >= 0 AND tax >= 0 AND total = amount + tax)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE payments (
  tenant_id CHAR(26) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  id CHAR(26) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  invoice_id CHAR(26) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  gateway VARCHAR(48) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  gateway_ref VARCHAR(160) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  amount DECIMAL(18,2) NOT NULL,
  status VARCHAR(24) NOT NULL DEFAULT 'pending',
  paid_at DATETIME(6) NULL,
  created_at DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
  PRIMARY KEY (tenant_id,id),
  UNIQUE KEY uq_payment_gateway_ref (gateway,gateway_ref),
  KEY idx_payment_invoice (tenant_id,invoice_id,status),
  FOREIGN KEY (tenant_id,invoice_id) REFERENCES invoices (tenant_id,id),
  CONSTRAINT ck_payment_amount CHECK (amount > 0)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE payment_webhook_events (
  id CHAR(26) CHARACTER SET ascii COLLATE ascii_bin NOT NULL PRIMARY KEY,
  gateway VARCHAR(48) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  event_id VARCHAR(160) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  payload_json JSON NOT NULL,
  processed_at DATETIME(6) NULL,
  created_at DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
  UNIQUE KEY uq_payment_webhook_event (gateway,event_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE tenant_usage (
  tenant_id CHAR(26) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  period DATE NOT NULL,
  employees_count INT UNSIGNED NOT NULL DEFAULT 0,
  users_count INT UNSIGNED NOT NULL DEFAULT 0,
  storage_mb BIGINT UNSIGNED NOT NULL DEFAULT 0,
  computed_at DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
  PRIMARY KEY (tenant_id,period),
  FOREIGN KEY (tenant_id) REFERENCES tenants (id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE support_tickets (
  id CHAR(26) CHARACTER SET ascii COLLATE ascii_bin NOT NULL PRIMARY KEY,
  tenant_id CHAR(26) CHARACTER SET ascii COLLATE ascii_bin NULL,
  subject VARCHAR(255) NOT NULL,
  status VARCHAR(24) NOT NULL DEFAULT 'open',
  assignee_user_id CHAR(26) CHARACTER SET ascii COLLATE ascii_bin NULL,
  created_by CHAR(26) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  created_at DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
  updated_at DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6) ON UPDATE CURRENT_TIMESTAMP(6),
  KEY idx_support_ticket_tenant (tenant_id,status,created_at),
  FOREIGN KEY (tenant_id) REFERENCES tenants (id),
  FOREIGN KEY (assignee_user_id) REFERENCES users (id),
  FOREIGN KEY (created_by) REFERENCES users (id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE support_ticket_messages (
  id CHAR(26) CHARACTER SET ascii COLLATE ascii_bin NOT NULL PRIMARY KEY,
  ticket_id CHAR(26) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  author_user_id CHAR(26) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  author_context VARCHAR(16) NOT NULL,
  body TEXT NOT NULL,
  created_at DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
  KEY idx_ticket_messages (ticket_id,created_at),
  FOREIGN KEY (ticket_id) REFERENCES support_tickets (id),
  FOREIGN KEY (author_user_id) REFERENCES users (id),
  CONSTRAINT ck_ticket_author_context CHECK (author_context IN ('platform','tenant'))
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE support_sessions (
  id CHAR(26) CHARACTER SET ascii COLLATE ascii_bin NOT NULL PRIMARY KEY,
  staff_user_id CHAR(26) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  tenant_id CHAR(26) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  reason TEXT NOT NULL,
  started_at DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
  expires_at DATETIME(6) NOT NULL,
  ended_at DATETIME(6) NULL,
  KEY idx_support_session_active (staff_user_id,tenant_id,expires_at),
  FOREIGN KEY (staff_user_id) REFERENCES users (id),
  FOREIGN KEY (tenant_id) REFERENCES tenants (id),
  CONSTRAINT ck_support_session_dates CHECK (expires_at > started_at AND (ended_at IS NULL OR ended_at >= started_at))
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE platform_settings (
  setting_key VARCHAR(96) CHARACTER SET ascii COLLATE ascii_bin NOT NULL PRIMARY KEY,
  value_json JSON NOT NULL,
  updated_by CHAR(26) CHARACTER SET ascii COLLATE ascii_bin NULL,
  updated_at DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6) ON UPDATE CURRENT_TIMESTAMP(6),
  FOREIGN KEY (updated_by) REFERENCES users (id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE access_reviews (
  tenant_id CHAR(26) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  id CHAR(26) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  reviewer_user_id CHAR(26) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  period DATE NOT NULL,
  status VARCHAR(24) NOT NULL DEFAULT 'draft',
  findings_json JSON NULL,
  created_at DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
  PRIMARY KEY (tenant_id,id),
  KEY idx_access_review_period (tenant_id,period,status),
  FOREIGN KEY (tenant_id,reviewer_user_id) REFERENCES tenant_memberships (tenant_id,user_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

-- +goose Down
-- Billing and support records are retained on rollback.
SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='Use a reviewed compensating migration for 00019';
