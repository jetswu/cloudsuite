-- Sprint 1.4: provisioning job queue + per-user provisioning state.
-- user_id references the Authentik user PK (external IdP DB, no FK possible).

-- +goose Up
-- +goose StatementBegin
CREATE TABLE provisioning_jobs (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id INT NOT NULL,
    user_email VARCHAR(255) NOT NULL,
    action VARCHAR(20) NOT NULL,
    service VARCHAR(50) NOT NULL,
    status VARCHAR(20) NOT NULL DEFAULT 'queued',
    attempts INT NOT NULL DEFAULT 0,
    max_attempts INT NOT NULL DEFAULT 3,
    next_retry_at TIMESTAMPTZ,
    last_error TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT chk_prov_action CHECK (action IN ('provision', 'deprovision')),
    CONSTRAINT chk_prov_service CHECK (service IN ('stalwart', 'nextcloud', 'odoo')),
    CONSTRAINT chk_prov_status CHECK (status IN ('queued', 'running', 'success', 'failed'))
);
-- +goose StatementEnd

-- +goose StatementBegin
CREATE INDEX idx_prov_status ON provisioning_jobs(status, next_retry_at);
-- +goose StatementEnd

-- +goose StatementBegin
CREATE INDEX idx_prov_user ON provisioning_jobs(user_id, action);
-- +goose StatementEnd

-- +goose StatementBegin
CREATE TABLE user_provisioning (
    user_id INT PRIMARY KEY,
    user_email VARCHAR(255) NOT NULL,
    stalwart_status VARCHAR(20) NOT NULL DEFAULT 'pending',
    nextcloud_status VARCHAR(20) NOT NULL DEFAULT 'pending',
    odoo_status VARCHAR(20) NOT NULL DEFAULT 'pending',
    stalwart_external_id VARCHAR(255),
    nextcloud_external_id VARCHAR(255),
    odoo_external_id VARCHAR(255),
    last_error TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT chk_up_stalwart CHECK (stalwart_status IN ('pending', 'provisioning', 'active', 'failed', 'deleted')),
    CONSTRAINT chk_up_nextcloud CHECK (nextcloud_status IN ('pending', 'provisioning', 'active', 'failed', 'deleted')),
    CONSTRAINT chk_up_odoo CHECK (odoo_status IN ('pending', 'provisioning', 'active', 'failed', 'deleted'))
);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS user_provisioning;
-- +goose StatementEnd

-- +goose StatementBegin
DROP TABLE IF EXISTS provisioning_jobs;
-- +goose StatementEnd
