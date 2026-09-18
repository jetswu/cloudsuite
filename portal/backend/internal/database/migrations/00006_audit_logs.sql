-- Sprint 1.5a: append-only audit trail for admin actions performed through
-- the portal (users, groups, domains, provisioning retries). UPDATE and
-- DELETE are rejected by trigger: rows are immutable forensic evidence.

-- +goose Up
CREATE TABLE audit_logs (
  id BIGSERIAL PRIMARY KEY,
  tenant_id UUID,
  actor_id INT,               -- Authentik user PK (nil for system actors)
  actor_email VARCHAR(255),
  actor_type VARCHAR(20) NOT NULL DEFAULT 'user',
  action VARCHAR(100) NOT NULL,
  target_type VARCHAR(50),
  target_id VARCHAR(255),
  service_code VARCHAR(50),
  ip_address INET,
  user_agent TEXT,
  metadata JSONB DEFAULT '{}',
  created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_audit_logs_created ON audit_logs(created_at DESC);
CREATE INDEX idx_audit_logs_actor ON audit_logs(actor_id, created_at DESC);
CREATE INDEX idx_audit_logs_action ON audit_logs(action, created_at DESC);
CREATE INDEX idx_audit_logs_service ON audit_logs(service_code, created_at DESC);

-- Immutability: the trail must not be editable, even by the portal DB role.
-- NOTE: goose splits statements on ';', so the PL/pgSQL function and the
-- trigger statements MUST be wrapped in StatementBegin/StatementEnd
-- (learned the hard way: "unterminated dollar-quoted string" crash loop).
-- +goose StatementBegin
CREATE OR REPLACE FUNCTION prevent_audit_modification()
RETURNS TRIGGER AS $$
BEGIN
  RAISE EXCEPTION 'audit_logs is append-only';
END;
$$ LANGUAGE plpgsql;
-- +goose StatementEnd

-- +goose StatementBegin
CREATE TRIGGER trg_audit_logs_no_update
  BEFORE UPDATE ON audit_logs
  FOR EACH ROW EXECUTE FUNCTION prevent_audit_modification();
-- +goose StatementEnd

-- +goose StatementBegin
CREATE TRIGGER trg_audit_logs_no_delete
  BEFORE DELETE ON audit_logs
  FOR EACH ROW EXECUTE FUNCTION prevent_audit_modification();
-- +goose StatementEnd

-- +goose Down
DROP TRIGGER IF EXISTS trg_audit_logs_no_update ON audit_logs;
DROP TRIGGER IF EXISTS trg_audit_logs_no_delete ON audit_logs;
DROP FUNCTION IF EXISTS prevent_audit_modification();
DROP INDEX IF EXISTS idx_audit_logs_service;
DROP INDEX IF EXISTS idx_audit_logs_action;
DROP INDEX IF EXISTS idx_audit_logs_actor;
DROP INDEX IF EXISTS idx_audit_logs_created;
DROP TABLE IF EXISTS audit_logs;
