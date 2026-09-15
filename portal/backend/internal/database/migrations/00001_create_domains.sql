-- +goose Up
-- +goose StatementBegin
CREATE TABLE domains (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name VARCHAR(255) NOT NULL UNIQUE,
    tenant_id UUID,
    status VARCHAR(20) NOT NULL DEFAULT 'pending',
    stalwart_domain_id VARCHAR(100),
    verified_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_domains_status ON domains(status);
CREATE INDEX idx_domains_name ON domains(name);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE domains;
-- +goose StatementEnd
